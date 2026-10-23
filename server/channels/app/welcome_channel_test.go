// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// postTypes lists the types of the system messages of a channel, newest first.
func postTypes(t *testing.T, th *TestHelper, channelID string) []string {
	t.Helper()
	list, appErr := th.App.GetPosts(th.Context, channelID, 0, 200)
	require.Nil(t, appErr)
	var types []string
	for _, id := range list.Order {
		if post := list.Posts[id]; post.Type != "" {
			types = append(types, post.Type)
		}
	}
	return types
}

func countType(types []string, postType string) int {
	n := 0
	for _, ty := range types {
		if ty == postType {
			n++
		}
	}
	return n
}

// Antimatter: new communities get General, where people land, and Welcome, where they're told who joins and leaves.
func TestWelcomeChannel(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.ExperimentalEnableDefaultChannelLeaveJoinMessages = true
	})

	team := th.CreateTeam(t)
	general, appErr := th.App.GetChannelByName(th.Context, model.DefaultChannelName, team.Id, false)
	require.Nil(t, appErr)
	welcome, appErr := th.App.GetChannelByName(th.Context, model.WelcomeChannelName, team.Id, false)
	require.Nil(t, appErr)

	t.Run("a new community has General and Welcome, and no Off-Topic", func(t *testing.T) {
		assert.Equal(t, welcome.Id, team.WelcomeChannelId)
		fetched, appErr := th.App.GetTeam(team.Id)
		require.Nil(t, appErr)
		assert.Equal(t, welcome.Id, fetched.WelcomeChannelId)
		assert.False(t, fetched.DisableJoinLeaveMessages)
		assert.Equal(t, model.ChannelTypeOpen, welcome.Type)
		assert.NotEmpty(t, welcome.Purpose)
		_, appErr = th.App.GetChannelByName(th.Context, "off-topic", team.Id, false)
		require.NotNil(t, appErr, "Welcome replaces Off-Topic")

		got := th.App.GetJoinLeaveChannelForTeam(th.Context, fetched)
		require.NotNil(t, got)
		assert.Equal(t, welcome.Id, got.Id)
		assert.Equal(t, general.Id, th.App.GetDefaultChannelForTeam(th.Context, fetched).Id, "people land in General")
	})

	t.Run("joining and leaving are told in Welcome, not in General", func(t *testing.T) {
		user := th.CreateUser(t)
		th.LinkUserToTeam(t, user, team)
		assert.True(t, isChannelMember(th, welcome.Id, user.Id), "everyone is in Welcome")
		assert.True(t, isChannelMember(th, general.Id, user.Id))
		assert.Equal(t, 1, countType(postTypes(t, th, welcome.Id), model.PostTypeJoinTeam))
		generalTypes := postTypes(t, th, general.Id)
		assert.Zero(t, countType(generalTypes, model.PostTypeJoinTeam))
		assert.Zero(t, countType(generalTypes, model.PostTypeJoinChannel))

		require.Nil(t, th.App.RemoveUserFromTeam(th.Context, team.Id, user.Id, user.Id))
		assert.Equal(t, 1, countType(postTypes(t, th, welcome.Id), model.PostTypeLeaveTeam))
		assert.Zero(t, countType(postTypes(t, th, general.Id), model.PostTypeLeaveTeam))
	})

	t.Run("the admins can turn them off", func(t *testing.T) {
		before := postTypes(t, th, welcome.Id)
		patched, appErr := th.App.PatchTeam(team.Id, &model.TeamPatch{DisableJoinLeaveMessages: model.NewPointer(true)})
		require.Nil(t, appErr)
		assert.True(t, patched.DisableJoinLeaveMessages)
		assert.Nil(t, th.App.GetJoinLeaveChannelForTeam(th.Context, patched))

		user := th.CreateUser(t)
		th.LinkUserToTeam(t, user, team)
		assert.True(t, isChannelMember(th, welcome.Id, user.Id), "Welcome stays a channel everyone is in")
		require.Nil(t, th.App.RemoveUserFromTeam(th.Context, team.Id, user.Id, user.Id))
		assert.Equal(t, before, postTypes(t, th, welcome.Id), "nothing told")
		assert.Zero(t, countType(postTypes(t, th, general.Id), model.PostTypeJoinTeam))

		_, appErr = th.App.PatchTeam(team.Id, &model.TeamPatch{DisableJoinLeaveMessages: model.NewPointer(false)})
		require.Nil(t, appErr)
	})

	t.Run("another public channel of the community can take them", func(t *testing.T) {
		private := th.CreatePrivateChannel(t, team)
		_, appErr := th.App.PatchTeam(team.Id, &model.TeamPatch{WelcomeChannelId: model.NewPointer(private.Id)})
		require.NotNil(t, appErr, "a private channel isn't for everyone")
		assert.Equal(t, "app.team.welcome_channel.app_error", appErr.Id)
		_, appErr = th.App.PatchTeam(team.Id, &model.TeamPatch{WelcomeChannelId: model.NewPointer(th.BasicChannel.Id)})
		require.NotNil(t, appErr, "nor is another community's")

		early := th.CreateUser(t)
		th.LinkUserToTeam(t, early, team)
		lobby := th.CreateChannel(t, team)
		_, appErr = th.App.PatchTeam(team.Id, &model.TeamPatch{WelcomeChannelId: model.NewPointer(lobby.Id)})
		require.Nil(t, appErr)
		require.Eventually(t, func() bool { return isChannelMember(th, lobby.Id, early.Id) }, 10*time.Second, 50*time.Millisecond, "everyone is in it")

		user := th.CreateUser(t)
		th.LinkUserToTeam(t, user, team)
		assert.True(t, isChannelMember(th, lobby.Id, user.Id))
		assert.Equal(t, 1, countType(postTypes(t, th, lobby.Id), model.PostTypeJoinTeam))

		// Archived, it tells nothing anymore.
		require.Nil(t, th.App.DeleteChannel(th.Context, lobby, th.SystemAdminUser.Id))
		fetched, appErr := th.App.GetTeam(team.Id)
		require.Nil(t, appErr)
		assert.Nil(t, th.App.GetJoinLeaveChannelForTeam(th.Context, fetched))

		_, appErr = th.App.PatchTeam(team.Id, &model.TeamPatch{WelcomeChannelId: model.NewPointer(welcome.Id)})
		require.Nil(t, appErr)
	})

	t.Run("a community made before Welcome tells them in its default channel, as before", func(t *testing.T) {
		older := th.CreateTeam(t)
		older, appErr := th.App.PatchTeam(older.Id, &model.TeamPatch{WelcomeChannelId: model.NewPointer("")})
		require.Nil(t, appErr)
		assert.Empty(t, older.WelcomeChannelId)
		olderGeneral, appErr := th.App.GetChannelByName(th.Context, model.DefaultChannelName, older.Id, false)
		require.Nil(t, appErr)

		user := th.CreateUser(t)
		th.LinkUserToTeam(t, user, older)
		assert.Equal(t, 1, countType(postTypes(t, th, olderGeneral.Id), model.PostTypeJoinTeam))
		require.Nil(t, th.App.RemoveUserFromTeam(th.Context, older.Id, user.Id, user.Id))
		assert.Equal(t, 1, countType(postTypes(t, th, olderGeneral.Id), model.PostTypeLeaveTeam))
	})

	t.Run("the old default channels stay default channels", func(t *testing.T) {
		names := th.App.DefaultChannelNames(th.Context)
		assert.Contains(t, names, model.DefaultChannelName)
		assert.Contains(t, names, model.WelcomeChannelName)
		assert.Contains(t, names, "off-topic", "communities that have Off-Topic keep it as a default channel")
	})
}

// Antimatter: where everyone is in all the channels, channels don't tell who joins or leaves them; the community's
// Welcome channel tells who joins and leaves the community.
func TestAllChannelsCommunityChannelMembershipMessages(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.ExperimentalEnableDefaultChannelLeaveJoinMessages = true
	})

	guild := allChannelsTeam(t, th)
	th.LinkUserToTeam(t, th.BasicUser, guild)
	welcome, appErr := th.App.GetChannelByName(th.Context, model.WelcomeChannelName, guild.Id, false)
	require.Nil(t, appErr)

	created, appErr := th.App.CreateChannelWithUser(th.Context, &model.Channel{TeamId: guild.Id, Name: "c" + model.NewId(), DisplayName: "Created", Type: model.ChannelTypeOpen}, th.BasicUser.Id)
	require.Nil(t, appErr)
	assert.Zero(t, countType(postTypes(t, th, created.Id), model.PostTypeJoinChannel), "no message for its creator joining it")

	joined := countType(postTypes(t, th, welcome.Id), model.PostTypeJoinTeam)
	user := th.CreateUser(t)
	th.LinkUserToTeam(t, user, guild)
	assert.Equal(t, joined+1, countType(postTypes(t, th, welcome.Id), model.PostTypeJoinTeam), "joining the community is told in Welcome")
	for _, channelID := range []string{created.Id, welcome.Id} {
		assert.Zero(t, countType(postTypes(t, th, channelID), model.PostTypeJoinChannel), "joining its channels isn't")
	}

	private := th.CreatePrivateChannel(t, guild)
	require.Nil(t, th.App.PostAddToChannelMessage(th.Context, th.BasicUser, user, private, ""))
	require.Nil(t, th.App.postLeaveChannelMessage(th.Context, user, private))
	require.Nil(t, th.App.postRemoveFromChannelMessage(th.Context, th.BasicUser.Id, user, private))
	types := postTypes(t, th, private.Id)
	for _, postType := range []string{model.PostTypeAddToChannel, model.PostTypeLeaveChannel, model.PostTypeRemoveFromChannel} {
		assert.Zero(t, countType(types, postType), postType)
	}

	// Elsewhere, channels still tell.
	other := th.CreateTeam(t)
	th.LinkUserToTeam(t, th.BasicUser, other)
	th.LinkUserToTeam(t, user, other)
	otherPrivate := th.CreatePrivateChannel(t, other)
	require.Nil(t, th.App.PostAddToChannelMessage(th.Context, th.BasicUser, user, otherPrivate, ""))
	assert.Equal(t, 1, countType(postTypes(t, th, otherPrivate.Id), model.PostTypeAddToChannel))
}

// Antimatter: the default role every member of a community has, which can't be deleted nor given or taken away.
func TestTeamDefaultRole(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCustomGroups = true })

	team := th.CreateTeam(t)
	early := th.CreateUser(t)
	th.LinkUserToTeam(t, early, team)
	guest := th.CreateGuest(t)
	th.LinkUserToTeam(t, guest, team)
	assert.Empty(t, team.DefaultRoleId, "made the first time the roles are managed")

	role, appErr := th.App.EnsureTeamDefaultRole(th.Context, team.Id)
	require.Nil(t, appErr)
	assert.True(t, role.IsRole())
	assert.Equal(t, team.Id, role.TeamId)
	assert.Equal(t, model.GroupPositionMax, role.Position, "below the other roles")
	assert.False(t, role.AllowReference, "nobody mentions everyone through it")
	assert.False(t, role.SelfAssignable)
	assert.NotEmpty(t, role.DisplayName)

	fetched, appErr := th.App.GetTeam(team.Id)
	require.Nil(t, appErr)
	assert.Equal(t, role.Id, fetched.DefaultRoleId)
	assert.True(t, th.App.IsTeamDefaultRole(role))

	again, appErr := th.App.EnsureTeamDefaultRole(th.Context, team.Id)
	require.Nil(t, appErr)
	assert.Equal(t, role.Id, again.Id, "one default role")

	require.Eventually(t, func() bool { return th.App.IsGroupMember(role.Id, early.Id) }, 10*time.Second, 50*time.Millisecond, "the members already there get it")
	assert.False(t, th.App.IsGroupMember(role.Id, guest.Id), "guests don't")

	user := th.CreateUser(t)
	th.LinkUserToTeam(t, user, team)
	assert.True(t, th.App.IsGroupMember(role.Id, user.Id), "newcomers get it")
	assert.Equal(t, -1, th.App.highestTeamRolePosition(user.Id, team.Id), "it doesn't rank")
	assert.Equal(t, 0, th.App.nextTeamRolePosition(team.Id), "new roles go above it")

	require.Nil(t, th.App.RemoveUserFromTeam(th.Context, team.Id, user.Id, user.Id))
	assert.False(t, th.App.IsGroupMember(role.Id, user.Id), "leaving the community leaves it")

	appErr = th.App.CheckTeamDefaultRoleChange(role, nil, true, false)
	require.NotNil(t, appErr)
	assert.Equal(t, "app.team_role.default_role.delete.app_error", appErr.Id)
	appErr = th.App.CheckTeamDefaultRoleChange(role, nil, false, true)
	require.NotNil(t, appErr)
	assert.Equal(t, "app.team_role.default_role.members.app_error", appErr.Id)
	for _, patch := range []*model.GroupPatch{
		{Position: model.NewPointer(0)},
		{SelfAssignable: model.NewPointer(true)},
		{AllowReference: model.NewPointer(true)},
		{Name: model.NewPointer("everyone")},
	} {
		appErr = th.App.CheckTeamDefaultRoleChange(role, patch, false, false)
		require.NotNil(t, appErr)
		assert.Equal(t, "app.team_role.default_role.patch.app_error", appErr.Id)
	}
	assert.Nil(t, th.App.CheckTeamDefaultRoleChange(role, &model.GroupPatch{Color: model.NewPointer("#3498db"), Hoist: model.NewPointer(true), DisplayName: model.NewPointer("Everyone")}, false, false), "its looks are the admins'")

	other, appErr := th.App.CreateGroup(&model.Group{Name: model.NewPointer("r" + model.NewId()), DisplayName: "Other", Source: model.GroupSourceCustom, AllowReference: true, TeamId: team.Id})
	require.Nil(t, appErr)
	assert.Nil(t, th.App.CheckTeamDefaultRoleChange(other, nil, true, true), "other roles are as before")

	// One deleted before deleting it was refused is replaced.
	_, err := th.Store.Group().Delete(role.Id)
	require.NoError(t, err)
	replaced, appErr := th.App.EnsureTeamDefaultRole(th.Context, team.Id)
	require.Nil(t, appErr)
	assert.NotEqual(t, role.Id, replaced.Id)
	fetched, appErr = th.App.GetTeam(team.Id)
	require.Nil(t, appErr)
	assert.Equal(t, replaced.Id, fetched.DefaultRoleId)
}

// Antimatter: a community's address can be left to the server, made from its name.
func TestFreeTeamName(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t)

	assert.Equal(t, "detecteur-physics", teamNameFromDisplayName("Détecteur  Physics!"))
	assert.Equal(t, "", teamNameFromDisplayName("日本語"))
	assert.Equal(t, "", teamNameFromDisplayName("a"), "too short")
	assert.Equal(t, "", teamNameFromDisplayName("API people"), "reserved")
	assert.LessOrEqual(t, len(teamNameFromDisplayName("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")), model.TeamNameMaxLength-4)

	suffix := model.NewId()[:8]
	name, err := th.App.FreeTeamName("Lumen " + suffix)
	require.NoError(t, err)
	assert.Equal(t, "lumen-"+suffix, name)

	team := &model.Team{DisplayName: "Lumen " + suffix, Name: name, Email: "l@example.com", Type: model.TeamOpen}
	_, appErr := th.App.CreateTeam(th.Context, team)
	require.Nil(t, appErr)
	name, err = th.App.FreeTeamName("Lumen " + suffix)
	require.NoError(t, err)
	assert.Equal(t, "lumen-"+suffix+"-2", name, "a taken address gets a number")

	name, err = th.App.FreeTeamName("日本語")
	require.NoError(t, err)
	assert.True(t, model.IsValidTeamName(name), "a name without Latin letters gets a random address")
	assert.False(t, slices.Contains([]string{"", "-"}, name))
}
