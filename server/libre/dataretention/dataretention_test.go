// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package dataretention

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/channels/store/storetest"
	"github.com/mattermost/mattermost/server/v8/platform/shared/filestore"
)

func newTestConfig() *model.Config {
	cfg := &model.Config{}
	cfg.SetDefaults()
	return cfg
}

func newTestDataRetention(s *storetest.Store, cfg *model.Config) *DataRetention {
	d := New(func() store.Store { return s }, func() *model.Config { return cfg })
	d.now = func() int64 { return 1_000_000_000_000 }
	return d
}

func TestGetGlobalPolicy(t *testing.T) {
	cfg := newTestConfig()
	d := newTestDataRetention(&storetest.Store{}, cfg)

	policy, appErr := d.GetGlobalPolicy()
	require.Nil(t, appErr)
	assert.Equal(t, &model.GlobalRetentionPolicy{}, policy)

	*cfg.DataRetentionSettings.EnableMessageDeletion = true
	*cfg.DataRetentionSettings.MessageRetentionHours = 48
	*cfg.DataRetentionSettings.EnableFileDeletion = true
	*cfg.DataRetentionSettings.FileRetentionHours = 0
	*cfg.DataRetentionSettings.FileRetentionDays = 3

	policy, appErr = d.GetGlobalPolicy()
	require.Nil(t, appErr)
	assert.True(t, policy.MessageDeletionEnabled)
	assert.True(t, policy.FileDeletionEnabled)
	assert.Equal(t, int64(1_000_000_000_000-48*msPerHour), policy.MessageRetentionCutoff)
	assert.Equal(t, int64(1_000_000_000_000-72*msPerHour), policy.FileRetentionCutoff)
}

func TestCreatePolicy(t *testing.T) {
	cfg := newTestConfig()

	t.Run("validation", func(t *testing.T) {
		d := newTestDataRetention(&storetest.Store{}, cfg)
		for name, p := range map[string]*model.RetentionPolicyWithTeamAndChannelIDs{
			"no name":      {RetentionPolicy: model.RetentionPolicy{PostDurationDays: model.NewPointer(int64(10))}},
			"no duration":  {RetentionPolicy: model.RetentionPolicy{DisplayName: "p"}},
			"bad duration": {RetentionPolicy: model.RetentionPolicy{DisplayName: "p", PostDurationDays: model.NewPointer(int64(0))}},
			"bad team id":  {RetentionPolicy: model.RetentionPolicy{DisplayName: "p", PostDurationDays: model.NewPointer(int64(1))}, TeamIDs: []string{"nope"}},
		} {
			_, appErr := d.CreatePolicy(p)
			require.NotNil(t, appErr, name)
			assert.Equal(t, "ent.data_retention.policies.invalid_policy", appErr.Id, name)
			assert.Equal(t, http.StatusBadRequest, appErr.StatusCode, name)
		}
	})

	t.Run("success", func(t *testing.T) {
		s := &storetest.Store{}
		d := newTestDataRetention(s, cfg)
		teamID := model.NewId()
		expected := &model.RetentionPolicyWithTeamAndChannelCounts{RetentionPolicy: model.RetentionPolicy{ID: model.NewId(), DisplayName: "p"}, TeamCount: 1}
		s.RetentionPolicyStore.On("Save", mock.MatchedBy(func(p *model.RetentionPolicyWithTeamAndChannelIDs) bool {
			return p.ID == "" && len(p.TeamIDs) == 1 && p.TeamIDs[0] == teamID
		})).Return(expected, nil)

		policy, appErr := d.CreatePolicy(&model.RetentionPolicyWithTeamAndChannelIDs{
			RetentionPolicy: model.RetentionPolicy{ID: "ignored", DisplayName: "p", PostDurationDays: model.NewPointer(PostDurationForever)},
			TeamIDs:         []string{teamID, teamID},
		})
		require.Nil(t, appErr)
		assert.Equal(t, expected, policy)
	})

	t.Run("missing team", func(t *testing.T) {
		s := &storetest.Store{}
		d := newTestDataRetention(s, cfg)
		s.RetentionPolicyStore.On("Save", mock.Anything).Return(nil, store.NewErrNotFound("Team", "x"))
		_, appErr := d.CreatePolicy(&model.RetentionPolicyWithTeamAndChannelIDs{
			RetentionPolicy: model.RetentionPolicy{DisplayName: "p", PostDurationDays: model.NewPointer(int64(5))},
			TeamIDs:         []string{model.NewId()},
		})
		require.NotNil(t, appErr)
		assert.Equal(t, http.StatusBadRequest, appErr.StatusCode)
	})
}

func TestGetPolicyAndDelete(t *testing.T) {
	cfg := newTestConfig()
	s := &storetest.Store{}
	d := newTestDataRetention(s, cfg)
	missing := model.NewId()
	existing := model.NewId()

	s.RetentionPolicyStore.On("Get", missing).Return(nil, sql.ErrNoRows)
	s.RetentionPolicyStore.On("Get", existing).Return(&model.RetentionPolicyWithTeamAndChannelCounts{RetentionPolicy: model.RetentionPolicy{ID: existing}}, nil)
	s.RetentionPolicyStore.On("Delete", existing).Return(nil)

	_, appErr := d.GetPolicy(missing)
	require.NotNil(t, appErr)
	assert.Equal(t, http.StatusNotFound, appErr.StatusCode)

	appErr = d.DeletePolicy(missing)
	require.NotNil(t, appErr)
	assert.Equal(t, http.StatusNotFound, appErr.StatusCode)

	require.Nil(t, d.DeletePolicy(existing))

	s.RetentionPolicyStore.On("GetTeams", existing, 0, 10).Return([]*model.Team{{Id: "t"}}, nil)
	s.RetentionPolicyStore.On("GetTeamsCount", existing).Return(int64(1), nil)
	teams, appErr := d.GetTeamsForPolicy(existing, 0, 10)
	require.Nil(t, appErr)
	assert.Equal(t, int64(1), teams.TotalCount)
	assert.Len(t, teams.Teams, 1)
}

func TestPatchPolicy(t *testing.T) {
	cfg := newTestConfig()
	s := &storetest.Store{}
	d := newTestDataRetention(s, cfg)
	id := model.NewId()
	s.RetentionPolicyStore.On("Get", id).Return(&model.RetentionPolicyWithTeamAndChannelCounts{RetentionPolicy: model.RetentionPolicy{ID: id}}, nil)

	_, appErr := d.PatchPolicy(&model.RetentionPolicyWithTeamAndChannelIDs{RetentionPolicy: model.RetentionPolicy{ID: id, PostDurationDays: model.NewPointer(int64(-5))}})
	require.NotNil(t, appErr)
	assert.Equal(t, http.StatusBadRequest, appErr.StatusCode)

	s.RetentionPolicyStore.On("Patch", mock.Anything).Return(&model.RetentionPolicyWithTeamAndChannelCounts{RetentionPolicy: model.RetentionPolicy{ID: id, DisplayName: "new"}}, nil)
	policy, appErr := d.PatchPolicy(&model.RetentionPolicyWithTeamAndChannelIDs{RetentionPolicy: model.RetentionPolicy{ID: id, DisplayName: "new"}})
	require.Nil(t, appErr)
	assert.Equal(t, "new", policy.DisplayName)
}

func allDone() model.RetentionPolicyCursor {
	return model.RetentionPolicyCursor{ChannelPoliciesDone: true, TeamPoliciesDone: true, GlobalPoliciesDone: true}
}

func TestRetentionJobExecute(t *testing.T) {
	const now = int64(10_000_000_000)
	cfg := newTestConfig()
	*cfg.DataRetentionSettings.EnableMessageDeletion = true
	*cfg.DataRetentionSettings.MessageRetentionHours = 1
	*cfg.DataRetentionSettings.EnableFileDeletion = true
	*cfg.DataRetentionSettings.FileRetentionHours = 2
	*cfg.DataRetentionSettings.TimeBetweenBatchesMilliseconds = 0
	*cfg.DataRetentionSettings.BatchSize = 10

	backend, err := filestore.NewFileBackend(filestore.FileBackendSettings{DriverName: model.ImageDriverLocal, Directory: t.TempDir()})
	require.NoError(t, err)
	for _, p := range []string{"post/file.txt", "post/thumb.png", "old/file.txt", "bookmark/file.txt", "new/file.txt"} {
		_, err = backend.WriteFile(bytes.NewReader([]byte("x")), p)
		require.NoError(t, err)
	}

	s := &storetest.Store{}
	messageCutoff := now - msPerHour
	fileCutoff := now - 2*msPerHour

	// Posts: first batch hits the limit, second batch completes.
	s.PostStore.On("PermanentDeleteBatchForRetentionPolicies", model.RetentionPolicyBatchConfigs{Now: now, GlobalPolicyEndTime: messageCutoff, Limit: 10}, model.RetentionPolicyCursor{}).
		Return(int64(10), model.RetentionPolicyCursor{ChannelPoliciesDone: true}, nil).Once()
	s.PostStore.On("PermanentDeleteBatchForRetentionPolicies", mock.Anything, model.RetentionPolicyCursor{ChannelPoliciesDone: true}).
		Return(int64(3), allDone(), nil).Once()

	row := &model.RetentionIdsForDeletion{Id: "r1", TableName: "Posts", Ids: []string{"p1", "p2"}}
	s.RetentionPolicyStore.On("GetIdsForDeletionByTableName", "Posts", 100).Return([]*model.RetentionIdsForDeletion{row}, nil).Once()
	s.RetentionPolicyStore.On("GetIdsForDeletionByTableName", "Posts", 100).Return([]*model.RetentionIdsForDeletion{}, nil).Once()
	s.FileInfoStore.On("GetForPost", "p1", true, true, false).Return([]*model.FileInfo{{Id: "f1", PostId: "p1", Path: "post/file.txt", ThumbnailPath: "post/thumb.png"}}, nil)
	s.FileInfoStore.On("GetForPost", "p2", true, true, false).Return([]*model.FileInfo{}, nil)
	s.FileInfoStore.On("PermanentDeleteForPost", mock.Anything, "p1").Return(nil).Once()
	s.ReactionStore.On("DeleteOrphanedRowsByIds", row).Return(int64(4), nil)

	s.ThreadStore.On("PermanentDeleteBatchForRetentionPolicies", mock.Anything, mock.Anything).Return(int64(2), allDone(), nil)
	s.ThreadStore.On("PermanentDeleteBatchThreadMembershipsForRetentionPolicies", mock.Anything, mock.Anything).Return(int64(1), allDone(), nil)
	s.ChannelMemberHistoryStore.On("PermanentDeleteBatchForRetentionPolicies", mock.Anything, mock.Anything).Return(int64(0), allDone(), nil)

	s.ThreadStore.On("DeleteOrphanedRows", 10).Return(int64(0), nil)
	s.ChannelMemberHistoryStore.On("DeleteOrphanedRows", 10).Return(int64(1), nil)
	s.RetentionPolicyStore.On("DeleteOrphanedRows", 10).Return(int64(0), nil)
	s.PreferenceStore.On("DeleteOrphanedRows", 10).Return(int64(0), nil)

	s.FileInfoStore.On("GetWithOptions", 0, 10, mock.Anything).Return([]*model.FileInfo{
		{Id: "bm", CreatorId: model.BookmarkFileOwner, CreateAt: fileCutoff - 100, Path: "bookmark/file.txt"},
		{Id: "old", CreatorId: model.NewId(), CreateAt: fileCutoff - 50, Path: "old/file.txt"},
		{Id: "new", CreatorId: model.NewId(), CreateAt: fileCutoff + 1, Path: "new/file.txt"},
	}, nil).Once()
	s.FileInfoStore.On("PermanentDelete", mock.Anything, "old").Return(nil).Once()

	s.PostStore.On("ClearCaches").Return().Once()
	s.FileInfoStore.On("ClearCaches").Return().Once()

	var pluginCalled bool
	j := NewJob(JobDeps{
		Store:       func() store.Store { return s },
		Config:      func() *model.Config { return cfg },
		FileBackend: func() filestore.FileBackend { return backend },
		RunPluginsDataRetention: func(nowMillis, batchSize int64) (int64, error) {
			pluginCalled = true
			assert.Equal(t, now, nowMillis)
			assert.Equal(t, int64(10), batchSize)
			return 7, errors.New("plugin failed")
		},
		Now: func() int64 { return now },
	})

	job := &model.Job{Id: model.NewId(), Type: model.JobTypeDataRetention, Data: model.StringMap{}}
	require.NoError(t, j.execute(context.Background(), mlog.CreateConsoleTestLogger(t), job))

	s.AssertExpectations(t)
	assert.True(t, pluginCalled)
	assert.Equal(t, "13", job.Data[JobDataPostsDeleted])
	assert.Equal(t, "4", job.Data[JobDataReactionsDeleted])
	assert.Equal(t, "2", job.Data[JobDataFilesDeleted])
	assert.Equal(t, "2", job.Data[JobDataThreadsDeleted])
	assert.Equal(t, "1", job.Data[JobDataOrphansDeleted])
	assert.Equal(t, "7", job.Data[JobDataPluginRecordsDeleted])
	assert.Equal(t, "1", job.Data[JobDataWarnings])

	for path, shouldExist := range map[string]bool{
		"post/file.txt":     false,
		"post/thumb.png":    false,
		"old/file.txt":      false,
		"bookmark/file.txt": true,
		"new/file.txt":      true,
	} {
		exists, err := backend.FileExists(path)
		require.NoError(t, err)
		assert.Equal(t, shouldExist, exists, path)
	}
}

func TestRetentionJobCanceled(t *testing.T) {
	cfg := newTestConfig()
	s := &storetest.Store{}
	j := NewJob(JobDeps{
		Store:  func() store.Store { return s },
		Config: func() *model.Config { return cfg },
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	job := &model.Job{Id: model.NewId(), Data: model.StringMap{}}
	err := j.execute(ctx, mlog.CreateConsoleTestLogger(t), job)
	require.ErrorIs(t, err, context.Canceled)
}

func TestDeletionJobStartTime(t *testing.T) {
	cfg := newTestConfig()
	*cfg.DataRetentionSettings.DeletionJobStartTime = "13:45"
	st := deletionJobStartTime(cfg)
	assert.Equal(t, 13, st.Hour())
	assert.Equal(t, 45, st.Minute())

	*cfg.DataRetentionSettings.DeletionJobStartTime = "garbage"
	st = deletionJobStartTime(cfg)
	assert.Equal(t, 2, st.Hour())
}
