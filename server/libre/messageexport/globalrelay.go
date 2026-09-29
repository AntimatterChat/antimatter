// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package messageexport

import (
	"archive/zip"
	"bytes"
	"fmt"
	"html/template"
	"io"
	"net/mail"
	"regexp"
	"strings"
	"time"

	gomail "github.com/wneessen/go-mail"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

const (
	// globalRelayMsgType is the value of the X-GlobalRelay-MsgType header
	// identifying Mattermost conversations in the Global Relay archive.
	globalRelayMsgType = "Mattermost"

	// globalRelayMaxMessageSize is the maximum size of an EML accepted by
	// Global Relay; attachments are dropped from larger messages.
	globalRelayMaxMessageSize = 250 * 1024 * 1024

	globalRelayTimeFormat = "2006-01-02 15:04:05 MST"
)

// globalRelayEML is one generated EML: a channel's conversation during a batch.
type globalRelayEML struct {
	channel *channel
	from    string // also used as the SMTP envelope sender
	data    []byte
}

// validEmail returns the address if it is a syntactically valid email.
func validEmail(email string) (string, bool) {
	if email == "" {
		return "", false
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return "", false
	}
	return addr.Address, true
}

var fileNameUnsafe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// emlFileName returns the name of an EML inside a globalrelay-zip archive.
func emlFileName(b *batch, c *channel) string {
	name := fileNameUnsafe.ReplaceAllString(c.Name, "_")
	if len(name) > 64 {
		name = name[:64]
	}
	return fmt.Sprintf("%s-%s-%d-%d.eml", name, c.ID, b.start, b.end)
}

type grParticipantView struct {
	Username string
	Email    string
	UserType string
	Joined   string
	Left     string
	Messages int
}

type grMessageView struct {
	Time        string
	User        string
	Message     string
	Status      string
	Attachments []string
}

type grView struct {
	ChannelName  string
	ChannelType  string
	TeamName     string
	Start        string
	End          string
	Duration     string
	MessageCount int
	Participants []grParticipantView
	Messages     []grMessageView
}

var grHTMLTemplate = template.Must(template.New("eml").Parse(`<html>
<head><meta charset="UTF-8"></head>
<body>
<h2>Conversation summary</h2>
<table>
<tr><th align="left">Channel</th><td>{{.ChannelName}}</td></tr>
<tr><th align="left">Channel type</th><td>{{.ChannelType}}</td></tr>
{{if .TeamName}}<tr><th align="left">Team</th><td>{{.TeamName}}</td></tr>{{end}}
<tr><th align="left">Started</th><td>{{.Start}}</td></tr>
<tr><th align="left">Ended</th><td>{{.End}}</td></tr>
<tr><th align="left">Duration</th><td>{{.Duration}}</td></tr>
<tr><th align="left">Messages</th><td>{{.MessageCount}}</td></tr>
</table>
<h3>Participants</h3>
<table border="1" cellpadding="4" cellspacing="0">
<tr><th>Username</th><th>Email</th><th>User type</th><th>Joined</th><th>Left</th><th>Messages</th></tr>
{{range .Participants}}<tr><td>{{.Username}}</td><td>{{.Email}}</td><td>{{.UserType}}</td><td>{{.Joined}}</td><td>{{.Left}}</td><td>{{.Messages}}</td></tr>
{{end}}</table>
<h3>Messages</h3>
<ul>
{{range .Messages}}<li><span class="time">{{.Time}}</span> <strong>{{.User}}</strong>{{if .Status}} <em>({{.Status}})</em>{{end}}: <span class="message" style="white-space: pre-wrap">{{.Message}}</span>{{range .Attachments}}<br>Attachment: {{.}}{{end}}</li>
{{end}}</ul>
</body>
</html>
`))

func formatGRTime(ms int64) string {
	if ms <= 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format(globalRelayTimeFormat)
}

func (v *grView) text() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Conversation summary\n\nChannel: %s\nChannel type: %s\n", v.ChannelName, v.ChannelType)
	if v.TeamName != "" {
		fmt.Fprintf(&sb, "Team: %s\n", v.TeamName)
	}
	fmt.Fprintf(&sb, "Started: %s\nEnded: %s\nDuration: %s\nMessages: %d\n\nParticipants\n\n", v.Start, v.End, v.Duration, v.MessageCount)
	for _, p := range v.Participants {
		fmt.Fprintf(&sb, "- %s <%s> (%s) joined: %s left: %s messages: %d\n", p.Username, p.Email, p.UserType, p.Joined, p.Left, p.Messages)
	}
	sb.WriteString("\nMessages\n\n")
	for _, m := range v.Messages {
		status := ""
		if m.Status != "" {
			status = " (" + m.Status + ")"
		}
		fmt.Fprintf(&sb, "%s %s%s: %s\n", m.Time, m.User, status, m.Message)
		for _, a := range m.Attachments {
			fmt.Fprintf(&sb, "    Attachment: %s\n", a)
		}
	}
	return sb.String()
}

func buildGRView(b *batch, c *channel) *grView {
	v := &grView{
		ChannelName:  c.DisplayName,
		ChannelType:  c.typeName(),
		TeamName:     c.TeamDisplayName,
		Start:        formatGRTime(b.start),
		End:          formatGRTime(b.end),
		Duration:     (time.Duration(b.end-b.start) * time.Millisecond).Round(time.Second).String(),
		MessageCount: len(c.posts),
	}
	for _, p := range c.participants {
		pv := grParticipantView{Username: p.Username, Email: p.Email, UserType: p.userType(), Messages: p.Messages}
		var joined, left []string
		for _, m := range p.Intervals {
			joined = append(joined, formatGRTime(m.JoinTime))
			if m.LeaveTime > 0 && m.LeaveTime <= b.end {
				left = append(left, formatGRTime(m.LeaveTime))
			}
		}
		pv.Joined = strings.Join(joined, ", ")
		pv.Left = strings.Join(left, ", ")
		v.Participants = append(v.Participants, pv)
	}
	for _, p := range c.posts {
		mv := grMessageView{Time: formatGRTime(p.createAt()), User: p.displayUser(), Message: p.message()}
		switch updatedType, editedNewMsgID := p.updatedType(); updatedType {
		case UpdatedTypeEditedOriginalMsg:
			mv.Status = fmt.Sprintf("original message, edited at %s, new message ID %s", formatGRTime(p.updateAt()), editedNewMsgID)
		case UpdatedTypeEditedNewMsg:
			mv.Status = "edited at " + formatGRTime(p.editAt())
		case UpdatedTypeDeleted:
			mv.Status = "deleted at " + formatGRTime(p.deleteAt())
		case UpdatedTypeUpdatedNoMsgChange:
			mv.Status = "updated at " + formatGRTime(p.updateAt())
		}
		for _, info := range p.files {
			name := info.Name
			if info.DeleteAt > 0 {
				name += " (deleted at " + formatGRTime(info.DeleteAt) + ")"
			}
			mv.Attachments = append(mv.Attachments, name)
		}
		v.Messages = append(v.Messages, mv)
	}
	return v
}

// buildGlobalRelayEML generates the EML of a channel's conversation.
func (r *exportRun) buildGlobalRelayEML(b *batch, c *channel) (*globalRelayEML, error) {
	gr := r.settings.GlobalRelaySettings
	if gr == nil {
		gr = &model.GlobalRelayMessageExportSettings{}
		gr.SetDefaults()
	}

	var recipients []string
	seen := map[string]bool{}
	for _, p := range c.participants {
		if email, ok := validEmail(p.Email); ok && !seen[email] {
			seen[email] = true
			recipients = append(recipients, email)
		}
	}

	from := ""
	if sender, ok := validEmail(model.SafeDereference(gr.SenderAddress)); ok {
		from = sender
	} else if len(recipients) > 0 {
		// Participants are sorted by username.
		for _, p := range c.participants {
			if email, ok := validEmail(p.Email); ok {
				from = email
				break
			}
		}
	}
	archiveAddress, archiveOK := validEmail(model.SafeDereference(gr.EmailAddress))
	if from == "" {
		if !archiveOK {
			return nil, fmt.Errorf("no valid sender address for channel %s", c.ID)
		}
		from = archiveAddress
	}
	if len(recipients) == 0 {
		if !archiveOK {
			return nil, fmt.Errorf("no valid recipient address for channel %s", c.ID)
		}
		recipients = []string{archiveAddress}
	}

	m := gomail.NewMsg(gomail.WithNoDefaultUserAgent())
	if err := m.From(from); err != nil {
		return nil, fmt.Errorf("invalid sender %q: %w", from, err)
	}
	if err := m.To(recipients...); err != nil {
		return nil, fmt.Errorf("invalid recipients: %w", err)
	}
	m.Subject(fmt.Sprintf("Mattermost Compliance Export: %s", c.DisplayName))
	m.SetDateWithValue(time.UnixMilli(b.end))
	m.SetMessageIDWithValue(fmt.Sprintf("%s.%d.%d@compliance-export.mattermost", c.ID, b.start, b.end))
	m.SetGenHeader(gomail.Header(model.GlobalRelayMsgTypeHeader), globalRelayMsgType)
	m.SetGenHeader(gomail.Header(model.GlobalRelayChannelNameHeader), c.DisplayName)
	m.SetGenHeader(gomail.Header(model.GlobalRelayChannelIDHeader), c.ID)
	m.SetGenHeader(gomail.Header(model.GlobalRelayChannelTypeHeader), c.typeName())
	m.SetGenHeader(gomail.Header("Auto-Submitted"), "auto-generated")
	m.SetGenHeader(gomail.HeaderPrecedence, "bulk")
	if name, value := model.SafeDereference(gr.CustomHeaderName), model.SafeDereference(gr.CustomHeaderValue); name != "" && value != "" && !model.IsGlobalRelayReservedHeader(name) {
		m.SetGenHeader(gomail.Header(name), value)
	}

	view := buildGRView(b, c)
	var html bytes.Buffer
	if err := grHTMLTemplate.Execute(&html, view); err != nil {
		return nil, err
	}
	text := view.text()
	m.SetBodyString(gomail.TypeTextPlain, text)
	m.AddAlternativeString(gomail.TypeTextHTML, html.String())

	// Attachments, unless the message would exceed the size limit.
	var size int64 = int64(len(text) + html.Len())
	var files []*model.FileInfo
	for _, p := range c.posts {
		for _, info := range p.files {
			files = append(files, info)
			size += info.Size * 4 / 3
		}
	}
	var closers []io.Closer
	defer func() {
		for _, c := range closers {
			c.Close()
		}
	}()
	if size > globalRelayMaxMessageSize {
		postIDs := []string{}
		fileIDs := []string{}
		for _, info := range files {
			postIDs = append(postIDs, info.PostId)
			fileIDs = append(fileIDs, info.Id)
		}
		r.logger.Error("global_relay_attachments_removed",
			mlog.String("channel_id", c.ID),
			mlog.Array("post_ids", postIDs),
			mlog.Array("file_ids", fileIDs),
		)
		r.warn("Attachments removed from a Global Relay message exceeding the size limit", mlog.String("channel_id", c.ID), mlog.Int("files", len(fileIDs)))
	} else if len(files) > 0 {
		if r.fileBackend == nil {
			r.warn("Unable to export attachments: no file backend", mlog.String("channel_id", c.ID))
		} else {
			for _, info := range files {
				reader, err := r.fileBackend.Reader(info.Path)
				if err != nil {
					r.warn("Unable to read attachment from the file store", mlog.String("file_id", info.Id), mlog.String("post_id", info.PostId), mlog.String("path", info.Path), mlog.Err(err))
					continue
				}
				closers = append(closers, reader)
				m.AttachReadSeeker(info.Name, reader)
				r.filesExported++
			}
		}
	}

	var buf bytes.Buffer
	if _, err := m.WriteTo(&buf); err != nil {
		return nil, fmt.Errorf("failed to generate the EML for channel %s: %w", c.ID, err)
	}
	return &globalRelayEML{channel: c, from: from, data: buf.Bytes()}, nil
}

// writeGlobalRelayZipBatch fills the archive of a globalrelay-zip batch: one
// EML per channel, metadata.json and warning.txt when needed.
func (r *exportRun) writeGlobalRelayZipBatch(zw *zip.Writer, b *batch) error {
	for _, c := range b.channels {
		if err := r.ctx.Err(); err != nil {
			return err
		}
		eml, err := r.buildGlobalRelayEML(b, c)
		if err != nil {
			return err
		}
		w, err := createZipEntry(zw, emlFileName(b, c), r.now)
		if err != nil {
			return err
		}
		if _, err := w.Write(eml.data); err != nil {
			return err
		}
	}
	if err := r.writeMetadata(zw, b); err != nil {
		return err
	}
	return r.writeWarnings(zw)
}

// deliverGlobalRelayBatch sends one EML per channel of the batch to the
// Global Relay archive through SMTP.
func (r *exportRun) deliverGlobalRelayBatch(b *batch, sender emlSender) error {
	archive, ok := validEmail(model.SafeDereference(r.settings.GlobalRelaySettings.EmailAddress))
	if !ok {
		return fmt.Errorf("invalid Global Relay email address")
	}
	for _, c := range b.channels {
		if err := r.ctx.Err(); err != nil {
			return err
		}
		eml, err := r.buildGlobalRelayEML(b, c)
		if err != nil {
			return err
		}
		if err := sender.Send(r.ctx, eml.from, archive, eml.data); err != nil {
			return fmt.Errorf("failed to deliver the EML of channel %s: %w", c.ID, err)
		}
	}
	return nil
}
