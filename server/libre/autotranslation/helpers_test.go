// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package autotranslation

import (
	"net/http"
	"sort"
	"sync"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/channels/jobs"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/channels/store/storetest/mocks"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

// fakeHost is an in-memory host for tests.
type fakeHost struct {
	mu     sync.Mutex
	cfg    *model.Config
	st     store.Store
	events []*model.WebSocketEvent
	client *http.Client
	logger mlog.LoggerIFace

	agentsAvailable bool
	agentsReply     func(serviceID string, req app.BridgeCompletionRequest) (string, error)
	agentsRequests  []app.BridgeCompletionRequest
}

func newTestConfig(providerName, url string) *model.Config {
	cfg := &model.Config{}
	cfg.SetDefaults()
	cfg.FeatureFlags.AutoTranslation = true
	cfg.AutoTranslationSettings.Enable = model.NewPointer(true)
	cfg.AutoTranslationSettings.Provider = model.NewPointer(providerName)
	cfg.AutoTranslationSettings.LibreTranslate.URL = model.NewPointer(url)
	cfg.AutoTranslationSettings.Agents.LLMServiceID = model.NewPointer("openai")
	cfg.AutoTranslationSettings.TargetLanguages = &[]string{"en", "es", "pt-BR"}
	cfg.AutoTranslationSettings.Workers = model.NewPointer(2)
	cfg.AutoTranslationSettings.TimeoutMs = model.NewPointer(2000)
	return cfg
}

func (h *fakeHost) Config() *model.Config {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.cfg
}

func (h *fakeHost) setConfig(cfg *model.Config) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cfg = cfg
}

func (h *fakeHost) Store() store.Store                    { return h.st }
func (h *fakeHost) Log() mlog.LoggerIFace                 { return h.logger }
func (h *fakeHost) Metrics() einterfaces.MetricsInterface { return nil }

func (h *fakeHost) Publish(event *model.WebSocketEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = append(h.events, event)
}

func (h *fakeHost) publishedEvents() []*model.WebSocketEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]*model.WebSocketEvent(nil), h.events...)
}

func (h *fakeHost) AddConfigListener(func(oldCfg, newCfg *model.Config)) string { return "listener" }
func (h *fakeHost) RemoveConfigListener(string)                                 {}

func (h *fakeHost) HTTPClient() *http.Client {
	if h.client != nil {
		return h.client
	}
	return http.DefaultClient
}

func (h *fakeHost) AgentsBridgeStatus() (bool, string) {
	if h.agentsAvailable {
		return true, ""
	}
	return false, "app.agents.bridge.not_available.plugin_not_active"
}

func (h *fakeHost) AgentsServiceCompletion(serviceID string, req app.BridgeCompletionRequest) (string, error) {
	h.mu.Lock()
	h.agentsRequests = append(h.agentsRequests, req)
	h.mu.Unlock()
	return h.agentsReply(serviceID, req)
}

func (h *fakeHost) JobServer() *jobs.JobServer { return nil }

// memATStore is an in-memory store.AutoTranslationStore mimicking the SQL
// store's upsert semantics.
type memATStore struct {
	mu           sync.Mutex
	rows         map[string]*model.Translation
	userLocales  map[string]string // userID -> locale (enabled members only)
	channelUsers map[string][]string
	saves        int
}

func newMemATStore() *memATStore {
	return &memATStore{
		rows:         map[string]*model.Translation{},
		userLocales:  map[string]string{},
		channelUsers: map[string][]string{},
	}
}

func rowKey(objectType, objectID, lang string) string {
	return objectType + "|" + objectID + "|" + lang
}

func (m *memATStore) addMember(channelID, userID, locale string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.userLocales[userID] = locale
	m.channelUsers[channelID] = append(m.channelUsers[channelID], userID)
}

func (m *memATStore) get(objectType, objectID, lang string) *model.Translation {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t := m.rows[rowKey(objectType, objectID, lang)]; t != nil {
		return t.Clone()
	}
	return nil
}

func (m *memATStore) put(t *model.Translation) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rows[rowKey(t.ObjectType, t.ObjectID, t.Lang)] = t.Clone()
}

func (m *memATStore) isMember(userID, channelID string) bool {
	for _, u := range m.channelUsers[channelID] {
		if u == userID {
			return true
		}
	}
	return false
}

func (m *memATStore) IsUserEnabled(userID, channelID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.isMember(userID, channelID), nil
}

func (m *memATStore) GetUserLanguage(userID, channelID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.isMember(userID, channelID) {
		return "", nil
	}
	return m.userLocales[userID], nil
}

func (m *memATStore) GetActiveDestinationLanguages(channelID, excludeUserID string, filterUserIDs []string) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	seen := map[string]bool{}
	var out []string
	for _, u := range m.channelUsers[channelID] {
		if u == excludeUserID {
			continue
		}
		if l := m.userLocales[u]; !seen[l] {
			seen[l] = true
			out = append(out, l)
		}
	}
	sort.Strings(out)
	return out, nil
}

func (m *memATStore) Get(objectType, objectID, dstLang string) (*model.Translation, error) {
	if t := m.get(objectType, objectID, dstLang); t != nil {
		t.ChannelID = ""
		return t, nil
	}
	return nil, store.NewErrNotFound("Translation", objectID)
}

func (m *memATStore) GetBatch(objectType string, objectIDs []string, dstLang string) (map[string]*model.Translation, error) {
	out := map[string]*model.Translation{}
	for _, id := range objectIDs {
		if t := m.get(objectType, id, dstLang); t != nil {
			out[id] = t
		}
	}
	return out, nil
}

func (m *memATStore) GetAllForObject(objectType, objectID string) ([]*model.Translation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*model.Translation
	for _, t := range m.rows {
		if t.ObjectType == objectType && t.ObjectID == objectID {
			out = append(out, t.Clone())
		}
	}
	return out, nil
}

func (m *memATStore) Save(t *model.Translation) error {
	if err := t.IsValid(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	key := rowKey(t.ObjectType, t.ObjectID, t.Lang)
	if existing := m.rows[key]; existing != nil && existing.NormHash == t.NormHash && existing.State == t.State {
		return nil
	}
	c := t.Clone()
	c.UpdateAt = model.GetMillis()
	m.rows[key] = c
	m.saves++
	return nil
}

func (m *memATStore) GetByStateOlderThan(state model.TranslationState, olderThanMillis int64, limit int) ([]*model.Translation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*model.Translation
	for _, t := range m.rows {
		if t.State == state && t.UpdateAt < olderThanMillis {
			c := t.Clone()
			c.ChannelID = ""
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdateAt < out[j].UpdateAt })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *memATStore) ClearCaches()                              {}
func (m *memATStore) InvalidateUserAutoTranslation(_, _ string) {}
func (m *memATStore) InvalidateUserLocaleCache(string)          {}
func (m *memATStore) GetLatestPostUpdateAtForChannel(string) (int64, error) {
	return 0, nil
}
func (m *memATStore) InvalidatePostTranslationEtag(string) {}
func (m *memATStore) GetTranslationsSinceForChannel(_, _ string, _ int64) (map[string]*model.Translation, error) {
	return map[string]*model.Translation{}, nil
}

var _ store.AutoTranslationStore = (*memATStore)(nil)

// testEnv bundles a service with its fakes.
type testEnv struct {
	h       *fakeHost
	st      *mocks.Store
	at      *memATStore
	chStore *mocks.ChannelStore
	pStore  *mocks.PostStore
	svc     *Service
}

func newTestEnv(t *testing.T, providerName, url string) *testEnv {
	t.Helper()
	st := &mocks.Store{}
	at := newMemATStore()
	chStore := &mocks.ChannelStore{}
	pStore := &mocks.PostStore{}
	st.On("AutoTranslation").Return(at)
	st.On("Channel").Return(chStore)
	st.On("Post").Return(pStore)

	h := &fakeHost{cfg: newTestConfig(providerName, url), st: st, logger: mlog.CreateConsoleTestLogger(t)}
	env := &testEnv{h: h, st: st, at: at, chStore: chStore, pStore: pStore, svc: New(h)}
	t.Cleanup(func() { _ = env.svc.Close() })
	return env
}
