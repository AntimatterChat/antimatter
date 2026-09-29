// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package metrics

import (
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

const defaultReplicaLagRefreshInterval = 5 * time.Second

// replicaLagCollector exposes the replica lag gauges. The lag itself is measured by running
// the queries configured in SqlSettings.ReplicaLagSettings (through store.ReplicaLagAbs and
// store.ReplicaLagTime, which report back via SetReplicaLagAbsolute/SetReplicaLagTime).
//
// Measurements are driven by scrapes: when the gauges are collected and the last
// measurement is older than SqlSettings.ReplicaMonitorIntervalSeconds, a new measurement
// is started in the background. Scrapes never wait for the database, so a slow or
// unreachable replica can't stall the metrics endpoint; the values exposed are those of
// the latest completed measurement. No goroutine outlives a measurement, so there's
// nothing to stop on shutdown.
type replicaLagCollector struct {
	m      *MetricsInterfaceImpl
	config func() *model.Config
	store  func() store.Store

	mut         sync.Mutex
	running     bool
	lastRefresh time.Time

	// done is signalled when a refresh completes; used by tests.
	done chan struct{}
}

func newReplicaLagCollector(m *MetricsInterfaceImpl, config func() *model.Config, st func() store.Store) *replicaLagCollector {
	return &replicaLagCollector{
		m:      m,
		config: config,
		store:  st,
	}
}

func (c *replicaLagCollector) Describe(ch chan<- *prometheus.Desc) {
	c.m.ReplicaLagAbsolute.Describe(ch)
	c.m.ReplicaLagTime.Describe(ch)
}

func (c *replicaLagCollector) Collect(ch chan<- prometheus.Metric) {
	c.maybeRefresh()
	c.m.ReplicaLagAbsolute.Collect(ch)
	c.m.ReplicaLagTime.Collect(ch)
}

func (c *replicaLagCollector) maybeRefresh() {
	if c.config == nil || c.store == nil {
		return
	}
	cfg := c.config()
	if cfg == nil || len(cfg.SqlSettings.ReplicaLagSettings) == 0 {
		return
	}

	interval := defaultReplicaLagRefreshInterval
	if s := cfg.SqlSettings.ReplicaMonitorIntervalSeconds; s != nil && *s > 0 {
		interval = time.Duration(*s) * time.Second
	}

	c.mut.Lock()
	if c.running || time.Since(c.lastRefresh) < interval {
		c.mut.Unlock()
		return
	}
	c.running = true
	c.mut.Unlock()

	go c.refresh()
}

func (c *replicaLagCollector) refresh() {
	defer func() {
		if r := recover(); r != nil {
			c.m.logger.Warn("Panic while measuring replica lag", mlog.String("panic", fmt.Sprint(r)))
		}
		c.mut.Lock()
		c.running = false
		c.lastRefresh = time.Now()
		done := c.done
		c.mut.Unlock()
		if done != nil {
			select {
			case done <- struct{}{}:
			default:
			}
		}
	}()

	st := c.store()
	if st == nil {
		return
	}

	if err := st.ReplicaLagAbs(); err != nil {
		c.m.logger.Warn("Failed to measure absolute replica lag", mlog.Err(err))
	}
	if err := st.ReplicaLagTime(); err != nil {
		c.m.logger.Warn("Failed to measure time based replica lag", mlog.Err(err))
	}
}
