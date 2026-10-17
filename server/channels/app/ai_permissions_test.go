// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/store/sqlstore"
	"github.com/mattermost/mattermost/server/v8/channels/store/storetest/mocks"
)

// Antimatter: on upgrade, the system admins get the management of every model connection, and the community's
// admins (team_admin, and the admin role of team schemes) their community's; nobody else gets anything.
func TestAntimatterAIConnectionPermissionsMigration(t *testing.T) {
	mainHelper.Parallel(t)

	th := SetupWithStoreMock(t)
	migrationMap, err := th.App.getAntimatterAIConnectionPermissionsMigration()
	require.NoError(t, err)

	systemAdmin := &model.Role{Name: model.SystemAdminRoleId, Permissions: []string{model.PermissionManageSystem.Id}}
	teamAdmin := &model.Role{Name: model.TeamAdminRoleId, Permissions: []string{model.PermissionManageTeam.Id}}
	schemeTeamAdmin := &model.Role{Name: model.NewId(), DisplayName: sqlstore.SchemeRoleDisplayNameTeamAdmin, SchemeManaged: true, Permissions: []string{model.PermissionManageTeam.Id}}
	systemUser := &model.Role{Name: model.SystemUserRoleId, Permissions: []string{model.PermissionCreateTeam.Id}}
	teamUser := &model.Role{Name: model.TeamUserRoleId, Permissions: []string{model.PermissionViewTeam.Id}}
	channelAdmin := &model.Role{Name: model.ChannelAdminRoleId, Permissions: []string{model.PermissionManageChannelRoles.Id}}
	roles := []*model.Role{systemAdmin, teamAdmin, schemeTeamAdmin, systemUser, teamUser, channelAdmin}

	mockStore := th.App.Srv().Store().(*mocks.Store)
	roleStore := mocks.RoleStore{}
	systemStore := mocks.SystemStore{}
	mockStore.On("Role").Return(&roleStore)
	mockStore.On("System").Return(&systemStore)
	systemStore.On("GetByName", model.MigrationKeyAntimatterAIConnectionPermissions).
		Return(nil, model.NewAppError("test", "missing", nil, "", 404)).Once()
	systemStore.On("SaveOrUpdate", mock.MatchedBy(func(system *model.System) bool {
		return system.Name == model.MigrationKeyAntimatterAIConnectionPermissions && system.Value == "true"
	})).Return(nil).Once()
	roleStore.On("SavePreservingUnknownPermissions", mock.AnythingOfType("*model.Role")).
		Return(func(role *model.Role) *model.Role { return role }, nil)

	require.Nil(t, th.App.Srv().doPermissionsMigration(model.MigrationKeyAntimatterAIConnectionPermissions, migrationMap, roles))

	assert.ElementsMatch(t, []string{model.PermissionManageSystem.Id, model.PermissionManageAIConnections.Id, model.PermissionManageTeamAIConnections.Id}, systemAdmin.Permissions)
	assert.ElementsMatch(t, []string{model.PermissionManageTeam.Id, model.PermissionManageTeamAIConnections.Id}, teamAdmin.Permissions)
	assert.ElementsMatch(t, []string{model.PermissionManageTeam.Id, model.PermissionManageTeamAIConnections.Id}, schemeTeamAdmin.Permissions, "a team scheme's admins too")
	assert.ElementsMatch(t, []string{model.PermissionCreateTeam.Id}, systemUser.Permissions, "members get nothing")
	assert.ElementsMatch(t, []string{model.PermissionViewTeam.Id}, teamUser.Permissions)
	assert.ElementsMatch(t, []string{model.PermissionManageChannelRoles.Id}, channelAdmin.Permissions)
}

// The fresh install's defaults are the ones the migration brings existing servers to.
func TestAIConnectionPermissionsDefaults(t *testing.T) {
	roles := model.MakeDefaultRoles()
	assert.Contains(t, roles[model.SystemAdminRoleId].Permissions, model.PermissionManageAIConnections.Id)
	assert.Contains(t, roles[model.SystemAdminRoleId].Permissions, model.PermissionManageTeamAIConnections.Id)
	assert.Contains(t, roles[model.TeamAdminRoleId].Permissions, model.PermissionManageTeamAIConnections.Id)
	for name, role := range roles {
		if name == model.SystemAdminRoleId {
			continue
		}
		assert.NotContains(t, role.Permissions, model.PermissionManageAIConnections.Id, name)
		if name != model.TeamAdminRoleId {
			assert.NotContains(t, role.Permissions, model.PermissionManageTeamAIConnections.Id, name)
		}
	}
}
