// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package notify_admin

import (
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/v8/channels/jobs"
)

const (
	InstallNotifyJobName = "InstallNotifyAdmin"
)

type AppIface interface {
	DoCheckForAdminNotifications(trial bool) *model.AppError
}

func MakeInstallPluginNotifyWorker(jobServer *jobs.JobServer, app AppIface) *jobs.SimpleWorker {
	isEnabled := func(_ *model.Config) bool {
		return true
	}
	execute := func(logger mlog.LoggerIFace, job *model.Job) error {
		defer jobServer.HandleJobPanic(logger, job)

		appErr := app.DoCheckForAdminNotifications(false)
		if appErr != nil {
			return appErr
		}

		return nil
	}
	worker := jobs.NewSimpleWorker(InstallNotifyJobName, jobServer, execute, isEnabled)
	return worker
}
