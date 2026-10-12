// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestOnTopPosts(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	hour := time.Hour.Milliseconds()

	onTopIDs := func(t *testing.T, channelID string) []string {
		t.Helper()
		onTop, appErr := th.App.GetOnTopPosts(th.Context, channelID)
		require.Nil(t, appErr)
		ids := []string{}
		for _, o := range onTop {
			ids = append(ids, o.PostId)
		}
		return ids
	}

	t.Run("messages go on top for a while or until removed, the latest first", func(t *testing.T) {
		channel := th.CreateChannel(t, th.BasicTeam)
		first, second := th.CreatePost(t, channel), th.CreatePost(t, channel)

		onTop, appErr := th.App.PutPostOnTop(th.Context, first, channel, th.BasicUser.Id, hour)
		require.Nil(t, appErr)
		assert.Equal(t, th.BasicUser.Id, onTop.CreatedBy)
		assert.InDelta(t, onTop.CreateAt+hour, onTop.ExpiresAt, 1)

		// Created a moment later, so that the order is certain.
		time.Sleep(5 * time.Millisecond)
		onTop, appErr = th.App.PutPostOnTop(th.Context, second, channel, th.BasicUser2.Id, 0)
		require.Nil(t, appErr)
		assert.Zero(t, onTop.ExpiresAt, "until removed")
		assert.Equal(t, []string{second.Id, first.Id}, onTopIDs(t, channel.Id))

		time.Sleep(5 * time.Millisecond)
		_, appErr = th.App.PutPostOnTop(th.Context, first, channel, th.BasicUser.Id, 0)
		require.Nil(t, appErr)
		assert.Equal(t, []string{first.Id, second.Id}, onTopIDs(t, channel.Id), "putting it there again brings it first")

		require.Nil(t, th.App.RemovePostFromTop(th.Context, first, channel))
		assert.Equal(t, []string{second.Id}, onTopIDs(t, channel.Id))

		appErr = th.App.RemovePostFromTop(th.Context, first, channel)
		require.NotNil(t, appErr)
		assert.Equal(t, http.StatusNotFound, appErr.StatusCode, "it's no longer there")
	})

	t.Run("expired messages leave the top, and are forgotten on the next put", func(t *testing.T) {
		channel := th.CreateChannel(t, th.BasicTeam)
		expired, fresh := th.CreatePost(t, channel), th.CreatePost(t, channel)
		_, err := th.App.Srv().Store().OnTopPost().Save(th.Context, &model.OnTopPost{PostId: expired.Id, ChannelId: channel.Id, CreateAt: model.GetMillis() - 2*hour, ExpiresAt: model.GetMillis() - hour})
		require.NoError(t, err)
		assert.Empty(t, onTopIDs(t, channel.Id))

		_, appErr := th.App.PutPostOnTop(th.Context, fresh, channel, th.BasicUser.Id, hour)
		require.Nil(t, appErr)
		assert.Equal(t, []string{fresh.Id}, onTopIDs(t, channel.Id))
		_, err = th.App.Srv().Store().OnTopPost().Get(th.Context, expired.Id)
		require.Error(t, err, "the expired one was forgotten")
	})

	t.Run("a channel keeps a bounded number of messages on top", func(t *testing.T) {
		channel := th.CreateChannel(t, th.BasicTeam)
		posts := []*model.Post{}
		for range model.OnTopPostsMaxPerChannel {
			post := th.CreatePost(t, channel)
			posts = append(posts, post)
			_, appErr := th.App.PutPostOnTop(th.Context, post, channel, th.BasicUser.Id, 0)
			require.Nil(t, appErr)
		}
		_, appErr := th.App.PutPostOnTop(th.Context, th.CreatePost(t, channel), channel, th.BasicUser.Id, 0)
		require.NotNil(t, appErr)
		assert.Equal(t, "app.on_top_post.too_many.app_error", appErr.Id)

		_, appErr = th.App.PutPostOnTop(th.Context, posts[0], channel, th.BasicUser.Id, hour)
		require.Nil(t, appErr, "one already there can get a new time")
	})

	t.Run("durations are bounded", func(t *testing.T) {
		channel := th.CreateChannel(t, th.BasicTeam)
		post := th.CreatePost(t, channel)
		for _, duration := range []int64{-1, model.OnTopPostMaxDuration.Milliseconds() + 1} {
			_, appErr := th.App.PutPostOnTop(th.Context, post, channel, th.BasicUser.Id, duration)
			require.NotNil(t, appErr)
			assert.Equal(t, "model.on_top_post.duration.app_error", appErr.Id)
		}
	})

	t.Run("system messages and archived channels are left out", func(t *testing.T) {
		channel := th.CreateChannel(t, th.BasicTeam)
		system, err := th.App.Srv().Store().Post().Save(th.Context, &model.Post{ChannelId: channel.Id, UserId: th.BasicUser.Id, Type: model.PostTypeJoinChannel, Message: "joined"})
		require.NoError(t, err)
		_, appErr := th.App.PutPostOnTop(th.Context, system, channel, th.BasicUser.Id, 0)
		require.NotNil(t, appErr)
		assert.Equal(t, "app.on_top_post.post_type.app_error", appErr.Id)

		post := th.CreatePost(t, channel)
		require.Nil(t, th.App.DeleteChannel(th.Context, channel, th.BasicUser.Id))
		channel, appErr = th.App.GetChannel(th.Context, channel.Id)
		require.Nil(t, appErr)
		_, appErr = th.App.PutPostOnTop(th.Context, post, channel, th.BasicUser.Id, 0)
		require.NotNil(t, appErr)
		assert.Equal(t, "app.on_top_post.archived_channel.app_error", appErr.Id)
	})

	t.Run("a deleted message leaves the top", func(t *testing.T) {
		channel := th.CreateChannel(t, th.BasicTeam)
		post := th.CreatePost(t, channel)
		_, appErr := th.App.PutPostOnTop(th.Context, post, channel, th.BasicUser.Id, 0)
		require.Nil(t, appErr)

		_, appErr = th.App.DeletePost(th.Context, post.Id, th.BasicUser.Id)
		require.Nil(t, appErr)
		assert.Empty(t, onTopIDs(t, channel.Id))
		require.Eventually(t, func() bool {
			_, err := th.App.Srv().Store().OnTopPost().Get(th.Context, post.Id)
			return err != nil
		}, 5*time.Second, 50*time.Millisecond, "its entry is removed too")
	})
}
