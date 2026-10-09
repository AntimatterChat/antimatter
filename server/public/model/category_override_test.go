// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCategoryOverride(t *testing.T) {
	teamID := NewId()
	valid := func() *CategoryOverride {
		return &CategoryOverride{TeamId: teamID, Category: "community", TargetType: ChannelOverrideTargetRole, TargetId: NewId(), Deny: []string{PermissionCreatePost.Id}}
	}

	t.Run("is valid as a channel's would be", func(t *testing.T) {
		assert.Nil(t, valid().IsValid())
		o := valid()
		o.Category = ""
		assert.NotNil(t, o.IsValid())
		o = valid()
		o.Allow = []string{PermissionManageSystem.Id}
		assert.NotNil(t, o.IsValid())
		o = valid()
		o.Allow = o.Deny
		assert.NotNil(t, o.IsValid(), "not both allowed and denied")
	})

	t.Run("names its category as names compare", func(t *testing.T) {
		assert.Equal(t, "open space", CategoryOverrideKey("  Open Space "))
	})

	t.Run("a channel inherits its category's overrides, its own winning permission by permission", func(t *testing.T) {
		mods, muted, ada := NewId(), NewId(), NewId()
		category := []*CategoryOverride{
			{TargetType: ChannelOverrideTargetRole, TargetId: mods, Allow: []string{PermissionDeleteOthersPosts.Id}},
			{TargetType: ChannelOverrideTargetRole, TargetId: muted, Deny: []string{PermissionCreatePost.Id, PermissionAddReaction.Id}},
		}
		own := []*ChannelOverride{
			{ChannelId: "ch", TargetType: ChannelOverrideTargetRole, TargetId: muted, Allow: []string{PermissionCreatePost.Id}},
			{ChannelId: "ch", TargetType: ChannelOverrideTargetUser, TargetId: ada, Deny: []string{PermissionAddReaction.Id}},
		}

		effective := EffectiveChannelOverrides("ch", category, own)
		assert.Len(t, effective, 3)
		assert.Equal(t, &ChannelOverride{ChannelId: "ch", TargetType: ChannelOverrideTargetRole, TargetId: mods, Allow: []string{PermissionDeleteOthersPosts.Id}}, effective[0])
		assert.True(t, ApplyChannelOverrides(false, PermissionDeleteOthersPosts.Id, []string{mods}, "", effective), "inherited")
		assert.True(t, ApplyChannelOverrides(true, PermissionCreatePost.Id, []string{muted}, "", effective), "the channel's allow wins")
		assert.False(t, ApplyChannelOverrides(true, PermissionAddReaction.Id, []string{muted}, "", effective), "the category's other deny stands")
		assert.False(t, ApplyChannelOverrides(true, PermissionAddReaction.Id, nil, ada, effective))
		assert.Equal(t, []string{PermissionCreatePost.Id, PermissionAddReaction.Id}, category[1].Deny, "the category's own lists are left alone")
	})
}
