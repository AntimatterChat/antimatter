// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// Antimatter's RBAC regression tests. They guard against an update that quietly hands out power: a migration that
// makes ordinary users admins, a default role that gains an admin permission, a guest who stops being one.

// The golden file of every role a fresh install creates, with its permissions. A change to it must be deliberate:
// regenerate it with
//
//	AM_UPDATE_GOLDEN=1 go test ./channels/app -run TestBuiltInRolePermissionsGolden
//
// then read the diff of testdata/builtin_role_permissions.json before committing it, and say in the commit why each
// role gains or loses what it does.
const builtInRolesGoldenFile = "builtin_role_permissions.json"

type goldenRole struct {
	SchemeManaged bool     `json:"scheme_managed"`
	BuiltIn       bool     `json:"built_in"`
	Permissions   []string `json:"permissions"`
}

// memberRoles are the roles ordinary people hold, which must never carry an admin permission.
var memberRoles = []string{
	model.SystemUserRoleId,
	model.SystemGuestRoleId,
	model.SystemPostAllRoleId,
	model.SystemPostAllPublicRoleId,
	model.SystemUserAccessTokenRoleId,
	model.TeamUserRoleId,
	model.TeamGuestRoleId,
	model.TeamPostAllRoleId,
	model.TeamPostAllPublicRoleId,
	model.ChannelUserRoleId,
	model.ChannelGuestRoleId,
	model.CustomGroupUserRoleId,
	model.PlaybookMemberRoleId,
	model.RunMemberRoleId,
}

// guestRoles are the roles of guests, held to a tighter list still.
var guestRoles = []string{model.SystemGuestRoleId, model.TeamGuestRoleId, model.ChannelGuestRoleId}

// adminPermission tells whether a permission belongs to admins only: holding it lets someone run the server, change
// what others may do, act on other people's accounts, or manage a team or channel as its admin.
func adminPermission(p string) bool {
	if strings.HasPrefix(p, "sysconsole_") {
		return true
	}
	switch p {
	case model.PermissionManageSystem.Id,
		model.PermissionManageRoles.Id,
		model.PermissionAssignSystemAdminRole.Id,
		model.PermissionEditOtherUsers.Id,
		model.PermissionPermanentDeleteUser.Id,
		model.PermissionManageTeam.Id,
		model.PermissionManageTeamRoles.Id,
		model.PermissionManageChannelRoles.Id,
		model.PermissionRemoveUserFromTeam.Id,
		model.PermissionPromoteGuest.Id,
		model.PermissionDemoteToGuest.Id,
		model.PermissionDeleteOthersPosts.Id,
		model.PermissionEditOthersPosts.Id,
		model.PermissionManageOthersIncomingWebhooks.Id,
		model.PermissionManageOthersOutgoingWebhooks.Id,
		model.PermissionManageOthersSlashCommands.Id,
		model.PermissionManageSystemWideOAuth.Id,
		model.PermissionManageSharedChannels.Id,
		model.PermissionManageSecureConnections.Id,
		model.PermissionInvalidateCaches.Id,
		model.PermissionManageJobs.Id,
		model.PermissionReadAudits.Id,
		model.PermissionGetLogs.Id,
		model.PermissionManageLicenseInformation.Id,
		model.PermissionReadOtherUsersTeams.Id,
		model.PermissionManageAIConnections.Id,
		model.PermissionManageTeamAIConnections.Id:
		return true
	}
	return false
}

func readRoles(t *testing.T, s store.Store) map[string]goldenRole {
	t.Helper()
	roles, err := s.Role().GetAll()
	require.NoError(t, err)
	out := make(map[string]goldenRole, len(roles))
	for _, r := range roles {
		perms := slices.Clone(r.Permissions)
		perms = slices.DeleteFunc(perms, func(p string) bool { return strings.TrimSpace(p) == "" })
		slices.Sort(perms)
		perms = slices.Compact(perms)
		if perms == nil {
			perms = []string{}
		}
		out[r.Name] = goldenRole{SchemeManaged: r.SchemeManaged, BuiltIn: r.BuiltIn, Permissions: perms}
	}
	return out
}

func goldenPath() string {
	// The test main helper changes directory, so find the package from the module root.
	return filepath.Join(server.GetPackagePath(), "channels", "app", "testdata", builtInRolesGoldenFile)
}

func readGoldenRoles(t *testing.T) map[string]goldenRole {
	t.Helper()
	data, err := os.ReadFile(goldenPath())
	require.NoError(t, err, "regenerate with AM_UPDATE_GOLDEN=1 (see builtInRolesGoldenFile)")
	var golden map[string]goldenRole
	require.NoError(t, json.Unmarshal(data, &golden))
	return golden
}

// TestBuiltInRolePermissionsGolden snapshots every role of a fresh install, once all the migrations ran: the system,
// team, channel, custom group, Playbooks roles and any other a migration creates.
func TestBuiltInRolePermissionsGolden(t *testing.T) {
	// An empty database: the schema is there, nothing else, so the server creates the roles and runs every
	// permission migration, as on a new install.
	th := SetupWithoutPreloadMigrations(t)

	roles := readRoles(t, th.App.Srv().Store())
	data, err := json.MarshalIndent(roles, "", "  ")
	require.NoError(t, err)
	data = append(data, '\n')

	if os.Getenv("AM_UPDATE_GOLDEN") == "1" {
		require.NoError(t, os.WriteFile(goldenPath(), data, 0o644))
	}
	expected, err := os.ReadFile(goldenPath())
	require.NoError(t, err, "regenerate with AM_UPDATE_GOLDEN=1 (see builtInRolesGoldenFile)")
	require.Equal(t, string(expected), string(data),
		"the default roles changed: if that's deliberate, regenerate %s with AM_UPDATE_GOLDEN=1 and review the diff", goldenPath())
}

// TestMemberRolesHoldNoAdminPermission checks the roles of a fresh install, and the golden file itself, so that
// regenerating the golden file can't slip an admin permission into a member's role.
func TestMemberRolesHoldNoAdminPermission(t *testing.T) {
	check := func(t *testing.T, roles map[string]goldenRole) {
		for _, name := range memberRoles {
			role, ok := roles[name]
			if !assert.True(t, ok, "role %s is missing", name) {
				continue
			}
			for _, p := range role.Permissions {
				assert.False(t, adminPermission(p), "member role %s holds the admin permission %s", name, p)
			}
		}
		for _, name := range guestRoles {
			for _, p := range roles[name].Permissions {
				assert.NotContains(t, []string{
					model.PermissionCreateTeam.Id,
					model.PermissionCreatePublicChannel.Id, model.PermissionCreatePrivateChannel.Id,
					model.PermissionAddUserToTeam.Id, model.PermissionInviteUser.Id, model.PermissionInviteGuest.Id,
					model.PermissionListPublicTeams.Id, model.PermissionJoinPublicTeams.Id, model.PermissionJoinPublicChannels.Id,
					model.PermissionCreateCustomGroup.Id, model.PermissionCreateEmojis.Id,
				}, p, "guest role %s holds %s", name, p)
			}
		}
		// Personal access tokens are for those an admin gives system_user_access_token to, not for everyone.
		assert.NotContains(t, roles[model.SystemUserRoleId].Permissions, model.PermissionCreateUserAccessToken.Id)
		// The admin roles still are admins: an update mustn't lock the admins out either.
		assert.Contains(t, roles[model.SystemAdminRoleId].Permissions, model.PermissionManageSystem.Id)
		assert.Contains(t, roles[model.TeamAdminRoleId].Permissions, model.PermissionManageTeam.Id)
		assert.Contains(t, roles[model.ChannelAdminRoleId].Permissions, model.PermissionManageChannelRoles.Id)
	}

	t.Run("the golden file", func(t *testing.T) {
		check(t, readGoldenRoles(t))
	})

	t.Run("a fresh install", func(t *testing.T) {
		th := SetupWithoutPreloadMigrations(t)
		check(t, readRoles(t, th.App.Srv().Store()))
	})
}

// TestUpgradeDoesNotPromoteAnyone starts the server on a database from an older version (the test warm-up snapshot:
// its roles and the migrations it had run) that already holds ordinary users, guests, team and channel members and an
// admin, so every role, permission and Antimatter app migration since runs over them. Nobody may come out of it with
// more than they had.
func TestUpgradeDoesNotPromoteAnyone(t *testing.T) {
	dbStore, sqlStore, dbSettings, searchEngine := setupStores(t)
	rctx := request.TestContext(t)

	newUser := func(roles string) *model.User {
		id := model.NewId()
		u, err := dbStore.User().Save(rctx, &model.User{
			Email:    "u" + id + "@example.com",
			Username: "u" + id,
			Password: "Password-" + id,
			Roles:    roles,
		})
		require.NoError(t, err)
		return u
	}
	admin := newUser(model.SystemAdminRoleId + " " + model.SystemUserRoleId)
	users := []*model.User{
		newUser(model.SystemUserRoleId),
		newUser(model.SystemUserRoleId),
		newUser(model.SystemGuestRoleId),
		newUser(model.SystemUserRoleId + " " + model.SystemPostAllRoleId),
		newUser(model.SystemUserRoleId + " " + model.SystemUserAccessTokenRoleId),
		admin,
	}
	guest := users[2]

	team, err := dbStore.Team().Save(&model.Team{
		DisplayName: "Community", Name: "c" + model.NewId(), Email: "team@example.com", Type: model.TeamOpen,
	})
	require.NoError(t, err)
	channel, err := dbStore.Channel().Save(rctx, &model.Channel{
		TeamId: team.Id, DisplayName: "Town", Name: "town-" + model.NewId(), Type: model.ChannelTypeOpen,
	}, -1)
	require.NoError(t, err)

	for _, u := range users {
		tm := &model.TeamMember{TeamId: team.Id, UserId: u.Id}
		cm := &model.ChannelMember{ChannelId: channel.Id, UserId: u.Id, NotifyProps: model.GetDefaultChannelNotifyProps()}
		switch u.Id {
		case guest.Id:
			tm.SchemeGuest, cm.SchemeGuest = true, true
		case admin.Id:
			tm.SchemeUser, tm.SchemeAdmin, cm.SchemeUser, cm.SchemeAdmin = true, true, true, true
		default:
			tm.SchemeUser, cm.SchemeUser = true, true
		}
		_, err = dbStore.Team().SaveMember(rctx, tm, -1)
		require.NoError(t, err)
		_, err = dbStore.Channel().SaveMember(rctx, cm)
		require.NoError(t, err)
	}

	// A server-wide custom group, which an Antimatter migration moves into a community.
	group, err := dbStore.Group().Create(&model.Group{
		Name: model.NewPointer("g" + model.NewId()), DisplayName: "Group", Source: model.GroupSourceCustom,
		RemoteId: model.NewPointer(model.NewId()), AllowReference: true,
	})
	require.NoError(t, err)
	_, err = dbStore.Group().UpsertMember(group.Id, users[0].Id)
	require.NoError(t, err)

	before := snapshotMemberships(t, dbStore, rctx, users, team.Id, channel.Id)

	// The upgrade: the server starts and runs every migration the old database lacks.
	th := setupTestHelper(dbStore, sqlStore, dbSettings, searchEngine, true, nil, nil, t)
	after := snapshotMemberships(t, th.App.Srv().Store(), rctx, users, team.Id, channel.Id)

	assert.Equal(t, before, after, "an upgrade changed someone's system, team or channel roles")

	for _, u := range users {
		fresh, appErr := th.App.GetUser(th.Context, u.Id)
		require.Nil(t, appErr)
		assert.Equal(t, u.Id == admin.Id, fresh.IsSystemAdmin(), "user %s (%s)", u.Username, u.Roles)
		assert.Equal(t, u.Id == guest.Id, fresh.IsGuest(), "user %s (%s)", u.Username, u.Roles)
		member, appErr := th.App.GetTeamMember(th.Context, team.Id, u.Id)
		require.Nil(t, appErr)
		assert.Equal(t, u.Id == admin.Id, member.SchemeAdmin, "team member %s", u.Username)
		assert.Equal(t, u.Id == guest.Id, member.SchemeGuest, "team member %s", u.Username)
	}

	t.Run("no member role gains an admin permission", func(t *testing.T) {
		upgraded := readRoles(t, th.App.Srv().Store())
		golden := readGoldenRoles(t)
		for _, name := range memberRoles {
			role, ok := upgraded[name]
			if !assert.True(t, ok, "role %s is missing after the upgrade", name) {
				continue
			}
			for _, p := range role.Permissions {
				assert.False(t, adminPermission(p), "after the upgrade, member role %s holds the admin permission %s", name, p)
				assert.Contains(t, golden[name].Permissions, p,
					"after the upgrade, member role %s holds %s, which a fresh install doesn't give it", name, p)
			}
		}
	})

	// Model connections (MigrationKeyAntimatterAIConnectionPermissions): the system admins manage them all, the
	// community's admins their community's; an upgraded server ends up as a fresh install, and nobody else gets them.
	t.Run("who manages model connections is the same as on a fresh install", func(t *testing.T) {
		upgraded := readRoles(t, th.App.Srv().Store())
		golden := readGoldenRoles(t)
		for name, role := range golden {
			for _, p := range []string{model.PermissionManageAIConnections.Id, model.PermissionManageTeamAIConnections.Id} {
				assert.Equal(t, slices.Contains(role.Permissions, p), slices.Contains(upgraded[name].Permissions, p),
					"%s holding %s after the upgrade", name, p)
			}
		}
		assert.Contains(t, upgraded[model.SystemAdminRoleId].Permissions, model.PermissionManageAIConnections.Id)
		assert.Contains(t, upgraded[model.TeamAdminRoleId].Permissions, model.PermissionManageTeamAIConnections.Id)
		assert.NotContains(t, upgraded[model.TeamAdminRoleId].Permissions, model.PermissionManageAIConnections.Id)
		assert.NotContains(t, upgraded[model.SystemUserRoleId].Permissions, model.PermissionManageTeamAIConnections.Id)
	})

	// Archiving channels moved from the members to the community's admins (MigrationKeyAntimatterChannelArchiveRights):
	// an upgraded server ends up as a fresh install.
	t.Run("who archives channels is the same as on a fresh install", func(t *testing.T) {
		upgraded := readRoles(t, th.App.Srv().Store())
		golden := readGoldenRoles(t)
		for _, name := range []string{model.ChannelUserRoleId, model.ChannelAdminRoleId, model.TeamUserRoleId, model.TeamAdminRoleId} {
			for _, p := range []string{model.PermissionDeletePublicChannel.Id, model.PermissionDeletePrivateChannel.Id} {
				assert.Equal(t, slices.Contains(golden[name].Permissions, p), slices.Contains(upgraded[name].Permissions, p),
					"%s holding %s after the upgrade", name, p)
			}
		}
		assert.NotContains(t, upgraded[model.ChannelUserRoleId].Permissions, model.PermissionDeletePublicChannel.Id)
		assert.Contains(t, upgraded[model.TeamAdminRoleId].Permissions, model.PermissionDeletePublicChannel.Id)
	})
}

type membershipSnapshot struct {
	UserRoles    map[string]string
	TeamRoles    map[string]string
	ChannelRoles map[string]string
}

func snapshotMemberships(t *testing.T, s store.Store, rctx request.CTX, users []*model.User, teamID, channelID string) membershipSnapshot {
	t.Helper()
	snap := membershipSnapshot{UserRoles: map[string]string{}, TeamRoles: map[string]string{}, ChannelRoles: map[string]string{}}
	for _, u := range users {
		fresh, err := s.User().Get(rctx, u.Id)
		require.NoError(t, err)
		snap.UserRoles[u.Id] = fresh.Roles
		tm, err := s.Team().GetMember(rctx, teamID, u.Id)
		require.NoError(t, err)
		snap.TeamRoles[u.Id] = strings.Join([]string{
			tm.ExplicitRoles, boolFlag("guest", tm.SchemeGuest), boolFlag("user", tm.SchemeUser), boolFlag("admin", tm.SchemeAdmin),
		}, "|")
		cm, err := s.Channel().GetMember(rctx, channelID, u.Id)
		require.NoError(t, err)
		snap.ChannelRoles[u.Id] = strings.Join([]string{
			cm.ExplicitRoles, boolFlag("guest", cm.SchemeGuest), boolFlag("user", cm.SchemeUser), boolFlag("admin", cm.SchemeAdmin),
		}, "|")
	}
	return snap
}

func boolFlag(name string, v bool) string {
	if v {
		return name
	}
	return "-"
}
