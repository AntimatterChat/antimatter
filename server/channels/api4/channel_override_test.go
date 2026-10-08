// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// Antimatter channel overrides and role-gated channels.
func TestChannelOverrides(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	team := th.BasicTeam
	client2 := th.CreateClient()
	_, _, err := client2.Login(context.Background(), th.BasicUser2.Email, th.BasicUser2.Password)
	require.NoError(t, err)

	role := func(t *testing.T, name string, userIDs ...string) *model.Group {
		t.Helper()
		created, _, err := th.SystemAdminClient.CreateGroup(context.Background(), &model.Group{
			Name: new(name + model.NewId()[:8]), DisplayName: name, Source: model.GroupSourceCustom, AllowReference: true, TeamId: team.Id,
		})
		require.NoError(t, err)
		if len(userIDs) > 0 {
			_, _, err = th.SystemAdminClient.UpsertGroupMembers(context.Background(), created.Id, &model.GroupModifyMembers{UserIds: userIDs})
			require.NoError(t, err)
		}
		return created
	}
	put := func(client *model.Client4, path string, body any) (*http.Response, error) {
		data, err := json.Marshal(body)
		require.NoError(t, err)
		return client.DoAPIPut(context.Background(), path, string(data))
	}
	post := func(client *model.Client4, channelID string) error {
		_, _, err := client.CreatePost(context.Background(), &model.Post{ChannelId: channelID, Message: "hello"})
		return err
	}

	t.Run("a role's deny takes posting away in that channel only, and a person's allow gives it back", func(t *testing.T) {
		channel := th.CreatePublicChannel(t)
		th.AddUserToChannel(t, th.BasicUser2, channel)
		muted := role(t, "muted", th.BasicUser2.Id)

		resp, err := put(th.SystemAdminClient, "/channels/"+channel.Id+"/overrides", model.ChannelOverride{TargetType: model.ChannelOverrideTargetRole, TargetId: muted.Id, Deny: []string{model.PermissionCreatePost.Id}})
		require.NoError(t, err)
		resp.Body.Close()

		require.Error(t, post(client2, channel.Id), "the role can't post here")
		require.NoError(t, post(th.Client, channel.Id), "others still can")
		require.NoError(t, post(client2, th.BasicChannel.Id), "the role posts elsewhere")

		resp, err = put(th.SystemAdminClient, "/channels/"+channel.Id+"/overrides", model.ChannelOverride{TargetType: model.ChannelOverrideTargetUser, TargetId: th.BasicUser2.Id, Allow: []string{model.PermissionCreatePost.Id}})
		require.NoError(t, err)
		resp.Body.Close()
		require.NoError(t, post(client2, channel.Id))
	})

	t.Run("only the channel's or team's admins set overrides, for roles of the team", func(t *testing.T) {
		channel := th.CreatePublicChannel(t)
		th.AddUserToChannel(t, th.BasicUser2, channel)
		r := role(t, "watchers")

		resp, err := put(client2, "/channels/"+channel.Id+"/overrides", model.ChannelOverride{TargetType: model.ChannelOverrideTargetRole, TargetId: r.Id, Deny: []string{model.PermissionCreatePost.Id}})
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)

		resp, err = put(th.SystemAdminClient, "/channels/"+channel.Id+"/overrides", model.ChannelOverride{TargetType: model.ChannelOverrideTargetRole, TargetId: model.NewId(), Deny: []string{model.PermissionCreatePost.Id}})
		require.Error(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("a role-gated channel is for its roles' people, and follows them", func(t *testing.T) {
		channel := th.CreatePrivateChannel(t)
		outsider := th.CreateUser(t)
		th.LinkUserToTeam(t, outsider, team)
		th.AddUserToChannel(t, outsider, channel)
		insiders := role(t, "insiders", th.BasicUser2.Id)

		resp, err := put(th.SystemAdminClient, "/channels/"+channel.Id+"/role_access", map[string][]string{"group_ids": {insiders.Id}})
		require.NoError(t, err)
		resp.Body.Close()

		isMember := func(userID string) bool {
			_, err := th.App.GetChannelMember(th.Context, channel.Id, userID)
			return err == nil
		}
		assert.True(t, isMember(th.BasicUser2.Id), "the role's people join")
		assert.False(t, isMember(outsider.Id), "others leave")

		newcomer := th.CreateUser(t)
		th.LinkUserToTeam(t, newcomer, team)
		_, _, err = th.SystemAdminClient.UpsertGroupMembers(context.Background(), insiders.Id, &model.GroupModifyMembers{UserIds: []string{newcomer.Id}})
		require.NoError(t, err)
		assert.True(t, isMember(newcomer.Id), "joining the role joins the channel")

		_, _, err = th.SystemAdminClient.DeleteGroupMembers(context.Background(), insiders.Id, &model.GroupModifyMembers{UserIds: []string{newcomer.Id}})
		require.NoError(t, err)
		assert.False(t, isMember(newcomer.Id), "leaving the role leaves the channel")
	})

	t.Run("only private channels are role-gated", func(t *testing.T) {
		channel := th.CreatePublicChannel(t)
		r := role(t, "public")
		resp, err := put(th.SystemAdminClient, "/channels/"+channel.Id+"/role_access", map[string][]string{"group_ids": {r.Id}})
		require.Error(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}
