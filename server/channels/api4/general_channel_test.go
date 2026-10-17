// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// The default channel is general now; apps, scripts and old clients asking for town-square, its old name, get it.
func TestGetChannelByLegacyDefaultName(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	client := th.Client

	general, _, err := client.GetChannelByName(context.Background(), model.DefaultChannelName, th.BasicTeam.Id, "")
	require.NoError(t, err)
	require.Equal(t, "general", general.Name)
	require.Equal(t, "General", general.DisplayName)

	t.Run("by community ID", func(t *testing.T) {
		channel, _, err := client.GetChannelByName(context.Background(), model.LegacyDefaultChannelName, th.BasicTeam.Id, "")
		require.NoError(t, err)
		require.Equal(t, general.Id, channel.Id)
		require.Equal(t, model.DefaultChannelName, channel.Name, "under its own name")

		channel, _, err = client.GetChannelByNameIncludeDeleted(context.Background(), model.LegacyDefaultChannelName, th.BasicTeam.Id, "")
		require.NoError(t, err)
		require.Equal(t, general.Id, channel.Id)
	})

	t.Run("by community name", func(t *testing.T) {
		channel, _, err := client.GetChannelByNameForTeamName(context.Background(), model.LegacyDefaultChannelName, th.BasicTeam.Name, "")
		require.NoError(t, err)
		require.Equal(t, general.Id, channel.Id)
	})

	t.Run("only for those who may see the default channel", func(t *testing.T) {
		outsider := th.CreateUser(t)
		outsiderClient := th.CreateClient()
		_, _, err := outsiderClient.Login(context.Background(), outsider.Email, outsider.Password)
		require.NoError(t, err)

		_, resp, err := outsiderClient.GetChannelByName(context.Background(), model.DefaultChannelName, th.BasicTeam.Id, "")
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		_, resp, err = outsiderClient.GetChannelByName(context.Background(), model.LegacyDefaultChannelName, th.BasicTeam.Id, "")
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		_, resp, err = outsiderClient.GetChannelByNameForTeamName(context.Background(), model.LegacyDefaultChannelName, th.BasicTeam.Name, "")
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("a community's own town-square comes first", func(t *testing.T) {
		team := th.CreateTeam(t)
		th.LinkUserToTeam(t, th.BasicUser, team)
		own, _, err := th.SystemAdminClient.CreateChannel(context.Background(), &model.Channel{TeamId: team.Id, Name: model.LegacyDefaultChannelName, DisplayName: "Old square", Type: model.ChannelTypeOpen})
		require.NoError(t, err)

		channel, _, err := client.GetChannelByName(context.Background(), model.LegacyDefaultChannelName, team.Id, "")
		require.NoError(t, err)
		require.Equal(t, own.Id, channel.Id)
	})
}
