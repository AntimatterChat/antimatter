// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package ldap

import (
	"errors"
	"net/http"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/jobs"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
	ejobs "github.com/mattermost/mattermost/server/v8/einterfaces/jobs"
)

// syncJob implements ejobs.LdapSyncInterface.
type syncJob struct {
	b         Backend
	current   func() einterfaces.LdapInterface
	jobServer func() *jobs.JobServer
}

var _ ejobs.LdapSyncInterface = (*syncJob)(nil)

// NewSyncJob creates the AD/LDAP synchronization job. current returns the
// server's LDAP service, which runs the synchronization when it is this
// implementation; otherwise (a mock in tests) a dedicated instance on b does.
func NewSyncJob(b Backend, current func() einterfaces.LdapInterface, jobServer func() *jobs.JobServer) ejobs.LdapSyncInterface {
	return &syncJob{b: b, current: current, jobServer: jobServer}
}

// impl returns the LDAP implementation registered on the server, falling back
// to a dedicated instance.
func (j *syncJob) impl() *Ldap {
	if l, ok := j.current().(*Ldap); ok && l != nil {
		return l
	}
	return newLdap(j.b, nil)
}

func syncEnabled(cfg *model.Config) bool {
	return model.SafeDereference(cfg.LdapSettings.EnableSync)
}

func (j *syncJob) MakeWorker() model.Worker {
	jobServer := j.jobServer()
	execute := func(logger mlog.LoggerIFace, job *model.Job) error {
		defer jobServer.HandleJobPanic(logger, job)

		l := j.impl()
		rctx := request.EmptyContext(logger)

		opts := syncOptions{
			ReAddRemovedMembers: model.SafeDereference(j.b.Config().LdapSettings.ReAddRemovedMembers),
		}
		if v, ok := job.Data[jobDataIncludeRemovedMembers]; ok {
			opts.ReAddRemovedMembers = v == "true"
		}

		stats, appErr := l.Synchronize(rctx, opts)
		job.Data = stats.toJobData(job.Data)
		if appErr != nil {
			if err := jobServer.UpdateInProgressJobData(job); err != nil {
				logger.Warn("Failed to save AD/LDAP synchronization statistics", mlog.Err(err))
			}
			return appErr
		}
		return nil
	}
	return jobs.NewSimpleWorker("LdapSync", jobServer, execute, syncEnabled)
}

func (j *syncJob) MakeScheduler() ejobs.Scheduler {
	return &syncScheduler{jobServer: j.jobServer}
}

// syncScheduler schedules a synchronization every
// LdapSettings.SyncIntervalMinutes.
type syncScheduler struct {
	jobServer func() *jobs.JobServer
}

func (s *syncScheduler) Enabled(cfg *model.Config) bool {
	return syncEnabled(cfg)
}

func (s *syncScheduler) NextScheduleTime(cfg *model.Config, now time.Time, _ bool, lastSuccessfulJob *model.Job) *time.Time {
	return nextSyncTime(cfg, now, lastSuccessfulJob)
}

func nextSyncTime(cfg *model.Config, now time.Time, lastSuccessfulJob *model.Job) *time.Time {
	interval := max(time.Duration(model.SafeDereference(cfg.LdapSettings.SyncIntervalMinutes))*time.Minute, time.Minute)
	next := now.Add(interval)
	if lastSuccessfulJob != nil && lastSuccessfulJob.StartAt > 0 {
		next = time.UnixMilli(lastSuccessfulJob.StartAt).Add(interval)
		if next.Before(now) {
			next = now
		}
	}
	return &next
}

func (s *syncScheduler) ScheduleJob(rctx request.CTX, _ *model.Config, pendingJobs bool, _ *model.Job) (*model.Job, *model.AppError) {
	if pendingJobs {
		return nil, nil
	}
	jobServer := s.jobServer()
	if jobServer == nil {
		return nil, model.NewAppError("syncScheduler.ScheduleJob", "app.job.save.app_error", nil, "", http.StatusInternalServerError).Wrap(errors.New("job server not available"))
	}
	return jobServer.CreateJob(rctx, model.JobTypeLdapSync, nil)
}
