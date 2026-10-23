// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommunityNotifyProps(t *testing.T) {
	teamID := NewId()
	pref := func(value string) *Preference {
		return &Preference{UserId: NewId(), Category: PreferenceCategoryCommunityNotify, Name: teamID, Value: value}
	}

	t.Run("a community sets some notification settings over the person's own", func(t *testing.T) {
		assert.Nil(t, pref(`{"desktop":"all","push":"none","email":"false","channel":"false","desktop_sound":"true","desktop_notification_sound":"Crackle","desktop_threads":"mention","push_threads":"all"}`).IsValid())
		assert.Nil(t, pref(`{}`).IsValid())
	})

	t.Run("nothing else", func(t *testing.T) {
		for _, value := range []string{`{"desktop":"loud"}`, `{"auto_responder_active":"true"}`, `{"mention_keys":"x"}`, `{"desktop_notification_sound":""}`, `[]`, `nope`} {
			appErr := pref(value).IsValid()
			require.NotNil(t, appErr, value)
			assert.Equal(t, "model.preference.is_valid.community_notify.app_error", appErr.Id)
		}
		p := pref(`{"desktop":"all"}`)
		p.Name = "default"
		assert.NotNil(t, p.IsValid(), "named after a community")
	})

	t.Run("they apply to a copy of the user", func(t *testing.T) {
		user := &User{Id: NewId(), NotifyProps: StringMap{DesktopNotifyProp: UserNotifyMention, PushNotifyProp: UserNotifyMention}}
		seen := user.WithCommunityNotifyProps(StringMap{PushNotifyProp: UserNotifyNone})
		assert.Equal(t, UserNotifyNone, seen.NotifyProps[PushNotifyProp])
		assert.Equal(t, UserNotifyMention, seen.NotifyProps[DesktopNotifyProp])
		assert.Equal(t, UserNotifyMention, user.NotifyProps[PushNotifyProp])
		assert.Same(t, user, user.WithCommunityNotifyProps(nil))
	})
}
