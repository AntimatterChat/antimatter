// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package slashcommands

import (
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
		opts.Since = model.GetMillis() - int64(days)*(24*time.Hour).Milliseconds()
	}

	file, appErr := a.ExportChannel(rctx, args.UserId, args.ChannelId, opts)
	if appErr != nil {
		if appErr.Id == "app.channel_export.too_large.app_error" {
			return ephemeral(args.T("api.command_export.too_large.app_error", map[string]any{"Max": model.ChannelExportMaxPosts}))
		}
		appErr.Translate(args.T)
		return ephemeral(appErr.Message)
	}

	channel, appErr := a.GetChannel(rctx, args.ChannelId)
	if appErr != nil {
		return ephemeral(args.T("api.command_export.send.app_error"))
	}
	if _, appErr := a.SendChannelExportToUser(rctx, args.UserId, channel, file); appErr != nil {
		rctx.Logger().Warn("Unable to send the channel export", mlog.String("channel_id", args.ChannelId), mlog.Err(appErr))
		return ephemeral(args.T("api.command_export.send.app_error"))
	}
	return ephemeral(args.T("api.command_export.sent", map[string]any{"Bot": model.BotSystemBotUsername}))
}
