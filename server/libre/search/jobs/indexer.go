// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

// Package jobs implements the search indexing and post index aggregation
// jobs of the Elasticsearch / OpenSearch engine.
package jobs

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/libre/search/engine"
)

// Job data keys. The index_* keys and sub_type are also set by the admin
// console; start_time and end_time (epoch milliseconds) may be provided to
// only index a time range.
const (
	dataStartTime       = "start_time"
	dataEndTime         = "end_time"
	dataFailedDocuments = "failed_documents"

	staleJobThreshold = 15 * time.Minute
	batchTimeout      = 10 * time.Minute
)

type entity string

const (
	entityChannels entity = "channels"
	entityUsers    entity = "users"
	entityPosts    entity = "posts"
	entityFiles    entity = "files"
)

// Channels and users first: they are small and make autocomplete usable
// early.
var allEntities = []entity{entityChannels, entityUsers, entityPosts, entityFiles}

func (en entity) enabledKey() string { return "index_" + string(en) }
func (en entity) timeKey() string    { return string(en) + "_last_time" }
func (en entity) idKey() string      { return string(en) + "_last_id" }
func (en entity) doneKey() string    { return string(en) + "_done" }
func (en entity) countKey() string   { return string(en) + "_indexed" }

// JobServer is the subset of the job server used by the workers.
type JobServer interface {
	Config() *model.Config
	Logger() mlog.LoggerIFace
	ClaimJob(job *model.Job) (*model.Job, *model.AppError)
	GetJob(rctx request.CTX, id string) (*model.Job, *model.AppError)
	GetJobsByTypeAndStatus(rctx request.CTX, jobType string, status string) ([]*model.Job, *model.AppError)
	SetJobProgress(job *model.Job, progress int64) *model.AppError
	SetJobSuccess(job *model.Job) *model.AppError
	SetJobError(job *model.Job, jobError *model.AppError) *model.AppError
	SetJobPending(job *model.Job) *model.AppError
	SetJobCanceled(job *model.Job) *model.AppError
}

// BulkIndexer is the subset of the search engine used by the indexer.
type BulkIndexer interface {
	IsActive() bool
	PostAction(post *model.PostForIndexing) (engine.BulkAction, bool)
	ChannelAction(channel *model.Channel, userIDs []string) (engine.BulkAction, bool)
	UserAction(user *model.User, teamIDs, channelIDs []string) engine.BulkAction
	FileAction(file *model.FileForIndexing) engine.BulkAction
	BulkIndex(ctx context.Context, actions []engine.BulkAction) ([]engine.BulkFailure, *model.AppError)
}

// IndexerWorker rebuilds the search indexes from the database. The job is
// resumable: its position is saved in the job data after every batch, and a
// job interrupted by a server shutdown is set back to pending and resumed.
type IndexerWorker struct {
	jobType   string
	jobServer JobServer
	store     store.Store
	getIndex  func() BulkIndexer
	logger    mlog.LoggerIFace

	stateMut sync.Mutex
	stopped  bool
	stopCh   chan struct{}
	doneCh   chan struct{}
	jobs     chan model.Job
}

func NewIndexerWorker(jobServer JobServer, st store.Store, getIndex func() BulkIndexer) *IndexerWorker {
	return &IndexerWorker{
		jobType:   model.JobTypeElasticsearchPostIndexing,
		jobServer: jobServer,
		store:     st,
		getIndex:  getIndex,
		logger:    jobServer.Logger().With(mlog.String("worker_name", "SearchIndexer")),
		stopped:   true,
		jobs:      make(chan model.Job),
	}
}

func (w *IndexerWorker) Run() {
	w.stateMut.Lock()
	if !w.stopped {
		w.stateMut.Unlock()
		return
	}
	w.stopped = false
	w.stopCh = make(chan struct{})
	w.doneCh = make(chan struct{})
	stopCh, doneCh := w.stopCh, w.doneCh
	w.stateMut.Unlock()

	w.logger.Debug("Worker started")
	defer func() {
		w.logger.Debug("Worker finished")
		close(doneCh)
	}()

	w.requeueStaleJobs()

	for {
		select {
		case <-stopCh:
			return
		case job := <-w.jobs:
			w.doJob(&job, stopCh)
		}
	}
}

func (w *IndexerWorker) Stop() {
	w.stateMut.Lock()
	if w.stopped {
		w.stateMut.Unlock()
		return
	}
	w.stopped = true
	close(w.stopCh)
	doneCh := w.doneCh
	w.stateMut.Unlock()

	w.logger.Debug("Worker stopping")
	<-doneCh
}

func (w *IndexerWorker) JobChannel() chan<- model.Job {
	return w.jobs
}

func (w *IndexerWorker) IsEnabled(cfg *model.Config) bool {
	return cfg != nil && model.SafeDereference(cfg.ElasticsearchSettings.EnableIndexing)
}

// requeueStaleJobs sets back to pending the indexing jobs left in progress by
// a server that died, so that they are resumed.
func (w *IndexerWorker) requeueStaleJobs() {
	rctx := request.EmptyContext(w.logger)
	jobs, appErr := w.jobServer.GetJobsByTypeAndStatus(rctx, w.jobType, model.JobStatusInProgress)
	if appErr != nil {
		w.logger.Warn("Failed to look for interrupted indexing jobs", mlog.Err(appErr))
		return
	}
	threshold := model.GetMillis() - staleJobThreshold.Milliseconds()
	for _, job := range jobs {
		lastActivity := max(job.LastActivityAt, job.StartAt)
		if lastActivity > threshold {
			continue
		}
		w.logger.Info("Resuming an interrupted indexing job", mlog.String("job_id", job.Id))
		if appErr := w.jobServer.SetJobPending(job); appErr != nil {
			w.logger.Warn("Failed to set an interrupted indexing job back to pending", mlog.String("job_id", job.Id), mlog.Err(appErr))
		}
	}
}

// indexerRun is the state of one execution of a job.
type indexerRun struct {
	worker    *IndexerWorker
	job       *model.Job
	logger    mlog.LoggerIFace
	rctx      request.CTX
	index     BulkIndexer
	startTime int64
	endTime   int64
	batchSize int
	failed    int64
}

type jobOutcome int

const (
	outcomeDone jobOutcome = iota
	outcomeStopped
	outcomeCanceled
	outcomeFailed
)

func (w *IndexerWorker) doJob(job *model.Job, stopCh <-chan struct{}) {
	logger := w.logger.With(mlog.String("job_id", job.Id))

	claimed, appErr := w.jobServer.ClaimJob(job)
	if appErr != nil {
		logger.Warn("Failed to claim the indexing job", mlog.Err(appErr))
		return
	}
	if claimed == nil {
		return
	}
	job = claimed
	if job.Data == nil {
		job.Data = model.StringMap{}
	}

	defer func() {
		if r := recover(); r != nil {
			logger.Error("Indexing job panicked", mlog.Any("panic", r))
			w.setError(logger, job, model.NewAppError("IndexerWorker", "app.job.update.app_error", nil, "", http.StatusInternalServerError))
		}
	}()

	run := &indexerRun{worker: w, job: job, logger: logger, rctx: request.EmptyContext(logger)}
	outcome, appErr := run.execute(stopCh)
	switch outcome {
	case outcomeDone:
		if run.failed > 0 {
			appErr = model.NewAppError("IndexerWorker", "ent.elasticsearch.indexer.do_job.bulk_failures.error", map[string]any{"NumFailed": run.failed}, "", http.StatusInternalServerError)
			w.setError(logger, job, appErr)
			return
		}
		if err := w.jobServer.SetJobProgress(job, 100); err != nil {
			logger.Warn("Failed to update the progress of the indexing job", mlog.Err(err))
		}
		if err := w.jobServer.SetJobSuccess(job); err != nil {
			logger.Error("Failed to set the indexing job as successful", mlog.Err(err))
		}
		logger.Info("Indexing job completed")
	case outcomeStopped:
		logger.Info("Indexing job interrupted, it will be resumed later")
		if err := w.jobServer.SetJobPending(job); err != nil {
			logger.Error("Failed to set the indexing job back to pending", mlog.Err(err))
		}
	case outcomeCanceled:
		logger.Info("Indexing job canceled")
		if err := w.jobServer.SetJobCanceled(job); err != nil {
			logger.Error("Failed to set the indexing job as canceled", mlog.Err(err))
		}
	case outcomeFailed:
		logger.Error("Indexing job failed", mlog.Err(appErr))
		w.setError(logger, job, appErr)
	}
}

func (w *IndexerWorker) setError(logger mlog.LoggerIFace, job *model.Job, appErr *model.AppError) {
	if err := w.jobServer.SetJobError(job, appErr); err != nil {
		logger.Error("Failed to set the indexing job as failed", mlog.Err(err))
	}
}

func parseInt64(data model.StringMap, key string) (int64, bool, error) {
	v, ok := data[key]
	if !ok || v == "" {
		return 0, false, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	return n, true, err
}

func (r *indexerRun) init() *model.AppError {
	data := r.job.Data
	params := map[string]any{"Backend": "Search"}

	start, ok, err := parseInt64(data, dataStartTime)
	if err != nil {
		return model.NewAppError("IndexerWorker", "ent.elasticsearch.indexer.do_job.parse_start_time.error", params, "", http.StatusBadRequest).Wrap(err)
	}
	if !ok {
		oldest, err := r.worker.store.Post().GetOldestEntityCreationTime()
		if err != nil {
			return model.NewAppError("IndexerWorker", "ent.elasticsearch.indexer.do_job.get_oldest_entity.error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
		// Batches start strictly after the cursor.
		start = max(oldest-1, 0)
		data[dataStartTime] = strconv.FormatInt(start, 10)
	}

	end, ok, err := parseInt64(data, dataEndTime)
	if err != nil {
		return model.NewAppError("IndexerWorker", "ent.elasticsearch.indexer.do_job.parse_end_time.error", params, "", http.StatusBadRequest).Wrap(err)
	}
	if !ok {
		end = model.GetMillis()
		data[dataEndTime] = strconv.FormatInt(end, 10)
	}

	failed, _, _ := parseInt64(data, dataFailedDocuments)
	r.failed = failed
	r.startTime = start
	r.endTime = end

	r.batchSize = model.SafeDereference(r.worker.jobServer.Config().ElasticsearchSettings.BatchSize)
	if r.batchSize <= 0 {
		r.batchSize = model.ElasticsearchSettingsDefaultBatchSize
	}
	return nil
}

func (r *indexerRun) enabled(en entity) bool {
	return r.job.Data[en.enabledKey()] != "false"
}

func (r *indexerRun) cursor(en entity) (int64, string) {
	t, ok, err := parseInt64(r.job.Data, en.timeKey())
	if !ok || err != nil {
		t = r.startTime
	}
	return t, r.job.Data[en.idKey()]
}

func (r *indexerRun) setCursor(en entity, t int64, id string, count int, done bool) {
	r.job.Data[en.timeKey()] = strconv.FormatInt(t, 10)
	r.job.Data[en.idKey()] = id
	indexed, _, _ := parseInt64(r.job.Data, en.countKey())
	r.job.Data[en.countKey()] = strconv.FormatInt(indexed+int64(count), 10)
	if done {
		r.job.Data[en.doneKey()] = "true"
	}
}

// progress estimates the completion from the position of every entity in
// the indexed time range.
func (r *indexerRun) progress() int64 {
	var total float64
	n := 0
	for _, en := range allEntities {
		if !r.enabled(en) {
			continue
		}
		n++
		if r.job.Data[en.doneKey()] == "true" {
			total++
			continue
		}
		t, _ := r.cursor(en)
		if r.endTime > r.startTime {
			total += min(max(float64(t-r.startTime)/float64(r.endTime-r.startTime), 0), 1)
		}
	}
	if n == 0 {
		return 99
	}
	return min(int64(total/float64(n)*100), 99)
}

// checkCanceled reports whether a cancellation was requested for the job.
func (r *indexerRun) checkCanceled() bool {
	current, appErr := r.worker.jobServer.GetJob(r.rctx, r.job.Id)
	if appErr != nil {
		r.logger.Warn("Failed to check the status of the indexing job", mlog.Err(appErr))
		return false
	}
	return current.Status == model.JobStatusCancelRequested || current.Status == model.JobStatusCanceled
}

func (r *indexerRun) execute(stopCh <-chan struct{}) (jobOutcome, *model.AppError) {
	r.index = r.worker.getIndex()
	if r.index == nil || !r.index.IsActive() {
		return outcomeFailed, model.NewAppError("IndexerWorker", "ent.elasticsearch.not_started.error", map[string]any{"Backend": "Search engine"}, "", http.StatusInternalServerError)
	}
	if appErr := r.init(); appErr != nil {
		return outcomeFailed, appErr
	}

	for _, en := range allEntities {
		if !r.enabled(en) || r.job.Data[en.doneKey()] == "true" {
			continue
		}
		r.logger.Info("Indexing entities", mlog.String("entity", string(en)))
		for {
			select {
			case <-stopCh:
				return outcomeStopped, nil
			default:
			}
			if r.checkCanceled() {
				return outcomeCanceled, nil
			}
			if !r.index.IsActive() {
				return outcomeStopped, nil
			}

			done, appErr := r.indexBatch(en)
			if appErr != nil {
				return outcomeFailed, appErr
			}
			r.job.Data[dataFailedDocuments] = strconv.FormatInt(r.failed, 10)
			if err := r.worker.jobServer.SetJobProgress(r.job, r.progress()); err != nil {
				r.logger.Warn("Failed to save the progress of the indexing job", mlog.Err(err))
			}
			if done {
				break
			}
		}
	}
	return outcomeDone, nil
}

func (r *indexerRun) send(actions []engine.BulkAction) *model.AppError {
	if len(actions) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), batchTimeout)
	defer cancel()
	failures, appErr := r.index.BulkIndex(ctx, actions)
	if appErr != nil {
		return appErr
	}
	r.failed += int64(len(failures))
	return nil
}

func (r *indexerRun) indexBatch(en entity) (bool, *model.AppError) {
	switch en {
	case entityChannels:
		return r.indexChannels()
	case entityUsers:
		return r.indexUsers()
	case entityPosts:
		return r.indexPosts()
	default:
		return r.indexFiles()
	}
}

func (r *indexerRun) indexPosts() (bool, *model.AppError) {
	lastTime, lastID := r.cursor(entityPosts)
	posts, err := r.worker.store.Post().GetPostsBatchForIndexing(lastTime, lastID, r.batchSize)
	if err != nil {
		return false, model.NewAppError("IndexerWorker", "ent.elasticsearch.post.get_posts_batch_for_indexing.error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	done := len(posts) < r.batchSize
	actions := make([]engine.BulkAction, 0, len(posts))
	for _, post := range posts {
		if post.CreateAt > r.endTime {
			done = true
			break
		}
		if action, ok := r.index.PostAction(post); ok {
			actions = append(actions, action)
		}
		lastTime, lastID = post.CreateAt, post.Id
	}
	if appErr := r.send(actions); appErr != nil {
		return false, appErr
	}
	r.setCursor(entityPosts, lastTime, lastID, len(actions), done)
	return done, nil
}

func (r *indexerRun) indexFiles() (bool, *model.AppError) {
	lastTime, lastID := r.cursor(entityFiles)
	files, err := r.worker.store.FileInfo().GetFilesBatchForIndexing(lastTime, lastID, true, r.batchSize)
	if err != nil {
		return false, model.NewAppError("IndexerWorker", "ent.elasticsearch.post.get_files_batch_for_indexing.error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	done := len(files) < r.batchSize
	actions := make([]engine.BulkAction, 0, len(files))
	for _, file := range files {
		if file.CreateAt > r.endTime {
			done = true
			break
		}
		actions = append(actions, r.index.FileAction(file))
		lastTime, lastID = file.CreateAt, file.Id
	}
	if appErr := r.send(actions); appErr != nil {
		return false, appErr
	}
	r.setCursor(entityFiles, lastTime, lastID, len(actions), done)
	return done, nil
}

func (r *indexerRun) indexChannels() (bool, *model.AppError) {
	// Private channels need a membership lookup each: keep batches moderate.
	limit := min(r.batchSize, 1000)
	lastTime, lastID := r.cursor(entityChannels)
	channels, err := r.worker.store.Channel().GetChannelsBatchForIndexing(lastTime, lastID, limit)
	if err != nil {
		return false, model.NewAppError("IndexerWorker", "ent.elasticsearch.index_channels_batch.error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	done := len(channels) < limit
	actions := make([]engine.BulkAction, 0, len(channels))
	for _, channel := range channels {
		if channel.CreateAt > r.endTime {
			done = true
			break
		}
		lastTime, lastID = channel.CreateAt, channel.Id

		var userIDs []string
		if channel.Type == model.ChannelTypePrivate {
			userIDs, err = r.worker.store.Channel().GetAllChannelMemberIdsByChannelId(channel.Id)
			if err != nil {
				return false, model.NewAppError("IndexerWorker", "ent.elasticsearch.getAllChannelMembers.error", nil, "channel_id="+channel.Id, http.StatusInternalServerError).Wrap(err)
			}
		}
		if action, ok := r.index.ChannelAction(channel, userIDs); ok {
			actions = append(actions, action)
		}
	}
	if appErr := r.send(actions); appErr != nil {
		return false, appErr
	}
	r.setCursor(entityChannels, lastTime, lastID, len(actions), done)
	return done, nil
}

func (r *indexerRun) indexUsers() (bool, *model.AppError) {
	// Every user needs a membership lookup: keep batches moderate.
	limit := min(r.batchSize, 500)
	lastTime, lastID := r.cursor(entityUsers)
	batch, err := r.worker.store.User().GetUsersBatchForIndexing(lastTime, lastID, limit)
	if err != nil {
		return false, model.NewAppError("IndexerWorker", "ent.elasticsearch.index_user.error", nil, "failed to get the users batch", http.StatusInternalServerError).Wrap(err)
	}

	done := len(batch) < limit
	var inRange []*model.UserForIndexing
	for _, u := range batch {
		if u.CreateAt > r.endTime {
			done = true
			continue
		}
		inRange = append(inRange, u)
		// The batch is only sorted by creation time: track the greatest
		// (CreateAt, Id) pair explicitly.
		if u.CreateAt > lastTime || (u.CreateAt == lastTime && u.Id > lastID) {
			lastTime, lastID = u.CreateAt, u.Id
		}
	}

	ids := make([]string, 0, len(inRange))
	for _, u := range inRange {
		ids = append(ids, u.Id)
	}
	// The batch lacks the email addresses, needed for email autocomplete.
	profiles := map[string]*model.User{}
	if len(ids) > 0 {
		users, err := r.worker.store.User().GetProfileByIds(r.rctx, ids, nil, false)
		if err != nil {
			return false, model.NewAppError("IndexerWorker", "ent.elasticsearch.index_user.error", nil, "failed to get user profiles", http.StatusInternalServerError).Wrap(err)
		}
		for _, u := range users {
			profiles[u.Id] = u
		}
	}

	actions := make([]engine.BulkAction, 0, len(inRange))
	for _, u := range inRange {
		user := profiles[u.Id]
		if user == nil {
			user = &model.User{Id: u.Id, Username: u.Username, Nickname: u.Nickname, FirstName: u.FirstName, LastName: u.LastName, Roles: u.Roles, CreateAt: u.CreateAt, DeleteAt: u.DeleteAt}
		}
		// The batch only lists memberships of public, direct and group
		// channels: add the private channels, like live indexing does.
		channelIDs := append([]string{}, u.ChannelsIds...)
		members, err := r.worker.store.Channel().GetAllChannelMembersForUser(r.rctx, u.Id, false, true)
		if err != nil {
			return false, model.NewAppError("IndexerWorker", "ent.elasticsearch.getAllChannelMembers.error", nil, "user_id="+u.Id, http.StatusInternalServerError).Wrap(err)
		}
		seen := make(map[string]bool, len(channelIDs)+len(members))
		for _, id := range channelIDs {
			seen[id] = true
		}
		for id := range members {
			if !seen[id] {
				seen[id] = true
				channelIDs = append(channelIDs, id)
			}
		}
		actions = append(actions, r.index.UserAction(user, u.TeamsIds, channelIDs))
	}
	if appErr := r.send(actions); appErr != nil {
		return false, appErr
	}
	r.setCursor(entityUsers, lastTime, lastID, len(actions), done)
	return done, nil
}
