// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package compliance

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/channels/store/storetest"
)

func newTestCompliance(t *testing.T, s *storetest.Store, cfg *model.Config, now time.Time) *Compliance {
	return New(Deps{
		Store:  func() store.Store { return s },
		Config: func() *model.Config { return cfg },
		Logger: mlog.CreateConsoleTestLogger(t),
		Now:    func() time.Time { return now },
	})
}

func testConfig(t *testing.T) *model.Config {
	cfg := &model.Config{}
	cfg.SetDefaults()
	*cfg.ComplianceSettings.Enable = true
	*cfg.ComplianceSettings.Directory = t.TempDir() + string(os.PathSeparator)
	*cfg.ComplianceSettings.BatchSize = 2
	return cfg
}

func readZip(t *testing.T, path string) map[string][]byte {
	r, err := zip.OpenReader(path)
	require.NoError(t, err)
	defer r.Close()
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

func TestRunComplianceJob(t *testing.T) {
	cfg := testConfig(t)
	s := &storetest.Store{}
	c := newTestCompliance(t, s, cfg, time.Now())

	job := &model.Compliance{
		Id:       model.NewId(),
		CreateAt: model.GetMillis(),
		Desc:     "test",
		Type:     model.ComplianceTypeAdhoc,
		StartAt:  1,
		EndAt:    model.GetMillis(),
	}

	posts := []*model.CompliancePost{
		{TeamName: "team", ChannelName: "town-square", UserUsername: "alice", PostId: "p1", PostCreateAt: 10, PostUpdateAt: 10, PostMessage: "hello"},
		{TeamName: "team", ChannelName: "town-square", UserUsername: "bob", PostId: "p2", PostCreateAt: 11, PostUpdateAt: 11, PostMessage: "=cmd"},
		{TeamName: "direct-messages", ChannelName: "a__b", UserUsername: "bob", PostId: "p3", PostCreateAt: 12, PostUpdateAt: 12, PostMessage: "dm", IsBot: true},
	}

	var statuses []string
	s.ComplianceStore.On("Update", mock.Anything).Run(func(args mock.Arguments) {
		statuses = append(statuses, args.Get(0).(*model.Compliance).Status)
	}).Return(job, nil)
	s.ComplianceStore.On("ComplianceExport", job, model.ComplianceExportCursor{}, 2).
		Return(posts[:2], model.ComplianceExportCursor{LastChannelsQueryPostCreateAt: 11, LastChannelsQueryPostID: "p2"}, nil).Once()
	s.ComplianceStore.On("ComplianceExport", job, model.ComplianceExportCursor{LastChannelsQueryPostCreateAt: 11, LastChannelsQueryPostID: "p2"}, 2).
		Return(posts[2:], model.ComplianceExportCursor{ChannelsQueryCompleted: true, DirectMessagesQueryCompleted: true}, nil).Once()

	appErr := c.RunComplianceJob(request.TestContext(t), job)
	require.Nil(t, appErr)

	assert.Equal(t, []string{model.ComplianceStatusRunning, model.ComplianceStatusFinished}, statuses)
	assert.Equal(t, 3, job.Count)

	path := ReportPath(cfg, job)
	assert.Equal(t, filepath.Join(*cfg.ComplianceSettings.Directory, "compliance", "adhoc-"+job.Id+".zip"), path)

	files := readZip(t, path)
	require.Contains(t, files, ReportPostsFileName)
	require.Contains(t, files, ReportMetadataFileName)

	rows, err := csv.NewReader(bytes.NewReader(files[ReportPostsFileName])).ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 4)
	assert.Equal(t, model.CompliancePostHeader(), rows[0])
	assert.Equal(t, "p1", rows[1][9])
	assert.Equal(t, "'=cmd", rows[2][15])
	assert.Equal(t, "bot", rows[3][8])

	var meta reportMetadata
	require.NoError(t, json.Unmarshal(files[ReportMetadataFileName], &meta))
	assert.Equal(t, 3, meta.Count)
	assert.Equal(t, job.Id, meta.Id)

	// No temporary file should be left behind.
	entries, err := os.ReadDir(filepath.Dir(path))
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestRunComplianceJobFailure(t *testing.T) {
	cfg := testConfig(t)
	s := &storetest.Store{}
	c := newTestCompliance(t, s, cfg, time.Now())

	job := &model.Compliance{Id: model.NewId(), CreateAt: 1, Desc: "x", Type: model.ComplianceTypeAdhoc, StartAt: 1, EndAt: 2}

	var lastStatus string
	s.ComplianceStore.On("Update", mock.Anything).Run(func(args mock.Arguments) {
		lastStatus = args.Get(0).(*model.Compliance).Status
	}).Return(job, nil)
	s.ComplianceStore.On("ComplianceExport", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, model.ComplianceExportCursor{}, errors.New("db down"))

	appErr := c.RunComplianceJob(request.TestContext(t), job)
	require.NotNil(t, appErr)
	assert.Equal(t, "ent.compliance.run_failed.error", appErr.Id)
	assert.Equal(t, model.ComplianceStatusFailed, lastStatus)

	_, err := os.Stat(ReportPath(cfg, job))
	assert.True(t, os.IsNotExist(err))
}

func TestRunDailyReportIfNeeded(t *testing.T) {
	now := time.Date(2026, 3, 10, 3, 15, 0, 0, time.Local)
	todayStart := time.Date(2026, 3, 10, 0, 0, 0, 0, time.Local)

	t.Run("disabled", func(t *testing.T) {
		cfg := testConfig(t)
		s := &storetest.Store{}
		c := newTestCompliance(t, s, cfg, now)
		c.RunDailyReportIfNeeded()
		s.ComplianceStore.AssertNotCalled(t, "GetAll", mock.Anything, mock.Anything)
	})

	t.Run("already generated", func(t *testing.T) {
		cfg := testConfig(t)
		*cfg.ComplianceSettings.EnableDaily = true
		s := &storetest.Store{}
		c := newTestCompliance(t, s, cfg, now)
		s.ComplianceStore.On("GetAll", 0, dailyLookupPageSize).Return(model.Compliances{
			{Type: model.ComplianceTypeAdhoc, Desc: "x", CreateAt: todayStart.UnixMilli() + 10},
			{Type: model.ComplianceTypeDaily, Desc: "2026-03-09", CreateAt: todayStart.UnixMilli() + 5, Status: model.ComplianceStatusFinished},
		}, nil)
		c.RunDailyReportIfNeeded()
		s.ComplianceStore.AssertNotCalled(t, "Save", mock.Anything)
	})

	t.Run("generates report for yesterday", func(t *testing.T) {
		cfg := testConfig(t)
		*cfg.ComplianceSettings.EnableDaily = true
		s := &storetest.Store{}
		c := newTestCompliance(t, s, cfg, now)
		s.ComplianceStore.On("GetAll", 0, dailyLookupPageSize).Return(model.Compliances{
			{Type: model.ComplianceTypeDaily, Desc: "2026-03-08", CreateAt: todayStart.UnixMilli() - 1000},
		}, nil)

		var saved *model.Compliance
		s.ComplianceStore.On("Save", mock.Anything).Return(func(c *model.Compliance) (*model.Compliance, error) {
			c.PreSave()
			saved = c
			return c, nil
		})
		s.ComplianceStore.On("Update", mock.Anything).Return(nil, nil)
		s.ComplianceStore.On("ComplianceExport", mock.Anything, mock.Anything, mock.Anything).
			Return([]*model.CompliancePost{}, model.ComplianceExportCursor{ChannelsQueryCompleted: true, DirectMessagesQueryCompleted: true}, nil)

		c.RunDailyReportIfNeeded()

		require.NotNil(t, saved)
		assert.Equal(t, model.ComplianceTypeDaily, saved.Type)
		assert.Equal(t, "2026-03-09", saved.Desc)
		assert.Equal(t, todayStart.AddDate(0, 0, -1).UnixMilli(), saved.StartAt)
		assert.Equal(t, todayStart.UnixMilli(), saved.EndAt)
		assert.Equal(t, model.ComplianceStatusFinished, saved.Status)

		_, err := os.Stat(ReportPath(cfg, saved))
		require.NoError(t, err)
	})
}
