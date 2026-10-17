// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/v8/channels/app/pushproxy"
	"github.com/mattermost/mattermost/server/v8/channels/jobs"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// newPushProxy creates the push proxy token manager. The server makes one for
// the push notification path and one for the job worker; they share their
// token cache.
func newPushProxy(a *App) *pushproxy.PushProxy {
	return pushproxy.New(
		func() store.SystemStore { return a.Srv().Store().System() },
		func() mlog.LoggerIFace { return a.Log() },
		func() *jobs.JobServer { return a.Srv().Jobs },
	)
}
