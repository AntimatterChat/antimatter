// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package messageexport

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/jobs"
)

// errJobCanceled is the cancellation cause used when an administrator asked
// for the running job to be canceled.
var errJobCanceled = errors.New("job cancellation requested")

// cancelPollInterval is how often a running job checks whether its
// cancellation was requested.
var cancelPollInterval = 5 * time.Second

// executeFunc runs a claimed job. The context is canceled when the job must
// stop (cancellation requested or worker stopping); implementations should
// return ctx.Err() (or any error) promptly in that case. The job's Data may be
// modified and is persisted by the worker. When warning is true the job ends
// with the warning status instead of success.
type executeFunc func(ctx context.Context, logger mlog.LoggerIFace, job *model.Job) (warning bool, err error)

// worker is a model.Worker running one job at a time, supporting job
// cancellation and graceful interruption on shutdown (the interrupted job is
// put back to pending so that it is picked up again later).
type worker struct {
	name      string
	jobServer *jobs.JobServer
	logger    mlog.LoggerIFace
	execute   executeFunc
	isEnabled func(cfg *model.Config) bool

	jobs    chan model.Job
	stop    chan struct{}
	stopped chan struct{}

	mut    sync.Mutex
	cancel context.CancelFunc
}

var _ model.Worker = (*worker)(nil)

func newWorker(name string, jobServer *jobs.JobServer, execute executeFunc, isEnabled func(cfg *model.Config) bool) *worker {
	return &worker{
		name:      name,
		jobServer: jobServer,
		logger:    jobServer.Logger().With(mlog.String("worker_name", name)),
		execute:   execute,
		isEnabled: isEnabled,
		jobs:      make(chan model.Job),
		stop:      make(chan struct{}, 1),
		stopped:   make(chan struct{}, 1),
	}
}

func (w *worker) Run() {
	w.logger.Debug("Worker started")

	ctx, cancel := context.WithCancel(context.Background())
	w.mut.Lock()
	w.cancel = cancel
	w.mut.Unlock()

	defer func() {
		cancel()
		w.logger.Debug("Worker finished")
		w.stopped <- struct{}{}
	}()

	for {
		select {
		case <-w.stop:
			w.logger.Debug("Worker received stop signal")
			return
		case job := <-w.jobs:
			w.DoJob(ctx, &job)
		}
	}
}

func (w *worker) Stop() {
	w.logger.Debug("Worker stopping")
	w.mut.Lock()
	cancel := w.cancel
	w.mut.Unlock()
	if cancel != nil {
		cancel()
	}
	w.stop <- struct{}{}
	<-w.stopped
}

func (w *worker) JobChannel() chan<- model.Job {
	return w.jobs
}

func (w *worker) IsEnabled(cfg *model.Config) bool {
	return w.isEnabled(cfg)
}

// DoJob claims and runs the job. The job is only run if it could be claimed
// (i.e. it was still pending).
func (w *worker) DoJob(ctx context.Context, job *model.Job) {
	logger := w.logger.With(jobs.JobLoggerFields(job)...)
	if ctx.Err() != nil {
		return
	}

	claimed, appErr := w.jobServer.ClaimJob(job)
	if appErr != nil {
		logger.Warn("Worker experienced an error while trying to claim job", mlog.Err(appErr))
		return
	} else if claimed == nil {
		return
	}

	w.RunClaimedJob(ctx, logger, claimed, true)
}

// RunClaimedJob runs a job which was already claimed (status in_progress) and
// records its outcome. When ctx is canceled, the job is put back to pending if
// requeueOnInterrupt is true, and marked as canceled otherwise.
func (w *worker) RunClaimedJob(ctx context.Context, logger mlog.LoggerIFace, job *model.Job, requeueOnInterrupt bool) {
	defer w.jobServer.HandleJobPanic(logger, job)

	jobCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	go w.watchCancellation(jobCtx, cancel, job.Id)

	if job.Data == nil {
		job.Data = make(model.StringMap)
	}

	warning, err := w.execute(jobCtx, logger, job)

	switch {
	case err == nil:
		if appErr := w.jobServer.SetJobProgress(job, 100); appErr != nil {
			logger.Error("Worker: Failed to update progress for job", mlog.Err(appErr))
		}
		if warning {
			if appErr := w.jobServer.SetJobWarning(job); appErr != nil {
				logger.Error("Worker: Failed to set warning for job", mlog.Err(appErr))
			}
			logger.Info("Job finished with warnings")
		} else {
			if appErr := w.jobServer.SetJobSuccess(job); appErr != nil {
				logger.Error("Worker: Failed to set success for job", mlog.Err(appErr))
			}
			logger.Info("Job finished successfully")
		}
	case errors.Is(context.Cause(jobCtx), errJobCanceled) || (ctx.Err() != nil && !requeueOnInterrupt):
		logger.Info("Job canceled")
		if appErr := w.jobServer.SetJobCanceled(job); appErr != nil {
			logger.Error("Worker: Failed to mark job as canceled", mlog.Err(appErr))
		}
	case ctx.Err() != nil:
		// The worker is stopping: keep the progress and put the job back
		// to pending so that it is resumed later.
		logger.Info("Job interrupted, it will be resumed later")
		if appErr := w.jobServer.UpdateInProgressJobData(job); appErr != nil {
			logger.Warn("Worker: Failed to save job progress", mlog.Err(appErr))
		}
		if appErr := w.jobServer.SetJobPending(job); appErr != nil {
			logger.Error("Worker: Failed to set job back to pending", mlog.Err(appErr))
		}
	default:
		logger.Error("Job failed", mlog.Err(err))
		var appErr *model.AppError
		if !errors.As(err, &appErr) {
			appErr = model.NewAppError("MessageExportJob", "ent.message_export.run_export.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
		if setErr := w.jobServer.SetJobError(job, appErr); setErr != nil {
			logger.Error("Worker: Failed to set job error", mlog.Err(setErr))
		}
	}
}

func (w *worker) watchCancellation(ctx context.Context, cancel context.CancelCauseFunc, jobID string) {
	ticker := time.NewTicker(cancelPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			job, err := w.jobServer.Store.Job().Get(request.EmptyContext(w.logger), jobID)
			if err != nil {
				w.logger.Warn("Unable to check job status", mlog.String("job_id", jobID), mlog.Err(err))
				continue
			}
			if job.Status == model.JobStatusCancelRequested {
				cancel(errJobCanceled)
				return
			}
		}
	}
}
