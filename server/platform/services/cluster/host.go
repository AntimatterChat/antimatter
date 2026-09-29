// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package cluster

import (

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
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

// KeepNodeSpecificSettings copies the settings that legitimately differ
// between the nodes of a cluster from local into cfg.
func KeepNodeSpecificSettings(cfg, local *model.Config) {
	if cfg == nil || local == nil {
		return
	}
	cfg.ClusterSettings.OverrideHostname = local.ClusterSettings.OverrideHostname
	cfg.ClusterSettings.NetworkInterface = local.ClusterSettings.NetworkInterface
	cfg.ClusterSettings.BindAddress = local.ClusterSettings.BindAddress
	cfg.ClusterSettings.AdvertiseAddress = local.ClusterSettings.AdvertiseAddress
}
