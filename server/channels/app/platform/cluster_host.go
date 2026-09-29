// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package platform

import (
	"strconv"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
	"github.com/mattermost/mattermost/server/v8/platform/services/cluster"
)

// clusterHost gives the cluster service access to the platform.
type clusterHost struct {
	ps *PlatformService
}

var _ cluster.Host = (*clusterHost)(nil)

func (h *clusterHost) Config() *model.Config { return h.ps.Config() }

func (h *clusterHost) Log() mlog.LoggerIFace { return h.ps.Log() }

func (h *clusterHost) Metrics() einterfaces.MetricsInterface { return h.ps.Metrics() }

// The store is created after the cluster service, so it is only resolved
// lazily, once the node starts communicating.
func (h *clusterHost) ClusterDiscoveryStore() store.ClusterDiscoveryStore {
	return h.ps.Store.ClusterDiscovery()
}

func (h *clusterHost) SystemStore() store.SystemStore { return h.ps.Store.System() }

func (h *clusterHost) SchemaVersion() string {
	if h.ps.Store == nil {
		return ""
	}
	v, err := h.ps.Store.GetDBSchemaVersion()
	if err != nil {
		h.ps.Log().Warn("Cluster: failed to read the database schema version", mlog.Err(err))
		return ""
	}
	return strconv.Itoa(v)
}

func (h *clusterHost) AddConfigListener(listener func(oldCfg, newCfg *model.Config)) string {
	return h.ps.AddConfigListener(listener)
}

func (h *clusterHost) RemoveConfigListener(id string) { h.ps.RemoveConfigListener(id) }

func (h *clusterHost) ApplyRemoteConfig(cfg *model.Config) error {
	cfgStore := h.ps.GetConfigStore()
	backend := cfgStore.String()
	if !strings.HasPrefix(backend, "file://") && !strings.HasPrefix(backend, "memory://") {
		// The configuration lives in the shared database: the node that
		// saved it already persisted it, we only need to reload it.
		return h.ps.ReloadConfig()
	}

	// Each node has its own configuration file: persist the new
	// configuration locally, but keep the settings that are specific to
	// this node.
	local := cfgStore.GetNoEnv()
	newCfg := cfg.Clone()
	cluster.KeepNodeSpecificSettings(newCfg, local)
	if _, _, appErr := h.ps.SaveConfig(newCfg, false); appErr != nil {
		return appErr
	}
	return nil
}

func (h *clusterHost) InvokeClusterLeaderChangedListeners() {
	h.ps.InvokeClusterLeaderChangedListeners()
}

func (h *clusterHost) GetLogsSkipSend(rctx request.CTX, page, perPage int, logFilter *model.LogFilter) ([]string, *model.AppError) {
	return h.ps.GetLogsSkipSend(rctx, page, perPage, logFilter)
}

func (h *clusterHost) GenerateSupportPacket(rctx request.CTX, options *model.SupportPacketOptions) ([]model.FileData, error) {
	return h.ps.GenerateSupportPacket(rctx, options)
}

func (h *clusterHost) GetPluginStatuses() (model.PluginStatuses, *model.AppError) {
	return h.ps.GetPluginStatuses()
}

func (h *clusterHost) TotalWebsocketConnections() int { return h.ps.TotalWebsocketConnections() }

func (h *clusterHost) TotalMasterDbConnections() int {
	if h.ps.Store == nil {
		return 0
	}
	return h.ps.Store.TotalMasterDbConnections()
}

func (h *clusterHost) TotalReadDbConnections() int {
	if h.ps.Store == nil {
		return 0
	}
	return h.ps.Store.TotalReadDbConnections()
}

func (h *clusterHost) WebConnCountForUser(userID string) int {
	return h.ps.WebConnCountForUser(userID)
}

func (h *clusterHost) GetWSQueues(userID, connectionID string, seqNum int64) (*model.WSQueues, error) {
	return h.ps.GetWSQueues(userID, connectionID, seqNum)
}
