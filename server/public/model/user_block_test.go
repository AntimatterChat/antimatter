// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelMemberSanitizeHidesMessageRequests(t *testing.T) {
	props := StringMap{DesktopNotifyProp: ChannelNotifyAll, ChannelMemberDMRequestProp: DMRequestPending, ChannelMemberDMRequestDismissalsProp: "2"}
	userID := NewId()

	t.Run("another member doesn't see whether it's a request", func(t *testing.T) {
		member := &ChannelMember{UserId: userID, NotifyProps: props}
		member.SanitizeForCurrentUser(NewId())
		assert.NotContains(t, member.NotifyProps, ChannelMemberDMRequestProp)
		assert.NotContains(t, member.NotifyProps, ChannelMemberDMRequestDismissalsProp)
		assert.Equal(t, ChannelNotifyAll, member.NotifyProps[DesktopNotifyProp])
		assert.Equal(t, DMRequestPending, props[ChannelMemberDMRequestProp], "the shared map is left alone")
	})

	t.Run("its member does", func(t *testing.T) {
		member := &ChannelMember{UserId: userID, NotifyProps: props}
		member.SanitizeForCurrentUser(userID)
		assert.Equal(t, DMRequestPending, member.NotifyProps[ChannelMemberDMRequestProp])
	})

	assert.True(t, IsDMRequestWaiting(StringMap{ChannelMemberDMRequestProp: DMRequestDismissed}))
	assert.False(t, IsDMRequestWaiting(StringMap{ChannelMemberDMRequestProp: DMRequestAccepted}))
	assert.False(t, IsDMRequestWaiting(nil))
}

func TestPreferenceIsValidUserBlock(t *testing.T) {
	userID := NewId()
	pref := func(category, name, value string) *Preference {
		return &Preference{UserId: userID, Category: category, Name: name, Value: value}
	}

	require.Nil(t, pref(PreferenceCategoryUserBlock, NewId(), UserBlockIgnore).IsValid())
	require.Nil(t, pref(PreferenceCategoryUserBlock, NewId(), UserBlockBlock).IsValid())
	require.NotNil(t, pref(PreferenceCategoryUserBlock, NewId(), "mute").IsValid())
	require.NotNil(t, pref(PreferenceCategoryUserBlock, "someone", UserBlockBlock).IsValid())
	require.NotNil(t, pref(PreferenceCategoryUserBlock, userID, UserBlockBlock).IsValid(), "not oneself")

	require.Nil(t, pref(PreferenceCategoryDMPrivacy, PreferenceNameDMPrivacyDefault, DMPrivacyAsk).IsValid())
	require.Nil(t, pref(PreferenceCategoryDMPrivacy, NewId(), DMPrivacyEveryone).IsValid())
	require.NotNil(t, pref(PreferenceCategoryDMPrivacy, NewId(), "friends").IsValid())
	require.NotNil(t, pref(PreferenceCategoryDMPrivacy, "a-team", DMPrivacyAsk).IsValid())
}
