// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package accesscontrol

import (
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/channels/app/platform"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
	ejobs "github.com/mattermost/mattermost/server/v8/einterfaces/jobs"
)

func init() {
	app.RegisterAccessControlServiceInterface(func(a *app.App) einterfaces.AccessControlServiceInterface {
		return New(Backend{
			Store:  func() store.Store { return a.Srv().Store() },
			Config: a.Config,
			Logger: func() mlog.LoggerIFace { return a.Log() },
		})
	})

	// The platform layer only keeps a decision point; the store is attached
	// after enterprise interfaces are initialized, hence the lazy accessor.
	platform.RegisterAccessControlServiceInterface(func(ps *platform.PlatformService) einterfaces.AccessControlServiceInterface {
		return New(Backend{
			Store:  func() store.Store { return ps.Store },
			Config: ps.Config,
			Logger: ps.Log,
		})
	})

	app.RegisterJobsAccessControlSyncJobInterface(func(s *app.Server) ejobs.AccessControlSyncJobInterface {
		return &syncJob{srv: s}
	})
	app.RegisterJobsAccessControlTeamSyncJobInterface(func(s *app.Server) ejobs.AccessControlSyncJobInterface {
		return &syncJob{srv: s, team: true}
	})
}
