// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package messageexport

import (
	"bytes"
	"encoding/csv"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// channelExportFixture is a channel c1 whose posts the store returns by
// creation time, in pages of two:
//   - p1 (alice, with a file and a file deleted since), created at 110
//   - p2 (bob, a reply to p1, edited, a formula), created at 120
//   - p3 (a bot), created at 150
//   - p4 (alice's webhook, posted as "CI" with message attachments), created
//     at 170
func channelExportFixture(t *testing.T, filter model.ChannelExportFilter, count int) *testStore {
	old := channelExportPageSize
	channelExportPageSize = 2
	t.Cleanup(func() { channelExportPageSize = old })

	p1 := &model.ChannelExportPost{PostId: "p1", PostCreateAt: 110, UserId: "u1", Username: "alice", UserEmail: "alice@example.com", PostMessage: "hello world", PostFileIds: model.StringArray{"f1", "f2"}}
	p2 := &model.ChannelExportPost{PostId: "p2", PostCreateAt: 120, PostEditAt: 160, PostRootId: "p1", UserId: "u2", Username: "bob", UserEmail: "bob@example.com", PostMessage: "=1+1"}
	p3 := &model.ChannelExportPost{PostId: "p3", PostCreateAt: 150, UserId: "b1", Username: "helper", UserEmail: "helper@example.com", IsBot: true, PostMessage: "beep"}
	p4 := &model.ChannelExportPost{PostId: "p4", PostCreateAt: 170, UserId: "u1", Username: "alice", UserEmail: "alice@example.com",
		FromWebhook: "true", OverrideUsername: "CI",
		Attachments: `[{"pretext":"Build","title":"#42","text":"passed"},{"fallback":"fallback only"},{"color":"#fff"}]`}

	s := newTestStore()
	s.ComplianceStore.On("ChannelExportCount", mock.Anything, filter, mock.Anything).Return(count, nil)
	s.ComplianceStore.On("ChannelExportPosts", mock.Anything, filter, model.ChannelExportCursor{}, 2).Return([]*model.ChannelExportPost{p1, p2}, nil)
	s.ComplianceStore.On("ChannelExportPosts", mock.Anything, filter, model.ChannelExportCursor{AfterCreateAt: 120, AfterId: "p2"}, 2).Return([]*model.ChannelExportPost{p3, p4}, nil)
	s.ComplianceStore.On("ChannelExportPosts", mock.Anything, filter, model.ChannelExportCursor{AfterCreateAt: 170, AfterId: "p4"}, 2).Return([]*model.ChannelExportPost{}, nil)
	s.FileInfoStore.On("GetByIds", []string{"f1", "f2"}, true, false, false).Return([]*model.FileInfo{
		{Id: "f1", PostId: "p1", Name: "report.txt", Path: "20260101/teams/t/channels/c1/users/u1/f1/report.txt"},
		{Id: "f2", PostId: "p1", Name: "gone.png", DeleteAt: 200},
	}, nil)
	return s
}

func exportChannel(t *testing.T, s *testStore, params ChannelExportParams) (*bytes.Buffer, int, error) {
	var buf bytes.Buffer
	count, err := ExportChannelCSV(request.TestContext(t), s, params, func() (io.Writer, error) { return &buf, nil })
	return &buf, count, err
}

func exportChannelRows(t *testing.T, s *testStore, params ChannelExportParams) ([][]string, int) {
	buf, count, err := exportChannel(t, s, params)
	require.NoError(t, err)
	rows, err := csv.NewReader(buf).ReadAll()
	require.NoError(t, err)
	return rows, count
}

func TestExportChannelCSV(t *testing.T) {
	filter := model.ChannelExportFilter{ChannelId: "c1", SinceCreateAt: 100, UntilCreateAt: 1000}
	params := ChannelExportParams{ChannelID: "c1", Since: 100, Until: 1000, MaxPosts: 10, IncludeEmails: true}

	t.Run("the channel's messages, oldest first", func(t *testing.T) {
		s := channelExportFixture(t, filter, 4)
		rows, count := exportChannelRows(t, s, params)
		assert.Equal(t, 4, count)
		require.Len(t, rows, 5)
		assert.Equal(t, channelExportHeader, rows[0])
		assert.Equal(t, "Post Creation Time (UTC)", rows[0][0])
		assert.Equal(t, "Edit Time (UTC)", rows[0][9])

		at := func(ms int64) string { return time.UnixMilli(ms).UTC().Format(time.RFC3339) }
		assert.Equal(t, []string{at(110), "p1", "", "u1", "alice", "alice@example.com", "user", "hello world", "report.txt", ""}, rows[1])
		assert.Equal(t, []string{at(120), "p2", "p1", "u2", "bob", "bob@example.com", "user", "'=1+1", "", at(160)}, rows[2])
		assert.Equal(t, []string{at(150), "p3", "", "b1", "helper", "helper@example.com", "bot", "beep", "", ""}, rows[3])
		// A webhook's post shows under the name it was posted with, with the
		// text of its message attachments.
		assert.Equal(t, []string{at(170), "p4", "", "u1", "CI", "alice@example.com", "user", "Build\n#42\npassed\n\nfallback only", "", ""}, rows[4])

		// The limit is counted, not read.
		s.ComplianceStore.AssertCalled(t, "ChannelExportCount", mock.Anything, filter, 11)
	})

	t.Run("without the emails when they're hidden", func(t *testing.T) {
		s := channelExportFixture(t, filter, 4)
		p := params
		p.IncludeEmails = false
		rows, _ := exportChannelRows(t, s, p)
		for _, row := range rows[1:] {
			assert.Empty(t, row[5])
		}
	})

	t.Run("too many messages: refused before reading any", func(t *testing.T) {
		s := channelExportFixture(t, filter, 3)
		p := params
		p.MaxPosts = 2
		opened := false
		_, err := ExportChannelCSV(request.TestContext(t), s, p, func() (io.Writer, error) {
			opened = true
			return io.Discard, nil
		})
		assert.ErrorIs(t, err, ErrChannelExportTooLarge)
		assert.False(t, opened, "nothing is written")
		s.ComplianceStore.AssertCalled(t, "ChannelExportCount", mock.Anything, filter, 3)
		s.ComplianceStore.AssertNotCalled(t, "ChannelExportPosts", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("posts added since they were counted stop at the limit", func(t *testing.T) {
		s := channelExportFixture(t, filter, 3)
		p := params
		p.MaxPosts = 3
		rows, count := exportChannelRows(t, s, p)
		assert.Equal(t, 3, count)
		assert.Len(t, rows, 4)
	})

	t.Run("the plugins' hooks see each page and can redact it", func(t *testing.T) {
		s := channelExportFixture(t, filter, 4)
		p := params
		var pages [][]string
		p.ConsumePosts = func(posts map[string]*model.Post) {
			var ids []string
			for id, post := range posts {
				ids = append(ids, id)
				if id == "p4" {
					assert.Equal(t, "true", post.GetProp(model.PostPropsFromWebhook))
					assert.Equal(t, "CI", post.GetProp(model.PostPropsOverrideUsername))
					require.Len(t, post.Attachments(), 3)
				}
			}
			pages = append(pages, ids)
			if post := posts["p1"]; post != nil {
				redacted := post.Clone()
				redacted.Message = "[redacted]"
				posts["p1"] = redacted
			}
			if post := posts["p4"]; post != nil {
				post.DelProp(model.PostPropsAttachments)
			}
		}
		rows, _ := exportChannelRows(t, s, p)
		require.Len(t, pages, 2)
		assert.ElementsMatch(t, []string{"p1", "p2"}, pages[0])
		assert.ElementsMatch(t, []string{"p3", "p4"}, pages[1])
		assert.Equal(t, "[redacted]", rows[1][7])
		assert.Equal(t, "", rows[4][7])
	})

	t.Run("an empty channel has the header only", func(t *testing.T) {
		s := newTestStore()
		s.ComplianceStore.On("ChannelExportCount", mock.Anything, mock.Anything, mock.Anything).Return(0, nil)
		s.ComplianceStore.On("ChannelExportPosts", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*model.ChannelExportPost{}, nil)
		rows, count := exportChannelRows(t, s, ChannelExportParams{ChannelID: "c1", MaxPosts: 10})
		assert.Zero(t, count)
		assert.Equal(t, [][]string{channelExportHeader}, rows)
	})
}

func TestExportCell(t *testing.T) {
	for in, out := range map[string]string{
		"":            "",
		"hello":       "hello",
		"=1+1":        "'=1+1",
		"+33 6":       "'+33 6",
		"- a list":    "'- a list",
		"@here":       "'@here",
		"\tTab":       "'\tTab",
		"  =1+1":      "'  =1+1",
		"\n@SUM(A1)":  "'\n@SUM(A1)",
		"＝1+1":        "'＝1+1",
		"＋1":          "'＋1",
		"－1":          "'－1",
		"＠here":       "'＠here",
		"  hello":     "  hello",
		"a=1":         "a=1",
		"日本語":         "日本語",
		"\r\nx":       "'\r\nx",
		"   ":         "   ",
		"user@domain": "user@domain",
	} {
		assert.Equal(t, out, exportCell(in), "%q", in)
	}
}
