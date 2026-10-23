// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// Notification settings per community: they apply to that community's channels only, over the person's own, and a
// channel's own settings still win.
func TestCommunityNotifyProps(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	other := th.CreateTeam(t)
	th.LinkUserToTeam(t, th.BasicUser, other)
	th.LinkUserToTeam(t, th.BasicUser2, other)
	otherChannel := th.CreateChannel(t, other)
	th.AddUserToChannel(t, th.BasicUser2, otherChannel)
	th.AddUserToChannel(t, th.BasicUser2, th.BasicChannel)

	// BasicUser2 is notified of everything, @channel included, but in BasicTeam: push for nothing, no @channel.
	user2 := th.BasicUser2
	user2.NotifyProps[model.PushNotifyProp] = model.UserNotifyAll
	user2.NotifyProps[model.ChannelMentionsNotifyProp] = "true"
	user2, appErr := th.App.UpdateUser(th.Context, user2, false)
	require.Nil(t, appErr)
	require.Nil(t, th.App.UpdatePreferences(th.Context, user2.Id, model.Preferences{{
		UserId: user2.Id, Category: model.PreferenceCategoryCommunityNotify, Name: th.BasicTeam.Id,
		Value: `{"push":"none","channel":"false"}`,
	}}))

	profiles := func(teamID string) map[string]*model.User {
		t.Helper()
		user, appErr := th.App.GetUser(th.Context, user2.Id)
		require.Nil(t, appErr)
		return th.App.withCommunityNotifyProps(th.Context, teamID, map[string]*model.User{user2.Id: user})
	}

	t.Run("only that community's channels take them", func(t *testing.T) {
		assert.Equal(t, model.UserNotifyNone, profiles(th.BasicTeam.Id)[user2.Id].NotifyProps[model.PushNotifyProp])
		assert.Equal(t, model.UserNotifyAll, profiles(other.Id)[user2.Id].NotifyProps[model.PushNotifyProp])
		assert.Equal(t, model.UserNotifyAll, profiles("")[user2.Id].NotifyProps[model.PushNotifyProp], "direct and group messages follow the person's own")

		// The cached profile isn't changed.
		user, appErr := th.App.GetUser(th.Context, user2.Id)
		require.Nil(t, appErr)
		assert.Equal(t, model.UserNotifyAll, user.NotifyProps[model.PushNotifyProp])
	})

	t.Run("push follows them, and a channel's own setting still wins", func(t *testing.T) {
		post := &model.Post{Id: model.NewId(), UserId: th.BasicUser.Id, ChannelId: th.BasicChannel.Id, Message: "news"}
		assert.Equal(t, model.NotificationReasonLevelSetToNone, doesNotifyPropsAllowPushNotification(profiles(th.BasicTeam.Id)[user2.Id], model.StringMap{model.PushNotifyProp: model.ChannelNotifyDefault}, post, false, false))
		assert.Equal(t, model.NotificationReason(""), doesNotifyPropsAllowPushNotification(profiles(other.Id)[user2.Id], model.StringMap{model.PushNotifyProp: model.ChannelNotifyDefault}, post, false, false))
		assert.Equal(t, model.NotificationReason(""), doesNotifyPropsAllowPushNotification(profiles(th.BasicTeam.Id)[user2.Id], model.StringMap{model.PushNotifyProp: model.ChannelNotifyAll}, post, false, false))
	})

	t.Run("sending a post's notifications applies them", func(t *testing.T) {
		send := func(channel *model.Channel, team *model.Team) []string {
			t.Helper()
			post, _, appErr := th.App.CreatePost(th.Context, &model.Post{UserId: th.BasicUser.Id, ChannelId: channel.Id, Message: "@channel hello"}, channel, model.CreatePostFlags{})
			require.Nil(t, appErr)
			mentions, err := th.App.SendNotifications(th.Context, post, team, channel, th.BasicUser, nil, false)
			require.NoError(t, err)
			return mentions
		}
		assert.False(t, slices.Contains(send(th.BasicChannel, th.BasicTeam), user2.Id), "no @channel in BasicTeam")
		assert.True(t, slices.Contains(send(otherChannel, other), user2.Id), "@channel elsewhere")
	})

	t.Run("an invalid value is refused", func(t *testing.T) {
		appErr := th.App.UpdatePreferences(th.Context, user2.Id, model.Preferences{{
			UserId: user2.Id, Category: model.PreferenceCategoryCommunityNotify, Name: th.BasicTeam.Id, Value: `{"push":"loud"}`,
		}})
		require.NotNil(t, appErr)
	})
}
