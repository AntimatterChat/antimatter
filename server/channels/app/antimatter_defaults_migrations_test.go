// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// teamRoleWith makes a role of a team granting permissions, as the role editor does.
func teamRoleWith(t *testing.T, th *TestHelper, teamID string, permissions []string) (*model.Group, *model.Role) {
	t.Helper()
	group, err := th.Store.Group().Create(&model.Group{Name: new("role" + model.NewId()[:8]), DisplayName: "Moderators", Source: model.GroupSourceCustom, AllowReference: true, TeamId: teamID})
	require.NoError(t, err)
	role, appErr := th.App.GetTeamRole(th.Context, group)
	require.Nil(t, appErr)
	role, appErr = th.App.PatchRole(role, &model.RolePatch{Permissions: &permissions})
	require.Nil(t, appErr)
	return group, role
}

func rolePermissions(t *testing.T, th *TestHelper, name string) []string {
	t.Helper()
	role, err := th.Store.Role().GetByName(th.Context, name)
	require.NoError(t, err)
	permissions := slices.Clone(role.Permissions)
	slices.Sort(permissions)
	return permissions
}

func sorted(permissions []string) []string {
	permissions = slices.Clone(permissions)
	slices.Sort(permissions)
	return permissions
}

func TestModeratorPresetMigration(t *testing.T) {
	th := Setup(t).InitBasic(t)

	_, preset := teamRoleWith(t, th, th.BasicTeam.Id, moderatorPresetPermissionsV1)
	edited := append(slices.Clone(moderatorPresetPermissionsV1), model.PermissionManageTeamRoles.Id)
	_, editedRole := teamRoleWith(t, th, th.BasicTeam.Id, edited)
	fewer := moderatorPresetPermissionsV1[:2]
	_, fewerRole := teamRoleWith(t, th, th.BasicTeam.Id, fewer)

	_, err := th.Store.System().PermanentDeleteByName(moderatorPresetMigrationKey)
	require.NoError(t, err)
	require.NoError(t, th.Server.doModeratorPresetMigration())

	assert.Equal(t, sorted(ModeratorPresetPermissions()), rolePermissions(t, th, preset.Name), "the preset's role gets Remove people")
	assert.Contains(t, ModeratorPresetPermissions(), model.PermissionRemoveUserFromTeam.Id)
	assert.Equal(t, sorted(edited), rolePermissions(t, th, editedRole.Name), "an edited role is left alone")
	assert.Equal(t, sorted(fewer), rolePermissions(t, th, fewerRole.Name), "an edited role is left alone")
	assert.Equal(t, sorted(model.MakeDefaultRoles()[model.ChannelUserRoleId].Permissions), rolePermissions(t, th, model.ChannelUserRoleId), "built-in roles are left alone")

	t.Run("runs once", func(t *testing.T) {
		_, later := teamRoleWith(t, th, th.BasicTeam.Id, moderatorPresetPermissionsV1)
		require.NoError(t, th.Server.doModeratorPresetMigration())
		assert.Equal(t, sorted(moderatorPresetPermissionsV1), rolePermissions(t, th, later.Name))
	})
}

func TestAdministratorRolesMigration(t *testing.T) {
	th := Setup(t).InitBasic(t)

	_, administrator := teamRoleWith(t, th, th.BasicTeam.Id, []string{model.PermissionManageTeam.Id})
	_, other := teamRoleWith(t, th, th.BasicTeam.Id, []string{model.PermissionDeleteOthersPosts.Id})

	_, err := th.Store.System().PermanentDeleteByName(administratorRolesMigrationKey)
	require.NoError(t, err)
	require.NoError(t, th.Server.doAdministratorRolesMigration())

	expected, appErr := th.App.TeamAdminRolePermissions(th.Context, th.BasicTeam.Id)
	require.Nil(t, appErr)
	got := rolePermissions(t, th, administrator.Name)
	assert.Equal(t, sorted(expected), got)
	for _, p := range []*model.Permission{model.PermissionManageTeam, model.PermissionRemoveUserFromTeam, model.PermissionManageTeamRoles, model.PermissionDeleteOthersPosts, model.PermissionManageChannelRoles, model.PermissionDeletePublicChannel} {
		assert.Contains(t, got, p.Id)
	}
	for _, p := range got {
		assert.Contains(t, model.TeamRolePermissions(), p, "nothing a role may not hold")
	}
	assert.Equal(t, []string{model.PermissionDeleteOthersPosts.Id}, rolePermissions(t, th, other.Name), "a role without Administrator is left alone")

	t.Run("runs once", func(t *testing.T) {
		_, later := teamRoleWith(t, th, th.BasicTeam.Id, []string{model.PermissionManageTeam.Id})
		require.NoError(t, th.Server.doAdministratorRolesMigration())
		assert.Equal(t, []string{model.PermissionManageTeam.Id}, rolePermissions(t, th, later.Name))
	})
}

func TestTeamAdminRolePermissions(t *testing.T) {
	th := Setup(t).InitBasic(t)

	permissions, appErr := th.App.TeamAdminRolePermissions(th.Context, th.BasicTeam.Id)
	require.Nil(t, appErr)
	teamAdmin := rolePermissions(t, th, model.TeamAdminRoleId)
	for _, p := range permissions {
		assert.Contains(t, model.TeamRolePermissions(), p)
		assert.Contains(t, teamAdmin, p)
	}
	for _, p := range teamAdmin {
		if slices.Contains(model.TeamRolePermissions(), p) {
			assert.Contains(t, permissions, p)
		}
	}

	t.Run("follows the team's scheme", func(t *testing.T) {
		scheme, appErr := th.App.CreateScheme(&model.Scheme{Name: model.NewId(), DisplayName: "Scheme", Scope: model.SchemeScopeTeam})
		require.Nil(t, appErr)
		_, appErr = th.App.PatchRole(mustRole(t, th, scheme.DefaultTeamAdminRole), &model.RolePatch{Permissions: &[]string{model.PermissionManageTeam.Id}})
		require.Nil(t, appErr)
		team := th.CreateTeam(t)
		team.SchemeId = &scheme.Id
		_, appErr = th.App.UpdateTeamScheme(team)
		require.Nil(t, appErr)

		permissions, appErr := th.App.TeamAdminRolePermissions(th.Context, team.Id)
		require.Nil(t, appErr)
		assert.Equal(t, []string{model.PermissionManageTeam.Id}, permissions)
	})
}

func mustRole(t *testing.T, th *TestHelper, name string) *model.Role {
	t.Helper()
	role, appErr := th.App.GetRoleByName(th.Context, name)
	require.Nil(t, appErr)
	return role
}
