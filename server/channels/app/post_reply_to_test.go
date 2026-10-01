// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func createInlineReply(t *testing.T, th *TestHelper, channel *model.Channel, rootID string, replyTo any) *model.Post {
	t.Helper()
	post := &model.Post{
		UserId:    th.BasicUser.Id,
		ChannelId: channel.Id,
		RootId:    rootID,
		Message:   "reply_" + model.NewId(),
	}
	post.AddProp(model.PostPropsReplyTo, replyTo)
	created, _, appErr := th.App.CreatePost(th.Context, post, channel, model.CreatePostFlags{})
	require.Nil(t, appErr)
	return created
}

func TestInlineReplyCreate(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	target := th.CreatePost(t, th.BasicChannel)

	t.Run("keeps a reply to a message of the channel", func(t *testing.T) {
		reply := createInlineReply(t, th, th.BasicChannel, "", target.Id)
		assert.Equal(t, target.Id, reply.GetReplyToProp())
	})

	t.Run("keeps a reply to a thread reply of the channel", func(t *testing.T) {
		threadReply := th.CreatePostReply(t, target)
		reply := createInlineReply(t, th, th.BasicChannel, "", threadReply.Id)
		assert.Equal(t, threadReply.Id, reply.GetReplyToProp())
	})

	t.Run("keeps a thread reply to a message of its thread", func(t *testing.T) {
		threadReply := th.CreatePostReply(t, target)
		reply := createInlineReply(t, th, th.BasicChannel, target.Id, threadReply.Id)
		assert.Equal(t, threadReply.Id, reply.GetReplyToProp())

		reply = createInlineReply(t, th, th.BasicChannel, target.Id, target.Id)
		assert.Equal(t, target.Id, reply.GetReplyToProp())
	})

	t.Run("drops a thread reply's reference outside its thread", func(t *testing.T) {
		other := th.CreatePost(t, th.BasicChannel)
		reply := createInlineReply(t, th, th.BasicChannel, target.Id, other.Id)
		assert.NotContains(t, reply.GetProps(), model.PostPropsReplyTo)
	})

	t.Run("drops a reference to another channel's message", func(t *testing.T) {
		elsewhere := th.CreatePost(t, th.CreateChannel(t, th.BasicTeam))
		reply := createInlineReply(t, th, th.BasicChannel, "", elsewhere.Id)
		assert.NotContains(t, reply.GetProps(), model.PostPropsReplyTo)
	})

	t.Run("drops a reference to a deleted message", func(t *testing.T) {
		deleted := th.CreatePost(t, th.BasicChannel)
		_, appErr := th.App.DeletePost(th.Context, deleted.Id, th.BasicUser.Id)
		require.Nil(t, appErr)
		reply := createInlineReply(t, th, th.BasicChannel, "", deleted.Id)
		assert.NotContains(t, reply.GetProps(), model.PostPropsReplyTo)
	})

	t.Run("drops a reference to a system message", func(t *testing.T) {
		system, err := th.App.Srv().Store().Post().Save(th.Context, &model.Post{
			ChannelId: th.BasicChannel.Id,
			UserId:    th.BasicUser.Id,
			Type:      model.PostTypeJoinChannel,
			Message:   "joined",
		})
		require.NoError(t, err)
		reply := createInlineReply(t, th, th.BasicChannel, "", system.Id)
		assert.NotContains(t, reply.GetProps(), model.PostPropsReplyTo)
	})

	t.Run("drops unknown and malformed references", func(t *testing.T) {
		for _, value := range []any{model.NewId(), "not-an-id", 42, map[string]any{"id": target.Id}} {
			reply := createInlineReply(t, th, th.BasicChannel, "", value)
			assert.NotContains(t, reply.GetProps(), model.PostPropsReplyTo, "%v", value)
		}
	})

	t.Run("drops references when inline replies are off", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableInlineReplies = false })
		defer th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableInlineReplies = true })

		reply := createInlineReply(t, th, th.BasicChannel, "", target.Id)
		assert.NotContains(t, reply.GetProps(), model.PostPropsReplyTo)
	})
}

func TestInlineReplyUpdate(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	target := th.CreatePost(t, th.BasicChannel)

	edit := func(t *testing.T, post *model.Post, replyTo any) *model.Post {
		t.Helper()
		received := post.Clone()
		received.Message = "edited_" + model.NewId()
		if replyTo == nil {
			received.DelProp(model.PostPropsReplyTo)
		} else {
			received.AddProp(model.PostPropsReplyTo, replyTo)
		}
		updated, _, appErr := th.App.UpdatePost(th.Context, received, nil)
		require.Nil(t, appErr)
		return updated
	}

	t.Run("keeps the reference after the quoted message is deleted", func(t *testing.T) {
		quoted := th.CreatePost(t, th.BasicChannel)
		reply := createInlineReply(t, th, th.BasicChannel, "", quoted.Id)
		_, appErr := th.App.DeletePost(th.Context, quoted.Id, th.BasicUser.Id)
		require.Nil(t, appErr)

		updated := edit(t, reply, quoted.Id)
		assert.Equal(t, quoted.Id, updated.GetReplyToProp())
	})

	t.Run("puts back the reference when it is changed to an invalid one", func(t *testing.T) {
		reply := createInlineReply(t, th, th.BasicChannel, "", target.Id)
		elsewhere := th.CreatePost(t, th.CreateChannel(t, th.BasicTeam))

		updated := edit(t, reply, elsewhere.Id)
		assert.Equal(t, target.Id, updated.GetReplyToProp())
	})

	t.Run("lets the reference change to another valid message", func(t *testing.T) {
		reply := createInlineReply(t, th, th.BasicChannel, "", target.Id)
		other := th.CreatePost(t, th.BasicChannel)

		updated := edit(t, reply, other.Id)
		assert.Equal(t, other.Id, updated.GetReplyToProp())
	})

	t.Run("lets the reference be removed", func(t *testing.T) {
		reply := createInlineReply(t, th, th.BasicChannel, "", target.Id)

		updated := edit(t, reply, nil)
		assert.NotContains(t, updated.GetProps(), model.PostPropsReplyTo)
	})

	t.Run("doesn't let a plain post become a reply to an invalid message", func(t *testing.T) {
		post := th.CreatePost(t, th.BasicChannel)

		updated := edit(t, post, model.NewId())
		assert.NotContains(t, updated.GetProps(), model.PostPropsReplyTo)
	})
}
