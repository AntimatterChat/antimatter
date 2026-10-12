// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestModeration(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	team := th.BasicTeam

	loggedIn := func(t *testing.T, user *model.User) *model.Client4 {
		t.Helper()
		client := th.CreateClient()
		_, _, err := client.Login(context.Background(), user.Email, user.Password)
		require.NoError(t, err)
		return client
	}
	member := func(t *testing.T) (*model.User, *model.Client4) {
		t.Helper()
		user := th.CreateUser(t)
		th.LinkUserToTeam(t, user, team)
		th.AddUserToChannel(t, user, th.BasicChannel)
		return user, loggedIn(t, user)
	}
	timeOut := func(t *testing.T, client *model.Client4, userID string, until int64, reason string) (*http.Response, error) {
		t.Helper()
		data, err := json.Marshal(model.MemberTimeoutRequest{Until: until, Reason: reason})
		require.NoError(t, err)
		resp, err := client.DoAPIPut(context.Background(), "/teams/"+team.Id+"/members/"+userID+"/timeout", string(data))
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		return resp, err
	}
	post := func(client *model.Client4, channelID string) error {
		_, _, err := client.CreatePost(context.Background(), &model.Post{ChannelId: channelID, Message: "hello"})
		return err
	}
	inAnHour := func() int64 { return model.GetMillis() + time.Hour.Milliseconds() }
	newRole := func(t *testing.T, position int, permissions []string, userIDs ...string) *model.Group {
		t.Helper()
		role, _, err := th.SystemAdminClient.CreateGroup(context.Background(), &model.Group{Name: new("mods" + model.NewId()[:8]), DisplayName: "mods", Source: model.GroupSourceCustom, AllowReference: true, TeamId: team.Id, Position: position})
		require.NoError(t, err)
		if len(permissions) > 0 {
			data, _ := json.Marshal(map[string][]string{"permissions": permissions})
			resp, err := th.SystemAdminClient.DoAPIPut(context.Background(), "/groups/"+role.Id+"/team_role/permissions", string(data))
			require.NoError(t, err)
			resp.Body.Close()
		}
		if len(userIDs) > 0 {
			_, _, err = th.SystemAdminClient.UpsertGroupMembers(context.Background(), role.Id, &model.GroupModifyMembers{UserIds: userIDs})
			require.NoError(t, err)
		}
		return role
	}

	t.Run("a timeout keeps someone from speaking in the team, until it's lifted", func(t *testing.T) {
		user, client := member(t)
		require.NoError(t, post(client, th.BasicChannel.Id))

		_, err := timeOut(t, th.SystemAdminClient, user.Id, inAnHour(), "flooding")
		require.NoError(t, err)
		require.Error(t, post(client, th.BasicChannel.Id), "no messages")
		target, _, err := th.Client.CreatePost(context.Background(), &model.Post{ChannelId: th.BasicChannel.Id, Message: "react to me"})
		require.NoError(t, err)
		_, _, err = client.SaveReaction(context.Background(), &model.Reaction{UserId: user.Id, PostId: target.Id, EmojiName: "smile"})
		require.Error(t, err, "no reactions")
		_, _, err = client.GetPostsForChannel(context.Background(), th.BasicChannel.Id, 0, 10, "", false, false)
		require.NoError(t, err, "reading stays")

		dm, _, err := client.CreateDirectChannel(context.Background(), user.Id, th.BasicUser.Id)
		require.NoError(t, err)
		require.NoError(t, post(client, dm.Id), "direct messages aren't the team's")

		_, err = timeOut(t, th.SystemAdminClient, user.Id, 0, "")
		require.NoError(t, err)
		require.NoError(t, post(client, th.BasicChannel.Id))
	})

	t.Run("an override of the channel doesn't lift a timeout", func(t *testing.T) {
		user, client := member(t)
		data, _ := json.Marshal(model.ChannelOverride{TargetType: model.ChannelOverrideTargetUser, TargetId: user.Id, Allow: []string{model.PermissionCreatePost.Id}})
		resp, err := th.SystemAdminClient.DoAPIPut(context.Background(), "/channels/"+th.BasicChannel.Id+"/overrides", string(data))
		require.NoError(t, err)
		resp.Body.Close()

		_, err = timeOut(t, th.SystemAdminClient, user.Id, inAnHour(), "")
		require.NoError(t, err)
		require.Error(t, post(client, th.BasicChannel.Id))
	})

	t.Run("the team's members see who's timed out; its moderators, the mod view", func(t *testing.T) {
		user, client := member(t)
		require.NoError(t, post(client, th.BasicChannel.Id))
		_, err := timeOut(t, th.SystemAdminClient, user.Id, inAnHour(), "spam")
		require.NoError(t, err)

		resp, err := th.Client.DoAPIGet(context.Background(), "/teams/"+team.Id+"/timeouts", "")
		require.NoError(t, err)
		var timeouts []*model.MemberTimeout
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&timeouts))
		resp.Body.Close()
		found := false
		for _, timeout := range timeouts {
			found = found || (timeout.UserId == user.Id && timeout.Reason == "spam")
		}
		assert.True(t, found)

		resp, err = th.Client.DoAPIGet(context.Background(), "/teams/"+team.Id+"/members/"+user.Id+"/moderation", "")
		require.Error(t, err, "members don't get the mod view")
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)

		resp, err = th.SystemAdminClient.DoAPIGet(context.Background(), "/teams/"+team.Id+"/members/"+user.Id+"/moderation", "")
		require.NoError(t, err)
		var moderation model.MemberModeration
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&moderation))
		resp.Body.Close()
		assert.NotZero(t, moderation.JoinedAt)
		require.NotNil(t, moderation.Timeout)
		assert.Equal(t, "spam", moderation.Timeout.Reason)
		assert.Equal(t, int64(1), moderation.PostsLastDay)
		assert.Equal(t, int64(1), moderation.PostsLastWeek)
	})

	t.Run("members don't time out, nobody times out themselves or the team's admins", func(t *testing.T) {
		user, client := member(t)
		resp, err := timeOut(t, th.Client, user.Id, inAnHour(), "")
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)

		resp, err = timeOut(t, client, user.Id, inAnHour(), "")
		require.Error(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		admin, _ := member(t)
		_, appErr := th.App.UpdateTeamMemberSchemeRoles(th.Context, team.Id, admin.Id, false, true, true)
		require.Nil(t, appErr)
		resp, err = timeOut(t, th.SystemAdminClient, admin.Id, inAnHour(), "")
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("a timeout ends in the future, within 28 days", func(t *testing.T) {
		user, _ := member(t)
		resp, err := timeOut(t, th.SystemAdminClient, user.Id, model.GetMillis()-1, "")
		require.Error(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		resp, err = timeOut(t, th.SystemAdminClient, user.Id, model.GetMillis()+model.MemberTimeoutMaxDuration.Milliseconds()+time.Hour.Milliseconds(), "")
		require.Error(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("those who may remove members moderate those below their own highest role", func(t *testing.T) {
		mod, modClient := member(t)
		newRole(t, 1, []string{model.PermissionRemoveUserFromTeam.Id}, mod.Id)
		plain, _ := member(t)
		above, _ := member(t)
		newRole(t, 0, nil, above.Id)
		peer, _ := member(t)
		newRole(t, 1, nil, peer.Id)

		_, err := timeOut(t, modClient, plain.Id, inAnHour(), "")
		require.NoError(t, err, "someone without roles")
		resp, err := timeOut(t, modClient, above.Id, inAnHour(), "")
		require.Error(t, err, "someone above")
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		resp, err = timeOut(t, modClient, peer.Id, inAnHour(), "")
		require.Error(t, err, "someone at the same place")
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)

		resp, err = modClient.DoAPIGet(context.Background(), "/teams/"+team.Id+"/members/"+plain.Id+"/moderation", "")
		require.NoError(t, err, "they get the mod view")
		resp.Body.Close()
	})

	t.Run("those who may remove members only remove those below their own highest role", func(t *testing.T) {
		mod, modClient := member(t)
		newRole(t, 1, []string{model.PermissionRemoveUserFromTeam.Id}, mod.Id)
		plain, _ := member(t)
		above, _ := member(t)
		newRole(t, 0, nil, above.Id)
		peer, _ := member(t)
		newRole(t, 1, nil, peer.Id)
		admin, _ := member(t)
		_, err := th.SystemAdminClient.UpdateTeamMemberRoles(context.Background(), team.Id, admin.Id, model.TeamUserRoleId+" "+model.TeamAdminRoleId)
		require.NoError(t, err)

		_, err = modClient.RemoveTeamMember(context.Background(), team.Id, plain.Id)
		require.NoError(t, err, "someone without roles")
		for name, user := range map[string]*model.User{"someone above": above, "someone at the same place": peer, "a team admin": admin} {
			resp, err := modClient.RemoveTeamMember(context.Background(), team.Id, user.Id)
			require.Error(t, err, name)
			assert.Equal(t, http.StatusForbidden, resp.StatusCode, name)
			_, _, err = th.SystemAdminClient.GetTeamMember(context.Background(), team.Id, user.Id, "")
			require.NoError(t, err, "%s is still in the team", name)
		}

		_, err = th.SystemAdminClient.RemoveTeamMember(context.Background(), team.Id, above.Id)
		require.NoError(t, err, "admins remove anyone")
		_, err = th.SystemAdminClient.RemoveTeamMember(context.Background(), team.Id, admin.Id)
		require.NoError(t, err, "admins remove other admins")
	})
}
