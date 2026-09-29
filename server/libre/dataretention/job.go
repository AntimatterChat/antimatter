// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package dataretention

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/channels/jobs"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	ejobs "github.com/mattermost/mattermost/server/v8/einterfaces/jobs"
	"github.com/mattermost/mattermost/server/v8/platform/services/searchengine"
	"github.com/mattermost/mattermost/server/v8/platform/shared/filestore"
)

// Keys of the data retention job's Data map.
const (
	JobDataMessageCutoff         = "message_retention_cutoff"
	JobDataFileCutoff            = "file_retention_cutoff"
	JobDataStage                 = "stage"
	JobDataPostsDeleted          = "posts_deleted"
	JobDataReactionsDeleted      = "reactions_deleted"
	JobDataFilesDeleted          = "files_deleted"
	JobDataThreadsDeleted        = "threads_deleted"
	JobDataThreadMembersDeleted  = "thread_memberships_deleted"
	JobDataMemberHistoryDeleted  = "channel_member_history_deleted"
	JobDataOrphansDeleted        = "orphaned_rows_deleted"
	JobDataPluginRecordsDeleted  = "plugin_records_deleted"
	JobDataWarnings              = "warning_count"
	postsTableForRetentionIDs    = "Posts"
	workerName                   = "DataRetention"
	defaultDeletionJobStartTime  = "02:00"
	defaultBatchSize             = 3000
	defaultRetentionIdsBatchSize = 100
)

// Stages of the job, in execution order. They are used for progress
// reporting.
var stages = []string{
	"posts",
	"posts_dependencies",
	"threads",
	"thread_memberships",
	"channel_member_history",
	"orphans",
	"files",
	"search_indexes",
	"plugins",
}

// JobDeps holds the dependencies of the data retention job.
type JobDeps struct {
	JobServer   *jobs.JobServer
	Store       func() store.Store
	Config      func() *model.Config
	FileBackend func() filestore.FileBackend
	// SearchEngines returns the active search engines (may be nil).
	SearchEngines func() []searchengine.SearchEngineInterface
	// RunPluginsDataRetention invokes the RunDataRetention hook of the
	// plugins and returns the number of deleted records (may be nil).
	RunPluginsDataRetention func(nowMillis, batchSize int64) (int64, error)
	// Now returns the current time in milliseconds. Defaults to model.GetMillis.
	Now func() int64
}

func serverJobDeps(s *app.Server) JobDeps {
	return JobDeps{
		JobServer:   s.Jobs,
		Store:       s.Store,
		Config:      s.Config,
		FileBackend: s.FileBackend,
		SearchEngines: func() []searchengine.SearchEngineInterface {
			if s.Platform() == nil || s.Platform().SearchEngine == nil {
				return nil
			}
			return s.Platform().SearchEngine.GetActiveEngines()
		},
		RunPluginsDataRetention: func(nowMillis, batchSize int64) (int64, error) {
			ch := s.Channels()
			if ch == nil {
				return 0, nil
			}
			var total int64
			var errs []error
			ch.RunMultiHook(func(hooks plugin.Hooks, manifest *model.Manifest) bool {
				deleted, err := hooks.RunDataRetention(nowMillis, batchSize)
				if err != nil {
					errs = append(errs, fmt.Errorf("plugin %s: %w", manifest.Id, err))
				}
				total += deleted
				return true
			}, plugin.RunDataRetentionID)
			return total, errors.Join(errs...)
		},
	}
}

// Job implements ejobs.DataRetentionJobInterface.
type Job struct {
	deps JobDeps
}

var _ ejobs.DataRetentionJobInterface = (*Job)(nil)

// NewJob creates the data retention job builder.
func NewJob(deps JobDeps) *Job {
	if deps.Now == nil {
		deps.Now = model.GetMillis
	}
	return &Job{deps: deps}
}

// MakeWorker creates the worker running data retention jobs. The worker is
// always enabled: granular policies may exist even when the global policy is
// disabled, and a job with nothing to do finishes immediately.
func (j *Job) MakeWorker() model.Worker {
	return newWorker(workerName, j.deps.JobServer, j.execute, func(*model.Config) bool { return true })
}

// MakeScheduler creates the daily scheduler of the data retention job, which
// runs at DataRetentionSettings.DeletionJobStartTime.
func (j *Job) MakeScheduler() ejobs.Scheduler {
	return &scheduler{DailyScheduler: jobs.NewDailyScheduler(j.deps.JobServer, model.JobTypeDataRetention, deletionJobStartTime, func(*model.Config) bool { return true })}
}

// scheduler is a daily scheduler that does not queue a new job while another
// data retention job is still pending.
type scheduler struct {
	*jobs.DailyScheduler
}

func (s *scheduler) ScheduleJob(rctx request.CTX, cfg *model.Config, pendingJobs bool, lastSuccessfulJob *model.Job) (*model.Job, *model.AppError) {
	if pendingJobs {
		return nil, nil
	}
	return s.DailyScheduler.ScheduleJob(rctx, cfg, pendingJobs, lastSuccessfulJob)
}

func deletionJobStartTime(cfg *model.Config) *time.Time {
	value := model.SafeDereference(cfg.DataRetentionSettings.DeletionJobStartTime)
	t, err := time.Parse("15:04", value)
	if err != nil {
		mlog.Warn("Invalid data retention job start time, using the default", mlog.String("value", value), mlog.Err(err))
		t, _ = time.Parse("15:04", defaultDeletionJobStartTime)
	}
	return &t
}

// run holds the state of one execution of the job.
type run struct {
	j       *Job
	ctx     context.Context
	logger  mlog.LoggerIFace
	job     *model.Job
	store   store.Store
	backend filestore.FileBackend

	now            int64
	messageCutoff  int64 // 0 when global message deletion is disabled
	fileCutoff     int64 // 0 when global file deletion is disabled
	preservePinned bool
	batchSize      int64
	idsBatchSize   int
	pause          time.Duration
	counters       map[string]int64
}

func (j *Job) execute(ctx context.Context, logger mlog.LoggerIFace, job *model.Job) error {
	cfg := j.deps.Config()
	settings := cfg.DataRetentionSettings

	r := &run{
		j:              j,
		ctx:            ctx,
		logger:         logger,
		job:            job,
		store:          j.deps.Store(),
		now:            j.deps.Now(),
		preservePinned: model.SafeDereference(settings.PreservePinnedPosts),
		batchSize:      int64(model.SafeDereference(settings.BatchSize)),
		idsBatchSize:   model.SafeDereference(settings.RetentionIdsBatchSize),
		pause:          time.Duration(model.SafeDereference(settings.TimeBetweenBatchesMilliseconds)) * time.Millisecond,
		counters:       map[string]int64{},
	}
	if j.deps.FileBackend != nil {
		r.backend = j.deps.FileBackend()
	}
	if r.batchSize <= 0 {
		r.batchSize = defaultBatchSize
	}
	if r.idsBatchSize <= 0 {
		r.idsBatchSize = defaultRetentionIdsBatchSize
	}
	if model.SafeDereference(settings.EnableMessageDeletion) {
		r.messageCutoff = r.now - int64(settings.GetMessageRetentionHours())*msPerHour
	}
	if model.SafeDereference(settings.EnableFileDeletion) {
		r.fileCutoff = r.now - int64(settings.GetFileRetentionHours())*msPerHour
	}

	job.Data[JobDataMessageCutoff] = strconv.FormatInt(r.messageCutoff, 10)
	job.Data[JobDataFileCutoff] = strconv.FormatInt(r.fileCutoff, 10)

	logger.Info("Data retention job started",
		mlog.Int("message_retention_cutoff", r.messageCutoff),
		mlog.Int("file_retention_cutoff", r.fileCutoff),
		mlog.Bool("preserve_pinned_posts", r.preservePinned),
	)

	steps := []func() error{
		r.deletePosts,
		r.deletePostsDependencies,
		r.deleteThreads,
		r.deleteThreadMemberships,
		r.deleteChannelMemberHistory,
		r.deleteOrphans,
		r.deleteFiles,
		r.deleteSearchIndexes,
		r.runPlugins,
	}
	for i, step := range steps {
		r.setStage(i)
		if err := step(); err != nil {
			return err
		}
	}

	// The deletions bypass the store caches: drop them so that deleted
	// posts and files are not served from the cache of this node.
	if r.counters[JobDataPostsDeleted] > 0 {
		r.store.Post().ClearCaches()
	}
	if r.counters[JobDataFilesDeleted] > 0 {
		r.store.FileInfo().ClearCaches()
	}

	r.saveProgress(100)
	logger.Info("Data retention job finished",
		mlog.Int("posts_deleted", r.counters[JobDataPostsDeleted]),
		mlog.Int("files_deleted", r.counters[JobDataFilesDeleted]),
	)
	return nil
}

func (r *run) setStage(i int) {
	r.job.Data[JobDataStage] = stages[i]
	r.saveProgress(int64(i * 100 / len(stages)))
}

func (r *run) add(key string, n int64) {
	r.counters[key] += n
	r.job.Data[key] = strconv.FormatInt(r.counters[key], 10)
}

func (r *run) warn(msg string, fields ...mlog.Field) {
	r.logger.Warn(msg, fields...)
	r.add(JobDataWarnings, 1)
}

func (r *run) saveProgress(progress int64) {
	if r.j.deps.JobServer == nil {
		return
	}
	if appErr := r.j.deps.JobServer.SetJobProgress(r.job, progress); appErr != nil {
		r.logger.Warn("Unable to save the data retention job progress", mlog.Err(appErr))
	}
}

func (r *run) batchConfig() model.RetentionPolicyBatchConfigs {
	return model.RetentionPolicyBatchConfigs{
		Now:                 r.now,
		GlobalPolicyEndTime: r.messageCutoff,
		Limit:               r.batchSize,
		PreservePinnedPosts: r.preservePinned,
	}
}

type policyDeleteFunc func(model.RetentionPolicyBatchConfigs, model.RetentionPolicyCursor) (int64, model.RetentionPolicyCursor, error)

// deleteForPolicies runs a retention-policy aware deletion until all
// policies (channel, team, global) are done, pausing between batches.
func (r *run) deleteForPolicies(what, counter string, fn policyDeleteFunc) error {
	cursor := model.RetentionPolicyCursor{}
	for {
		if err := r.ctx.Err(); err != nil {
			return err
		}
		deleted, next, err := fn(r.batchConfig(), cursor)
		if err != nil {
			return fmt.Errorf("failed to delete %s: %w", what, err)
		}
		cursor = next
		r.add(counter, deleted)
		if cursor.ChannelPoliciesDone && cursor.TeamPoliciesDone && cursor.GlobalPoliciesDone {
			return nil
		}
		if err := sleepCtx(r.ctx, r.pause); err != nil {
			return err
		}
	}
}

func (r *run) deletePosts() error {
	return r.deleteForPolicies("posts", JobDataPostsDeleted, r.store.Post().PermanentDeleteBatchForRetentionPolicies)
}

func (r *run) deleteThreads() error {
	return r.deleteForPolicies("threads", JobDataThreadsDeleted, r.store.Thread().PermanentDeleteBatchForRetentionPolicies)
}

func (r *run) deleteThreadMemberships() error {
	return r.deleteForPolicies("thread memberships", JobDataThreadMembersDeleted, r.store.Thread().PermanentDeleteBatchThreadMembershipsForRetentionPolicies)
}

func (r *run) deleteChannelMemberHistory() error {
	return r.deleteForPolicies("channel member history", JobDataMemberHistoryDeleted, r.store.ChannelMemberHistory().PermanentDeleteBatchForRetentionPolicies)
}

// deletePostsDependencies processes the IDs of the posts deleted by the
// retention policies (recorded by the store in RetentionIdsForDeletion):
// the attached files and the reactions of those posts are deleted.
func (r *run) deletePostsDependencies() error {
	for {
		if err := r.ctx.Err(); err != nil {
			return err
		}
		rows, err := r.store.RetentionPolicy().GetIdsForDeletionByTableName(postsTableForRetentionIDs, r.idsBatchSize)
		if err != nil {
			return fmt.Errorf("failed to get the IDs of deleted posts: %w", err)
		}
		if len(rows) == 0 {
			return nil
		}
		for _, row := range rows {
			if err := r.ctx.Err(); err != nil {
				return err
			}
			if err := r.deleteFilesForPosts(row.Ids); err != nil {
				return err
			}
			// This also removes the RetentionIdsForDeletion row.
			deleted, err := r.store.Reaction().DeleteOrphanedRowsByIds(row)
			if err != nil {
				return fmt.Errorf("failed to delete reactions of deleted posts: %w", err)
			}
			r.add(JobDataReactionsDeleted, deleted)
		}
		if err := sleepCtx(r.ctx, r.pause); err != nil {
			return err
		}
	}
}

// deleteFilesForPosts permanently deletes the files attached to the given
// (already deleted) posts, both from the file store and the database.
func (r *run) deleteFilesForPosts(postIDs []string) error {
	rctx := request.EmptyContext(r.logger)
	for _, postID := range postIDs {
		infos, err := r.store.FileInfo().GetForPost(postID, true, true, false)
		if err != nil {
			var nfErr *store.ErrNotFound
			if errors.As(err, &nfErr) {
				continue
			}
			return fmt.Errorf("failed to get the files of post %s: %w", postID, err)
		}
		if len(infos) == 0 {
			continue
		}
		for _, info := range infos {
			r.removeFileContent(info)
		}
		if err := r.store.FileInfo().PermanentDeleteForPost(rctx, postID); err != nil {
			return fmt.Errorf("failed to delete the files of post %s: %w", postID, err)
		}
		r.add(JobDataFilesDeleted, int64(len(infos)))
	}
	return nil
}

// removeFileContent removes the file, its thumbnail and preview from the
// file store. Failures are logged and counted as warnings, they don't stop
// the job (the database record is deleted anyway).
func (r *run) removeFileContent(info *model.FileInfo) {
	if r.backend == nil {
		return
	}
	for _, path := range []string{info.Path, info.ThumbnailPath, info.PreviewPath} {
		if path == "" {
			continue
		}
		exists, err := r.backend.FileExists(path)
		if err != nil {
			r.warn("Unable to check a file before deleting it", mlog.String("file_id", info.Id), mlog.String("path", path), mlog.Err(err))
			continue
		}
		if !exists {
			continue
		}
		if err := r.backend.RemoveFile(path); err != nil {
			r.warn("Unable to delete a file from the file store", mlog.String("file_id", info.Id), mlog.String("path", path), mlog.Err(err))
		}
	}
}

// deleteOrphans removes rows referencing data which no longer exists.
func (r *run) deleteOrphans() error {
	limit := int(r.batchSize)
	cleaners := []struct {
		what string
		fn   func(limit int) (int64, error)
	}{
		{"thread memberships", r.store.Thread().DeleteOrphanedRows},
		{"channel member history", r.store.ChannelMemberHistory().DeleteOrphanedRows},
		{"retention policy teams/channels", r.store.RetentionPolicy().DeleteOrphanedRows},
		{"flagged post preferences", r.store.Preference().DeleteOrphanedRows},
	}
	for _, c := range cleaners {
		for {
			if err := r.ctx.Err(); err != nil {
				return err
			}
			deleted, err := c.fn(limit)
			if err != nil {
				return fmt.Errorf("failed to delete orphaned %s: %w", c.what, err)
			}
			r.add(JobDataOrphansDeleted, deleted)
			if deleted < int64(limit) {
				break
			}
			if err := sleepCtx(r.ctx, r.pause); err != nil {
				return err
			}
		}
	}
	return nil
}

// deleteFiles applies the global file retention policy: files created before
// the cutoff are removed from the file store and the database. Channel
// bookmark files are kept, as are files of pinned posts when
// PreservePinnedPosts is set.
func (r *run) deleteFiles() error {
	if r.fileCutoff <= 0 {
		return nil
	}
	rctx := request.EmptyContext(r.logger)
	kept := map[string]struct{}{}
	for {
		if err := r.ctx.Err(); err != nil {
			return err
		}
		perPage := int(r.batchSize) + len(kept)
		infos, err := r.store.FileInfo().GetWithOptions(0, perPage, &model.GetFileInfosOptions{
			IncludeDeleted: true,
			SortBy:         model.FileinfoSortByCreated,
		})
		if err != nil {
			return fmt.Errorf("failed to get the files to delete: %w", err)
		}

		candidates := make([]*model.FileInfo, 0, len(infos))
		reachedCutoff := false
		for _, info := range infos {
			if info.CreateAt >= r.fileCutoff {
				reachedCutoff = true
				break
			}
			if _, ok := kept[info.Id]; ok {
				continue
			}
			if info.CreatorId == model.BookmarkFileOwner {
				kept[info.Id] = struct{}{}
				continue
			}
			candidates = append(candidates, info)
		}

		if r.preservePinned {
			candidates, err = r.filterPinned(candidates, kept)
			if err != nil {
				return err
			}
		}

		for _, info := range candidates {
			r.removeFileContent(info)
			if err := r.store.FileInfo().PermanentDelete(rctx, info.Id); err != nil {
				return fmt.Errorf("failed to delete file %s: %w", info.Id, err)
			}
			r.add(JobDataFilesDeleted, 1)
		}

		// Files are sorted by creation time: once a file newer than the
		// cutoff was seen, or the page was not full, everything eligible was
		// processed. Otherwise the page only held files older than the
		// cutoff, each of which was either deleted or newly kept (the page
		// size accounts for the previously kept ones), so progress is made.
		if reachedCutoff || len(infos) < perPage {
			return nil
		}
		if err := sleepCtx(r.ctx, r.pause); err != nil {
			return err
		}
	}
}

// filterPinned removes the files attached to pinned (non deleted) posts from
// candidates and records them as kept.
func (r *run) filterPinned(candidates []*model.FileInfo, kept map[string]struct{}) ([]*model.FileInfo, error) {
	postIDs := make([]string, 0, len(candidates))
	for _, info := range candidates {
		if info.PostId != "" {
			postIDs = append(postIDs, info.PostId)
		}
	}
	if len(postIDs) == 0 {
		return candidates, nil
	}
	posts, err := r.store.Post().GetPostsByIds(postIDs)
	if err != nil {
		var nfErr *store.ErrNotFound
		if !errors.As(err, &nfErr) {
			return nil, fmt.Errorf("failed to get the posts of the files to delete: %w", err)
		}
	}
	pinned := map[string]bool{}
	for _, p := range posts {
		if p.IsPinned && p.DeleteAt == 0 {
			pinned[p.Id] = true
		}
	}
	out := candidates[:0]
	for _, info := range candidates {
		if pinned[info.PostId] {
			kept[info.Id] = struct{}{}
			continue
		}
		out = append(out, info)
	}
	return out, nil
}

// deleteSearchIndexes lets the active search engines drop the indexes which
// only contain data older than the global message cutoff.
func (r *run) deleteSearchIndexes() error {
	if r.messageCutoff <= 0 || r.j.deps.SearchEngines == nil {
		return nil
	}
	rctx := request.EmptyContext(r.logger)
	for _, engine := range r.j.deps.SearchEngines() {
		if engine == nil || !engine.IsActive() {
			continue
		}
		if appErr := engine.DataRetentionDeleteIndexes(rctx, time.UnixMilli(r.messageCutoff)); appErr != nil {
			r.warn("Unable to delete old search indexes", mlog.String("engine", engine.GetName()), mlog.Err(appErr))
		}
	}
	return nil
}

// runPlugins invokes the RunDataRetention hook of the plugins.
func (r *run) runPlugins() error {
	if r.j.deps.RunPluginsDataRetention == nil {
		return nil
	}
	deleted, err := r.j.deps.RunPluginsDataRetention(r.now, r.batchSize)
	r.add(JobDataPluginRecordsDeleted, deleted)
	if err != nil {
		r.warn("Plugin data retention failed", mlog.Err(err))
	}
	return nil
}
