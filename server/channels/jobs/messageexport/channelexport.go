// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package messageexport

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// The channel export (Antimatter's replacement of Mattermost's Channel Export
// plugin) writes one channel's messages as a CSV file, for someone who can
// read the channel. It lists what the channel shows now rather than everything
// that happened, as the compliance export does: one row per message, oldest
// first, without deleted messages nor edit history, join and leave events or
// burn-on-read messages, with the attachments by name and the times in UTC.
//
// It counts the posts first, and refuses past the limit before reading any.
// Then it reads them a page at a time, by creation time and id (which don't
// change, unlike the update time: a message edited, replied to or reacted to
// during the export is read once), with a query of its own that reads only
// what the file shows, and writes each page as it goes: an export never holds
// more than a page.
//
// Cells starting with = + - or @ get a leading apostrophe, so that spreadsheets
// show them rather than run them as formulas (see exportCell).

// ChannelExportParams is what the channel exporter is asked for.
type ChannelExportParams struct {
	ChannelID string

	// The creation time range of the posts, in milliseconds, inclusive; 0 leaves a bound open.
	Since int64
	Until int64

	// MaxPosts is the most posts the export may hold; 0 for no limit.
	MaxPosts int

	// IncludeEmails fills the authors' email addresses in, when the person exporting may see them.
	IncludeEmails bool

	// ConsumePosts runs the plugins' MessagesWillBeConsumed hooks on a page of posts (by id), which may replace
	// them, as when posts are read anywhere else: redaction plugins apply to the export too. Nil runs none.
	ConsumePosts func(posts map[string]*model.Post)
}

// ErrChannelExportTooLarge is returned by ExportChannelCSV when the channel has more than MaxPosts posts in the
// range.
var ErrChannelExportTooLarge = errors.New("too many posts to export")

// channelExportPageSize is how many posts are read from the store at once.
var channelExportPageSize = 500

// channelExportHeader lists the columns of a channel export. The names are
// those of the compliance CSV for the columns both have; the times are in UTC.
var channelExportHeader = []string{
	"Post Creation Time (UTC)",
	"Post Id",
	"Replied to Post Id",
	"User Id",
	"Username",
	"User Email",
	"User Type",
	"Post Message",
	"Attachments",
	"Edit Time (UTC)",
}

// readableTime formats a time in milliseconds as RFC 3339, in UTC.
func readableTime(ms int64) string {
	if ms == 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format(time.RFC3339)
}

// exportCell keeps a spreadsheet from running a cell as a formula, with a
// leading apostrophe: csvSafe's guard (shared with the compliance export, for
// cells starting with = + - @, a tab or a carriage return), and as the channel
// export's files are meant to be opened in spreadsheets, also for those
// characters after leading spaces or line breaks, and for their full-width
// forms (＝＋－＠), which some spreadsheets take for them.
func exportCell(s string) string {
	if t := strings.TrimLeft(s, " \t\r\n"); t != "" {
		switch r, _ := utf8.DecodeRuneInString(t); r {
		case '=', '+', '-', '@', '＝', '＋', '－', '＠':
			return "'" + s
		}
	}
	return csvSafe(s)
}

// ExportChannelCSV is the app layer's ChannelExporter: it writes the channel's
// posts created in the range, oldest first, with their attachments that still
// exist.
func ExportChannelCSV(rctx request.CTX, st store.Store, params ChannelExportParams, open func() (io.Writer, error)) (int, error) {
	filter := model.ChannelExportFilter{
		ChannelId:     params.ChannelID,
		SinceCreateAt: params.Since,
		UntilCreateAt: params.Until,
	}

	if params.MaxPosts > 0 {
		count, err := st.Compliance().ChannelExportCount(rctx, filter, params.MaxPosts+1)
		if err != nil {
			return 0, fmt.Errorf("failed to count the channel's posts: %w", err)
		}
		if count > params.MaxPosts {
			return 0, ErrChannelExportTooLarge
		}
	}

	w, err := open()
	if err != nil {
		return 0, err
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(channelExportHeader); err != nil {
		return 0, err
	}

	written := 0
	var cursor model.ChannelExportCursor
	for {
		if ctx := rctx.Context(); ctx != nil {
			if err := ctx.Err(); err != nil {
				return written, err
			}
		}
		page, err := st.Compliance().ChannelExportPosts(rctx, filter, cursor, channelExportPageSize)
		if err != nil {
			return written, fmt.Errorf("failed to get the channel's posts: %w", err)
		}
		if len(page) == 0 {
			break
		}
		last := page[len(page)-1]
		cursor = model.ChannelExportCursor{AfterCreateAt: last.PostCreateAt, AfterId: last.PostId}

		rows, err := channelExportRows(st, page, params)
		if err != nil {
			return written, err
		}
		for _, row := range rows {
			if params.MaxPosts > 0 && written >= params.MaxPosts {
				// Posts were added to the range (imported, say) since they
				// were counted: the file stops at the limit.
				rctx.Logger().Warn("The channel export stops at its limit", mlog.String("channel_id", params.ChannelID), mlog.Int("max_posts", params.MaxPosts))
				cw.Flush()
				return written, cw.Error()
			}
			if err := cw.Write(row); err != nil {
				return written, err
			}
			written++
		}
		cw.Flush()
		if err := cw.Error(); err != nil {
			return written, err
		}
		if len(page) < channelExportPageSize {
			break
		}
	}
	cw.Flush()
	return written, cw.Error()
}

// channelExportRows makes the rows of a page of posts, after the plugins'
// MessagesWillBeConsumed hooks have seen them (redaction plugins apply to the
// file too).
func channelExportRows(st store.Store, page []*model.ChannelExportPost, params ChannelExportParams) ([][]string, error) {
	names, err := channelExportFileNames(st, page)
	if err != nil {
		return nil, err
	}

	// The hooks work on posts: these hold what the file shows.
	posts := make(map[string]*model.Post, len(page))
	for _, p := range page {
		posts[p.PostId] = channelExportPost(params.ChannelID, p)
	}
	if params.ConsumePosts != nil {
		params.ConsumePosts(posts)
	}

	rows := make([][]string, 0, len(page))
	for _, p := range page {
		consumed := posts[p.PostId]
		if consumed == nil {
			continue
		}
		email := ""
		if params.IncludeEmails {
			email = exportCell(p.UserEmail)
		}
		rows = append(rows, []string{
			readableTime(p.PostCreateAt),
			p.PostId,
			p.PostRootId,
			p.UserId,
			exportCell(exportUsername(p, consumed)),
			email,
			userType(p.IsBot),
			exportCell(exportMessage(consumed)),
			exportCell(strings.Join(names[p.PostId], "; ")),
			readableTime(p.PostEditAt),
		})
	}
	return rows, nil
}

// channelExportPost is the post the plugins' hooks are given: its message, and
// the props the file shows.
func channelExportPost(channelID string, p *model.ChannelExportPost) *model.Post {
	post := &model.Post{
		Id:        p.PostId,
		CreateAt:  p.PostCreateAt,
		UpdateAt:  p.PostCreateAt,
		EditAt:    p.PostEditAt,
		ChannelId: channelID,
		UserId:    p.UserId,
		RootId:    p.PostRootId,
		Type:      p.PostType,
		Message:   p.PostMessage,
		FileIds:   p.PostFileIds,
	}
	if p.FromWebhook != "" {
		post.AddProp(model.PostPropsFromWebhook, p.FromWebhook)
	}
	if p.OverrideUsername != "" {
		post.AddProp(model.PostPropsOverrideUsername, p.OverrideUsername)
	}
	if p.Attachments != "" {
		// As a post read from the database has them.
		var attachments []any
		if err := json.Unmarshal([]byte(p.Attachments), &attachments); err == nil && len(attachments) > 0 {
			post.AddProp(model.PostPropsAttachments, attachments)
		}
	}
	return post
}

// exportUsername is the author's username, or for a webhook's post the name
// it was posted under (override_username), as the channel shows it: the User
// Id column still tells whose webhook it is.
func exportUsername(p *model.ChannelExportPost, consumed *model.Post) string {
	if fromWebhook, _ := consumed.GetProp(model.PostPropsFromWebhook).(string); fromWebhook == "true" {
		if override, _ := consumed.GetProp(model.PostPropsOverrideUsername).(string); override != "" {
			return override
		}
	}
	return p.Username
}

// exportMessage is the post's message followed by the text of its message
// attachments (the pretext, title and text of each, or its fallback without
// them), which integrations often post instead of a message.
func exportMessage(post *model.Post) string {
	var parts []string
	if post.Message != "" {
		parts = append(parts, post.Message)
	}
	for _, a := range post.Attachments() {
		var text []string
		for _, s := range []string{a.Pretext, a.Title, a.Text} {
			if s != "" {
				text = append(text, s)
			}
		}
		if len(text) == 0 && a.Fallback != "" {
			text = append(text, a.Fallback)
		}
		if len(text) > 0 {
			parts = append(parts, strings.Join(text, "\n"))
		}
	}
	return strings.Join(parts, "\n\n")
}

// channelExportFileNames returns the names of the files of the page's posts
// (by post id) that still exist: files removed since, by data retention or
// with their message, are left out.
func channelExportFileNames(st store.Store, page []*model.ChannelExportPost) (map[string][]string, error) {
	var ids []string
	for _, p := range page {
		ids = append(ids, p.PostFileIds...)
	}
	names := map[string][]string{}
	if len(ids) == 0 {
		return names, nil
	}
	infos := map[string]*model.FileInfo{}
	for start := 0; start < len(ids); start += fileInfoChunkSize {
		chunk := ids[start:min(start+fileInfoChunkSize, len(ids))]
		list, err := st.FileInfo().GetByIds(chunk, true, false, false)
		if err != nil {
			return nil, fmt.Errorf("failed to get file infos: %w", err)
		}
		for _, info := range list {
			infos[info.Id] = info
		}
	}
	for _, p := range page {
		for _, id := range p.PostFileIds {
			if info := infos[id]; info != nil && info.DeleteAt == 0 {
				names[p.PostId] = append(names[p.PostId], info.Name)
			}
		}
	}
	return names, nil
}
