// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// allChannelsTeam makes a community that has everyone in all its channels.
func allChannelsTeam(t *testing.T, th *TestHelper) *model.Team {
	t.Helper()
	team := th.CreateTeam(t)
	team, appErr := th.App.PatchTeam(team.Id, &model.TeamPatch{AllChannelsMembership: model.NewPointer(true)})
	require.Nil(t, appErr)
	require.True(t, team.AllChannelsMembership)
	return team
}

func TestCommunitySettingsPersist(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	team := th.CreateTeam(t)
	assert.False(t, team.AllChannelsMembership, "communities made outside the API keep the old behaviour")

	_, appErr := th.App.PatchTeam(team.Id, &model.TeamPatch{AllChannelsMembership: model.NewPointer(true), DefaultChannelId: model.NewPointer(th.CreateChannel(t, team).Id)})
	require.Nil(t, appErr)
	got, appErr := th.App.GetTeam(team.Id)
	require.Nil(t, appErr)
	assert.True(t, got.AllChannelsMembership)
	assert.NotEmpty(t, got.DefaultChannelId)

	// Other changes keep them.
	_, appErr = th.App.PatchTeam(team.Id, &model.TeamPatch{DisplayName: model.NewPointer("Renamed")})
	require.Nil(t, appErr)
	got, appErr = th.App.GetTeam(team.Id)
	require.Nil(t, appErr)
	assert.True(t, got.AllChannelsMembership)
	assert.NotEmpty(t, got.DefaultChannelId)

	// And they're read with the team's other lists.
	teams, appErr := th.App.GetTeamsForUser(th.BasicUser.Id)
	require.Nil(t, appErr)
	for _, other := range teams {
		assert.False(t, other.AllChannelsMembership, "other communities don't have it")
	}

	_, appErr = th.App.PatchTeam(team.Id, &model.TeamPatch{AllChannelsMembership: model.NewPointer(false), DefaultChannelId: model.NewPointer("")})
	require.Nil(t, appErr)
	got, appErr = th.App.GetTeam(team.Id)
	require.Nil(t, appErr)
	assert.False(t, got.AllChannelsMembership)
	assert.Empty(t, got.DefaultChannelId)
}

func TestAllChannelsMembershipJoiningTheCommunity(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	team := allChannelsTeam(t, th)
	open1 := th.CreateChannel(t, team)
	open2 := th.CreateChannel(t, team)
	inviteOnly := th.CreatePrivateChannel(t, team)
	archived := th.CreateChannel(t, team)
	require.Nil(t, th.App.DeleteChannel(th.Context, archived, th.SystemAdminUser.Id))

	user := th.CreateUser(t)
	th.LinkUserToTeam(t, user, team)
	assert.True(t, isChannelMember(th, open1.Id, user.Id))
	assert.True(t, isChannelMember(th, open2.Id, user.Id))
	assert.False(t, isChannelMember(th, inviteOnly.Id, user.Id), "invite-only channels stay so")
	assert.False(t, isChannelMember(th, archived.Id, user.Id))

	guest := th.CreateGuest(t)
	th.LinkUserToTeam(t, guest, team)
	assert.False(t, isChannelMember(th, open1.Id, guest.Id), "guests are in the channels they're added to")

	// Off, joining is as before: the default channels only.
	plain := th.CreateTeam(t)
	open3 := th.CreateChannel(t, plain)
	other := th.CreateUser(t)
	th.LinkUserToTeam(t, other, plain)
	assert.False(t, isChannelMember(th, open3.Id, other.Id))
}

func TestAllChannelsMembershipLeaving(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	team := allChannelsTeam(t, th)
	channel := th.CreateChannel(t, team)
	user := th.CreateUser(t)
	th.LinkUserToTeam(t, user, team)
	require.True(t, isChannelMember(th, channel.Id, user.Id))

	held, appErr := th.App.IsAllChannelsMembership(th.Context, channel, user.Id)
	require.Nil(t, appErr)
	assert.True(t, held)

	appErr = th.App.LeaveChannel(th.Context, channel.Id, user.Id)
	require.NotNil(t, appErr)
	assert.Equal(t, "api.channel.leave.all_channels.app_error", appErr.Id)
	assert.Equal(t, http.StatusBadRequest, appErr.StatusCode)

	appErr = th.App.RemoveUserFromChannel(th.Context, user.Id, th.SystemAdminUser.Id, channel)
	require.NotNil(t, appErr, "they'd come back: moderation goes through the community")
	assert.Equal(t, "api.channel.remove.all_channels.app_error", appErr.Id)
	assert.True(t, isChannelMember(th, channel.Id, user.Id))

	t.Run("the server's own syncs still remove", func(t *testing.T) {
		other := th.CreateChannel(t, team)
		// The community brings them in, in the background (adding them here too raced with it).
		require.Eventually(t, func() bool { return isChannelMember(th, other.Id, user.Id) }, 10*time.Second, 50*time.Millisecond)
		require.Nil(t, th.App.RemoveUserFromChannel(th.Context, user.Id, "", other))
		assert.False(t, isChannelMember(th, other.Id, user.Id))
	})

	t.Run("bots and invite-only channels leave as before", func(t *testing.T) {
		bot := th.CreateBot(t)
		botUser, appErr := th.App.GetUser(th.Context, bot.UserId)
		require.Nil(t, appErr)
		th.LinkUserToTeam(t, botUser, team)
		th.AddUserToChannel(t, botUser, channel)
		require.Nil(t, th.App.LeaveChannel(th.Context, channel.Id, bot.UserId))

		private := th.CreatePrivateChannel(t, team)
		th.AddUserToChannel(t, user, private)
		require.Nil(t, th.App.LeaveChannel(th.Context, private.Id, user.Id))
	})

	t.Run("turned off, people can leave, and keep what they're in", func(t *testing.T) {
		_, appErr := th.App.PatchTeam(team.Id, &model.TeamPatch{AllChannelsMembership: model.NewPointer(false)})
		require.Nil(t, appErr)
		assert.True(t, isChannelMember(th, channel.Id, user.Id))
		require.Nil(t, th.App.LeaveChannel(th.Context, channel.Id, user.Id))
	})
}

func TestAllChannelsMembershipNewChannels(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	team := allChannelsTeam(t, th)
	user := th.CreateUser(t)
	th.LinkUserToTeam(t, user, team)
	guest := th.CreateGuest(t)
	th.LinkUserToTeam(t, guest, team)

	t.Run("a new public channel gets everyone", func(t *testing.T) {
		channel := th.CreateChannel(t, team)
		require.Eventually(t, func() bool { return isChannelMember(th, channel.Id, user.Id) }, 10*time.Second, 50*time.Millisecond)
		assert.False(t, isChannelMember(th, channel.Id, guest.Id))
	})

	t.Run("filling a channel brings its people, past the guests it skips", func(t *testing.T) {
		channel, appErr := th.App.CreateChannel(th.Context, &model.Channel{TeamId: team.Id, Name: "batch-" + model.NewId()[:8], DisplayName: "Batch", Type: model.ChannelTypeOpen, CreatorId: th.BasicUser.Id}, false)
		require.Nil(t, appErr)
		th.App.fillCommunityChannel(th.Context, team, channel)
		assert.True(t, isChannelMember(th, channel.Id, user.Id))
		assert.False(t, isChannelMember(th, channel.Id, guest.Id))
	})

	t.Run("an unarchived channel gets everyone back", func(t *testing.T) {
		channel := th.CreateChannel(t, team)
		require.Eventually(t, func() bool { return isChannelMember(th, channel.Id, user.Id) }, 10*time.Second, 50*time.Millisecond)
		require.Nil(t, th.App.DeleteChannel(th.Context, channel, th.SystemAdminUser.Id))
		channel, appErr := th.App.GetChannel(th.Context, channel.Id)
		require.Nil(t, appErr)

		// Someone went while it was archived.
		require.Nil(t, th.App.RemoveUserFromChannel(th.Context, user.Id, "", channel))
		_, appErr = th.App.RestoreChannel(th.Context, channel, th.SystemAdminUser.Id)
		require.Nil(t, appErr)
		require.Eventually(t, func() bool { return isChannelMember(th, channel.Id, user.Id) }, 10*time.Second, 50*time.Millisecond)
	})

	t.Run("an invite-only private channel doesn't", func(t *testing.T) {
		channel := th.CreatePrivateChannel(t, team)
		th.App.FillCommunityChannelInBackground(th.Context, channel)
		time.Sleep(200 * time.Millisecond)
		assert.False(t, isChannelMember(th, channel.Id, user.Id))
	})
}

func TestAllChannelsMembershipTurnedOn(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	team := th.CreateTeam(t)
	users := []*model.User{th.CreateUser(t), th.CreateUser(t), th.CreateUser(t)}
	for _, u := range users {
		th.LinkUserToTeam(t, u, team)
	}
	channels := []*model.Channel{th.CreateChannel(t, team), th.CreateChannel(t, team)}
	for _, c := range channels {
		require.False(t, isChannelMember(th, c.Id, users[0].Id), "off, people choose their channels")
	}

	_, appErr := th.App.PatchTeam(team.Id, &model.TeamPatch{AllChannelsMembership: model.NewPointer(true)})
	require.Nil(t, appErr)
	require.Eventually(t, func() bool {
		for _, c := range channels {
			for _, u := range users {
				if !isChannelMember(th, c.Id, u.Id) {
					return false
				}
			}
		}
		return true
	}, 20*time.Second, 100*time.Millisecond, "turning it on fills every channel")
}

func TestAllChannelsMembershipRoleGatedChannels(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCustomGroups = true })

	team := allChannelsTeam(t, th)
	insider := th.CreateUser(t)
	outsider := th.CreateUser(t)
	th.LinkUserToTeam(t, insider, team)
	th.LinkUserToTeam(t, outsider, team)

	role, appErr := th.App.CreateGroupWithUserIds(&model.GroupWithUserIds{
		Group: model.Group{
			Name: model.NewPointer("insiders" + model.NewId()[:8]), DisplayName: "Insiders", Source: model.GroupSourceCustom,
			AllowReference: true, TeamId: team.Id,
		},
		UserIds: []string{insider.Id},
	})
	require.Nil(t, appErr)

	channel := th.CreatePrivateChannel(t, team)
	require.Nil(t, th.App.SetChannelRoleAccess(th.Context, channel, []string{role.Id}))
	channel, appErr = th.App.GetChannel(th.Context, channel.Id)
	require.Nil(t, appErr)

	assert.True(t, isChannelMember(th, channel.Id, insider.Id), "the role's people are in it")
	assert.False(t, isChannelMember(th, channel.Id, outsider.Id), "the others aren't")

	held, appErr := th.App.IsAllChannelsMembership(th.Context, channel, insider.Id)
	require.Nil(t, appErr)
	assert.True(t, held)
	appErr = th.App.LeaveChannel(th.Context, channel.Id, insider.Id)
	require.NotNil(t, appErr)
	assert.Equal(t, "api.channel.leave.all_channels.app_error", appErr.Id)

	// Gaining the role brings the channel; losing it takes it away.
	_, appErr = th.App.UpsertGroupMembers(role.Id, []string{outsider.Id})
	require.Nil(t, appErr)
	assert.True(t, isChannelMember(th, channel.Id, outsider.Id))
	_, appErr = th.App.DeleteGroupMembers(role.Id, []string{outsider.Id})
	require.Nil(t, appErr)
	assert.False(t, isChannelMember(th, channel.Id, outsider.Id))

	// Someone who left it before the setting was on comes back when the channels are filled.
	_, appErr = th.App.PatchTeam(team.Id, &model.TeamPatch{AllChannelsMembership: model.NewPointer(false)})
	require.Nil(t, appErr)
	require.Nil(t, th.App.LeaveChannel(th.Context, channel.Id, insider.Id))
	team, appErr = th.App.PatchTeam(team.Id, &model.TeamPatch{AllChannelsMembership: model.NewPointer(true)})
	require.Nil(t, appErr)
	require.Eventually(t, func() bool { return isChannelMember(th, channel.Id, insider.Id) }, 20*time.Second, 100*time.Millisecond)
	assert.False(t, isChannelMember(th, channel.Id, outsider.Id), "filling a role-gated channel brings its roles' people only")
}
