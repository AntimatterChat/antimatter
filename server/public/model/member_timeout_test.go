// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMemberTimeout(t *testing.T) {
	now := GetMillis()
	valid := func() *MemberTimeout {
		return &MemberTimeout{TeamId: NewId(), UserId: NewId(), Until: now + time.Hour.Milliseconds()}
	}

	t.Run("holds until its time", func(t *testing.T) {
		timeout := valid()
		assert.True(t, timeout.IsActive(now))
		assert.False(t, timeout.IsActive(timeout.Until))
		assert.False(t, (*MemberTimeout)(nil).IsActive(now))
	})

	t.Run("is in the future, up to 28 days", func(t *testing.T) {
		assert.Nil(t, valid().IsValid(now))
		past := valid()
		past.Until = now - 1
		assert.NotNil(t, past.IsValid(now))
		long := valid()
		long.Until = now + MemberTimeoutMaxDuration.Milliseconds() + 1
		assert.NotNil(t, long.IsValid(now))
	})

	t.Run("has a bounded reason", func(t *testing.T) {
		timeout := valid()
		timeout.Reason = strings.Repeat("é", MemberTimeoutReasonMaxRunes)
		assert.Nil(t, timeout.IsValid(now))
		timeout.Reason += "!"
		assert.NotNil(t, timeout.IsValid(now))
	})

	t.Run("takes away speaking, nothing else", func(t *testing.T) {
		assert.Contains(t, MemberTimeoutPermissions(), PermissionCreatePost.Id)
		assert.NotContains(t, MemberTimeoutPermissions(), PermissionReadChannel.Id)
	})
}
