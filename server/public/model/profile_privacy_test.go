// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfilePrivacy(t *testing.T) {
	guild, workspace, other := NewId(), NewId(), NewId()

	t.Run("reads the prop, and refuses what it can't read", func(t *testing.T) {
		privacy, ok := ParseProfilePrivacy(`{"default":["position"],"` + guild + `":["full_name","email"]}`)
		require.True(t, ok)
		assert.Equal(t, []string{"position"}, privacy[ProfilePrivacyDefault])

		privacy, ok = ParseProfilePrivacy("")
		assert.True(t, ok)
		assert.Empty(t, privacy)

		for _, raw := range []string{"{", `["full_name"]`, `{"default":["shoe_size"]}`, `{"not-an-id":["email"]}`, `{"default":"email"}`} {
			_, ok := ParseProfilePrivacy(raw)
			assert.False(t, ok, raw)
		}
	})

	t.Run("each community follows the default until it has its own entry", func(t *testing.T) {
		privacy := ProfilePrivacy{ProfilePrivacyDefault: {ProfileFieldPosition}, guild: {ProfileFieldFullName}, workspace: {}}
		assert.Equal(t, map[string]bool{ProfileFieldFullName: true}, privacy.HiddenFrom([]string{guild}))
		assert.Nil(t, privacy.HiddenFrom([]string{workspace}), "an empty entry shows everything there")
		assert.Equal(t, map[string]bool{ProfileFieldPosition: true}, privacy.HiddenFrom([]string{other}))
		assert.Equal(t, map[string]bool{ProfileFieldPosition: true}, privacy.HiddenFrom(nil), "with no community in common, the default decides")
	})

	t.Run("the most restrictive of the communities in common wins", func(t *testing.T) {
		privacy := ProfilePrivacy{guild: {ProfileFieldFullName, ProfileFieldEmail}, workspace: {}}
		// Sharing the open workspace as well never shows more than the guild does.
		assert.Equal(t, map[string]bool{ProfileFieldFullName: true, ProfileFieldEmail: true}, privacy.HiddenFrom([]string{workspace, guild}))
		assert.Equal(t, map[string]bool{ProfileFieldFullName: true, ProfileFieldEmail: true}, privacy.AllHidden())
	})

	t.Run("a prop that can't be read hides everything", func(t *testing.T) {
		user := &User{Id: NewId(), Props: StringMap{UserPropsKeyProfilePrivacy: "{broken"}}
		assert.Equal(t, AllProfileFields(), user.ProfileFieldsHiddenFrom([]string{guild}))
		assert.Nil(t, (&User{Id: NewId()}).ProfileFieldsHiddenFrom([]string{guild}))
	})

	t.Run("hiding blanks the details, and the prop itself, without touching the original props", func(t *testing.T) {
		props := StringMap{UserPropsKeyProfilePrivacy: `{"default":["full_name"]}`, UserPropsKeyRemoteEmail: "a@b.c", "other": "kept"}
		user := &User{
			Id: NewId(), Username: "sofia", FirstName: "Sofia", LastName: "Rossi", Nickname: "Sof", Position: "Designer",
			Email: "sofia@example.com", Timezone: StringMap{"manualTimezone": "Europe/Rome"}, Props: props,
		}
		user.HideProfileFields(map[string]bool{ProfileFieldFullName: true, ProfileFieldPosition: true, ProfileFieldEmail: true, ProfileFieldLocalTime: true})
		assert.Equal(t, "sofia", user.Username)
		assert.Empty(t, user.FirstName)
		assert.Empty(t, user.LastName)
		assert.Equal(t, "Sof", user.Nickname)
		assert.Empty(t, user.Position)
		assert.Empty(t, user.Email)
		assert.Empty(t, user.Timezone)
		assert.Equal(t, StringMap{"other": "kept"}, user.Props)
		assert.Contains(t, props, UserPropsKeyProfilePrivacy, "the map shared with the cache isn't changed")

		// Nothing hidden: only the prop goes.
		user = &User{Id: NewId(), Nickname: "Sof", Props: StringMap{UserPropsKeyProfilePrivacy: `{}`}}
		user.HideProfileFields(nil)
		assert.Equal(t, "Sof", user.Nickname)
		assert.NotContains(t, user.Props, UserPropsKeyProfilePrivacy)
	})

	t.Run("a user with an invalid choice isn't valid", func(t *testing.T) {
		user := &User{Id: NewId(), CreateAt: GetMillis(), UpdateAt: GetMillis(), Username: "sofia", Email: "sofia@example.com", Roles: SystemUserRoleId, Locale: DefaultLocale}
		user.Props = StringMap{UserPropsKeyProfilePrivacy: `{"default":["full_name"],"` + guild + `":[]}`}
		require.Nil(t, user.IsValid())
		user.Props = StringMap{UserPropsKeyProfilePrivacy: `{"default":["password"]}`}
		appErr := user.IsValid()
		require.NotNil(t, appErr)
		assert.Equal(t, "model.user.is_valid.profile_privacy.app_error", appErr.Id)
	})

	t.Run("the etag key names what's hidden", func(t *testing.T) {
		assert.Empty(t, ProfileFieldsKey(nil))
		assert.NotEqual(t, ProfileFieldsKey(map[string]bool{ProfileFieldEmail: true}), ProfileFieldsKey(map[string]bool{ProfileFieldFullName: true}))
	})
}
