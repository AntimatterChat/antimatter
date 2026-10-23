// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package model

import (
	"encoding/json"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Community profiles (Antimatter, as Discord's server profiles): in one community, someone can go by a nickname and a
// picture of their own, shown instead of their global ones in that community's channels, member list, mentions and
// cards; direct and group messages keep the global profile. Refusing to be shown by name in a community is part of
// its privacy settings (which profile details its people see: full name and nickname hidden), not of the profile.
//
// A profile is kept as a preference of its owner (PreferenceCategoryCommunityProfile, named after the community's id,
// its value the profile as JSON), so a community's profiles are read at once by the preferences' name index; nobody
// writes that preference through the preferences API, only through the community profile API.
const (
	PreferenceCategoryCommunityProfile = "community_profile"

	// CommunityNicknameMaxRunes is the global nickname's limit.
	CommunityNicknameMaxRunes = UserNicknameMaxRunes
)

// CommunityProfile is someone's profile in one community. AvatarUpdateAt is when they set their community picture,
// 0 for none (their global picture shows).
type CommunityProfile struct {
	TeamId         string `json:"team_id"`
	UserId         string `json:"user_id"`
	Nickname       string `json:"nickname,omitempty"`
	AvatarUpdateAt int64  `json:"avatar_update_at,omitempty"`
	UpdateAt       int64  `json:"update_at"`
}

// CommunityProfilePatch is what the person changes of their own community profile: an empty nickname removes it.
type CommunityProfilePatch struct {
	Nickname *string `json:"nickname"`
}

// CommunityProfileReset takes a nickname and/or a picture off a community profile: the person themselves, or the
// community's moderators when they break its rules.
type CommunityProfileReset struct {
	Nickname bool `json:"nickname"`
	Avatar   bool `json:"avatar"`
}

// communityProfileValue is how a profile is stored, as its preference's value.
type communityProfileValue struct {
	Nickname       string `json:"nickname,omitempty"`
	AvatarUpdateAt int64  `json:"avatar_update_at,omitempty"`
	UpdateAt       int64  `json:"update_at"`
}

// IsEmpty tells whether the profile changes nothing: it's then not kept.
func (p *CommunityProfile) IsEmpty() bool {
	return p.Nickname == "" && p.AvatarUpdateAt == 0
}

// NormalizeCommunityNickname trims a community nickname and drops its control characters.
func NormalizeCommunityNickname(nickname string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, SanitizeUnicode(nickname)))
}

func (p *CommunityProfile) IsValid() *AppError {
	if !IsValidId(p.TeamId) || !IsValidId(p.UserId) {
		return NewAppError("CommunityProfile.IsValid", "model.community_profile.is_valid.id.app_error", nil, "", http.StatusBadRequest)
	}
	if utf8.RuneCountInString(p.Nickname) > CommunityNicknameMaxRunes || p.Nickname != NormalizeCommunityNickname(p.Nickname) {
		return NewAppError("CommunityProfile.IsValid", "model.community_profile.is_valid.nickname.app_error", map[string]any{"Max": CommunityNicknameMaxRunes}, "", http.StatusBadRequest)
	}
	if p.AvatarUpdateAt < 0 {
		return NewAppError("CommunityProfile.IsValid", "model.community_profile.is_valid.avatar.app_error", nil, "", http.StatusBadRequest)
	}
	return nil
}

// ToPreference gives the preference that keeps the profile.
func (p *CommunityProfile) ToPreference() (*Preference, error) {
	value, err := json.Marshal(communityProfileValue{Nickname: p.Nickname, AvatarUpdateAt: p.AvatarUpdateAt, UpdateAt: p.UpdateAt})
	if err != nil {
		return nil, err
	}
	return &Preference{UserId: p.UserId, Category: PreferenceCategoryCommunityProfile, Name: p.TeamId, Value: string(value)}, nil
}

// CommunityProfileFromPreference reads a profile back from its preference; nil if it isn't one.
func CommunityProfileFromPreference(pref *Preference) *CommunityProfile {
	if pref == nil || pref.Category != PreferenceCategoryCommunityProfile {
		return nil
	}
	var value communityProfileValue
	if err := json.Unmarshal([]byte(pref.Value), &value); err != nil {
		return nil
	}
	return &CommunityProfile{
		TeamId:         pref.Name,
		UserId:         pref.UserId,
		Nickname:       value.Nickname,
		AvatarUpdateAt: value.AvatarUpdateAt,
		UpdateAt:       value.UpdateAt,
	}
}

// isValidCommunityProfilePreference checks a community profile preference (Preference.IsValid).
func isValidCommunityProfilePreference(pref *Preference) bool {
	if !IsValidId(pref.Name) {
		return false
	}
	profile := CommunityProfileFromPreference(pref)
	return profile != nil && profile.IsValid() == nil
}
