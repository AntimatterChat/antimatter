// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTeamRolePermissions(t *testing.T) {
	permissions := TeamRolePermissions()

	t.Run("is set up by the time it's asked for", func(t *testing.T) {
		require.NotEmpty(t, permissions)
		for _, p := range permissions {
			require.NotEmpty(t, p)
		}
	})

	t.Run("lists each permission once", func(t *testing.T) {
		seen := map[string]bool{}
		for _, p := range permissions {
			assert.False(t, seen[p], "%s is listed twice", p)
			seen[p] = true
		}
	})

	t.Run("holds team and channel permissions only", func(t *testing.T) {
		scoped := map[string]bool{}
		for _, p := range AllPermissions {
			if p.Scope == PermissionScopeTeam || p.Scope == PermissionScopeChannel {
				scoped[p.Id] = true
			}
		}
		for _, p := range permissions {
			assert.True(t, scoped[p], "%s isn't a team or channel permission", p)
		}
	})

	t.Run("holds the channels' admins' community-level permissions, not the admins' own", func(t *testing.T) {
		for _, p := range []*Permission{
			PermissionManagePublicChannelBanner, PermissionManagePrivateChannelBanner, PermissionManageChannelJoinRequests,
			PermissionManagePrivateChannelDiscoverability, PermissionManagePublicChannelAutoTranslation, PermissionManagePrivateChannelAutoTranslation,
		} {
			assert.Contains(t, permissions, p.Id)
		}
		for _, p := range []*Permission{PermissionBypassIncomingWebhookChannelLock, PermissionManageChannelAccessRules, PermissionManageTeamAccessRules} {
			assert.NotContains(t, permissions, p.Id)
		}
	})

	t.Run("hands out nothing system-wide", func(t *testing.T) {
		for _, p := range []*Permission{PermissionManageSystem, PermissionManageRoles, PermissionCreateTeam, PermissionManageSharedChannels, PermissionEditOtherUsers, PermissionManageAIConnections} {
			assert.NotContains(t, permissions, p.Id)
		}
	})

	t.Run("holds the community's own model connections", func(t *testing.T) {
		assert.Contains(t, permissions, PermissionManageTeamAIConnections.Id)
	})
}

func TestAreTeamRolePermissions(t *testing.T) {
	assert.True(t, AreTeamRolePermissions(nil))
	assert.True(t, AreTeamRolePermissions([]string{PermissionDeleteOthersPosts.Id, PermissionManageChannelRoles.Id}))
	assert.False(t, AreTeamRolePermissions([]string{PermissionDeleteOthersPosts.Id, PermissionManageSystem.Id}))
	assert.False(t, AreTeamRolePermissions([]string{""}))
	assert.False(t, AreTeamRolePermissions([]string{"made_up"}))
}

func TestTeamRoleName(t *testing.T) {
	id := NewId()
	name := TeamRoleName(id)
	assert.Equal(t, "team_role_"+id, name)
	assert.LessOrEqual(t, len(name), RoleNameMaxLength)
	assert.True(t, IsValidRoleName(name))
}

func TestTeamRolePermissionsAreCurrent(t *testing.T) {
	for _, p := range DeprecatedPermissions {
		assert.NotContains(t, TeamRolePermissions(), p.Id, "%s is deprecated: the server no longer checks it", p.Id)
	}
}
