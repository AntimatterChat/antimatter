// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func createInlineReply(t *testing.T, th *TestHelper, channel *model.Channel, rootID string, replyTo any, setters ...func(*model.Post)) *model.Post {
	t.Helper()
	post := &model.Post{
		UserId:    th.BasicUser.Id,
		ChannelId: channel.Id,
		RootId:    rootID,
		Message:   "reply_" + model.NewId(),
	}
	if replyTo != nil {
		post.AddProp(model.PostPropsReplyTo, replyTo)
	}
	for _, set := range setters {
		set(post)
	}
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

	withMention := func(value any) func(*model.Post) {
		return func(post *model.Post) { post.AddProp(model.PostPropsReplyToMention, value) }
	}

	t.Run("keeps a boolean reply_to_mention with the reference", func(t *testing.T) {
		for _, value := range []bool{false, true} {
			reply := createInlineReply(t, th, th.BasicChannel, "", target.Id, withMention(value))
			assert.Equal(t, value, reply.GetProp(model.PostPropsReplyToMention))
		}
	})

	t.Run("drops a reply_to_mention that isn't a boolean", func(t *testing.T) {
		for _, value := range []any{"false", 0, nil} {
			reply := createInlineReply(t, th, th.BasicChannel, "", target.Id, withMention(value))
			assert.Equal(t, target.Id, reply.GetReplyToProp())
			assert.NotContains(t, reply.GetProps(), model.PostPropsReplyToMention, "%v", value)
		}
	})

	t.Run("drops reply_to_mention without a kept reference", func(t *testing.T) {
		reply := createInlineReply(t, th, th.BasicChannel, "", nil, withMention(false))
		assert.NotContains(t, reply.GetProps(), model.PostPropsReplyToMention)

		reply = createInlineReply(t, th, th.BasicChannel, "", model.NewId(), withMention(false))
		assert.NotContains(t, reply.GetProps(), model.PostPropsReplyToMention)
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

	t.Run("drops reply_to_mention along with the reference", func(t *testing.T) {
		reply := createInlineReply(t, th, th.BasicChannel, "", target.Id, func(post *model.Post) {
			post.AddProp(model.PostPropsReplyToMention, false)
		})

		updated := edit(t, reply, target.Id)
		assert.Equal(t, false, updated.GetProp(model.PostPropsReplyToMention))

		updated = edit(t, updated, nil)
		assert.NotContains(t, updated.GetProps(), model.PostPropsReplyToMention)
	})

	t.Run("doesn't let a plain post become a reply to an invalid message", func(t *testing.T) {
		post := th.CreatePost(t, th.BasicChannel)

		updated := edit(t, post, model.NewId())
		assert.NotContains(t, updated.GetProps(), model.PostPropsReplyTo)
	})
}

func TestInlineReplyMetadata(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	// Posts as read from the database, without metadata.
	fromStore := func(t *testing.T, id string) *model.Post {
		t.Helper()
		post, err := th.App.Srv().Store().Post().GetSingle(th.Context, id, true)
		require.NoError(t, err)
		return post
	}

	target := th.CreatePost(t, th.BasicChannel)
	created := createInlineReply(t, th, th.BasicChannel, "", target.Id)
	require.NotNil(t, created.Metadata.ReplyTo, "creating a reply returns it with the quoted message")
	reply := fromStore(t, created.Id)

	t.Run("describes the quoted message", func(t *testing.T) {
		prepared := th.App.PreparePostForClient(th.Context, reply, &model.PreparePostForClientOpts{})
		require.NotNil(t, prepared.Metadata.ReplyTo)
		assert.Equal(t, target.Id, prepared.Metadata.ReplyTo.PostId)
		assert.Equal(t, target.UserId, prepared.Metadata.ReplyTo.UserId)
		assert.Equal(t, target.Message, prepared.Metadata.ReplyTo.Message)
		assert.False(t, prepared.Metadata.ReplyTo.Deleted)
	})

	t.Run("describes the quoted message in post lists", func(t *testing.T) {
		other := th.CreatePost(t, th.BasicChannel)
		otherReply := fromStore(t, createInlineReply(t, th, th.BasicChannel, "", other.Id).Id)
		plain := th.CreatePost(t, th.BasicChannel)

		// One reply's message is in the list, the other's isn't.
		list := model.NewPostList()
		for _, post := range []*model.Post{target, reply, otherReply, plain} {
			list.AddPost(post)
			list.AddOrder(post.Id)
		}
		prepared := th.App.PreparePostListForClient(th.Context, list, nil)
		require.NotNil(t, prepared.Posts[reply.Id].Metadata.ReplyTo)
		assert.Equal(t, target.Message, prepared.Posts[reply.Id].Metadata.ReplyTo.Message)
		require.NotNil(t, prepared.Posts[otherReply.Id].Metadata.ReplyTo)
		assert.Equal(t, other.Message, prepared.Posts[otherReply.Id].Metadata.ReplyTo.Message)
		assert.Nil(t, prepared.Posts[plain.Id].Metadata.ReplyTo)
		assert.Nil(t, prepared.Posts[target.Id].Metadata.ReplyTo)

		// With several messages to read.
		third := th.CreatePost(t, th.BasicChannel)
		thirdReply := fromStore(t, createInlineReply(t, th, th.BasicChannel, "", third.Id).Id)
		list = model.NewPostList()
		for _, post := range []*model.Post{reply, otherReply, thirdReply} {
			list.AddPost(post)
			list.AddOrder(post.Id)
		}
		prepared = th.App.PreparePostListForClient(th.Context, list, nil)
		assert.Equal(t, target.Message, prepared.Posts[reply.Id].Metadata.ReplyTo.Message)
		assert.Equal(t, other.Message, prepared.Posts[otherReply.Id].Metadata.ReplyTo.Message)
		assert.Equal(t, third.Message, prepared.Posts[thirdReply.Id].Metadata.ReplyTo.Message)
	})

	t.Run("counts the images among the quoted message's files", func(t *testing.T) {
		var fileIDs []string
		for _, file := range []struct{ name, mime string }{{"photo.png", "image/png"}, {"notes.pdf", "application/pdf"}} {
			info, err := th.App.Srv().Store().FileInfo().Save(th.Context, &model.FileInfo{
				CreatorId: th.BasicUser.Id,
				Name:      file.name,
				Path:      "/data/" + file.name,
				MimeType:  file.mime,
			})
			require.NoError(t, err)
			fileIDs = append(fileIDs, info.Id)
		}
		withFiles, _, appErr := th.App.CreatePost(th.Context, &model.Post{UserId: th.BasicUser.Id, ChannelId: th.BasicChannel.Id, FileIds: fileIDs}, th.BasicChannel, model.CreatePostFlags{})
		require.Nil(t, appErr)
		filesReply := fromStore(t, createInlineReply(t, th, th.BasicChannel, "", withFiles.Id).Id)

		prepared := th.App.PreparePostForClient(th.Context, filesReply, &model.PreparePostForClientOpts{})
		require.NotNil(t, prepared.Metadata.ReplyTo)
		assert.Equal(t, 2, prepared.Metadata.ReplyTo.FileCount)
		assert.Equal(t, 1, prepared.Metadata.ReplyTo.ImageCount)

		// A message without files has no images.
		prepared = th.App.PreparePostForClient(th.Context, reply, &model.PreparePostForClientOpts{})
		assert.Zero(t, prepared.Metadata.ReplyTo.ImageCount)
	})

	t.Run("follows edits of the quoted message", func(t *testing.T) {
		edited := target.Clone()
		edited.Message = "edited " + model.NewId()
		_, _, appErr := th.App.UpdatePost(th.Context, edited, nil)
		require.Nil(t, appErr)

		prepared := th.App.PreparePostForClient(th.Context, reply, &model.PreparePostForClientOpts{})
		assert.Equal(t, edited.Message, prepared.Metadata.ReplyTo.Message)
		assert.NotZero(t, prepared.Metadata.ReplyTo.EditAt)
	})

	t.Run("marks a deleted quoted message", func(t *testing.T) {
		quoted := th.CreatePost(t, th.BasicChannel)
		quotingReply := createInlineReply(t, th, th.BasicChannel, "", quoted.Id)
		_, appErr := th.App.DeletePost(th.Context, quoted.Id, th.BasicUser.Id)
		require.Nil(t, appErr)

		prepared := th.App.PreparePostForClient(th.Context, fromStore(t, quotingReply.Id), &model.PreparePostForClientOpts{})
		assert.Equal(t, &model.PostReplyTo{PostId: quoted.Id, Deleted: true}, prepared.Metadata.ReplyTo)
	})

	t.Run("leaves deleted replies empty", func(t *testing.T) {
		deletedReply := createInlineReply(t, th, th.BasicChannel, "", target.Id)
		_, appErr := th.App.DeletePost(th.Context, deletedReply.Id, th.BasicUser.Id)
		require.Nil(t, appErr)

		prepared := th.App.PreparePostForClient(th.Context, fromStore(t, deletedReply.Id), &model.PreparePostForClientOpts{})
		assert.Nil(t, prepared.Metadata.ReplyTo)
	})

	t.Run("describes nothing when inline replies are off", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableInlineReplies = false })
		defer th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableInlineReplies = true })

		prepared := th.App.PreparePostForClient(th.Context, reply, &model.PreparePostForClientOpts{})
		assert.Nil(t, prepared.Metadata.ReplyTo)
	})
}

func TestInlineReplyNotifiesQuotedAuthor(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	th.AddUserToChannel(t, th.BasicUser2, th.BasicChannel)

	theirs := th.CreatePost(t, th.BasicChannel, func(post *model.Post) { post.UserId = th.BasicUser2.Id })
	mine := th.CreatePost(t, th.BasicChannel)

	notified := func(t *testing.T, reply *model.Post) []string {
		t.Helper()
		mentions, err := th.App.SendNotifications(th.Context, reply, th.BasicTeam, th.BasicChannel, th.BasicUser, nil, true)
		require.NoError(t, err)
		return mentions
	}

	t.Run("notifies the author of the quoted message", func(t *testing.T) {
		reply := createInlineReply(t, th, th.BasicChannel, "", theirs.Id)
		assert.Contains(t, notified(t, reply), th.BasicUser2.Id)
	})

	t.Run("doesn't notify the author when the reply says not to", func(t *testing.T) {
		reply := createInlineReply(t, th, th.BasicChannel, "", theirs.Id, func(post *model.Post) {
			post.AddProp(model.PostPropsReplyToMention, false)
		})
		assert.NotContains(t, notified(t, reply), th.BasicUser2.Id)
	})

	t.Run("still notifies people mentioned in a reply that doesn't notify the author", func(t *testing.T) {
		reply := createInlineReply(t, th, th.BasicChannel, "", theirs.Id, func(post *model.Post) {
			post.Message = "@" + th.BasicUser2.Username + " look"
			post.AddProp(model.PostPropsReplyToMention, false)
		})
		assert.Contains(t, notified(t, reply), th.BasicUser2.Id)
	})

	t.Run("doesn't notify people replying to themselves", func(t *testing.T) {
		reply := createInlineReply(t, th, th.BasicChannel, "", mine.Id)
		assert.Empty(t, notified(t, reply))
	})

	t.Run("doesn't notify people who left the channel", func(t *testing.T) {
		other := th.CreateChannel(t, th.BasicTeam)
		th.AddUserToChannel(t, th.BasicUser2, other)
		target := th.CreatePost(t, other, func(post *model.Post) { post.UserId = th.BasicUser2.Id })
		reply := createInlineReply(t, th, other, "", target.Id)
		require.Nil(t, th.App.RemoveUserFromChannel(th.Context, th.BasicUser2.Id, th.SystemAdminUser.Id, other))

		mentions, err := th.App.SendNotifications(th.Context, reply, th.BasicTeam, other, th.BasicUser, nil, true)
		require.NoError(t, err)
		assert.NotContains(t, mentions, th.BasicUser2.Id)
	})

	t.Run("doesn't notify anyone when inline replies are off", func(t *testing.T) {
		reply := createInlineReply(t, th, th.BasicChannel, "", theirs.Id)
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableInlineReplies = false })
		defer th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableInlineReplies = true })

		assert.NotContains(t, notified(t, reply), th.BasicUser2.Id)
	})
}
