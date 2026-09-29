// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

// Package metrics implements einterfaces.MetricsInterface on top of the Prometheus client
// library. Every server instance owns its own registry which is exposed on the metrics
// server (MetricsSettings.ListenAddress) under /metrics.
package metrics

import (
	"database/sql"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/v8/channels/app/platform"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

const (
	namespace = "mattermost"

	subsystemAPI            = "api"
	subsystemAccessControl  = "access_control"
	subsystemAutoTranslate  = "autotranslate"
	subsystemCache          = "cache"
	subsystemCluster        = "cluster"
	subsystemDB             = "db"
	subsystemDesktop        = "desktopapp"
	subsystemHTTP           = "http"
	subsystemJobs           = "jobs"
	subsystemLogging        = "logging"
	subsystemLogin          = "login"
	subsystemMobile         = "mobileapp"
	subsystemNotifications  = "notifications"
	subsystemPlugin         = "plugin"
	subsystemPost           = "post"
	subsystemRedis          = "redis"
	subsystemRemoteCluster  = "remote_cluster"
	subsystemSearch         = "search"
	subsystemServer         = "server"
	subsystemSharedChannels = "shared_channels"
	subsystemWebapp         = "webapp"
	subsystemWebsocket      = "websocket"

	// Cardinality budgets for labels whose values are provided by clients.
	maxClientPluginIDs     = 100
	maxClientPluginMetrics = 200
	maxDesktopVersions     = 50
	maxDesktopProcesses    = 20
	maxMobileVersions      = 100
	maxMobilePlatforms     = 10
	maxNotificationPlatf   = 20
)

var (
	// Buckets for short server-side operations, in seconds.
	serverDurationBuckets = []float64{0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30}
	// Buckets for client side durations (page loads, channel switches ...), in seconds.
	clientDurationBuckets = []float64{0.05, 0.1, 0.25, 0.5, 1, 1.5, 2, 3, 5, 7.5, 10, 15, 20, 30, 60}
	// Buckets for fast client side interactions (INP, TTFB ...), in seconds.
	clientFastBuckets = []float64{0.01, 0.025, 0.05, 0.1, 0.2, 0.3, 0.5, 0.75, 1, 2, 4, 8}
	// Buckets for the cumulative layout shift score (unitless).
	clsBuckets = []float64{0.001, 0.01, 0.025, 0.05, 0.1, 0.15, 0.25, 0.5, 1, 2}
	// Buckets for mobile network timings as reported by the mobile app (milliseconds).
	mobileNetworkTimeBuckets = prometheus.ExponentialBuckets(10, 2, 14) // 10ms .. ~82s
	// Buckets for mobile network sizes as reported by the mobile app (bytes).
	mobileNetworkSizeBuckets = prometheus.ExponentialBuckets(1024, 4, 10) // 1KiB .. 256MiB
	// Buckets for mobile network transfer speeds as reported by the mobile app.
	mobileNetworkSpeedBuckets = prometheus.ExponentialBuckets(1024, 4, 10)
	// Buckets for mobile request counts.
	mobileNetworkCountBuckets = []float64{1, 2, 5, 10, 20, 50, 100, 200, 500, 1000}
	// Buckets for desktop app CPU usage (percent).
	desktopCPUBuckets = []float64{1, 5, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 150, 200}
	// Buckets for desktop app memory usage.
	desktopMemoryBuckets = prometheus.ExponentialBuckets(16, 2, 10)
	// Buckets for lengths/sizes of internal queues.
	queueLengthBuckets = prometheus.ExponentialBuckets(1, 2, 14)
)

func init() {
	platform.RegisterMetricsInterface(func(ps *platform.PlatformService, _, _ string) einterfaces.MetricsInterface {
		return New(Options{
			Logger:        ps.Log(),
			HandleMetrics: ps.HandleMetrics,
			Config:        ps.Config,
			Store:         func() store.Store { return ps.Store },
		})
	})
}

// Options configures a MetricsInterfaceImpl. Every field is optional.
type Options struct {
	// Logger is used to report problems collecting metrics.
	Logger mlog.LoggerIFace
	// HandleMetrics mounts an http.Handler on the metrics server router. It is called with
	// the /metrics route from Register.
	HandleMetrics func(route string, h http.Handler)
	// Config returns the current server configuration.
	Config func() *model.Config
	// Store returns the current store, used to poll replica lag.
	Store func() store.Store
}

type dbCollector struct {
	db        *sql.DB
	collector prometheus.Collector
}

// MetricsInterfaceImpl implements einterfaces.MetricsInterface with Prometheus collectors.
type MetricsInterfaceImpl struct {
	opts     Options
	logger   mlog.LoggerIFace
	Registry *prometheus.Registry
	handler  http.Handler

	dbCollectorsMut sync.Mutex
	dbCollectors    map[string]dbCollector

	loggerCollector *loggerMetricsCollector

	// Labels whose values come from clients.
	desktopVersionLabel    *boundedLabel
	desktopProcessLabel    *boundedLabel
	pluginIDLabel          *boundedLabel
	pluginMetricLabel      *boundedLabel
	mobileVersionLabel     *boundedLabel
	mobilePlatformLabel    *boundedLabel
	notificationPlatformLb *boundedLabel

	// Posts
	PostCreate         prometheus.Counter
	WebhookPost        prometheus.Counter
	PostSentEmail      prometheus.Counter
	PostSentPush       prometheus.Counter
	PostBroadcast      prometheus.Counter
	PostFileAttachment prometheus.Counter

	// HTTP
	HTTPRequest    prometheus.Counter
	HTTPError      prometheus.Counter
	HTTPWebsockets *prometheus.GaugeVec

	// Cluster
	ClusterRequest                prometheus.Counter
	ClusterRequestDuration        prometheus.Histogram
	ClusterEventType              *prometheus.CounterVec
	ClusterReliableFallbackLength *prometheus.HistogramVec

	// Login
	Login     prometheus.Counter
	LoginFail prometheus.Counter

	// Caches
	EtagHit                   *prometheus.CounterVec
	EtagMiss                  *prometheus.CounterVec
	MemCacheHit               *prometheus.CounterVec
	MemCacheMiss              *prometheus.CounterVec
	MemCacheInvalidation      *prometheus.CounterVec
	MemCacheHitSession        prometheus.Counter
	MemCacheMissSession       prometheus.Counter
	MemCacheInvalidationSess  prometheus.Counter
	RedisEndpointDuration     *prometheus.HistogramVec
	AccessControlCacheInvalid prometheus.Counter

	// Websockets
	WebsocketEvent                  *prometheus.CounterVec
	WebsocketBroadcast              *prometheus.CounterVec
	WebsocketBroadcastBufferSize    *prometheus.GaugeVec
	WebsocketBroadcastUsersRegister *prometheus.GaugeVec
	WebsocketReconnect              *prometheus.CounterVec

	// Search
	PostsSearch         prometheus.Counter
	PostsSearchDuration prometheus.Histogram
	FilesSearch         prometheus.Counter
	FilesSearchDuration prometheus.Histogram
	PostIndex           prometheus.Counter
	FileIndex           prometheus.Counter
	UserIndex           prometheus.Counter
	ChannelIndex        prometheus.Counter

	// Store / DB
	StoreMethodDuration *prometheus.HistogramVec
	ReplicaLagAbsolute  *prometheus.GaugeVec
	ReplicaLagTime      *prometheus.GaugeVec

	// API
	APIEndpointDuration *prometheus.HistogramVec

	// Plugins
	PluginHookDuration               *prometheus.HistogramVec
	PluginMultiHookIterationDuration *prometheus.HistogramVec
	PluginMultiHookDuration          prometheus.Histogram
	PluginAPIDuration                *prometheus.HistogramVec

	// Server
	EnabledUsers prometheus.Gauge

	// Remote clusters
	RemoteClusterMsgSent         *prometheus.CounterVec
	RemoteClusterMsgReceived     *prometheus.CounterVec
	RemoteClusterMsgErrors       *prometheus.CounterVec
	RemoteClusterPingDuration    *prometheus.HistogramVec
	RemoteClusterClockSkew       *prometheus.GaugeVec
	RemoteClusterConnStateChange *prometheus.CounterVec

	// Shared channels
	SharedChannelsSync                   *prometheus.CounterVec
	SharedChannelsTaskInQueueDuration    prometheus.Histogram
	SharedChannelsQueueSize              prometheus.Gauge
	SharedChannelsSyncCollectionDuration *prometheus.HistogramVec
	SharedChannelsSyncSendDuration       *prometheus.HistogramVec
	SharedChannelsSyncCollectionStep     *prometheus.HistogramVec
	SharedChannelsSyncSendStep           *prometheus.HistogramVec

	// Jobs
	JobsActive *prometheus.GaugeVec

	// Notifications
	NotificationTotalCounters       *prometheus.CounterVec
	NotificationAckCounters         *prometheus.CounterVec
	NotificationSuccessCounters     *prometheus.CounterVec
	NotificationErrorCounters       *prometheus.CounterVec
	NotificationNotSentCounters     *prometheus.CounterVec
	NotificationUnsupportedCounters *prometheus.CounterVec

	// Web app client metrics
	ClientTimeToFirstByte        *prometheus.HistogramVec
	ClientTimeToLastByte         *prometheus.HistogramVec
	ClientTimeToDomInteractive   *prometheus.HistogramVec
	ClientSplashScreenEnd        *prometheus.HistogramVec
	ClientFirstContentfulPaint   *prometheus.HistogramVec
	ClientLargestContentfulPaint *prometheus.HistogramVec
	ClientInteractionToNextPaint *prometheus.HistogramVec
	ClientCumulativeLayoutShift  *prometheus.HistogramVec
	ClientLongTasks              *prometheus.CounterVec
	ClientPageLoadDuration       *prometheus.HistogramVec
	ClientChannelSwitchDuration  *prometheus.HistogramVec
	ClientTeamSwitchDuration     *prometheus.HistogramVec
	ClientRHSLoadDuration        *prometheus.HistogramVec
	ClientGlobalThreadsLoad      *prometheus.HistogramVec
	ClientPluginWebappPerf       *prometheus.HistogramVec

	// Mobile app client metrics
	MobileClientLoadDuration               *prometheus.HistogramVec
	MobileClientChannelSwitchDuration      *prometheus.HistogramVec
	MobileClientTeamSwitchDuration         *prometheus.HistogramVec
	MobileClientNetworkAverageSpeed        *prometheus.HistogramVec
	MobileClientNetworkEffectiveLatency    *prometheus.HistogramVec
	MobileClientNetworkElapsedTime         *prometheus.HistogramVec
	MobileClientNetworkLatency             *prometheus.HistogramVec
	MobileClientNetworkTotalCompressedSize *prometheus.HistogramVec
	MobileClientNetworkTotalParallelReqs   *prometheus.HistogramVec
	MobileClientNetworkTotalRequests       *prometheus.HistogramVec
	MobileClientNetworkTotalSequentialReqs *prometheus.HistogramVec
	MobileClientNetworkTotalSize           *prometheus.HistogramVec
	MobileClientSessionMetadata            *prometheus.GaugeVec
	mobileClientSessionMetadataMut         sync.Mutex
	DesktopClientCPUUsage                  *prometheus.HistogramVec
	DesktopClientMemoryUsage               *prometheus.HistogramVec
	AccessControlSearchQueryDuration       prometheus.Histogram
	AccessControlExpressionCompileDuration prometheus.Histogram
	AccessControlEvaluateDuration          prometheus.Histogram
	AutoTranslateTranslateDuration         *prometheus.HistogramVec
	AutoTranslateLinguaDetectionDuration   prometheus.Histogram
	AutoTranslateProviderCallDuration      *prometheus.HistogramVec
	AutoTranslateQueueDepth                prometheus.Gauge
	AutoTranslateWorkerTaskDuration        prometheus.Histogram
	AutoTranslateRecoveryStuckFound        prometheus.Counter
	AutoTranslateNormHash                  *prometheus.CounterVec
	replicaLag                             *replicaLagCollector
}

// New creates a MetricsInterfaceImpl with its own Prometheus registry.
func New(opts Options) *MetricsInterfaceImpl {
	logger := opts.Logger
	if logger == nil {
		logger = mlog.CreateConsoleLogger()
	}

	m := &MetricsInterfaceImpl{
		opts:                   opts,
		logger:                 logger,
		Registry:               prometheus.NewRegistry(),
		dbCollectors:           make(map[string]dbCollector),
		desktopVersionLabel:    newBoundedLabel(maxDesktopVersions),
		desktopProcessLabel:    newBoundedLabel(maxDesktopProcesses),
		pluginIDLabel:          newBoundedLabel(maxClientPluginIDs),
		pluginMetricLabel:      newBoundedLabel(maxClientPluginMetrics),
		mobileVersionLabel:     newBoundedLabel(maxMobileVersions),
		mobilePlatformLabel:    newBoundedLabel(maxMobilePlatforms),
		notificationPlatformLb: newBoundedLabel(maxNotificationPlatf),
	}

	m.Registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	m.initServerMetrics()
	m.initClientMetrics()

	m.loggerCollector = newLoggerMetricsCollector(m.Registry)

	m.replicaLag = newReplicaLagCollector(m, opts.Config, opts.Store)
	m.Registry.MustRegister(m.replicaLag)

	m.handler = promhttp.InstrumentMetricHandler(m.Registry, promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{
		ErrorLog:      promErrorLogger{logger: logger},
		ErrorHandling: promhttp.ContinueOnError,
		Registry:      m.Registry,
	}))

	return m
}

type promErrorLogger struct {
	logger mlog.LoggerIFace
}

func (l promErrorLogger) Println(v ...any) {
	l.logger.Warn("Error while serving metrics", mlog.Any("details", v))
}

func (m *MetricsInterfaceImpl) newCounter(subsystem, name, help string) prometheus.Counter {
	c := prometheus.NewCounter(prometheus.CounterOpts{Namespace: namespace, Subsystem: subsystem, Name: name, Help: help})
	m.Registry.MustRegister(c)
	return c
}

func (m *MetricsInterfaceImpl) newCounterVec(subsystem, name, help string, labels ...string) *prometheus.CounterVec {
	c := prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: namespace, Subsystem: subsystem, Name: name, Help: help}, labels)
	m.Registry.MustRegister(c)
	return c
}

func (m *MetricsInterfaceImpl) newGauge(subsystem, name, help string) prometheus.Gauge {
	g := prometheus.NewGauge(prometheus.GaugeOpts{Namespace: namespace, Subsystem: subsystem, Name: name, Help: help})
	m.Registry.MustRegister(g)
	return g
}

func (m *MetricsInterfaceImpl) newGaugeVec(subsystem, name, help string, labels ...string) *prometheus.GaugeVec {
	g := prometheus.NewGaugeVec(prometheus.GaugeOpts{Namespace: namespace, Subsystem: subsystem, Name: name, Help: help}, labels)
	m.Registry.MustRegister(g)
	return g
}

func (m *MetricsInterfaceImpl) newHistogram(subsystem, name, help string, buckets []float64) prometheus.Histogram {
	h := prometheus.NewHistogram(prometheus.HistogramOpts{Namespace: namespace, Subsystem: subsystem, Name: name, Help: help, Buckets: buckets})
	m.Registry.MustRegister(h)
	return h
}

func (m *MetricsInterfaceImpl) newHistogramVec(subsystem, name, help string, buckets []float64, labels ...string) *prometheus.HistogramVec {
	h := prometheus.NewHistogramVec(prometheus.HistogramOpts{Namespace: namespace, Subsystem: subsystem, Name: name, Help: help, Buckets: buckets}, labels)
	m.Registry.MustRegister(h)
	return h
}

func (m *MetricsInterfaceImpl) initServerMetrics() {
	// Posts
	m.PostCreate = m.newCounter(subsystemPost, "total", "The total number of posts created.")
	m.WebhookPost = m.newCounter(subsystemPost, "webhooks_total", "Total number of webhook posts.")
	m.PostSentEmail = m.newCounter(subsystemPost, "emails_sent_total", "The total number of emails sent because a post was made.")
	m.PostSentPush = m.newCounter(subsystemPost, "pushes_sent_total", "The total number of mobile push notifications sent because a post was made.")
	m.PostBroadcast = m.newCounter(subsystemPost, "broadcasts_total", "The total number of websocket broadcasts sent because a post was made.")
	m.PostFileAttachment = m.newCounter(subsystemPost, "file_attachments_total", "The total number of file attachments created because a post was made.")

	// HTTP
	m.HTTPRequest = m.newCounter(subsystemHTTP, "requests_total", "The total number of HTTP requests.")
	m.HTTPError = m.newCounter(subsystemHTTP, "errors_total", "The total number of HTTP errors.")
	m.HTTPWebsockets = m.newGaugeVec(subsystemHTTP, "websockets_total", "The current number of websocket connections to this server.", "origin_client")

	// Cluster
	m.ClusterRequest = m.newCounter(subsystemCluster, "cluster_requests_total", "The total number of inter-node requests.")
	m.ClusterRequestDuration = m.newHistogram(subsystemCluster, "cluster_request_duration_seconds", "The total duration in seconds of the inter-node cluster requests.", serverDurationBuckets)
	m.ClusterEventType = m.newCounterVec(subsystemCluster, "cluster_event_type_totals", "The total number of cluster requests sent for any type.", "name_type")
	m.ClusterReliableFallbackLength = m.newHistogramVec(subsystemCluster, "cluster_reliable_fallback_length", "The length of the fallback queue used for reliable cluster messages.", queueLengthBuckets, "event")

	// Login
	m.Login = m.newCounter(subsystemLogin, "logins_total", "The total number of successful logins.")
	m.LoginFail = m.newCounter(subsystemLogin, "logins_fail_total", "The total number of failed logins.")

	// Caches
	m.EtagHit = m.newCounterVec(subsystemCache, "etag_hit_total", "Total number of etag cache hits for a specific route.", "route")
	m.EtagMiss = m.newCounterVec(subsystemCache, "etag_miss_total", "Total number of etag cache misses for a specific route.", "route")
	m.MemCacheHit = m.newCounterVec(subsystemCache, "mem_hit_total", "Total number of memory cache hits for a specific cache.", "name")
	m.MemCacheMiss = m.newCounterVec(subsystemCache, "mem_miss_total", "Total number of memory cache misses for a specific cache.", "name")
	m.MemCacheInvalidation = m.newCounterVec(subsystemCache, "mem_invalidation_total", "Total number of memory cache invalidations for a specific cache.", "name")
	m.MemCacheHitSession = m.newCounter(subsystemCache, "mem_hit_session_total", "Total number of memory cache hits for sessions.")
	m.MemCacheMissSession = m.newCounter(subsystemCache, "mem_miss_session_total", "Total number of memory cache misses for sessions.")
	m.MemCacheInvalidationSess = m.newCounter(subsystemCache, "mem_invalidation_session_total", "Total number of memory cache invalidations for sessions.")
	m.RedisEndpointDuration = m.newHistogramVec(subsystemRedis, "cache_time", "Time to execute the cache handler, in seconds.", serverDurationBuckets, "cache_name", "operation")

	// Websockets
	m.WebsocketEvent = m.newCounterVec(subsystemWebsocket, "event_total", "Total number of websocket events published to the cluster.", "type")
	m.WebsocketBroadcast = m.newCounterVec(subsystemWebsocket, "broadcasts_total", "Total number of websocket broadcasts sent to clients.", "name")
	m.WebsocketBroadcastBufferSize = m.newGaugeVec(subsystemWebsocket, "broadcast_buffer_size", "Number of events waiting in the broadcast queue of each websocket hub.", "hub")
	m.WebsocketBroadcastUsersRegister = m.newGaugeVec(subsystemWebsocket, "broadcast_users_registered", "Number of users registered in each websocket hub.", "hub")
	m.WebsocketReconnect = m.newCounterVec(subsystemWebsocket, "reconnects_total", "Total number of websocket reconnect attempts, by result and disconnect code.", "type", "disconnect_err_code")

	// Search
	m.PostsSearch = m.newCounter(subsystemSearch, "posts_searches_total", "The total number of post searches carried out.")
	m.PostsSearchDuration = m.newHistogram(subsystemSearch, "posts_searches_duration_seconds", "The total duration in seconds of post searches.", serverDurationBuckets)
	m.FilesSearch = m.newCounter(subsystemSearch, "files_searches_total", "The total number of file searches carried out.")
	m.FilesSearchDuration = m.newHistogram(subsystemSearch, "files_searches_duration_seconds", "The total duration in seconds of file searches.", serverDurationBuckets)
	m.PostIndex = m.newCounter(subsystemSearch, "post_index_total", "The total number of posts indexed.")
	m.FileIndex = m.newCounter(subsystemSearch, "file_index_total", "The total number of files indexed.")
	m.UserIndex = m.newCounter(subsystemSearch, "user_index_total", "The total number of users indexed.")
	m.ChannelIndex = m.newCounter(subsystemSearch, "channel_index_total", "The total number of channels indexed.")

	// Store / DB
	m.StoreMethodDuration = m.newHistogramVec(subsystemDB, "store_time", "Time to execute the store method, in seconds.", serverDurationBuckets, "method", "success")
	m.ReplicaLagAbsolute = prometheus.NewGaugeVec(prometheus.GaugeOpts{Namespace: namespace, Subsystem: subsystemDB, Name: "replica_lag_abs", Help: "An abstract unit for measuring replica lag (e.g. bytes of WAL not yet replayed)."}, []string{"node"})
	m.ReplicaLagTime = prometheus.NewGaugeVec(prometheus.GaugeOpts{Namespace: namespace, Subsystem: subsystemDB, Name: "replica_lag_time", Help: "A time unit for measuring replica lag."}, []string{"node"})

	// API
	m.APIEndpointDuration = m.newHistogramVec(subsystemAPI, "time", "Time to execute the API handler, in seconds.", serverDurationBuckets,
		"handler", "method", "status_code", "origin_client", "page_load_context")

	// Plugins
	m.PluginHookDuration = m.newHistogramVec(subsystemPlugin, "hook_time", "Time to execute a plugin hook handler, in seconds.", serverDurationBuckets, "plugin_id", "hook_name", "success")
	m.PluginMultiHookIterationDuration = m.newHistogramVec(subsystemPlugin, "multi_hook_time", "Time for a plugin to execute its part of a hook run across all plugins, in seconds.", serverDurationBuckets, "plugin_id")
	m.PluginMultiHookDuration = m.newHistogram(subsystemPlugin, "multi_hook_server_time", "Time for the server to execute a hook across all plugins, in seconds.", serverDurationBuckets)
	m.PluginAPIDuration = m.newHistogramVec(subsystemPlugin, "api_time", "Time to execute a plugin API call, in seconds.", serverDurationBuckets, "plugin_id", "api_name", "success")

	// Server
	m.EnabledUsers = m.newGauge(subsystemServer, "enabled_users", "The number of enabled users.")

	// Remote clusters
	m.RemoteClusterMsgSent = m.newCounterVec(subsystemRemoteCluster, "msg_sent_total", "Total number of messages sent to a remote cluster.", "remote_id")
	m.RemoteClusterMsgReceived = m.newCounterVec(subsystemRemoteCluster, "msg_received_total", "Total number of messages received from a remote cluster.", "remote_id")
	m.RemoteClusterMsgErrors = m.newCounterVec(subsystemRemoteCluster, "msg_errors_total", "Total number of errors sending messages to a remote cluster.", "remote_id", "timeout")
	m.RemoteClusterPingDuration = m.newHistogramVec(subsystemRemoteCluster, "ping_time", "The round trip time of a ping to a remote cluster, in seconds.", serverDurationBuckets, "remote_id")
	m.RemoteClusterClockSkew = m.newGaugeVec(subsystemRemoteCluster, "clock_skew", "An approximation of the clock skew between this server and a remote cluster, in seconds.", "remote_id")
	m.RemoteClusterConnStateChange = m.newCounterVec(subsystemRemoteCluster, "conn_state_change_total", "Total number of connection state changes of a remote cluster.", "remote_id", "online")

	// Shared channels
	m.SharedChannelsSync = m.newCounterVec(subsystemSharedChannels, "sync_count", "Total number of synchronisations with a remote cluster.", "remote_id")
	m.SharedChannelsTaskInQueueDuration = m.newHistogram(subsystemSharedChannels, "task_in_queue_duration_seconds", "Time a task spends in the shared channels queue before being processed, in seconds.", serverDurationBuckets)
	m.SharedChannelsQueueSize = m.newGauge(subsystemSharedChannels, "queue_size", "The current number of tasks in the shared channels queue.")
	m.SharedChannelsSyncCollectionDuration = m.newHistogramVec(subsystemSharedChannels, "sync_collection_duration_seconds", "Time to collect the data to synchronise with a remote cluster, in seconds.", serverDurationBuckets, "remote_id")
	m.SharedChannelsSyncSendDuration = m.newHistogramVec(subsystemSharedChannels, "sync_send_duration_seconds", "Time to send the synchronisation data to a remote cluster, in seconds.", serverDurationBuckets, "remote_id")
	m.SharedChannelsSyncCollectionStep = m.newHistogramVec(subsystemSharedChannels, "sync_collection_step_duration_seconds", "Time to collect the data of one synchronisation step, in seconds.", serverDurationBuckets, "remote_id", "step")
	m.SharedChannelsSyncSendStep = m.newHistogramVec(subsystemSharedChannels, "sync_send_step_duration_seconds", "Time to send the data of one synchronisation step, in seconds.", serverDurationBuckets, "remote_id", "step")

	// Jobs
	m.JobsActive = m.newGaugeVec(subsystemJobs, "active", "The number of active jobs, by job type.", "type")

	// Notifications
	m.NotificationTotalCounters = m.newCounterVec(subsystemNotifications, "total", "Total number of notification events.", "type", "platform")
	m.NotificationAckCounters = m.newCounterVec(subsystemNotifications, "total_ack", "Total number of notification events acknowledged by clients.", "type", "platform")
	m.NotificationSuccessCounters = m.newCounterVec(subsystemNotifications, "success", "Total number of successfully delivered notifications.", "type", "platform")
	m.NotificationErrorCounters = m.newCounterVec(subsystemNotifications, "error", "Total number of notifications that failed because of an error.", "type", "reason", "platform")
	m.NotificationNotSentCounters = m.newCounterVec(subsystemNotifications, "not_sent", "Total number of notifications that were intentionally not sent.", "type", "reason", "platform")
	m.NotificationUnsupportedCounters = m.newCounterVec(subsystemNotifications, "unsupported", "Total number of notifications not sent because the client doesn't support them.", "type", "reason", "platform")

	// Access control
	m.AccessControlSearchQueryDuration = m.newHistogram(subsystemAccessControl, "search_query_duration_seconds", "Time to run an access control search query, in seconds.", serverDurationBuckets)
	m.AccessControlExpressionCompileDuration = m.newHistogram(subsystemAccessControl, "expression_compile_duration_seconds", "Time to compile an access control expression, in seconds.", serverDurationBuckets)
	m.AccessControlEvaluateDuration = m.newHistogram(subsystemAccessControl, "evaluate_duration_seconds", "Time to evaluate an access control expression, in seconds.", serverDurationBuckets)
	m.AccessControlCacheInvalid = m.newCounter(subsystemAccessControl, "cache_invalidation_total", "Total number of access control cache invalidations.")

	// Auto-translation
	m.AutoTranslateTranslateDuration = m.newHistogramVec(subsystemAutoTranslate, "translate_duration_seconds", "Time to translate an object, in seconds.", serverDurationBuckets, "object_type")
	m.AutoTranslateLinguaDetectionDuration = m.newHistogram(subsystemAutoTranslate, "lingua_detection_duration_seconds", "Time to detect the language of a text, in seconds.", serverDurationBuckets)
	m.AutoTranslateProviderCallDuration = m.newHistogramVec(subsystemAutoTranslate, "provider_call_duration_seconds", "Time of a call to the translation provider, in seconds.", serverDurationBuckets, "provider", "result")
	m.AutoTranslateQueueDepth = m.newGauge(subsystemAutoTranslate, "queue_depth", "The current number of items waiting to be translated.")
	m.AutoTranslateWorkerTaskDuration = m.newHistogram(subsystemAutoTranslate, "worker_task_duration_seconds", "Time to run a translation worker task, in seconds.", serverDurationBuckets)
	m.AutoTranslateRecoveryStuckFound = m.newCounter(subsystemAutoTranslate, "recovery_stuck_found_total", "Total number of stuck translations found by the recovery process.")
	m.AutoTranslateNormHash = m.newCounterVec(subsystemAutoTranslate, "norm_hash_total", "Total number of normalized hash lookups, by result.", "result")
}

func (m *MetricsInterfaceImpl) initClientMetrics() {
	pa := []string{"platform", "agent"}

	m.ClientTimeToFirstByte = m.newHistogramVec(subsystemWebapp, "time_to_first_byte", "Duration from when a browser starts to request a page from a server until when it starts to receive data in response (seconds).", clientFastBuckets, pa...)
	m.ClientTimeToLastByte = m.newHistogramVec(subsystemWebapp, "time_to_last_byte", "Duration from when a browser starts to request a page from a server until when it receives the last byte of the resource (seconds).", clientFastBuckets, pa...)
	m.ClientTimeToDomInteractive = m.newHistogramVec(subsystemWebapp, "dom_interactive", "Duration from when a browser starts to request a page until the page becomes interactive (seconds).", clientDurationBuckets, pa...)
	m.ClientSplashScreenEnd = m.newHistogramVec(subsystemWebapp, "splash_screen", "Duration from when a browser starts to request a page until the splash screen ends (seconds).", clientDurationBuckets, "platform", "agent", "page_type")
	m.ClientFirstContentfulPaint = m.newHistogramVec(subsystemWebapp, "first_contentful_paint", "Duration of how long it takes for any content to be displayed on screen to a user (seconds).", clientDurationBuckets, pa...)
	m.ClientLargestContentfulPaint = m.newHistogramVec(subsystemWebapp, "largest_contentful_paint", "Duration of how long it takes for large content to be displayed on screen to a user (seconds).", clientDurationBuckets, "platform", "agent", "region")
	m.ClientInteractionToNextPaint = m.newHistogramVec(subsystemWebapp, "interaction_to_next_paint", "Measure of how long it takes for a user to see the effects of clicking with a mouse, tapping with a touchscreen, or pressing a key on the keyboard (seconds).", clientFastBuckets, "platform", "agent", "interaction")
	m.ClientCumulativeLayoutShift = m.newHistogramVec(subsystemWebapp, "cumulative_layout_shift", "Measure of how much a page's content shifts unexpectedly.", clsBuckets, pa...)
	m.ClientLongTasks = m.newCounterVec(subsystemWebapp, "long_tasks", "Counter of the number of times that the browser's main UI thread is blocked for more than 50ms by a single task.", pa...)
	m.ClientPageLoadDuration = m.newHistogramVec(subsystemWebapp, "page_load", "The amount of time from when the browser starts loading the web app until when the web app's load event has finished (seconds).", clientDurationBuckets, pa...)
	m.ClientChannelSwitchDuration = m.newHistogramVec(subsystemWebapp, "channel_switch", "Duration of the time taken from when a user clicks on a channel in the LHS to when posts in that channel become visible (seconds).", clientDurationBuckets, "platform", "agent", "fresh")
	m.ClientTeamSwitchDuration = m.newHistogramVec(subsystemWebapp, "team_switch", "Duration of the time taken from when a user clicks on a team in the LHS to when posts in that team become visible (seconds).", clientDurationBuckets, "platform", "agent", "fresh")
	m.ClientRHSLoadDuration = m.newHistogramVec(subsystemWebapp, "rhs_load", "Duration of the time taken from when a user clicks to open a thread in the RHS until when posts in that thread become visible (seconds).", clientDurationBuckets, pa...)
	m.ClientGlobalThreadsLoad = m.newHistogramVec(subsystemWebapp, "global_threads_load", "Duration of the time taken from when a user clicks to open Threads in the LHS until when the global threads view becomes visible (seconds).", clientDurationBuckets, pa...)
	m.ClientPluginWebappPerf = m.newHistogramVec(subsystemWebapp, "plugin_perf", "Performance measurements reported by web app plugins.", clientDurationBuckets, "platform", "agent", "plugin_id", "plugin_metric_label")

	m.MobileClientLoadDuration = m.newHistogramVec(subsystemMobile, "mobile_load", "Duration of the time taken from when a user opens the app until it finishes loading all relevant data (seconds).", clientDurationBuckets, "platform")
	m.MobileClientChannelSwitchDuration = m.newHistogramVec(subsystemMobile, "mobile_channel_switch", "Duration of the time taken from when a user switches into a channel until the relevant data is loaded (seconds).", clientDurationBuckets, "platform")
	m.MobileClientTeamSwitchDuration = m.newHistogramVec(subsystemMobile, "mobile_team_switch", "Duration of the time taken from when a user switches into a team until the relevant data is loaded (seconds).", clientDurationBuckets, "platform")

	netLabels := []string{"platform", "agent", "network_request_group"}
	m.MobileClientNetworkAverageSpeed = m.newHistogramVec(subsystemMobile, "mobile_network_requests_average_speed", "Average download speed of a group of network requests, as reported by the mobile app.", mobileNetworkSpeedBuckets, netLabels...)
	m.MobileClientNetworkEffectiveLatency = m.newHistogramVec(subsystemMobile, "mobile_network_requests_effective_latency", "Effective latency of a group of network requests, as reported by the mobile app (milliseconds).", mobileNetworkTimeBuckets, netLabels...)
	m.MobileClientNetworkElapsedTime = m.newHistogramVec(subsystemMobile, "mobile_network_requests_elapsed_time", "Total elapsed time of a group of network requests, as reported by the mobile app (milliseconds).", mobileNetworkTimeBuckets, netLabels...)
	m.MobileClientNetworkLatency = m.newHistogramVec(subsystemMobile, "mobile_network_requests_latency", "Average latency of a group of network requests, as reported by the mobile app (milliseconds).", mobileNetworkTimeBuckets, netLabels...)
	m.MobileClientNetworkTotalCompressedSize = m.newHistogramVec(subsystemMobile, "mobile_network_requests_total_compressed_size", "Total compressed size of a group of network requests, as reported by the mobile app (bytes).", mobileNetworkSizeBuckets, netLabels...)
	m.MobileClientNetworkTotalParallelReqs = m.newHistogramVec(subsystemMobile, "mobile_network_requests_total_parallel_requests", "Number of parallel requests in a group of network requests.", mobileNetworkCountBuckets, netLabels...)
	m.MobileClientNetworkTotalRequests = m.newHistogramVec(subsystemMobile, "mobile_network_requests_total_requests", "Total number of requests in a group of network requests.", mobileNetworkCountBuckets, netLabels...)
	m.MobileClientNetworkTotalSequentialReqs = m.newHistogramVec(subsystemMobile, "mobile_network_requests_total_sequential_requests", "Number of sequential requests in a group of network requests.", mobileNetworkCountBuckets, netLabels...)
	m.MobileClientNetworkTotalSize = m.newHistogramVec(subsystemMobile, "mobile_network_requests_total_size", "Total uncompressed size of a group of network requests, as reported by the mobile app (bytes).", mobileNetworkSizeBuckets, netLabels...)
	m.MobileClientSessionMetadata = m.newGaugeVec(subsystemMobile, "mobile_session_metadata", "Number of mobile sessions by app version, platform and notification state.", "version", "platform", "notifications_disabled")

	m.DesktopClientCPUUsage = m.newHistogramVec(subsystemDesktop, "cpu_usage", "CPU usage of the Desktop App processes (percent).", desktopCPUBuckets, "platform", "version", "process")
	m.DesktopClientMemoryUsage = m.newHistogramVec(subsystemDesktop, "memory_usage", "Memory usage of the Desktop App processes, as reported by the client.", desktopMemoryBuckets, "platform", "version", "process")
}

// Register exposes the registry on the metrics server. It's called by the platform each
// time the metrics server is (re)started, right after the router is created.
func (m *MetricsInterfaceImpl) Register() {
	if m.opts.HandleMetrics == nil {
		return
	}
	m.opts.HandleMetrics("/metrics", m.handler)
	m.logger.Debug("Metrics endpoint registered")
}

// Handler returns the http.Handler serving the metrics of this instance in the Prometheus
// exposition format.
func (m *MetricsInterfaceImpl) Handler() http.Handler {
	return m.handler
}

func (m *MetricsInterfaceImpl) RegisterDBCollector(db *sql.DB, name string) {
	if db == nil {
		return
	}

	m.dbCollectorsMut.Lock()
	defer m.dbCollectorsMut.Unlock()

	if existing, ok := m.dbCollectors[name]; ok {
		if existing.db == db {
			return
		}
		m.Registry.Unregister(existing.collector)
		delete(m.dbCollectors, name)
	}

	c := collectors.NewDBStatsCollector(db, name)
	if err := m.Registry.Register(c); err != nil {
		m.logger.Warn("Failed to register database metrics collector", mlog.String("db", name), mlog.Err(err))
		return
	}
	m.dbCollectors[name] = dbCollector{db: db, collector: c}
}

func (m *MetricsInterfaceImpl) UnregisterDBCollector(db *sql.DB, name string) {
	m.dbCollectorsMut.Lock()
	defer m.dbCollectorsMut.Unlock()

	existing, ok := m.dbCollectors[name]
	if !ok || (db != nil && existing.db != db) {
		return
	}
	m.Registry.Unregister(existing.collector)
	delete(m.dbCollectors, name)
}

// Posts

func (m *MetricsInterfaceImpl) IncrementPostCreate()    { m.PostCreate.Inc() }
func (m *MetricsInterfaceImpl) IncrementWebhookPost()   { m.WebhookPost.Inc() }
func (m *MetricsInterfaceImpl) IncrementPostSentEmail() { m.PostSentEmail.Inc() }
func (m *MetricsInterfaceImpl) IncrementPostSentPush()  { m.PostSentPush.Inc() }
func (m *MetricsInterfaceImpl) IncrementPostBroadcast() { m.PostBroadcast.Inc() }

func (m *MetricsInterfaceImpl) IncrementPostFileAttachment(count int) {
	if count > 0 {
		m.PostFileAttachment.Add(float64(count))
	}
}

// HTTP

func (m *MetricsInterfaceImpl) IncrementHTTPRequest() { m.HTTPRequest.Inc() }
func (m *MetricsInterfaceImpl) IncrementHTTPError()   { m.HTTPError.Inc() }

func (m *MetricsInterfaceImpl) IncrementHTTPWebSockets(originClient string) {
	m.HTTPWebsockets.WithLabelValues(originClient).Inc()
}

func (m *MetricsInterfaceImpl) DecrementHTTPWebSockets(originClient string) {
	m.HTTPWebsockets.WithLabelValues(originClient).Dec()
}

// Cluster

func (m *MetricsInterfaceImpl) IncrementClusterRequest() { m.ClusterRequest.Inc() }

func (m *MetricsInterfaceImpl) ObserveClusterRequestDuration(elapsed float64) {
	m.ClusterRequestDuration.Observe(elapsed)
}

func (m *MetricsInterfaceImpl) IncrementClusterEventType(eventType model.ClusterEvent) {
	m.ClusterEventType.WithLabelValues(string(eventType)).Inc()
}

func (m *MetricsInterfaceImpl) ObserveClusterReliableFallbackLength(event model.ClusterEvent, length int) {
	m.ClusterReliableFallbackLength.WithLabelValues(string(event)).Observe(float64(length))
}

// Login

func (m *MetricsInterfaceImpl) IncrementLogin()     { m.Login.Inc() }
func (m *MetricsInterfaceImpl) IncrementLoginFail() { m.LoginFail.Inc() }

// Caches

func (m *MetricsInterfaceImpl) IncrementEtagHitCounter(route string) {
	m.EtagHit.WithLabelValues(route).Inc()
}

func (m *MetricsInterfaceImpl) IncrementEtagMissCounter(route string) {
	m.EtagMiss.WithLabelValues(route).Inc()
}

func (m *MetricsInterfaceImpl) IncrementMemCacheHitCounter(cacheName string) {
	m.MemCacheHit.WithLabelValues(cacheName).Inc()
}

func (m *MetricsInterfaceImpl) IncrementMemCacheMissCounter(cacheName string) {
	m.MemCacheMiss.WithLabelValues(cacheName).Inc()
}

func (m *MetricsInterfaceImpl) IncrementMemCacheInvalidationCounter(cacheName string) {
	m.MemCacheInvalidation.WithLabelValues(cacheName).Inc()
}

func (m *MetricsInterfaceImpl) IncrementMemCacheMissCounterSession() { m.MemCacheMissSession.Inc() }
func (m *MetricsInterfaceImpl) IncrementMemCacheHitCounterSession()  { m.MemCacheHitSession.Inc() }
func (m *MetricsInterfaceImpl) IncrementMemCacheInvalidationCounterSession() {
	m.MemCacheInvalidationSess.Inc()
}

func (m *MetricsInterfaceImpl) AddMemCacheHitCounter(cacheName string, amount float64) {
	if amount > 0 {
		m.MemCacheHit.WithLabelValues(cacheName).Add(amount)
	}
}

func (m *MetricsInterfaceImpl) AddMemCacheMissCounter(cacheName string, amount float64) {
	if amount > 0 {
		m.MemCacheMiss.WithLabelValues(cacheName).Add(amount)
	}
}

func (m *MetricsInterfaceImpl) ObserveRedisEndpointDuration(cacheName, operation string, elapsed float64) {
	m.RedisEndpointDuration.WithLabelValues(cacheName, operation).Observe(elapsed)
}

// Websockets

func (m *MetricsInterfaceImpl) IncrementWebsocketEvent(eventType model.WebsocketEventType) {
	m.WebsocketEvent.WithLabelValues(string(eventType)).Inc()
}

func (m *MetricsInterfaceImpl) IncrementWebSocketBroadcast(eventType model.WebsocketEventType) {
	m.WebsocketBroadcast.WithLabelValues(string(eventType)).Inc()
}

func (m *MetricsInterfaceImpl) IncrementWebSocketBroadcastBufferSize(hub string, amount float64) {
	m.WebsocketBroadcastBufferSize.WithLabelValues(hub).Add(amount)
}

func (m *MetricsInterfaceImpl) DecrementWebSocketBroadcastBufferSize(hub string, amount float64) {
	m.WebsocketBroadcastBufferSize.WithLabelValues(hub).Sub(amount)
}

func (m *MetricsInterfaceImpl) IncrementWebSocketBroadcastUsersRegistered(hub string, amount float64) {
	m.WebsocketBroadcastUsersRegister.WithLabelValues(hub).Add(amount)
}

func (m *MetricsInterfaceImpl) DecrementWebSocketBroadcastUsersRegistered(hub string, amount float64) {
	m.WebsocketBroadcastUsersRegister.WithLabelValues(hub).Sub(amount)
}

func (m *MetricsInterfaceImpl) IncrementWebsocketReconnectEventWithDisconnectErrCode(eventType string, disconnectErrCode string) {
	m.WebsocketReconnect.WithLabelValues(eventType, sanitizeLabelValue(disconnectErrCode)).Inc()
}

// Search

func (m *MetricsInterfaceImpl) IncrementPostsSearchCounter() { m.PostsSearch.Inc() }
func (m *MetricsInterfaceImpl) ObservePostsSearchDuration(elapsed float64) {
	m.PostsSearchDuration.Observe(elapsed)
}
func (m *MetricsInterfaceImpl) IncrementFilesSearchCounter() { m.FilesSearch.Inc() }
func (m *MetricsInterfaceImpl) ObserveFilesSearchDuration(elapsed float64) {
	m.FilesSearchDuration.Observe(elapsed)
}
func (m *MetricsInterfaceImpl) IncrementPostIndexCounter()    { m.PostIndex.Inc() }
func (m *MetricsInterfaceImpl) IncrementFileIndexCounter()    { m.FileIndex.Inc() }
func (m *MetricsInterfaceImpl) IncrementUserIndexCounter()    { m.UserIndex.Inc() }
func (m *MetricsInterfaceImpl) IncrementChannelIndexCounter() { m.ChannelIndex.Inc() }

// Store & API

func (m *MetricsInterfaceImpl) ObserveStoreMethodDuration(method, success string, elapsed float64) {
	m.StoreMethodDuration.WithLabelValues(method, success).Observe(elapsed)
}

func (m *MetricsInterfaceImpl) ObserveAPIEndpointDuration(endpoint, method, statusCode, originClient, pageLoadContext string, elapsed float64) {
	m.APIEndpointDuration.WithLabelValues(endpoint, method, statusCode, originClient, pageLoadContext).Observe(elapsed)
}

func (m *MetricsInterfaceImpl) SetReplicaLagAbsolute(node string, value float64) {
	m.ReplicaLagAbsolute.WithLabelValues(sanitizeLabelValue(node)).Set(value)
}

func (m *MetricsInterfaceImpl) SetReplicaLagTime(node string, value float64) {
	m.ReplicaLagTime.WithLabelValues(sanitizeLabelValue(node)).Set(value)
}

// Plugins

func (m *MetricsInterfaceImpl) ObservePluginHookDuration(pluginID, hookName string, success bool, elapsed float64) {
	m.PluginHookDuration.WithLabelValues(pluginID, hookName, boolLabel(success)).Observe(elapsed)
}

func (m *MetricsInterfaceImpl) ObservePluginMultiHookIterationDuration(pluginID string, elapsed float64) {
	m.PluginMultiHookIterationDuration.WithLabelValues(pluginID).Observe(elapsed)
}

func (m *MetricsInterfaceImpl) ObservePluginMultiHookDuration(elapsed float64) {
	m.PluginMultiHookDuration.Observe(elapsed)
}

func (m *MetricsInterfaceImpl) ObservePluginAPIDuration(pluginID, apiName string, success bool, elapsed float64) {
	m.PluginAPIDuration.WithLabelValues(pluginID, apiName, boolLabel(success)).Observe(elapsed)
}

// Server

func (m *MetricsInterfaceImpl) ObserveEnabledUsers(users int64) {
	m.EnabledUsers.Set(float64(users))
}

func (m *MetricsInterfaceImpl) GetLoggerMetricsCollector() mlog.MetricsCollector {
	return m.loggerCollector
}

// Remote clusters

func (m *MetricsInterfaceImpl) IncrementRemoteClusterMsgSentCounter(remoteID string) {
	m.RemoteClusterMsgSent.WithLabelValues(remoteID).Inc()
}

func (m *MetricsInterfaceImpl) IncrementRemoteClusterMsgReceivedCounter(remoteID string) {
	m.RemoteClusterMsgReceived.WithLabelValues(remoteID).Inc()
}

func (m *MetricsInterfaceImpl) IncrementRemoteClusterMsgErrorsCounter(remoteID string, timeout bool) {
	m.RemoteClusterMsgErrors.WithLabelValues(remoteID, boolLabel(timeout)).Inc()
}

func (m *MetricsInterfaceImpl) ObserveRemoteClusterPingDuration(remoteID string, elapsed float64) {
	m.RemoteClusterPingDuration.WithLabelValues(remoteID).Observe(elapsed)
}

func (m *MetricsInterfaceImpl) ObserveRemoteClusterClockSkew(remoteID string, skew float64) {
	m.RemoteClusterClockSkew.WithLabelValues(remoteID).Set(skew)
}

func (m *MetricsInterfaceImpl) IncrementRemoteClusterConnStateChangeCounter(remoteID string, online bool) {
	m.RemoteClusterConnStateChange.WithLabelValues(remoteID, boolLabel(online)).Inc()
}

// Shared channels

func (m *MetricsInterfaceImpl) IncrementSharedChannelsSyncCounter(remoteID string) {
	m.SharedChannelsSync.WithLabelValues(remoteID).Inc()
}

func (m *MetricsInterfaceImpl) ObserveSharedChannelsTaskInQueueDuration(elapsed float64) {
	m.SharedChannelsTaskInQueueDuration.Observe(elapsed)
}

func (m *MetricsInterfaceImpl) ObserveSharedChannelsQueueSize(size int64) {
	m.SharedChannelsQueueSize.Set(float64(size))
}

func (m *MetricsInterfaceImpl) ObserveSharedChannelsSyncCollectionDuration(remoteID string, elapsed float64) {
	m.SharedChannelsSyncCollectionDuration.WithLabelValues(remoteID).Observe(elapsed)
}

func (m *MetricsInterfaceImpl) ObserveSharedChannelsSyncSendDuration(remoteID string, elapsed float64) {
	m.SharedChannelsSyncSendDuration.WithLabelValues(remoteID).Observe(elapsed)
}

func (m *MetricsInterfaceImpl) ObserveSharedChannelsSyncCollectionStepDuration(remoteID string, step string, elapsed float64) {
	m.SharedChannelsSyncCollectionStep.WithLabelValues(remoteID, step).Observe(elapsed)
}

func (m *MetricsInterfaceImpl) ObserveSharedChannelsSyncSendStepDuration(remoteID string, step string, elapsed float64) {
	m.SharedChannelsSyncSendStep.WithLabelValues(remoteID, step).Observe(elapsed)
}

// Jobs

func (m *MetricsInterfaceImpl) IncrementJobActive(jobType string) {
	m.JobsActive.WithLabelValues(jobType).Inc()
}

func (m *MetricsInterfaceImpl) DecrementJobActive(jobType string) {
	m.JobsActive.WithLabelValues(jobType).Dec()
}

// Notifications

func (m *MetricsInterfaceImpl) notificationPlatform(platform string) string {
	return m.notificationPlatformLb.value(platform)
}

func (m *MetricsInterfaceImpl) IncrementNotificationCounter(notificationType model.NotificationType, platform string) {
	m.NotificationTotalCounters.WithLabelValues(string(notificationType), m.notificationPlatform(platform)).Inc()
}

func (m *MetricsInterfaceImpl) IncrementNotificationAckCounter(notificationType model.NotificationType, platform string) {
	m.NotificationAckCounters.WithLabelValues(string(notificationType), m.notificationPlatform(platform)).Inc()
}

func (m *MetricsInterfaceImpl) IncrementNotificationSuccessCounter(notificationType model.NotificationType, platform string) {
	m.NotificationSuccessCounters.WithLabelValues(string(notificationType), m.notificationPlatform(platform)).Inc()
}

func (m *MetricsInterfaceImpl) IncrementNotificationErrorCounter(notificationType model.NotificationType, errorReason model.NotificationReason, platform string) {
	m.NotificationErrorCounters.WithLabelValues(string(notificationType), string(errorReason), m.notificationPlatform(platform)).Inc()
}

func (m *MetricsInterfaceImpl) IncrementNotificationNotSentCounter(notificationType model.NotificationType, notSentReason model.NotificationReason, platform string) {
	m.NotificationNotSentCounters.WithLabelValues(string(notificationType), string(notSentReason), m.notificationPlatform(platform)).Inc()
}

func (m *MetricsInterfaceImpl) IncrementNotificationUnsupportedCounter(notificationType model.NotificationType, notSentReason model.NotificationReason, platform string) {
	m.NotificationUnsupportedCounters.WithLabelValues(string(notificationType), string(notSentReason), m.notificationPlatform(platform)).Inc()
}

// Access control

func (m *MetricsInterfaceImpl) ObserveAccessControlSearchQueryDuration(value float64) {
	m.AccessControlSearchQueryDuration.Observe(value)
}

func (m *MetricsInterfaceImpl) ObserveAccessControlExpressionCompileDuration(value float64) {
	m.AccessControlExpressionCompileDuration.Observe(value)
}

func (m *MetricsInterfaceImpl) ObserveAccessControlEvaluateDuration(value float64) {
	m.AccessControlEvaluateDuration.Observe(value)
}

func (m *MetricsInterfaceImpl) IncrementAccessControlCacheInvalidation() {
	m.AccessControlCacheInvalid.Inc()
}

// Auto-translation

func (m *MetricsInterfaceImpl) ObserveAutoTranslateTranslateDuration(objectType string, elapsed float64) {
	m.AutoTranslateTranslateDuration.WithLabelValues(objectType).Observe(elapsed)
}

func (m *MetricsInterfaceImpl) ObserveAutoTranslateLinguaDetectionDuration(elapsed float64) {
	m.AutoTranslateLinguaDetectionDuration.Observe(elapsed)
}

func (m *MetricsInterfaceImpl) ObserveAutoTranslateProviderCallDuration(provider, result string, elapsed float64) {
	m.AutoTranslateProviderCallDuration.WithLabelValues(provider, result).Observe(elapsed)
}

func (m *MetricsInterfaceImpl) SetAutoTranslateQueueDepth(depth float64) {
	m.AutoTranslateQueueDepth.Set(depth)
}

func (m *MetricsInterfaceImpl) ObserveAutoTranslateWorkerTaskDuration(elapsed float64) {
	m.AutoTranslateWorkerTaskDuration.Observe(elapsed)
}

func (m *MetricsInterfaceImpl) AddAutoTranslateRecoveryStuckFound(count float64) {
	if count > 0 {
		m.AutoTranslateRecoveryStuckFound.Add(count)
	}
}

func (m *MetricsInterfaceImpl) IncrementAutoTranslateNormHash(result string) {
	m.AutoTranslateNormHash.WithLabelValues(result).Inc()
}

var _ einterfaces.MetricsInterface = (*MetricsInterfaceImpl)(nil)
