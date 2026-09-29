// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package autotranslation

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// Meta keys stored with a translation.
const (
	metaSrcLang = "src_lang"
	metaError   = "error"
	metaPath    = "path"
)

// Translate records the translations channelID needs for the object and
// queues the work. See einterfaces.AutoTranslationInterface.
func (s *Service) Translate(ctx context.Context, objectType, objectID, channelID, userID string, content any) (*model.Translation, *model.AppError) {
	start := time.Now()
	defer func() {
		if m := s.h.Metrics(); m != nil {
			m.ObserveAutoTranslateTranslateDuration(objectType, time.Since(start).Seconds())
		}
	}()

	if appErr := s.notAvailableError("Translate"); appErr != nil {
		return nil, appErr
	}
	if objectType == "" {
		return nil, model.NewAppError("Translate", "ent.autotranslation.add_task.missing_object_type", nil, "", http.StatusBadRequest)
	}
	for _, id := range []string{objectID, channelID, userID} {
		if appErr := validateID("Translate", id); appErr != nil {
			return nil, appErr
		}
	}

	src, appErr := extractContent(content)
	if appErr != nil {
		return nil, appErr
	}

	langs, locales, err := s.destinationLanguages(channelID)
	if err != nil {
		return nil, storeError("Translate", err)
	}

	// Detect the source language locally when the text is long enough for
	// the result to be trusted; otherwise the provider detects it.
	detectStart := time.Now()
	srcLang, srcConf := detectLocal(src.Plain)
	if m := s.h.Metrics(); m != nil {
		m.ObserveAutoTranslateLinguaDetectionDuration(time.Since(detectStart).Seconds())
	}
	var srcConfPtr *float64
	if srcLang != "" {
		srcConfPtr = &srcConf
	}

	path := string(model.GetAutoTranslationPath(ctx))
	translations := make(map[string]*model.Translation, len(langs))
	var pending []string
	for _, lang := range langs {
		existing, err := s.h.Store().AutoTranslation().Get(objectType, objectID, lang)
		if err != nil {
			var nfErr *store.ErrNotFound
			if !errors.As(err, &nfErr) {
				return nil, storeError("Translate", err)
			}
			existing = nil
		}

		if existing != nil && existing.NormHash == src.NormHash && existing.State != "" {
			s.countNormHash("hit")
			existing.ChannelID = channelID
			translations[lang] = existing
			continue
		}
		s.countNormHash("miss")

		t := &model.Translation{
			ObjectID:   objectID,
			ObjectType: objectType,
			ChannelID:  channelID,
			Lang:       lang,
			Provider:   s.providerName(),
			Type:       src.Type,
			State:      model.TranslationStateProcessing,
			NormHash:   src.NormHash,
			Meta:       map[string]any{metaPath: path},
		}
		if srcLang != "" {
			t.Meta[metaSrcLang] = srcLang
			if srcLang == lang {
				t.State = model.TranslationStateSkipped
				t.Confidence = srcConfPtr
			}
		}
		if err := s.h.Store().AutoTranslation().Save(t); err != nil {
			return nil, model.NewAppError("Translate", "ent.autotranslation.create_translation_failed", nil, "", http.StatusInternalServerError).Wrap(err)
		}
		t.UpdateAt = model.GetMillis()
		translations[lang] = t
		if t.State == model.TranslationStateProcessing {
			pending = append(pending, lang)
		}
	}

	if len(pending) > 0 {
		err := s.pool.enqueue(&task{
			objectType: objectType,
			objectID:   objectID,
			channelID:  channelID,
			content:    src,
			langs:      pending,
			srcLang:    srcLang,
			srcConf:    srcConfPtr,
			locales:    locales,
		})
		if err != nil {
			// The rows stay processing; the recovery sweep retries them.
			s.h.Log().Warn("Failed to queue translation task",
				mlog.String("object_type", objectType),
				mlog.String("object_id", objectID),
				mlog.Err(err))
		}
	}

	// Posts are broadcast right after this call: attach every language's
	// state so the "posted"/"post_edited" event already carries them.
	if post, ok := content.(*model.Post); ok && post != nil {
		for lang, t := range translations {
			if post.Metadata == nil {
				post.Metadata = &model.PostMetadata{}
			}
			if post.Metadata.Translations == nil {
				post.Metadata.Translations = make(map[string]*model.PostTranslation, len(translations))
			}
			post.Metadata.Translations[lang] = t.ToPostTranslation()
		}
	}

	userLang, appErr := s.GetUserLanguage(userID, channelID)
	if appErr != nil {
		return nil, appErr
	}
	if userLang == "" {
		return nil, nil
	}
	if t, ok := translations[userLang]; ok {
		return t.Clone(), nil
	}
	return nil, nil
}

func (s *Service) countNormHash(result string) {
	if m := s.h.Metrics(); m != nil {
		m.IncrementAutoTranslateNormHash(result)
	}
}

// processTask translates one object into the task's languages.
func (s *Service) processTask(t *task) {
	start := time.Now()
	remaining := append([]string(nil), t.langs...)
	defer func() {
		if r := recover(); r != nil {
			s.h.Log().Error("Auto-translation worker panicked",
				mlog.String("object_id", t.objectID),
				mlog.Any("panic", r))
		}
		for _, l := range remaining {
			s.pool.done(t, l)
		}
		if m := s.h.Metrics(); m != nil {
			m.ObserveAutoTranslateWorkerTaskDuration(time.Since(start).Seconds())
		}
	}()

	prov, provErr := s.getProvider()
	if provErr == nil {
		if ok, reason := featureAvailable(s.h.Config()); !ok {
			provErr = fmt.Errorf("auto-translation is not available: %s", reason)
		}
	}

	srcLang, srcConf := t.srcLang, t.srcConf

	// LibreTranslate detects cheaply, which lets languages equal to the
	// source be skipped without a translation call.
	if provErr == nil && srcLang == "" && len(t.langs) > 0 {
		if _, ok := prov.(*libreTranslateProvider); ok {
			ctx, cancel := context.WithTimeout(context.Background(), providerTimeout(s.h.Config()))
			callStart := time.Now()
			lang, conf, err := prov.Detect(ctx, truncateBytes(t.content.Plain, maxDetectionBytes))
			cancel()
			s.observeProviderCall(prov.Name(), err, callStart)
			if err == nil && lang != "" && (conf == nil || *conf >= 0.5) {
				srcLang, srcConf = lang, conf
			}
		}
	}

	for len(remaining) > 0 {
		lang := remaining[0]
		srcLang, srcConf = s.translateOne(t, prov, provErr, lang, srcLang, srcConf)
		s.pool.done(t, lang)
		remaining = remaining[1:]
	}
}

// translateOne produces and stores the translation of t into lang. It
// returns the source language, which a provider call may have detected.
func (s *Service) translateOne(t *task, prov provider, provErr error, lang, srcLang string, srcConf *float64) (string, *float64) {
	ats := s.h.Store().AutoTranslation()

	current, err := ats.Get(t.objectType, t.objectID, lang)
	if err != nil {
		var nfErr *store.ErrNotFound
		if !errors.As(err, &nfErr) {
			s.h.Log().Warn("Failed to load translation", mlog.String("object_id", t.objectID), mlog.String("lang", lang), mlog.Err(err))
			return srcLang, srcConf
		}
		current = nil
	}
	// Content changed since the task was queued, or another worker already
	// finished this language.
	if current != nil && (current.NormHash != t.content.NormHash || current.State != model.TranslationStateProcessing) {
		return srcLang, srcConf
	}

	tr := &model.Translation{
		ObjectID:   t.objectID,
		ObjectType: t.objectType,
		ChannelID:  t.channelID,
		Lang:       lang,
		Provider:   s.providerName(),
		Type:       t.content.Type,
		NormHash:   t.content.NormHash,
		Meta:       map[string]any{},
	}
	if current != nil {
		for k, v := range current.Meta {
			if k != "type" && k != metaError {
				tr.Meta[k] = v
			}
		}
	}

	switch {
	case provErr != nil:
		tr.State = model.TranslationStateUnavailable
		tr.Meta[metaError] = provErr.Error()
	case srcLang != "" && srcLang == lang:
		tr.State = model.TranslationStateSkipped
		tr.Confidence = srcConf
	default:
		tr.Provider = prov.Name()
		ctx, cancel := context.WithTimeout(context.Background(), providerTimeout(s.h.Config()))
		callStart := time.Now()
		res, err := prov.Translate(ctx, t.content.segmentTexts(), srcLang, lang)
		cancel()
		s.observeProviderCall(prov.Name(), err, callStart)

		if err == nil && len(res.Texts) != len(t.content.Segments) {
			err = fmt.Errorf("provider returned %d segments, expected %d", len(res.Texts), len(t.content.Segments))
		}
		if err != nil {
			tr.State = model.TranslationStateUnavailable
			tr.Meta[metaError] = err.Error()
			s.h.Log().Debug("Translation failed",
				mlog.String("object_id", t.objectID), mlog.String("lang", lang), mlog.Err(err))
			break
		}

		if srcLang == "" && res.SourceLang != "" {
			srcLang, srcConf = res.SourceLang, res.Confidence
		}
		if srcLang != "" && srcLang == lang {
			tr.State = model.TranslationStateSkipped
			tr.Confidence = srcConf
			break
		}

		text, obj, err := t.content.build(res.Texts)
		if err != nil {
			tr.State = model.TranslationStateUnavailable
			tr.Meta[metaError] = err.Error()
			break
		}
		tr.State = model.TranslationStateReady
		tr.Text = text
		tr.ObjectJSON = obj
		tr.Confidence = srcConf
	}
	if srcLang != "" {
		tr.Meta[metaSrcLang] = srcLang
	}

	if err := ats.Save(tr); err != nil {
		s.h.Log().Warn("Failed to save translation",
			mlog.String("object_id", t.objectID), mlog.String("lang", lang), mlog.Err(err))
		return srcLang, srcConf
	}
	tr.UpdateAt = model.GetMillis()
	s.publish(t, tr)
	return srcLang, srcConf
}

// publish notifies the channel members that a post translation changed. The
// event carries the translation under its base language and under every user
// locale that maps to it, since clients look it up by their own locale.
func (s *Service) publish(t *task, tr *model.Translation) {
	if t.objectType != model.TranslationObjectTypePost || t.channelID == "" {
		return
	}

	entry := map[string]any{
		"state":            string(tr.State),
		"translation_type": string(tr.Type),
	}
	if src, ok := tr.Meta[metaSrcLang].(string); ok && src != "" {
		entry["src_lang"] = src
	}
	if tr.State == model.TranslationStateReady {
		if tr.Type == model.TranslationTypeObject {
			entry["translation"] = string(tr.ObjectJSON)
		} else {
			entry["translation"] = tr.Text
		}
	}

	keys := []string{tr.Lang}
	keys = append(keys, t.locales[tr.Lang]...)
	sort.Strings(keys)
	translations := make(map[string]any, len(keys))
	for _, k := range keys {
		translations[k] = entry
	}

	event := model.NewWebSocketEvent(model.WebsocketEventPostTranslationUpdated, "", t.channelID, "", nil, "")
	event.Add("object_id", t.objectID)
	event.Add("translations", translations)
	s.h.Publish(event)
}
