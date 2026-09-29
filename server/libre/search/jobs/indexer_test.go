// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package jobs

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/channels/store/storetest/mocks"
	"github.com/mattermost/mattermost/server/v8/libre/search/engine"
)

// fakeJobServer keeps jobs in memory.
type fakeJobServer struct {
	t      *testing.T
	cfg    *model.Config
	mu     sync.Mutex
	jobs   map[string]*model.Job
	events []string
	// onProgress is called on every progress update.
	onProgress func(job *model.Job)
}

func newFakeJobServer(t *testing.T) *fakeJobServer {
	cfg := &model.Config{}
	cfg.SetDefaults()
	cfg.ElasticsearchSettings.EnableIndexing = model.NewPointer(true)
	return &fakeJobServer{t: t, cfg: cfg, jobs: map[string]*model.Job{}}
}

func (s *fakeJobServer) add(job *model.Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	clone := *job
	s.jobs[job.Id] = &clone
}

func (s *fakeJobServer) get(id string) *model.Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	clone := *s.jobs[id]
	return &clone
}

func (s *fakeJobServer) setStatus(id, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[id].Status = status
}

func (s *fakeJobServer) Config() *model.Config    { return s.cfg }
func (s *fakeJobServer) Logger() mlog.LoggerIFace { return mlog.CreateConsoleTestLogger(s.t) }

func (s *fakeJobServer) ClaimJob(job *model.Job) (*model.Job, *model.AppError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored := s.jobs[job.Id]
	if stored == nil || stored.Status != model.JobStatusPending {
		return nil, nil
	}
	stored.Status = model.JobStatusInProgress
	clone := *stored
	clone.Data = copyData(stored.Data)
	return &clone, nil
}

func copyData(data model.StringMap) model.StringMap {
	out := model.StringMap{}
	for k, v := range data {
		out[k] = v
	}
	return out
}

func (s *fakeJobServer) GetJob(rctx request.CTX, id string) (*model.Job, *model.AppError) {
	return s.get(id), nil
}

func (s *fakeJobServer) GetJobsByTypeAndStatus(rctx request.CTX, jobType string, status string) ([]*model.Job, *model.AppError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*model.Job
	for _, j := range s.jobs {
		if j.Type == jobType && j.Status == status {
			clone := *j
			out = append(out, &clone)
		}
	}
	return out, nil
}

func (s *fakeJobServer) SetJobProgress(job *model.Job, progress int64) *model.AppError {
	s.mu.Lock()
	stored := s.jobs[job.Id]
	if stored.Status == model.JobStatusInProgress {
		stored.Progress = progress
		stored.Data = copyData(job.Data)
		stored.LastActivityAt = model.GetMillis()
	}
	s.events = append(s.events, "progress")
	s.mu.Unlock()
	if s.onProgress != nil {
		s.onProgress(job)
	}
	return nil
}

func (s *fakeJobServer) finish(job *model.Job, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.Id].Status = status
	s.events = append(s.events, status)
}

func (s *fakeJobServer) SetJobSuccess(job *model.Job) *model.AppError {
	s.finish(job, model.JobStatusSuccess)
	return nil
}

func (s *fakeJobServer) SetJobError(job *model.Job, jobError *model.AppError) *model.AppError {
	s.mu.Lock()
	s.jobs[job.Id].Data = copyData(job.Data)
	if jobError != nil {
		s.jobs[job.Id].Data["error"] = jobError.Id
	}
	s.mu.Unlock()
	s.finish(job, model.JobStatusError)
	return nil
}

func (s *fakeJobServer) SetJobPending(job *model.Job) *model.AppError {
	s.finish(job, model.JobStatusPending)
	return nil
}

func (s *fakeJobServer) SetJobCanceled(job *model.Job) *model.AppError {
	s.finish(job, model.JobStatusCanceled)
	return nil
}

// fakeIndexer records the bulk writes.
type fakeIndexer struct {
	*engine.Engine
	mu      sync.Mutex
	actions []engine.BulkAction
	failIDs map[string]bool
}

func newFakeIndexer() *fakeIndexer {
	cfg := &model.Config{}
	cfg.SetDefaults()
	cfg.ElasticsearchSettings.IndexPrefix = model.NewPointer("mm_")
	return &fakeIndexer{Engine: engine.New(cfg, nil, nil), failIDs: map[string]bool{}}
}

func (f *fakeIndexer) IsActive() bool { return true }

func (f *fakeIndexer) BulkIndex(ctx context.Context, actions []engine.BulkAction) ([]engine.BulkFailure, *model.AppError) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var failures []engine.BulkFailure
	for _, a := range actions {
		if f.failIDs[a.ID] {
			failures = append(failures, engine.BulkFailure{ID: a.ID, Index: a.Index, Status: 400})
			continue
		}
		f.actions = append(f.actions, a)
	}
	return failures, nil
}

func (f *fakeIndexer) ids(op string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var ids []string
	for _, a := range f.actions {
		if a.Op == op {
			ids = append(ids, a.ID)
		}
	}
	return ids
}

func toMap(t *testing.T, v any) map[string]any {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(data, &out))
	return out
}

type storeMocks struct {
	store    *mocks.Store
	posts    *mocks.PostStore
	channels *mocks.ChannelStore
	users    *mocks.UserStore
	files    *mocks.FileInfoStore
}

func newStoreMocks() *storeMocks {
	m := &storeMocks{
		store:    &mocks.Store{},
		posts:    &mocks.PostStore{},
		channels: &mocks.ChannelStore{},
		users:    &mocks.UserStore{},
		files:    &mocks.FileInfoStore{},
	}
	m.store.On("Post").Return(m.posts)
	m.store.On("Channel").Return(m.channels)
	m.store.On("User").Return(m.users)
	m.store.On("FileInfo").Return(m.files)
	return m
}

func post(id string, createAt int64, typ string, deleteAt int64) *model.PostForIndexing {
	return &model.PostForIndexing{Post: model.Post{Id: id, CreateAt: createAt, Type: typ, DeleteAt: deleteAt, ChannelId: "c1", Message: "m"}, TeamId: "t1", ChannelType: "O"}
}

func TestIndexerFullRun(t *testing.T) {
	js := newFakeJobServer(t)
	js.cfg.ElasticsearchSettings.BatchSize = model.NewPointer(2)
	m := newStoreMocks()
	idx := newFakeIndexer()

	m.posts.On("GetOldestEntityCreationTime").Return(int64(100), nil)

	// Channels: a private one needs its members, a DM is skipped.
	m.channels.On("GetChannelsBatchForIndexing", int64(99), "", 2).Return([]*model.Channel{
		{Id: "ch1", CreateAt: 100, Type: model.ChannelTypeOpen, TeamId: "t1"},
		{Id: "ch2", CreateAt: 110, Type: model.ChannelTypePrivate, TeamId: "t1"},
	}, nil)
	m.channels.On("GetChannelsBatchForIndexing", int64(110), "ch2", 2).Return([]*model.Channel{
		{Id: "dm", CreateAt: 120, Type: model.ChannelTypeDirect},
	}, nil)
	m.channels.On("GetAllChannelMemberIdsByChannelId", "ch2").Return([]string{"u1"}, nil)

	// Users: the batch is completed with profiles and all memberships.
	m.users.On("GetUsersBatchForIndexing", int64(99), "", 2).Return([]*model.UserForIndexing{
		{Id: "u2", CreateAt: 100, Username: "b", TeamsIds: []string{"t1"}, ChannelsIds: []string{"ch1"}},
		{Id: "u1", CreateAt: 100, Username: "a", TeamsIds: []string{"t1"}, ChannelsIds: []string{"ch1"}},
	}, nil)
	m.users.On("GetUsersBatchForIndexing", int64(100), "u2", 2).Return([]*model.UserForIndexing{}, nil)
	m.users.On("GetProfileByIds", mock.Anything, []string{"u2", "u1"}, (*store.UserGetByIdsOpts)(nil), false).Return([]*model.User{
		{Id: "u1", Username: "a", Email: "a@example.com"},
		{Id: "u2", Username: "b", Email: "b@example.com"},
	}, nil)
	m.channels.On("GetAllChannelMembersForUser", mock.Anything, "u1", false, true).Return(map[string]string{"ch1": "", "ch2": ""}, nil)
	m.channels.On("GetAllChannelMembersForUser", mock.Anything, "u2", false, true).Return(map[string]string{"ch1": ""}, nil)

	// Posts: system messages are skipped, deleted posts removed, the end
	// time stops the job.
	m.posts.On("GetPostsBatchForIndexing", int64(99), "", 2).Return([]*model.PostForIndexing{
		post("p1", 100, "", 0),
		post("p2", 101, model.PostTypeJoinChannel, 0),
	}, nil)
	m.posts.On("GetPostsBatchForIndexing", int64(101), "p2", 2).Return([]*model.PostForIndexing{
		post("p3", 102, "", 5),
		post("p4", 5000, "", 0),
	}, nil)

	m.files.On("GetFilesBatchForIndexing", int64(99), "", true, 2).Return([]*model.FileForIndexing{
		{FileInfo: model.FileInfo{Id: "f1", CreateAt: 100, PostId: "p1", Name: "a.txt"}, ChannelId: "c1"},
		{FileInfo: model.FileInfo{Id: "f2", CreateAt: 101}, ChannelId: "c1"},
	}, nil)
	m.files.On("GetFilesBatchForIndexing", int64(101), "f2", true, 2).Return([]*model.FileForIndexing{}, nil)

	w := NewIndexerWorker(js, m.store, func() BulkIndexer { return idx })
	job := &model.Job{Id: model.NewId(), Type: model.JobTypeElasticsearchPostIndexing, Status: model.JobStatusPending, Data: model.StringMap{dataEndTime: "1000"}}
	js.add(job)

	w.doJob(job, make(chan struct{}))

	final := js.get(job.Id)
	assert.Equal(t, model.JobStatusSuccess, final.Status)
	assert.Equal(t, []string{"ch1", "ch2", "u2", "u1", "p1", "f1"}, idx.ids("index"))
	assert.Equal(t, []string{"p3", "f2"}, idx.ids("delete"))

	// Private channel members and user memberships.
	for _, a := range idx.actions {
		switch a.ID {
		case "u1":
			doc := toMap(t, a.Doc)
			assert.Equal(t, "a@example.com", doc["email"])
			assert.ElementsMatch(t, []any{"ch1", "ch2"}, doc["channel_ids"])
		case "ch2":
			assert.Equal(t, []any{"u1"}, toMap(t, a.Doc)["user_ids"])
		case "p1":
			assert.Equal(t, "mm_posts_1970_01_01", a.Index)
		}
	}
	assert.Equal(t, "true", final.Data["posts_done"])
	assert.Equal(t, "102", final.Data["posts_last_time"])
	assert.Equal(t, "2", final.Data["users_indexed"])
	m.store.AssertExpectations(t)
}

func TestIndexerOnlyChannelsAndFailures(t *testing.T) {
	js := newFakeJobServer(t)
	m := newStoreMocks()
	idx := newFakeIndexer()
	idx.failIDs["ch1"] = true

	m.channels.On("GetChannelsBatchForIndexing", int64(0), "", 1000).Return([]*model.Channel{
		{Id: "ch1", CreateAt: 10, Type: model.ChannelTypeOpen},
		{Id: "ch2", CreateAt: 20, Type: model.ChannelTypeOpen},
	}, nil)

	w := NewIndexerWorker(js, m.store, func() BulkIndexer { return idx })
	job := &model.Job{Id: model.NewId(), Type: model.JobTypeElasticsearchPostIndexing, Status: model.JobStatusPending, Data: model.StringMap{
		"index_posts": "false", "index_users": "false", "index_files": "false", "index_channels": "true",
		dataStartTime: "0", dataEndTime: "1000", "sub_type": "channels_index_rebuild",
	}}
	js.add(job)
	w.doJob(job, make(chan struct{}))

	final := js.get(job.Id)
	assert.Equal(t, model.JobStatusError, final.Status)
	assert.Equal(t, "ent.elasticsearch.indexer.do_job.bulk_failures.error", final.Data["error"])
	assert.Equal(t, "1", final.Data[dataFailedDocuments])
	assert.Equal(t, []string{"ch2"}, idx.ids("index"))
	m.posts.AssertNotCalled(t, "GetPostsBatchForIndexing", mock.Anything, mock.Anything, mock.Anything)
}

func TestIndexerStopAndResume(t *testing.T) {
	js := newFakeJobServer(t)
	js.cfg.ElasticsearchSettings.BatchSize = model.NewPointer(1)
	m := newStoreMocks()
	idx := newFakeIndexer()

	m.posts.On("GetPostsBatchForIndexing", int64(0), "", 1).Return([]*model.PostForIndexing{post("p1", 10, "", 0)}, nil).Once()
	m.posts.On("GetPostsBatchForIndexing", int64(10), "p1", 1).Return([]*model.PostForIndexing{post("p2", 20, "", 0)}, nil).Once()
	m.posts.On("GetPostsBatchForIndexing", int64(20), "p2", 1).Return([]*model.PostForIndexing{}, nil).Once()

	stopCh := make(chan struct{})
	js.onProgress = func(job *model.Job) {
		// Interrupt the job after the first batch.
		select {
		case <-stopCh:
		default:
			close(stopCh)
		}
	}

	w := NewIndexerWorker(js, m.store, func() BulkIndexer { return idx })
	job := &model.Job{Id: model.NewId(), Type: model.JobTypeElasticsearchPostIndexing, Status: model.JobStatusPending, Data: model.StringMap{
		"index_users": "false", "index_files": "false", "index_channels": "false", dataStartTime: "0", dataEndTime: "1000",
	}}
	js.add(job)
	w.doJob(job, stopCh)

	stored := js.get(job.Id)
	assert.Equal(t, model.JobStatusPending, stored.Status)
	assert.Equal(t, "10", stored.Data["posts_last_time"])
	assert.Equal(t, "p1", stored.Data["posts_last_id"])
	assert.Equal(t, []string{"p1"}, idx.ids("index"))

	// Resume from the saved position.
	js.onProgress = nil
	w.doJob(stored, make(chan struct{}))
	assert.Equal(t, model.JobStatusSuccess, js.get(job.Id).Status)
	assert.Equal(t, []string{"p1", "p2"}, idx.ids("index"))
	m.posts.AssertExpectations(t)
}

func TestIndexerCancel(t *testing.T) {
	js := newFakeJobServer(t)
	js.cfg.ElasticsearchSettings.BatchSize = model.NewPointer(1)
	m := newStoreMocks()
	idx := newFakeIndexer()
	m.posts.On("GetPostsBatchForIndexing", int64(0), "", 1).Return([]*model.PostForIndexing{post("p1", 10, "", 0)}, nil).Once()

	job := &model.Job{Id: model.NewId(), Type: model.JobTypeElasticsearchPostIndexing, Status: model.JobStatusPending, Data: model.StringMap{
		"index_users": "false", "index_files": "false", "index_channels": "false", dataStartTime: "0", dataEndTime: "1000",
	}}
	js.add(job)
	js.onProgress = func(*model.Job) { js.setStatus(job.Id, model.JobStatusCancelRequested) }

	w := NewIndexerWorker(js, m.store, func() BulkIndexer { return idx })
	w.doJob(job, make(chan struct{}))
	assert.Equal(t, model.JobStatusCanceled, js.get(job.Id).Status)
}

func TestIndexerEngineNotStarted(t *testing.T) {
	js := newFakeJobServer(t)
	m := newStoreMocks()
	w := NewIndexerWorker(js, m.store, func() BulkIndexer { return nil })
	job := &model.Job{Id: model.NewId(), Type: model.JobTypeElasticsearchPostIndexing, Status: model.JobStatusPending}
	js.add(job)
	w.doJob(job, make(chan struct{}))
	final := js.get(job.Id)
	assert.Equal(t, model.JobStatusError, final.Status)
	assert.Equal(t, "ent.elasticsearch.not_started.error", final.Data["error"])
}

func TestIndexerWorkerRunStopAndStaleJobs(t *testing.T) {
	js := newFakeJobServer(t)
	m := newStoreMocks()
	idx := newFakeIndexer()
	m.posts.On("GetPostsBatchForIndexing", mock.Anything, mock.Anything, mock.Anything).Return([]*model.PostForIndexing{}, nil)

	stale := &model.Job{Id: model.NewId(), Type: model.JobTypeElasticsearchPostIndexing, Status: model.JobStatusInProgress, LastActivityAt: model.GetMillis() - time.Hour.Milliseconds()}
	fresh := &model.Job{Id: model.NewId(), Type: model.JobTypeElasticsearchPostIndexing, Status: model.JobStatusInProgress, LastActivityAt: model.GetMillis()}
	js.add(stale)
	js.add(fresh)

	w := NewIndexerWorker(js, m.store, func() BulkIndexer { return idx })
	assert.True(t, w.IsEnabled(js.cfg))
	go w.Run()

	require.Eventually(t, func() bool { return js.get(stale.Id).Status == model.JobStatusPending }, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, model.JobStatusInProgress, js.get(fresh.Id).Status)

	job := &model.Job{Id: model.NewId(), Type: model.JobTypeElasticsearchPostIndexing, Status: model.JobStatusPending, Data: model.StringMap{
		"index_users": "false", "index_files": "false", "index_channels": "false", dataStartTime: "0", dataEndTime: "1000",
	}}
	js.add(job)
	w.JobChannel() <- *job
	require.Eventually(t, func() bool { return js.get(job.Id).Status == model.JobStatusSuccess }, 5*time.Second, 10*time.Millisecond)

	w.Stop()
	w.Stop()
	// It can be started again after a stop (e.g. config change).
	go w.Run()
	require.Eventually(t, func() bool {
		w.stateMut.Lock()
		defer w.stateMut.Unlock()
		return !w.stopped
	}, 5*time.Second, 10*time.Millisecond)
	w.Stop()
}

type fakeAggregator struct {
	active bool
	cutoff time.Time
}

func (f *fakeAggregator) IsActive() bool { return f.active }

func (f *fakeAggregator) AggregatePostIndexes(ctx context.Context, cutoff time.Time) (int, *model.AppError) {
	f.cutoff = cutoff
	return 4, nil
}

func TestRunAggregation(t *testing.T) {
	cfg := &model.Config{}
	cfg.SetDefaults()
	cfg.ElasticsearchSettings.AggregatePostsAfterDays = model.NewPointer(30)
	logger := mlog.CreateConsoleTestLogger(t)

	agg := &fakeAggregator{active: true}
	job := &model.Job{}
	require.NoError(t, RunAggregation(cfg, agg, logger, job))
	assert.Equal(t, "4", job.Data["merged_indexes"])
	assert.WithinDuration(t, time.Now().AddDate(0, 0, -30), agg.cutoff, time.Minute)

	require.Error(t, RunAggregation(cfg, &fakeAggregator{}, logger, job))
	require.Error(t, RunAggregation(cfg, nil, logger, job))

	start, err := parseStartTime("03:15")
	require.NoError(t, err)
	assert.Equal(t, 3, start.Hour())
	assert.Equal(t, 15, start.Minute())
	_, err = parseStartTime("25:00")
	require.Error(t, err)
}
