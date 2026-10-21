// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestFocusAPI(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	ctx := context.Background()
	me := th.BasicUser

	t.Run("start, read and end your own focus", func(t *testing.T) {
		state, resp, err := th.Client.StartFocus(ctx, model.Me, &model.FocusRequest{Duration: "1h", Reason: "Working on the new release"})
		require.NoError(t, err)
		checkHTTPStatus(t, resp, http.StatusCreated)
		assert.True(t, state.Active)
		assert.Equal(t, "Working on the new release", state.Reason)

		got, _, err := th.Client.GetFocus(ctx, me.Id)
		require.NoError(t, err)
		assert.Equal(t, state, got)

		status, _, err := th.Client.GetUserStatus(ctx, me.Id, "")
		require.NoError(t, err)
		assert.Equal(t, model.StatusDnd, status.Status)
		assert.Equal(t, state.EndAt/1000, status.DNDEndTime)

		// Others see the 🎯 custom status and the marker, not the focus itself.
		other := th.CreateClient()
		th.LoginBasic2WithClient(t, other)
		seen, _, err := other.GetUser(ctx, me.Id, "")
		require.NoError(t, err)
		assert.Contains(t, seen.Props[model.UserPropsKeyCustomStatus], `"emoji":"dart"`)
		assert.NotEmpty(t, seen.Props[model.UserPropsKeyFocusUntil])

		ended, _, err := th.Client.EndFocus(ctx, model.Me)
		require.NoError(t, err)
		assert.False(t, ended.Active)
		assert.NotZero(t, ended.EndedAt)

		got, _, err = th.Client.GetFocus(ctx, model.Me)
		require.NoError(t, err)
		assert.False(t, got.Active)
		assert.Equal(t, ended.EndedAt, got.EndedAt, "the last focus, for its summary")

		status, _, err = th.Client.GetUserStatus(ctx, me.Id, "")
		require.NoError(t, err)
		assert.NotEqual(t, model.StatusDnd, status.Status)
	})

	t.Run("bad requests", func(t *testing.T) {
		for name, request := range map[string]*model.FocusRequest{
			"no end":            {Reason: "x"},
			"too short":         {Duration: "10s"},
			"too long":          {Duration: "200h"},
			"a reason too long": {Duration: "1h", Reason: strings.Repeat("a", model.CustomStatusTextMaxRunes+1)},
		} {
			_, resp, err := th.Client.StartFocus(ctx, model.Me, request)
			require.Error(t, err, name)
			checkHTTPStatus(t, resp, http.StatusBadRequest)
		}
	})

	t.Run("nobody else may start, end or read someone's focus or summary (regression)", func(t *testing.T) {
		_, _, err := th.Client.StartFocus(ctx, model.Me, &model.FocusRequest{Duration: "1h", Reason: "Private"})
		require.NoError(t, err)
		defer func() {
			_, _, endErr := th.Client.EndFocus(ctx, model.Me)
			require.NoError(t, endErr)
		}()

		other := th.CreateClient()
		th.LoginBasic2WithClient(t, other)
		now := model.GetMillis()
		for name, client := range map[string]*model.Client4{"another user": other, "a system admin": th.SystemAdminClient} {
			var resp *model.Response
			_, resp, err = client.GetFocus(ctx, me.Id)
			require.Error(t, err, name)
			CheckForbiddenStatus(t, resp)
			_, resp, err = client.StartFocus(ctx, me.Id, &model.FocusRequest{Duration: "2h"})
			require.Error(t, err, name)
			CheckForbiddenStatus(t, resp)
			_, resp, err = client.EndFocus(ctx, me.Id)
			require.Error(t, err, name)
			CheckForbiddenStatus(t, resp)
			_, resp, err = client.GetFocusSummary(ctx, me.Id, now-3600000, now)
			require.Error(t, err, name)
			CheckForbiddenStatus(t, resp)
			resp, err = client.MarkFocusSummaryRead(ctx, me.Id, now-3600000, now)
			require.Error(t, err, name)
			CheckForbiddenStatus(t, resp)
		}

		state, _, err := th.Client.GetFocus(ctx, model.Me)
		require.NoError(t, err)
		assert.True(t, state.Active, "still focusing")
		assert.Equal(t, "Private", state.Reason, "unchanged")

		_, resp, err := th.Client.GetFocus(ctx, "junk")
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("picking a status ends the focus", func(t *testing.T) {
		_, _, err := th.Client.StartFocus(ctx, model.Me, &model.FocusRequest{Duration: "1h"})
		require.NoError(t, err)
		_, _, err = th.Client.UpdateUserStatus(ctx, me.Id, &model.Status{UserId: me.Id, Status: model.StatusOnline})
		require.NoError(t, err)
		state, _, err := th.Client.GetFocus(ctx, model.Me)
		require.NoError(t, err)
		assert.False(t, state.Active)
		status, _, err := th.Client.GetUserStatus(ctx, me.Id, "")
		require.NoError(t, err)
		assert.Equal(t, model.StatusOnline, status.Status, "the status picked stays")
	})

	t.Run("focus_ended goes to every session of the user, and only theirs", func(t *testing.T) {
		second := th.CreateClient()
		th.LoginBasicWithClient(t, second)
		other := th.CreateClient()
		th.LoginBasic2WithClient(t, other)
		wsFirst := th.CreateConnectedWebSocketClient(t)
		wsSecond := th.CreateConnectedWebSocketClientWithClient(t, second)
		wsOther := th.CreateConnectedWebSocketClientWithClient(t, other)

		started, _, err := th.Client.StartFocus(ctx, model.Me, &model.FocusRequest{Duration: "1h"})
		require.NoError(t, err)
		ended, _, err := second.EndFocus(ctx, model.Me)
		require.NoError(t, err)

		waitFor := func(ws *model.WebSocketClient) *model.WebSocketEvent {
			t.Helper()
			timeout := time.After(5 * time.Second)
			for {
				select {
				case event := <-ws.EventChannel:
					if event.EventType() == model.WebsocketEventFocusEnded {
						return event
					}
				case <-timeout:
					require.FailNow(t, "no focus_ended event")
					return nil
				}
			}
		}
		for _, ws := range []*model.WebSocketClient{wsFirst, wsSecond} {
			event := waitFor(ws)
			assert.Equal(t, me.Id, event.GetBroadcast().UserId)
			assert.EqualValues(t, started.StartAt, event.GetData()["start_at"])
			assert.EqualValues(t, ended.EndedAt, event.GetData()["end_at"])
			assert.Equal(t, model.FocusEndedByUser, event.GetData()["ended_by"])
		}

		timeout := time.After(time.Second)
		for done := false; !done; {
			select {
			case event := <-wsOther.EventChannel:
				require.NotEqual(t, model.WebsocketEventFocusEnded, event.EventType(), "nobody else hears of it")
			case <-timeout:
				done = true
			}
		}
	})

	t.Run("the summary: only what the user can read, the counts, mark read up to the end", func(t *testing.T) {
		other := th.CreateClient()
		th.LoginBasic2WithClient(t, other)
		dm, _, err := other.CreateDirectChannel(ctx, th.BasicUser2.Id, me.Id)
		require.NoError(t, err)
		secret, _, err := th.SystemAdminClient.CreateChannel(ctx, &model.Channel{TeamId: th.BasicTeam.Id, Name: "focus-secret-" + model.NewId()[:8], DisplayName: "Secret", Type: model.ChannelTypePrivate})
		require.NoError(t, err)
		_, _, err = th.SystemAdminClient.AddChannelMember(ctx, secret.Id, th.BasicUser2.Id)
		require.NoError(t, err)

		// Everything read before the focus.
		_, _, err = th.Client.ViewChannel(ctx, me.Id, &model.ChannelView{ChannelId: th.BasicChannel.Id})
		require.NoError(t, err)
		_, _, err = th.Client.ViewChannel(ctx, me.Id, &model.ChannelView{ChannelId: dm.Id})
		require.NoError(t, err)
		time.Sleep(5 * time.Millisecond)

		started, _, err := th.Client.StartFocus(ctx, model.Me, &model.FocusRequest{Duration: "1h"})
		require.NoError(t, err)
		post := func(channelID, message string) *model.Post {
			p, _, postErr := other.CreatePost(ctx, &model.Post{ChannelId: channelID, Message: message})
			require.NoError(t, postErr)
			return p
		}
		post(th.BasicChannel.Id, "one")
		last := post(th.BasicChannel.Id, "two @"+me.Username)
		post(dm.Id, "are you there?")
		post(secret.Id, "not for them")
		time.Sleep(5 * time.Millisecond)
		ended, _, err := th.Client.EndFocus(ctx, model.Me)
		require.NoError(t, err)
		time.Sleep(5 * time.Millisecond)
		after := post(th.BasicChannel.Id, "after the focus")

		summary, _, err := th.Client.GetFocusSummary(ctx, model.Me, started.StartAt, ended.EndedAt)
		require.NoError(t, err)
		require.NotEmpty(t, summary.Channels)
		assert.Equal(t, dm.Id, summary.Channels[0].ChannelId, "direct messages first")
		byID := map[string]*model.FocusSummaryChannel{}
		for _, c := range summary.Channels {
			byID[c.ChannelId] = c
		}
		assert.NotContains(t, byID, secret.Id, "only channels the user is a member of")
		require.Contains(t, byID, th.BasicChannel.Id)
		assert.Equal(t, int64(2), byID[th.BasicChannel.Id].Count, "the messages of the window, not the one after")
		assert.Equal(t, th.BasicUser2.Id, byID[th.BasicChannel.Id].LastSenderId)
		assert.Equal(t, last.CreateAt, byID[th.BasicChannel.Id].LastPostAt)
		assert.Equal(t, int64(1), byID[dm.Id].Count)
		assert.GreaterOrEqual(t, summary.MentionCount, int64(2), "the mention and the direct message")

		_, err = th.Client.MarkFocusSummaryRead(ctx, model.Me, started.StartAt, ended.EndedAt)
		require.NoError(t, err)
		member, appErr := th.App.GetChannelMember(th.Context, th.BasicChannel.Id, me.Id)
		require.Nil(t, appErr)
		assert.GreaterOrEqual(t, member.LastViewedAt, last.CreateAt, "read up to the end")
		assert.Less(t, member.LastViewedAt, after.CreateAt, "what came after stays unread")
		member, appErr = th.App.GetChannelMember(th.Context, dm.Id, me.Id)
		require.Nil(t, appErr)
		assert.Zero(t, member.MentionCount, "the direct message is read")

		summary, _, err = th.Client.GetFocusSummary(ctx, model.Me, started.StartAt, ended.EndedAt)
		require.NoError(t, err)
		assert.Empty(t, summary.Channels, "nothing left from the window")
	})

	t.Run("the summary's window is bounded", func(t *testing.T) {
		now := model.GetMillis()
		_, resp, err := th.Client.GetFocusSummary(ctx, model.Me, now, now-1000)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		_, resp, err = th.Client.GetFocusSummary(ctx, model.Me, now-30*24*3600000, now)
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})
}
