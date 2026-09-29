// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package cluster

import (
	"strconv"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app/platform"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

// Host is the subset of the platform service the cluster implementation
// depends on. It exists so the cluster logic can be exercised in tests
// without a database or a full platform service.
type Host interface {
	Config() *model.Config
	Log() mlog.LoggerIFace
	Metrics() einterfaces.MetricsInterface

	ClusterDiscoveryStore() store.ClusterDiscoveryStore
	SystemStore() store.SystemStore
	SchemaVersion() string

	AddConfigListener(listener func(oldCfg, newCfg *model.Config)) string
	RemoveConfigListener(id string)
	// ApplyRemoteConfig makes this node pick up a configuration saved on
	// another node of the cluster.
	ApplyRemoteConfig(cfg *model.Config) error

	InvokeClusterLeaderChangedListeners()

	// Local data served to the other nodes.
	GetLogsSkipSend(rctx request.CTX, page, perPage int, logFilter *model.LogFilter) ([]string, *model.AppError)
	GenerateSupportPacket(rctx request.CTX, options *model.SupportPacketOptions) ([]model.FileData, error)
	GetPluginStatuses() (model.PluginStatuses, *model.AppError)
	TotalWebsocketConnections() int
	TotalMasterDbConnections() int
	TotalReadDbConnections() int
	WebConnCountForUser(userID string) int
	GetWSQueues(userID, connectionID string, seqNum int64) (*model.WSQueues, error)
}

// platformHost adapts *platform.PlatformService to Host.
type platformHost struct {
	ps *platform.PlatformService
}

var _ Host = (*platformHost)(nil)

func newPlatformHost(ps *platform.PlatformService) *platformHost {
	return &platformHost{ps: ps}
}

func (h *platformHost) Config() *model.Config { return h.ps.Config() }

func (h *platformHost) Log() mlog.LoggerIFace { return h.ps.Log() }

func (h *platformHost) Metrics() einterfaces.MetricsInterface { return h.ps.Metrics() }

// The store is created after the cluster interface, so it is only resolved
// lazily, once the node starts communicating.
func (h *platformHost) ClusterDiscoveryStore() store.ClusterDiscoveryStore {
	return h.ps.Store.ClusterDiscovery()
}

func (h *platformHost) SystemStore() store.SystemStore { return h.ps.Store.System() }

func (h *platformHost) SchemaVersion() string {
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

func (h *platformHost) AddConfigListener(listener func(oldCfg, newCfg *model.Config)) string {
	return h.ps.AddConfigListener(listener)
}

func (h *platformHost) RemoveConfigListener(id string) { h.ps.RemoveConfigListener(id) }

func (h *platformHost) ApplyRemoteConfig(cfg *model.Config) error {
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
	keepNodeSpecificSettings(newCfg, local)
	if _, _, appErr := h.ps.SaveConfig(newCfg, false); appErr != nil {
		return appErr
	}
	return nil
}

func (h *platformHost) InvokeClusterLeaderChangedListeners() {
	h.ps.InvokeClusterLeaderChangedListeners()
}

func (h *platformHost) GetLogsSkipSend(rctx request.CTX, page, perPage int, logFilter *model.LogFilter) ([]string, *model.AppError) {
	return h.ps.GetLogsSkipSend(rctx, page, perPage, logFilter)
}

func (h *platformHost) GenerateSupportPacket(rctx request.CTX, options *model.SupportPacketOptions) ([]model.FileData, error) {
	return h.ps.GenerateSupportPacket(rctx, options)
}

func (h *platformHost) GetPluginStatuses() (model.PluginStatuses, *model.AppError) {
	return h.ps.GetPluginStatuses()
}

func (h *platformHost) TotalWebsocketConnections() int { return h.ps.TotalWebsocketConnections() }

func (h *platformHost) TotalMasterDbConnections() int {
	if h.ps.Store == nil {
		return 0
	}
	return h.ps.Store.TotalMasterDbConnections()
}

func (h *platformHost) TotalReadDbConnections() int {
	if h.ps.Store == nil {
		return 0
	}
	return h.ps.Store.TotalReadDbConnections()
}

func (h *platformHost) WebConnCountForUser(userID string) int {
	return h.ps.WebConnCountForUser(userID)
}

func (h *platformHost) GetWSQueues(userID, connectionID string, seqNum int64) (*model.WSQueues, error) {
	return h.ps.GetWSQueues(userID, connectionID, seqNum)
}

// keepNodeSpecificSettings copies the settings that legitimately differ
// between the nodes of a cluster from local into cfg.
func keepNodeSpecificSettings(cfg, local *model.Config) {
	if cfg == nil || local == nil {
		return
	}
	cfg.ClusterSettings.OverrideHostname = local.ClusterSettings.OverrideHostname
	cfg.ClusterSettings.NetworkInterface = local.ClusterSettings.NetworkInterface
	cfg.ClusterSettings.BindAddress = local.ClusterSettings.BindAddress
	cfg.ClusterSettings.AdvertiseAddress = local.ClusterSettings.AdvertiseAddress
}
