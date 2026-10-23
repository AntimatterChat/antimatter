// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/utils/testutils"
)

// Antimatter community profiles: a nickname and a picture per community, set by the person only; moderators may
// take them off (RBAC regressions).

func TestCommunityProfileRights(t *testing.T) {
	mainHelper.Parallel(t)
	h := newRightsHelper(t, "")
	ctx := context.Background()
	user, client := h.member(t)
	other, otherClient := h.member(t)

	t.Run("the person sets their nickname, the community sees it", func(t *testing.T) {
		profile, _, err := client.PatchCommunityProfile(ctx, h.team.Id, user.Id, &model.CommunityProfilePatch{Nickname: model.NewPointer("  Captain  ")})
		require.NoError(t, err)
		assert.Equal(t, "Captain", profile.Nickname, "trimmed")

		profiles, _, err := otherClient.GetCommunityProfiles(ctx, h.team.Id)
		require.NoError(t, err)
		require.Len(t, profiles, 1)
		assert.Equal(t, user.Id, profiles[0].UserId)
		assert.Equal(t, "Captain", profiles[0].Nickname)
	})

	t.Run("nobody sets someone else's profile, system admins included", func(t *testing.T) {
		_, resp, err := otherClient.PatchCommunityProfile(ctx, h.team.Id, user.Id, &model.CommunityProfilePatch{Nickname: model.NewPointer("Imposter")})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		_, resp, err = h.th.SystemAdminClient.PatchCommunityProfile(ctx, h.team.Id, user.Id, &model.CommunityProfilePatch{Nickname: model.NewPointer("Admin's pick")})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		data, err := testutils.ReadTestFile("test.png")
		require.NoError(t, err)
		_, resp, err = otherClient.SetCommunityAvatar(ctx, h.team.Id, user.Id, data)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		profiles, _, err := client.GetCommunityProfiles(ctx, h.team.Id)
		require.NoError(t, err)
		require.Len(t, profiles, 1)
		assert.Equal(t, "Captain", profiles[0].Nickname)
		assert.Zero(t, profiles[0].AvatarUpdateAt)
	})

	t.Run("nor through the preferences", func(t *testing.T) {
		forged := model.Preference{UserId: other.Id, Category: model.PreferenceCategoryCommunityProfile, Name: h.team.Id, Value: `{"nickname":"Admin","update_at":1}`}
		resp, err := otherClient.UpdatePreferences(ctx, other.Id, model.Preferences{forged})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		mine := model.Preference{UserId: user.Id, Category: model.PreferenceCategoryCommunityProfile, Name: h.team.Id}
		resp, err = client.DeletePreferences(ctx, user.Id, model.Preferences{mine})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)

		profiles, _, err := otherClient.GetCommunityProfiles(ctx, h.team.Id)
		require.NoError(t, err)
		require.Len(t, profiles, 1, "the forged profile wasn't kept, the real one stays")
		assert.Equal(t, user.Id, profiles[0].UserId)
	})

	t.Run("outsiders neither read nor set a community's profiles", func(t *testing.T) {
		outsider := h.th.CreateUser(t)
		outsiderClient := h.th.CreateClient()
		_, _, err := outsiderClient.Login(ctx, outsider.Email, outsider.Password)
		require.NoError(t, err)
		_, resp, err := outsiderClient.GetCommunityProfiles(ctx, h.team.Id)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		_, resp, err = outsiderClient.PatchCommunityProfile(ctx, h.team.Id, outsider.Id, &model.CommunityProfilePatch{Nickname: model.NewPointer("Lurker")})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		_, resp, err = outsiderClient.GetCommunityAvatar(ctx, h.team.Id, user.Id, "")
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("a nickname is checked", func(t *testing.T) {
		_, resp, err := client.PatchCommunityProfile(ctx, h.team.Id, user.Id, &model.CommunityProfilePatch{Nickname: model.NewPointer(strings.Repeat("n", model.CommunityNicknameMaxRunes+1))})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		profile, _, err := client.PatchCommunityProfile(ctx, h.team.Id, user.Id, &model.CommunityProfilePatch{Nickname: model.NewPointer("Cap\ntain")})
		require.NoError(t, err)
		assert.Equal(t, "Captain", profile.Nickname, "control characters are dropped")
	})

	t.Run("a picture of one's own, kept as a square PNG; anything else is refused", func(t *testing.T) {
		data, err := testutils.ReadTestFile("test.png")
		require.NoError(t, err)
		profile, _, err := client.SetCommunityAvatar(ctx, h.team.Id, user.Id, data)
		require.NoError(t, err)
		assert.NotZero(t, profile.AvatarUpdateAt)

		img, _, err := otherClient.GetCommunityAvatar(ctx, h.team.Id, user.Id, "")
		require.NoError(t, err)
		assert.True(t, len(img) > 8 && string(img[1:4]) == "PNG")

		_, resp, err := client.SetCommunityAvatar(ctx, h.team.Id, user.Id, []byte("not an image at all"))
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)

		_, resp, err = client.GetCommunityAvatar(ctx, h.team.Id, other.Id, "")
		require.Error(t, err, "without a picture of their own, the profile picture shows")
		CheckNotFoundStatus(t, resp)
	})

	t.Run("moderators take off a nickname or a picture, nothing else", func(t *testing.T) {
		_, resp, err := otherClient.ResetCommunityProfile(ctx, h.team.Id, user.Id, &model.CommunityProfileReset{Nickname: true})
		require.Error(t, err, "members don't")
		CheckForbiddenStatus(t, resp)

		_, adminClient := h.admin(t)
		profile, _, err := adminClient.ResetCommunityProfile(ctx, h.team.Id, user.Id, &model.CommunityProfileReset{Nickname: true, Avatar: true})
		require.NoError(t, err)
		assert.Empty(t, profile.Nickname)
		assert.Zero(t, profile.AvatarUpdateAt)
		_, resp, err = adminClient.GetCommunityAvatar(ctx, h.team.Id, user.Id, "")
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)

		_, resp, err = adminClient.PatchCommunityProfile(ctx, h.team.Id, user.Id, &model.CommunityProfilePatch{Nickname: model.NewPointer("Renamed")})
		require.Error(t, err, "moderators don't set a nickname")
		CheckForbiddenStatus(t, resp)

		// The people who may remove members, below them: as for timeouts.
		_, modClient, _ := h.withRole(t, model.PermissionRemoveUserFromTeam.Id)
		_, _, err = client.PatchCommunityProfile(ctx, h.team.Id, user.Id, &model.CommunityProfilePatch{Nickname: model.NewPointer("Again")})
		require.NoError(t, err)
		profile, _, err = modClient.ResetCommunityProfile(ctx, h.team.Id, user.Id, &model.CommunityProfileReset{Nickname: true})
		require.NoError(t, err)
		assert.Empty(t, profile.Nickname)
		admin, _ := h.admin(t)
		_, resp, err = modClient.ResetCommunityProfile(ctx, h.team.Id, admin.Id, &model.CommunityProfileReset{Nickname: true})
		require.Error(t, err, "not the community's admins")
		CheckForbiddenStatus(t, resp)

		// One's own: the person resets theirs too.
		_, _, err = client.PatchCommunityProfile(ctx, h.team.Id, user.Id, &model.CommunityProfilePatch{Nickname: model.NewPointer("Mine")})
		require.NoError(t, err)
		profile, _, err = client.ResetCommunityProfile(ctx, h.team.Id, user.Id, &model.CommunityProfileReset{Nickname: true})
		require.NoError(t, err)
		assert.Empty(t, profile.Nickname)
	})
}

// Antimatter: the people a community knows by a nickname of theirs there are found by it in its autocomplete.
func TestCommunityNicknameAutocomplete(t *testing.T) {
	mainHelper.Parallel(t)
	// A workspace: its members aren't in every channel.
	h := newRightsHelper(t, model.TeamKindWorkspace)
	ctx := context.Background()
	person, personClient := h.member(t)
	other, otherClient := h.member(t)
	_, _, err := personClient.PatchCommunityProfile(ctx, h.team.Id, person.Id, &model.CommunityProfilePatch{Nickname: model.NewPointer("Zephyr Quill")})
	require.NoError(t, err)

	for _, term := range []string{"zeph", "quill", "@Zephyr"} {
		ac, _, err := otherClient.AutocompleteUsersInTeam(ctx, h.team.Id, term, 100, "")
		require.NoError(t, err, term)
		found := 0
		for _, u := range ac.Users {
			if u.Id == person.Id {
				found++
			}
		}
		assert.Equal(t, 1, found, term)
	}

	// Not in another community, where they have no such nickname.
	elsewhere := h.th.CreateTeamWithClient(t, h.th.SystemAdminClient)
	h.th.LinkUserToTeam(t, person, elsewhere)
	ac, _, err := h.th.SystemAdminClient.AutocompleteUsersInTeam(ctx, elsewhere.Id, "zephyr", 100, "")
	require.NoError(t, err)
	for _, u := range ac.Users {
		assert.NotEqual(t, person.Id, u.Id)
	}

	// The one typing is in the channel, the person isn't.
	channel := h.channel(t, h.th.SystemAdminClient, model.ChannelTypeOpen, other)
	ac, _, err = otherClient.AutocompleteUsersInChannel(ctx, h.team.Id, channel.Id, "zephyr", 100, "")
	require.NoError(t, err)
	var outside bool
	for _, u := range ac.OutOfChannel {
		outside = outside || u.Id == person.Id
	}
	assert.True(t, outside, "not in the channel: listed out of it")
}

// Antimatter: refusing to be shown by name in a community is hiding one's full name and nickname from it (its privacy
// settings): its members get the person by their username only, everywhere (privacy regression), while a nickname
// chosen for the community still shows and still finds them.
func TestCommunityProfileWithHiddenName(t *testing.T) {
	mainHelper.Parallel(t)
	h := newRightsHelper(t, "")
	ctx := context.Background()
	h.th.App.UpdateConfig(func(cfg *model.Config) { *cfg.PrivacySettings.ShowFullName = true })
	person, personClient := h.member(t)
	_, otherClient := h.member(t)

	privacy, err := json.Marshal(model.ProfilePrivacy{h.team.Id: {model.ProfileFieldFullName, model.ProfileFieldNickname}})
	require.NoError(t, err)
	me, appErr := h.th.App.GetUser(h.th.Context, person.Id)
	require.Nil(t, appErr)
	me = me.DeepCopy()
	me.FirstName, me.LastName, me.Nickname = "Zoë", "Marchetti", "Zo"
	me.Props = model.StringMap{model.UserPropsKeyProfilePrivacy: string(privacy)}
	_, appErr = h.th.App.UpdateUser(h.th.Context, me, false)
	require.Nil(t, appErr)
	_, _, err = personClient.PatchCommunityProfile(ctx, h.team.Id, person.Id, &model.CommunityProfilePatch{Nickname: model.NewPointer("Quillfeather")})
	require.NoError(t, err)

	hidden := func(t *testing.T, u *model.User) {
		t.Helper()
		require.NotNil(t, u)
		assert.Empty(t, u.FirstName)
		assert.Empty(t, u.LastName)
		assert.Empty(t, u.Nickname)
		assert.Equal(t, person.Username, u.Username)
	}

	u, _, err := otherClient.GetUser(ctx, person.Id, "")
	require.NoError(t, err)
	hidden(t, u)

	// Found by the nickname chosen for the community, never by a hidden name.
	ac, _, err := otherClient.AutocompleteUsersInTeam(ctx, h.team.Id, "quill", 100, "")
	require.NoError(t, err)
	var found *model.User
	for _, user := range ac.Users {
		if user.Id == person.Id {
			found = user
		}
	}
	hidden(t, found)
	ac, _, err = otherClient.AutocompleteUsersInTeam(ctx, h.team.Id, "marchetti", 100, "")
	require.NoError(t, err)
	for _, user := range ac.Users {
		assert.NotEqual(t, person.Id, user.Id, "not by the hidden last name")
	}

	profiles, _, err := otherClient.GetCommunityProfiles(ctx, h.team.Id)
	require.NoError(t, err)
	require.Len(t, profiles, 1)
	assert.Equal(t, "Quillfeather", profiles[0].Nickname)
}

// Antimatter: a community's admins choose how it shows people; its members don't.
func TestCommunityNameDisplay(t *testing.T) {
	mainHelper.Parallel(t)
	h := newRightsHelper(t, "")
	ctx := context.Background()
	assert.Empty(t, h.team.NameDisplay, "the server's default until chosen")

	_, member := h.member(t)
	_, resp, err := member.PatchTeam(ctx, h.team.Id, &model.TeamPatch{NameDisplay: model.NewPointer(model.ShowUsername)})
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	_, resp, err = h.th.SystemAdminClient.PatchTeam(ctx, h.team.Id, &model.TeamPatch{NameDisplay: model.NewPointer("first_name")})
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	_, admin := h.admin(t)
	for _, display := range []string{model.ShowFullName, model.ShowNicknameFullName, model.ShowUsername, ""} {
		patched, _, patchErr := admin.PatchTeam(ctx, h.team.Id, &model.TeamPatch{NameDisplay: model.NewPointer(display)})
		require.NoError(t, patchErr, display)
		assert.Equal(t, display, patched.NameDisplay)
		fetched, _, getErr := member.GetTeam(ctx, h.team.Id, "")
		require.NoError(t, getErr)
		assert.Equal(t, display, fetched.NameDisplay, "members see it")
	}

	_, _, err = admin.PatchTeam(ctx, h.team.Id, &model.TeamPatch{NameDisplay: model.NewPointer(model.ShowFullName)})
	require.NoError(t, err)
	patched, _, err := admin.PatchTeam(ctx, h.team.Id, &model.TeamPatch{Description: model.NewPointer("about")})
	require.NoError(t, err)
	assert.Equal(t, model.ShowFullName, patched.NameDisplay, "patching something else keeps it")
}
