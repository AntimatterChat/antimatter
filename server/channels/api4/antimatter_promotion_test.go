// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// TestWhoCanPromoteWhom: only a system admin makes system admins or changes built-in roles, nobody raises their own
// roles, guests stay guests, and the most powerful Antimatter role a community can make still ends at the community.
func TestWhoCanPromoteWhom(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.GuestAccountsSettings.Enable = true })
	team, channel := th.BasicTeam, th.BasicChannel
	ctx := context.Background()

	loggedIn := func(t *testing.T, user *model.User) *model.Client4 {
		t.Helper()
		client := th.CreateClient()
		_, _, err := client.Login(ctx, user.Email, user.Password)
		require.NoError(t, err)
		return client
	}
	member := func(t *testing.T) (*model.User, *model.Client4) {
		t.Helper()
		user := th.CreateUser(t)
		th.LinkUserToTeam(t, user, team)
		th.AddUserToChannel(t, user, channel)
		return user, loggedIn(t, user)
	}
	forbidden := func(t *testing.T, resp *model.Response, err error, what string) {
		t.Helper()
		require.Error(t, err, what)
		CheckForbiddenStatus(t, resp)
	}

	teamAdmin, _ := member(t)
	th.UpdateUserToTeamAdmin(t, teamAdmin, team)
	channelAdmin, _ := member(t)
	th.MakeUserChannelAdmin(t, channelAdmin, channel)
	plain, _ := member(t)

	// The strongest Antimatter role a community's admins can make: every permission a role may hold.
	strongest, _, err := th.SystemAdminClient.CreateGroup(ctx, &model.Group{
		Name: new("strongest" + model.NewId()[:8]), DisplayName: "strongest", Source: model.GroupSourceCustom,
		AllowReference: true, TeamId: team.Id, Position: 0,
	})
	require.NoError(t, err)
	roleHolder, _ := member(t)
	_, _, err = th.SystemAdminClient.UpsertGroupMembers(ctx, strongest.Id, &model.GroupModifyMembers{UserIds: []string{roleHolder.Id}})
	require.NoError(t, err)
	body, err := json.Marshal(map[string][]string{"permissions": model.TeamRolePermissions()})
	require.NoError(t, err)
	resp, err := th.SystemAdminClient.DoAPIPut(ctx, "/groups/"+strongest.Id+"/team_role/permissions", string(body))
	require.NoError(t, err)
	resp.Body.Close()
	require.True(t, th.App.SessionHasPermissionToTeam(model.Session{UserId: roleHolder.Id, Roles: roleHolder.Roles,
		TeamMembers: mustTeamMembers(t, th, roleHolder.Id)}, team.Id, model.PermissionManageTeam), "the role works")

	// Log in again now that everyone has their roles, so no session predates them.
	teamAdminClient, channelAdminClient := loggedIn(t, teamAdmin), loggedIn(t, channelAdmin)
	plainClient, roleHolderClient := loggedIn(t, plain), loggedIn(t, roleHolder)
	_, _, err = roleHolderClient.PatchTeam(ctx, team.Id, &model.TeamPatch{Description: model.NewPointer("managed through a role")})
	require.NoError(t, err, "the role holder does manage the community")
	_, _, err = teamAdminClient.PatchTeam(ctx, team.Id, &model.TeamPatch{Description: model.NewPointer("managed by its admin")})
	require.NoError(t, err, "the community admin does manage the community")

	nonAdmins := []struct {
		name   string
		user   *model.User
		client *model.Client4
	}{
		{"a community admin", teamAdmin, teamAdminClient},
		{"a channel admin", channelAdmin, channelAdminClient},
		{"a member", plain, plainClient},
		{"the holder of an Antimatter role with every permission", roleHolder, roleHolderClient},
	}

	for _, who := range nonAdmins {
		t.Run(who.name+" makes nobody a system admin, themselves included", func(t *testing.T) {
			for _, target := range []*model.User{who.user, plain, th.BasicUser2} {
				resp, err := who.client.UpdateUserRoles(ctx, target.Id, model.SystemUserRoleId+" "+model.SystemAdminRoleId)
				forbidden(t, resp, err, "system admin")
				resp, err = who.client.UpdateUserRoles(ctx, target.Id, model.SystemUserRoleId+" "+model.SystemManagerRoleId)
				forbidden(t, resp, err, "system manager")
				resp, err = who.client.UpdateUserRoles(ctx, target.Id, model.SystemUserRoleId+" "+model.SystemUserAccessTokenRoleId)
				forbidden(t, resp, err, "personal access tokens")
			}
			user, appErr := th.App.GetUser(th.Context, who.user.Id)
			require.Nil(t, appErr)
			assert.False(t, user.IsSystemAdmin())
		})

		t.Run(who.name+" changes no built-in role", func(t *testing.T) {
			role, appErr := th.App.GetRoleByName(th.Context, model.SystemUserRoleId)
			require.Nil(t, appErr)
			perms := append(append([]string{}, role.Permissions...), model.PermissionManageSystem.Id)
			_, resp, err := who.client.PatchRole(ctx, role.Id, &model.RolePatch{Permissions: &perms})
			forbidden(t, resp, err, "patch system_user")

			teamUser, appErr := th.App.GetRoleByName(th.Context, model.TeamUserRoleId)
			require.Nil(t, appErr)
			perms = append(append([]string{}, teamUser.Permissions...), model.PermissionManageTeam.Id)
			_, resp, err = who.client.PatchRole(ctx, teamUser.Id, &model.RolePatch{Permissions: &perms})
			forbidden(t, resp, err, "patch team_user")
		})

		t.Run(who.name+" edits no one else's account", func(t *testing.T) {
			_, resp, err := who.client.PatchUser(ctx, th.BasicUser2.Id, &model.UserPatch{Email: model.NewPointer("taken" + model.NewId() + "@example.com")})
			forbidden(t, resp, err, "patch another user")
			resp, err = who.client.UpdateUserActive(ctx, th.BasicUser2.Id, false)
			forbidden(t, resp, err, "deactivate another user")
		})

		t.Run(who.name+" promotes no guest and demotes nobody to guest", func(t *testing.T) {
			guest := th.CreateGuestUser(t)
			resp, err := who.client.PromoteGuestToUser(ctx, guest.Id)
			forbidden(t, resp, err, "promote guest")
			resp, err = who.client.DemoteUserToGuest(ctx, th.BasicUser2.Id)
			forbidden(t, resp, err, "demote to guest")
		})
	}

	for _, who := range nonAdmins[1:] {
		t.Run(who.name+" doesn't make themselves a community admin", func(t *testing.T) {
			resp, err := who.client.UpdateTeamMemberRoles(ctx, team.Id, who.user.Id, model.TeamUserRoleId+" "+model.TeamAdminRoleId)
			forbidden(t, resp, err, "team roles")
			resp, err = who.client.UpdateTeamMemberSchemeRoles(ctx, team.Id, who.user.Id, &model.SchemeRoles{SchemeUser: true, SchemeAdmin: true})
			forbidden(t, resp, err, "team scheme roles")
			tm, appErr := th.App.GetTeamMember(th.Context, team.Id, who.user.Id)
			require.Nil(t, appErr)
			assert.False(t, tm.SchemeAdmin)
		})
	}

	t.Run("a member doesn't make themselves a channel admin", func(t *testing.T) {
		resp, err := plainClient.UpdateChannelRoles(ctx, channel.Id, plain.Id, model.ChannelUserRoleId+" "+model.ChannelAdminRoleId)
		forbidden(t, resp, err, "channel roles")
		resp, err = plainClient.UpdateChannelMemberSchemeRoles(ctx, channel.Id, plain.Id, &model.SchemeRoles{SchemeUser: true, SchemeAdmin: true})
		forbidden(t, resp, err, "channel scheme roles")
	})

	t.Run("a community admin makes community admins of their own community only", func(t *testing.T) {
		_, err := teamAdminClient.UpdateTeamMemberSchemeRoles(ctx, team.Id, plain.Id, &model.SchemeRoles{SchemeUser: true, SchemeAdmin: true})
		require.NoError(t, err)
		_, err = teamAdminClient.UpdateTeamMemberSchemeRoles(ctx, team.Id, plain.Id, &model.SchemeRoles{SchemeUser: true, SchemeAdmin: false})
		require.NoError(t, err)

		other := th.CreateTeam(t)
		th.LinkUserToTeam(t, plain, other)
		resp, err := teamAdminClient.UpdateTeamMemberSchemeRoles(ctx, other.Id, plain.Id, &model.SchemeRoles{SchemeUser: true, SchemeAdmin: true})
		forbidden(t, resp, err, "another community")
	})

	t.Run("a community's role can't hold a system permission, even set by a system admin", func(t *testing.T) {
		for _, p := range []string{model.PermissionManageSystem.Id, model.PermissionManageRoles.Id, model.PermissionAssignSystemAdminRole.Id,
			model.PermissionEditOtherUsers.Id, model.PermissionPromoteGuest.Id, model.PermissionCreateUserAccessToken.Id} {
			body, err := json.Marshal(map[string][]string{"permissions": {p}})
			require.NoError(t, err)
			resp, err := th.SystemAdminClient.DoAPIPut(ctx, "/groups/"+strongest.Id+"/team_role/permissions", string(body))
			require.Error(t, err, p)
			if resp != nil {
				resp.Body.Close()
				assert.Equal(t, 400, resp.StatusCode, p)
			}
		}
	})

	t.Run("a guest stays a guest", func(t *testing.T) {
		guest, guestClient := th.CreateGuestAndClient(t)
		th.LinkUserToTeam(t, guest, team)
		th.AddUserToChannel(t, guest, channel)

		resp, err := guestClient.PromoteGuestToUser(ctx, guest.Id)
		forbidden(t, resp, err, "promote themselves")
		resp, err = guestClient.UpdateUserRoles(ctx, guest.Id, model.SystemUserRoleId)
		forbidden(t, resp, err, "system_user")
		resp, err = guestClient.UpdateTeamMemberSchemeRoles(ctx, team.Id, guest.Id, &model.SchemeRoles{SchemeUser: true})
		forbidden(t, resp, err, "team member")
		resp, err = guestClient.UpdateChannelMemberSchemeRoles(ctx, channel.Id, guest.Id, &model.SchemeRoles{SchemeUser: true})
		forbidden(t, resp, err, "channel member")

		// Nor does a community admin make a guest a member: that's promote_guest, a system permission.
		resp, err = teamAdminClient.UpdateTeamMemberSchemeRoles(ctx, team.Id, guest.Id, &model.SchemeRoles{SchemeUser: true})
		require.Error(t, err)
		assert.Contains(t, []int{400, 403}, resp.StatusCode)

		user, appErr := th.App.GetUser(th.Context, guest.Id)
		require.Nil(t, appErr)
		assert.True(t, user.IsGuest())
		tm, appErr := th.App.GetTeamMember(th.Context, team.Id, guest.Id)
		require.Nil(t, appErr)
		assert.True(t, tm.SchemeGuest)
		assert.False(t, tm.SchemeUser)
	})
}

func mustTeamMembers(t *testing.T, th *TestHelper, userID string) []*model.TeamMember {
	t.Helper()
	members, appErr := th.App.GetTeamMembersForUser(th.Context, userID, "", true)
	require.Nil(t, appErr)
	return members
}
