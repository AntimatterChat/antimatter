// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// Antimatter: a community created without an address gets one from its name.
func TestCreateTeamWithoutAddress(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	ctx := context.Background()

	suffix := strings.ToLower(model.NewId()[:8])
	team, _, err := th.Client.CreateTeam(ctx, &model.Team{DisplayName: "Détecteur " + suffix, Type: model.TeamOpen})
	require.NoError(t, err)
	assert.Equal(t, "detecteur-"+suffix, team.Name)

	again, _, err := th.Client.CreateTeam(ctx, &model.Team{DisplayName: "Détecteur " + suffix, Type: model.TeamOpen})
	require.NoError(t, err)
	assert.Equal(t, "detecteur-"+suffix+"-2", again.Name, "a taken address gets a number")

	other, _, err := th.Client.CreateTeam(ctx, &model.Team{DisplayName: "日本語", Type: model.TeamOpen})
	require.NoError(t, err)
	assert.True(t, model.IsValidTeamName(other.Name))

	// An address asked for is checked as before.
	_, resp, err := th.Client.CreateTeam(ctx, &model.Team{DisplayName: "Taken", Name: team.Name, Type: model.TeamOpen})
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)
}

// Antimatter: guilds have everyone in all their channels, workspaces don't, on creation and when the kind changes.
func TestTeamKindSetsAllChannelsMembership(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	ctx := context.Background()
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.AllChannelsMembershipForNewTeams = true })

	create := func(kind string) *model.Team {
		t.Helper()
		team, _, err := th.Client.CreateTeam(ctx, &model.Team{DisplayName: "Kind " + kind, Type: model.TeamOpen, Kind: kind})
		require.NoError(t, err)
		return team
	}

	guild := create(model.TeamKindGuild)
	assert.Equal(t, model.TeamKindGuild, guild.Kind)
	assert.True(t, guild.AllChannelsMembership, "a guild has everyone in all its channels")

	workspace := create(model.TeamKindWorkspace)
	assert.Equal(t, model.TeamKindWorkspace, workspace.Kind)
	assert.False(t, workspace.AllChannelsMembership, "a workspace doesn't")

	former := create("workplace")
	assert.Equal(t, model.TeamKindWorkspace, former.Kind, "the former names are accepted")
	assert.False(t, former.AllChannelsMembership)

	t.Run("the server can turn it off for new communities", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.AllChannelsMembershipForNewTeams = false })
		defer th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.AllChannelsMembershipForNewTeams = true })
		assert.False(t, create(model.TeamKindGuild).AllChannelsMembership)
	})

	t.Run("changing the kind changes it, unless the change says otherwise", func(t *testing.T) {
		patched, _, err := th.Client.PatchTeam(ctx, guild.Id, &model.TeamPatch{Kind: model.NewPointer(model.TeamKindWorkspace)})
		require.NoError(t, err)
		assert.Equal(t, model.TeamKindWorkspace, patched.Kind)
		assert.False(t, patched.AllChannelsMembership, "members can leave channels in a workspace")

		patched, _, err = th.Client.PatchTeam(ctx, guild.Id, &model.TeamPatch{Kind: model.NewPointer(model.TeamKindGuild)})
		require.NoError(t, err)
		assert.True(t, patched.AllChannelsMembership, "a guild has everyone in all its channels")

		patched, _, err = th.Client.PatchTeam(ctx, guild.Id, &model.TeamPatch{Kind: model.NewPointer(model.TeamKindWorkspace), AllChannelsMembership: model.NewPointer(true)})
		require.NoError(t, err)
		assert.True(t, patched.AllChannelsMembership, "the change's own setting wins")

		patched, _, err = th.Client.PatchTeam(ctx, guild.Id, &model.TeamPatch{Kind: model.NewPointer(model.TeamKindWorkspace)})
		require.NoError(t, err)
		assert.True(t, patched.AllChannelsMembership, "the same kind again changes nothing")

		patched, _, err = th.Client.PatchTeam(ctx, guild.Id, &model.TeamPatch{AllChannelsMembership: model.NewPointer(false)})
		require.NoError(t, err)
		assert.False(t, patched.AllChannelsMembership, "it can still be changed on its own")
		assert.Equal(t, model.TeamKindWorkspace, patched.Kind)
	})

	t.Run("members don't change the kind", func(t *testing.T) {
		client := th.CreateClient()
		_, _, err := client.Login(ctx, th.BasicUser2.Email, th.BasicUser2.Password)
		require.NoError(t, err)
		th.LinkUserToTeam(t, th.BasicUser2, workspace)
		_, resp, err := client.PatchTeam(ctx, workspace.Id, &model.TeamPatch{Kind: model.NewPointer(model.TeamKindGuild)})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		fetched, appErr := th.App.GetTeam(workspace.Id)
		require.Nil(t, appErr)
		assert.False(t, fetched.AllChannelsMembership)
	})
}

// Antimatter: the community's channel for join and leave messages is its admins' to choose or turn off.
func TestWelcomeChannelSettings(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	ctx := context.Background()

	team := th.CreateTeam(t)
	welcome, appErr := th.App.GetChannelByName(th.Context, model.WelcomeChannelName, team.Id, false)
	require.Nil(t, appErr)
	assert.Equal(t, welcome.Id, team.WelcomeChannelId, "a new community's Welcome channel takes them")

	client := th.CreateClient()
	_, _, err := client.Login(ctx, th.BasicUser2.Email, th.BasicUser2.Password)
	require.NoError(t, err)
	th.LinkUserToTeam(t, th.BasicUser2, team)
	for _, patch := range []*model.TeamPatch{
		{DisableJoinLeaveMessages: model.NewPointer(true)},
		{WelcomeChannelId: model.NewPointer("")},
	} {
		_, resp, patchErr := client.PatchTeam(ctx, team.Id, patch)
		require.Error(t, patchErr, "members don't change it")
		CheckForbiddenStatus(t, resp)
	}

	private, _, err := th.Client.CreateChannel(ctx, &model.Channel{TeamId: team.Id, Name: GenerateTestChannelName(), DisplayName: "Private", Type: model.ChannelTypePrivate})
	require.NoError(t, err)
	_, resp, err := th.Client.PatchTeam(ctx, team.Id, &model.TeamPatch{WelcomeChannelId: model.NewPointer(private.Id)})
	require.Error(t, err, "a private channel isn't everyone's")
	CheckBadRequestStatus(t, resp)
	_, resp, err = th.Client.PatchTeam(ctx, team.Id, &model.TeamPatch{WelcomeChannelId: model.NewPointer("not-an-id")})
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	patched, _, err := th.Client.PatchTeam(ctx, team.Id, &model.TeamPatch{DisableJoinLeaveMessages: model.NewPointer(true)})
	require.NoError(t, err)
	assert.True(t, patched.DisableJoinLeaveMessages)
	assert.Equal(t, welcome.Id, patched.WelcomeChannelId, "turning them off keeps the channel")
	fetched, _, err := client.GetTeam(ctx, team.Id, "")
	require.NoError(t, err)
	assert.True(t, fetched.DisableJoinLeaveMessages, "members see it")
}

func ensureDefaultRole(t *testing.T, client *model.Client4, teamID string) (*model.Group, *http.Response, error) {
	t.Helper()
	resp, err := client.DoAPIPost(context.Background(), "/teams/"+teamID+"/default_role", "")
	if err != nil {
		return nil, resp, err
	}
	defer resp.Body.Close()
	var group model.Group
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&group))
	return &group, resp, nil
}

// Antimatter: a community's default role is made by those who manage its roles, and the server keeps it: it can't be
// deleted, given, taken away, opened to all, moved or mentioned.
func TestTeamDefaultRoleAPI(t *testing.T) {
	mainHelper.Parallel(t)
	h := newRightsHelper(t, "")
	ctx := context.Background()
	h.th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableCustomGroups = true })

	member, memberClient := h.member(t)
	_, httpResp, err := ensureDefaultRole(t, memberClient, h.team.Id)
	require.Error(t, err, "members don't make it")
	assert.Equal(t, http.StatusForbidden, httpResp.StatusCode)
	var resp *model.Response

	_, adminClient := h.admin(t)
	role, _, err := ensureDefaultRole(t, adminClient, h.team.Id)
	require.NoError(t, err)
	assert.Equal(t, h.team.Id, role.TeamId)
	assert.Equal(t, model.GroupPositionMax, role.Position)
	assert.False(t, role.AllowReference)

	again, _, err := ensureDefaultRole(t, adminClient, h.team.Id)
	require.NoError(t, err)
	assert.Equal(t, role.Id, again.Id)

	fetched, _, err := memberClient.GetTeam(ctx, h.team.Id, "")
	require.NoError(t, err)
	assert.Equal(t, role.Id, fetched.DefaultRoleId, "everyone knows the community's default role")
	require.Eventually(t, func() bool { return h.th.App.IsGroupMember(role.Id, member.Id) }, 10*time.Second, 50*time.Millisecond, "members have it")

	// Members see it with the community's other roles, though it isn't mentioned.
	groups, _, err := memberClient.GetGroups(ctx, model.GroupSearchOpts{Source: model.GroupSourceCustom, PageOpts: &model.PageOpts{Page: 0, PerPage: 200}})
	require.NoError(t, err)
	found := false
	for _, g := range groups {
		found = found || g.Id == role.Id
	}
	assert.True(t, found)

	// But not with the groups to mention.
	mentionable, _, err := memberClient.GetGroups(ctx, model.GroupSearchOpts{Source: model.GroupSourceCustom, FilterAllowReference: true, PageOpts: &model.PageOpts{Page: 0, PerPage: 200}})
	require.NoError(t, err)
	for _, g := range mentionable {
		assert.NotEqual(t, role.Id, g.Id, "nobody mentions everyone through the default role")
	}

	for _, client := range []*model.Client4{adminClient, h.th.SystemAdminClient} {
		_, resp, err = client.DeleteGroup(ctx, role.Id)
		require.Error(t, err, "it can't be deleted")
		CheckBadRequestStatus(t, resp)

		_, resp, err = client.DeleteGroupMembers(ctx, role.Id, &model.GroupModifyMembers{UserIds: []string{member.Id}})
		require.Error(t, err, "nor taken away")
		CheckBadRequestStatus(t, resp)

		other := h.th.CreateUser(t)
		h.th.LinkUserToTeam(t, other, h.team)
		_, resp, err = client.UpsertGroupMembers(ctx, role.Id, &model.GroupModifyMembers{UserIds: []string{other.Id}})
		require.Error(t, err, "nor given")
		CheckBadRequestStatus(t, resp)

		for _, patch := range []*model.GroupPatch{
			{Position: model.NewPointer(0)},
			{SelfAssignable: model.NewPointer(true)},
			{AllowReference: model.NewPointer(true)},
		} {
			_, resp, err = client.PatchGroup(ctx, role.Id, patch)
			require.Error(t, err)
			CheckBadRequestStatus(t, resp)
		}
	}

	// Its looks and permissions are the admins'.
	patched, _, err := adminClient.PatchGroup(ctx, role.Id, &model.GroupPatch{Color: model.NewPointer("#3498db"), DisplayName: model.NewPointer("Everyone")})
	require.NoError(t, err)
	assert.Equal(t, "#3498db", patched.Color)
	h.grant(t, role, model.PermissionCreatePublicChannel.Id)
	assert.Contains(t, h.rolePermissions(t, role), model.PermissionCreatePublicChannel.Id)

	// Members can't change it.
	_, resp, err = memberClient.PatchGroup(ctx, role.Id, &model.GroupPatch{Color: model.NewPointer("#e74c3c")})
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)
}
