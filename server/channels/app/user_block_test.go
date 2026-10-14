// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestUserBlocking(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	newUser := func(t *testing.T) *model.User {
		t.Helper()
		user := th.CreateUser(t)
		th.LinkUserToTeam(t, user, th.BasicTeam)
		return user
	}
	setPref := func(t *testing.T, userID, category, name, value string) {
		t.Helper()
		if value == "" {
			require.Nil(t, th.App.DeletePreferences(th.Context, userID, model.Preferences{{UserId: userID, Category: category, Name: name}}))
			return
		}
		require.Nil(t, th.App.UpdatePreferences(th.Context, userID, model.Preferences{{UserId: userID, Category: category, Name: name, Value: value}}))
	}
	setLevel := func(t *testing.T, userID, otherID, level string) {
		t.Helper()
		setPref(t, userID, model.PreferenceCategoryUserBlock, otherID, level)
	}
	post := func(t *testing.T, user *model.User, channel *model.Channel, message string) (*model.Post, *model.AppError) {
		t.Helper()
		p, _, appErr := th.App.CreatePost(th.Context, &model.Post{UserId: user.Id, ChannelId: channel.Id, Message: message}, channel, model.CreatePostFlags{})
		return p, appErr
	}
	mustPost := func(t *testing.T, user *model.User, channel *model.Channel, message string) {
		t.Helper()
		_, appErr := post(t, user, channel, message)
		require.Nil(t, appErr)
	}
	member := func(t *testing.T, channelID, userID string) *model.ChannelMember {
		t.Helper()
		m, appErr := th.App.GetChannelMember(th.Context, channelID, userID)
		require.Nil(t, appErr)
		return m
	}
	requestState := func(t *testing.T, channelID, userID string) string {
		t.Helper()
		return member(t, channelID, userID).NotifyProps[model.ChannelMemberDMRequestProp]
	}
	dm := func(t *testing.T, from, to *model.User) *model.Channel {
		t.Helper()
		channel, appErr := th.App.GetOrCreateDirectChannel(th.Context, from.Id, to.Id)
		require.Nil(t, appErr)
		return channel
	}
	setEnabled := func(enabled bool) {
		th.App.UpdateConfig(func(cfg *model.Config) {
			*cfg.PrivacySettings.EnableUserBlocking = enabled
		})
	}

	t.Run("someone blocked can't start a direct message or write in one, nor can who blocked them", func(t *testing.T) {
		alice, bob, carol := newUser(t), newUser(t), newUser(t)
		channel := dm(t, alice, bob)
		mustPost(t, alice, channel, "hi")

		setLevel(t, bob.Id, alice.Id, model.UserBlockBlock)
		_, appErr := post(t, alice, channel, "hello?")
		require.NotNil(t, appErr)
		assert.Equal(t, "app.user_block.blocked.app_error", appErr.Id)
		assert.Equal(t, http.StatusForbidden, appErr.StatusCode)

		_, appErr = post(t, bob, channel, "bye")
		require.NotNil(t, appErr)
		assert.Equal(t, "app.user_block.you_blocked.app_error", appErr.Id)

		// The conversation still opens, to read it.
		again, appErr := th.App.GetOrCreateDirectChannel(th.Context, alice.Id, bob.Id)
		require.Nil(t, appErr)
		assert.Equal(t, channel.Id, again.Id)

		setLevel(t, carol.Id, alice.Id, model.UserBlockBlock)
		_, appErr = th.App.GetOrCreateDirectChannel(th.Context, alice.Id, carol.Id)
		require.NotNil(t, appErr)
		assert.Equal(t, "app.user_block.blocked.app_error", appErr.Id)

		// Ignoring doesn't stop anyone from writing.
		setLevel(t, bob.Id, alice.Id, model.UserBlockIgnore)
		mustPost(t, alice, channel, "still there")

		setLevel(t, bob.Id, alice.Id, "")
		mustPost(t, bob, channel, "welcome back")
	})

	t.Run("someone blocked can't start a group message with who blocked them, others can", func(t *testing.T) {
		alice, bob, carol := newUser(t), newUser(t), newUser(t)
		setLevel(t, bob.Id, alice.Id, model.UserBlockBlock)

		_, appErr := th.App.CreateGroupChannel(th.Context, []string{alice.Id, bob.Id, carol.Id}, alice.Id)
		require.NotNil(t, appErr)
		assert.Equal(t, "app.user_block.group_blocked.app_error", appErr.Id)

		group, appErr := th.App.CreateGroupChannel(th.Context, []string{alice.Id, bob.Id, carol.Id}, carol.Id)
		require.Nil(t, appErr)

		// Once it exists, it opens for everyone, and everyone writes in it.
		again, appErr := th.App.CreateGroupChannel(th.Context, []string{alice.Id, bob.Id, carol.Id}, alice.Id)
		require.Nil(t, appErr)
		assert.Equal(t, group.Id, again.Id)
		mustPost(t, alice, group, "hi all")

		// Who blocks starts group messages with whom they blocked.
		dave := newUser(t)
		_, appErr = th.App.CreateGroupChannel(th.Context, []string{alice.Id, bob.Id, dave.Id}, bob.Id)
		require.Nil(t, appErr)
	})

	t.Run("system admins reach people who blocked them", func(t *testing.T) {
		alice := newUser(t)
		setLevel(t, alice.Id, th.SystemAdminUser.Id, model.UserBlockBlock)
		channel := dm(t, th.SystemAdminUser, alice)
		mustPost(t, th.SystemAdminUser, channel, "a notice")
	})

	t.Run("ignoring or blocking someone silences their mentions", func(t *testing.T) {
		alice, bob := newUser(t), newUser(t)
		channel := th.CreateChannel(t, th.BasicTeam)
		th.AddUserToChannel(t, alice, channel)
		th.AddUserToChannel(t, bob, channel)

		setLevel(t, bob.Id, alice.Id, model.UserBlockIgnore)
		mustPost(t, alice, channel, "@"+bob.Username+" look")
		assert.Zero(t, member(t, channel.Id, bob.Id).MentionCount)

		setLevel(t, bob.Id, alice.Id, model.UserBlockBlock)
		mustPost(t, alice, channel, "@"+bob.Username+" look!")
		assert.Zero(t, member(t, channel.Id, bob.Id).MentionCount)

		setLevel(t, bob.Id, alice.Id, "")
		mustPost(t, alice, channel, "@"+bob.Username+" look!!")
		assert.EqualValues(t, 1, member(t, channel.Id, bob.Id).MentionCount)
	})

	t.Run("direct messages from people you don't know wait as requests when you ask first", func(t *testing.T) {
		alice, bob := newUser(t), newUser(t)
		setPref(t, bob.Id, model.PreferenceCategoryDMPrivacy, model.PreferenceNameDMPrivacyDefault, model.DMPrivacyAsk)

		channel := dm(t, alice, bob)
		assert.Equal(t, model.DMRequestPending, requestState(t, channel.Id, bob.Id))
		assert.Equal(t, model.DMRequestAccepted, requestState(t, channel.Id, alice.Id))

		mustPost(t, alice, channel, "hi")
		mustPost(t, alice, channel, "it's me")
		assert.Zero(t, member(t, channel.Id, bob.Id).MentionCount, "a request doesn't notify")

		m, appErr := th.App.UpdateMessageRequest(th.Context, channel, bob.Id, model.MessageRequestActionAccept)
		require.Nil(t, appErr)
		assert.Equal(t, model.DMRequestAccepted, m.NotifyProps[model.ChannelMemberDMRequestProp])

		mustPost(t, alice, channel, "thanks")
		assert.EqualValues(t, 1, member(t, channel.Id, bob.Id).MentionCount)
	})

	t.Run("replying to a request accepts it", func(t *testing.T) {
		alice, bob := newUser(t), newUser(t)
		setPref(t, bob.Id, model.PreferenceCategoryDMPrivacy, model.PreferenceNameDMPrivacyDefault, model.DMPrivacyAsk)
		channel := dm(t, alice, bob)
		mustPost(t, alice, channel, "hi")
		mustPost(t, bob, channel, "hello")
		assert.Equal(t, model.DMRequestAccepted, requestState(t, channel.Id, bob.Id))
	})

	t.Run("a community's choice overrides the choice for all", func(t *testing.T) {
		alice, bob := newUser(t), newUser(t)
		setPref(t, bob.Id, model.PreferenceCategoryDMPrivacy, model.PreferenceNameDMPrivacyDefault, model.DMPrivacyAsk)
		setPref(t, bob.Id, model.PreferenceCategoryDMPrivacy, th.BasicTeam.Id, model.DMPrivacyEveryone)
		assert.Equal(t, model.DMRequestAccepted, requestState(t, dm(t, alice, bob).Id, bob.Id))

		carol, dave := newUser(t), newUser(t)
		setPref(t, dave.Id, model.PreferenceCategoryDMPrivacy, th.BasicTeam.Id, model.DMPrivacyAsk)
		assert.Equal(t, model.DMRequestPending, requestState(t, dm(t, carol, dave).Id, dave.Id))

		// Any community in common that accepts lets them through.
		other := th.CreateTeam(t)
		th.LinkUserToTeam(t, carol, other)
		th.LinkUserToTeam(t, dave, other)
		erin := newUser(t)
		th.LinkUserToTeam(t, erin, other)
		assert.Equal(t, model.DMRequestAccepted, requestState(t, dm(t, erin, dave).Id, dave.Id))
	})

	t.Run("bots and system admins always get through", func(t *testing.T) {
		bob := newUser(t)
		setPref(t, bob.Id, model.PreferenceCategoryDMPrivacy, model.PreferenceNameDMPrivacyDefault, model.DMPrivacyAsk)
		assert.Equal(t, model.DMRequestAccepted, requestState(t, dm(t, th.SystemAdminUser, bob).Id, bob.Id))

		bot := th.CreateBot(t)
		botUser, appErr := th.App.GetUser(th.Context, bot.UserId)
		require.Nil(t, appErr)
		assert.Equal(t, model.DMRequestAccepted, requestState(t, dm(t, botUser, bob).Id, bob.Id))
	})

	t.Run("an ignored request comes back with a new message, counted", func(t *testing.T) {
		alice, bob := newUser(t), newUser(t)
		setPref(t, bob.Id, model.PreferenceCategoryDMPrivacy, model.PreferenceNameDMPrivacyDefault, model.DMPrivacyAsk)
		channel := dm(t, alice, bob)
		mustPost(t, alice, channel, "hi")

		m, appErr := th.App.UpdateMessageRequest(th.Context, channel, bob.Id, model.MessageRequestActionDismiss)
		require.Nil(t, appErr)
		assert.Equal(t, model.DMRequestDismissed, m.NotifyProps[model.ChannelMemberDMRequestProp])
		assert.Equal(t, "1", m.NotifyProps[model.ChannelMemberDMRequestDismissalsProp])

		mustPost(t, alice, channel, "hi again")
		assert.Equal(t, model.DMRequestPending, requestState(t, channel.Id, bob.Id))
		assert.Zero(t, member(t, channel.Id, bob.Id).MentionCount)

		m, appErr = th.App.UpdateMessageRequest(th.Context, channel, bob.Id, model.MessageRequestActionDismiss)
		require.Nil(t, appErr)
		assert.Equal(t, "2", m.NotifyProps[model.ChannelMemberDMRequestDismissalsProp])

		_, appErr = th.App.UpdateMessageRequest(th.Context, channel, bob.Id, "maybe")
		require.NotNil(t, appErr)
		assert.Equal(t, http.StatusBadRequest, appErr.StatusCode)
	})

	t.Run("the direct messages of someone ignored become requests", func(t *testing.T) {
		alice, bob := newUser(t), newUser(t)
		channel := dm(t, alice, bob)
		mustPost(t, alice, channel, "hi")
		assert.EqualValues(t, 1, member(t, channel.Id, bob.Id).MentionCount)

		setLevel(t, bob.Id, alice.Id, model.UserBlockIgnore)
		mustPost(t, alice, channel, "are you there?")
		assert.Equal(t, model.DMRequestPending, requestState(t, channel.Id, bob.Id))
		assert.EqualValues(t, 1, member(t, channel.Id, bob.Id).MentionCount)
	})

	t.Run("direct messages from before message requests stay accepted once opened", func(t *testing.T) {
		alice, bob, carol := newUser(t), newUser(t), newUser(t)
		setPref(t, bob.Id, model.PreferenceCategoryDMPrivacy, model.PreferenceNameDMPrivacyDefault, model.DMPrivacyAsk)

		setEnabled(false)
		viewed := dm(t, alice, bob)
		unseen := dm(t, carol, bob)
		mustPost(t, alice, viewed, "before")
		_, appErr := th.App.ViewChannel(th.Context, &model.ChannelView{ChannelId: viewed.Id}, bob.Id, "", true)
		require.Nil(t, appErr)
		setEnabled(true)
		assert.Empty(t, requestState(t, viewed.Id, bob.Id))

		mustPost(t, alice, viewed, "hi")
		assert.Equal(t, model.DMRequestAccepted, requestState(t, viewed.Id, bob.Id))

		mustPost(t, carol, unseen, "hi")
		assert.Equal(t, model.DMRequestPending, requestState(t, unseen.Id, bob.Id))
	})

	t.Run("turned off, nothing is enforced and choices are kept", func(t *testing.T) {
		alice, bob := newUser(t), newUser(t)
		channel := th.CreateChannel(t, th.BasicTeam)
		th.AddUserToChannel(t, alice, channel)
		th.AddUserToChannel(t, bob, channel)
		direct := dm(t, alice, bob)
		setLevel(t, bob.Id, alice.Id, model.UserBlockBlock)

		setEnabled(false)
		defer setEnabled(true)
		mustPost(t, alice, direct, "hi")
		mustPost(t, alice, channel, "@"+bob.Username+" look")
		assert.EqualValues(t, 1, member(t, channel.Id, bob.Id).MentionCount)
		_, appErr := th.App.UpdateMessageRequest(th.Context, direct, bob.Id, model.MessageRequestActionAccept)
		require.NotNil(t, appErr)

		level, appErr := th.App.GetUserBlockLevel(bob.Id, alice.Id)
		require.Nil(t, appErr)
		assert.Equal(t, model.UserBlockBlock, level)
	})

	t.Run("no notification from a plugin for someone ignored or a request", func(t *testing.T) {
		alice, bob := newUser(t), newUser(t)
		channel := dm(t, alice, bob)
		assert.False(t, th.App.notificationSilenced(th.Context, bob.Id, alice.Id, channel.Id))

		setLevel(t, bob.Id, alice.Id, model.UserBlockIgnore)
		assert.True(t, th.App.notificationSilenced(th.Context, bob.Id, alice.Id, channel.Id))
		setLevel(t, bob.Id, alice.Id, "")

		_, appErr := th.App.UpdateMessageRequest(th.Context, channel, bob.Id, model.MessageRequestActionRequest)
		require.Nil(t, appErr)
		assert.True(t, th.App.notificationSilenced(th.Context, bob.Id, alice.Id, channel.Id))
	})
}
