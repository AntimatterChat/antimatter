// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// revertHostedPushNotificationServer switches EmailSettings.PushNotificationServer away from
// the Mattermost-hosted push notification service (MHPNS: global, regional or legacy
// production endpoints) back to the test (TPNS) endpoint. The hosted service is a paid
// Mattermost Inc. offering that refuses to deliver for this server, so leaving push pointing
// at it would silently drop notifications; operators are expected to run their own
// mattermost-push-proxy. It runs on server start and when this node becomes the cluster
// leader.
//
// Custom endpoints and env-managed values are never touched.
func (s *Server) revertHostedPushNotificationServer() {
	if !s.IsLeader() {
		return
	}

	if s.platform.IsConfigReadOnly() {
		return
	}

	// Respect an environment-variable override on the setting. The config store re-applies env
	// overrides on save anyway, so without this guard a save would be futile and only produce
	// spurious audit records, logs, and cluster config traffic.
	if emailOverrides, ok := s.platform.GetEnvironmentOverrides()["EmailSettings"].(map[string]any); ok {
		if _, overridden := emailOverrides["PushNotificationServer"]; overridden {
			return
		}
	}

	// Decide and mutate on the same snapshot so a concurrent config write between the
	// decision and the save can't be stomped with a stale value. The residual race between
	// Clone and Set is inherent to every SaveConfig caller.
	cfg := s.platform.Config().Clone()
	current := *cfg.EmailSettings.PushNotificationServer
	if !model.IsMHPNSEndpoint(current) {
		return
	}
	target := model.GenericNotificationServer

	cfg.EmailSettings.PushNotificationServer = model.NewPointer(target)
	if _, _, appErr := s.platform.SaveConfig(cfg, true); appErr != nil {
		mlog.Warn("Failed to switch push notification server away from the hosted push notification service",
			mlog.String("old", current), mlog.String("new", target), mlog.Err(appErr))
		return
	}
	mlog.Info("Automatically switched push notification server away from the unsupported hosted push notification service",
		mlog.String("old", current), mlog.String("new", target))

	rctx := request.EmptyContext(s.Log())
	appInstance := New(ServerConnector(s.Channels()))
	rec := appInstance.MakeAuditRecord(rctx, model.AuditEventAutoSelectPushNotificationServer, model.AuditStatusSuccess)
	model.AddEventParameterToAuditRec(rec, "old_push_notification_server", current)
	model.AddEventParameterToAuditRec(rec, "new_push_notification_server", target)
	appInstance.LogAuditRec(rctx, rec, nil)
}
