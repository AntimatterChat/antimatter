// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
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
