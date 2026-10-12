// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package messageexport

import (
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// The channel export (Antimatter's replacement of Mattermost's Channel Export
// plugin) writes one channel's messages as a CSV file, for someone who can
// read the channel. It uses the compliance export's query and post handling,
// but lists what the channel shows now rather than everything that happened:
// no deleted messages nor edit history, no join and leave events, the
// attachments by name, and times people can read.

// channelExportPageSize is how many posts are read from the store at once.
var channelExportPageSize = 1000

// channelExportHeader lists the columns of a channel export. The names are
// those of the compliance CSV for the columns both have.
var channelExportHeader = []string{
	"Post Creation Time",
	"Post Id",
	"Replied to Post Id",
	"User Id",
	"Username",
	"User Email",
	"User Type",
	"Post Message",
	"Attachments",
	"Edit Time",
}

// readableTime formats a time in milliseconds as RFC 3339, in UTC.
func readableTime(ms int64) string {
	if ms == 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}

// ExportChannelCSV implements app.ChannelExporter: it writes the channel's
// posts created in the range, oldest first, with their attachments that still
// exist.
func ExportChannelCSV(rctx request.CTX, st store.Store, params app.ChannelExportParams, w io.Writer) (int, error) {
	posts, err := channelPosts(rctx, st, params)
	if err != nil {
		return 0, err
	}

	b := &batch{posts: posts}
	if err := (&batchBuilder{store: st}).addFiles(b); err != nil {
		return 0, err
	}

	cw := csv.NewWriter(w)
	if err := cw.Write(channelExportHeader); err != nil {
		return 0, err
	}
	for _, p := range posts {
		if err := cw.Write(channelExportRow(p, params.IncludeEmails)); err != nil {
			return 0, err
		}
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return 0, err
	}
	return len(posts), nil
}

// channelPosts reads the channel's posts created in the range, without the
// deleted ones, sorted by creation time. It stops with
// app.ErrChannelExportTooLarge past params.MaxPosts.
func channelPosts(rctx request.CTX, st store.Store, params app.ChannelExportParams) ([]*post, error) {
	// The query pages by update time: since a post is updated at or after its
	// creation, nothing created in the range was updated before it starts.
	cursor := model.MessageExportCursor{
		LastPostUpdateAt: params.Since,
		ChannelId:        params.ChannelID,
		SinceCreateAt:    params.Since,
		UntilCreateAt:    params.Until,
		ExcludeDeleted:   true,
	}

	var posts []*post
	for {
		if ctx := rctx.Context(); ctx != nil {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		page, next, err := st.Compliance().MessageExport(rctx, cursor, channelExportPageSize)
		if err != nil {
			return nil, fmt.Errorf("failed to get the channel's posts: %w", err)
		}
		for _, m := range page {
			posts = append(posts, &post{MessageExport: m})
		}
		if params.MaxPosts > 0 && len(posts) > params.MaxPosts {
			return nil, app.ErrChannelExportTooLarge
		}
		if len(page) < channelExportPageSize {
			break
		}
		cursor = next
	}

	sort.Slice(posts, func(i, j int) bool {
		if posts[i].createAt() != posts[j].createAt() {
			return posts[i].createAt() < posts[j].createAt()
		}
		return posts[i].id() < posts[j].id()
	})
	return posts, nil
}

func channelExportRow(p *post, includeEmails bool) []string {
	email := ""
	if includeEmails {
		email = csvSafe(p.userEmail())
	}

	// Files removed since (by data retention, or with their message) are not
	// listed.
	var names []string
	for _, info := range p.files {
		if info.DeleteAt == 0 {
			names = append(names, info.Name)
		}
	}

	return []string{
		readableTime(p.createAt()),
		p.id(),
		p.rootID(),
		p.userID(),
		csvSafe(p.username()),
		email,
		p.userType(),
		csvSafe(p.message()),
		csvSafe(strings.Join(names, "; ")),
		readableTime(p.editAt()),
	}
}
