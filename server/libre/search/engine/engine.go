// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

// Package engine implements searchengine.SearchEngineInterface on top of an
// Elasticsearch or OpenSearch cluster.
package engine

import (
	"context"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/platform/services/searchengine"
)

var _ searchengine.SearchEngineInterface = (*Engine)(nil)

var versionRegex = regexp.MustCompile(`^(\d+)\.(\d+)`)

// Engine is the Elasticsearch / OpenSearch search engine.
type Engine struct {
	getLogger func() mlog.LoggerIFace
	readFile  clientFileReader

	cfg atomic.Pointer[model.Config]

	lifecycleMu sync.Mutex
	state       atomic.Pointer[runtimeState]
	healthy     atomic.Bool

	// Last known server information, kept after Stop for the support packet.
	infoMu       sync.RWMutex
	fullVersion  string
	majorVersion int
	plugins      []string

	knownIndexes sync.Map
	outdated     atomic.Pointer[map[string]bool]
}

type runtimeState struct {
	client    *client
	processor *bulkProcessor
}

// New returns a stopped engine. getLogger and readFile may be nil.
func New(cfg *model.Config, getLogger func() mlog.LoggerIFace, readFile func(name string) ([]byte, error)) *Engine {
	e := &Engine{getLogger: getLogger, readFile: readFile}
	if cfg != nil {
		e.cfg.Store(cfg)
	}
	return e
}

func (e *Engine) logger() mlog.LoggerIFace {
	if e.getLogger != nil {
		if l := e.getLogger(); l != nil {
			return l
		}
	}
	return fallbackLogger()
}

var fallbackLogger = sync.OnceValue(func() mlog.LoggerIFace {
	logger, err := mlog.NewLogger()
	if err != nil {
		return mlog.CreateConsoleLogger()
	}
	return logger
})

func (e *Engine) config() *model.Config {
	if cfg := e.cfg.Load(); cfg != nil {
		return cfg
	}
	cfg := &model.Config{}
	cfg.SetDefaults()
	return cfg
}

func (e *Engine) settings() *model.ElasticsearchSettings {
	return &e.config().ElasticsearchSettings
}

func backendName(s *model.ElasticsearchSettings) string {
	if model.SafeDereference(s.Backend) == backendOpenSearch {
		return backendOpenSearch
	}
	return backendElasticsearch
}

func backendDisplayName(backend string) string {
	if backend == backendOpenSearch {
		return "OpenSearch"
	}
	return "Elasticsearch"
}

func (e *Engine) backendParams() map[string]any {
	return map[string]any{"Backend": backendDisplayName(backendName(e.settings()))}
}

func namesFor(cfg *model.Config) indexNames {
	return indexNames{
		prefix:       model.SafeDereference(cfg.ElasticsearchSettings.IndexPrefix),
		globalPrefix: model.SafeDereference(cfg.ElasticsearchSettings.GlobalSearchPrefix),
	}
}

func (e *Engine) processor() *bulkProcessor {
	if st := e.state.Load(); st != nil {
		return st.processor
	}
	return nil
}

// activeClient returns the client of a started engine.
func (e *Engine) activeClient(where string) (*client, *model.Config, *model.AppError) {
	st := e.state.Load()
	if st == nil {
		return nil, nil, model.NewAppError(where, "ent.elasticsearch.not_started.error", e.backendParams(), "", http.StatusInternalServerError)
	}
	return st.client, e.config(), nil
}

func (e *Engine) requestContext() (context.Context, context.CancelFunc) {
	timeout := time.Duration(max(model.SafeDereference(e.settings().RequestTimeoutSeconds), 1)) * time.Second
	return context.WithTimeout(context.Background(), timeout)
}

func parseMajorVersion(version string) (int, bool) {
	m := versionRegex.FindStringSubmatch(version)
	if m == nil {
		return 0, false
	}
	major, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	return major, true
}

// connect creates a client and validates the server behind it.
func (e *Engine) connect(ctx context.Context, s *model.ElasticsearchSettings) (*client, *serverInfo, int, *model.AppError) {
	params := map[string]any{"Backend": backendDisplayName(backendName(s))}
	c, appErr := newClient(s, e.logger(), e.readFile)
	if appErr != nil {
		return nil, nil, 0, appErr
	}
	info, err := c.info(ctx)
	if err != nil {
		c.close()
		return nil, nil, 0, model.NewAppError("Engine.connect", "ent.elasticsearch.start.get_server_version.app_error", params, "", http.StatusInternalServerError).Wrap(err)
	}
	major, ok := parseMajorVersion(info.Version.Number)
	if !ok {
		c.close()
		return nil, nil, 0, model.NewAppError("Engine.connect", "ent.elasticsearch.start.parse_server_version.app_error", params, "version="+info.Version.Number, http.StatusInternalServerError)
	}

	isOpenSearch := info.Version.Distribution == "opensearch"
	switch {
	case c.backend == backendOpenSearch && !isOpenSearch:
		c.close()
		return nil, nil, 0, model.NewAppError("Engine.connect", "ent.elasticsearch.start.get_server_version.app_error", params, "the server is not an OpenSearch server, check the Backend setting", http.StatusBadRequest)
	case c.backend == backendElasticsearch && isOpenSearch:
		c.close()
		return nil, nil, 0, model.NewAppError("Engine.connect", "ent.elasticsearch.start.get_server_version.app_error", params, "the server is an OpenSearch server, check the Backend setting", http.StatusBadRequest)
	case c.backend == backendElasticsearch && major < 7:
		c.close()
		return nil, nil, 0, model.NewAppError("Engine.connect", "ent.elasticsearch.start.parse_server_version.app_error", params, "Elasticsearch 7.17 or later is required, found "+info.Version.Number, http.StatusBadRequest)
	}
	return c, info, major, nil
}

// Start connects to the cluster and installs the index templates.
func (e *Engine) Start(ctx context.Context) *model.AppError {
	e.lifecycleMu.Lock()
	defer e.lifecycleMu.Unlock()

	if e.state.Load() != nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	cfg := e.config()
	s := &cfg.ElasticsearchSettings
	if !model.SafeDereference(s.EnableIndexing) {
		return nil
	}

	c, info, major, appErr := e.connect(ctx, s)
	if appErr != nil {
		return appErr
	}

	plugins, err := c.plugins(ctx)
	if err != nil {
		e.logger().Warn("Failed to list the plugins of the search backend", mlog.Err(err))
	}

	names := namesFor(cfg)
	if appErr := e.putTemplates(ctx, c, names, s); appErr != nil {
		c.close()
		return appErr
	}
	e.forgetIndexes()
	for _, name := range []string{names.channels(), names.users(), names.files()} {
		if err := e.ensureIndex(ctx, c, name); err != nil {
			c.close()
			return model.NewAppError("Engine.Start", "ent.elasticsearch.create_client.connect_failed", e.backendParams(), "failed to create index "+name, http.StatusInternalServerError).Wrap(err)
		}
	}

	e.checkSchemas(ctx, c, names)

	e.infoMu.Lock()
	e.fullVersion = info.Version.Number
	e.majorVersion = major
	e.plugins = plugins
	e.infoMu.Unlock()

	st := &runtimeState{client: c, processor: newBulkProcessor(e, c, max(model.SafeDereference(s.LiveIndexingBatchSize), 1))}
	e.state.Store(st)
	e.healthy.Store(true)

	e.logger().Info("Search engine started",
		mlog.String("backend", c.backend),
		mlog.String("version", info.Version.Number),
		mlog.String("index_prefix", names.prefix))
	return nil
}

// Stop flushes pending writes and disconnects.
func (e *Engine) Stop() *model.AppError {
	e.lifecycleMu.Lock()
	defer e.lifecycleMu.Unlock()

	e.healthy.Store(false)
	st := e.state.Swap(nil)
	if st == nil {
		return nil
	}
	st.processor.stop()
	st.client.close()
	e.logger().Info("Search engine stopped")
	return nil
}

func (e *Engine) HealthCheck(rctx request.CTX) *model.AppError {
	c, _, appErr := e.activeClient("Engine.HealthCheck")
	if appErr != nil {
		return model.NewAppError("Engine.HealthCheck", "ent.elasticsearch.healthcheck.not_started.app_error", e.backendParams(), "", http.StatusInternalServerError)
	}
	ctx, cancel := e.requestContext()
	defer cancel()
	if _, err := c.info(ctx); err != nil {
		return model.NewAppError("Engine.HealthCheck", "ent.elasticsearch.healthcheck.unreachable.app_error", e.backendParams(), "", http.StatusInternalServerError).Wrap(err)
	}
	return nil
}

func (e *Engine) GetFullVersion() string {
	e.infoMu.RLock()
	defer e.infoMu.RUnlock()
	return e.fullVersion
}

func (e *Engine) GetVersion() int {
	e.infoMu.RLock()
	defer e.infoMu.RUnlock()
	return e.majorVersion
}

func (e *Engine) GetPlugins() []string {
	e.infoMu.RLock()
	defer e.infoMu.RUnlock()
	return append([]string{}, e.plugins...)
}

// UpdateConfig records the new configuration. Connection changes are handled
// by the platform watcher (Stop + Start); index layout changes are applied
// here.
func (e *Engine) UpdateConfig(cfg *model.Config) {
	if cfg == nil {
		return
	}
	old := e.cfg.Swap(cfg)

	st := e.state.Load()
	if st == nil || old == nil {
		return
	}
	oldS, newS := &old.ElasticsearchSettings, &cfg.ElasticsearchSettings

	st.processor.setBatchSize(max(model.SafeDereference(newS.LiveIndexingBatchSize), 1))

	// The platform restarts the engine when the URL or the credentials
	// change; the other client settings are handled here.
	if model.SafeDereference(oldS.Backend) != model.SafeDereference(newS.Backend) ||
		model.SafeDereference(oldS.CA) != model.SafeDereference(newS.CA) ||
		model.SafeDereference(oldS.ClientCert) != model.SafeDereference(newS.ClientCert) ||
		model.SafeDereference(oldS.ClientKey) != model.SafeDereference(newS.ClientKey) ||
		model.SafeDereference(oldS.SkipTLSVerification) != model.SafeDereference(newS.SkipTLSVerification) ||
		model.SafeDereference(oldS.RequestTimeoutSeconds) != model.SafeDereference(newS.RequestTimeoutSeconds) ||
		model.SafeDereference(oldS.Trace) != model.SafeDereference(newS.Trace) {
		go e.restart()
		return
	}

	if model.SafeDereference(oldS.IndexPrefix) != model.SafeDereference(newS.IndexPrefix) ||
		model.SafeDereference(oldS.EnableCJKAnalyzers) != model.SafeDereference(newS.EnableCJKAnalyzers) ||
		model.SafeDereference(oldS.PostIndexShards) != model.SafeDereference(newS.PostIndexShards) ||
		model.SafeDereference(oldS.PostIndexReplicas) != model.SafeDereference(newS.PostIndexReplicas) ||
		model.SafeDereference(oldS.ChannelIndexShards) != model.SafeDereference(newS.ChannelIndexShards) ||
		model.SafeDereference(oldS.ChannelIndexReplicas) != model.SafeDereference(newS.ChannelIndexReplicas) ||
		model.SafeDereference(oldS.UserIndexShards) != model.SafeDereference(newS.UserIndexShards) ||
		model.SafeDereference(oldS.UserIndexReplicas) != model.SafeDereference(newS.UserIndexReplicas) {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			names := namesFor(cfg)
			if appErr := e.putTemplates(ctx, st.client, names, newS); appErr != nil {
				e.logger().Error("Failed to update the search index templates", mlog.Err(appErr))
				return
			}
			e.forgetIndexes()
			for _, name := range []string{names.channels(), names.users(), names.files()} {
				if err := e.ensureIndex(ctx, st.client, name); err != nil {
					e.logger().Error("Failed to create search index", mlog.String("index", name), mlog.Err(err))
				}
			}
			e.checkSchemas(ctx, st.client, names)
		}()
	}
}

// restart reconnects with the current settings. If the new connection fails,
// the engine stays stopped and the platform watcher retries later.
func (e *Engine) restart() {
	e.logger().Info("Search engine client settings changed, reconnecting")
	_ = e.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if appErr := e.Start(ctx); appErr != nil {
		e.logger().Error("Failed to restart the search engine", mlog.Err(appErr))
	}
}

func (e *Engine) GetName() string {
	return backendName(e.settings())
}

func (e *Engine) IsEnabled() bool {
	return model.SafeDereference(e.settings().EnableIndexing)
}

func (e *Engine) IsActive() bool {
	return e.state.Load() != nil
}

func (e *Engine) IsHealthy() bool {
	return e.healthy.Load()
}

func (e *Engine) SetHealthy(healthy bool) {
	if healthy && !e.IsActive() {
		return
	}
	e.healthy.Store(healthy)
}

func (e *Engine) IsIndexingEnabled() bool {
	return e.IsActive() && model.SafeDereference(e.settings().EnableIndexing)
}

func (e *Engine) IsSearchEnabled() bool {
	s := e.settings()
	return e.IsActive() && model.SafeDereference(s.EnableIndexing) && model.SafeDereference(s.EnableSearching)
}

// IsAutocompletionEnabled reports whether channel and user autocompletion
// must be served by the engine: it must be enabled, and the channel and user
// indexes must have the current mappings.
func (e *Engine) IsAutocompletionEnabled() bool {
	s := e.settings()
	return e.IsActive() && model.SafeDereference(s.EnableIndexing) && model.SafeDereference(s.EnableAutocomplete) &&
		!e.isOutdated(indexChannels) && !e.isOutdated(indexUsers)
}

// IsIndexingSync reports whether writes must be performed synchronously by
// the search layer. Live writes are asynchronous: they are either sent right
// away from a goroutine of the search layer, or batched by the processor.
func (e *Engine) IsIndexingSync() bool {
	return false
}

// TestConfig checks that the given settings allow to reach a supported
// cluster, and that the index templates can be installed.
func (e *Engine) TestConfig(rctx request.CTX, cfg *model.Config) *model.AppError {
	if cfg == nil {
		cfg = e.config()
	}
	s := &cfg.ElasticsearchSettings
	if !model.SafeDereference(s.EnableIndexing) {
		return model.NewAppError("Engine.TestConfig", "ent.elasticsearch.test_config.indexing_disabled.error", map[string]any{"Backend": backendDisplayName(backendName(s))}, "", http.StatusNotImplemented)
	}

	timeout := time.Duration(max(model.SafeDereference(s.RequestTimeoutSeconds), 1)) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 2*timeout)
	defer cancel()

	c, _, _, appErr := e.connect(ctx, s)
	if appErr != nil {
		return appErr
	}
	defer c.close()

	return e.putTemplates(ctx, c, namesFor(cfg), s)
}

func (e *Engine) PurgeIndexes(rctx request.CTX) *model.AppError {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	return e.purge(ctx, allKinds)
}

func (e *Engine) PurgeIndexList(rctx request.CTX, indexes []string) *model.AppError {
	kinds, err := resolvePurgeKinds(namesFor(e.config()), indexes)
	if err != nil {
		return model.NewAppError("Engine.PurgeIndexList", "ent.elasticsearch.purge_indexes.unknown_index", nil, "", http.StatusBadRequest).Wrap(err)
	}
	if len(kinds) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	return e.purge(ctx, kinds)
}

func (e *Engine) RefreshIndexes(rctx request.CTX) *model.AppError {
	c, cfg, appErr := e.activeClient("Engine.RefreshIndexes")
	if appErr != nil {
		return appErr
	}
	if p := e.processor(); p != nil {
		p.flush()
	}
	names := namesFor(cfg)
	ctx, cancel := e.requestContext()
	defer cancel()
	target := names.postsPattern() + "," + names.channels() + "," + names.users() + "," + names.files()
	if _, err := c.do(ctx, http.MethodPost, "/"+target+"/_refresh", ignoreUnavailable(), nil); err != nil {
		return model.NewAppError("Engine.RefreshIndexes", "ent.elasticsearch.refresh_indexes.refresh_failed", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return nil
}
