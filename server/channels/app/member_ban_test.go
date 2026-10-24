// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// banFromTeam bans someone from a community straight in the store, as a system admin would.
func banFromTeam(t *testing.T, th *TestHelper, teamID, userID string) {
	t.Helper()
	_, err := th.App.Srv().Store().Moderation().SaveBan(th.Context, &model.MemberBan{TeamId: teamID, UserId: userID, CreatedBy: th.SystemAdminUser.Id})
	require.NoError(t, err)
}

func memberOfTeamNow(t *testing.T, th *TestHelper, teamID, userID string) bool {
	t.Helper()
	member, err := th.App.Srv().Store().Team().GetMember(th.Context, teamID, userID)
	return err == nil && member.DeleteAt == 0
}

func memberOfChannelNow(t *testing.T, th *TestHelper, channelID, userID string) bool {
	t.Helper()
	_, err := th.App.Srv().Store().Channel().GetMember(th.Context, channelID, userID)
	return err == nil
}

// TestMemberBanRefusesGroupSyncAndApps runs the real group sync and the apps' API against a ban: neither brings a
// banned person into the community or one of its channels.
func TestMemberBanRefusesGroupSyncAndApps(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	t.Run("group sync adds nobody banned to a synced community or channel", func(t *testing.T) {
		team := th.CreateTeam(t)
		channel := th.CreateChannel(t, team)
		group := th.CreateGroup(t)
		banned, other := th.CreateUser(t), th.CreateUser(t)
		for _, user := range []*model.User{banned, other} {
			_, appErr := th.App.UpsertGroupMember(group.Id, user.Id)
			require.Nil(t, appErr)
		}
		banFromTeam(t, th, team.Id, banned.Id)
		_, appErr := th.App.UpsertGroupSyncable(model.NewGroupTeam(group.Id, team.Id, true))
		require.Nil(t, appErr)
		_, appErr = th.App.UpsertGroupSyncable(model.NewGroupChannel(group.Id, channel.Id, true))
		require.Nil(t, appErr)

		// The ban keeps them out without failing the sync, which goes on to the channels for everyone else.
		require.NoError(t, th.App.CreateDefaultMemberships(th.Context, model.CreateDefaultMembershipParams{Since: 0, ReAddRemovedMembers: true}))
		assert.False(t, memberOfTeamNow(t, th, team.Id, banned.Id), "group sync let a banned person into the community")
		assert.False(t, memberOfChannelNow(t, th, channel.Id, banned.Id), "group sync let a banned person into a channel")
		assert.True(t, memberOfTeamNow(t, th, team.Id, other.Id), "the others are synced")
		assert.True(t, memberOfChannelNow(t, th, channel.Id, other.Id))
	})

	t.Run("group sync doesn't put a banned former member into a synced channel", func(t *testing.T) {
		// Their community membership is still there, deleted, so the channel sync skips the community's join and adds
		// them to the channel straight away: the channel add itself must refuse.
		team := th.CreateTeam(t)
		banned := th.CreateUser(t)
		th.LinkUserToTeam(t, banned, team)
		require.Nil(t, th.App.RemoveUserFromTeam(th.Context, team.Id, banned.Id, th.SystemAdminUser.Id))
		banFromTeam(t, th, team.Id, banned.Id)
		channel := th.CreateChannel(t, team)
		group := th.CreateGroup(t)
		_, appErr := th.App.UpsertGroupMember(group.Id, banned.Id)
		require.Nil(t, appErr)
		_, appErr = th.App.UpsertGroupSyncable(model.NewGroupChannel(group.Id, channel.Id, true))
		require.Nil(t, appErr)

		require.NoError(t, th.App.CreateDefaultMemberships(th.Context, model.CreateDefaultMembershipParams{Since: 0, ReAddRemovedMembers: true}))
		assert.False(t, memberOfChannelNow(t, th, channel.Id, banned.Id), "group sync let a banned person into a channel")
		assert.False(t, memberOfTeamNow(t, th, team.Id, banned.Id))
	})

	t.Run("the apps' API adds nobody banned", func(t *testing.T) {
		team := th.CreateTeam(t)
		channel := th.CreateChannel(t, team)
		banned := th.CreateUser(t)
		banFromTeam(t, th, team.Id, banned.Id)
		api := NewPluginAPI(th.App, th.Context, &model.Manifest{Id: "com.example.bans"})

		_, appErr := api.CreateTeamMember(team.Id, banned.Id)
		require.NotNil(t, appErr, "CreateTeamMember")
		assert.Equal(t, "app.team.join_user_to_team.banned.app_error", appErr.Id)
		_, appErr = api.CreateTeamMembers(team.Id, []string{banned.Id}, th.SystemAdminUser.Id)
		require.NotNil(t, appErr, "CreateTeamMembers")
		results, appErr := api.CreateTeamMembersGracefully(team.Id, []string{banned.Id}, th.SystemAdminUser.Id)
		require.Nil(t, appErr)
		require.Len(t, results, 1)
		assert.NotNil(t, results[0].Error, "CreateTeamMembersGracefully")
		_, appErr = api.AddChannelMember(channel.Id, banned.Id)
		require.NotNil(t, appErr, "AddChannelMember")
		_, appErr = api.AddUserToChannel(channel.Id, banned.Id, th.SystemAdminUser.Id)
		require.NotNil(t, appErr, "AddUserToChannel")

		assert.False(t, memberOfTeamNow(t, th, team.Id, banned.Id), "an app let a banned person into the community")
		assert.False(t, memberOfChannelNow(t, th, channel.Id, banned.Id), "an app let a banned person into a channel")
	})
}

// TestTeamInviteLinkConcurrentJoins has many people join by one link at once: never more than its uses get in, and
// the uses counted are those who did.
func TestTeamInviteLinkConcurrentJoins(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	const maxUses, joiners = 3, 12
	team := th.CreateTeam(t)
	link, err := th.App.Srv().Store().Moderation().SaveInviteLink(th.Context, &model.TeamInviteLink{Id: model.NewId(), TeamId: team.Id, CreatorId: th.SystemAdminUser.Id, CreateAt: model.GetMillis(), MaxUses: maxUses})
	require.NoError(t, err)
	users := make([]*model.User, joiners)
	for i := range users {
		users[i] = th.CreateUser(t)
	}

	var wg sync.WaitGroup
	var joined atomic.Int32
	start := make(chan struct{})
	for _, user := range users {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if _, _, appErr := th.App.AddUserToTeamByInviteId(th.Context, link.Id, user.Id); appErr == nil {
				joined.Add(1)
			} else {
				assert.Equal(t, "app.team_invite_link.used_up.app_error", appErr.Id)
			}
		}()
	}
	close(start)
	wg.Wait()

	assert.Equal(t, int32(maxUses), joined.Load(), "exactly the link's uses get in")
	in := 0
	for _, user := range users {
		if memberOfTeamNow(t, th, team.Id, user.Id) {
			in++
		}
	}
	assert.Equal(t, maxUses, in)
	stored, err := th.App.Srv().Store().Moderation().GetInviteLink(th.Context, link.Id)
	require.NoError(t, err)
	assert.Equal(t, maxUses, stored.Uses)
}
