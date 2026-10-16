// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"github.com/mattermost/mattermost/server/v8/channels/app/accesscontrol"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

// accessControlSyncJobDeps returns what the access control membership sync
// jobs need from the server.
func (s *Server) accessControlSyncJobDeps() accesscontrol.SyncJobDeps {
	return accesscontrol.SyncJobDeps{
		JobServer: s.Jobs,
		Config:    s.Config,
		Store:     s.Store,
		AccessControl: func() einterfaces.AccessControlServiceInterface {
			return s.Channels().AccessControl
		},
		Members: func() accesscontrol.MembershipSyncer {
			return New(ServerConnector(s.Channels()))
		},
	}
}
