// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package slashcommands

import (
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/i18n"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app"
)

type LeaveProvider struct {
}

const (
	CmdLeave = "leave"
)

func init() {
	app.RegisterCommandProvider(&LeaveProvider{})
}

func (*LeaveProvider) GetTrigger() string {
	return CmdLeave
}

func (*LeaveProvider) GetCommand(a *app.App, T i18n.TranslateFunc) *model.Command {
	return &model.Command{
		Trigger:          CmdLeave,
		AutoComplete:     true,
		AutoCompleteDesc: T("api.command_leave.desc"),
		DisplayName:      T("api.command_leave.name"),
	}
}

func (*LeaveProvider) DoCommand(a *app.App, rctx request.CTX, args *model.CommandArgs, message string) *model.CommandResponse {
	var channel *model.Channel
	var noChannelErr *model.AppError
	if channel, noChannelErr = a.GetChannel(rctx, args.ChannelId); noChannelErr != nil {
		return &model.CommandResponse{Text: args.T("api.command_leave.fail.app_error"), ResponseType: model.CommandResponseTypeEphemeral}
	}

	team, err := a.GetTeam(args.TeamId)
	if err != nil {
		return &model.CommandResponse{Text: args.T("api.command_leave.fail.app_error"), ResponseType: model.CommandResponseTypeEphemeral}
	}

	isDefault := a.IsDefaultChannel(rctx, channel)
	err = a.LeaveChannel(rctx, args.ChannelId, args.UserId)
	if err != nil {
		if isDefault {
			return &model.CommandResponse{Text: args.T("api.channel.leave.default.app_error", map[string]any{"Channel": channel.DisplayName}), ResponseType: model.CommandResponseTypeEphemeral}
		}
		if err.Id == "api.channel.leave.all_channels.app_error" {
			return &model.CommandResponse{Text: args.T(err.Id), ResponseType: model.CommandResponseTypeEphemeral}
		}
		return &model.CommandResponse{Text: args.T("api.command_leave.fail.app_error"), ResponseType: model.CommandResponseTypeEphemeral}
	}

	member, err := a.GetTeamMember(rctx, team.Id, args.UserId)
	if err != nil || member.DeleteAt != 0 {
		return &model.CommandResponse{GotoLocation: args.SiteURL + "/"}
	}

	user, err := a.GetUser(rctx, args.UserId)
	if err != nil {
		return &model.CommandResponse{Text: args.T("api.command_leave.fail.app_error"), ResponseType: model.CommandResponseTypeEphemeral}
	}

	if user.IsGuest() {
		members, err := a.GetChannelMembersForUser(rctx, team.Id, args.UserId)
		if err != nil || len(members) == 0 {
			return &model.CommandResponse{Text: args.T("api.command_leave.fail.app_error"), ResponseType: model.CommandResponseTypeEphemeral}
		}
		channel, err := a.GetChannel(rctx, members[0].ChannelId)
		if err != nil {
			return &model.CommandResponse{Text: args.T("api.command_leave.fail.app_error"), ResponseType: model.CommandResponseTypeEphemeral}
		}
		return &model.CommandResponse{GotoLocation: args.SiteURL + "/" + team.Name + "/channels/" + channel.Name}
	}

	// Back to the community's default channel; without one, to the community, which opens the first channel there.
	if defaultChannel := a.GetDefaultChannelForTeam(rctx, team); defaultChannel != nil {
		return &model.CommandResponse{GotoLocation: args.SiteURL + "/" + team.Name + "/channels/" + defaultChannel.Name}
	}
	return &model.CommandResponse{GotoLocation: args.SiteURL + "/" + team.Name}
}
