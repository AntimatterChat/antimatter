// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func isChannelMember(th *TestHelper, channelID, userID string) bool {
	_, appErr := th.App.GetChannelMember(th.Context, channelID, userID)
	return appErr == nil
}

func TestCommunityDefaultChannel(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	team := th.CreateTeam(t)
	townSquare, appErr := th.App.GetChannelByName(th.Context, model.DefaultChannelName, team.Id, false)
	require.Nil(t, appErr)

	t.Run("Town Square, unless the community chooses another public channel", func(t *testing.T) {
		got := th.App.GetDefaultChannelForTeam(th.Context, team)
		require.NotNil(t, got)
		assert.Equal(t, townSquare.Id, got.Id)

		private := th.CreatePrivateChannel(t, team)
		_, appErr := th.App.PatchTeam(team.Id, &model.TeamPatch{DefaultChannelId: model.NewPointer(private.Id)})
		require.NotNil(t, appErr, "a private channel isn't for everyone")
		assert.Equal(t, "app.team.default_channel.app_error", appErr.Id)
		_, appErr = th.App.PatchTeam(team.Id, &model.TeamPatch{DefaultChannelId: model.NewPointer(th.BasicChannel.Id)})
		require.NotNil(t, appErr, "nor is another community's")

		early := th.CreateUser(t)
		th.LinkUserToTeam(t, early, team)
		lobby := th.CreateChannel(t, team)
		team, appErr = th.App.PatchTeam(team.Id, &model.TeamPatch{DefaultChannelId: model.NewPointer(lobby.Id)})
		require.Nil(t, appErr)
		require.Eventually(t, func() bool { return isChannelMember(th, lobby.Id, early.Id) }, 10*time.Second, 50*time.Millisecond, "as in Town Square, everyone is in it")
		got = th.App.GetDefaultChannelForTeam(th.Context, team)
		require.NotNil(t, got)
		assert.Equal(t, lobby.Id, got.Id)
		assert.False(t, th.App.IsDefaultChannel(th.Context, townSquare))

		// Newcomers land in it, and Town Square is an ordinary channel they can leave.
		user := th.CreateUser(t)
		th.LinkUserToTeam(t, user, team)
		assert.True(t, isChannelMember(th, lobby.Id, user.Id))
		appErr = th.App.LeaveChannel(th.Context, lobby.Id, user.Id)
		require.NotNil(t, appErr)
		assert.Equal(t, "api.channel.remove.default.app_error", appErr.Id)
		require.Nil(t, th.App.LeaveChannel(th.Context, townSquare.Id, user.Id))

		_, appErr = th.App.PatchTeam(team.Id, &model.TeamPatch{DefaultChannelId: model.NewPointer("")})
		require.Nil(t, appErr)
		team, appErr = th.App.GetTeam(team.Id)
		require.Nil(t, appErr)
	})

	t.Run("the default channels can be archived, and joining, leaving and messages cope", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.ServiceSettings.ExperimentalEnableDefaultChannelLeaveJoinMessages = true
		})
		require.Nil(t, th.App.DeleteChannel(th.Context, townSquare, th.SystemAdminUser.Id))
		offTopic, appErr := th.App.GetChannelByName(th.Context, "off-topic", team.Id, false)
		require.Nil(t, appErr)
		require.Nil(t, th.App.DeleteChannel(th.Context, offTopic, th.SystemAdminUser.Id))
		assert.Nil(t, th.App.GetDefaultChannelForTeam(th.Context, team), "no default channel")

		user := th.CreateUser(t)
		th.LinkUserToTeam(t, user, team)
		assert.False(t, isChannelMember(th, townSquare.Id, user.Id), "nobody joins an archived channel")

		require.Nil(t, th.App.RemoveUserFromTeam(th.Context, team.Id, user.Id, user.Id), "leaving a community without a default channel works")

		// Deleted for good too.
		require.Nil(t, th.App.PermanentDeleteChannel(th.Context, townSquare))
		other := th.CreateUser(t)
		th.LinkUserToTeam(t, other, team)
		require.Nil(t, th.App.RemoveUserFromTeam(th.Context, team.Id, other.Id, th.SystemAdminUser.Id))
	})
}
