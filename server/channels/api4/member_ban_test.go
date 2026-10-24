// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// banTestKit gives the ban and invite link tests their people, roles and requests.
type banTestKit struct {
	th   *TestHelper
	team *model.Team
}

func (k *banTestKit) loggedIn(t *testing.T, user *model.User) *model.Client4 {
	t.Helper()
	client := k.th.CreateClient()
	_, _, err := client.Login(context.Background(), user.Email, user.Password)
	require.NoError(t, err)
	return client
}

// member is someone new in the community and its basic channel.
func (k *banTestKit) member(t *testing.T) (*model.User, *model.Client4) {
	t.Helper()
	user := k.th.CreateUser(t)
	k.th.LinkUserToTeam(t, user, k.team)
	k.th.AddUserToChannel(t, user, k.th.BasicChannel)
	return user, k.loggedIn(t, user)
}

// role makes one of the community's roles, at a place (0 is the top), granting permissions, given to people.
func (k *banTestKit) role(t *testing.T, position int, permissions []string, userIDs ...string) *model.Group {
	t.Helper()
	th := k.th
	role, _, err := th.SystemAdminClient.CreateGroup(context.Background(), &model.Group{Name: new("mods" + model.NewId()[:8]), DisplayName: "mods", Source: model.GroupSourceCustom, AllowReference: true, TeamId: k.team.Id, Position: position})
	require.NoError(t, err)
	if len(permissions) > 0 {
		data, _ := json.Marshal(map[string][]string{"permissions": permissions})
		resp, putErr := th.SystemAdminClient.DoAPIPut(context.Background(), "/groups/"+role.Id+"/team_role/permissions", string(data))
		require.NoError(t, putErr)
		resp.Body.Close()
	}
	if len(userIDs) > 0 {
		_, _, err = th.SystemAdminClient.UpsertGroupMembers(context.Background(), role.Id, &model.GroupModifyMembers{UserIds: userIDs})
		require.NoError(t, err)
	}
	return role
}

// moderator is a member whose role, at a place, lets them remove (and so ban) members.
func (k *banTestKit) moderator(t *testing.T, position int) (*model.User, *model.Client4) {
	t.Helper()
	user, client := k.member(t)
	k.role(t, position, []string{model.PermissionRemoveUserFromTeam.Id}, user.Id)
	return user, client
}

func (k *banTestKit) teamAdmin(t *testing.T) (*model.User, *model.Client4) {
	t.Helper()
	user, client := k.member(t)
	_, err := k.th.SystemAdminClient.UpdateTeamMemberRoles(context.Background(), k.team.Id, user.Id, model.TeamUserRoleId+" "+model.TeamAdminRoleId)
	require.NoError(t, err)
	return user, client
}

// closeResp closes a response's body, when there is a response (a failed request may have none).
func closeResp(resp *http.Response) {
	if resp != nil {
		closeBody(resp)
	}
}

func (k *banTestKit) ban(t *testing.T, client *model.Client4, userID string, request model.MemberBanRequest) (*model.MemberBanResult, *http.Response, error) {
	t.Helper()
	data, err := json.Marshal(request)
	require.NoError(t, err)
	resp, err := client.DoAPIPut(context.Background(), "/teams/"+k.team.Id+"/bans/"+userID, string(data))
	if err != nil {
		closeResp(resp)
		return nil, resp, err
	}
	defer closeResp(resp)
	var result model.MemberBanResult
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	return &result, resp, nil
}

func (k *banTestKit) unban(t *testing.T, client *model.Client4, userID string) (*http.Response, error) {
	t.Helper()
	resp, err := client.DoAPIDelete(context.Background(), "/teams/"+k.team.Id+"/bans/"+userID)
	closeResp(resp)
	return resp, err
}

func (k *banTestKit) bans(t *testing.T, client *model.Client4) ([]*model.MemberBan, *http.Response, error) {
	t.Helper()
	resp, err := client.DoAPIGet(context.Background(), "/teams/"+k.team.Id+"/bans", "")
	if err != nil {
		closeResp(resp)
		return nil, resp, err
	}
	defer closeResp(resp)
	var bans []*model.MemberBan
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&bans))
	return bans, resp, nil
}

func (k *banTestKit) inTeam(t *testing.T, userID string) bool {
	t.Helper()
	member, _, err := k.th.SystemAdminClient.GetTeamMember(context.Background(), k.team.Id, userID, "")
	return err == nil && member.DeleteAt == 0
}

func (k *banTestKit) inChannel(t *testing.T, channelID, userID string) bool {
	t.Helper()
	_, _, err := k.th.SystemAdminClient.GetChannelMember(context.Background(), channelID, userID, "")
	return err == nil
}

func (k *banTestKit) createLink(t *testing.T, client *model.Client4, request model.TeamInviteLinkRequest) (*model.TeamInviteLink, *http.Response, error) {
	t.Helper()
	data, err := json.Marshal(request)
	require.NoError(t, err)
	resp, err := client.DoAPIPost(context.Background(), "/teams/"+k.team.Id+"/invite_links", string(data))
	if err != nil {
		closeResp(resp)
		return nil, resp, err
	}
	defer closeResp(resp)
	var link model.TeamInviteLink
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&link))
	return &link, resp, nil
}

func (k *banTestKit) links(t *testing.T, client *model.Client4) ([]*model.TeamInviteLink, *http.Response, error) {
	t.Helper()
	resp, err := client.DoAPIGet(context.Background(), "/teams/"+k.team.Id+"/invite_links", "")
	if err != nil {
		closeResp(resp)
		return nil, resp, err
	}
	defer closeResp(resp)
	var links []*model.TeamInviteLink
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&links))
	return links, resp, nil
}

func (k *banTestKit) revokeLink(t *testing.T, client *model.Client4, linkID string) (*http.Response, error) {
	t.Helper()
	resp, err := client.DoAPIDelete(context.Background(), "/teams/"+k.team.Id+"/invite_links/"+linkID)
	closeResp(resp)
	return resp, err
}

func (k *banTestKit) linkUses(t *testing.T, linkID string) int {
	t.Helper()
	link, err := k.th.App.Srv().Store().Moderation().GetInviteLink(k.th.Context, linkID)
	require.NoError(t, err)
	return link.Uses
}

func TestMemberBans(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	k := &banTestKit{th: th, team: th.BasicTeam}
	inADay := func() int64 { return model.GetMillis() + 24*time.Hour.Milliseconds() }

	t.Run("a ban removes someone from the community and all its channels, and lists who, by whom, why and until when", func(t *testing.T) {
		user, _ := k.member(t)
		private := th.CreatePrivateChannel(t)
		th.AddUserToChannel(t, user, private)
		admin, adminClient := k.teamAdmin(t)
		until := inADay()

		result, _, err := k.ban(t, adminClient, user.Id, model.MemberBanRequest{Reason: "  raids  ", ExpireAt: until})
		require.NoError(t, err)
		require.NotNil(t, result.Ban)
		assert.Equal(t, "raids", result.Ban.Reason)
		assert.False(t, k.inTeam(t, user.Id))
		assert.False(t, k.inChannel(t, th.BasicChannel.Id, user.Id))
		assert.False(t, k.inChannel(t, private.Id, user.Id), "private channels too")

		bans, _, err := k.bans(t, adminClient)
		require.NoError(t, err)
		var found *model.MemberBan
		for _, ban := range bans {
			if ban.UserId == user.Id {
				found = ban
			}
		}
		require.NotNil(t, found)
		assert.Equal(t, admin.Id, found.CreatedBy)
		assert.Equal(t, "raids", found.Reason)
		assert.Equal(t, until, found.ExpireAt)
		assert.NotZero(t, found.CreateAt)
	})

	t.Run("members don't ban; nobody bans themselves, the community's admins or the system's", func(t *testing.T) {
		user, client := k.member(t)
		other, _ := k.member(t)
		_, resp, err := k.ban(t, client, other.Id, model.MemberBanRequest{})
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		assert.True(t, k.inTeam(t, other.Id))

		_, resp, err = k.ban(t, th.SystemAdminClient, th.SystemAdminUser.Id, model.MemberBanRequest{})
		require.Error(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "not oneself")

		admin, _ := k.teamAdmin(t)
		_, resp, err = k.ban(t, th.SystemAdminClient, admin.Id, model.MemberBanRequest{})
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "not a community admin")
		assert.True(t, k.inTeam(t, admin.Id))

		_, modClient := k.moderator(t, 1)
		_, resp, err = k.ban(t, modClient, th.SystemAdminUser.Id, model.MemberBanRequest{})
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "not a system admin")
		assert.True(t, k.inTeam(t, user.Id))
	})

	t.Run("those who may remove members ban those below their own highest role", func(t *testing.T) {
		_, modClient := k.moderator(t, 1)
		plain, _ := k.member(t)
		above, _ := k.member(t)
		k.role(t, 0, nil, above.Id)
		peer, _ := k.member(t)
		k.role(t, 1, nil, peer.Id)

		_, _, err := k.ban(t, modClient, plain.Id, model.MemberBanRequest{})
		require.NoError(t, err, "someone without roles")
		for name, user := range map[string]*model.User{"someone above": above, "someone at the same place": peer} {
			_, resp, banErr := k.ban(t, modClient, user.Id, model.MemberBanRequest{})
			require.Error(t, banErr, name)
			assert.Equal(t, http.StatusForbidden, resp.StatusCode, name)
			assert.True(t, k.inTeam(t, user.Id), "%s is still in the community", name)
		}
	})

	t.Run("someone who already left can be banned, and can't come back", func(t *testing.T) {
		user, client := k.member(t)
		th.UnlinkUserFromTeam(t, user, k.team)
		_, modClient := k.moderator(t, 1)
		_, _, err := k.ban(t, modClient, user.Id, model.MemberBanRequest{})
		require.NoError(t, err)
		_, resp, err := client.AddTeamMemberFromInvite(context.Background(), "", k.team.InviteId)
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("only those who may ban see the bans", func(t *testing.T) {
		_, memberClient := k.member(t)
		_, resp, err := k.bans(t, memberClient)
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "a member")

		// Someone who manages messages gets the mod view, but not the bans.
		messageMod, messageModClient := k.member(t)
		k.role(t, 1, []string{model.PermissionDeleteOthersPosts.Id}, messageMod.Id)
		_, resp, err = k.bans(t, messageModClient)
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "a moderator of messages only")

		_, modClient := k.moderator(t, 1)
		_, _, err = k.bans(t, modClient)
		require.NoError(t, err, "those who may remove members")
		_, adminClient := k.teamAdmin(t)
		_, _, err = k.bans(t, adminClient)
		require.NoError(t, err, "the community's admins")

		// Another community's admins don't see this one's.
		otherTeam := th.CreateTeamWithClient(t, th.SystemAdminClient)
		outsider := th.CreateUser(t)
		th.LinkUserToTeam(t, outsider, otherTeam)
		_, err = th.SystemAdminClient.UpdateTeamMemberRoles(context.Background(), otherTeam.Id, outsider.Id, model.TeamUserRoleId+" "+model.TeamAdminRoleId)
		require.NoError(t, err)
		_, resp, err = k.bans(t, k.loggedIn(t, outsider))
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "another community's admin")
	})

	t.Run("every way back in is refused while the ban holds", func(t *testing.T) {
		user, client := k.member(t)
		_, adminClient := k.teamAdmin(t)
		link, _, err := k.createLink(t, th.SystemAdminClient, model.TeamInviteLinkRequest{})
		require.NoError(t, err)
		_, _, err = k.ban(t, adminClient, user.Id, model.MemberBanRequest{})
		require.NoError(t, err)

		refused := func(name string, resp *model.Response, err error) {
			t.Helper()
			require.Error(t, err, name)
			assert.Equal(t, http.StatusForbidden, resp.StatusCode, name)
			assert.False(t, k.inTeam(t, user.Id), "%s let them back in", name)
		}

		// Open join.
		_, _, err = th.SystemAdminClient.PatchTeam(context.Background(), k.team.Id, &model.TeamPatch{AllowOpenInvite: model.NewPointer(true)})
		require.NoError(t, err)
		_, resp, err := client.AddTeamMember(context.Background(), k.team.Id, user.Id)
		refused("open join", resp, err)

		// The permanent invite code, and an invite link (whose use isn't taken).
		_, resp, err = client.AddTeamMemberFromInvite(context.Background(), "", k.team.InviteId)
		refused("the invite code", resp, err)
		_, resp, err = client.AddTeamMemberFromInvite(context.Background(), "", link.Id)
		refused("an invite link", resp, err)
		assert.Equal(t, 0, k.linkUses(t, link.Id), "a refused join takes no use")

		// An email invitation.
		token := model.NewToken(model.TokenTypeTeamInvitation, model.MapToJSON(map[string]string{"teamId": k.team.Id, "email": user.Email}))
		require.NoError(t, th.App.Srv().Store().Token().Save(token))
		_, resp, err = client.AddTeamMemberFromInvite(context.Background(), token.Token, "")
		refused("an email invitation", resp, err)

		// Being added, by anyone: a system admin too (they lift the ban first).
		_, resp, err = th.SystemAdminClient.AddTeamMember(context.Background(), k.team.Id, user.Id)
		refused("a system admin adding them", resp, err)
		_, resp, err = adminClient.AddTeamMember(context.Background(), k.team.Id, user.Id)
		refused("a community admin adding them", resp, err)
		results, _, err := th.SystemAdminClient.AddTeamMembersGracefully(context.Background(), k.team.Id, []string{user.Id})
		require.NoError(t, err)
		require.Len(t, results, 1)
		assert.NotNil(t, results[0].Error, "adding several people")
		assert.False(t, k.inTeam(t, user.Id))

		// Group sync and the apps add people through the app: refused too, and so is a channel add that doesn't check
		// the membership first (group-synced channels).
		_, appErr := th.App.AddTeamMember(th.Context, k.team.Id, user.Id)
		require.NotNil(t, appErr, "group sync, apps")
		assert.Equal(t, "app.team.join_user_to_team.banned.app_error", appErr.Id)
		_, appErr = th.App.AddUserToChannel(th.Context, user, th.BasicChannel, true)
		require.NotNil(t, appErr, "a group-synced channel")
		assert.False(t, k.inChannel(t, th.BasicChannel.Id, user.Id))
		_, appErr = th.App.JoinUserToTeam(th.Context, k.team, user, th.SystemAdminUser.Id)
		require.NotNil(t, appErr)
		assert.False(t, k.inTeam(t, user.Id))
	})

	t.Run("a ban ends at its time", func(t *testing.T) {
		user, client := k.member(t)
		_, _, err := k.ban(t, th.SystemAdminClient, user.Id, model.MemberBanRequest{ExpireAt: inADay()})
		require.NoError(t, err)
		_, _, err = client.AddTeamMemberFromInvite(context.Background(), "", k.team.InviteId)
		require.Error(t, err)

		// Its end comes.
		_, err = th.App.Srv().Store().Moderation().SaveBan(th.Context, &model.MemberBan{TeamId: k.team.Id, UserId: user.Id, CreatedBy: th.SystemAdminUser.Id, ExpireAt: model.GetMillis() - 1})
		require.NoError(t, err)
		_, _, err = client.AddTeamMemberFromInvite(context.Background(), "", k.team.InviteId)
		require.NoError(t, err, "an ended ban no longer holds")
		bans, _, err := k.bans(t, th.SystemAdminClient)
		require.NoError(t, err)
		for _, ban := range bans {
			assert.NotEqual(t, user.Id, ban.UserId, "nor is it listed")
		}
	})

	t.Run("a ban's end, reason and purge are checked", func(t *testing.T) {
		user, _ := k.member(t)
		for name, request := range map[string]model.MemberBanRequest{
			"an end in the past":        {ExpireAt: model.GetMillis() - 1},
			"an end beyond a year":      {ExpireAt: model.GetMillis() + model.MemberBanMaxDuration.Milliseconds() + time.Hour.Milliseconds()},
			"a purge beyond seven days": {DeleteMessagesSeconds: int64(model.MemberBanPurgeMaxDuration.Seconds()) + 1},
		} {
			_, resp, err := k.ban(t, th.SystemAdminClient, user.Id, request)
			require.Error(t, err, name)
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode, name)
		}
		assert.True(t, k.inTeam(t, user.Id), "a refused ban removes nobody")
	})

	t.Run("lifting a ban: the community's admins any, moderators their own and those of people not above them", func(t *testing.T) {
		_, lowClient := k.moderator(t, 2)
		_, highClient := k.moderator(t, 1)
		_, adminClient := k.teamAdmin(t)
		_, memberClient := k.member(t)
		banned := func(by *model.Client4) *model.User {
			t.Helper()
			user, _ := k.member(t)
			_, _, err := k.ban(t, by, user.Id, model.MemberBanRequest{})
			require.NoError(t, err)
			return user
		}

		byHigh := banned(highClient)
		resp, err := k.unban(t, memberClient, byHigh.Id)
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "a member")
		resp, err = k.unban(t, lowClient, byHigh.Id)
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "a moderator below the one who banned")

		byAdmin := banned(adminClient)
		resp, err = k.unban(t, highClient, byAdmin.Id)
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "a community admin's ban")

		byLow := banned(lowClient)
		_, err = k.unban(t, highClient, byLow.Id)
		require.NoError(t, err, "a moderator above the one who banned")
		ownBan := banned(lowClient)
		_, err = k.unban(t, lowClient, ownBan.Id)
		require.NoError(t, err, "one's own ban")
		_, err = k.unban(t, adminClient, byHigh.Id)
		require.NoError(t, err, "the community's admins lift any")
		_, err = k.unban(t, th.SystemAdminClient, byAdmin.Id)
		require.NoError(t, err)

		resp, err = k.unban(t, adminClient, byAdmin.Id)
		require.Error(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode, "no ban left")

		// Once lifted, they come back by the invite code.
		client := k.loggedIn(t, byLow)
		_, _, err = client.AddTeamMemberFromInvite(context.Background(), "", k.team.InviteId)
		require.NoError(t, err)
		assert.True(t, k.inTeam(t, byLow.Id))
	})

	t.Run("a ban deletes the person's messages of the time asked, in that community only", func(t *testing.T) {
		user, client := k.member(t)
		otherTeam := th.CreateTeamWithClient(t, th.SystemAdminClient)
		th.LinkUserToTeam(t, user, otherTeam)
		otherChannel, _, err := th.SystemAdminClient.CreateChannel(context.Background(), &model.Channel{TeamId: otherTeam.Id, Name: "other-" + model.NewId()[:8], DisplayName: "Other", Type: model.ChannelTypeOpen})
		require.NoError(t, err)
		th.AddUserToChannel(t, user, otherChannel)
		dm, _, err := client.CreateDirectChannel(context.Background(), user.Id, th.BasicUser.Id)
		require.NoError(t, err)

		post := func(channelID string) *model.Post {
			t.Helper()
			p, _, postErr := client.CreatePost(context.Background(), &model.Post{ChannelId: channelID, Message: "spam"})
			require.NoError(t, postErr)
			return p
		}
		recent := []*model.Post{post(th.BasicChannel.Id), post(th.BasicChannel.Id)}
		reply, _, err := client.CreatePost(context.Background(), &model.Post{ChannelId: th.BasicChannel.Id, RootId: recent[0].Id, Message: "more"})
		require.NoError(t, err)
		elsewhere := post(otherChannel.Id)
		direct := post(dm.Id)
		old, err := th.App.Srv().Store().Post().Save(th.Context, &model.Post{ChannelId: th.BasicChannel.Id, UserId: user.Id, Message: "old", CreateAt: model.GetMillis() - 2*time.Hour.Milliseconds()})
		require.NoError(t, err)
		others, _, err := th.Client.CreatePost(context.Background(), &model.Post{ChannelId: th.BasicChannel.Id, Message: "someone else's"})
		require.NoError(t, err)

		result, _, err := k.ban(t, th.SystemAdminClient, user.Id, model.MemberBanRequest{DeleteMessagesSeconds: 3600})
		require.NoError(t, err)
		assert.Empty(t, result.PurgeJobId, "few messages are deleted at once")
		assert.GreaterOrEqual(t, result.DeletedPosts, 2)

		gone := func(p *model.Post) bool {
			t.Helper()
			_, resp, getErr := th.SystemAdminClient.GetPost(context.Background(), p.Id, "")
			return getErr != nil && resp.StatusCode == http.StatusNotFound
		}
		for _, p := range append(recent, reply) {
			assert.True(t, gone(p), "their recent messages in the community are deleted")
		}
		assert.False(t, gone(old), "not older ones")
		assert.False(t, gone(elsewhere), "not another community's")
		assert.False(t, gone(direct), "not their direct messages")
		assert.False(t, gone(others), "not someone else's")
	})

	t.Run("a large purge is a job", func(t *testing.T) {
		user, _ := k.member(t)
		now := model.GetMillis()
		for i := range 101 {
			_, err := th.App.Srv().Store().Post().Save(th.Context, &model.Post{ChannelId: th.BasicChannel.Id, UserId: user.Id, Message: "flood", CreateAt: now - int64(i+1)*1000})
			require.NoError(t, err)
		}
		result, _, err := k.ban(t, th.SystemAdminClient, user.Id, model.MemberBanRequest{DeleteMessagesSeconds: 3600})
		require.NoError(t, err)
		require.NotEmpty(t, result.PurgeJobId)
		assert.Zero(t, result.DeletedPosts, "none at once")
		job, err := th.App.Srv().Store().Job().Get(th.Context, result.PurgeJobId)
		require.NoError(t, err)
		assert.Equal(t, model.JobTypeBanPurge, job.Type)
		assert.Equal(t, k.team.Id, job.Data["team_id"])
		assert.Equal(t, user.Id, job.Data["user_id"])
		assert.Equal(t, th.SystemAdminUser.Id, job.Data["deleted_by"])
		since, err := strconv.ParseInt(job.Data["since"], 10, 64)
		require.NoError(t, err)
		assert.InDelta(t, now-time.Hour.Milliseconds(), since, float64(time.Minute.Milliseconds()))
	})

	t.Run("what the job runs deletes them all, in batches, in that community only", func(t *testing.T) {
		user, _ := k.member(t)
		otherTeam := th.CreateTeamWithClient(t, th.SystemAdminClient)
		otherChannel, _, err := th.SystemAdminClient.CreateChannel(context.Background(), &model.Channel{TeamId: otherTeam.Id, Name: "other-" + model.NewId()[:8], DisplayName: "Other", Type: model.ChannelTypeOpen})
		require.NoError(t, err)
		now := model.GetMillis()
		for i := range 250 {
			_, err = th.App.Srv().Store().Post().Save(th.Context, &model.Post{ChannelId: th.BasicChannel.Id, UserId: user.Id, Message: "flood", CreateAt: now - int64(i+1)*1000})
			require.NoError(t, err)
		}
		elsewhere, err := th.App.Srv().Store().Post().Save(th.Context, &model.Post{ChannelId: otherChannel.Id, UserId: user.Id, Message: "elsewhere", CreateAt: now - 1000})
		require.NoError(t, err)

		since := now - time.Hour.Milliseconds()
		deleted, appErr := th.App.PurgeMemberPosts(th.Context, k.team.Id, user.Id, since, now+1, th.SystemAdminUser.Id)
		require.Nil(t, appErr)
		assert.Equal(t, 250, deleted)
		left, err := th.App.Srv().Store().Moderation().CountPosts(th.Context, k.team.Id, user.Id, since)
		require.NoError(t, err)
		assert.Zero(t, left)
		_, _, err = th.SystemAdminClient.GetPost(context.Background(), elsewhere.Id, "")
		require.NoError(t, err, "another community's message stays")
	})
}

func TestTeamInviteLinks(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	k := &banTestKit{th: th, team: th.BasicTeam}
	newcomer := func(t *testing.T) (*model.User, *model.Client4) {
		t.Helper()
		user := th.CreateUser(t)
		return user, k.loggedIn(t, user)
	}

	t.Run("those who may invite make links; they see and revoke their own, the community's admins all of them", func(t *testing.T) {
		_, aClient := k.member(t)
		_, bClient := k.member(t)
		_, adminClient := k.teamAdmin(t)
		aLink, resp, err := k.createLink(t, aClient, model.TeamInviteLinkRequest{MaxUses: 5})
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, 5, aLink.MaxUses)
		bLink, _, err := k.createLink(t, bClient, model.TeamInviteLinkRequest{})
		require.NoError(t, err)

		ids := func(links []*model.TeamInviteLink) []string {
			out := []string{}
			for _, link := range links {
				out = append(out, link.Id)
			}
			return out
		}
		aLinks, _, err := k.links(t, aClient)
		require.NoError(t, err)
		assert.Equal(t, []string{aLink.Id}, ids(aLinks), "a member sees their own")
		all, _, err := k.links(t, adminClient)
		require.NoError(t, err)
		assert.Subset(t, ids(all), []string{aLink.Id, bLink.Id}, "an admin sees all")

		resp, err = k.revokeLink(t, aClient, bLink.Id)
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, "not someone else's")
		_, err = k.revokeLink(t, aClient, aLink.Id)
		require.NoError(t, err, "one's own")
		_, err = k.revokeLink(t, adminClient, bLink.Id)
		require.NoError(t, err, "an admin, anyone's")

		// A link of another community isn't revoked through this one.
		otherTeam := th.CreateTeamWithClient(t, th.SystemAdminClient)
		other, _, err := (&banTestKit{th: th, team: otherTeam}).createLink(t, th.SystemAdminClient, model.TeamInviteLinkRequest{})
		require.NoError(t, err)
		resp, revokeErr := k.revokeLink(t, adminClient, other.Id)
		require.Error(t, revokeErr)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("people outside the community neither make nor see its links", func(t *testing.T) {
		_, outsiderClient := newcomer(t)
		_, resp, err := k.createLink(t, outsiderClient, model.TeamInviteLinkRequest{})
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		_, resp, err = k.links(t, outsiderClient)
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("a link's end and uses are checked", func(t *testing.T) {
		for name, request := range map[string]model.TeamInviteLinkRequest{
			"an end in the past":   {ExpireAt: model.GetMillis() - 1},
			"an end beyond a year": {ExpireAt: model.GetMillis() + model.TeamInviteLinkMaxDuration.Milliseconds() + time.Hour.Milliseconds()},
			"too many uses":        {MaxUses: model.TeamInviteLinkMaxUses + 1},
			"negative uses":        {MaxUses: -1},
		} {
			_, resp, err := k.createLink(t, th.SystemAdminClient, request)
			require.Error(t, err, name)
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode, name)
		}
	})

	t.Run("a link lets people in up to its uses; someone already in takes none", func(t *testing.T) {
		link, _, err := k.createLink(t, th.SystemAdminClient, model.TeamInviteLinkRequest{MaxUses: 1})
		require.NoError(t, err)
		first, firstClient := newcomer(t)
		_, _, err = firstClient.AddTeamMemberFromInvite(context.Background(), "", link.Id)
		require.NoError(t, err)
		assert.True(t, k.inTeam(t, first.Id))
		assert.Equal(t, 1, k.linkUses(t, link.Id))

		_, _, err = firstClient.AddTeamMemberFromInvite(context.Background(), "", link.Id)
		require.NoError(t, err, "already in: fine")
		assert.Equal(t, 1, k.linkUses(t, link.Id), "and no use taken")

		second, secondClient := newcomer(t)
		_, resp, err := secondClient.AddTeamMemberFromInvite(context.Background(), "", link.Id)
		require.Error(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		CheckErrorID(t, err, "app.team_invite_link.used_up.app_error")
		assert.False(t, k.inTeam(t, second.Id))
	})

	t.Run("an ended link lets nobody in, nor shows the community", func(t *testing.T) {
		link := &model.TeamInviteLink{Id: model.NewId(), TeamId: k.team.Id, CreatorId: th.SystemAdminUser.Id, CreateAt: model.GetMillis() - 2*time.Hour.Milliseconds(), ExpireAt: model.GetMillis() - time.Hour.Milliseconds()}
		_, err := th.App.Srv().Store().Moderation().SaveInviteLink(th.Context, link)
		require.NoError(t, err)
		user, client := newcomer(t)
		_, resp, err := client.AddTeamMemberFromInvite(context.Background(), "", link.Id)
		require.Error(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		CheckErrorID(t, err, "app.team_invite_link.expired.app_error")
		assert.False(t, k.inTeam(t, user.Id))
		_, _, err = th.CreateClient().GetTeamInviteInfo(context.Background(), link.Id)
		require.Error(t, err)
	})

	t.Run("a revoked link lets nobody in", func(t *testing.T) {
		link, _, err := k.createLink(t, th.SystemAdminClient, model.TeamInviteLinkRequest{})
		require.NoError(t, err)
		_, _, err = th.CreateClient().GetTeamInviteInfo(context.Background(), link.Id)
		require.NoError(t, err, "a working link shows the community it's for")
		_, err = k.revokeLink(t, th.SystemAdminClient, link.Id)
		require.NoError(t, err)

		user, client := newcomer(t)
		_, resp, err := client.AddTeamMemberFromInvite(context.Background(), "", link.Id)
		require.Error(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		assert.False(t, k.inTeam(t, user.Id))
		_, _, err = th.CreateClient().GetTeamInviteInfo(context.Background(), link.Id)
		require.Error(t, err)
		_, _, err = th.CreateClient().CreateUserWithInviteId(context.Background(), &model.User{Email: th.GenerateTestEmail(), Username: GenerateTestUsername(), Password: model.NewTestPassword()}, link.Id)
		require.Error(t, err, "nor signs anyone up")
	})

	t.Run("a link whose creator may no longer invite lets nobody in", func(t *testing.T) {
		creator, creatorClient := k.member(t)
		link, _, err := k.createLink(t, creatorClient, model.TeamInviteLinkRequest{})
		require.NoError(t, err)
		th.UnlinkUserFromTeam(t, creator, k.team)
		user, client := newcomer(t)
		_, _, err = client.AddTeamMemberFromInvite(context.Background(), "", link.Id)
		require.Error(t, err)
		CheckErrorID(t, err, "app.team_invite_link.invalid.app_error")
		assert.False(t, k.inTeam(t, user.Id))
	})

	t.Run("signing up with a link joins the community and takes a use; a used-up link signs nobody up", func(t *testing.T) {
		link, _, err := k.createLink(t, th.SystemAdminClient, model.TeamInviteLinkRequest{MaxUses: 1})
		require.NoError(t, err)
		ruser, _, err := th.CreateClient().CreateUserWithInviteId(context.Background(), &model.User{Email: th.GenerateTestEmail(), Username: GenerateTestUsername(), Password: model.NewTestPassword()}, link.Id)
		require.NoError(t, err)
		assert.True(t, k.inTeam(t, ruser.Id))
		assert.Equal(t, 1, k.linkUses(t, link.Id))

		email := th.GenerateTestEmail()
		_, _, err = th.CreateClient().CreateUserWithInviteId(context.Background(), &model.User{Email: email, Username: GenerateTestUsername(), Password: model.NewTestPassword()}, link.Id)
		require.Error(t, err)
		_, _, err = th.SystemAdminClient.GetUserByEmail(context.Background(), email, "")
		require.Error(t, err, "no account made")
	})

	t.Run("the permanent invite code keeps working alongside the links", func(t *testing.T) {
		_, _, err := k.createLink(t, th.SystemAdminClient, model.TeamInviteLinkRequest{})
		require.NoError(t, err)
		user, client := newcomer(t)
		_, _, err = client.AddTeamMemberFromInvite(context.Background(), "", k.team.InviteId)
		require.NoError(t, err)
		assert.True(t, k.inTeam(t, user.Id))
	})

	t.Run("a ban wins over a link", func(t *testing.T) {
		link, _, err := k.createLink(t, th.SystemAdminClient, model.TeamInviteLinkRequest{MaxUses: 3})
		require.NoError(t, err)
		user, client := k.member(t)
		_, _, err = k.ban(t, th.SystemAdminClient, user.Id, model.MemberBanRequest{})
		require.NoError(t, err)
		_, resp, err := client.AddTeamMemberFromInvite(context.Background(), "", link.Id)
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		assert.Equal(t, 0, k.linkUses(t, link.Id))
	})
}

// TestTeamInviteLinkRights changes the default roles: not in parallel.
func TestTeamInviteLinkRights(t *testing.T) {
	th := Setup(t).InitBasic(t)
	k := &banTestKit{th: th, team: th.BasicTeam}
	defaultRolePermissions := th.SaveDefaultRolePermissions(t)
	defer th.RestoreDefaultRolePermissions(t, defaultRolePermissions)

	_, memberClient := k.member(t)
	link, _, err := k.createLink(t, memberClient, model.TeamInviteLinkRequest{})
	require.NoError(t, err, "members may invite by default")

	th.RemovePermissionFromRole(t, model.PermissionInviteUser.Id, model.TeamUserRoleId)
	_, resp, err := k.createLink(t, memberClient, model.TeamInviteLinkRequest{})
	require.Error(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "without invite_user, no links")
	_, resp, err = k.links(t, memberClient)
	require.Error(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "nor the list")

	// Nor do the links they made let anyone in any more.
	newcomer := th.CreateUser(t)
	_, _, err = k.loggedIn(t, newcomer).AddTeamMemberFromInvite(context.Background(), "", link.Id)
	require.Error(t, err)
	CheckErrorID(t, err, "app.team_invite_link.invalid.app_error")
}
