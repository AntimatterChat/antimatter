// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package messageexport

import (
	"bytes"
	"encoding/csv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app"
)

// channelExportFixture is a channel c1 whose posts the store returns by update
// time, in two pages of two:
//   - p2 (bob, a reply to p1, edited, a formula), updated at 160
//   - p1 (alice, with a file and a file deleted since), created at 110, after
//     p2 in the pages since a reaction updated it at 170
//   - p3 (a bot), created at 150 and updated at 180
func channelExportFixture(t *testing.T, since, until int64) *testStore {
	old := channelExportPageSize
	channelExportPageSize = 2
	t.Cleanup(func() { channelExportPageSize = old })

	p1 := messageExport("p1", "c1", model.ChannelTypeOpen, "u1", "alice", 110, 170, "hello world")
	p1.PostFileIds = model.StringArray{"f1", "f2"}
	p2 := messageExport("p2", "c1", model.ChannelTypeOpen, "u2", "bob", 120, 160, "=1+1")
	p2.PostEditAt = ptr(int64(160))
	p2.PostRootId = ptr("p1")
	p3 := messageExport("p3", "c1", model.ChannelTypeOpen, "b1", "helper", 150, 180, "beep")
	p3.IsBot = true

	first := model.MessageExportCursor{LastPostUpdateAt: since, ChannelId: "c1", SinceCreateAt: since, UntilCreateAt: until, ExcludeDeleted: true}
	second := first
	second.LastPostUpdateAt, second.LastPostId = 170, "p1"

	s := newTestStore()
	s.ComplianceStore.On("MessageExport", mock.Anything, first, 2).Return([]*model.MessageExport{p2, p1}, second, nil)
	s.ComplianceStore.On("MessageExport", mock.Anything, second, 2).Return([]*model.MessageExport{p3}, second, nil)
	s.FileInfoStore.On("GetByIds", []string{"f1", "f2"}, true, false, false).Return([]*model.FileInfo{
		{Id: "f1", PostId: "p1", Name: "report.txt", Path: "20260101/teams/t/channels/c1/users/u1/f1/report.txt"},
		{Id: "f2", PostId: "p1", Name: "gone.png", DeleteAt: 200},
	}, nil)
	return s
}

func exportChannelRows(t *testing.T, s *testStore, params app.ChannelExportParams) ([][]string, int) {
	var buf bytes.Buffer
	count, err := ExportChannelCSV(request.TestContext(t), s, params, &buf)
	require.NoError(t, err)
	rows, err := csv.NewReader(&buf).ReadAll()
	require.NoError(t, err)
	return rows, count
}

func TestExportChannelCSV(t *testing.T) {
	t.Run("the channel's messages, oldest first", func(t *testing.T) {
		s := channelExportFixture(t, 100, 1000)
		rows, count := exportChannelRows(t, s, app.ChannelExportParams{ChannelID: "c1", Since: 100, Until: 1000, IncludeEmails: true})
		assert.Equal(t, 3, count)
		require.Len(t, rows, 4)
		assert.Equal(t, channelExportHeader, rows[0])

		at := func(ms int64) string { return time.UnixMilli(ms).UTC().Format(time.RFC3339) }
		assert.Equal(t, []string{at(110), "p1", "", "u1", "alice", "alice@example.com", "user", "hello world", "report.txt", ""}, rows[1])
		assert.Equal(t, []string{at(120), "p2", "p1", "u2", "bob", "bob@example.com", "user", "'=1+1", "", at(160)}, rows[2])
		assert.Equal(t, []string{at(150), "p3", "", "b1", "helper", "helper@example.com", "bot", "beep", "", ""}, rows[3])
	})

	t.Run("without the emails when they're hidden", func(t *testing.T) {
		s := channelExportFixture(t, 0, 0)
		rows, _ := exportChannelRows(t, s, app.ChannelExportParams{ChannelID: "c1"})
		for _, row := range rows[1:] {
			assert.Empty(t, row[5])
		}
	})

	t.Run("too many messages", func(t *testing.T) {
		s := channelExportFixture(t, 0, 0)
		var buf bytes.Buffer
		_, err := ExportChannelCSV(request.TestContext(t), s, app.ChannelExportParams{ChannelID: "c1", MaxPosts: 2}, &buf)
		assert.ErrorIs(t, err, app.ErrChannelExportTooLarge)
		assert.Zero(t, buf.Len(), "nothing is written")
	})

	t.Run("an empty channel has the header only", func(t *testing.T) {
		s := newTestStore()
		s.ComplianceStore.On("MessageExport", mock.Anything, mock.Anything, mock.Anything).Return([]*model.MessageExport{}, model.MessageExportCursor{}, nil)
		rows, count := exportChannelRows(t, s, app.ChannelExportParams{ChannelID: "c1"})
		assert.Zero(t, count)
		assert.Equal(t, [][]string{channelExportHeader}, rows)
	})
}
