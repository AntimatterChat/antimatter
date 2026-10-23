// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/app/platform"
)

// Profile privacy: who sees which details of someone's profile, by the communities they share.
func TestProfilePrivacy(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	guild := th.CreateTeam(t)
	workspace := th.CreateTeam(t)

	// Sofia hides her full name, email and local time from the guild, shows everything in the workspace, and hides
	// her position from everyone else.
	sofia := th.CreateUser(t)
	th.LinkUserToTeam(t, sofia, guild)
	th.LinkUserToTeam(t, sofia, workspace)
	sofia.FirstName, sofia.LastName, sofia.Nickname, sofia.Position = "Sofia", "Rossi", "Sof", "Designer"
	sofia.Timezone = model.StringMap{"useAutomaticTimezone": "false", "manualTimezone": "Europe/Rome"}
	privacy, err := json.Marshal(model.ProfilePrivacy{
		model.ProfilePrivacyDefault: {model.ProfileFieldPosition},
		guild.Id:                    {model.ProfileFieldFullName, model.ProfileFieldEmail, model.ProfileFieldLocalTime},
		workspace.Id:                {},
	})
	require.NoError(t, err)
	sofia.Props = model.StringMap{model.UserPropsKeyProfilePrivacy: string(privacy)}
	sofia, appErr := th.App.UpdateUser(th.Context, sofia, false)
	require.Nil(t, appErr)

	member := func(teams ...*model.Team) *model.User {
		t.Helper()
		user := th.CreateUser(t)
		for _, team := range teams {
			th.LinkUserToTeam(t, user, team)
		}
		return user
	}
	seenBy := func(viewer *model.User, asAdmin bool) *model.User {
		t.Helper()
		user, appErr := th.App.GetUser(th.Context, sofia.Id)
		require.Nil(t, appErr)
		th.App.SanitizeProfile(user, asAdmin)
		th.App.HideProfileDetails(viewer.Id, asAdmin, user)
		return user
	}

	t.Run("someone met in the guild doesn't see what she hides there", func(t *testing.T) {
		seen := seenBy(member(guild), false)
		assert.Empty(t, seen.FirstName)
		assert.Empty(t, seen.LastName)
		assert.Empty(t, seen.Email)
		assert.Empty(t, seen.Timezone)
		assert.Equal(t, "Sof", seen.Nickname)
		assert.Equal(t, "Designer", seen.Position, "the guild's own choice replaces the default")
		assert.Equal(t, sofia.Username, seen.Username)
		assert.NotContains(t, seen.Props, model.UserPropsKeyProfilePrivacy, "nobody is told what she hides")
	})

	t.Run("someone met in the workspace sees everything", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.PrivacySettings.ShowEmailAddress = true })
		seen := seenBy(member(workspace), false)
		assert.Equal(t, "Sofia", seen.FirstName)
		assert.Equal(t, "Designer", seen.Position)
		assert.Equal(t, sofia.Email, seen.Email)
		assert.NotEmpty(t, seen.Timezone)
		assert.NotContains(t, seen.Props, model.UserPropsKeyProfilePrivacy)
	})

	t.Run("someone in both gets the most restrictive: joining the workspace shows them nothing more", func(t *testing.T) {
		seen := seenBy(member(guild, workspace), false)
		assert.Empty(t, seen.FirstName)
		assert.Empty(t, seen.Email)
	})

	t.Run("someone who shares no community with her gets the default", func(t *testing.T) {
		seen := seenBy(member(), false)
		assert.Empty(t, seen.Position)
		assert.Equal(t, "Sofia", seen.FirstName)
	})

	t.Run("she and system admins see everything", func(t *testing.T) {
		seen := seenBy(th.SystemAdminUser, true)
		assert.Equal(t, "Sofia", seen.FirstName)
		assert.Equal(t, "Designer", seen.Position)
		assert.NotEmpty(t, seen.Timezone)

		user, appErr := th.App.GetUser(th.Context, sofia.Id)
		require.Nil(t, appErr)
		th.App.HideProfileDetails(sofia.Id, false, user)
		assert.Equal(t, "Rossi", user.LastName)
		assert.Contains(t, user.Props, model.UserPropsKeyProfilePrivacy)
	})

	t.Run("nobody is found by what she hides from them", func(t *testing.T) {
		viewer := member(guild)
		user, appErr := th.App.GetUser(th.Context, sofia.Id)
		require.Nil(t, appErr)
		other := &model.User{Id: model.NewId(), Username: "rossi.fan", FirstName: "Ann", LastName: "Rossi"}
		users := []*model.User{user, other}
		changed := th.App.HideProfileDetails(viewer.Id, false, users...)
		assert.Equal(t, map[string]bool{sofia.Id: true}, changed)

		assert.Equal(t, []*model.User{other}, FilterHiddenSearchMatches("rossi", users, changed))
		assert.Equal(t, []*model.User{other}, FilterHiddenSearchMatches("@Sofia", users, changed))
		// The others found are kept, whatever they matched by: only what's hidden from the viewer counts.
		assert.Contains(t, FilterHiddenSearchMatches(sofia.Username, users, changed), user, "her username still finds her")
		assert.Contains(t, FilterHiddenSearchMatches("sof", users, changed), user, "her nickname too")
		assert.Len(t, FilterHiddenSearchMatches("rossi", users, nil), 2, "nothing hidden, nothing dropped")
	})

	t.Run("her attributes, when she hides them", func(t *testing.T) {
		attrs, err := json.Marshal(model.ProfilePrivacy{guild.Id: {model.ProfileFieldAttributes}})
		require.NoError(t, err)
		erin := th.CreateUser(t)
		th.LinkUserToTeam(t, erin, guild)
		erin.Props = model.StringMap{model.UserPropsKeyProfilePrivacy: string(attrs)}
		_, appErr := th.App.UpdateUser(th.Context, erin, false)
		require.Nil(t, appErr)

		viewer := member(guild)
		assert.Equal(t, map[string]bool{erin.Id: true}, th.App.attributesHiddenFrom(th.Context, viewer.Id, false, []string{erin.Id, sofia.Id}))
		assert.Empty(t, th.App.attributesHiddenFrom(th.Context, viewer.Id, true, []string{erin.Id}), "admins see them")
		assert.Empty(t, th.App.attributesHiddenFrom(th.Context, erin.Id, false, []string{erin.Id}), "she sees hers")
	})

	t.Run("push and email notifications name her as their recipient may see her", func(t *testing.T) {
		recipient := member(guild)
		channel := &model.Channel{Id: model.NewId(), Type: model.ChannelTypeDirect}
		user, appErr := th.App.GetUser(th.Context, sofia.Id)
		require.Nil(t, appErr)
		notification := &PostNotification{Channel: channel, Post: &model.Post{Message: "hi"}, Sender: user}

		seen := notification.seenBy(th.App, recipient)
		assert.Equal(t, "@"+sofia.Username, seen.GetSenderName(model.ShowFullName, false))
		assert.Equal(t, "Sofia Rossi", notification.GetSenderName(model.ShowFullName, false), "the notification itself isn't changed")

		inWorkspace := member(workspace)
		assert.Equal(t, "Sofia Rossi", notification.seenBy(th.App, inWorkspace).GetSenderName(model.ShowFullName, false))

		// A group message's name lists its members as the recipient sees them.
		group := &PostNotification{Channel: &model.Channel{Id: model.NewId(), Type: model.ChannelTypeGroup}, Post: &model.Post{}, Sender: recipient,
			ProfileMap: map[string]*model.User{sofia.Id: user, recipient.Id: recipient}}
		assert.Equal(t, sofia.Username, group.seenBy(th.App, recipient).GetChannelName(model.ShowFullName, recipient.Id))
	})

	t.Run("a user_updated event gives each recipient what they may see", func(t *testing.T) {
		user, appErr := th.App.GetUser(th.Context, sofia.Id)
		require.Nil(t, appErr)
		th.App.SanitizeProfile(user, false)
		message := model.NewWebSocketEvent(model.WebsocketEventUserUpdated, "", "", "", nil, "")
		th.App.publishUserProfile(message, user)

		// What goes out if the hook can't run: everything hidden somewhere.
		base, ok := message.GetData()["user"].(*model.User)
		require.True(t, ok)
		assert.Empty(t, base.FirstName)
		assert.Empty(t, base.Position)
		assert.NotContains(t, base.Props, model.UserPropsKeyProfilePrivacy)

		_, hooks, args := message.WithoutBroadcastHooks()
		require.Equal(t, []string{broadcastProfilePrivacy}, hooks)
		hook := &profilePrivacyBroadcastHook{}
		process := func(viewer *model.User, roles string) *model.User {
			t.Helper()
			wc := &platform.WebConn{Platform: th.Server.Platform(), Suite: th.App, UserId: viewer.Id}
			wc.SetSession(&model.Session{UserId: viewer.Id, Roles: roles})
			msg := platform.MakeHookedWebSocketEvent(message)
			require.NoError(t, hook.Process(msg, wc, args[0]))
			seen, ok := msg.Get("user").(*model.User)
			require.True(t, ok)
			return seen
		}

		inGuild := process(member(guild), model.SystemUserRoleId)
		assert.Empty(t, inGuild.FirstName)
		assert.Equal(t, "Designer", inGuild.Position)
		assert.NotContains(t, inGuild.Props, model.UserPropsKeyProfilePrivacy)

		inWorkspace := process(member(workspace), model.SystemUserRoleId)
		assert.Equal(t, "Sofia", inWorkspace.FirstName)
		assert.Equal(t, "Designer", inWorkspace.Position)

		admin := process(th.SystemAdminUser, model.SystemAdminRoleId+" "+model.SystemUserRoleId)
		assert.Equal(t, "Sofia", admin.FirstName)

		herself := process(sofia, model.SystemUserRoleId)
		assert.Contains(t, herself.Props, model.UserPropsKeyProfilePrivacy)
	})

	t.Run("someone who hides nothing goes out as before, without the hook", func(t *testing.T) {
		user := th.CreateUser(t)
		message := model.NewWebSocketEvent(model.WebsocketEventUserUpdated, "", "", "", nil, "")
		th.App.publishUserProfile(message, user)
		_, hooks, _ := message.WithoutBroadcastHooks()
		assert.Empty(t, hooks)
		assert.Same(t, user, message.GetData()["user"])
	})

	t.Run("a thread's participants as its follower sees them", func(t *testing.T) {
		viewer := member(guild)
		user, appErr := th.App.GetUser(th.Context, sofia.Id)
		require.Nil(t, appErr)
		participants := []*model.User{user}
		th.App.sanitizeParticipants(participants, viewer.Id)
		assert.Empty(t, participants[0].FirstName)
	})
}
