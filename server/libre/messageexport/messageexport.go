// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

// Package messageexport implements the compliance (message) export:
// einterfaces.MessageExportInterface and the message export job
// (ejobs.MessageExportJobInterface).
//
// Posts are exported incrementally, by UpdateAt, in batches of
// MessageExportSettings.BatchSize posts. Each job continues where the
// previous regular job stopped (or from ExportFromTimestamp for the first
// one). For the CSV, Actiance XML and Global Relay zip formats, every batch
// is written as a zip file to
//
//	export/<compliance-export-YYYY-MM-DD-HHhMMm>-<start>-<end>/batchNNN-<batch start>-<batch end>.zip
//
// in the export file store; for the Global Relay format, one EML per channel
// and batch is delivered by SMTP to the Global Relay archive.
package messageexport

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/channels/jobs"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
	ejobs "github.com/mattermost/mattermost/server/v8/einterfaces/jobs"
	"github.com/mattermost/mattermost/server/v8/libre/messageexport/shared"
)

// syncPollInterval is how often StartSynchronizeJob polls a job run by
// another worker.
var syncPollInterval = time.Second

func init() {
	app.RegisterJobsMessageExportJobInterface(func(s *app.Server) ejobs.MessageExportJobInterface {
		return NewJob(serverDeps(s))
	})
	app.RegisterMessageExportInterface(func(a *app.App) einterfaces.MessageExportInterface {
		return &MessageExport{deps: func() Deps { return serverDeps(a.Srv()) }}
	})
}

func serverDeps(s *app.Server) Deps {
	return Deps{
		JobServer:         s.Jobs,
		Store:             s.Store,
		Config:            s.Config,
		FileBackend:       s.FileBackend,
		ExportFileBackend: s.ExportFileBackend,
	}
}

// MessageExport implements einterfaces.MessageExportInterface.
type MessageExport struct {
	deps func() Deps
}

var _ einterfaces.MessageExportInterface = (*MessageExport)(nil)

// NewMessageExport creates a MessageExport using the given dependencies.
func NewMessageExport(deps Deps) *MessageExport {
	return &MessageExport{deps: func() Deps { return deps }}
}

// StartSynchronizeJob creates a message export job and runs it synchronously,
// in the calling goroutine, until it completes or rctx's context is done (in
// which case the job is canceled). When exportFromTimestamp is positive, the
// posts updated after it are exported and the job does not affect where the
// next scheduled export starts; otherwise the export continues where the
// previous one stopped. The returned job reflects its final status.
func (me *MessageExport) StartSynchronizeJob(rctx request.CTX, exportFromTimestamp int64) (*model.Job, *model.AppError) {
	deps := me.deps()
	if deps.JobServer == nil {
		return nil, model.NewAppError("StartSynchronizeJob", "ent.message_export.run_export.app_error", nil, "job server unavailable", http.StatusInternalServerError)
	}
	j := NewJob(deps)

	data := model.StringMap{}
	if exportFromTimestamp > 0 {
		start := strconv.FormatInt(exportFromTimestamp, 10)
		data[shared.JobDataJobStartTime] = start
		data[shared.JobDataBatchStartTime] = start
		data[shared.JobDataInitiatedBy] = shared.InitiatedByCLI
	}

	job, appErr := deps.JobServer.CreateJob(rctx, model.JobTypeMessageExport, data)
	if appErr != nil {
		return nil, appErr
	}

	logger := rctx.Logger().With(jobs.JobLoggerFields(job)...)
	ctx := rctx.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	claimed, appErr := deps.JobServer.ClaimJob(job)
	if appErr != nil {
		return nil, appErr
	}
	if claimed != nil {
		j.newWorker().RunClaimedJob(ctx, logger, claimed, false)
	} else {
		// Another worker claimed the job first: wait for it.
		logger.Debug("Message export job claimed by another worker, waiting for it to finish")
	}

	return waitForJob(ctx, rctx, deps.JobServer, job.Id)
}

func isTerminal(status string) bool {
	switch status {
	case model.JobStatusSuccess, model.JobStatusWarning, model.JobStatusError, model.JobStatusCanceled:
		return true
	}
	return false
}

// waitForJob polls the job until it reaches a terminal status. When ctx is
// done first, cancellation of the job is requested.
func waitForJob(ctx context.Context, rctx request.CTX, jobServer *jobs.JobServer, id string) (*model.Job, *model.AppError) {
	for {
		job, appErr := jobServer.GetJob(rctx, id)
		if appErr != nil {
			return nil, appErr
		}
		if isTerminal(job.Status) {
			return job, nil
		}
		select {
		case <-ctx.Done():
			if appErr := jobServer.RequestCancellation(rctx, id); appErr != nil {
				rctx.Logger().Warn("Unable to cancel the message export job", mlog.String("job_id", id), mlog.Err(appErr))
			}
			job.Status = model.JobStatusCanceled
			return job, nil
		case <-time.After(syncPollInterval):
		}
	}
}
