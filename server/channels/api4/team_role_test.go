// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// Antimatter roles are groups of one team, managed by its admins.
func TestTeamRoles(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	role := func(name string) *model.Group {
		return &model.Group{
			Name:           new(name + model.NewId()[:8]),
			DisplayName:    name,
			Source:         model.GroupSourceCustom,
			AllowReference: true,
			TeamId:         th.BasicTeam.Id,
			Color:          "#ff8800",
			Hoist:          true,
		}
	}

	t.Run("team admins create roles, members don't", func(t *testing.T) {
		created, _, err := th.SystemAdminClient.CreateGroup(context.Background(), role("moderators"))
		require.NoError(t, err)
		assert.Equal(t, th.BasicTeam.Id, created.TeamId)
		assert.Equal(t, "#ff8800", created.Color)
		assert.True(t, created.Hoist)

		_, resp, err := th.Client.CreateGroup(context.Background(), role("rebels"))
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("a role's members can't manage it", func(t *testing.T) {
		created, _, err := th.SystemAdminClient.CreateGroup(context.Background(), role("helpers"))
		require.NoError(t, err)
		_, _, err = th.SystemAdminClient.UpsertGroupMembers(context.Background(), created.Id, &model.GroupModifyMembers{UserIds: []string{th.BasicUser.Id}})
		require.NoError(t, err)

		_, resp, err := th.Client.UpsertGroupMembers(context.Background(), created.Id, &model.GroupModifyMembers{UserIds: []string{th.BasicUser2.Id}})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		color := "#000000"
		_, resp, err = th.Client.PatchGroup(context.Background(), created.Id, &model.GroupPatch{Color: &color})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		position, hoist := 1, false
		patched, _, err := th.SystemAdminClient.PatchGroup(context.Background(), created.Id, &model.GroupPatch{Position: &position, Hoist: &hoist})
		require.NoError(t, err)
		assert.Equal(t, 1, patched.Position)
		assert.False(t, patched.Hoist)
	})

	t.Run("a role needs a team that exists", func(t *testing.T) {
		r := role("ghosts")
		r.TeamId = model.NewId()
		_, _, err := th.SystemAdminClient.CreateGroup(context.Background(), r)
		require.Error(t, err)
	})

	t.Run("team admins set the team's display settings", func(t *testing.T) {
		hide, badges := true, true
		team, _, err := th.SystemAdminClient.PatchTeam(context.Background(), th.BasicTeam.Id, &model.TeamPatch{HideRoleColors: &hide, RequireTeamBadges: &badges})
		require.NoError(t, err)
		assert.True(t, team.HideRoleColors)
		assert.True(t, team.RequireTeamBadges)

		fetched, _, err := th.Client.GetTeam(context.Background(), th.BasicTeam.Id, "")
		require.NoError(t, err)
		assert.True(t, fetched.HideRoleColors)
		assert.True(t, fetched.RequireTeamBadges)
	})
}

// Antimatter roles grant permissions in their team, through a Mattermost role their members hold.
func TestTeamRolePermissions(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	team := th.BasicTeam

	newRole := func(t *testing.T, client *model.Client4, name string, position int) *model.Group {
		t.Helper()
		created, _, err := client.CreateGroup(context.Background(), &model.Group{
			Name:           new(name + model.NewId()[:8]),
			DisplayName:    name,
			Source:         model.GroupSourceCustom,
			AllowReference: true,
			TeamId:         team.Id,
			Position:       position,
		})
		require.NoError(t, err)
		return created
	}
	grant := func(client *model.Client4, group *model.Group, permissions ...string) (*http.Response, error) {
		body, err := json.Marshal(map[string][]string{"permissions": permissions})
		require.NoError(t, err)
		return client.DoAPIPut(context.Background(), "/groups/"+group.Id+"/team_role/permissions", string(body))
	}
	join := func(t *testing.T, group *model.Group, userIDs ...string) {
		t.Helper()
		_, _, err := th.SystemAdminClient.UpsertGroupMembers(context.Background(), group.Id, &model.GroupModifyMembers{UserIds: userIDs})
		require.NoError(t, err)
	}

	t.Run("a role's permission works in its team, and goes with it", func(t *testing.T) {
		kickers := newRole(t, th.SystemAdminClient, "kickers", 0)
		resp, err := grant(th.SystemAdminClient, kickers, model.PermissionRemoveUserFromTeam.Id)
		require.NoError(t, err)
		resp.Body.Close()

		other := th.CreateUser(t)
		th.LinkUserToTeam(t, other, team)

		_, err = th.Client.RemoveTeamMember(context.Background(), team.Id, other.Id)
		require.Error(t, err, "without the role, members can't remove people")

		join(t, kickers, th.BasicUser.Id)
		_, err = th.Client.RemoveTeamMember(context.Background(), team.Id, other.Id)
		require.NoError(t, err)

		_, _, err = th.SystemAdminClient.DeleteGroupMembers(context.Background(), kickers.Id, &model.GroupModifyMembers{UserIds: []string{th.BasicUser.Id}})
		require.NoError(t, err)
		member, appErr := th.App.GetTeamMember(th.Context, team.Id, th.BasicUser.Id)
		require.Nil(t, appErr)
		assert.NotContains(t, member.Roles, model.TeamRoleName(kickers.Id))
	})

	t.Run("a role grants team and channel permissions only", func(t *testing.T) {
		role := newRole(t, th.SystemAdminClient, "admins", 0)
		resp, err := grant(th.SystemAdminClient, role, model.PermissionManageSystem.Id)
		require.Error(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("only people of the team can have its roles", func(t *testing.T) {
		role := newRole(t, th.SystemAdminClient, "insiders", 0)
		outsider := th.CreateUser(t)
		_, resp, err := th.SystemAdminClient.UpsertGroupMembers(context.Background(), role.Id, &model.GroupModifyMembers{UserIds: []string{outsider.Id}})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})

	t.Run("those who manage roles do so below their own highest role", func(t *testing.T) {
		managers := newRole(t, th.SystemAdminClient, "managers", 0)
		resp, err := grant(th.SystemAdminClient, managers, model.PermissionManageTeamRoles.Id)
		require.NoError(t, err)
		resp.Body.Close()
		join(t, managers, th.BasicUser.Id)

		// A role they make lands below theirs.
		made := newRole(t, th.Client, "helpers", 0)
		assert.Equal(t, 1, made.Position)

		// They manage roles below theirs, not their own.
		hoist := true
		_, _, err = th.Client.PatchGroup(context.Background(), made.Id, &model.GroupPatch{Hoist: &hoist})
		require.NoError(t, err)
		_, resp2, err := th.Client.PatchGroup(context.Background(), managers.Id, &model.GroupPatch{Hoist: &hoist})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp2)

		// They can't move a role above theirs, nor hand out what they don't have.
		top := 0
		_, resp2, err = th.Client.PatchGroup(context.Background(), made.Id, &model.GroupPatch{Position: &top})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp2)

		resp, err = grant(th.Client, made, model.PermissionManageTeam.Id)
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		resp, err = grant(th.Client, made, model.PermissionManageTeamRoles.Id)
		require.NoError(t, err)
		resp.Body.Close()
	})
}

// Roles open to all of their team are taken and left by people themselves; teams hold their onboarding.
func TestSelfAssignedRolesAndOnboarding(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	team := th.BasicTeam

	newRole := func(t *testing.T, name string, open bool) *model.Group {
		t.Helper()
		created, _, err := th.SystemAdminClient.CreateGroup(context.Background(), &model.Group{
			Name: new(name + model.NewId()[:8]), DisplayName: name, Source: model.GroupSourceCustom, AllowReference: true, TeamId: team.Id,
		})
		require.NoError(t, err)
		if open {
			selfAssignable := true
			created, _, err = th.SystemAdminClient.PatchGroup(context.Background(), created.Id, &model.GroupPatch{SelfAssignable: &selfAssignable})
			require.NoError(t, err)
			require.True(t, created.SelfAssignable)
		}
		return created
	}
	members := func(userIDs ...string) *model.GroupModifyMembers {
		return &model.GroupModifyMembers{UserIds: userIDs}
	}

	t.Run("people take and leave a role open to all, for themselves only", func(t *testing.T) {
		gamers := newRole(t, "gamers", true)

		_, _, err := th.Client.UpsertGroupMembers(context.Background(), gamers.Id, members(th.BasicUser.Id))
		require.NoError(t, err)
		member, appErr := th.App.GetTeamMember(th.Context, team.Id, th.BasicUser.Id)
		require.Nil(t, appErr)
		assert.Contains(t, member.Roles, model.TeamRoleName(gamers.Id))

		_, resp, err := th.Client.UpsertGroupMembers(context.Background(), gamers.Id, members(th.BasicUser2.Id))
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		_, _, err = th.Client.DeleteGroupMembers(context.Background(), gamers.Id, members(th.BasicUser.Id))
		require.NoError(t, err)
	})

	t.Run("other roles stay with the admins", func(t *testing.T) {
		closed := newRole(t, "closed", false)
		_, resp, err := th.Client.UpsertGroupMembers(context.Background(), closed.Id, members(th.BasicUser.Id))
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("a team keeps its onboarding, as JSON", func(t *testing.T) {
		onboarding := `{"welcome":"Hi","questions":[]}`
		patched, _, err := th.SystemAdminClient.PatchTeam(context.Background(), team.Id, &model.TeamPatch{Onboarding: &onboarding})
		require.NoError(t, err)
		assert.Equal(t, onboarding, patched.Onboarding)

		broken := `{"welcome":`
		_, resp, err := th.SystemAdminClient.PatchTeam(context.Background(), team.Id, &model.TeamPatch{Onboarding: &broken})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
	})
}
