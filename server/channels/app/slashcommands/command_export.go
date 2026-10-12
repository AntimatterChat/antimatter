// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package slashcommands

import (
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/i18n"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app"
)

// ExportProvider is /export [days]: it exports the current channel's messages (all of them, or those of the last
// days) and sends the CSV file to the user in their direct messages with the system bot, as Mattermost's Channel
// Export plugin did.
type ExportProvider struct {
}

const (
	CmdExport = "export"
)

func init() {
	app.RegisterCommandProvider(&ExportProvider{})
}

func (*ExportProvider) GetTrigger() string {
	return CmdExport
}

func (*ExportProvider) GetCommand(a *app.App, T i18n.TranslateFunc) *model.Command {
	return &model.Command{
		Trigger:          CmdExport,
		AutoComplete:     true,
		AutoCompleteDesc: T("api.command_export.desc"),
		AutoCompleteHint: T("api.command_export.hint"),
		DisplayName:      T("api.command_export.name"),
	}
}

func (*ExportProvider) DoCommand(a *app.App, rctx request.CTX, args *model.CommandArgs, message string) *model.CommandResponse {
	ephemeral := func(text string) *model.CommandResponse {
		return &model.CommandResponse{Text: text, ResponseType: model.CommandResponseTypeEphemeral}
	}

	opts := app.ChannelExportOptions{Format: model.ChannelExportFormatCSV}
	if arg := strings.TrimSpace(message); arg != "" {
		days, err := strconv.Atoi(arg)
		if err != nil || days <= 0 {
			return ephemeral(args.T("api.command_export.days.app_error"))
		}
		opts.Since = exportSince(days, model.GetMillis())
	}

	// The file is written to a temporary file (in the server's temporary directory), then sent from there.
	tmp, err := os.CreateTemp("", "channel-export-*.csv")
	if err != nil {
		rctx.Logger().Warn("Unable to create the channel export's temporary file", mlog.Err(err))
		return ephemeral(args.T("api.command_export.send.app_error"))
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()

	file, appErr := a.ExportChannel(rctx, args.UserId, args.ChannelId, opts, func(*app.ChannelExportFile) (io.Writer, error) {
		return tmp, nil
	})
	if appErr != nil {
		if appErr.Id == "app.channel_export.too_large.app_error" {
			return ephemeral(args.T("api.command_export.too_large.app_error", map[string]any{"Max": model.ChannelExportMaxPosts}))
		}
		appErr.Translate(args.T)
		return ephemeral(appErr.Message)
	}

	size, err := tmp.Seek(0, io.SeekCurrent)
	if err == nil {
		_, err = tmp.Seek(0, io.SeekStart)
	}
	if err != nil {
		rctx.Logger().Warn("Unable to read the channel export's temporary file", mlog.Err(err))
		return ephemeral(args.T("api.command_export.send.app_error"))
	}

	channel, appErr := a.GetChannel(rctx, args.ChannelId)
	if appErr != nil {
		return ephemeral(args.T("api.command_export.send.app_error"))
	}
	if _, appErr := a.SendChannelExportToUser(rctx, args.UserId, channel, file, tmp, size); appErr != nil {
		rctx.Logger().Warn("Unable to send the channel export", mlog.String("channel_id", args.ChannelId), mlog.Err(appErr))
		return ephemeral(args.T("api.command_export.send.app_error"))
	}
	return ephemeral(args.T("api.command_export.sent", map[string]any{"Bot": model.BotSystemBotUsername}))
}

// exportSince is the start of the last days (> 0) at now, in milliseconds: 0, so all the messages, when they go
// back further than 1970.
func exportSince(days int, now int64) int64 {
	day := (24 * time.Hour).Milliseconds()
	if int64(days) > now/day {
		return 0
	}
	return now - int64(days)*day
}
