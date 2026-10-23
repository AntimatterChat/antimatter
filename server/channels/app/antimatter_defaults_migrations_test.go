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
		require.NoError(t, th.App.SetPhase2PermissionsMigrationStatus(true))
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

func TestMaxUsersPerTeamUnlimitedMigration(t *testing.T) {
	setLimit := func(th *TestHelper, limit int) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.MaxUsersPerTeam = limit })
	}
	beforeUpgrade := func(t *testing.T, th *TestHelper, limit int) {
		t.Helper()
		_, err := th.Store.System().PermanentDeleteByName(maxUsersPerTeamUnlimitedMigrationKey)
		require.NoError(t, err)
		setLimit(th, limit)
	}

	t.Run("lifts the old default to no limit, once", func(t *testing.T) {
		th := Setup(t)
		beforeUpgrade(t, th, 50)
		require.NoError(t, th.Server.doMaxUsersPerTeamUnlimitedMigration())
		assert.Equal(t, 0, *th.App.Config().TeamSettings.MaxUsersPerTeam)
		_, err := th.Store.System().GetByName(maxUsersPerTeamUnlimitedMigrationKey)
		require.NoError(t, err)

		setLimit(th, 50)
		require.NoError(t, th.Server.doMaxUsersPerTeamUnlimitedMigration())
		assert.Equal(t, 50, *th.App.Config().TeamSettings.MaxUsersPerTeam, "an admin who sets 50 afterwards keeps it")
	})

	t.Run("keeps another limit", func(t *testing.T) {
		th := Setup(t)
		beforeUpgrade(t, th, 20)
		require.NoError(t, th.Server.doMaxUsersPerTeamUnlimitedMigration())
		assert.Equal(t, 20, *th.App.Config().TeamSettings.MaxUsersPerTeam)
	})

	t.Run("keeps a limit set in the environment", func(t *testing.T) {
		th := Setup(t)
		beforeUpgrade(t, th, 50)
		t.Setenv("AM_TEAMSETTINGS_MAXUSERSPERTEAM", "50")
		require.NoError(t, th.Server.doMaxUsersPerTeamUnlimitedMigration())
		assert.Equal(t, 50, *th.App.Config().TeamSettings.MaxUsersPerTeam)
	})

	t.Run("the default is no limit", func(t *testing.T) {
		cfg := &model.Config{}
		cfg.SetDefaults()
		assert.Equal(t, 0, *cfg.TeamSettings.MaxUsersPerTeam)
	})
}

func teamKindRow(t *testing.T, th *TestHelper, teamID string) string {
	t.Helper()
	row, err := th.Store.System().GetByName(model.TeamKindSystemKeyPrefix + teamID)
	if err != nil {
		return ""
	}
	return row.Value
}

func TestCommunityKinds(t *testing.T) {
	setDefault := func(th *TestHelper, kind string) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.DefaultCommunityKind = kind })
	}

	t.Run("a new community gets the setting's kind, unless created with one, and its row", func(t *testing.T) {
		th := Setup(t)
		team := th.CreateTeam(t)
		assert.Equal(t, model.TeamKindGuild, team.Kind)
		assert.Equal(t, model.TeamKindStoredGuild, teamKindRow(t, th, team.Id), "the row is written for the default kind too, under its former name")

		setDefault(th, model.TeamKindWorkspace)
		team = th.CreateTeam(t)
		assert.Equal(t, model.TeamKindWorkspace, team.Kind)
		assert.Equal(t, model.TeamKindStoredWorkspace, teamKindRow(t, th, team.Id))

		chosen, appErr := th.App.CreateTeam(th.Context, &model.Team{DisplayName: "Chosen", Name: "c" + model.NewId(), Email: "c@example.com", Type: model.TeamOpen, Kind: model.TeamKindGuild})
		require.Nil(t, appErr)
		assert.Equal(t, model.TeamKindGuild, chosen.Kind, "a preset's kind wins over the setting")
		assert.Equal(t, model.TeamKindStoredGuild, teamKindRow(t, th, chosen.Id))

		// Saving a team without a kind keeps the one it has.
		team.Kind = ""
		_, err := th.Store.Team().Update(team)
		require.NoError(t, err)
		assert.Equal(t, model.TeamKindStoredWorkspace, teamKindRow(t, th, team.Id))
	})

	t.Run("a community stored with a former name reads as its kind", func(t *testing.T) {
		th := Setup(t)
		team := th.CreateTeam(t)
		require.NoError(t, th.Store.System().SaveOrUpdate(&model.System{Name: model.TeamKindSystemKeyPrefix + team.Id, Value: "workplace"}))
		th.Store.Team().ClearCaches()
		fetched, appErr := th.App.GetTeam(team.Id)
		require.Nil(t, appErr)
		assert.Equal(t, model.TeamKindWorkspace, fetched.Kind)

		// A server configured with a former name gives new communities its kind.
		setDefault(th, "workplace")
		created, appErr := th.App.CreateTeam(th.Context, &model.Team{DisplayName: "Old setting", Name: "o" + model.NewId(), Email: "o@example.com", Type: model.TeamOpen})
		require.Nil(t, appErr)
		assert.Equal(t, model.TeamKindWorkspace, created.Kind)
	})

	// Before the upgrade, communities had no kind row.
	beforeUpgrade := func(t *testing.T, th *TestHelper, teams ...*model.Team) {
		t.Helper()
		for _, team := range teams {
			_, err := th.Store.System().PermanentDeleteByName(model.TeamKindSystemKeyPrefix + team.Id)
			require.NoError(t, err)
		}
		_, err := th.Store.System().PermanentDeleteByName(communityKindsMigrationKey)
		require.NoError(t, err)
	}

	for _, kind := range []string{model.TeamKindGuild, model.TeamKindWorkspace} {
		t.Run("on upgrade, communities without a kind get the setting's: "+kind, func(t *testing.T) {
			th := Setup(t)
			old, other := th.CreateTeam(t), th.CreateTeam(t)
			beforeUpgrade(t, th, old, other)
			setDefault(th, kind)

			require.NoError(t, th.Server.doCommunityKindsMigration())
			assert.Equal(t, model.TeamKindStoredValue(kind), teamKindRow(t, th, old.Id))
			assert.Equal(t, model.TeamKindStoredValue(kind), teamKindRow(t, th, other.Id))
			fetched, appErr := th.App.GetTeam(old.Id)
			require.Nil(t, appErr)
			assert.Equal(t, kind, fetched.Kind)
		})
	}

	t.Run("on upgrade, a kind already set is kept, and it runs once", func(t *testing.T) {
		th := Setup(t)
		set, unset := th.CreateTeam(t), th.CreateTeam(t)
		beforeUpgrade(t, th, unset)
		require.NoError(t, th.Store.System().SaveOrUpdate(&model.System{Name: model.TeamKindSystemKeyPrefix + set.Id, Value: model.TeamKindStoredWorkspace}))
		setDefault(th, model.TeamKindGuild)

		require.NoError(t, th.Server.doCommunityKindsMigration())
		assert.Equal(t, model.TeamKindStoredWorkspace, teamKindRow(t, th, set.Id), "not overwritten")
		assert.Equal(t, model.TeamKindStoredGuild, teamKindRow(t, th, unset.Id))
		_, err := th.Store.System().GetByName(communityKindsMigrationKey)
		require.NoError(t, err)

		later := th.CreateTeam(t)
		_, err = th.Store.System().PermanentDeleteByName(model.TeamKindSystemKeyPrefix + later.Id)
		require.NoError(t, err)
		setDefault(th, model.TeamKindWorkspace)
		require.NoError(t, th.Server.doCommunityKindsMigration())
		assert.Empty(t, teamKindRow(t, th, later.Id), "it ran once")
		assert.Equal(t, model.TeamKindStoredGuild, teamKindRow(t, th, unset.Id))
	})
}
