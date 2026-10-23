// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"encoding/json"
	"slices"
)

// Profile privacy (Antimatter): which details of their profile someone shows to the people of each community. It's
// a user prop (UserPropsKeyProfilePrivacy), so that the profile carries it wherever the server sends it out: a JSON
// object naming the details hidden for all communities (ProfilePrivacyDefault) and, where they chose otherwise, in
// a community (by its id). A community without an entry follows the default; an empty list there shows everything.
//
// Someone who shares several communities with you sees a detail only when none of them hides it: the most restrictive
// of the communities in common wins, so that joining an open community never shows anyone more of you. Someone who
// shares none with you sees what the default shows. You, and system admins (the System Console needs the profiles),
// see everything; nobody else is told what you hide, the prop itself included.
const (
	UserPropsKeyProfilePrivacy = "profile_privacy"
	ProfilePrivacyDefault      = "default"

	ProfileFieldFullName   = "full_name"
	ProfileFieldNickname   = "nickname"
	ProfileFieldPosition   = "position"
	ProfileFieldEmail      = "email"
	ProfileFieldLocalTime  = "local_time"
	ProfileFieldAttributes = "attributes"

	// ProfilePrivacyMaxEntries bounds the prop: the default and one entry per community.
	ProfilePrivacyMaxEntries = 501
)

// ProfileFields are the details someone can hide, in the order the apps list them.
var ProfileFields = []string{
	ProfileFieldFullName,
	ProfileFieldNickname,
	ProfileFieldPosition,
	ProfileFieldEmail,
	ProfileFieldLocalTime,
	ProfileFieldAttributes,
}

// ProfilePrivacy maps ProfilePrivacyDefault or a community's id to the details hidden there.
type ProfilePrivacy map[string][]string

func IsValidProfileField(field string) bool {
	return slices.Contains(ProfileFields, field)
}

// ParseProfilePrivacy reads the prop. ok is false when it can't be read or names something unknown: the caller then
// hides everything (fail closed).
func ParseProfilePrivacy(raw string) (privacy ProfilePrivacy, ok bool) {
	if raw == "" {
		return nil, true
	}
	if err := json.Unmarshal([]byte(raw), &privacy); err != nil {
		return nil, false
	}
	return privacy, privacy.IsValid()
}

// IsValid tells whether every entry is the default or a community id, and lists known details only.
func (p ProfilePrivacy) IsValid() bool {
	if len(p) > ProfilePrivacyMaxEntries {
		return false
	}
	for key, fields := range p {
		if key != ProfilePrivacyDefault && !IsValidId(key) {
			return false
		}
		if len(fields) > len(ProfileFields) {
			return false
		}
		for _, field := range fields {
			if !IsValidProfileField(field) {
				return false
			}
		}
	}
	return true
}

// HiddenFrom gives the details hidden from someone who shares the communities shared with the profile's owner:
// those any of them hides, each community following the default when it has no entry; with none in common, those the
// default hides. nil when nothing is hidden.
func (p ProfilePrivacy) HiddenFrom(shared []string) map[string]bool {
	if len(p) == 0 {
		return nil
	}
	var hidden map[string]bool
	add := func(fields []string) {
		for _, field := range fields {
			if hidden == nil {
				hidden = make(map[string]bool, len(ProfileFields))
			}
			hidden[field] = true
		}
	}
	if len(shared) == 0 {
		add(p[ProfilePrivacyDefault])
		return hidden
	}
	for _, teamID := range shared {
		fields, ok := p[teamID]
		if !ok {
			fields = p[ProfilePrivacyDefault]
		}
		add(fields)
	}
	return hidden
}

// AllHidden gives every detail hidden somewhere: what someone may be denied at most.
func (p ProfilePrivacy) AllHidden() map[string]bool {
	var hidden map[string]bool
	for _, fields := range p {
		for _, field := range fields {
			if hidden == nil {
				hidden = make(map[string]bool, len(ProfileFields))
			}
			hidden[field] = true
		}
	}
	return hidden
}

// AllProfileFields hides every detail, for a prop that can't be read.
func AllProfileFields() map[string]bool {
	hidden := make(map[string]bool, len(ProfileFields))
	for _, field := range ProfileFields {
		hidden[field] = true
	}
	return hidden
}

// HasProfilePrivacy tells whether the user hides anything anywhere (or has a prop that can't be read).
func (u *User) HasProfilePrivacy() bool {
	return u.Props[UserPropsKeyProfilePrivacy] != ""
}

// GetProfilePrivacy reads the user's profile privacy; ok is false when it can't be read.
func (u *User) GetProfilePrivacy() (ProfilePrivacy, bool) {
	return ParseProfilePrivacy(u.Props[UserPropsKeyProfilePrivacy])
}

// ProfileFieldsHiddenFrom gives the details of the user hidden from someone who shares the communities shared with
// them (see ProfilePrivacy.HiddenFrom): all of them when the prop can't be read.
func (u *User) ProfileFieldsHiddenFrom(shared []string) map[string]bool {
	if !u.HasProfilePrivacy() {
		return nil
	}
	privacy, ok := u.GetProfilePrivacy()
	if !ok {
		return AllProfileFields()
	}
	return privacy.HiddenFrom(shared)
}

// HideProfileFields blanks the details hidden, and the profile privacy prop itself, for someone else than the user.
// The attributes are served apart (see the attribute routes).
func (u *User) HideProfileFields(hidden map[string]bool) {
	if u.Props != nil {
		if _, ok := u.Props[UserPropsKeyProfilePrivacy]; ok {
			u.Props = CopyStringMap(u.Props)
			delete(u.Props, UserPropsKeyProfilePrivacy)
		}
	}
	if hidden[ProfileFieldFullName] {
		u.FirstName = ""
		u.LastName = ""
	}
	if hidden[ProfileFieldNickname] {
		u.Nickname = ""
	}
	if hidden[ProfileFieldPosition] {
		u.Position = ""
	}
	if hidden[ProfileFieldEmail] {
		u.Email = ""
		if u.Props != nil {
			if _, ok := u.Props[UserPropsKeyRemoteEmail]; ok {
				u.Props = CopyStringMap(u.Props)
				delete(u.Props, UserPropsKeyRemoteEmail)
			}
		}
	}
	if hidden[ProfileFieldLocalTime] && len(u.Timezone) > 0 {
		u.Timezone = StringMap{}
	}
}

// ProfileFieldsKey names a set of hidden details, for etags: the same profile reads differently to different people.
func ProfileFieldsKey(hidden map[string]bool) string {
	key := ""
	for i, field := range ProfileFields {
		if hidden[field] {
			key += string(rune('a' + i))
		}
	}
	return key
}
