// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMemberBan(t *testing.T) {
	now := GetMillis()
	hour := time.Hour.Milliseconds()
	valid := func() *MemberBan {
		return &MemberBan{TeamId: NewId(), UserId: NewId(), CreatedBy: NewId()}
	}

	t.Run("holds until it's lifted, or until its end", func(t *testing.T) {
		ban := valid()
		assert.True(t, ban.IsActive(now), "no end")
		assert.True(t, ban.IsActive(now+100*24*hour))
		ban.ExpireAt = now + hour
		assert.True(t, ban.IsActive(now))
		assert.False(t, ban.IsActive(ban.ExpireAt))
		assert.False(t, (*MemberBan)(nil).IsActive(now))
	})

	t.Run("ends in the future, within a year, or never", func(t *testing.T) {
		assert.Nil(t, valid().IsValid(now))
		ending := valid()
		ending.ExpireAt = now + hour
		assert.Nil(t, ending.IsValid(now))
		past := valid()
		past.ExpireAt = now - 1
		assert.NotNil(t, past.IsValid(now))
		far := valid()
		far.ExpireAt = now + MemberBanMaxDuration.Milliseconds() + hour
		assert.NotNil(t, far.IsValid(now))
	})

	t.Run("names a community and a person, with a short reason", func(t *testing.T) {
		noTeam := valid()
		noTeam.TeamId = ""
		assert.NotNil(t, noTeam.IsValid(now))
		noUser := valid()
		noUser.UserId = "nope"
		assert.NotNil(t, noUser.IsValid(now))
		long := valid()
		long.Reason = strings.Repeat("é", MemberBanReasonMaxRunes+1)
		assert.NotNil(t, long.IsValid(now))
		long.Reason = strings.Repeat("é", MemberBanReasonMaxRunes)
		assert.Nil(t, long.IsValid(now), "runes, not bytes")
	})

	t.Run("deletes messages from up to 7 days back", func(t *testing.T) {
		assert.Nil(t, (&MemberBanRequest{}).IsValid())
		assert.Nil(t, (&MemberBanRequest{DeleteMessagesSeconds: 7 * 24 * 3600}).IsValid())
		assert.NotNil(t, (&MemberBanRequest{DeleteMessagesSeconds: 7*24*3600 + 1}).IsValid())
		assert.NotNil(t, (&MemberBanRequest{DeleteMessagesSeconds: -1}).IsValid())
	})
}

func TestTeamInviteLink(t *testing.T) {
	now := GetMillis()
	hour := time.Hour.Milliseconds()
	valid := func() *TeamInviteLink {
		return &TeamInviteLink{Id: NewId(), TeamId: NewId(), CreatorId: NewId(), CreateAt: now}
	}

	t.Run("ends at its time, or never", func(t *testing.T) {
		link := valid()
		assert.False(t, link.IsExpired(now+100*24*hour))
		link.ExpireAt = now + hour
		assert.False(t, link.IsExpired(now))
		assert.True(t, link.IsExpired(link.ExpireAt))
	})

	t.Run("is used up after its uses, or never without a limit", func(t *testing.T) {
		link := valid()
		link.Uses = 1000
		assert.False(t, link.IsUsedUp())
		link.MaxUses = 2
		link.Uses = 1
		assert.False(t, link.IsUsedUp())
		link.Uses = 2
		assert.True(t, link.IsUsedUp())
	})

	t.Run("ends in the future within a year, and allows up to its maximum of uses", func(t *testing.T) {
		assert.Nil(t, valid().IsValid(now))
		past := valid()
		past.ExpireAt = now
		assert.NotNil(t, past.IsValid(now))
		far := valid()
		far.ExpireAt = now + TeamInviteLinkMaxDuration.Milliseconds() + hour
		assert.NotNil(t, far.IsValid(now))
		negative := valid()
		negative.MaxUses = -1
		assert.NotNil(t, negative.IsValid(now))
		many := valid()
		many.MaxUses = TeamInviteLinkMaxUses + 1
		assert.NotNil(t, many.IsValid(now))
		noCreator := valid()
		noCreator.CreatorId = ""
		assert.NotNil(t, noCreator.IsValid(now))
	})
}
