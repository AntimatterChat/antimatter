// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package slashcommands

import (
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/i18n"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app"
)

// RulesProvider is /rules (Antimatter): it shows the current community's rules, and /rules agree agrees to them. It
// is how the classic web app and the mobile app (which have no screen for the rules yet) read and agree to them; the
// answers are only seen by the one who asked.
type RulesProvider struct {
}

const (
	CmdRules      = "rules"
	cmdRulesAgree = "agree"
)

func init() {
	app.RegisterCommandProvider(&RulesProvider{})
}

func (*RulesProvider) GetTrigger() string {
	return CmdRules
}

func (*RulesProvider) GetCommand(a *app.App, T i18n.TranslateFunc) *model.Command {
	return &model.Command{
		Trigger:          CmdRules,
		AutoComplete:     true,
		AutoCompleteDesc: T("api.command_rules.desc"),
		AutoCompleteHint: T("api.command_rules.hint"),
		DisplayName:      T("api.command_rules.name"),
	}
}

func (*RulesProvider) DoCommand(a *app.App, rctx request.CTX, args *model.CommandArgs, message string) *model.CommandResponse {
	ephemeral := func(text string) *model.CommandResponse {
		return &model.CommandResponse{ResponseType: model.CommandResponseTypeEphemeral, Text: text}
	}

	// The rules of the channel's community; in a direct message, of the community open in the app.
	teamID := args.TeamId
	if channel, appErr := a.GetChannel(rctx, args.ChannelId); appErr == nil && channel.TeamId != "" {
		teamID = channel.TeamId
	}
	team, appErr := a.GetTeam(teamID)
	if appErr != nil {
		return ephemeral(args.T("api.command_rules.none"))
	}

	if strings.EqualFold(strings.TrimSpace(message), cmdRulesAgree) {
		if _, appErr = a.AcceptCommunityRules(rctx, team.Id, args.UserId, 0); appErr != nil {
			return ephemeral(args.T(appErr.Id))
		}
		return ephemeral(args.T("api.command_rules.agreed", map[string]any{"Community": team.DisplayName}))
	}

	rules, appErr := a.GetCommunityRulesForUser(rctx, team.Id, args.UserId)
	if appErr != nil {
		return ephemeral(args.T(appErr.Id))
	}
	if !rules.IsActive() {
		return ephemeral(args.T("api.command_rules.none"))
	}
	text := args.T("api.command_rules.title", map[string]any{"Community": team.DisplayName}) + "\n\n" + rules.Text + "\n\n"
	if rules.MustAccept {
		text += args.T("api.command_rules.agree_hint")
	} else if rules.AcceptedVersion >= rules.Version {
		text += args.T("api.command_rules.already_agreed")
	}
	return ephemeral(strings.TrimSpace(text))
}
