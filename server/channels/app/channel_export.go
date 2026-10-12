// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/i18n"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// Antimatter's channel export: the people who can read a channel can download its messages as a CSV file, from the
// channel's menu or with /export. It replaces Mattermost's paid Channel Export plugin.
//
// The export is built synchronously, in memory, and is capped at model.ChannelExportMaxPosts messages: past that,
// people are asked for a shorter date range. That keeps the download immediate and the file small enough to open in
// a spreadsheet, without a job and a place to keep the files.

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
}

// ErrChannelExportTooLarge is returned by the channel exporter when the channel has more than MaxPosts posts in the
// range.
var ErrChannelExportTooLarge = errors.New("too many posts to export")

// ChannelExporter writes the CSV export of a channel's posts to w and returns how many posts it holds. When there are
// too many posts, it returns ErrChannelExportTooLarge before writing anything.
type ChannelExporter func(rctx request.CTX, st store.Store, params ChannelExportParams, w io.Writer) (int, error)

var channelExporter ChannelExporter

// RegisterChannelExporter registers the channel exporter (libre/messageexport's).
func RegisterChannelExporter(f ChannelExporter) {
	channelExporter = f
}

// ChannelExportOptions is what someone asked to export of a channel.
type ChannelExportOptions struct {
	// Format of the file: model.ChannelExportFormatCSV (the default when empty).
	Format string

	// The creation time range of the messages, in milliseconds, inclusive; 0 leaves a bound open.
	Since int64
	Until int64
}

// ChannelExportFile is an exported channel.
type ChannelExportFile struct {
	Name        string
	ContentType string
	Data        []byte

	// Posts is how many messages the file holds.
	Posts int
}

// CanExportChannel tells whether the user may export the channel: they must be able to read it and, when the admins
// restrict exports to admins (TeamSettings.RestrictChannelExportToAdmins), manage its roles or its team.
func (a *App) CanExportChannel(rctx request.CTX, userID string, channel *model.Channel) *model.AppError {
	if ok, _ := a.HasPermissionToReadChannel(rctx, userID, channel); !ok {
		return model.NewAppError("ExportChannel", "app.channel_export.permission.app_error", nil, "", http.StatusForbidden)
	}
	if !*a.Config().TeamSettings.RestrictChannelExportToAdmins {
		return nil
	}
	if ok, _ := a.HasPermissionToChannel(rctx, userID, channel.Id, model.PermissionManageChannelRoles); ok {
		return nil
	}
	if channel.TeamId != "" && a.HasPermissionToTeam(rctx, userID, channel.TeamId, model.PermissionManageTeam) {
		return nil
	}
	return model.NewAppError("ExportChannel", "app.channel_export.admins_only.app_error", nil, "", http.StatusForbidden)
}

// ExportChannel exports the messages of a channel for the user, after checking they may. Every export, refused or
// not, is audit-logged.
func (a *App) ExportChannel(rctx request.CTX, userID, channelID string, opts ChannelExportOptions) (*ChannelExportFile, *model.AppError) {
	rec := a.makeChannelExportAuditRecord(rctx, userID)
	model.AddEventParameterToAuditRec(rec, "channel_id", channelID)
	model.AddEventParameterToAuditRec(rec, "format", opts.Format)
	model.AddEventParameterToAuditRec(rec, "since", opts.Since)
	model.AddEventParameterToAuditRec(rec, "until", opts.Until)

	file, appErr := a.exportChannel(rctx, userID, channelID, opts)
	if appErr != nil {
		a.LogAuditRec(rctx, rec, appErr)
		return nil, appErr
	}
	rec.AddMeta("exported_posts", file.Posts)
	rec.Success()
	a.LogAuditRec(rctx, rec, nil)
	return file, nil
}

func (a *App) exportChannel(rctx request.CTX, userID, channelID string, opts ChannelExportOptions) (*ChannelExportFile, *model.AppError) {
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
	if appErr := a.CanExportChannel(rctx, userID, channel); appErr != nil {
		return nil, appErr
	}

	isAdmin := a.HasPermissionTo(rctx, userID, model.PermissionManageSystem)
	params := ChannelExportParams{
		ChannelID:     channel.Id,
		Since:         opts.Since,
		Until:         opts.Until,
		MaxPosts:      model.ChannelExportMaxPosts,
		IncludeEmails: a.GetSanitizeOptions(isAdmin)["email"],
	}

	var buf bytes.Buffer
	count, err := channelExporter(rctx, a.Srv().Store(), params, &buf)
	if errors.Is(err, ErrChannelExportTooLarge) {
		return nil, model.NewAppError("ExportChannel", "app.channel_export.too_large.app_error", map[string]any{"Max": model.ChannelExportMaxPosts}, "", http.StatusBadRequest)
	}
	if err != nil {
		return nil, model.NewAppError("ExportChannel", "app.channel_export.failed.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	return &ChannelExportFile{
		Name:        channelExportFileName(channel, time.Now()),
		ContentType: "text/csv; charset=utf-8",
		Data:        buf.Bytes(),
		Posts:       count,
	}, nil
}

// channelExportFileName names the file after the channel and the day: town-square-2026-10-12.csv. Conversations have
// no readable name: theirs say what they are.
func channelExportFileName(channel *model.Channel, now time.Time) string {
	name := channel.Name
	switch channel.Type {
	case model.ChannelTypeDirect:
		name = "direct-message"
	case model.ChannelTypeGroup:
		name = "group-message"
	}
	return name + "-" + now.Format("2006-01-02") + "." + model.ChannelExportFormatCSV
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

// SendChannelExportToUser sends an exported channel to the user, in their direct messages with the system bot (as
// /export does). It returns that conversation.
func (a *App) SendChannelExportToUser(rctx request.CTX, userID string, channel *model.Channel, file *ChannelExportFile) (*model.Channel, *model.AppError) {
	bot, appErr := a.GetSystemBot(rctx)
	if appErr != nil {
		return nil, appErr
	}
	dm, appErr := a.GetOrCreateDirectChannel(rctx, userID, bot.UserId)
	if appErr != nil {
		return nil, appErr
	}
	info, appErr := a.UploadFileForUserAndTeam(rctx, file.Data, dm.Id, file.Name, bot.UserId, "")
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
