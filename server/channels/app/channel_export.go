// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/i18n"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/jobs/messageexport"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// Antimatter's channel export: the people who can read a channel can download its messages as a CSV file, from the
// channel's menu or with /export. It replaces Mattermost's paid Channel Export plugin.
//
// The export is built synchronously and streamed: the API writes it straight to the response, /export to a
// temporary file it then sends. It is capped at model.ChannelExportMaxPosts messages, counted before any is read:
// past that, people are asked for a shorter date range. That keeps the download immediate and the file small enough
// to open in a spreadsheet, without a job and a place to keep the files. At most channelExportConcurrency exports run
// at once on a server; past that, people are asked to try again in a moment.

// channelExportConcurrency is how many channel exports can run at once on a server.
const channelExportConcurrency = 2

// ChannelExportParams is what the channel exporter is asked for.
type ChannelExportParams = messageexport.ChannelExportParams

// ErrChannelExportTooLarge is returned by the channel exporter when the channel has more than MaxPosts posts in the
// range.
var ErrChannelExportTooLarge = messageexport.ErrChannelExportTooLarge

// ChannelExporter counts the channel's posts in the range first: when there are more than params.MaxPosts, it
// returns ErrChannelExportTooLarge without reading them nor calling open. Otherwise it calls open for the writer,
// writes the CSV export to it and returns how many posts it holds. The server's is messageexport.ExportChannelCSV.
type ChannelExporter func(rctx request.CTX, st store.Store, params ChannelExportParams, open func() (io.Writer, error)) (int, error)

// SetChannelExporter replaces the server's channel exporter; tests fake it.
func (s *Server) SetChannelExporter(f ChannelExporter) {
	s.channelExporter = f
}

// ChannelExportOptions is what someone asked to export of a channel.
type ChannelExportOptions struct {
	// Format of the file: model.ChannelExportFormatCSV (the default when empty).
	Format string

	// The creation time range of the messages, in milliseconds, inclusive; 0 leaves a bound open.
	Since int64
	Until int64
}

// ChannelExportFile describes an exported channel's file.
type ChannelExportFile struct {
	Name        string
	ContentType string

	// Posts is how many messages the file holds, once written.
	Posts int
}

// ChannelExportOutput opens what an export is written to, once every check has passed and the export is about to
// be written: the API's response, a temporary file. It's given the file's name and type.
type ChannelExportOutput func(file *ChannelExportFile) (io.Writer, error)

// CanExportChannel tells whether the user may export the channel: they must be able to read it and, when the admins
// restrict exports to admins (TeamSettings.RestrictChannelExportToAdmins), manage its roles or its team. It also
// tells whether they are a member of the channel (they can read a public channel without being one).
func (a *App) CanExportChannel(rctx request.CTX, userID string, channel *model.Channel) (bool, *model.AppError) {
	ok, isMember := a.HasPermissionToReadChannel(rctx, userID, channel)
	if !ok {
		return false, model.NewAppError("ExportChannel", "app.channel_export.permission.app_error", nil, "", http.StatusForbidden)
	}
	if !*a.Config().TeamSettings.RestrictChannelExportToAdmins {
		return isMember, nil
	}
	if ok, _ := a.HasPermissionToChannel(rctx, userID, channel.Id, model.PermissionManageChannelRoles); ok {
		return isMember, nil
	}
	if channel.TeamId != "" && a.HasPermissionToTeam(rctx, userID, channel.TeamId, model.PermissionManageTeam) {
		return isMember, nil
	}
	return isMember, model.NewAppError("ExportChannel", "app.channel_export.admins_only.app_error", nil, "", http.StatusForbidden)
}

// ExportChannel exports the messages of a channel for the user, after checking they may, and writes the file to
// what output opens. Output is called only once nothing is left to refuse: an error returned before is an answer to
// give, one returned after means the file was cut short (and can only be logged, the file being under way). Every
// export, refused or not, is audit-logged.
func (a *App) ExportChannel(rctx request.CTX, userID, channelID string, opts ChannelExportOptions, output ChannelExportOutput) (*ChannelExportFile, *model.AppError) {
	rec := a.makeChannelExportAuditRecord(rctx, userID)
	model.AddEventParameterToAuditRec(rec, "channel_id", channelID)
	model.AddEventParameterToAuditRec(rec, "format", opts.Format)
	model.AddEventParameterToAuditRec(rec, "since", opts.Since)
	model.AddEventParameterToAuditRec(rec, "until", opts.Until)

	file, appErr := a.exportChannel(rctx, rec, userID, channelID, opts, output)
	if appErr != nil {
		a.LogAuditRec(rctx, rec, appErr)
		return nil, appErr
	}
	rec.AddMeta("exported_posts", file.Posts)
	rec.Success()
	a.LogAuditRec(rctx, rec, nil)
	return file, nil
}

func (a *App) exportChannel(rctx request.CTX, rec *model.AuditRecord, userID, channelID string, opts ChannelExportOptions, output ChannelExportOutput) (*ChannelExportFile, *model.AppError) {
	channelExporter := a.Srv().channelExporter
	if channelExporter == nil {
		return nil, model.NewAppError("ExportChannel", "app.channel_export.unavailable.app_error", nil, "", http.StatusNotImplemented)
	}
	if opts.Format == "" {
		opts.Format = model.ChannelExportFormatCSV
	}
	if opts.Format != model.ChannelExportFormatCSV {
		return nil, model.NewAppError("ExportChannel", "app.channel_export.format.app_error", map[string]any{"Format": opts.Format}, "", http.StatusBadRequest)
	}
	if opts.Since < 0 || opts.Until < 0 || (opts.Until > 0 && opts.Since > opts.Until) {
		return nil, model.NewAppError("ExportChannel", "app.channel_export.range.app_error", nil, "", http.StatusBadRequest)
	}

	channel, appErr := a.GetChannel(rctx, channelID)
	if appErr != nil {
		return nil, appErr
	}
	isMember, appErr := a.CanExportChannel(rctx, userID, channel)
	if appErr != nil {
		return nil, appErr
	}
	if !isMember {
		model.AddEventParameterToAuditRec(rec, "non_channel_member_access", true)
	}

	release, ok := a.Srv().startChannelExport()
	if !ok {
		return nil, model.NewAppError("ExportChannel", "app.channel_export.busy.app_error", nil, "", http.StatusTooManyRequests)
	}
	defer release()

	// An open range ends now: messages posted while the export runs aren't in it.
	until := opts.Until
	if until == 0 {
		until = model.GetMillis()
	}

	isAdmin := a.HasPermissionTo(rctx, userID, model.PermissionManageSystem)
	params := ChannelExportParams{
		ChannelID:     channel.Id,
		Since:         opts.Since,
		Until:         until,
		MaxPosts:      model.ChannelExportMaxPosts,
		IncludeEmails: a.GetSanitizeOptions(isAdmin)["email"],
		ConsumePosts: func(posts map[string]*model.Post) {
			a.applyPostsWillBeConsumedHook(rctx, posts)
		},
	}

	file := &ChannelExportFile{
		Name:        channelExportFileName(channel, time.Now()),
		ContentType: "text/csv; charset=utf-8",
	}
	count, err := channelExporter(rctx, a.Srv().Store(), params, func() (io.Writer, error) { return output(file) })
	if errors.Is(err, ErrChannelExportTooLarge) {
		return nil, model.NewAppError("ExportChannel", "app.channel_export.too_large.app_error", map[string]any{"Max": model.ChannelExportMaxPosts}, "", http.StatusBadRequest)
	}
	if err != nil {
		return nil, model.NewAppError("ExportChannel", "app.channel_export.failed.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	file.Posts = count
	return file, nil
}

// startChannelExport takes one of the server's channel export slots, if one is free, and returns how to give it
// back.
func (s *Server) startChannelExport() (release func(), ok bool) {
	if s.channelExports == nil { // a server not made by NewServer
		return func() {}, true
	}
	select {
	case s.channelExports <- struct{}{}:
		return func() { <-s.channelExports }, true
	default:
		return nil, false
	}
}

// channelExportFileName names the file after the channel and the day, in UTC: town-square-2026-10-12.csv.
// Conversations have no readable name: theirs say what they are.
func channelExportFileName(channel *model.Channel, now time.Time) string {
	name := channel.Name
	switch channel.Type {
	case model.ChannelTypeDirect:
		name = "direct-message"
	case model.ChannelTypeGroup:
		name = "group-message"
	}
	return name + "-" + now.UTC().Format("2006-01-02") + "." + model.ChannelExportFormatCSV
}

// makeChannelExportAuditRecord makes the audit record of an export, by the user of the request.
func (a *App) makeChannelExportAuditRecord(rctx request.CTX, userID string) *model.AuditRecord {
	rec := a.MakeAuditRecord(rctx, model.AuditEventExportChannel, model.AuditStatusFail)
	rec.Actor = model.AuditEventActor{
		UserId:        userID,
		SessionId:     rctx.Session().Id,
		Client:        rctx.UserAgent(),
		IpAddress:     rctx.IPAddress(),
		XForwardedFor: rctx.XForwardedFor(),
	}
	rec.AddMeta(model.AuditKeyAPIPath, rctx.Path())
	return rec
}

// SendChannelExportToUser sends an exported channel, read from data (size bytes), to the user in their direct
// messages with the system bot (as /export does). It returns that conversation.
func (a *App) SendChannelExportToUser(rctx request.CTX, userID string, channel *model.Channel, file *ChannelExportFile, data io.Reader, size int64) (*model.Channel, *model.AppError) {
	bot, appErr := a.GetSystemBot(rctx)
	if appErr != nil {
		return nil, appErr
	}
	dm, appErr := a.GetOrCreateDirectChannel(rctx, userID, bot.UserId)
	if appErr != nil {
		return nil, appErr
	}
	info, appErr := a.UploadFileX(rctx, dm.Id, file.Name, data, func(t *UploadFileTask) {
		t.UserId = bot.UserId
		t.TeamId = "noteam"
		t.Timestamp = time.Now()
		t.ContentLength = size
	})
	if appErr != nil {
		return nil, appErr
	}

	user, appErr := a.GetUser(rctx, userID)
	if appErr != nil {
		return nil, appErr
	}
	T := i18n.GetUserTranslations(user.Locale)
	var message string
	if channel.Type == model.ChannelTypeDirect || channel.Type == model.ChannelTypeGroup {
		message = T("app.channel_export.sent_conversation", file.Posts, map[string]any{"Count": file.Posts})
	} else {
		message = T("app.channel_export.sent", file.Posts, map[string]any{"Channel": channel.Name, "Count": file.Posts})
	}
	post := &model.Post{
		ChannelId: dm.Id,
		UserId:    bot.UserId,
		Message:   message,
		FileIds:   []string{info.Id},
	}
	if _, _, appErr := a.CreatePost(rctx, post, dm, model.CreatePostFlags{SetOnline: true}); appErr != nil {
		return nil, appErr
	}
	return dm, nil
}
