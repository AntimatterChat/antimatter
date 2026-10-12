// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestOnTopPosts(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	ctx := context.Background()
	hour := time.Hour.Milliseconds()

	waitFor := func(t *testing.T, wsClient *model.WebSocketClient, eventType model.WebsocketEventType) *model.WebSocketEvent {
		t.Helper()
		timeout := time.After(5 * time.Second)
		for {
			select {
			case event := <-wsClient.EventChannel:
				if event.EventType() == eventType {
					return event
				}
			case <-timeout:
				require.FailNow(t, "no websocket event", string(eventType))
				return nil
			}
		}
	}

	t.Run("put on top, listed, and removed, with websocket events", func(t *testing.T) {
		wsClient := th.CreateConnectedWebSocketClient(t)
		post := th.CreatePost(t)

		onTop, resp, err := th.Client.PutPostOnTop(ctx, post.Id, hour)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		assert.Equal(t, post.Id, onTop.PostId)
		assert.Equal(t, th.BasicChannel.Id, onTop.ChannelId)
		assert.Equal(t, th.BasicUser.Id, onTop.CreatedBy)
		assert.InDelta(t, onTop.CreateAt+hour, onTop.ExpiresAt, 1)

		event := waitFor(t, wsClient, model.WebsocketEventPostPutOnTop)
		assert.Equal(t, th.BasicChannel.Id, event.GetBroadcast().ChannelId)
		data, ok := event.GetData()["on_top"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, post.Id, data["post_id"])

		list, _, err := th.Client.GetOnTopPosts(ctx, th.BasicChannel.Id)
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Equal(t, post.Id, list[0].PostId)

		// Others who may read the channel see it, and may take it off.
		other := th.CreateClient()
		_, _, err = other.Login(ctx, th.BasicUser2.Email, th.BasicUser2.Password)
		require.NoError(t, err)
		list, _, err = other.GetOnTopPosts(ctx, th.BasicChannel.Id)
		require.NoError(t, err)
		require.Len(t, list, 1)

		resp, err = other.RemovePostFromTop(ctx, post.Id)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
		event = waitFor(t, wsClient, model.WebsocketEventPostRemovedFromTop)
		assert.Equal(t, post.Id, event.GetData()["post_id"])

		list, _, err = th.Client.GetOnTopPosts(ctx, th.BasicChannel.Id)
		require.NoError(t, err)
		assert.Empty(t, list)

		resp, err = th.Client.RemovePostFromTop(ctx, post.Id)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("a bad duration is refused", func(t *testing.T) {
		post := th.CreatePost(t)
		_, resp, err := th.Client.PutPostOnTop(ctx, post.Id, -5)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("those who can't read the channel neither see nor change what's on top", func(t *testing.T) {
		post, _, err := th.Client.CreatePost(ctx, &model.Post{ChannelId: th.BasicPrivateChannel.Id, Message: "private"})
		require.NoError(t, err)
		_, _, err = th.Client.PutPostOnTop(ctx, post.Id, 0)
		require.NoError(t, err)

		outsider := th.CreateUser(t)
		th.LinkUserToTeam(t, outsider, th.BasicTeam)
		client := th.CreateClient()
		_, _, err = client.Login(ctx, outsider.Email, outsider.Password)
		require.NoError(t, err)

		_, resp, err := client.GetOnTopPosts(ctx, th.BasicPrivateChannel.Id)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		_, resp, err = client.PutPostOnTop(ctx, post.Id, 0)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		resp, err = client.RemovePostFromTop(ctx, post.Id)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		list, _, err := th.Client.GetOnTopPosts(ctx, th.BasicPrivateChannel.Id)
		require.NoError(t, err)
		assert.Len(t, list, 1, "still on top")
	})
}
