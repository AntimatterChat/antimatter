// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package messageexport

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/channels/store/storetest"
	"github.com/mattermost/mattermost/server/v8/platform/shared/filestore"
)

// fakeJobStore is a minimal in-memory store.JobStore.
type fakeJobStore struct {
	store.JobStore
	mut  sync.Mutex
	jobs map[string]*model.Job
}

func newFakeJobStore() *fakeJobStore {
	return &fakeJobStore{jobs: map[string]*model.Job{}}
}

func copyJob(j *model.Job) *model.Job {
	c := *j
	c.Data = model.StringMap{}
	for k, v := range j.Data {
		c.Data[k] = v
	}
	return &c
}

func (s *fakeJobStore) Save(job *model.Job) (*model.Job, error) {
	s.mut.Lock()
	defer s.mut.Unlock()
	s.jobs[job.Id] = copyJob(job)
	return job, nil
}

func (s *fakeJobStore) Get(_ request.CTX, id string) (*model.Job, error) {
	s.mut.Lock()
	defer s.mut.Unlock()
	j, ok := s.jobs[id]
	if !ok {
		return nil, store.NewErrNotFound("Job", id)
	}
	return copyJob(j), nil
}

func (s *fakeJobStore) UpdateOptimistically(job *model.Job, currentStatus string) (*model.Job, error) {
	s.mut.Lock()
	defer s.mut.Unlock()
	j, ok := s.jobs[job.Id]
	if !ok || j.Status != currentStatus {
		return nil, nil
	}
	s.jobs[job.Id] = copyJob(job)
	return copyJob(job), nil
}

func (s *fakeJobStore) UpdateStatus(id string, status string) (*model.Job, error) {
	s.mut.Lock()
	defer s.mut.Unlock()
	j, ok := s.jobs[id]
	if !ok {
		return nil, store.NewErrNotFound("Job", id)
	}
	j.Status = status
	return copyJob(j), nil
}

func (s *fakeJobStore) UpdateStatusOptimistically(id string, currentStatus string, newStatus string) (*model.Job, error) {
	s.mut.Lock()
	defer s.mut.Unlock()
	j, ok := s.jobs[id]
	if !ok || j.Status != currentStatus {
		return nil, nil
	}
	j.Status = newStatus
	return copyJob(j), nil
}

func (s *fakeJobStore) GetAllByTypePage(_ request.CTX, jobType string, offset int, limit int) ([]*model.Job, error) {
	s.mut.Lock()
	defer s.mut.Unlock()
	var list []*model.Job
	for _, j := range s.jobs {
		if j.Type == jobType {
			list = append(list, copyJob(j))
		}
	}
	sort.Slice(list, func(i, k int) bool { return list[i].CreateAt > list[k].CreateAt })
	if offset >= len(list) {
		return []*model.Job{}, nil
	}
	return list[offset:min(offset+limit, len(list))], nil
}

func (s *fakeJobStore) GetCountByStatusAndType(status string, jobType string) (int64, error) {
	s.mut.Lock()
	defer s.mut.Unlock()
	var n int64
	for _, j := range s.jobs {
		if j.Status == status && j.Type == jobType {
			n++
		}
	}
	return n, nil
}

// testStore combines the mock store with the in-memory job store.
type testStore struct {
	*storetest.Store
	jobs *fakeJobStore
}

func (s *testStore) Job() store.JobStore { return s.jobs }

func newTestStore() *testStore {
	return &testStore{Store: &storetest.Store{}, jobs: newFakeJobStore()}
}

// fakeConfigService implements configservice.ConfigService.
type fakeConfigService struct {
	cfg *model.Config
}

func (f *fakeConfigService) Config() *model.Config                                     { return f.cfg }
func (f *fakeConfigService) AddConfigListener(func(old, current *model.Config)) string { return "id" }
func (f *fakeConfigService) RemoveConfigListener(string)                               {}

func newTestConfig() *model.Config {
	cfg := &model.Config{}
	cfg.SetDefaults()
	*cfg.MessageExportSettings.EnableExport = true
	return cfg
}

func newLocalBackend(t *testing.T) filestore.FileBackend {
	b, err := filestore.NewFileBackend(filestore.FileBackendSettings{DriverName: model.ImageDriverLocal, Directory: t.TempDir()})
	require.NoError(t, err)
	return b
}

func readZipBytes(t *testing.T, data []byte) map[string][]byte {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	files := map[string][]byte{}
	for _, f := range r.File {
		rc, err := f.Open()
		require.NoError(t, err)
		b, err := io.ReadAll(rc)
		require.NoError(t, err)
		rc.Close()
		files[f.Name] = b
	}
	return files
}

func readBackendZip(t *testing.T, backend filestore.FileBackend, path string) map[string][]byte {
	data, err := backend.ReadFile(path)
	require.NoError(t, err)
	return readZipBytes(t, data)
}

// fakeSender records the delivered EMLs.
type fakeSender struct {
	mut    sync.Mutex
	sent   []sentEML
	closed bool
	err    error
}

type sentEML struct {
	from, to string
	data     []byte
}

func (f *fakeSender) Send(_ context.Context, from, to string, eml []byte) error {
	f.mut.Lock()
	defer f.mut.Unlock()
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, sentEML{from: from, to: to, data: eml})
	return nil
}

func (f *fakeSender) Close() error {
	f.closed = true
	return nil
}

func ptr[T any](v T) *T { return &v }

func messageExport(id, channelID string, channelType model.ChannelType, userID, username string, createAt, updateAt int64, message string) *model.MessageExport {
	m := &model.MessageExport{
		ChannelId:          ptr(channelID),
		ChannelName:        ptr("name-" + channelID),
		ChannelDisplayName: ptr("Display " + channelID),
		ChannelType:        ptr(channelType),
		UserId:             ptr(userID),
		UserEmail:          ptr(username + "@example.com"),
		Username:           ptr(username),
		PostId:             ptr(id),
		PostCreateAt:       ptr(createAt),
		PostUpdateAt:       ptr(updateAt),
		PostDeleteAt:       ptr(int64(0)),
		PostEditAt:         ptr(int64(0)),
		PostMessage:        ptr(message),
		PostType:           ptr(""),
		PostRootId:         ptr(""),
		PostProps:          ptr("{}"),
		PostOriginalId:     ptr(""),
		PostFileIds:        model.StringArray{},
	}
	if channelType == model.ChannelTypeOpen || channelType == model.ChannelTypePrivate {
		m.TeamId = ptr("team1")
		m.TeamName = ptr("team-one")
		m.TeamDisplayName = ptr("Team One")
	}
	return m
}
