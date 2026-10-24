// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// Antimatter forums over REST: rule 1 (forums take only topics) and the server-owned props of a topic, which no
// client or bot sets by posting or editing.

func titled(channelID, rootID, title, message string) *model.Post {
	post := &model.Post{ChannelId: channelID, RootId: rootID, Message: message}
	if title != "" {
		post.AddProp(model.PostPropsThreadTitle, title)
	}
	return post
}

func TestForumTakesOnlyTopicsOverREST(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	ctx := context.Background()
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.EnableBotAccountCreation = true
		*cfg.ServiceSettings.EnableUserAccessTokens = true
	})

	forum := createChannelWithPreset(t, th.SystemAdminClient, th.BasicTeam.Id, model.ChannelPresetForumAgora)
	th.AddUserToChannel(t, th.BasicUser, forum)

	_, resp, err := th.Client.CreatePost(ctx, titled(forum.Id, "", "", "No title"))
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)
	CheckErrorID(t, err, "app.post.forum.title_required.app_error")

	topic, resp, err := th.Client.CreatePost(ctx, titled(forum.Id, "", "Beam alignment", "How do I align it?"))
	require.NoError(t, err)
	CheckCreatedStatus(t, resp)
	assert.Equal(t, "Beam alignment", topic.GetProp(model.PostPropsThreadTitle))

	reply, resp, err := th.Client.CreatePost(ctx, titled(forum.Id, topic.Id, "", "Use the laser"))
	require.NoError(t, err, "a reply needs no title")
	CheckCreatedStatus(t, resp)
	assert.Nil(t, reply.GetProp(model.PostPropsThreadTitle))

	// A bot's token is held to it too.
	bot, _, err := th.SystemAdminClient.CreateBot(ctx, &model.Bot{Username: "digest" + model.NewId()[:8], DisplayName: "Digest"})
	require.NoError(t, err)
	botUser, appErr := th.App.GetUser(th.Context, bot.UserId)
	require.Nil(t, appErr)
	th.LinkUserToTeam(t, botUser, th.BasicTeam)
	th.AddUserToChannel(t, botUser, forum)
	token, _, err := th.SystemAdminClient.CreateUserAccessToken(ctx, bot.UserId, "digest", 0)
	require.NoError(t, err)
	botClient := th.CreateClient()
	botClient.AuthToken = token.Token
	botClient.AuthType = model.HeaderBearer
	_, resp, err = botClient.CreatePost(ctx, titled(forum.Id, "", "", "Daily digest"))
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)
	_, _, err = botClient.CreatePost(ctx, titled(forum.Id, "", "Digest", "Daily digest"))
	require.NoError(t, err)

	// Edits from a client that knows nothing of titles keep the topic's.
	patched, _, err := th.Client.PatchPost(ctx, topic.Id, &model.PostPatch{Message: model.NewPointer("How do I align the beam?"), Props: &model.StringInterface{}})
	require.NoError(t, err)
	assert.Equal(t, "Beam alignment", patched.GetProp(model.PostPropsThreadTitle))
	edit := patched.Clone()
	edit.SetProps(model.StringInterface{})
	updated, _, err := th.Client.UpdatePost(ctx, topic.Id, edit)
	require.NoError(t, err)
	assert.Equal(t, "Beam alignment", updated.GetProp(model.PostPropsThreadTitle))

	// Someone else can't rename it: the edit rights are the message's.
	client2 := th.CreateClient()
	th.LoginBasic2WithClient(t, client2)
	th.AddUserToChannel(t, th.BasicUser2, forum)
	_, resp, err = client2.PatchPost(ctx, topic.Id, &model.PostPatch{Props: &model.StringInterface{model.PostPropsThreadTitle: "Hijacked"}})
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	// A thread moved in over REST takes the title given.
	thread, _, err := th.Client.CreatePost(ctx, &model.Post{ChannelId: th.BasicChannel.Id, Message: "Asked in the wrong place"})
	require.NoError(t, err)
	_, resp, err = th.Client.MoveThreadToChannel(ctx, thread.Id, &model.MoveThreadToChannelParams{ChannelId: forum.Id})
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)
	CheckErrorID(t, err, "app.post.move_thread.forum_title_required.app_error")
	moved, _, err := th.Client.MoveThreadToChannel(ctx, thread.Id, &model.MoveThreadToChannelParams{ChannelId: forum.Id, Title: "Asked here now"})
	require.NoError(t, err)
	assert.Equal(t, "Asked here now", moved.GetProp(model.PostPropsThreadTitle))
}

// RBAC regression: a forged am_accepted_answer or am_topic_status is dropped on create, update and patch over REST.
func TestForumServerOwnedPropsOverREST(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	ctx := context.Background()

	forum := createChannelWithPreset(t, th.SystemAdminClient, th.BasicTeam.Id, model.ChannelPresetForumIdeas)
	th.AddUserToChannel(t, th.BasicUser, forum)
	forged := model.StringInterface{
		model.PostPropsThreadTitle:    "My idea",
		model.PostPropsAcceptedAnswer: model.NewId(),
		model.PostPropsTopicStatus:    model.TopicStatusDone,
		model.PostPropsTopicStatusBy:  th.BasicUser.Id,
	}
	noneOf := func(t *testing.T, post *model.Post, what string) {
		t.Helper()
		for _, key := range model.PostPropsServerOwned {
			assert.Nil(t, post.GetProp(key), "%s: %s", what, key)
		}
	}

	post := &model.Post{ChannelId: forum.Id, Message: "Let's do it"}
	post.SetProps(forged)
	created, _, err := th.Client.CreatePost(ctx, post)
	require.NoError(t, err)
	noneOf(t, created, "create")

	edit := created.Clone()
	edit.SetProps(forged)
	updated, _, err := th.Client.UpdatePost(ctx, created.Id, edit)
	require.NoError(t, err)
	noneOf(t, updated, "update")

	patched, _, err := th.Client.PatchPost(ctx, created.Id, &model.PostPatch{Props: &forged})
	require.NoError(t, err)
	noneOf(t, patched, "patch")

	stored, appErr := th.App.GetSinglePost(th.Context, created.Id, false)
	require.Nil(t, appErr)
	noneOf(t, stored, "stored")
	assert.Equal(t, "My idea", stored.GetProp(model.PostPropsThreadTitle))
}
