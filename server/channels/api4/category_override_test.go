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

func TestCategoryOverrides(t *testing.T) {
	mainHelper.Parallel(t)
	th := SetupConfig(t, func(cfg *model.Config) {
		cfg.FeatureFlags.ManagedChannelCategories = true
	}).InitBasic(t)
	team := th.BasicTeam
	client2 := th.CreateClient()
	_, _, err := client2.Login(context.Background(), th.BasicUser2.Email, th.BasicUser2.Password)
	require.NoError(t, err)

	put := func(t *testing.T, client *model.Client4, path string, body any) (*http.Response, error) {
		t.Helper()
		data, marshalErr := json.Marshal(body)
		require.NoError(t, marshalErr)
		resp, putErr := client.DoAPIPut(context.Background(), path, string(data))
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		return resp, putErr
	}
	categories := "/teams/" + team.Id + "/server_categories"
	_, err = put(t, th.SystemAdminClient, categories, model.ServerCategoriesUpdate{Categories: []*model.ServerCategory{{Name: "Rules", Locked: true}, {Name: "Community"}}})
	require.NoError(t, err)

	muted, _, err := th.SystemAdminClient.CreateGroup(context.Background(), &model.Group{Name: new("muted" + model.NewId()[:8]), DisplayName: "muted", Source: model.GroupSourceCustom, AllowReference: true, TeamId: team.Id})
	require.NoError(t, err)
	_, _, err = th.SystemAdminClient.UpsertGroupMembers(context.Background(), muted.Id, &model.GroupModifyMembers{UserIds: []string{th.BasicUser2.Id}})
	require.NoError(t, err)

	filed := th.CreatePublicChannel(t)
	th.AddUserToChannel(t, th.BasicUser2, filed)
	require.Nil(t, th.App.SetChannelManagedCategory(th.Context, filed.Id, "Community"))
	unfiled := th.CreatePublicChannel(t)
	th.AddUserToChannel(t, th.BasicUser2, unfiled)

	post := func(channelID string) error {
		_, _, err := client2.CreatePost(context.Background(), &model.Post{ChannelId: channelID, Message: "hello"})
		return err
	}
	react := func(channelID string) error {
		p, _, err := th.Client.CreatePost(context.Background(), &model.Post{ChannelId: channelID, Message: "react to me"})
		require.NoError(t, err)
		_, _, err = client2.SaveReaction(context.Background(), &model.Reaction{UserId: th.BasicUser2.Id, PostId: p.Id, EmojiName: "smile"})
		return err
	}
	deny := model.CategoryOverride{Category: "community", TargetType: model.ChannelOverrideTargetRole, TargetId: muted.Id, Deny: []string{model.PermissionCreatePost.Id, model.PermissionAddReaction.Id}}

	t.Run("only team admins set them, on the team's categories, for its roles and people", func(t *testing.T) {
		resp, err := put(t, th.Client, categories+"/overrides", deny)
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)

		unknown := deny
		unknown.Category = "Nowhere"
		resp, err = put(t, th.SystemAdminClient, categories+"/overrides", unknown)
		require.Error(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		outsider := th.CreateUser(t)
		resp, err = put(t, th.SystemAdminClient, categories+"/overrides", model.CategoryOverride{Category: "Community", TargetType: model.ChannelOverrideTargetUser, TargetId: outsider.Id, Deny: []string{model.PermissionCreatePost.Id}})
		require.Error(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("the channels of a category inherit its overrides", func(t *testing.T) {
		_, err := put(t, th.SystemAdminClient, categories+"/overrides", deny)
		require.NoError(t, err)

		require.Error(t, post(filed.Id), "denied in the category's channel")
		require.NoError(t, post(unfiled.Id), "not elsewhere")

		resp, err := th.Client.DoAPIGet(context.Background(), "/channels/"+filed.Id+"/overrides/effective", "")
		require.NoError(t, err)
		defer resp.Body.Close()
		var effective []*model.ChannelOverride
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&effective))
		require.Len(t, effective, 1)
		assert.Equal(t, filed.Id, effective[0].ChannelId)
		assert.ElementsMatch(t, deny.Deny, effective[0].Deny)
	})

	t.Run("a channel's own override wins, permission by permission", func(t *testing.T) {
		_, err := put(t, th.SystemAdminClient, "/channels/"+filed.Id+"/overrides", model.ChannelOverride{TargetType: model.ChannelOverrideTargetRole, TargetId: muted.Id, Allow: []string{model.PermissionCreatePost.Id}})
		require.NoError(t, err)
		require.NoError(t, post(filed.Id), "the channel allows it")
		require.Error(t, react(filed.Id), "the category's other deny stands")

		_, err = put(t, th.SystemAdminClient, "/channels/"+filed.Id+"/overrides", model.ChannelOverride{TargetType: model.ChannelOverrideTargetRole, TargetId: muted.Id})
		require.NoError(t, err)
		require.Error(t, post(filed.Id), "back to the category's")
	})

	t.Run("a channel moved out of the category no longer inherits", func(t *testing.T) {
		require.Nil(t, th.App.ClearChannelManagedCategory(th.Context, filed.Id))
		require.NoError(t, post(filed.Id))
		require.Nil(t, th.App.SetChannelManagedCategory(th.Context, filed.Id, "Community"))
		require.Error(t, post(filed.Id))
	})

	t.Run("overrides follow a renamed category, and go with a removed one", func(t *testing.T) {
		_, err := put(t, th.SystemAdminClient, categories, model.ServerCategoriesUpdate{
			Categories: []*model.ServerCategory{{Name: "Rules", Locked: true}, {Name: "Hangout"}},
			Renames:    map[string]string{"Community": "Hangout"},
		})
		require.NoError(t, err)
		require.Error(t, post(filed.Id), "still denied under its new name")

		overrides, appErr := th.App.GetCategoryOverrides(th.Context, team.Id)
		require.Nil(t, appErr)
		require.Len(t, overrides, 1)
		assert.Equal(t, "hangout", overrides[0].Category)

		_, err = put(t, th.SystemAdminClient, categories, model.ServerCategoriesUpdate{
			Categories: []*model.ServerCategory{{Name: "Rules", Locked: true}},
			Removed:    []string{"Hangout"},
		})
		require.NoError(t, err)
		require.NoError(t, post(filed.Id), "the category and its overrides are gone")
		overrides, appErr = th.App.GetCategoryOverrides(th.Context, team.Id)
		require.Nil(t, appErr)
		assert.Empty(t, overrides)
	})

	t.Run("team admins aren't affected", func(t *testing.T) {
		th.LinkUserToTeam(t, th.SystemAdminUser, team)
		_, err := put(t, th.SystemAdminClient, categories+"/overrides", model.CategoryOverride{Category: "Rules", TargetType: model.ChannelOverrideTargetUser, TargetId: th.SystemAdminUser.Id, Deny: []string{model.PermissionCreatePost.Id}})
		require.NoError(t, err)
		rules := th.CreatePublicChannel(t)
		require.Nil(t, th.App.SetChannelManagedCategory(th.Context, rules.Id, "Rules"))
		_, _, err = th.SystemAdminClient.CreatePost(context.Background(), &model.Post{ChannelId: rules.Id, Message: "still here"})
		require.NoError(t, err)
	})
}
