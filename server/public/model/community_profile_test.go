// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommunityProfileIsValid(t *testing.T) {
	profile := &CommunityProfile{TeamId: NewId(), UserId: NewId(), Nickname: "Captain"}
	require.Nil(t, profile.IsValid())

	profile.Nickname = strings.Repeat("é", CommunityNicknameMaxRunes)
	assert.Nil(t, profile.IsValid(), "runes, not bytes")
	profile.Nickname += "é"
	assert.NotNil(t, profile.IsValid())
	profile.Nickname = " padded "
	assert.NotNil(t, profile.IsValid(), "normalised first")
	profile.Nickname = "two\nlines"
	assert.NotNil(t, profile.IsValid())
	profile.Nickname = ""
	profile.TeamId = "nope"
	assert.NotNil(t, profile.IsValid())
}

func TestNormalizeCommunityNickname(t *testing.T) {
	assert.Equal(t, "Captain", NormalizeCommunityNickname("  Cap\ttain\n "))
	assert.Equal(t, "", NormalizeCommunityNickname(" \u0007 "))
	assert.Equal(t, "Zoë", NormalizeCommunityNickname("Zoë"))
}

func TestCommunityProfilePreference(t *testing.T) {
	profile := &CommunityProfile{TeamId: NewId(), UserId: NewId(), Nickname: "Captain", AvatarUpdateAt: 12, UpdateAt: 34}
	pref, err := profile.ToPreference()
	require.NoError(t, err)
	assert.Equal(t, PreferenceCategoryCommunityProfile, pref.Category)
	assert.Equal(t, profile.TeamId, pref.Name)
	assert.Equal(t, profile.UserId, pref.UserId)
	require.Nil(t, pref.IsValid())
	assert.Equal(t, profile, CommunityProfileFromPreference(pref))

	assert.Nil(t, CommunityProfileFromPreference(&Preference{Category: PreferenceCategoryTheme, Value: "{}"}))
	assert.Nil(t, CommunityProfileFromPreference(&Preference{Category: PreferenceCategoryCommunityProfile, Value: "not json"}))

	bad := *pref
	bad.Value = `{"nickname":"` + strings.Repeat("n", CommunityNicknameMaxRunes+1) + `"}`
	assert.NotNil(t, bad.IsValid())
	bad = *pref
	bad.Name = "general"
	assert.NotNil(t, bad.IsValid(), "named after a community's id")

	assert.True(t, (&CommunityProfile{UpdateAt: 1}).IsEmpty())
	assert.False(t, (&CommunityProfile{AvatarUpdateAt: 1}).IsEmpty())
}

func TestTeamNameDisplay(t *testing.T) {
	o := Team{DisplayName: "test", Name: "zzzzz", Type: TeamOpen}
	o.PreSave()
	require.Nil(t, o.IsValid())
	assert.Empty(t, o.NameDisplay, "the server's default")
	for _, display := range []string{ShowUsername, ShowNicknameFullName, ShowFullName, ""} {
		o.Patch(&TeamPatch{NameDisplay: NewPointer(display)})
		assert.Equal(t, display, o.NameDisplay)
		assert.Nil(t, o.IsValid(), display)
	}
	o.Patch(&TeamPatch{Description: NewPointer("about")})
	assert.Equal(t, "", o.NameDisplay, "patching something else keeps it")
	o.NameDisplay = "first_name"
	appErr := o.IsValid()
	require.NotNil(t, appErr)
	assert.Equal(t, "model.team.is_valid.name_display.app_error", appErr.Id)
}
