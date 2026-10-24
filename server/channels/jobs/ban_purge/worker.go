// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package ban_purge

import (
	"fmt"
	"strconv"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/jobs"
)

// Antimatter: deletes a banned person's messages in a community, when a ban asks for more of them than its request
// deletes at once (app.BanMember).

type AppIface interface {
	PurgeMemberPosts(rctx request.CTX, teamID, userID string, since, before int64, deletedBy string) (int, *model.AppError)
}

func MakeWorker(jobServer *jobs.JobServer, app AppIface) *jobs.SimpleWorker {
	isEnabled := func(*model.Config) bool { return true }
	execute := func(logger mlog.LoggerIFace, job *model.Job) error {
		defer jobServer.HandleJobPanic(logger, job)
		return purge(logger, job, app)
	}
	return jobs.NewSimpleWorker("BanPurge", jobServer, execute, isEnabled)
}

func purge(logger mlog.LoggerIFace, job *model.Job, app AppIface) error {
	teamID, userID, deletedBy := job.Data["team_id"], job.Data["user_id"], job.Data["deleted_by"]
	if !model.IsValidId(teamID) || !model.IsValidId(userID) {
		return fmt.Errorf("ban purge job %s: invalid team or user", job.Id)
	}
	since, err := strconv.ParseInt(job.Data["since"], 10, 64)
	if err != nil {
		return fmt.Errorf("ban purge job %s: invalid since: %w", job.Id, err)
	}
	before, err := strconv.ParseInt(job.Data["before"], 10, 64)
	if err != nil {
		return fmt.Errorf("ban purge job %s: invalid before: %w", job.Id, err)
	}
	deleted, appErr := app.PurgeMemberPosts(request.EmptyContext(logger), teamID, userID, since, before, deletedBy)
	if job.Data == nil {
		job.Data = model.StringMap{}
	}
	job.Data["deleted_posts"] = strconv.Itoa(deleted)
	logger.Info("Deleted a banned person's messages", mlog.String("team_id", teamID), mlog.String("user_id", userID), mlog.Int("deleted", deleted))
	if appErr != nil {
		return appErr
	}
	return nil
}
