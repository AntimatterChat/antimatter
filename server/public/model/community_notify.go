// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"encoding/json"
	"unicode/utf8"
)

// Notification settings per community (Antimatter): someone's notification settings can differ in one of their
// communities. They're a preference, PreferenceCategoryCommunityNotify named after the community's id, whose value
// is a JSON object of the notify props it sets over the person's own (CommunityNotifyKeys). The server applies them
// to push and email notifications of the community's channels, and the apps to desktop ones; a channel's own
// settings still win over them, as over the person's own; direct and group messages, in no community, follow the
// person's own settings.
const (
	PreferenceCategoryCommunityNotify = "community_notify"

	// DesktopNotificationSoundNotifyProp is the sound of desktop notifications (the apps play it).
	DesktopNotificationSoundNotifyProp = "desktop_notification_sound"

	communityNotifySoundMaxRunes = 64
)

// CommunityNotifyKeys are the notify props a community can set, each with the values it takes (nil: any short name).
var CommunityNotifyKeys = map[string][]string{
	DesktopNotifyProp:                  {UserNotifyAll, UserNotifyMention, UserNotifyNone},
	DesktopThreadsNotifyProp:           {UserNotifyAll, UserNotifyMention},
	DesktopSoundNotifyProp:             {"true", "false"},
	DesktopNotificationSoundNotifyProp: nil,
	PushNotifyProp:                     {UserNotifyAll, UserNotifyMention, UserNotifyNone},
	PushThreadsNotifyProp:              {UserNotifyAll, UserNotifyMention},
	EmailNotifyProp:                    {"true", "false"},
	ChannelMentionsNotifyProp:          {"true", "false"},
}

// ParseCommunityNotifyProps reads a community's notification settings; ok is false when they can't be read or set
// something unknown.
func ParseCommunityNotifyProps(raw string) (props StringMap, ok bool) {
	if err := json.Unmarshal([]byte(raw), &props); err != nil || props == nil {
		return nil, false
	}
	for key, value := range props {
		values, known := CommunityNotifyKeys[key]
		if !known {
			return nil, false
		}
		if values == nil {
			if value == "" || utf8.RuneCountInString(value) > communityNotifySoundMaxRunes {
				return nil, false
			}
			continue
		}
		valid := false
		for _, v := range values {
			valid = valid || v == value
		}
		if !valid {
			return nil, false
		}
	}
	return props, true
}

func IsValidCommunityNotifyProps(raw string) bool {
	_, ok := ParseCommunityNotifyProps(raw)
	return ok
}

// WithCommunityNotifyProps gives a copy of the user whose notify props are theirs with the community's over them.
func (u *User) WithCommunityNotifyProps(props StringMap) *User {
	if len(props) == 0 {
		return u
	}
	copied := u.DeepCopy()
	if copied.NotifyProps == nil {
		copied.NotifyProps = StringMap{}
	}
	for key, value := range props {
		copied.NotifyProps[key] = value
	}
	return copied
}
