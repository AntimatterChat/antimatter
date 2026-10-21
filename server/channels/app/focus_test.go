// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestFocusMode(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	calendar := &model.CustomStatus{Emoji: "calendar", Text: "In a meeting", Duration: "date_and_time", ExpiresAt: time.Now().Add(24 * time.Hour).UTC()}

	user := func(t *testing.T, userID string) *model.User {
		t.Helper()
		u, appErr := th.App.GetUser(th.Context, userID)
		require.Nil(t, appErr)
		return u
	}
	status := func(t *testing.T, userID string) *model.Status {
		t.Helper()
		s, appErr := th.App.GetStatus(userID)
		require.Nil(t, appErr)
		return s
	}
	stored := func(t *testing.T, userID string) *model.UserFocus {
		t.Helper()
		f, err := th.App.Srv().Store().Focus().Get(th.Context, userID)
		require.NoError(t, err)
		return f
	}

	t.Run("starting saves the status and custom status, sets 🎯 and the reason, and Do not disturb until the end", func(t *testing.T) {
		u := th.CreateUser(t)
		th.App.SetStatusAwayIfNeeded(u.Id, true)
		require.Nil(t, th.App.SetCustomStatus(th.Context, u.Id, calendar))

		state, appErr := th.App.StartFocus(th.Context, u.Id, &model.FocusRequest{Duration: "1h", Reason: "Working on the new release"})
		require.Nil(t, appErr)
		assert.True(t, state.Active)
		assert.Equal(t, "Working on the new release", state.Reason)
		assert.Zero(t, state.EndAt%60000, "the end lines up with the Do not disturb expiry")
		assert.InDelta(t, time.Now().Add(time.Hour).UnixMilli(), state.EndAt, float64(time.Minute.Milliseconds()))

		s := status(t, u.Id)
		assert.Equal(t, model.StatusDnd, s.Status)
		assert.True(t, s.Manual)
		assert.Equal(t, state.EndAt/1000, s.DNDEndTime)

		got := user(t, u.Id)
		cs := got.GetCustomStatus()
		require.NotNil(t, cs)
		assert.Equal(t, model.FocusEmoji, cs.Emoji)
		assert.Equal(t, "Working on the new release", cs.Text)
		assert.Equal(t, state.EndAt, cs.ExpiresAt.UnixMilli(), "the custom status expires with the focus")
		assert.Equal(t, strconv.FormatInt(state.EndAt, 10), got.Props[model.UserPropsKeyFocusUntil], "the marker everyone sees")

		f := stored(t, u.Id)
		assert.Equal(t, model.StatusAway, f.PrevStatus)
		assert.True(t, f.PrevManual)
		assert.Contains(t, f.PrevCustomStatus, "calendar")

		t.Run("ending brings everything back", func(t *testing.T) {
			ended, appErr := th.App.EndFocus(th.Context, u.Id)
			require.Nil(t, appErr)
			assert.False(t, ended.Active)
			assert.NotZero(t, ended.EndedAt)

			assert.Equal(t, model.StatusAway, status(t, u.Id).Status, "the status from before")
			assert.True(t, status(t, u.Id).Manual)
			got := user(t, u.Id)
			require.NotNil(t, got.GetCustomStatus())
			assert.Equal(t, "calendar", got.GetCustomStatus().Emoji, "the custom status from before")
			assert.NotContains(t, got.Props, model.UserPropsKeyFocusUntil)

			again, appErr := th.App.EndFocus(th.Context, u.Id)
			require.Nil(t, appErr)
			assert.Equal(t, ended.EndedAt, again.EndedAt, "ending twice changes nothing")
		})
	})

	t.Run("a custom status changed meanwhile stays", func(t *testing.T) {
		u := th.CreateUser(t)
		require.Nil(t, th.App.SetCustomStatus(th.Context, u.Id, calendar))
		_, appErr := th.App.StartFocus(th.Context, u.Id, &model.FocusRequest{Duration: "30m"})
		require.Nil(t, appErr)

		lunch := &model.CustomStatus{Emoji: "hamburger", Text: "Out for lunch"}
		require.Nil(t, th.App.SetCustomStatus(th.Context, u.Id, lunch))
		_, appErr = th.App.EndFocus(th.Context, u.Id)
		require.Nil(t, appErr)

		got := user(t, u.Id)
		require.NotNil(t, got.GetCustomStatus())
		assert.Equal(t, "hamburger", got.GetCustomStatus().Emoji, "the user's own change isn't overwritten")
		assert.NotContains(t, got.Props, model.UserPropsKeyFocusUntil)
	})

	t.Run("no custom status before: none after; an expired one isn't brought back", func(t *testing.T) {
		u := th.CreateUser(t)
		_, appErr := th.App.StartFocus(th.Context, u.Id, &model.FocusRequest{Duration: "30m"})
		require.Nil(t, appErr)
		_, appErr = th.App.EndFocus(th.Context, u.Id)
		require.Nil(t, appErr)
		assert.Nil(t, user(t, u.Id).GetCustomStatus())

		expiring := &model.CustomStatus{Emoji: "calendar", Text: "Soon over", Duration: "date_and_time", ExpiresAt: time.Now().Add(time.Hour).UTC()}
		require.Nil(t, th.App.SetCustomStatus(th.Context, u.Id, expiring))
		_, appErr = th.App.StartFocus(th.Context, u.Id, &model.FocusRequest{Duration: "2h"})
		require.Nil(t, appErr)
		f := stored(t, u.Id)
		f.PrevCustomStatus = `{"emoji":"calendar","text":"Soon over","duration":"date_and_time","expires_at":"2020-01-01T00:00:00Z"}`
		require.NoError(t, th.App.Srv().Store().Focus().Save(th.Context, f))
		_, appErr = th.App.EndFocus(th.Context, u.Id)
		require.Nil(t, appErr)
		assert.Nil(t, user(t, u.Id).GetCustomStatus())
	})

	t.Run("the timer ends it through the Do not disturb expiry", func(t *testing.T) {
		u := th.CreateUser(t)
		th.App.SetStatusOnline(u.Id, false)
		require.Nil(t, th.App.SetCustomStatus(th.Context, u.Id, calendar))
		state, appErr := th.App.StartFocus(th.Context, u.Id, &model.FocusRequest{Duration: "15m", Reason: "Deep work"})
		require.Nil(t, appErr)

		// Its time comes: the focus and its Do not disturb ended a minute ago.
		past := time.Now().Add(-time.Minute).Truncate(time.Minute)
		f := stored(t, u.Id)
		f.EndAt = past.UnixMilli()
		require.NoError(t, th.App.Srv().Store().Focus().Save(th.Context, f))
		s := *status(t, u.Id)
		s.DNDEndTime = past.Unix()
		th.App.SaveAndBroadcastStatus(&s)

		th.App.UpdateDNDStatusOfUsers()

		ended := stored(t, u.Id)
		assert.False(t, ended.IsActive())
		assert.Equal(t, past.UnixMilli(), ended.EndedAt, "it ended at its end, for the summary's window")
		assert.Equal(t, state.StartAt, ended.StartAt)
		assert.Equal(t, model.StatusOnline, status(t, u.Id).Status, "the status from before")
		got := user(t, u.Id)
		require.NotNil(t, got.GetCustomStatus())
		assert.Equal(t, "calendar", got.GetCustomStatus().Emoji)
		assert.NotContains(t, got.Props, model.UserPropsKeyFocusUntil)
	})

	t.Run("picking a status ends it, and the status picked stays", func(t *testing.T) {
		u := th.CreateUser(t)
		require.Nil(t, th.App.SetCustomStatus(th.Context, u.Id, calendar))
		_, appErr := th.App.StartFocus(th.Context, u.Id, &model.FocusRequest{Duration: "1h"})
		require.Nil(t, appErr)

		th.App.SetStatusAwayIfNeeded(u.Id, true)

		assert.False(t, stored(t, u.Id).IsActive())
		assert.Equal(t, model.StatusAway, status(t, u.Id).Status)
		assert.Equal(t, "calendar", user(t, u.Id).GetCustomStatus().Emoji)

		// So does picking Do not disturb on its own: it's no longer the focus's.
		_, appErr = th.App.StartFocus(th.Context, u.Id, &model.FocusRequest{Duration: "1h"})
		require.Nil(t, appErr)
		th.App.SetStatusDoNotDisturbTimed(u.Id, time.Now().Add(3*time.Hour).Unix())
		assert.False(t, stored(t, u.Id).IsActive())
		assert.Equal(t, model.StatusDnd, status(t, u.Id).Status)

		// Activity (not a status picked) doesn't end it.
		_, appErr = th.App.StartFocus(th.Context, u.Id, &model.FocusRequest{Duration: "1h"})
		require.Nil(t, appErr)
		th.App.SetStatusOnline(u.Id, false)
		assert.True(t, stored(t, u.Id).IsActive())
		assert.Equal(t, model.StatusDnd, status(t, u.Id).Status)
		_, appErr = th.App.EndFocus(th.Context, u.Id)
		require.Nil(t, appErr)
	})

	t.Run("Do not disturb from before comes back, with its own end", func(t *testing.T) {
		u := th.CreateUser(t)
		before := time.Now().Add(5 * time.Hour).Truncate(time.Minute).Unix()
		th.App.SetStatusDoNotDisturbTimed(u.Id, before)
		_, appErr := th.App.StartFocus(th.Context, u.Id, &model.FocusRequest{Duration: "1h"})
		require.Nil(t, appErr)
		_, appErr = th.App.EndFocus(th.Context, u.Id)
		require.Nil(t, appErr)
		s := status(t, u.Id)
		assert.Equal(t, model.StatusDnd, s.Status)
		assert.Equal(t, before, s.DNDEndTime)

		v := th.CreateUser(t)
		th.App.SetStatusDoNotDisturb(v.Id)
		_, appErr = th.App.StartFocus(th.Context, v.Id, &model.FocusRequest{Duration: "1h"})
		require.Nil(t, appErr)
		_, appErr = th.App.EndFocus(th.Context, v.Id)
		require.Nil(t, appErr)
		s = status(t, v.Id)
		assert.Equal(t, model.StatusDnd, s.Status)
		assert.Zero(t, s.DNDEndTime, "Do not disturb until turned off stays so")
	})

	t.Run("focusing again changes the end and the reason, and keeps what comes back", func(t *testing.T) {
		u := th.CreateUser(t)
		th.App.SetStatusAwayIfNeeded(u.Id, true)
		first, appErr := th.App.StartFocus(th.Context, u.Id, &model.FocusRequest{Duration: "1h", Reason: "One"})
		require.Nil(t, appErr)
		second, appErr := th.App.StartFocus(th.Context, u.Id, &model.FocusRequest{Duration: "3h", Reason: "Two"})
		require.Nil(t, appErr)
		assert.Equal(t, first.StartAt, second.StartAt)
		assert.Greater(t, second.EndAt, first.EndAt)
		assert.Equal(t, "Two", user(t, u.Id).GetCustomStatus().Text)
		assert.Equal(t, model.StatusAway, stored(t, u.Id).PrevStatus, "not the focus's own Do not disturb")
		_, appErr = th.App.EndFocus(th.Context, u.Id)
		require.Nil(t, appErr)
		assert.Equal(t, model.StatusAway, status(t, u.Id).Status)
	})

	t.Run("without custom statuses, only the marker and Do not disturb", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.EnableCustomUserStatuses = false })
		defer th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.EnableCustomUserStatuses = true })
		u := th.CreateUser(t)
		_, appErr := th.App.StartFocus(th.Context, u.Id, &model.FocusRequest{Duration: "1h", Reason: "Quiet"})
		require.Nil(t, appErr)
		assert.Nil(t, user(t, u.Id).GetCustomStatus())
		assert.Contains(t, user(t, u.Id).Props, model.UserPropsKeyFocusUntil)
		_, appErr = th.App.EndFocus(th.Context, u.Id)
		require.Nil(t, appErr)
	})

	t.Run("refused without statuses, for bots, and for bad requests", func(t *testing.T) {
		_, appErr := th.App.StartFocus(th.Context, th.BasicUser.Id, &model.FocusRequest{Duration: "10s"})
		require.NotNil(t, appErr)

		bot := th.CreateBot(t)
		_, appErr = th.App.StartFocus(th.Context, bot.UserId, &model.FocusRequest{Duration: "1h"})
		require.NotNil(t, appErr)

		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserStatuses = false })
		defer th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableUserStatuses = true })
		_, appErr = th.App.StartFocus(th.Context, th.BasicUser.Id, &model.FocusRequest{Duration: "1h"})
		require.NotNil(t, appErr)
	})

	t.Run("the marker can't be changed through the profile", func(t *testing.T) {
		u := th.CreateUser(t)
		_, appErr := th.App.StartFocus(th.Context, u.Id, &model.FocusRequest{Duration: "1h"})
		require.Nil(t, appErr)
		marker := user(t, u.Id).Props[model.UserPropsKeyFocusUntil]

		patched, appErr := th.App.PatchUser(th.Context, u.Id, &model.UserPatch{Props: model.StringMap{"other": "x"}}, false)
		require.Nil(t, appErr)
		assert.Equal(t, marker, patched.Props[model.UserPropsKeyFocusUntil], "a patch of the props keeps it")
		assert.Equal(t, "x", patched.Props["other"])

		_, appErr = th.App.EndFocus(th.Context, u.Id)
		require.Nil(t, appErr)
		patched, appErr = th.App.PatchUser(th.Context, u.Id, &model.UserPatch{Props: model.StringMap{model.UserPropsKeyFocusUntil: "99999999999999"}}, false)
		require.Nil(t, appErr)
		assert.NotContains(t, patched.Props, model.UserPropsKeyFocusUntil, "nor sets it")
	})
}

func TestFocusModeMutesNotifications(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	u := th.BasicUser2
	th.App.SetStatusOffline(u.Id, true, false)
	post := &model.Post{Id: model.NewId(), ChannelId: th.BasicChannel.Id, UserId: th.BasicUser.Id, Message: "@" + u.Username}
	props := model.GetDefaultChannelNotifyProps()

	status, appErr := th.App.GetStatus(u.Id)
	require.Nil(t, appErr)
	require.Equal(t, model.NotificationReason(""), doesStatusAllowPushNotification(u.NotifyProps, status, th.BasicChannel.Id, false), "offline: pushed")
	require.True(t, th.App.userAllowsEmail(th.Context, u, props, post), "offline: emailed")
	require.True(t, th.App.persistentNotificationsAllowedForStatus(u.Id))

	_, appErr = th.App.StartFocus(th.Context, u.Id, &model.FocusRequest{Duration: "1h", Reason: "Focusing"})
	require.Nil(t, appErr)

	status, appErr = th.App.GetStatus(u.Id)
	require.Nil(t, appErr)
	assert.Equal(t, model.NotificationReasonUserStatus, doesStatusAllowPushNotification(u.NotifyProps, status, th.BasicChannel.Id, false), "no push while focusing, urgent messages included")
	assert.Equal(t, model.NotificationReasonUserStatus, doesStatusAllowPushNotification(u.NotifyProps, status, th.BasicChannel.Id, true), "nor for threads")
	assert.False(t, th.App.userAllowsEmail(th.Context, u, props, post), "no email while focusing")
	assert.False(t, th.App.persistentNotificationsAllowedForStatus(u.Id), "no persistent notifications while focusing")

	_, appErr = th.App.EndFocus(th.Context, u.Id)
	require.Nil(t, appErr)
	status, appErr = th.App.GetStatus(u.Id)
	require.Nil(t, appErr)
	assert.Equal(t, model.StatusOffline, status.Status)
	assert.Equal(t, model.NotificationReason(""), doesStatusAllowPushNotification(u.NotifyProps, status, th.BasicChannel.Id, false), "pushed again after")
	assert.True(t, th.App.userAllowsEmail(th.Context, u, props, post), "emailed again after")
}
