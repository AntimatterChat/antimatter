// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestCommunityNotificationNames(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.PrivacySettings.ShowFullName = true })
	team := th.BasicTeam
	sender := th.BasicUser2.DeepCopy()
	sender.FirstName = "Maya"
	sender.LastName = "Lumen"
	notification := &PostNotification{Channel: th.BasicChannel, Post: th.BasicPost, Sender: sender}

	t.Run("the recipient's choice until the community makes one", func(t *testing.T) {
		format, n := th.App.communityNotificationNames(notification, model.ShowFullName)
		assert.Equal(t, model.ShowFullName, format)
		assert.Same(t, notification, n)
	})

	_, appErr := th.App.PatchTeam(team.Id, &model.TeamPatch{NameDisplay: model.NewPointer(model.ShowNicknameFullName)})
	require.Nil(t, appErr)
	_, appErr = th.App.PatchCommunityProfile(th.Context, team.Id, sender.Id, &model.CommunityProfilePatch{Nickname: model.NewPointer("Captain")})
	require.Nil(t, appErr)

	t.Run("the community's choice, with the sender's nickname there", func(t *testing.T) {
		format, n := th.App.communityNotificationNames(notification, model.ShowUsername)
		assert.Equal(t, model.ShowNicknameFullName, format)
		assert.Equal(t, "Captain", n.GetSenderName(format, false))
		assert.Equal(t, "Maya", notification.Sender.FirstName, "the notification shared by the recipients isn't changed")
	})

	t.Run("the community's full names, without their nickname there", func(t *testing.T) {
		_, appErr := th.App.PatchTeam(team.Id, &model.TeamPatch{NameDisplay: model.NewPointer(model.ShowFullName)})
		require.Nil(t, appErr)
		format, n := th.App.communityNotificationNames(notification, model.ShowNicknameFullName)
		assert.Equal(t, model.ShowFullName, format)
		assert.Equal(t, "Maya Lumen", n.GetSenderName(format, false))
	})

	t.Run("without full names on the server, by username whatever the community chose", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.PrivacySettings.ShowFullName = false })
		defer th.App.UpdateConfig(func(cfg *model.Config) { *cfg.PrivacySettings.ShowFullName = true })
		format, _ := th.App.communityNotificationNames(notification, model.ShowUsername)
		assert.Equal(t, model.ShowUsername, format)
	})

	t.Run("direct messages keep the recipient's choice and the global names", func(t *testing.T) {
		dm := th.CreateDmChannel(t, th.BasicUser)
		n := &PostNotification{Channel: dm, Post: th.BasicPost, Sender: sender}
		format, adjusted := th.App.communityNotificationNames(n, model.ShowFullName)
		assert.Equal(t, model.ShowFullName, format)
		assert.Same(t, n, adjusted)
	})
}

func TestCommunityNicknameMatches(t *testing.T) {
	assert.True(t, nicknameMatches("Zephyr Quill", "quil"))
	assert.True(t, nicknameMatches("Zephyr Quill", "@zeph"))
	assert.True(t, nicknameMatches("Zephyr Quill", "zephyr q"))
	assert.False(t, nicknameMatches("Zephyr Quill", "ill"))
	assert.False(t, nicknameMatches("Zephyr Quill", "zephyr smith"))
	assert.False(t, nicknameMatches("Zephyr Quill", " @ "))
}

func TestCommunityProfileStorage(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	team, user := th.BasicTeam, th.BasicUser

	profile, appErr := th.App.GetCommunityProfile(team.Id, user.Id)
	require.Nil(t, appErr)
	assert.True(t, profile.IsEmpty(), "none until set")

	_, appErr = th.App.PatchCommunityProfile(th.Context, team.Id, user.Id, &model.CommunityProfilePatch{Nickname: model.NewPointer(" Captain ")})
	require.Nil(t, appErr)
	profiles, appErr := th.App.GetCommunityProfiles(th.Context, team.Id)
	require.Nil(t, appErr)
	require.Len(t, profiles, 1)
	assert.Equal(t, "Captain", profiles[0].Nickname)

	// One community's profile isn't another's.
	other := th.CreateTeam(t)
	th.LinkUserToTeam(t, user, other)
	profiles, appErr = th.App.GetCommunityProfiles(th.Context, other.Id)
	require.Nil(t, appErr)
	assert.Empty(t, profiles)

	// An empty profile isn't kept.
	_, appErr = th.App.PatchCommunityProfile(th.Context, team.Id, user.Id, &model.CommunityProfilePatch{Nickname: model.NewPointer("")})
	require.Nil(t, appErr)
	_, err := th.App.Srv().Store().Preference().Get(user.Id, model.PreferenceCategoryCommunityProfile, team.Id)
	assert.Error(t, err)

	// Not for people outside the community.
	outsider := th.CreateUser(t)
	_, appErr = th.App.PatchCommunityProfile(th.Context, team.Id, outsider.Id, &model.CommunityProfilePatch{Nickname: model.NewPointer("Lurker")})
	require.NotNil(t, appErr)
	assert.Equal(t, "app.community_profile.not_member.app_error", appErr.Id)
}
