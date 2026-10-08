// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyChannelOverrides(t *testing.T) {
	post := PermissionCreatePost.Id
	mods, muted, me := NewId(), NewId(), NewId()
	override := func(targetType, id string, allow, deny []string) *ChannelOverride {
		return &ChannelOverride{TargetType: targetType, TargetId: id, Allow: allow, Deny: deny}
	}

	t.Run("no override keeps the answer", func(t *testing.T) {
		assert.True(t, ApplyChannelOverrides(true, post, []string{mods}, me, nil))
		assert.False(t, ApplyChannelOverrides(false, post, []string{mods}, me, nil))
	})

	t.Run("a role's deny takes a permission away", func(t *testing.T) {
		overrides := []*ChannelOverride{override(ChannelOverrideTargetRole, muted, nil, []string{post})}
		assert.False(t, ApplyChannelOverrides(true, post, []string{muted}, me, overrides))
		assert.True(t, ApplyChannelOverrides(true, post, []string{mods}, me, overrides), "only for the role's members")
	})

	t.Run("among roles, an allow beats a deny", func(t *testing.T) {
		overrides := []*ChannelOverride{
			override(ChannelOverrideTargetRole, muted, nil, []string{post}),
			override(ChannelOverrideTargetRole, mods, []string{post}, nil),
		}
		assert.True(t, ApplyChannelOverrides(true, post, []string{muted, mods}, me, overrides))
		assert.True(t, ApplyChannelOverrides(false, post, []string{mods}, me, overrides))
	})

	t.Run("a person's own override comes last", func(t *testing.T) {
		overrides := []*ChannelOverride{
			override(ChannelOverrideTargetRole, mods, []string{post}, nil),
			override(ChannelOverrideTargetUser, me, nil, []string{post}),
		}
		assert.False(t, ApplyChannelOverrides(true, post, []string{mods}, me, overrides))
		overrides[1] = override(ChannelOverrideTargetUser, me, []string{post}, nil)
		assert.True(t, ApplyChannelOverrides(false, post, nil, me, overrides))
	})

	t.Run("other permissions are left alone", func(t *testing.T) {
		overrides := []*ChannelOverride{override(ChannelOverrideTargetRole, muted, nil, []string{post})}
		assert.True(t, ApplyChannelOverrides(true, PermissionAddReaction.Id, []string{muted}, me, overrides))
	})
}

func TestChannelOverrideIsValid(t *testing.T) {
	valid := func() *ChannelOverride {
		return &ChannelOverride{ChannelId: NewId(), TargetType: ChannelOverrideTargetRole, TargetId: NewId(), Deny: []string{PermissionCreatePost.Id}}
	}
	assert.Nil(t, valid().IsValid())

	o := valid()
	o.TargetType = "team"
	assert.Equal(t, "model.channel_override.target_type.app_error", o.IsValid().Id)

	o = valid()
	o.Allow = []string{PermissionManageSystem.Id}
	assert.Equal(t, "model.channel_override.permission.app_error", o.IsValid().Id)

	o = valid()
	o.Allow = []string{PermissionCreatePost.Id}
	assert.Equal(t, "model.channel_override.both.app_error", o.IsValid().Id)
}

func TestTeamRoleGroupIDs(t *testing.T) {
	a, b := NewId(), NewId()
	assert.Equal(t, []string{a, b}, TeamRoleGroupIDs("team_user "+TeamRoleName(a)+" team_admin "+TeamRoleName(b)))
	assert.Empty(t, TeamRoleGroupIDs("team_user"))
}
