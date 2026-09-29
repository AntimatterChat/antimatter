// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

// Package autotranslation implements einterfaces.AutoTranslationInterface:
// per-channel automatic translation of posts into the languages of the
// channel members, backed by a LibreTranslate server or an LLM reached
// through the Agents plugin.
//
// Translate records a "processing" translation for every language the
// channel needs and returns immediately; a pool of workers then calls the
// provider, stores the result and notifies the channel over the websocket.
// A periodic recovery job re-queues translations left processing, for
// example by a restart.
package autotranslation

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

func init() {
	app.RegisterAutoTranslationInterface(func(s *app.Server) einterfaces.AutoTranslationInterface {
		return New(newServerHost(s))
	})
}

// Service implements einterfaces.AutoTranslationInterface.
type Service struct {
	h    host
	pool *workerPool

	provMu  sync.Mutex
	prov    provider
	provKey string

	lifecycleMu sync.Mutex
	listenerID  string
	workers     int
}

var _ einterfaces.AutoTranslationInterface = (*Service)(nil)

// New creates the auto-translation service. Workers start with Start.
func New(h host) *Service {
	s := &Service{h: h}
	s.pool = newWorkerPool(s.processTask, func(depth int) {
		if m := s.h.Metrics(); m != nil {
			m.SetAutoTranslateQueueDepth(float64(depth))
		}
	})
	return s
}

// featureAvailable reports whether the feature flag, the enable switch and
// the provider configuration all allow auto-translation.
func featureAvailable(cfg *model.Config) (bool, string) {
	if cfg == nil {
		return false, "no configuration"
	}
	if cfg.FeatureFlags == nil || !cfg.FeatureFlags.AutoTranslation {
		return false, "feature flag disabled"
	}
	settings := cfg.AutoTranslationSettings
	if !model.SafeDereference(settings.Enable) {
		return false, "disabled in configuration"
	}
	switch model.SafeDereference(settings.Provider) {
	case providerLibreTranslate:
		if settings.LibreTranslate == nil || model.SafeDereference(settings.LibreTranslate.URL) == "" {
			return false, "LibreTranslate URL not configured"
		}
	case providerAgents:
		if settings.Agents == nil || model.SafeDereference(settings.Agents.LLMServiceID) == "" {
			return false, "Agents LLM service not configured"
		}
	default:
		return false, "no supported provider configured"
	}
	return true, ""
}

func (s *Service) IsFeatureAvailable() bool {
	ok, _ := featureAvailable(s.h.Config())
	return ok
}

// notAvailableError returns the error callers recognise as "feature not
// available" (they test for *model.ErrAutoTranslationNotAvailable).
func (s *Service) notAvailableError(where string) *model.AppError {
	if ok, reason := featureAvailable(s.h.Config()); !ok {
		return model.NewAppError(where, "ent.autotranslation.feature_unavailable", nil, reason, http.StatusForbidden).
			Wrap(model.NewErrAutoTranslationNotAvailable(reason))
	}
	return nil
}

func storeError(where string, err error) *model.AppError {
	return model.NewAppError(where, "ent.autotranslation.store_error", nil, "", http.StatusInternalServerError).Wrap(err)
}

func (s *Service) IsChannelEnabled(channelID string) (bool, *model.AppError) {
	if !s.IsFeatureAvailable() {
		return false, nil
	}
	if appErr := validateID("IsChannelEnabled", channelID); appErr != nil {
		return false, appErr
	}
	channel, err := s.h.Store().Channel().Get(channelID, true)
	if err != nil {
		var nfErr *store.ErrNotFound
		if errors.As(err, &nfErr) {
			return false, nil
		}
		return false, storeError("IsChannelEnabled", err)
	}
	return s.channelEnabled(channel), nil
}

func (s *Service) channelEnabled(channel *model.Channel) bool {
	if channel == nil || !channel.AutoTranslation || channel.DeleteAt != 0 {
		return false
	}
	if channel.IsGroupOrDirect() && model.SafeDereference(s.h.Config().AutoTranslationSettings.RestrictDMAndGM) {
		return false
	}
	return true
}

func (s *Service) IsUserEnabled(channelID, userID string) (bool, *model.AppError) {
	enabled, appErr := s.IsChannelEnabled(channelID)
	if appErr != nil || !enabled {
		return false, appErr
	}
	if appErr := validateID("IsUserEnabled", userID); appErr != nil {
		return false, appErr
	}
	ok, err := s.h.Store().AutoTranslation().IsUserEnabled(userID, channelID)
	if err != nil {
		return false, storeError("IsUserEnabled", err)
	}
	return ok, nil
}

func (s *Service) GetUserLanguage(userID, channelID string) (string, *model.AppError) {
	if appErr := s.notAvailableError("GetUserLanguage"); appErr != nil {
		return "", appErr
	}
	if userID == "" || channelID == "" {
		return "", nil
	}
	enabled, appErr := s.IsChannelEnabled(channelID)
	if appErr != nil || !enabled {
		return "", appErr
	}
	locale, err := s.h.Store().AutoTranslation().GetUserLanguage(userID, channelID)
	if err != nil {
		return "", storeError("GetUserLanguage", err)
	}
	lang := normalizeLang(locale)
	if lang == "" || !targetLanguageSet(s.targetLanguages())[lang] {
		return "", nil
	}
	return lang, nil
}

func (s *Service) targetLanguages() []string {
	if t := s.h.Config().AutoTranslationSettings.TargetLanguages; t != nil {
		return *t
	}
	return nil
}

func (s *Service) GetBatch(objectType string, objectIDs []string, dstLang string) (map[string]*model.Translation, *model.AppError) {
	if appErr := s.notAvailableError("GetBatch"); appErr != nil {
		return nil, appErr
	}
	lang := normalizeLang(dstLang)
	if objectType == "" || lang == "" || len(objectIDs) == 0 {
		return map[string]*model.Translation{}, nil
	}
	result, err := s.h.Store().AutoTranslation().GetBatch(objectType, objectIDs, lang)
	if err != nil {
		return nil, storeError("GetBatch", err)
	}
	return result, nil
}

func (s *Service) DetectRemote(ctx context.Context, text string) (string, *float64, *model.AppError) {
	if appErr := s.notAvailableError("DetectRemote"); appErr != nil {
		return "", nil, appErr
	}
	if ctx == nil {
		return "", nil, model.NewAppError("DetectRemote", "ent.autotranslation.detect_language.nil_context", nil, "", http.StatusBadRequest)
	}
	if len(text) > maxContentBytes {
		return "", nil, model.NewAppError("DetectRemote", "ent.autotranslation.detect_language.text_too_large", nil, "", http.StatusRequestEntityTooLarge)
	}
	prov, err := s.getProvider()
	if err != nil {
		return "", nil, model.NewAppError("DetectRemote", "ent.autotranslation.provider_not_initialized", nil, "", http.StatusServiceUnavailable).Wrap(err)
	}

	ctx, cancel := context.WithTimeout(ctx, providerTimeout(s.h.Config()))
	defer cancel()

	start := time.Now()
	lang, conf, err := prov.Detect(ctx, truncateBytes(text, maxDetectionBytes))
	s.observeProviderCall(prov.Name(), err, start)
	if err != nil {
		return "", nil, model.NewAppError("DetectRemote", "ent.autotranslation.detect_remote.error", nil, "", http.StatusBadGateway).Wrap(err)
	}
	return lang, conf, nil
}

// getProvider returns the provider for the current configuration, building
// a new one when the relevant settings changed.
func (s *Service) getProvider() (provider, error) {
	cfg := s.h.Config()
	settings := cfg.AutoTranslationSettings
	key := model.SafeDereference(settings.Provider) + "\x00" + providerTimeout(cfg).String()
	if settings.LibreTranslate != nil {
		key += "\x00" + model.SafeDereference(settings.LibreTranslate.URL) + "\x00" + model.SafeDereference(settings.LibreTranslate.APIKey)
	}
	if settings.Agents != nil {
		key += "\x00" + model.SafeDereference(settings.Agents.LLMServiceID)
	}

	s.provMu.Lock()
	defer s.provMu.Unlock()
	if s.prov != nil && s.provKey == key {
		return s.prov, nil
	}
	prov, err := newProvider(s.h, cfg)
	if err != nil {
		s.prov, s.provKey = nil, ""
		return nil, err
	}
	s.prov, s.provKey = prov, key
	return prov, nil
}

// providerName is the name recorded on a translation; the model requires
// one for the unavailable state even when no provider could be built.
func (s *Service) providerName() string {
	if name := model.SafeDereference(s.h.Config().AutoTranslationSettings.Provider); name != "" {
		return name
	}
	return "none"
}

func (s *Service) observeProviderCall(name string, err error, start time.Time) {
	m := s.h.Metrics()
	if m == nil {
		return
	}
	result := "success"
	if err != nil {
		result = "error"
		if errors.Is(err, context.DeadlineExceeded) {
			result = "timeout"
		}
	}
	m.ObserveAutoTranslateProviderCallDuration(name, result, time.Since(start).Seconds())
}

// Start launches the translation workers and watches the configuration.
func (s *Service) Start() error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()

	s.workers = workerCount(s.h.Config())
	s.pool.start(s.workers)

	if s.listenerID == "" {
		s.listenerID = s.h.AddConfigListener(s.onConfigChange)
	}
	s.h.Log().Debug("Auto-translation service started", mlog.Int("workers", s.workers))
	return nil
}

func (s *Service) onConfigChange(_, newCfg *model.Config) {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if n := workerCount(newCfg); n != s.workers && s.workers != 0 {
		s.workers = n
		go s.pool.resize(n)
	}
	// The provider is rebuilt lazily on its next use.
}

// Shutdown stops the workers, letting in-flight tasks finish.
func (s *Service) Shutdown() error {
	s.lifecycleMu.Lock()
	s.workers = 0
	s.lifecycleMu.Unlock()
	s.pool.stop()
	return nil
}

// Close removes the configuration listener and stops the workers.
func (s *Service) Close() error {
	s.lifecycleMu.Lock()
	if s.listenerID != "" {
		s.h.RemoveConfigListener(s.listenerID)
		s.listenerID = ""
	}
	s.lifecycleMu.Unlock()
	return s.Shutdown()
}

func validateID(where, id string) *model.AppError {
	if id == "" {
		return model.NewAppError(where, "ent.autotranslation.validate_id.empty", nil, "", http.StatusBadRequest)
	}
	if !model.IsValidId(id) {
		return model.NewAppError(where, "ent.autotranslation.validate_id.invalid", nil, "", http.StatusBadRequest)
	}
	return nil
}

// destinationLanguages returns the base languages the channel's members read
// that are allowed targets, and the user locales behind each of them.
func (s *Service) destinationLanguages(channelID string) ([]string, map[string][]string, error) {
	locales, err := s.h.Store().AutoTranslation().GetActiveDestinationLanguages(channelID, "", nil)
	if err != nil {
		return nil, nil, err
	}
	targets := targetLanguageSet(s.targetLanguages())
	byLang := make(map[string][]string)
	for _, loc := range locales {
		lang := normalizeLang(loc)
		if lang == "" || !targets[lang] {
			continue
		}
		byLang[lang] = append(byLang[lang], loc)
	}
	langs := make([]string, 0, len(byLang))
	for l := range byLang {
		langs = append(langs, l)
	}
	sort.Strings(langs)
	return langs, byLang, nil
}
