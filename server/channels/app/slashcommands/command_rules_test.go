// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package slashcommands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestRulesProviderDoCommand(t *testing.T) {
	th := setup(t).initBasic(t)
	rp := RulesProvider{}
	args := &model.CommandArgs{
		T:         func(s string, args ...any) string { return s },
		UserId:    th.BasicUser.Id,
		ChannelId: th.BasicChannel.Id,
		TeamId:    th.BasicTeam.Id,
	}

	resp := rp.DoCommand(th.App, th.Context, args, "")
	assert.Equal(t, model.CommandResponseTypeEphemeral, resp.ResponseType)
	assert.Equal(t, "api.command_rules.none", resp.Text, "no rules yet")

	_, appErr := th.App.SaveCommunityRules(th.Context, th.BasicTeam.Id, th.SystemAdminUser.Id, &model.CommunityRulesRequest{Enabled: true, Text: "Be kind."})
	require.Nil(t, appErr)

	resp = rp.DoCommand(th.App, th.Context, args, "")
	assert.Equal(t, model.CommandResponseTypeEphemeral, resp.ResponseType, "only they see the rules")
	assert.Contains(t, resp.Text, "Be kind.")
	assert.Contains(t, resp.Text, "api.command_rules.agree_hint")

	resp = rp.DoCommand(th.App, th.Context, args, " Agree ")
	assert.Equal(t, "api.command_rules.agreed", resp.Text)
	rules, appErr := th.App.GetCommunityRulesForUser(th.Context, th.BasicTeam.Id, th.BasicUser.Id)
	require.Nil(t, appErr)
	assert.False(t, rules.MustAccept, "/rules agree agrees")

	resp = rp.DoCommand(th.App, th.Context, args, "")
	assert.Contains(t, resp.Text, "api.command_rules.already_agreed")

	// In a direct message, the rules are those of the community open in the app.
	dm, appErr := th.App.GetOrCreateDirectChannel(th.Context, th.BasicUser2.Id, th.BasicUser.Id)
	require.Nil(t, appErr)
	resp = rp.DoCommand(th.App, th.Context, &model.CommandArgs{T: args.T, UserId: th.BasicUser2.Id, ChannelId: dm.Id, TeamId: th.BasicTeam.Id}, "")
	assert.Contains(t, resp.Text, "Be kind.")

	outsider := th.createUser(t)
	resp = rp.DoCommand(th.App, th.Context, &model.CommandArgs{T: args.T, UserId: outsider.Id, ChannelId: dm.Id, TeamId: th.BasicTeam.Id}, "agree")
	assert.Equal(t, "app.community_rules.accept.not_member.app_error", resp.Text, "only members agree")
}
