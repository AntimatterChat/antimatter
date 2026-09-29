// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

// loggerMetricsCollector implements mlog.MetricsCollector (logr.MetricsCollector). Logr asks
// it for one gauge and a few counters per log target; the target name is used as label.
type loggerMetricsCollector struct {
	queueSize *prometheus.GaugeVec
	logged    *prometheus.CounterVec
	errors    *prometheus.CounterVec
	dropped   *prometheus.CounterVec
	blocked   *prometheus.CounterVec
}

func newLoggerMetricsCollector(reg prometheus.Registerer) *loggerMetricsCollector {
	c := &loggerMetricsCollector{
		queueSize: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace, Subsystem: subsystemLogging, Name: "logger_queue_used",
			Help: "Number of records currently queued by a log target.",
		}, []string{"name"}),
		logged: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Subsystem: subsystemLogging, Name: "logger_logged_total",
			Help: "Total number of records written by a log target.",
		}, []string{"name"}),
		errors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Subsystem: subsystemLogging, Name: "logger_error_total",
			Help: "Total number of errors encountered by a log target.",
		}, []string{"name"}),
		dropped: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Subsystem: subsystemLogging, Name: "logger_dropped_total",
			Help: "Total number of records dropped by a log target because its queue was full.",
		}, []string{"name"}),
		blocked: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace, Subsystem: subsystemLogging, Name: "logger_blocked_total",
			Help: "Total number of times a log target blocked because its queue was full.",
		}, []string{"name"}),
	}
	reg.MustRegister(c.queueSize, c.logged, c.errors, c.dropped, c.blocked)
	return c
}

func (c *loggerMetricsCollector) QueueSizeGauge(target string) (mlog.Gauge, error) {
	return c.queueSize.GetMetricWithLabelValues(sanitizeLabelValue(target))
}

func (c *loggerMetricsCollector) LoggedCounter(target string) (mlog.Counter, error) {
	return c.logged.GetMetricWithLabelValues(sanitizeLabelValue(target))
}

func (c *loggerMetricsCollector) ErrorCounter(target string) (mlog.Counter, error) {
	return c.errors.GetMetricWithLabelValues(sanitizeLabelValue(target))
}

func (c *loggerMetricsCollector) DroppedCounter(target string) (mlog.Counter, error) {
	return c.dropped.GetMetricWithLabelValues(sanitizeLabelValue(target))
}

func (c *loggerMetricsCollector) BlockedCounter(target string) (mlog.Counter, error) {
	return c.blocked.GetMetricWithLabelValues(sanitizeLabelValue(target))
}

var _ mlog.MetricsCollector = (*loggerMetricsCollector)(nil)
