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

// Antimatter's POST /posts/{post_id}/move_to_channel: who may move a thread, where to, and what the move keeps.

type movedThreadFixture struct {
	root    *model.Post
	replies []*model.Post
}

// makeThread writes a thread in channel: a first message by author, and replies by repliers, in the past.
func makeThread(t *testing.T, th *TestHelper, channel *model.Channel, author *model.User, repliers ...*model.User) movedThreadFixture {
	t.Helper()
	at := model.GetMillis() - 3600*1000
	root, _, appErr := th.App.CreatePost(th.Context, &model.Post{UserId: author.Id, ChannelId: channel.Id, Message: "Shall we move the beam test to Friday? @" + th.BasicUser2.Username, CreateAt: at}, channel, model.CreatePostFlags{})
	require.Nil(t, appErr)
	f := movedThreadFixture{root: root}
	for i, replier := range repliers {
		reply, _, appErr := th.App.CreatePost(th.Context, &model.Post{UserId: replier.Id, ChannelId: channel.Id, RootId: root.Id, Message: "reply " + replier.Username, CreateAt: at + int64(i+1)*60*1000}, channel, model.CreatePostFlags{})
		require.Nil(t, appErr)
		f.replies = append(f.replies, reply)
	}
	return f
}

func loggedInAs(t *testing.T, th *TestHelper, user *model.User) *model.Client4 {
	t.Helper()
	client := th.CreateClient()
	_, _, err := client.Login(context.Background(), user.Email, user.Password)
	require.NoError(t, err)
	return client
}

func moveTo(client *model.Client4, postID, channelID string, flatten bool) (*model.Post, *model.Response, error) {
	return client.MoveThreadToChannel(context.Background(), postID, &model.MoveThreadToChannelParams{ChannelId: channelID, Flatten: flatten})
}

// requireNotMoved checks that the thread is still where it was.
func requireNotMoved(t *testing.T, th *TestHelper, f movedThreadFixture) {
	t.Helper()
	root, appErr := th.App.GetSinglePost(th.Context, f.root.Id, false)
	require.Nil(t, appErr, "the thread's first message must still be there")
	assert.Equal(t, f.root.ChannelId, root.ChannelId)
}

func TestAntimatterMoveThreadToChannel(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	source := th.BasicChannel
	target := th.BasicChannel2

	t.Run("the author moves the thread: authors, times, reactions and a pointer kept", func(t *testing.T) {
		f := makeThread(t, th, source, th.BasicUser, th.BasicUser2, th.BasicUser)
		_, appErr := th.App.SaveReactionForPost(th.Context, &model.Reaction{UserId: th.BasicUser2.Id, PostId: f.replies[0].Id, EmojiName: "smile"})
		require.Nil(t, appErr)

		moved, resp, err := moveTo(th.Client, f.root.Id, target.Id, false)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		require.NotEqual(t, f.root.Id, moved.Id)
		assert.Equal(t, target.Id, moved.ChannelId)
		assert.Equal(t, f.root.UserId, moved.UserId)
		assert.Equal(t, f.root.Message, moved.Message)
		assert.Equal(t, f.root.CreateAt, moved.CreateAt)

		thread, appErr := th.App.GetPostThread(th.Context, moved.Id, model.GetPostsOptions{}, th.BasicUser.Id)
		require.Nil(t, appErr)
		var replies []*model.Post
		var note *model.Post
		for _, p := range thread.ToSlice() {
			switch {
			case p.Type == model.PostTypeWrangler:
				note = p
			case p.Id != moved.Id:
				replies = append(replies, p)
			}
		}
		require.Len(t, replies, 2)
		for _, original := range f.replies {
			var copied *model.Post
			for _, r := range replies {
				if r.Message == original.Message && r.CreateAt == original.CreateAt {
					copied = r
				}
			}
			require.NotNil(t, copied, "reply %q keeps its time", original.Message)
			assert.Equal(t, original.UserId, copied.UserId, "reply keeps its author")
			assert.Equal(t, moved.Id, copied.RootId)
			assert.Equal(t, target.Id, copied.ChannelId)
			assert.Equal(t, true, copied.GetProp(model.PostPropsSilentNotification), "a moved message notifies nobody again")
			if original.Id == f.replies[0].Id {
				reactions, reactionsErr := th.App.GetReactionsForPost(copied.Id)
				require.Nil(t, reactionsErr)
				require.Len(t, reactions, 1)
				assert.Equal(t, "smile", reactions[0].EmojiName)
			}
		}
		require.NotNil(t, note, "the target says the thread came from elsewhere")
		assert.Equal(t, "app.post.move_thread.from_another_channel", note.GetProp("TranslationID"))

		// The old thread is gone, and a pointer to the new place is left.
		_, appErr = th.App.GetSinglePost(th.Context, f.root.Id, false)
		require.NotNil(t, appErr)
		posts, appErr := th.App.GetPostsPage(th.Context, model.GetPostsOptions{ChannelId: source.Id, Page: 0, PerPage: 10})
		require.Nil(t, appErr)
		var pointer *model.Post
		for _, p := range posts.ToSlice() {
			if p.Type == model.PostTypeWrangler {
				pointer = p
			}
		}
		require.NotNil(t, pointer)
		assert.Contains(t, pointer.GetProp("MovedThreadPermalink"), "/"+th.BasicTeam.Name+"/pl/"+moved.Id)
		assert.EqualValues(t, 3, pointer.GetProp("NumMessages"))
	})

	t.Run("flatten turns the replies into the channel's messages, without the topic name", func(t *testing.T) {
		f := makeThread(t, th, source, th.BasicUser, th.BasicUser2)
		f.root.AddProp("am_thread_title", "Beam test")
		_, _, appErr := th.App.UpdatePost(th.Context, f.root, &model.UpdatePostOptions{})
		require.Nil(t, appErr)

		moved, resp, err := moveTo(th.Client, f.root.Id, target.Id, true)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		assert.Nil(t, moved.GetProp("am_thread_title"))

		posts, appErr := th.App.GetPostsPage(th.Context, model.GetPostsOptions{ChannelId: target.Id, Page: 0, PerPage: 20})
		require.Nil(t, appErr)
		found := false
		for _, p := range posts.ToSlice() {
			if p.Message == f.replies[0].Message && p.CreateAt == f.replies[0].CreateAt {
				found = true
				assert.Empty(t, p.RootId, "the reply is a message of the channel")
				assert.Equal(t, th.BasicUser2.Id, p.UserId)
			}
		}
		assert.True(t, found)
	})

	t.Run("a reply's id moves its whole thread", func(t *testing.T) {
		f := makeThread(t, th, source, th.BasicUser, th.BasicUser2)
		moved, resp, err := moveTo(th.Client, f.replies[0].Id, target.Id, false)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		assert.Equal(t, f.root.Message, moved.Message)
	})

	// Permission regressions.

	t.Run("a plain member can't move someone else's thread", func(t *testing.T) {
		f := makeThread(t, th, source, th.BasicUser, th.BasicUser2)
		client2 := loggedInAs(t, th, th.BasicUser2)
		_, resp, err := moveTo(client2, f.root.Id, target.Id, false)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		CheckErrorID(t, err, "api.post.move_thread.no_permission")
		requireNotMoved(t, th, f)

		// Not even by naming one of their own replies in it.
		_, resp, err = moveTo(client2, f.replies[0].Id, target.Id, false)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		requireNotMoved(t, th, f)
	})

	// An author without moderation rights moves others' replies only between public channels.

	t.Run("an author can't take others' replies out of a private channel", func(t *testing.T) {
		f := makeThread(t, th, th.BasicPrivateChannel, th.BasicUser, th.BasicUser2)
		_, resp, err := moveTo(th.Client, f.root.Id, target.Id, false)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		CheckErrorID(t, err, "api.post.move_thread.private_needs_moderator.app_error")
		requireNotMoved(t, th, f)
	})

	t.Run("an author can't hide others' replies in a private channel", func(t *testing.T) {
		mine := th.CreatePrivateChannel(t)
		f := makeThread(t, th, source, th.BasicUser, th.BasicUser2)
		_, resp, err := moveTo(th.Client, f.root.Id, mine.Id, false)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		CheckErrorID(t, err, "api.post.move_thread.private_needs_moderator.app_error")
		requireNotMoved(t, th, f)
	})

	t.Run("an author moves a thread of their own messages out of a private channel", func(t *testing.T) {
		f := makeThread(t, th, th.BasicPrivateChannel, th.BasicUser, th.BasicUser)
		moved, resp, err := moveTo(th.Client, f.root.Id, target.Id, false)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		assert.Equal(t, target.Id, moved.ChannelId)
	})

	t.Run("an author moves others' replies between public channels", func(t *testing.T) {
		f := makeThread(t, th, source, th.BasicUser, th.BasicUser2)
		_, resp, err := moveTo(th.Client, f.root.Id, target.Id, false)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
	})

	t.Run("a moderator moves others' replies from a private channel to a public one", func(t *testing.T) {
		moderator := th.CreateUser(t)
		th.LinkUserToTeam(t, moderator, th.BasicTeam)
		th.AddUserToChannel(t, moderator, th.BasicPrivateChannel)
		th.AddUserToChannel(t, moderator, target)
		th.MakeUserChannelAdmin(t, moderator, th.BasicPrivateChannel)
		f := makeThread(t, th, th.BasicPrivateChannel, th.BasicUser, th.BasicUser2)

		_, resp, err := moveTo(loggedInAs(t, th, moderator), f.root.Id, target.Id, false)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
	})

	t.Run("the channel's admins may", func(t *testing.T) {
		admin := th.CreateUser(t)
		th.LinkUserToTeam(t, admin, th.BasicTeam)
		th.AddUserToChannel(t, admin, source)
		th.AddUserToChannel(t, admin, target)
		th.MakeUserChannelAdmin(t, admin, source)
		f := makeThread(t, th, source, th.BasicUser, th.BasicUser2)

		_, resp, err := moveTo(loggedInAs(t, th, admin), f.root.Id, target.Id, false)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
	})

	t.Run("an admin of the target only may not", func(t *testing.T) {
		admin := th.CreateUser(t)
		th.LinkUserToTeam(t, admin, th.BasicTeam)
		th.AddUserToChannel(t, admin, source)
		th.AddUserToChannel(t, admin, target)
		th.MakeUserChannelAdmin(t, admin, target)
		f := makeThread(t, th, source, th.BasicUser, th.BasicUser2)

		_, resp, err := moveTo(loggedInAs(t, th, admin), f.root.Id, target.Id, false)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		requireNotMoved(t, th, f)
	})

	t.Run("the community's admins may", func(t *testing.T) {
		admin := th.CreateUser(t)
		th.LinkUserToTeam(t, admin, th.BasicTeam)
		th.UpdateUserToTeamAdmin(t, admin, th.BasicTeam)
		th.AddUserToChannel(t, admin, target)
		f := makeThread(t, th, source, th.BasicUser, th.BasicUser2)

		_, resp, err := moveTo(loggedInAs(t, th, admin), f.root.Id, target.Id, false)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
	})

	t.Run("system admins may", func(t *testing.T) {
		f := makeThread(t, th, source, th.BasicUser, th.BasicUser2)
		_, resp, err := moveTo(th.SystemAdminClient, f.root.Id, target.Id, false)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
	})

	t.Run("no moves to another community", func(t *testing.T) {
		otherTeam := th.CreateTeam(t)
		th.LinkUserToTeam(t, th.BasicUser, otherTeam)
		elsewhere := th.CreateChannelWithClientAndTeam(t, th.SystemAdminClient, model.ChannelTypeOpen, otherTeam.Id)
		th.AddUserToChannel(t, th.BasicUser, elsewhere)
		f := makeThread(t, th, source, th.BasicUser, th.BasicUser2)

		_, resp, err := moveTo(th.Client, f.root.Id, elsewhere.Id, false)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		CheckErrorID(t, err, "api.post.move_thread.same_community.app_error")
		requireNotMoved(t, th, f)

		// Not even for system admins.
		_, resp, err = moveTo(th.SystemAdminClient, f.root.Id, elsewhere.Id, false)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		requireNotMoved(t, th, f)
	})

	t.Run("no moves to a direct message", func(t *testing.T) {
		dm := th.CreateDmChannel(t, th.BasicUser2)
		f := makeThread(t, th, source, th.BasicUser, th.BasicUser2)
		_, resp, err := moveTo(th.Client, f.root.Id, dm.Id, false)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		requireNotMoved(t, th, f)
	})

	t.Run("archived channels are refused, as target and as source", func(t *testing.T) {
		f := makeThread(t, th, source, th.BasicUser, th.BasicUser2)
		_, resp, err := moveTo(th.Client, f.root.Id, th.BasicDeletedChannel.Id, false)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		CheckErrorID(t, err, "api.post.move_thread.archived.app_error")
		requireNotMoved(t, th, f)

		archived := th.CreatePublicChannel(t)
		th.AddUserToChannel(t, th.BasicUser, archived)
		g := makeThread(t, th, archived, th.BasicUser, th.BasicUser2)
		_, err = th.SystemAdminClient.DeleteChannel(context.Background(), archived.Id)
		require.NoError(t, err)
		_, resp, err = moveTo(th.SystemAdminClient, g.root.Id, target.Id, false)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		CheckErrorID(t, err, "api.post.move_thread.archived.app_error")
	})

	t.Run("the mover must be able to post in the target", func(t *testing.T) {
		notMine := th.CreateChannelWithClient(t, th.SystemAdminClient, model.ChannelTypeOpen)
		f := makeThread(t, th, source, th.BasicUser, th.BasicUser2)
		_, resp, err := moveTo(th.Client, f.root.Id, notMine.Id, false)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		CheckErrorID(t, err, "api.post.move_thread.cannot_post.app_error")
		requireNotMoved(t, th, f)
	})

	t.Run("a thread one can't read can't be moved", func(t *testing.T) {
		secret := th.CreateChannelWithClient(t, th.SystemAdminClient, model.ChannelTypePrivate)
		outsider := th.SystemAdminUser
		f := makeThread(t, th, secret, outsider)
		_, resp, err := moveTo(th.Client, f.root.Id, target.Id, false)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		requireNotMoved(t, th, f)
	})

	t.Run("not into the same channel, nor without a channel", func(t *testing.T) {
		f := makeThread(t, th, source, th.BasicUser, th.BasicUser2)
		_, resp, err := moveTo(th.Client, f.root.Id, source.Id, false)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		_, resp, err = moveTo(th.Client, f.root.Id, "", false)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		requireNotMoved(t, th, f)
	})
}

// "Manage messages" (deleting others' messages) lets a moderator move threads. Not parallel: it changes a built-in
// role.
func TestAntimatterMoveThreadToChannelWithManageMessages(t *testing.T) {
	th := Setup(t).InitBasic(t)
	defaultPermissions := th.SaveDefaultRolePermissions(t)
	defer th.RestoreDefaultRolePermissions(t, defaultPermissions)

	f := makeThread(t, th, th.BasicChannel, th.BasicUser, th.BasicUser2)
	client2 := loggedInAs(t, th, th.BasicUser2)

	_, resp, err := moveTo(client2, f.root.Id, th.BasicChannel2.Id, false)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	th.AddPermissionToRole(t, model.PermissionDeleteOthersPosts.Id, model.ChannelUserRoleId)
	_, resp, err = moveTo(client2, f.root.Id, th.BasicChannel2.Id, false)
	require.NoError(t, err)
	CheckOKStatus(t, resp)
}
