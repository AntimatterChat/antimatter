// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/i18n"
	"github.com/mattermost/mattermost/server/v8/channels/app/platform"
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

// A notification's sender's community nickname is looked up once for all its recipients, not once each.
func TestSendNotifications_CommunityProfileLookupNotRepeatedPerRecipient(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.PrivacySettings.ShowFullName = true })
	team := th.BasicTeam
	sender := th.BasicUser2.DeepCopy()
	_, appErr := th.App.PatchTeam(team.Id, &model.TeamPatch{NameDisplay: model.NewPointer(model.ShowNicknameFullName)})
	require.Nil(t, appErr)
	_, appErr = th.App.PatchCommunityProfile(th.Context, team.Id, sender.Id, &model.CommunityProfilePatch{Nickname: model.NewPointer("Captain")})
	require.Nil(t, appErr)

	notification := &PostNotification{Channel: th.BasicChannel, Post: th.BasicPost, Sender: sender, senderNickname: &communityNicknameMemo{}}
	format, n := th.App.communityNotificationNames(notification.seenBy(th.App, th.BasicUser), model.ShowUsername)
	assert.Equal(t, "Captain", n.GetSenderName(format, false))

	// The profile changes in the store: the recipients of the same notification still get the nickname looked up for
	// the first, so the store wasn't asked again.
	_, appErr = th.App.PatchCommunityProfile(th.Context, team.Id, sender.Id, &model.CommunityProfilePatch{Nickname: model.NewPointer("Admiral")})
	require.Nil(t, appErr)
	for _, recipient := range []*model.User{th.BasicUser, th.SystemAdminUser} {
		format, n = th.App.communityNotificationNames(notification.seenBy(th.App, recipient), model.ShowUsername)
		assert.Equal(t, "Captain", n.GetSenderName(format, false))
	}

	// A new notification looks it up again.
	next := &PostNotification{Channel: th.BasicChannel, Post: th.BasicPost, Sender: sender, senderNickname: &communityNicknameMemo{}}
	format, n = th.App.communityNotificationNames(next, model.ShowUsername)
	assert.Equal(t, "Admiral", n.GetSenderName(format, false))
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

// A nickname change goes live to the community's members who can see the person, as the REST routes give the
// profiles: not to a guest, nor to someone without view_members, who shares no channel with them.
func TestCommunityProfileEventOnlyToThoseWhoSeeThePerson(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	team := th.BasicTeam

	conn := func(user *model.User, guest bool) *platform.WebConn {
		session := &model.Session{UserId: user.Id, Roles: user.GetRawRoles()}
		session.AddProp(model.SessionPropIsGuest, strconv.FormatBool(guest))
		saved, appErr := th.App.CreateSession(th.Context, session)
		require.Nil(t, appErr)
		wc := &platform.WebConn{Platform: th.Server.Platform(), Suite: th.App, UserId: user.Id, T: i18n.T}
		wc.SetConnectionID(model.NewId())
		wc.SetSession(saved)
		wc.SetSessionToken(saved.Token)
		wc.SetSessionExpiresAt(saved.ExpiresAt)
		return wc
	}

	// Alice is in the community's default channels; the guest and the restricted member aren't.
	alice := th.CreateUser(t)
	th.LinkUserToTeam(t, alice, team)
	guest := th.CreateGuest(t)
	th.LinkUserToTeam(t, guest, team)
	guestChannel := th.CreateChannel(t, team)
	th.AddUserToChannel(t, guest, guestChannel)

	// A role like system_user without view_members; in the community, but in none of its channels.
	role, appErr := th.App.CreateRole(&model.Role{
		Name:        "restricted_" + strings.ToLower(model.NewId()[:10]),
		DisplayName: "Restricted",
		Permissions: []string{model.PermissionCreateDirectChannel.Id, model.PermissionCreateGroupChannel.Id},
	})
	require.Nil(t, appErr)
	restricted := th.CreateUser(t)
	restricted, appErr = th.App.UpdateUserRoles(th.Context, restricted.Id, role.Name, false)
	require.Nil(t, appErr)
	_, err := th.App.Srv().Store().Team().SaveMember(th.Context, &model.TeamMember{TeamId: team.Id, UserId: restricted.Id, SchemeUser: true}, -1)
	require.NoError(t, err)

	member, guestConn, restrictedConn := conn(th.BasicUser, false), conn(guest, true), conn(restricted, false)
	event := communityProfileEvent(&model.CommunityProfile{TeamId: team.Id, UserId: alice.Id, Nickname: "Alice (Legal)", UpdateAt: model.GetMillis()})

	assert.True(t, member.ShouldSendEvent(event), "a member who sees everyone")
	assert.False(t, guestConn.ShouldSendEvent(event), "not a guest who can't see Alice")
	assert.False(t, restrictedConn.ShouldSendEvent(event), "nor someone without view_members who can't")

	th.AddUserToChannel(t, alice, guestChannel)
	guestConn.InvalidateCache()
	th.App.Srv().Platform().InvalidateCacheForUser(guest.Id)
	assert.True(t, guestConn.ShouldSendEvent(event), "a guest who shares a channel with her")
}
