// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package messageexport

import (
	"archive/zip"
	"context"
	"fmt"
	"net/http"
	"path"
	"strconv"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/jobs"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	ejobs "github.com/mattermost/mattermost/server/v8/einterfaces/jobs"
	"github.com/mattermost/mattermost/server/v8/libre/messageexport/shared"
	"github.com/mattermost/mattermost/server/v8/platform/shared/filestore"
)

const (
	workerName = "MessageExport"

	defaultBatchSize = 10000

	// previousJobsPageSize is the page size used when looking for the
	// previous export job to continue from.
	previousJobsPageSize = 100
	// previousJobsMaxPages bounds that lookup.
	previousJobsMaxPages = 50
)

// Deps holds the dependencies of the message export job.
type Deps struct {
	JobServer *jobs.JobServer
	Store     func() store.Store
	Config    func() *model.Config
	// FileBackend is the file store holding the attachments.
	FileBackend func() filestore.FileBackend
	// ExportFileBackend is the file store where the exports are written.
	ExportFileBackend func() filestore.FileBackend
	// Now returns the current time. Defaults to time.Now.
	Now func() time.Time
	// newSender creates the Global Relay SMTP sender (overridable in tests).
	newSender func(settings *model.GlobalRelayMessageExportSettings) (emlSender, error)
}

// Job implements ejobs.MessageExportJobInterface.
type Job struct {
	deps Deps
}

var _ ejobs.MessageExportJobInterface = (*Job)(nil)

// NewJob creates the message export job builder.
func NewJob(deps Deps) *Job {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.newSender == nil {
		deps.newSender = func(settings *model.GlobalRelayMessageExportSettings) (emlSender, error) {
			return newGlobalRelaySender(settings)
		}
	}
	return &Job{deps: deps}
}

func isEnabled(cfg *model.Config) bool {
	return model.SafeDereference(cfg.MessageExportSettings.EnableExport)
}

// MakeWorker creates the worker running message export jobs.
func (j *Job) MakeWorker() model.Worker {
	return j.newWorker()
}

func (j *Job) newWorker() *worker {
	return newWorker(workerName, j.deps.JobServer, j.execute, isEnabled)
}

// MakeScheduler creates the daily scheduler of the message export, running at
// MessageExportSettings.DailyRunTime.
func (j *Job) MakeScheduler() ejobs.Scheduler {
	return &scheduler{DailyScheduler: jobs.NewDailyScheduler(j.deps.JobServer, model.JobTypeMessageExport, dailyRunTime, isEnabled)}
}

// scheduler is a daily scheduler that does not queue a new export while
// another one is still pending.
type scheduler struct {
	*jobs.DailyScheduler
}

func (s *scheduler) ScheduleJob(rctx request.CTX, cfg *model.Config, pendingJobs bool, lastSuccessfulJob *model.Job) (*model.Job, *model.AppError) {
	if pendingJobs {
		return nil, nil
	}
	return s.DailyScheduler.ScheduleJob(rctx, cfg, pendingJobs, lastSuccessfulJob)
}

func dailyRunTime(cfg *model.Config) *time.Time {
	value := model.SafeDereference(cfg.MessageExportSettings.DailyRunTime)
	t, err := time.Parse("15:04", value)
	if err != nil {
		mlog.Warn("Invalid compliance export daily run time, using 01:00", mlog.String("value", value), mlog.Err(err))
		t, _ = time.Parse("15:04", "01:00")
	}
	return &t
}

func conversionError(key, value string, err error) *model.AppError {
	return model.NewAppError("MessageExportJob", "ent.message_export.job_data_conversion.app_error", nil,
		fmt.Sprintf("key=%s value=%q", key, value), http.StatusInternalServerError).Wrap(err)
}

func getInt64(data model.StringMap, key string) (int64, bool, *model.AppError) {
	value := data[key]
	if value == "" {
		return 0, false, nil
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, false, conversionError(key, value, err)
	}
	return n, true, nil
}

func validExportType(t string) bool {
	switch t {
	case model.ComplianceExportTypeCsv, model.ComplianceExportTypeActiance,
		model.ComplianceExportTypeGlobalrelay, model.ComplianceExportTypeGlobalrelayZip:
		return true
	}
	return false
}

// isManualJob reports whether the job was initiated outside of the regular
// flow (mmctl, CLI with an explicit start): such jobs do not affect where the
// next regular export starts.
func isManualJob(job *model.Job) bool {
	switch job.Data[shared.JobDataInitiatedBy] {
	case shared.InitiatedByMmctl, shared.InitiatedByCLI:
		return true
	}
	return false
}

// previousJobPosition returns the position (batch_start_time/id) reached by
// the most recent successful, non manual, export job other than currentID.
func (j *Job) previousJobPosition(rctx request.CTX, st store.Store, currentID string) (int64, string, bool, error) {
	for page := 0; page < previousJobsMaxPages; page++ {
		list, err := st.Job().GetAllByTypePage(rctx, model.JobTypeMessageExport, page*previousJobsPageSize, previousJobsPageSize)
		if err != nil {
			return 0, "", false, fmt.Errorf("failed to get the previous export jobs: %w", err)
		}
		for _, job := range list {
			if job.Id == currentID || isManualJob(job) {
				continue
			}
			if job.Status != model.JobStatusSuccess && job.Status != model.JobStatusWarning {
				continue
			}
			start, ok, appErr := getInt64(job.Data, shared.JobDataBatchStartTime)
			if appErr != nil || !ok {
				continue
			}
			return start, job.Data[shared.JobDataBatchStartId], true, nil
		}
		if len(list) < previousJobsPageSize {
			break
		}
	}
	return 0, "", false, nil
}

// jobState is the state of a running export, mirrored in the job's Data.
type jobState struct {
	exportType string
	exportDir  string
	jobStart   int64
	jobEnd     int64
	position   model.MessageExportCursor
	batchNum   int
	messages   int64
	files      int64
	warnings   int64
	written    bool
}

func (s *jobState) save(job *model.Job) {
	job.Data[shared.JobDataExportType] = s.exportType
	job.Data[shared.JobDataExportDir] = s.exportDir
	job.Data[shared.JobDataJobStartTime] = strconv.FormatInt(s.jobStart, 10)
	job.Data[shared.JobDataJobEndTime] = strconv.FormatInt(s.jobEnd, 10)
	job.Data[shared.JobDataBatchStartTime] = strconv.FormatInt(s.position.LastPostUpdateAt, 10)
	job.Data[shared.JobDataBatchStartId] = s.position.LastPostId
	job.Data[shared.JobDataBatchNumber] = strconv.Itoa(s.batchNum)
	job.Data[shared.JobDataMessagesExported] = strconv.FormatInt(s.messages, 10)
	job.Data[shared.JobDataFilesExported] = strconv.FormatInt(s.files, 10)
	job.Data[shared.JobDataWarningCount] = strconv.FormatInt(s.warnings, 10)
	job.Data[shared.JobDataIsDownloadable] = strconv.FormatBool(s.written)
}

func (s *jobState) progress() int64 {
	if s.jobEnd <= s.jobStart {
		return 0
	}
	p := (s.position.LastPostUpdateAt - s.jobStart) * 100 / (s.jobEnd - s.jobStart)
	return max(0, min(p, 99))
}

// initState computes the parameters of the job from its Data (possibly left
// by a previous, interrupted, run) and the configuration.
func (j *Job) initState(rctx request.CTX, st store.Store, cfg *model.Config, job *model.Job) (*jobState, error) {
	s := &jobState{}
	now := j.deps.Now()

	s.exportType = job.Data[shared.JobDataExportType]
	if s.exportType == "" {
		s.exportType = model.SafeDereference(cfg.MessageExportSettings.ExportFormat)
	}
	if !validExportType(s.exportType) {
		return nil, model.NewAppError("MessageExportJob", "ent.message_export.run_export.app_error", nil,
			"unknown export type "+s.exportType, http.StatusBadRequest)
	}

	var ok bool
	var appErr *model.AppError
	if s.jobEnd, ok, appErr = getInt64(job.Data, shared.JobDataJobEndTime); appErr != nil {
		return nil, appErr
	} else if !ok {
		s.jobEnd = now.UnixMilli()
	}

	jobStartID := job.Data[shared.JobDataJobStartId]
	if s.jobStart, ok, appErr = getInt64(job.Data, shared.JobDataJobStartTime); appErr != nil {
		return nil, appErr
	} else if !ok {
		batchStart, hasBatchStart, appErr := getInt64(job.Data, shared.JobDataBatchStartTime)
		if appErr != nil {
			return nil, appErr
		}
		if hasBatchStart {
			s.jobStart = batchStart
			jobStartID = job.Data[shared.JobDataBatchStartId]
		} else {
			start, id, found, err := j.previousJobPosition(rctx, st, job.Id)
			if err != nil {
				return nil, err
			}
			if found {
				s.jobStart, jobStartID = start, id
			} else {
				// First export ever: start when the export was enabled.
				s.jobStart = model.SafeDereference(cfg.MessageExportSettings.ExportFromTimestamp)
				jobStartID = ""
			}
		}
	}
	job.Data[shared.JobDataJobStartId] = jobStartID

	s.position = model.MessageExportCursor{LastPostUpdateAt: s.jobStart, LastPostId: jobStartID, UntilUpdateAt: s.jobEnd}
	if batchStart, hasBatchStart, appErr := getInt64(job.Data, shared.JobDataBatchStartTime); appErr != nil {
		return nil, appErr
	} else if hasBatchStart {
		// Resuming an interrupted job, or a job created with an explicit
		// batch start.
		s.position.LastPostUpdateAt = batchStart
		s.position.LastPostId = job.Data[shared.JobDataBatchStartId]
	}

	s.exportDir = job.Data[shared.JobDataExportDir]
	if s.exportDir == "" {
		s.exportDir = path.Join(model.ComplianceExportPath,
			fmt.Sprintf("%s-%d-%d", now.Format(model.ComplianceExportDirectoryFormat), s.jobStart, s.jobEnd))
	}

	for key, dst := range map[string]*int64{
		shared.JobDataMessagesExported: &s.messages,
		shared.JobDataFilesExported:    &s.files,
		shared.JobDataWarningCount:     &s.warnings,
	} {
		v, _, appErr := getInt64(job.Data, key)
		if appErr != nil {
			return nil, appErr
		}
		*dst = v
	}
	batchNum, _, appErr := getInt64(job.Data, shared.JobDataBatchNumber)
	if appErr != nil {
		return nil, appErr
	}
	s.batchNum = int(batchNum)
	s.written = job.Data[shared.JobDataIsDownloadable] == "true"
	return s, nil
}

// execute runs a message export job.
func (j *Job) execute(ctx context.Context, logger mlog.LoggerIFace, job *model.Job) (bool, error) {
	cfg := j.deps.Config()
	st := j.deps.Store()
	rctx := request.EmptyContext(logger).WithContext(ctx)

	state, err := j.initState(rctx, st, cfg, job)
	if err != nil {
		return false, err
	}
	state.save(job)
	j.saveProgress(logger, job, state.progress())

	logger.Info("Message export started",
		mlog.String("export_type", state.exportType),
		mlog.String("export_dir", state.exportDir),
		mlog.Int("job_start_time", state.jobStart),
		mlog.Int("job_end_time", state.jobEnd),
		mlog.Int("batch_start_time", state.position.LastPostUpdateAt),
	)

	settings := cfg.MessageExportSettings
	batchSize := model.SafeDereference(settings.BatchSize)
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}
	builder := &batchBuilder{
		store:                   st,
		channelBatchSize:        model.SafeDereference(settings.ChannelBatchSize),
		channelHistoryBatchSize: model.SafeDereference(settings.ChannelHistoryBatchSize),
	}

	run := &exportRun{
		ctx:      ctx,
		logger:   logger,
		settings: settings,
	}
	if j.deps.FileBackend != nil {
		run.fileBackend = j.deps.FileBackend()
	}

	var sender emlSender
	if state.exportType == model.ComplianceExportTypeGlobalrelay {
		sender, err = j.deps.newSender(settings.GlobalRelaySettings)
		if err != nil {
			return false, model.NewAppError("MessageExportJob", "ent.message_export.run_export.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
		defer func() {
			if cerr := sender.Close(); cerr != nil {
				logger.Debug("Error closing the SMTP connection", mlog.Err(cerr))
			}
		}()
	}

	exportedInThisJob := state.messages > 0
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}

		posts, next, err := st.Compliance().MessageExport(rctx, state.position, batchSize)
		if err != nil {
			return false, model.NewAppError("MessageExportJob", "ent.message_export.run_export.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
		last := len(posts) < batchSize

		batchStart := state.position.LastPostUpdateAt
		batchEnd := next.LastPostUpdateAt
		if last {
			batchEnd = state.jobEnd
		}
		if len(posts) == 0 {
			if batchStart >= state.jobEnd {
				break
			}
			// No more posts, but channel membership changes may still have
			// happened in the remaining window.
			active, err := st.ChannelMemberHistory().GetChannelsWithActivityDuring(batchStart, batchEnd)
			if err != nil {
				return false, model.NewAppError("MessageExportJob", "ent.message_export.calculate_channel_exports.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
			}
			if len(active) == 0 {
				break
			}
		}

		b, err := builder.build(state.batchNum+1, batchStart, batchEnd, posts)
		if err != nil {
			return false, model.NewAppError("MessageExportJob", "ent.message_export.calculate_channel_exports.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}

		run.now = j.deps.Now()
		run.warnings = nil
		run.filesExported = 0
		if err := j.exportBatch(run, state, b, sender); err != nil {
			if ctx.Err() != nil {
				return false, ctx.Err()
			}
			return false, model.NewAppError("MessageExportJob", "ent.message_export.run_export.app_error", nil,
				fmt.Sprintf("batch=%d", b.number), http.StatusInternalServerError).Wrap(err)
		}

		state.batchNum = b.number
		state.messages += int64(len(posts))
		state.files += int64(run.filesExported)
		state.warnings += int64(len(run.warnings))
		if len(posts) > 0 {
			state.position.LastPostUpdateAt = next.LastPostUpdateAt
			state.position.LastPostId = next.LastPostId
			exportedInThisJob = true
		}
		job.Data[shared.JobDataProgressMessage] = fmt.Sprintf("%d messages exported in %d batches.", state.messages, state.batchNum)
		state.save(job)
		j.saveProgress(logger, job, state.progress())

		logger.Debug("Message export batch done", mlog.Int("batch", b.number), mlog.Int("posts", len(posts)), mlog.Int("channels", len(b.channels)))

		if last {
			break
		}
	}

	if !exportedInThisJob && state.position.LastPostUpdateAt < state.jobEnd {
		// Nothing was exported: the next export continues from the end of
		// this one.
		state.position.LastPostUpdateAt = state.jobEnd
		state.position.LastPostId = ""
	}
	job.Data[shared.JobDataProgressMessage] = fmt.Sprintf("%d messages exported in %d batches.", state.messages, state.batchNum)
	state.save(job)

	logger.Info("Message export finished",
		mlog.Int("messages_exported", state.messages),
		mlog.Int("files_exported", state.files),
		mlog.Int("batches", state.batchNum),
		mlog.Int("warnings", state.warnings),
	)
	return state.warnings > 0, nil
}

// exportBatch writes (or delivers) one batch in the job's format.
func (j *Job) exportBatch(run *exportRun, state *jobState, b *batch, sender emlSender) error {
	if state.exportType == model.ComplianceExportTypeGlobalrelay {
		return run.deliverGlobalRelayBatch(b, sender)
	}

	var fill func(zw *zip.Writer) error
	switch state.exportType {
	case model.ComplianceExportTypeCsv:
		fill = func(zw *zip.Writer) error { return run.writeCSVBatch(zw, b) }
	case model.ComplianceExportTypeActiance:
		fill = func(zw *zip.Writer) error { return run.writeActianceBatch(zw, b) }
	case model.ComplianceExportTypeGlobalrelayZip:
		fill = func(zw *zip.Writer) error { return run.writeGlobalRelayZipBatch(zw, b) }
	default:
		return fmt.Errorf("unsupported export type %q", state.exportType)
	}

	if j.deps.ExportFileBackend == nil {
		return fmt.Errorf("no export file backend")
	}
	backend := j.deps.ExportFileBackend()
	if err := writeZip(backend, batchFilePath(state.exportDir, b), fill); err != nil {
		return err
	}
	state.written = true
	return nil
}

func (j *Job) saveProgress(logger mlog.LoggerIFace, job *model.Job, progress int64) {
	if j.deps.JobServer == nil {
		return
	}
	if appErr := j.deps.JobServer.SetJobProgress(job, progress); appErr != nil {
		logger.Warn("Unable to save the message export progress", mlog.Err(appErr))
	}
}
