// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package messageexport

import (
	"archive/zip"
	"encoding/csv"
	"fmt"
	"path"
	"sort"
	"strconv"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/libre/messageexport/shared"
)

// csvHeader lists the columns of posts.csv.
var csvHeader = []string{
	"Post Creation Time",
	"Team Id",
	"Team Name",
	"Team Display Name",
	"Channel Id",
	"Channel Name",
	"Channel Display Name",
	"Channel Type",
	"User Id",
	"User Email",
	"Username",
	"Post Id",
	"Edited By Post Id",
	"Replied to Post Id",
	"Post Message",
	"Post Type",
	"User Type",
	"Previews Post Id",
	"Update Time",
	"Updated Type",
}

// Values of the "Post Type" column.
const (
	csvRowMessage    = "message"
	csvRowAttachment = "attachment"
)

type csvRow struct {
	time  int64
	order int // tie breaker: joins, then messages, then leaves
	seq   int
	cols  []string
}

// csvSafe prevents spreadsheet applications from interpreting a cell as a
// formula.
func csvSafe(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}
	return s
}

func formatTime(ms int64) string {
	if ms == 0 {
		return ""
	}
	return strconv.FormatInt(ms, 10)
}

func channelColumns(c *channel) []string {
	return []string{
		c.TeamID,
		csvSafe(c.TeamName),
		csvSafe(c.TeamDisplayName),
		c.ID,
		csvSafe(c.Name),
		csvSafe(c.DisplayName),
		c.typeName(),
	}
}

// csvAttachmentPath returns the path of an attachment inside the archive.
func csvAttachmentPath(info *model.FileInfo) string {
	return path.Join(shared.CSVFilesDirectory, info.Path)
}

// csvRows builds the rows of posts.csv for the batch, sorted chronologically.
func csvRows(b *batch) []csvRow {
	var rows []csvRow
	add := func(t int64, order int, cols []string) {
		rows = append(rows, csvRow{time: t, order: order, seq: len(rows), cols: cols})
	}

	for _, c := range b.channels {
		for _, ev := range c.events(b) {
			p := ev.Participant
			var msg string
			order := 0
			switch ev.Kind {
			case eventPreviouslyJoined:
				msg = fmt.Sprintf("User %s was already in the channel", p.displayUser())
			case eventJoin:
				msg = fmt.Sprintf("User %s joined the channel", p.displayUser())
			case eventLeave:
				msg = fmt.Sprintf("User %s left the channel", p.displayUser())
				order = 2
			}
			cols := append([]string{formatTime(ev.Time)}, channelColumns(c)...)
			cols = append(cols, p.UserID, csvSafe(p.Email), csvSafe(p.Username), "", "", "", msg, ev.Kind, p.userType(), "", "", "")
			add(ev.Time, order, cols)
		}

		for _, p := range c.posts {
			updatedType, editedNewMsgID := p.updatedType()
			updateTime := ""
			if updatedType != "" {
				updateTime = formatTime(p.updateAt())
			}
			base := append([]string{formatTime(p.createAt())}, channelColumns(c)...)
			base = append(base, p.userID(), csvSafe(p.userEmail()), csvSafe(p.username()))

			cols := append(append([]string{}, base...),
				p.id(), editedNewMsgID, p.rootID(), csvSafe(p.message()), csvRowMessage, p.userType(), p.previewedID(), updateTime, updatedType)
			add(p.createAt(), 1, cols)

			for _, info := range p.files {
				cols := append(append([]string{}, base...),
					p.id(), editedNewMsgID, p.rootID(), csvSafe(csvAttachmentPath(info)), csvRowAttachment, p.userType(), "", updateTime, updatedType)
				add(p.createAt(), 1, cols)
				if info.DeleteAt > 0 {
					cols := append(append([]string{}, base...),
						p.id(), "", p.rootID(), csvSafe(csvAttachmentPath(info)), csvRowAttachment, p.userType(), "", formatTime(info.DeleteAt), UpdatedTypeFileDeleted)
					add(p.createAt(), 1, cols)
				}
			}
		}
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].time != rows[j].time {
			return rows[i].time < rows[j].time
		}
		if rows[i].order != rows[j].order {
			return rows[i].order < rows[j].order
		}
		return rows[i].seq < rows[j].seq
	})
	return rows
}

// writeCSVBatch fills the archive of a CSV batch: posts.csv, metadata.json,
// the attachments under files/ and warning.txt when needed.
func (r *exportRun) writeCSVBatch(zw *zip.Writer, b *batch) error {
	w, err := createZipEntry(zw, shared.CSVExportFileName, r.now)
	if err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(csvHeader); err != nil {
		return err
	}
	for _, row := range csvRows(b) {
		if err := cw.Write(row.cols); err != nil {
			return err
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return err
	}

	if err := r.writeMetadata(zw, b); err != nil {
		return err
	}

	seen := map[string]bool{}
	for _, p := range b.posts {
		for _, info := range p.files {
			name := csvAttachmentPath(info)
			if seen[name] {
				continue
			}
			seen[name] = true
			if err := r.ctx.Err(); err != nil {
				return err
			}
			if err := r.copyAttachment(zw, info, name); err != nil {
				return err
			}
		}
	}

	return r.writeWarnings(zw)
}
