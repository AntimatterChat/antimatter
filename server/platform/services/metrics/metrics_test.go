// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package metrics

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/channels/store/storetest/mocks"
)

func newTestMetrics(t *testing.T, opts Options) *MetricsInterfaceImpl {
	t.Helper()
	if opts.Logger == nil {
		opts.Logger = mlog.CreateConsoleTestLogger(t)
	}
	return New(opts)
}

func gatherFamilies(t *testing.T, m *MetricsInterfaceImpl) map[string]*dto.MetricFamily {
	t.Helper()
	families, err := m.Registry.Gather()
	require.NoError(t, err)
	res := make(map[string]*dto.MetricFamily, len(families))
	for _, f := range families {
		res[f.GetName()] = f
	}
	return res
}

func TestNewInstancesAreIndependent(t *testing.T) {
	m1 := newTestMetrics(t, Options{})
	m2 := newTestMetrics(t, Options{})

	m1.IncrementPostCreate()
	assert.Equal(t, 1.0, testutil.ToFloat64(m1.PostCreate))
	assert.Equal(t, 0.0, testutil.ToFloat64(m2.PostCreate))
}

// TestAllMethods calls every method of the interface, making sure none of them panics
// (e.g. because of a label count mismatch) and that the registry is still consistent.
func TestAllMethods(t *testing.T) {
	m := newTestMetrics(t, Options{})

	m.Register()
	m.RegisterDBCollector(nil, "nil")
	m.UnregisterDBCollector(nil, "nil")

	m.IncrementPostCreate()
	m.IncrementWebhookPost()
	m.IncrementPostSentEmail()
	m.IncrementPostSentPush()
	m.IncrementPostBroadcast()
	m.IncrementPostFileAttachment(3)
	m.IncrementPostFileAttachment(-1)

	m.IncrementHTTPRequest()
	m.IncrementHTTPError()

	m.IncrementClusterRequest()
	m.ObserveClusterRequestDuration(0.2)
	m.IncrementClusterEventType(model.ClusterEventPublish)
	m.ObserveClusterReliableFallbackLength(model.ClusterEventPublish, 12)

	m.IncrementLogin()
	m.IncrementLoginFail()

	m.IncrementEtagHitCounter("getPosts")
	m.IncrementEtagMissCounter("getPosts")
	m.IncrementMemCacheHitCounter("Status")
	m.IncrementMemCacheMissCounter("Status")
	m.IncrementMemCacheInvalidationCounter("Status")
	m.IncrementMemCacheMissCounterSession()
	m.IncrementMemCacheHitCounterSession()
	m.IncrementMemCacheInvalidationCounterSession()

	m.IncrementWebsocketEvent(model.WebsocketEventPosted)
	m.IncrementWebSocketBroadcast(model.WebsocketEventPosted)
	m.IncrementWebSocketBroadcastBufferSize("0", 2)
	m.DecrementWebSocketBroadcastBufferSize("0", 1)
	m.IncrementWebSocketBroadcastUsersRegistered("0", 3)
	m.DecrementWebSocketBroadcastUsersRegistered("0", 1)
	m.IncrementWebsocketReconnectEventWithDisconnectErrCode("found", "1001")

	m.IncrementHTTPWebSockets("web")
	m.IncrementHTTPWebSockets("web")
	m.DecrementHTTPWebSockets("web")

	m.AddMemCacheHitCounter("Status", 4)
	m.AddMemCacheMissCounter("Status", 5)

	m.IncrementPostsSearchCounter()
	m.ObservePostsSearchDuration(0.1)
	m.IncrementFilesSearchCounter()
	m.ObserveFilesSearchDuration(0.1)
	m.ObserveStoreMethodDuration("PostStore.Get", "true", 0.01)
	m.ObserveAPIEndpointDuration("getPost", "GET", "200", "web", "page_load", 0.05)
	m.ObserveRedisEndpointDuration("Status", "Get", 0.001)
	m.IncrementPostIndexCounter()
	m.IncrementFileIndexCounter()
	m.IncrementUserIndexCounter()
	m.IncrementChannelIndexCounter()

	m.ObservePluginHookDuration("plugin", "OnActivate", true, 0.1)
	m.ObservePluginMultiHookIterationDuration("plugin", 0.1)
	m.ObservePluginMultiHookDuration(0.1)
	m.ObservePluginAPIDuration("plugin", "GetUser", false, 0.1)

	m.ObserveEnabledUsers(42)
	require.NotNil(t, m.GetLoggerMetricsCollector())

	m.IncrementRemoteClusterMsgSentCounter("remote")
	m.IncrementRemoteClusterMsgReceivedCounter("remote")
	m.IncrementRemoteClusterMsgErrorsCounter("remote", true)
	m.ObserveRemoteClusterPingDuration("remote", 0.1)
	m.ObserveRemoteClusterClockSkew("remote", -0.2)
	m.IncrementRemoteClusterConnStateChangeCounter("remote", false)

	m.IncrementSharedChannelsSyncCounter("remote")
	m.ObserveSharedChannelsTaskInQueueDuration(0.1)
	m.ObserveSharedChannelsQueueSize(7)
	m.ObserveSharedChannelsSyncCollectionDuration("remote", 0.1)
	m.ObserveSharedChannelsSyncSendDuration("remote", 0.1)
	m.ObserveSharedChannelsSyncCollectionStepDuration("remote", "Users", 0.1)
	m.ObserveSharedChannelsSyncSendStepDuration("remote", "Users", 0.1)

	m.IncrementJobActive("migrations")
	m.IncrementJobActive("migrations")
	m.DecrementJobActive("migrations")

	m.SetReplicaLagAbsolute("replica-1", 10)
	m.SetReplicaLagTime("replica-1", 2)

	m.IncrementNotificationCounter(model.NotificationTypePush, "ios")
	m.IncrementNotificationAckCounter(model.NotificationTypePush, "ios")
	m.IncrementNotificationSuccessCounter(model.NotificationTypePush, "ios")
	m.IncrementNotificationErrorCounter(model.NotificationTypePush, model.NotificationReasonFetchError, "ios")
	m.IncrementNotificationNotSentCounter(model.NotificationTypePush, model.NotificationReasonUserStatus, "ios")
	m.IncrementNotificationUnsupportedCounter(model.NotificationTypePush, model.NotificationReasonPushProxyError, "ios")

	userID := model.NewId()
	m.ObserveClientTimeToFirstByte("linux", "chrome", userID, 0.1)
	m.ObserveClientTimeToLastByte("linux", "chrome", userID, 0.1)
	m.ObserveClientTimeToDomInteractive("linux", "chrome", userID, 0.1)
	m.ObserveClientSplashScreenEnd("linux", "chrome", "root", userID, 0.1)
	m.ObserveClientFirstContentfulPaint("linux", "chrome", userID, 0.1)
	m.ObserveClientLargestContentfulPaint("linux", "chrome", "post", userID, 0.1)
	m.ObserveClientInteractionToNextPaint("linux", "chrome", "keyboard", userID, 0.1)
	m.ObserveClientCumulativeLayoutShift("linux", "chrome", userID, 0.1)
	m.IncrementClientLongTasks("linux", "chrome", userID, 2)
	m.ObserveClientPageLoadDuration("linux", "chrome", userID, 0.1)
	m.ObserveClientChannelSwitchDuration("linux", "chrome", "true", userID, 0.1)
	m.ObserveClientTeamSwitchDuration("linux", "chrome", "", userID, 0.1)
	m.ObserveClientRHSLoadDuration("linux", "chrome", userID, 0.1)
	m.ObserveGlobalThreadsLoadDuration("linux", "chrome", userID, 0.1)
	m.ObserveMobileClientLoadDuration("ios", 1)
	m.ObserveMobileClientChannelSwitchDuration("ios", 1)
	m.ObserveMobileClientTeamSwitchDuration("ios", 1)
	m.ObserveMobileClientNetworkRequestsAverageSpeed("ios", "rn", "Login", 1)
	m.ObserveMobileClientNetworkRequestsEffectiveLatency("ios", "rn", "Login", 1)
	m.ObserveMobileClientNetworkRequestsElapsedTime("ios", "rn", "Login", 1)
	m.ObserveMobileClientNetworkRequestsLatency("ios", "rn", "Login", 1)
	m.ObserveMobileClientNetworkRequestsTotalCompressedSize("ios", "rn", "Login", 1)
	m.ObserveMobileClientNetworkRequestsTotalParallelRequests("ios", "rn", "Login", 1)
	m.ObserveMobileClientNetworkRequestsTotalRequests("ios", "rn", "Login", 1)
	m.ObserveMobileClientNetworkRequestsTotalSequentialRequests("ios", "rn", "Login", 1)
	m.ObserveMobileClientNetworkRequestsTotalSize("ios", "rn", "Login", 1)
	m.ClearMobileClientSessionMetadata()
	m.ObserveMobileClientSessionMetadata("2.20.0", "ios", 5, "false")
	m.ObserveDesktopCpuUsage("linux", "5.10.0", "main", 12)
	m.ObserveDesktopMemoryUsage("linux", "5.10.0", "main", 300)
	m.ObservePluginWebappPerf("linux", "chrome", "com.example.plugin", "render", 0.2)

	m.ObserveAccessControlSearchQueryDuration(0.1)
	m.ObserveAccessControlExpressionCompileDuration(0.1)
	m.ObserveAccessControlEvaluateDuration(0.1)
	m.IncrementAccessControlCacheInvalidation()

	m.ObserveAutoTranslateTranslateDuration("post", 0.1)
	m.ObserveAutoTranslateLinguaDetectionDuration(0.1)
	m.ObserveAutoTranslateProviderCallDuration("libretranslate", "success", 0.1)
	m.SetAutoTranslateQueueDepth(3)
	m.ObserveAutoTranslateWorkerTaskDuration(0.1)
	m.AddAutoTranslateRecoveryStuckFound(2)
	m.IncrementAutoTranslateNormHash("hit")

	assert.Equal(t, 1.0, testutil.ToFloat64(m.PostCreate))
	assert.Equal(t, 3.0, testutil.ToFloat64(m.PostFileAttachment))
	assert.Equal(t, 1.0, testutil.ToFloat64(m.HTTPWebsockets.WithLabelValues("web")))
	assert.Equal(t, 5.0, testutil.ToFloat64(m.MemCacheHit.WithLabelValues("Status")))
	assert.Equal(t, 6.0, testutil.ToFloat64(m.MemCacheMiss.WithLabelValues("Status")))
	assert.Equal(t, 1.0, testutil.ToFloat64(m.WebsocketBroadcastBufferSize.WithLabelValues("0")))
	assert.Equal(t, 2.0, testutil.ToFloat64(m.WebsocketBroadcastUsersRegister.WithLabelValues("0")))
	assert.Equal(t, 1.0, testutil.ToFloat64(m.JobsActive.WithLabelValues("migrations")))
	assert.Equal(t, 42.0, testutil.ToFloat64(m.EnabledUsers))
	assert.Equal(t, 1.0, testutil.ToFloat64(m.NotificationTotalCounters.WithLabelValues(string(model.NotificationTypePush), "ios")))

	families := gatherFamilies(t, m)
	for _, name := range []string{
		"mattermost_post_total",
		"mattermost_http_requests_total",
		"mattermost_api_time",
		"mattermost_db_store_time",
		"mattermost_db_replica_lag_abs",
		"mattermost_db_replica_lag_time",
		"mattermost_plugin_hook_time",
		"mattermost_notifications_total",
		"mattermost_webapp_page_load",
		"mattermost_mobileapp_mobile_load",
		"mattermost_desktopapp_cpu_usage",
		"mattermost_autotranslate_norm_hash_total",
		"go_goroutines",
	} {
		assert.Contains(t, families, name)
	}

	// The user ID must never end up in a label.
	for _, f := range families {
		for _, metric := range f.GetMetric() {
			for _, l := range metric.GetLabel() {
				assert.NotEqual(t, userID, l.GetValue(), "user ID found in label %s of %s", l.GetName(), f.GetName())
			}
		}
	}
}

func TestRegisterExposesHandler(t *testing.T) {
	var (
		mut      sync.Mutex
		handlers = map[string]http.Handler{}
	)
	m := newTestMetrics(t, Options{
		HandleMetrics: func(route string, h http.Handler) {
			mut.Lock()
			defer mut.Unlock()
			handlers[route] = h
		},
	})

	// Register is called each time the metrics server restarts.
	m.Register()
	m.Register()

	h := handlers["/metrics"]
	require.NotNil(t, h)

	m.IncrementPostCreate()
	m.IncrementLogin()

	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	text := string(body)
	assert.Contains(t, text, "mattermost_post_total 1")
	assert.Contains(t, text, "mattermost_login_logins_total 1")
	assert.Contains(t, text, "go_goroutines")
	assert.Contains(t, text, "process_start_time_seconds")
}

// fakeDriver is a database/sql driver that never connects; it's enough to get *sql.DB stats.
type fakeDriver struct{}

func (fakeDriver) Open(string) (driver.Conn, error) { return nil, errors.New("not implemented") }

var registerFakeDriver sync.Once

func openFakeDB(t *testing.T) *sql.DB {
	t.Helper()
	registerFakeDriver.Do(func() { sql.Register("libre-metrics-fake", fakeDriver{}) })
	db, err := sql.Open("libre-metrics-fake", "")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func dbNames(t *testing.T, m *MetricsInterfaceImpl) []string {
	t.Helper()
	f, ok := gatherFamilies(t, m)["go_sql_max_open_connections"]
	if !ok {
		return nil
	}
	var names []string
	for _, metric := range f.GetMetric() {
		for _, l := range metric.GetLabel() {
			if l.GetName() == "db_name" {
				names = append(names, l.GetValue())
			}
		}
	}
	return names
}

func TestDBCollectors(t *testing.T) {
	m := newTestMetrics(t, Options{})

	master := openFakeDB(t)
	replica := openFakeDB(t)
	replica2 := openFakeDB(t)

	m.RegisterDBCollector(master, "master")
	m.RegisterDBCollector(master, "master") // idempotent
	m.RegisterDBCollector(replica, "replica-0")
	assert.ElementsMatch(t, []string{"master", "replica-0"}, dbNames(t, m))

	// Unregistering with another handle is a no-op.
	m.UnregisterDBCollector(replica2, "replica-0")
	assert.ElementsMatch(t, []string{"master", "replica-0"}, dbNames(t, m))

	// Reconnection: unregister then register the new handle under the same name.
	m.UnregisterDBCollector(replica, "replica-0")
	assert.ElementsMatch(t, []string{"master"}, dbNames(t, m))
	m.RegisterDBCollector(replica2, "replica-0")
	assert.ElementsMatch(t, []string{"master", "replica-0"}, dbNames(t, m))

	// Registering a new handle under an existing name replaces the old one.
	m.RegisterDBCollector(replica, "replica-0")
	assert.ElementsMatch(t, []string{"master", "replica-0"}, dbNames(t, m))
}

func TestLoggerMetricsCollector(t *testing.T) {
	m := newTestMetrics(t, Options{})
	c := m.GetLoggerMetricsCollector()

	g, err := c.QueueSizeGauge("console")
	require.NoError(t, err)
	g.Set(4)
	g.Add(2)
	g.Sub(1)

	logged, err := c.LoggedCounter("console")
	require.NoError(t, err)
	logged.Inc()
	logged.Add(2)

	for _, fn := range []func(string) (mlog.Counter, error){c.ErrorCounter, c.DroppedCounter, c.BlockedCounter} {
		counter, cErr := fn("console")
		require.NoError(t, cErr)
		counter.Inc()
	}

	lc := m.loggerCollector
	assert.Equal(t, 5.0, testutil.ToFloat64(lc.queueSize.WithLabelValues("console")))
	assert.Equal(t, 3.0, testutil.ToFloat64(lc.logged.WithLabelValues("console")))
	assert.Equal(t, 1.0, testutil.ToFloat64(lc.errors.WithLabelValues("console")))
	assert.Equal(t, 1.0, testutil.ToFloat64(lc.dropped.WithLabelValues("console")))
	assert.Equal(t, 1.0, testutil.ToFloat64(lc.blocked.WithLabelValues("console")))

	// It must work as an actual logr collector.
	logger, err := mlog.NewLogger()
	require.NoError(t, err)
	logger.SetMetricsCollector(c, 100)
	require.NoError(t, logger.Shutdown())
}

func TestClientLabelsAreBounded(t *testing.T) {
	m := newTestMetrics(t, Options{})

	t.Run("network request group is validated", func(t *testing.T) {
		m.ObserveMobileClientNetworkRequestsLatency("ios", "rn", "Login", 1)
		m.ObserveMobileClientNetworkRequestsLatency("ios", "rn", "cold start", 1)
		m.ObserveMobileClientNetworkRequestsLatency("ios", "rn", "something random", 1)
		assert.Equal(t, 3, testutil.CollectAndCount(m.MobileClientNetworkLatency))

		groups := map[string]bool{}
		f := gatherFamilies(t, m)["mattermost_mobileapp_mobile_network_requests_latency"]
		for _, metric := range f.GetMetric() {
			for _, l := range metric.GetLabel() {
				if l.GetName() == "network_request_group" {
					groups[l.GetValue()] = true
				}
			}
		}
		assert.Equal(t, map[string]bool{"Login": true, "Cold Start": true, "other": true}, groups)
	})

	t.Run("plugin ids are capped", func(t *testing.T) {
		for i := range maxClientPluginIDs + 50 {
			m.ObservePluginWebappPerf("linux", "chrome", model.NewId()+string(rune('a'+i%26)), "render", 0.1)
		}
		assert.Equal(t, maxClientPluginIDs+1, testutil.CollectAndCount(m.ClientPluginWebappPerf))
	})

	t.Run("long values are truncated", func(t *testing.T) {
		m.ObserveDesktopCpuUsage("linux", strings.Repeat("9", 500), "main", 1)
		f := gatherFamilies(t, m)["mattermost_desktopapp_cpu_usage"]
		for _, metric := range f.GetMetric() {
			for _, l := range metric.GetLabel() {
				assert.LessOrEqual(t, len(l.GetValue()), maxLabelValueLength)
			}
		}
	})

	t.Run("mobile session metadata is reset", func(t *testing.T) {
		m.ClearMobileClientSessionMetadata()
		m.ObserveMobileClientSessionMetadata("2.0.0", "ios", 3, "false")
		m.ObserveMobileClientSessionMetadata("2.1.0", "android", 4, "true")
		assert.Equal(t, 2, testutil.CollectAndCount(m.MobileClientSessionMetadata))
		assert.Equal(t, 3.0, testutil.ToFloat64(m.MobileClientSessionMetadata.WithLabelValues("2.0.0", "ios", "false")))

		m.ClearMobileClientSessionMetadata()
		m.ObserveMobileClientSessionMetadata("2.1.0", "android", 6, "true")
		assert.Equal(t, 1, testutil.CollectAndCount(m.MobileClientSessionMetadata))
		assert.Equal(t, 6.0, testutil.ToFloat64(m.MobileClientSessionMetadata.WithLabelValues("2.1.0", "android", "true")))
	})
}

func TestBoundedLabel(t *testing.T) {
	b := newBoundedLabel(2)
	assert.Equal(t, "a", b.value("a"))
	assert.Equal(t, "b", b.value(" b "))
	assert.Equal(t, otherLabelValue, b.value("c"))
	assert.Equal(t, "a", b.value("a"))
	assert.Equal(t, "", b.value(""))
	b.reset()
	assert.Equal(t, "c", b.value("c"))

	assert.Equal(t, "ok", sanitizeLabelValue("ok\xff"))
}

func TestSanitizeLabelValueKeepsRunesWhole(t *testing.T) {
	v := sanitizeLabelValue(strings.Repeat("x", maxLabelValueLength-1) + "éé")
	assert.Equal(t, strings.Repeat("x", maxLabelValueLength-1), v)
}

func TestReplicaLag(t *testing.T) {
	cfg := &model.Config{}
	cfg.SetDefaults()
	cfg.SqlSettings.ReplicaLagSettings = []*model.ReplicaLagSettings{{
		DataSource:       new("postgres://replica"),
		QueryAbsoluteLag: new("select 1"),
		QueryTimeLag:     new("select 1"),
	}}
	cfg.SqlSettings.ReplicaMonitorIntervalSeconds = new(3600)

	var m *MetricsInterfaceImpl
	st := &mocks.Store{}
	st.On("ReplicaLagAbs").Return(func() error {
		m.SetReplicaLagAbsolute("replica-1", 1234)
		return nil
	}).Once()
	st.On("ReplicaLagTime").Return(func() error {
		m.SetReplicaLagTime("replica-1", 2.5)
		return errors.New("boom") // errors are only logged
	}).Once()

	m = newTestMetrics(t, Options{
		Config: func() *model.Config { return cfg },
		Store:  func() store.Store { return st },
	})
	done := make(chan struct{}, 1)
	m.replicaLag.done = done

	// The first scrape triggers a measurement in the background.
	gatherFamilies(t, m)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("replica lag was not refreshed")
	}

	assert.Equal(t, 1234.0, testutil.ToFloat64(m.ReplicaLagAbsolute.WithLabelValues("replica-1")))
	assert.Equal(t, 2.5, testutil.ToFloat64(m.ReplicaLagTime.WithLabelValues("replica-1")))

	families := gatherFamilies(t, m)
	require.Contains(t, families, "mattermost_db_replica_lag_abs")
	assert.Equal(t, 1234.0, families["mattermost_db_replica_lag_abs"].GetMetric()[0].GetGauge().GetValue())

	// Within the refresh interval, no new measurement happens (the mock would fail otherwise).
	gatherFamilies(t, m)
	st.AssertExpectations(t)
}

func TestReplicaLagDisabled(t *testing.T) {
	cfg := &model.Config{}
	cfg.SetDefaults()

	st := &mocks.Store{} // any call would fail the test
	m := newTestMetrics(t, Options{
		Config: func() *model.Config { return cfg },
		Store:  func() store.Store { return st },
	})
	gatherFamilies(t, m)
	st.AssertExpectations(t)
}

func TestHandlerIsServedWithContext(t *testing.T) {
	m := newTestMetrics(t, Options{})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "promhttp_metric_handler_requests_total")
}

var _ prometheus.Collector = (*replicaLagCollector)(nil)
