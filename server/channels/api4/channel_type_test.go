// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// Antimatter channel types through the API: creating with a preset, reading them back, PUT /channels/{id}/type and
// who may use it, rule 2 over REST, and the generic properties API kept away from them.

func createChannelWithPreset(t *testing.T, client *model.Client4, teamID, preset string) *model.Channel {
	t.Helper()
	channel, resp, err := client.CreateChannel(context.Background(), &model.Channel{
		DisplayName: "Typed", Name: "typed-" + model.NewId(), Type: model.ChannelTypeOpen, TeamId: teamID,
		Props: map[string]any{model.ChannelPropPreset: preset},
	})
	require.NoError(t, err)
	CheckCreatedStatus(t, resp)
	return channel
}

func TestCreateChannelWithPresetAPI(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	channel := createChannelWithPreset(t, th.Client, th.BasicTeam.Id, model.ChannelPresetForumQA)
	assert.Equal(t, model.AMChannelTypeForum, channel.Props[model.ChannelPropType])
	assert.Equal(t, model.ChannelPresetForumQA, channel.Props[model.ChannelPropPreset])

	got, _, err := th.Client.GetChannel(context.Background(), channel.Id)
	require.NoError(t, err)
	assert.Equal(t, model.AMChannelModeQA, got.Props[model.ChannelPropMode])
	assert.Equal(t, model.AMForumViewTopics, got.Props[model.ChannelPropView])

	mine, _, err := th.Client.GetChannelsForTeamForUser(context.Background(), th.BasicTeam.Id, th.BasicUser.Id, false, "")
	require.NoError(t, err)
	for _, c := range mine {
		assert.Contains(t, c.Props, model.ChannelPropType, "every team channel carries its type, text ones included: %s", c.Name)
	}

	found, _, err := th.Client.SearchChannels(context.Background(), th.BasicTeam.Id, &model.ChannelSearch{Term: channel.Name})
	require.NoError(t, err)
	require.NotEmpty(t, found)
	assert.Equal(t, model.ChannelPresetForumQA, found[0].Props[model.ChannelPropPreset], "search results too")

	_, resp, err := th.Client.CreateChannel(context.Background(), &model.Channel{
		DisplayName: "Bad", Name: "bad-" + model.NewId(), Type: model.ChannelTypeOpen, TeamId: th.BasicTeam.Id,
		Props: map[string]any{model.ChannelPropPreset: "forum_wiki"},
	})
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)
}

// Not in parallel: it takes a permission from the channel members' role for a while.
func TestUpdateChannelTypeAPI(t *testing.T) {
	th := Setup(t).InitBasic(t)
	ctx := context.Background()

	channel := createChannelWithPreset(t, th.SystemAdminClient, th.BasicTeam.Id, model.ChannelPresetForumAgora)
	th.AddUserToChannel(t, th.BasicUser, channel)

	t.Run("a member without the right to manage the channel's properties gets 403", func(t *testing.T) {
		th.RemovePermissionFromRole(t, model.PermissionManagePublicChannelProperties.Id, model.ChannelUserRoleId)
		defer th.AddPermissionToRole(t, model.PermissionManagePublicChannelProperties.Id, model.ChannelUserRoleId)
		_, resp, err := th.Client.UpdateChannelType(ctx, channel.Id, &model.ChannelTypeRequest{Preset: model.ChannelPresetForumAnnouncement})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		assert.Equal(t, model.ChannelPresetForumAgora, th.App.GetChannelTypeInfo(channel.Id).Preset(), "unchanged")
	})

	t.Run("an override denying it gets 403 too", func(t *testing.T) {
		_, appErr := th.App.SaveChannelOverride(th.Context, channel, &model.ChannelOverride{
			TargetType: model.ChannelOverrideTargetUser, TargetId: th.BasicUser.Id, Deny: []string{model.PermissionManagePublicChannelProperties.Id},
		})
		require.Nil(t, appErr)
		defer func() {
			_, appErr = th.App.SaveChannelOverride(th.Context, channel, &model.ChannelOverride{TargetType: model.ChannelOverrideTargetUser, TargetId: th.BasicUser.Id})
			require.Nil(t, appErr)
		}()
		_, resp, err := th.Client.UpdateChannelType(ctx, channel.Id, &model.ChannelTypeRequest{Preset: model.ChannelPresetForumAnnouncement})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("a channel admin picks another preset of the type", func(t *testing.T) {
		th.MakeUserChannelAdmin(t, th.BasicUser, channel)
		updated, resp, err := th.Client.UpdateChannelType(ctx, channel.Id, &model.ChannelTypeRequest{Preset: model.ChannelPresetForumAnnouncement})
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		assert.Equal(t, model.ChannelPresetForumAnnouncement, updated.Props[model.ChannelPropPreset])
		assert.Equal(t, model.AMRootPostsModerators, updated.Props[model.ChannelPropRootPosts])
	})

	t.Run("the type is fixed", func(t *testing.T) {
		_, resp, err := th.SystemAdminClient.UpdateChannelType(ctx, channel.Id, &model.ChannelTypeRequest{Preset: model.ChannelPresetVoiceTalk})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		_, resp, err = th.SystemAdminClient.UpdateChannelType(ctx, channel.Id, &model.ChannelTypeRequest{Type: model.AMChannelTypeText})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		assert.Equal(t, model.AMChannelTypeForum, th.App.GetChannelTypeInfo(channel.Id).Type)
	})

	t.Run("an archived channel is left as it is", func(t *testing.T) {
		archived := createChannelWithPreset(t, th.SystemAdminClient, th.BasicTeam.Id, model.ChannelPresetTextChat)
		_, err := th.SystemAdminClient.DeleteChannel(ctx, archived.Id)
		require.NoError(t, err)
		_, resp, err := th.SystemAdminClient.UpdateChannelType(ctx, archived.Id, &model.ChannelTypeRequest{Preset: model.ChannelPresetTextAnnouncement})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})
}

func TestRootPostsOverREST(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	ctx := context.Background()
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.EnableBotAccountCreation = true
		*cfg.ServiceSettings.EnableUserAccessTokens = true
	})

	channel := createChannelWithPreset(t, th.SystemAdminClient, th.BasicTeam.Id, model.ChannelPresetTextAnnouncement)
	th.AddUserToChannel(t, th.BasicUser, channel)
	root, _, err := th.SystemAdminClient.CreatePost(ctx, &model.Post{ChannelId: channel.Id, Message: "News"})
	require.NoError(t, err)

	_, resp, err := th.Client.CreatePost(ctx, &model.Post{ChannelId: channel.Id, Message: "Me too"})
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)
	CheckErrorID(t, err, "app.post.root_posts.moderators_only.app_error")

	_, resp, err = th.Client.CreatePost(ctx, &model.Post{ChannelId: channel.Id, RootId: root.Id, Message: "A reply"})
	require.NoError(t, err)
	CheckCreatedStatus(t, resp)

	// A bot's token is judged as the bot.
	bot, _, err := th.SystemAdminClient.CreateBot(ctx, &model.Bot{Username: "news" + model.NewId()[:8], DisplayName: "News"})
	require.NoError(t, err)
	botUser, appErr := th.App.GetUser(th.Context, bot.UserId)
	require.Nil(t, appErr)
	th.LinkUserToTeam(t, botUser, th.BasicTeam)
	th.AddUserToChannel(t, botUser, channel)
	token, _, err := th.SystemAdminClient.CreateUserAccessToken(ctx, bot.UserId, "news", 0)
	require.NoError(t, err)
	botClient := th.CreateClient()
	botClient.AuthToken = token.Token
	botClient.AuthType = model.HeaderBearer
	_, resp, err = botClient.CreatePost(ctx, &model.Post{ChannelId: channel.Id, Message: "Breaking"})
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)
}

func TestChannelTypesNotThroughPropertiesAPI(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	ctx := context.Background()

	channel := th.CreatePublicChannel(t)
	th.MakeUserChannelAdmin(t, th.BasicUser, channel)
	body := []map[string]any{{"field_id": model.NewId(), "value": model.AMChannelTypeForum}}
	for _, client := range []*model.Client4{th.Client, th.SystemAdminClient} {
		resp, err := client.DoAPIPatchJSON(ctx, "/properties/groups/"+model.ChannelTypePropertyGroupName+"/channel/values/"+channel.Id, body)
		require.Error(t, err, "a channel admin, or even a system admin, can't write a channel's type there")
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)

		resp, err = client.DoAPIGet(ctx, "/properties/groups/"+model.ChannelTypePropertyGroupName+"/channel/fields", "")
		require.Error(t, err, "nor see its fields")
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	}
	assert.Equal(t, model.DefaultChannelTypeInfo(), th.App.GetChannelTypeInfo(channel.Id))
}
