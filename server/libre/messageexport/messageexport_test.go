// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package messageexport

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/jobs"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/libre/messageexport/shared"
	"github.com/mattermost/mattermost/server/v8/platform/shared/filestore"
)

const (
	testJobEnd     = int64(1000)
	testExportFrom = int64(100)
)

var testNow = time.UnixMilli(testJobEnd)

// exportFixture sets up a store with 3 posts in 2 batches (batch size 2):
//   - p1 (c1, public, alice, with a file) and p2 (c1, bob, edited)
//   - p3 (d1, direct message, bob)
//
// and a private channel c3 with only membership activity in the first batch.
type exportFixture struct {
	store         *testStore
	cfg           *model.Config
	fileBackend   filestore.FileBackend
	exportBackend filestore.FileBackend
	job           *Job
	sender        *fakeSender
}

func newExportFixture(t *testing.T, exportType string) *exportFixture {
	f := &exportFixture{
		store:         newTestStore(),
		cfg:           newTestConfig(),
		fileBackend:   newLocalBackend(t),
		exportBackend: newLocalBackend(t),
		sender:        &fakeSender{},
	}
	*f.cfg.MessageExportSettings.ExportFormat = exportType
	*f.cfg.MessageExportSettings.BatchSize = 2
	*f.cfg.MessageExportSettings.ExportFromTimestamp = testExportFrom
	*f.cfg.MessageExportSettings.GlobalRelaySettings.EmailAddress = "archive@globalrelay.example.com"
	*f.cfg.MessageExportSettings.GlobalRelaySettings.CustomHeaderName = "X-Archive-Type"
	*f.cfg.MessageExportSettings.GlobalRelaySettings.CustomHeaderValue = "Chat"

	_, err := f.fileBackend.WriteFile(bytes.NewReader([]byte("file content")), "20260101/teams/t/channels/c1/users/u1/f1/report.txt")
	require.NoError(t, err)

	p1 := messageExport("p1", "c1", model.ChannelTypeOpen, "u1", "alice", 110, 110, "hello world")
	p1.PostFileIds = model.StringArray{"f1"}
	p2 := messageExport("p2", "c1", model.ChannelTypeOpen, "u2", "bob", 120, 160, "edited =1+1")
	p2.PostEditAt = ptr(int64(160))
	p3 := messageExport("p3", "d1", model.ChannelTypeDirect, "u2", "bob", 170, 170, "hi alice")
	p3.ChannelDisplayName = ptr("Direct Message")

	ms := f.store.Store
	ms.ComplianceStore.On("MessageExport", mock.Anything, model.MessageExportCursor{LastPostUpdateAt: testExportFrom, UntilUpdateAt: testJobEnd}, 2).
		Return([]*model.MessageExport{p1, p2}, model.MessageExportCursor{LastPostUpdateAt: 160, LastPostId: "p2", UntilUpdateAt: testJobEnd}, nil)
	ms.ComplianceStore.On("MessageExport", mock.Anything, model.MessageExportCursor{LastPostUpdateAt: 160, LastPostId: "p2", UntilUpdateAt: testJobEnd}, 2).
		Return([]*model.MessageExport{p3}, model.MessageExportCursor{LastPostUpdateAt: 170, LastPostId: "p3", UntilUpdateAt: testJobEnd}, nil)

	ms.FileInfoStore.On("GetByIds", []string{"f1"}, true, false, false).Return([]*model.FileInfo{{
		Id: "f1", PostId: "p1", CreatorId: "u1", Name: "report.txt", Size: 12, CreateAt: 110,
		Path: "20260101/teams/t/channels/c1/users/u1/f1/report.txt",
	}}, nil)

	ms.ChannelMemberHistoryStore.On("GetChannelsWithActivityDuring", testExportFrom, int64(160)).Return([]string{"c1", "c3"}, nil)
	ms.ChannelMemberHistoryStore.On("GetChannelsWithActivityDuring", int64(160), testJobEnd).Return([]string{"d1"}, nil)
	ms.ChannelStore.On("GetChannelsByIds", []string{"c3"}, true).Return([]*model.Channel{{Id: "c3", Name: "secret", DisplayName: "Secret", Type: model.ChannelTypePrivate, TeamId: "team1"}}, nil)
	ms.TeamStore.On("GetMany", []string{"team1"}).Return([]*model.Team{{Id: "team1", Name: "team-one", DisplayName: "Team One"}}, nil)

	ms.ChannelMemberHistoryStore.On("GetUsersInChannelDuring", testExportFrom, int64(160), mock.Anything).Return([]*model.ChannelMemberHistoryResult{
		{ChannelId: "c1", UserId: "u1", JoinTime: 50, UserEmail: "alice@example.com", Username: "alice"},
		{ChannelId: "c1", UserId: "u2", JoinTime: 105, UserEmail: "bob@example.com", Username: "bob"},
		{ChannelId: "c3", UserId: "u1", JoinTime: 20, LeaveTime: ptr(int64(130)), UserEmail: "alice@example.com", Username: "alice"},
	}, nil)
	ms.ChannelMemberHistoryStore.On("GetUsersInChannelDuring", int64(160), testJobEnd, mock.Anything).Return([]*model.ChannelMemberHistoryResult{
		{ChannelId: "d1", UserId: "u1", JoinTime: 10, UserEmail: "alice@example.com", Username: "alice"},
		{ChannelId: "d1", UserId: "u2", JoinTime: 10, UserEmail: "bob@example.com", Username: "bob"},
	}, nil)

	f.job = NewJob(Deps{
		Store:             func() store.Store { return f.store },
		Config:            func() *model.Config { return f.cfg },
		FileBackend:       func() filestore.FileBackend { return f.fileBackend },
		ExportFileBackend: func() filestore.FileBackend { return f.exportBackend },
		Now:               func() time.Time { return testNow },
		newSender: func(*model.GlobalRelayMessageExportSettings) (emlSender, error) {
			return f.sender, nil
		},
	})
	return f
}

func (f *exportFixture) run(t *testing.T, data model.StringMap) (*model.Job, bool) {
	job := &model.Job{Id: model.NewId(), Type: model.JobTypeMessageExport, Data: data, CreateAt: model.GetMillis()}
	if job.Data == nil {
		job.Data = model.StringMap{}
	}
	warning, err := f.job.execute(context.Background(), mlog.CreateConsoleTestLogger(t), job)
	require.NoError(t, err)
	return job, warning
}

func exportDirFor(start, end int64) string {
	return path.Join("export", testNow.Format(model.ComplianceExportDirectoryFormat)+"-"+itoa(start)+"-"+itoa(end))
}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}

func TestExportCSV(t *testing.T) {
	f := newExportFixture(t, model.ComplianceExportTypeCsv)
	job, warning := f.run(t, nil)
	require.False(t, warning)

	dir := exportDirFor(testExportFrom, testJobEnd)
	assert.Equal(t, dir, job.Data[shared.JobDataExportDir])
	assert.Equal(t, "3", job.Data[shared.JobDataMessagesExported])
	assert.Equal(t, "1", job.Data[shared.JobDataFilesExported])
	assert.Equal(t, "2", job.Data[shared.JobDataBatchNumber])
	assert.Equal(t, "true", job.Data[shared.JobDataIsDownloadable])
	assert.Equal(t, "170", job.Data[shared.JobDataBatchStartTime])
	assert.Equal(t, "p3", job.Data[shared.JobDataBatchStartId])
	assert.Equal(t, "100", job.Data[shared.JobDataJobStartTime])
	assert.Equal(t, "1000", job.Data[shared.JobDataJobEndTime])
	assert.Equal(t, model.ComplianceExportTypeCsv, job.Data[shared.JobDataExportType])

	files, err := f.exportBackend.ListDirectory(dir)
	require.NoError(t, err)
	require.Len(t, files, 2)

	b1 := readBackendZip(t, f.exportBackend, path.Join(dir, "batch001-100-160.zip"))
	require.Contains(t, b1, shared.CSVExportFileName)
	require.Contains(t, b1, shared.MetadataFileName)
	assert.Equal(t, "file content", string(b1["files/20260101/teams/t/channels/c1/users/u1/f1/report.txt"]))
	assert.NotContains(t, b1, shared.WarningFileName)

	rows, err := csv.NewReader(bytes.NewReader(b1[shared.CSVExportFileName])).ReadAll()
	require.NoError(t, err)
	assert.Equal(t, csvHeader, rows[0])
	kinds := []string{}
	for _, r := range rows[1:] {
		kinds = append(kinds, r[0]+":"+r[4]+":"+r[15])
	}
	assert.Equal(t, []string{
		"100:c1:previously-joined", // alice
		"100:c3:previously-joined", // alice in c3
		"105:c1:enter",             // bob
		"110:c1:message",
		"110:c1:attachment",
		"120:c1:message",
		"130:c3:leave",
	}, kinds)

	// The edited post and the formula protection.
	var edited []string
	for _, r := range rows {
		if len(r) > 11 && r[11] == "p2" {
			edited = r
		}
	}
	require.NotNil(t, edited)
	assert.Equal(t, "edited =1+1", edited[14])
	assert.Equal(t, UpdatedTypeEditedNewMsg, edited[19])
	assert.Equal(t, "160", edited[18])

	var meta batchMetadata
	require.NoError(t, json.Unmarshal(b1[shared.MetadataFileName], &meta))
	assert.Equal(t, 2, meta.MessagesCount)
	assert.Equal(t, 1, meta.AttachmentsCount)
	assert.Equal(t, "public - c1", meta.Channels["c1"].RoomId)
	assert.Equal(t, "Secret", meta.Channels["c3"].ChannelDisplayName)

	b2 := readBackendZip(t, f.exportBackend, path.Join(dir, "batch002-160-1000.zip"))
	rows, err = csv.NewReader(bytes.NewReader(b2[shared.CSVExportFileName])).ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 4) // header, 2 previously-joined, 1 message
	assert.Equal(t, "hi alice", rows[3][14])
	assert.Equal(t, "direct", rows[3][7])
}

func TestExportActiance(t *testing.T) {
	f := newExportFixture(t, model.ComplianceExportTypeActiance)
	job, warning := f.run(t, nil)
	require.False(t, warning)

	b1 := readBackendZip(t, f.exportBackend, path.Join(job.Data[shared.JobDataExportDir], "batch001-100-160.zip"))
	require.Contains(t, b1, shared.ActianceExportFileName)
	assert.Equal(t, "file content", string(b1["20260101/teams/t/channels/c1/users/u1/f1/report.txt"]))

	type participant struct {
		LoginName   string
		DateTimeUTC int64
	}
	type message struct {
		MessageId          string
		LoginName          string
		DateTimeUTC        int64
		UpdatedDateTimeUTC int64
		UpdatedType        string
		Content            string
	}
	type transfer struct {
		UserFileName string
		FileName     string
		Status       string
	}
	type conversation struct {
		Perspective string `xml:"Perspective,attr"`
		RoomID      string
		StartTime   int64         `xml:"StartTimeUTC"`
		EndTime     int64         `xml:"EndTimeUTC"`
		Entered     []participant `xml:"ParticipantEntered"`
		Left        []participant `xml:"ParticipantLeft"`
		Messages    []message     `xml:"Message"`
		Started     []transfer    `xml:"FileTransferStarted"`
		Ended       []transfer    `xml:"FileTransferEnded"`
	}
	var dump struct {
		XMLName       xml.Name       `xml:"FileDump"`
		Conversations []conversation `xml:"Conversation"`
	}
	raw := b1[shared.ActianceExportFileName]
	require.NoError(t, xml.Unmarshal(raw, &dump))
	require.Len(t, dump.Conversations, 2)

	c1 := dump.Conversations[0]
	assert.Equal(t, "Display c1", c1.Perspective)
	assert.Equal(t, "public - c1", c1.RoomID)
	assert.Equal(t, int64(100), c1.StartTime)
	assert.Equal(t, int64(160), c1.EndTime)
	assert.Equal(t, []participant{{"alice@example.com", 50}, {"bob@example.com", 105}}, c1.Entered)
	assert.Equal(t, []participant{{"alice@example.com", 160}, {"bob@example.com", 160}}, c1.Left)
	require.Len(t, c1.Messages, 2)
	assert.Equal(t, message{MessageId: "p1", LoginName: "alice@example.com", DateTimeUTC: 110, Content: "hello world"}, c1.Messages[0])
	assert.Equal(t, message{MessageId: "p2", LoginName: "bob@example.com", DateTimeUTC: 120, UpdatedDateTimeUTC: 160, UpdatedType: UpdatedTypeEditedNewMsg, Content: "edited =1+1"}, c1.Messages[1])
	require.Len(t, c1.Started, 1)
	assert.Equal(t, "report.txt", c1.Started[0].UserFileName)
	assert.Equal(t, "Completed", c1.Ended[0].Status)

	// The elements are in the documented order.
	s := string(raw)
	assert.Less(t, strings.Index(s, "<RoomID>"), strings.Index(s, "<StartTimeUTC>"))
	assert.Less(t, strings.Index(s, "<StartTimeUTC>"), strings.Index(s, "<ParticipantEntered>"))
	assert.Less(t, strings.Index(s, "<ParticipantEntered>"), strings.Index(s, "<Message>"))
	assert.Less(t, strings.Index(s, "<Message>"), strings.Index(s, "<ParticipantLeft>"))
	assert.Less(t, strings.Index(s, "<ParticipantLeft>"), strings.Index(s, "<EndTimeUTC>"))
	assert.NotContains(t, s, "<PreviewsPost>")

	c3 := dump.Conversations[1]
	assert.Equal(t, "private - c3", c3.RoomID)
	assert.Equal(t, []participant{{"alice@example.com", 130}}, c3.Left)
}

func parseEML(t *testing.T, data []byte) (*mail.Message, string, []string) {
	msg, err := mail.ReadMessage(bytes.NewReader(data))
	require.NoError(t, err)
	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	require.NoError(t, err)
	var text string
	var attachments []string
	if strings.HasPrefix(mediaType, "multipart/") {
		var walk func(r io.Reader, boundary string)
		walk = func(r io.Reader, boundary string) {
			mr := multipart.NewReader(r, boundary)
			for {
				part, err := mr.NextPart()
				if err == io.EOF {
					return
				}
				require.NoError(t, err)
				mt, ps, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
				if strings.HasPrefix(mt, "multipart/") {
					walk(part, ps["boundary"])
					continue
				}
				if part.FileName() != "" {
					attachments = append(attachments, part.FileName())
					continue
				}
				if mt == "text/plain" {
					b, _ := io.ReadAll(decodePart(part))
					text = string(b)
				}
			}
		}
		walk(msg.Body, params["boundary"])
	}
	return msg, text, attachments
}

func decodePart(p *multipart.Part) io.Reader {
	// multipart.Reader transparently decodes quoted-printable parts.
	return p
}

func TestExportGlobalRelaySMTP(t *testing.T) {
	f := newExportFixture(t, model.ComplianceExportTypeGlobalrelay)
	job, warning := f.run(t, nil)
	require.False(t, warning)

	assert.Equal(t, "false", job.Data[shared.JobDataIsDownloadable])
	assert.True(t, f.sender.closed)
	require.Len(t, f.sender.sent, 3) // c1 and c3 in batch 1, d1 in batch 2

	first := f.sender.sent[0]
	assert.Equal(t, "archive@globalrelay.example.com", first.to)
	assert.Equal(t, "alice@example.com", first.from)

	msg, text, attachments := parseEML(t, first.data)
	assert.Equal(t, globalRelayMsgType, msg.Header.Get(model.GlobalRelayMsgTypeHeader))
	assert.Equal(t, "c1", msg.Header.Get(model.GlobalRelayChannelIDHeader))
	assert.Equal(t, "public", msg.Header.Get(model.GlobalRelayChannelTypeHeader))
	assert.Equal(t, "Display c1", msg.Header.Get(model.GlobalRelayChannelNameHeader))
	assert.Equal(t, "Chat", msg.Header.Get("X-Archive-Type"))
	assert.Equal(t, "auto-generated", msg.Header.Get("Auto-Submitted"))
	to, err := msg.Header.AddressList("To")
	require.NoError(t, err)
	require.Len(t, to, 2)
	assert.Contains(t, msg.Header.Get("Subject"), "Display c1")
	assert.Contains(t, text, "hello world")
	assert.Contains(t, text, "bob (bob@example.com)")
	assert.Equal(t, []string{"report.txt"}, attachments)

	// A fixed sender address is used when configured.
	f2 := newExportFixture(t, model.ComplianceExportTypeGlobalrelay)
	*f2.cfg.MessageExportSettings.GlobalRelaySettings.SenderAddress = "compliance@example.com"
	f2.run(t, nil)
	for _, s := range f2.sender.sent {
		assert.Equal(t, "compliance@example.com", s.from)
		msg, _, _ := parseEML(t, s.data)
		from, err := msg.Header.AddressList("From")
		require.NoError(t, err)
		assert.Equal(t, "compliance@example.com", from[0].Address)
	}
}

func TestExportGlobalRelayZip(t *testing.T) {
	f := newExportFixture(t, model.ComplianceExportTypeGlobalrelayZip)
	job, _ := f.run(t, nil)
	assert.Equal(t, "true", job.Data[shared.JobDataIsDownloadable])

	b1 := readBackendZip(t, f.exportBackend, path.Join(job.Data[shared.JobDataExportDir], "batch001-100-160.zip"))
	var emls []string
	for name := range b1 {
		if strings.HasSuffix(name, ".eml") {
			emls = append(emls, name)
		}
	}
	assert.Len(t, emls, 2)
	msg, _, _ := parseEML(t, b1["name-c1-c1-100-160.eml"])
	assert.Equal(t, "c1", msg.Header.Get(model.GlobalRelayChannelIDHeader))
	assert.Empty(t, f.sender.sent)
}

func TestExportMissingAttachmentIsAWarning(t *testing.T) {
	f := newExportFixture(t, model.ComplianceExportTypeCsv)
	require.NoError(t, f.fileBackend.RemoveFile("20260101/teams/t/channels/c1/users/u1/f1/report.txt"))
	job, warning := f.run(t, nil)
	assert.True(t, warning)
	assert.Equal(t, "1", job.Data[shared.JobDataWarningCount])
	b1 := readBackendZip(t, f.exportBackend, path.Join(job.Data[shared.JobDataExportDir], "batch001-100-160.zip"))
	assert.Contains(t, string(b1[shared.WarningFileName]), "f1")
}

func TestExportDeliveryFailure(t *testing.T) {
	f := newExportFixture(t, model.ComplianceExportTypeGlobalrelay)
	f.sender.err = errors.New("smtp down")
	job := &model.Job{Id: model.NewId(), Type: model.JobTypeMessageExport, Data: model.StringMap{}}
	_, err := f.job.execute(context.Background(), mlog.CreateConsoleTestLogger(t), job)
	require.Error(t, err)
	// The position did not move.
	assert.Equal(t, "100", job.Data[shared.JobDataBatchStartTime])
}

func TestInitStateStartTime(t *testing.T) {
	cfg := newTestConfig()
	*cfg.MessageExportSettings.ExportFromTimestamp = 42
	st := newTestStore()
	j := NewJob(Deps{Store: func() store.Store { return st }, Config: func() *model.Config { return cfg }, Now: func() time.Time { return testNow }})
	rctx := request.TestContext(t)

	t.Run("first export starts at ExportFromTimestamp", func(t *testing.T) {
		job := &model.Job{Id: model.NewId(), Data: model.StringMap{}}
		s, err := j.initState(rctx, st, cfg, job)
		require.NoError(t, err)
		assert.Equal(t, int64(42), s.jobStart)
		assert.Equal(t, testJobEnd, s.jobEnd)
		assert.Equal(t, model.MessageExportCursor{LastPostUpdateAt: 42, UntilUpdateAt: testJobEnd}, s.position)
		assert.Equal(t, model.ComplianceExportTypeActiance, s.exportType)
	})

	// Previous jobs: a regular successful one, a newer mmctl one and a newer failed one.
	_, _ = st.jobs.Save(&model.Job{Id: "regular", Type: model.JobTypeMessageExport, CreateAt: 1, Status: model.JobStatusWarning,
		Data: model.StringMap{shared.JobDataBatchStartTime: "500", shared.JobDataBatchStartId: "px"}})
	_, _ = st.jobs.Save(&model.Job{Id: "mmctl", Type: model.JobTypeMessageExport, CreateAt: 2, Status: model.JobStatusSuccess,
		Data: model.StringMap{shared.JobDataBatchStartTime: "900", shared.JobDataInitiatedBy: shared.InitiatedByMmctl}})
	_, _ = st.jobs.Save(&model.Job{Id: "failed", Type: model.JobTypeMessageExport, CreateAt: 3, Status: model.JobStatusError,
		Data: model.StringMap{shared.JobDataBatchStartTime: "950"}})

	t.Run("continues from the previous regular job", func(t *testing.T) {
		job := &model.Job{Id: model.NewId(), Data: model.StringMap{shared.JobDataInitiatedBy: shared.InitiatedByMmctl, shared.JobDataBatchStartId: "", shared.JobDataJobStartId: ""}}
		s, err := j.initState(rctx, st, cfg, job)
		require.NoError(t, err)
		assert.Equal(t, int64(500), s.jobStart)
		assert.Equal(t, "px", s.position.LastPostId)
		s.save(job)
		assert.Equal(t, "500", job.Data[shared.JobDataJobStartTime])
		assert.Equal(t, "px", job.Data[shared.JobDataJobStartId])
	})

	t.Run("explicit range", func(t *testing.T) {
		job := &model.Job{Id: model.NewId(), Data: model.StringMap{
			shared.JobDataBatchStartTime: "200", shared.JobDataJobStartTime: "200", shared.JobDataJobEndTime: "300",
			shared.JobDataExportDir: "export/custom", shared.JobDataExportType: model.ComplianceExportTypeCsv,
		}}
		s, err := j.initState(rctx, st, cfg, job)
		require.NoError(t, err)
		assert.Equal(t, int64(200), s.jobStart)
		assert.Equal(t, int64(300), s.jobEnd)
		assert.Equal(t, "export/custom", s.exportDir)
		assert.Equal(t, model.ComplianceExportTypeCsv, s.exportType)
	})

	t.Run("batch start only", func(t *testing.T) {
		job := &model.Job{Id: model.NewId(), Data: model.StringMap{shared.JobDataBatchStartTime: "250", shared.JobDataJobEndTime: "400"}}
		s, err := j.initState(rctx, st, cfg, job)
		require.NoError(t, err)
		assert.Equal(t, int64(250), s.jobStart)
		assert.Equal(t, int64(250), s.position.LastPostUpdateAt)
	})

	t.Run("invalid data", func(t *testing.T) {
		job := &model.Job{Id: model.NewId(), Data: model.StringMap{shared.JobDataJobEndTime: "abc"}}
		_, err := j.initState(rctx, st, cfg, job)
		var appErr *model.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, "ent.message_export.job_data_conversion.app_error", appErr.Id)
	})
}

func TestExportNoPostsAdvancesPosition(t *testing.T) {
	st := newTestStore()
	cfg := newTestConfig()
	st.Store.ComplianceStore.On("MessageExport", mock.Anything, mock.Anything, mock.Anything).Return([]*model.MessageExport{}, model.MessageExportCursor{}, nil)
	st.Store.ChannelMemberHistoryStore.On("GetChannelsWithActivityDuring", int64(10), int64(20)).Return([]string{}, nil)
	j := NewJob(Deps{Store: func() store.Store { return st }, Config: func() *model.Config { return cfg }, Now: func() time.Time { return testNow }})

	job := &model.Job{Id: model.NewId(), Data: model.StringMap{shared.JobDataBatchStartTime: "10", shared.JobDataJobEndTime: "20"}}
	warning, err := j.execute(context.Background(), mlog.CreateConsoleTestLogger(t), job)
	require.NoError(t, err)
	assert.False(t, warning)
	assert.Equal(t, "20", job.Data[shared.JobDataBatchStartTime])
	assert.Equal(t, "", job.Data[shared.JobDataBatchStartId])
	assert.Equal(t, "false", job.Data[shared.JobDataIsDownloadable])
	assert.Equal(t, "0", job.Data[shared.JobDataMessagesExported])
}

func newTestJobServer(t *testing.T, st store.Store, cfg *model.Config) *jobs.JobServer {
	return jobs.NewJobServer(&fakeConfigService{cfg: cfg}, st, nil, mlog.CreateConsoleTestLogger(t), nil)
}

func TestWorkerLifecycle(t *testing.T) {
	f := newExportFixture(t, model.ComplianceExportTypeCsv)
	js := newTestJobServer(t, f.store, f.cfg)
	f.job.deps.JobServer = js
	w := f.job.newWorker()
	js.RegisterJobType(model.JobTypeMessageExport, w, nil)

	job, appErr := js.CreateJob(request.TestContext(t), model.JobTypeMessageExport, nil)
	require.Nil(t, appErr)

	w.DoJob(context.Background(), job)

	saved, err := f.store.jobs.Get(nil, job.Id)
	require.NoError(t, err)
	assert.Equal(t, model.JobStatusSuccess, saved.Status)
	assert.Equal(t, int64(100), saved.Progress)
	assert.Equal(t, "3", saved.Data[shared.JobDataMessagesExported])

	// A job which is not pending anymore is not run again.
	w.DoJob(context.Background(), saved)
	f.store.Store.ComplianceStore.AssertNumberOfCalls(t, "MessageExport", 2)
}

func TestWorkerInterruptedJobIsRequeued(t *testing.T) {
	f := newExportFixture(t, model.ComplianceExportTypeCsv)
	js := newTestJobServer(t, f.store, f.cfg)
	f.job.deps.JobServer = js
	w := f.job.newWorker()
	js.RegisterJobType(model.JobTypeMessageExport, w, nil)

	job, appErr := js.CreateJob(request.TestContext(t), model.JobTypeMessageExport, nil)
	require.Nil(t, appErr)
	claimed, appErr := js.ClaimJob(job)
	require.Nil(t, appErr)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	w.RunClaimedJob(ctx, mlog.CreateConsoleTestLogger(t), claimed, true)
	saved, err := f.store.jobs.Get(nil, job.Id)
	require.NoError(t, err)
	assert.Equal(t, model.JobStatusPending, saved.Status)

	claimed, appErr = js.ClaimJob(saved)
	require.Nil(t, appErr)
	w.RunClaimedJob(ctx, mlog.CreateConsoleTestLogger(t), claimed, false)
	saved, err = f.store.jobs.Get(nil, job.Id)
	require.NoError(t, err)
	assert.Equal(t, model.JobStatusCanceled, saved.Status)
}

func TestWorkerFailure(t *testing.T) {
	st := newTestStore()
	cfg := newTestConfig()
	st.Store.ComplianceStore.On("MessageExport", mock.Anything, mock.Anything, mock.Anything).Return(nil, model.MessageExportCursor{}, errors.New("db down"))
	js := newTestJobServer(t, st, cfg)
	j := NewJob(Deps{JobServer: js, Store: func() store.Store { return st }, Config: func() *model.Config { return cfg }})
	w := j.newWorker()
	js.RegisterJobType(model.JobTypeMessageExport, w, nil)

	job, appErr := js.CreateJob(request.TestContext(t), model.JobTypeMessageExport, nil)
	require.Nil(t, appErr)
	w.DoJob(context.Background(), job)

	saved, err := st.jobs.Get(nil, job.Id)
	require.NoError(t, err)
	assert.Equal(t, model.JobStatusError, saved.Status)
	assert.Contains(t, saved.Data["error"], "db down")
}

func TestStartSynchronizeJob(t *testing.T) {
	f := newExportFixture(t, model.ComplianceExportTypeActiance)
	js := newTestJobServer(t, f.store, f.cfg)
	f.job.deps.JobServer = js
	js.RegisterJobType(model.JobTypeMessageExport, f.job.newWorker(), nil)

	me := NewMessageExport(f.job.deps)
	job, appErr := me.StartSynchronizeJob(request.TestContext(t), testExportFrom)
	require.Nil(t, appErr)
	assert.Equal(t, model.JobStatusSuccess, job.Status)
	assert.Equal(t, shared.InitiatedByCLI, job.Data[shared.JobDataInitiatedBy])
	assert.Equal(t, "3", job.Data[shared.JobDataMessagesExported])
}

func TestSchedulerSkipsWhenPending(t *testing.T) {
	cfg := newTestConfig()
	st := newTestStore()
	js := newTestJobServer(t, st, cfg)
	j := NewJob(Deps{JobServer: js, Store: func() store.Store { return st }, Config: func() *model.Config { return cfg }})
	js.RegisterJobType(model.JobTypeMessageExport, j.newWorker(), nil)
	s := j.MakeScheduler()

	assert.True(t, s.Enabled(cfg))
	job, appErr := s.ScheduleJob(request.TestContext(t), cfg, true, nil)
	require.Nil(t, appErr)
	assert.Nil(t, job)

	job, appErr = s.ScheduleJob(request.TestContext(t), cfg, false, nil)
	require.Nil(t, appErr)
	require.NotNil(t, job)
	assert.Equal(t, model.JobTypeMessageExport, job.Type)

	*cfg.MessageExportSettings.DailyRunTime = "03:30"
	next := s.NextScheduleTime(cfg, time.Date(2026, 1, 1, 4, 0, 0, 0, time.Local), false, nil)
	require.NotNil(t, next)
	assert.Equal(t, time.Date(2026, 1, 2, 3, 30, 0, 0, time.Local), *next)

	*cfg.MessageExportSettings.EnableExport = false
	assert.False(t, s.Enabled(cfg))
}

func TestNewGlobalRelaySender(t *testing.T) {
	settings := &model.GlobalRelayMessageExportSettings{}
	settings.SetDefaults()
	// A9 (the default) has no built-in endpoint.
	_, err := newGlobalRelaySender(settings)
	require.Error(t, err)

	*settings.CustomerType = model.GlobalrelayCustomerTypeA10
	s, err := newGlobalRelaySender(settings)
	require.NoError(t, err)
	assert.Equal(t, globalRelayA10Server, s.host)
	assert.Equal(t, "25", s.port)
	assert.Equal(t, 1800*time.Second, s.timeout)

	*settings.CustomerType = model.GlobalrelayCustomerTypeCustom
	*settings.CustomSMTPServerName = "smtp.example.com"
	*settings.CustomSMTPPort = "587"
	s, err = newGlobalRelaySender(settings)
	require.NoError(t, err)
	assert.Equal(t, "smtp.example.com", s.host)
	assert.Equal(t, "587", s.port)

	*settings.CustomerType = "B12"
	_, err = newGlobalRelaySender(settings)
	require.Error(t, err)
}

func TestUpdatedType(t *testing.T) {
	base := func() *post {
		return &post{MessageExport: messageExport("p", "c", model.ChannelTypeOpen, "u", "user", 10, 10, "m")}
	}
	p := base()
	typ, id := p.updatedType()
	assert.Equal(t, "", typ)
	assert.Equal(t, "", id)

	p = base()
	p.PostUpdateAt = ptr(int64(20))
	typ, _ = p.updatedType()
	assert.Equal(t, UpdatedTypeUpdatedNoMsgChange, typ)

	p = base()
	p.PostDeleteAt = ptr(int64(30))
	typ, _ = p.updatedType()
	assert.Equal(t, UpdatedTypeDeleted, typ)

	p = base()
	p.PostDeleteAt = ptr(int64(30))
	p.PostOriginalId = ptr("new")
	typ, id = p.updatedType()
	assert.Equal(t, UpdatedTypeEditedOriginalMsg, typ)
	assert.Equal(t, "new", id)
}
