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

// The lives of Antimatter's roles, overrides and server categories: what happens when people come and go, roles go
// away, and admins of one team reach for another.
func TestAntimatterLifecycles(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	team := th.BasicTeam

	newRole := func(t *testing.T, teamID, name string, userIDs ...string) *model.Group {
		t.Helper()
		created, _, err := th.SystemAdminClient.CreateGroup(context.Background(), &model.Group{
			Name: new(name + model.NewId()[:8]), DisplayName: name, Source: model.GroupSourceCustom, AllowReference: true, TeamId: teamID,
		})
		require.NoError(t, err)
		if len(userIDs) > 0 {
			_, _, err = th.SystemAdminClient.UpsertGroupMembers(context.Background(), created.Id, &model.GroupModifyMembers{UserIds: userIDs})
			require.NoError(t, err)
		}
		return created
	}
	put := func(t *testing.T, client *model.Client4, path string, body any) (*http.Response, error) {
		t.Helper()
		data, err := json.Marshal(body)
		require.NoError(t, err)
		resp, err := client.DoAPIPut(context.Background(), path, string(data))
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		return resp, err
	}
	grant := func(t *testing.T, role *model.Group, permissions ...string) {
		t.Helper()
		_, err := put(t, th.SystemAdminClient, "/groups/"+role.Id+"/team_role/permissions", map[string][]string{"permissions": permissions})
		require.NoError(t, err)
	}
	explicitRoles := func(t *testing.T, teamID, userID string) string {
		t.Helper()
		member, appErr := th.App.GetTeamMember(th.Context, teamID, userID)
		require.Nil(t, appErr)
		return member.ExplicitRoles
	}
	loggedIn := func(t *testing.T, user *model.User) *model.Client4 {
		t.Helper()
		client := th.CreateClient()
		_, _, err := client.Login(context.Background(), user.Email, user.Password)
		require.NoError(t, err)
		return client
	}

	t.Run("a deleted role takes its permissions with it", func(t *testing.T) {
		user := th.CreateUser(t)
		th.LinkUserToTeam(t, user, team)
		role := newRole(t, team.Id, "doomed", user.Id)
		require.Contains(t, explicitRoles(t, team.Id, user.Id), model.TeamRoleName(role.Id))

		_, _, err := th.SystemAdminClient.DeleteGroup(context.Background(), role.Id)
		require.NoError(t, err)
		assert.NotContains(t, explicitRoles(t, team.Id, user.Id), model.TeamRoleName(role.Id))
	})

	t.Run("a role's new permissions reach its members at once", func(t *testing.T) {
		user := th.CreateUser(t)
		th.LinkUserToTeam(t, user, team)
		other := th.CreateUser(t)
		th.LinkUserToTeam(t, other, team)
		role := newRole(t, team.Id, "late", user.Id)
		client := loggedIn(t, user)

		_, err := client.RemoveTeamMember(context.Background(), team.Id, other.Id)
		require.Error(t, err)
		grant(t, role, model.PermissionRemoveUserFromTeam.Id)
		_, err = client.RemoveTeamMember(context.Background(), team.Id, other.Id)
		require.NoError(t, err)
	})

	t.Run("someone who leaves the team leaves its roles", func(t *testing.T) {
		user := th.CreateUser(t)
		th.LinkUserToTeam(t, user, team)
		role := newRole(t, team.Id, "leavers", user.Id)

		_, err := th.SystemAdminClient.RemoveTeamMember(context.Background(), team.Id, user.Id)
		require.NoError(t, err)

		members, _, err := th.SystemAdminClient.GetGroupMembers(context.Background(), role.Id)
		require.NoError(t, err)
		for _, m := range members.Members {
			assert.NotEqual(t, user.Id, m.Id, "they're still listed in the role")
		}

		th.LinkUserToTeam(t, user, team)
		assert.NotContains(t, explicitRoles(t, team.Id, user.Id), model.TeamRoleName(role.Id), "coming back doesn't bring the role back")
	})

	t.Run("joining a denied role takes effect at once in its channels", func(t *testing.T) {
		user := th.CreateUser(t)
		th.LinkUserToTeam(t, user, team)
		channel := th.CreatePublicChannel(t)
		th.AddUserToChannel(t, user, channel)
		client := loggedIn(t, user)
		muted := newRole(t, team.Id, "muted")
		_, err := put(t, th.SystemAdminClient, "/channels/"+channel.Id+"/overrides", model.ChannelOverride{TargetType: model.ChannelOverrideTargetRole, TargetId: muted.Id, Deny: []string{model.PermissionCreatePost.Id}})
		require.NoError(t, err)

		_, _, err = client.CreatePost(context.Background(), &model.Post{ChannelId: channel.Id, Message: "before"})
		require.NoError(t, err)

		_, _, err = th.SystemAdminClient.UpsertGroupMembers(context.Background(), muted.Id, &model.GroupModifyMembers{UserIds: []string{user.Id}})
		require.NoError(t, err)
		_, _, err = client.CreatePost(context.Background(), &model.Post{ChannelId: channel.Id, Message: "after"})
		require.Error(t, err, "the deny applies as soon as they join the role")

		_, _, err = th.SystemAdminClient.DeleteGroupMembers(context.Background(), muted.Id, &model.GroupModifyMembers{UserIds: []string{user.Id}})
		require.NoError(t, err)
		_, _, err = client.CreatePost(context.Background(), &model.Post{ChannelId: channel.Id, Message: "again"})
		require.NoError(t, err, "and lifts as soon as they leave it")
	})

	t.Run("one team's admins can't reach into another team", func(t *testing.T) {
		otherTeam := th.CreateTeam(t)
		admin := th.CreateUser(t)
		th.LinkUserToTeam(t, admin, otherTeam)
		th.LinkUserToTeam(t, admin, team)
		_, appErr := th.App.UpdateTeamMemberRoles(th.Context, otherTeam.Id, admin.Id, model.TeamUserRoleId+" "+model.TeamAdminRoleId)
		require.Nil(t, appErr)
		client := loggedIn(t, admin)

		resp, err := put(t, client, "/teams/"+team.Id+"/server_categories", map[string]any{"categories": []map[string]any{{"name": "Taken", "locked": true}}})
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)

		_, groupResp, err := client.CreateGroup(context.Background(), &model.Group{
			Name: new("intruders" + model.NewId()[:8]), DisplayName: "Intruders", Source: model.GroupSourceCustom, AllowReference: true, TeamId: team.Id,
		})
		require.Error(t, err)
		CheckForbiddenStatus(t, groupResp)

		role := newRole(t, team.Id, "theirs")
		resp, err = put(t, client, "/groups/"+role.Id+"/team_role/permissions", map[string][]string{"permissions": {model.PermissionRemoveUserFromTeam.Id}})
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)

		resp, err = put(t, client, "/channels/"+th.BasicChannel.Id+"/overrides", model.ChannelOverride{TargetType: model.ChannelOverrideTargetRole, TargetId: role.Id, Deny: []string{model.PermissionCreatePost.Id}})
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("a role of one team can't be used in another team's channel", func(t *testing.T) {
		otherTeam := th.CreateTeam(t)
		foreign := newRole(t, otherTeam.Id, "foreign")
		resp, err := put(t, th.SystemAdminClient, "/channels/"+th.BasicChannel.Id+"/overrides", model.ChannelOverride{TargetType: model.ChannelOverrideTargetRole, TargetId: foreign.Id, Deny: []string{model.PermissionCreatePost.Id}})
		require.Error(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("a self-assignable role of another team stays out of reach", func(t *testing.T) {
		otherTeam := th.CreateTeamWithClient(t, th.SystemAdminClient)
		open, _, err := th.SystemAdminClient.CreateGroup(context.Background(), &model.Group{
			Name: new("open" + model.NewId()[:8]), DisplayName: "Open", Source: model.GroupSourceCustom, AllowReference: true, TeamId: otherTeam.Id, SelfAssignable: true,
		})
		require.NoError(t, err)
		_, _, err = th.Client.UpsertGroupMembers(context.Background(), open.Id, &model.GroupModifyMembers{UserIds: []string{th.BasicUser.Id}})
		require.Error(t, err)
	})

	createRole := func(t *testing.T, fields model.Group, userIDs ...string) *model.Group {
		t.Helper()
		fields.Name = new(fields.DisplayName + model.NewId()[:8])
		fields.Source = model.GroupSourceCustom
		fields.AllowReference = true
		if fields.TeamId == "" {
			fields.TeamId = team.Id
		}
		created, _, err := th.SystemAdminClient.CreateGroup(context.Background(), &fields)
		require.NoError(t, err)
		if len(userIDs) > 0 {
			_, _, err = th.SystemAdminClient.UpsertGroupMembers(context.Background(), created.Id, &model.GroupModifyMembers{UserIds: userIDs})
			require.NoError(t, err)
		}
		return created
	}

	t.Run("managing roles through a role doesn't make anyone a team admin", func(t *testing.T) {
		manager := th.CreateUser(t)
		th.LinkUserToTeam(t, manager, team)
		managers := createRole(t, model.Group{DisplayName: "managers", Position: 1}, manager.Id)
		grant(t, managers, model.PermissionManageTeamRoles.Id)
		client := loggedIn(t, manager)

		resp, err := client.UpdateTeamMemberSchemeRoles(context.Background(), team.Id, manager.Id, &model.SchemeRoles{SchemeUser: true, SchemeAdmin: true})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		resp, err = client.UpdateTeamMemberRoles(context.Background(), team.Id, manager.Id, model.TeamUserRoleId+" "+model.TeamAdminRoleId)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		other := createRole(t, model.Group{DisplayName: "other", Position: 0})
		resp, err = client.UpdateTeamMemberRoles(context.Background(), team.Id, manager.Id, model.TeamUserRoleId+" "+model.TeamRoleName(other.Id))
		require.Error(t, err, "nor take another role's Mattermost role directly")
		CheckForbiddenStatus(t, resp)
		// Nor make the members of a synced LDAP group the team's admins.
		ldap, appErr := th.App.CreateGroup(&model.Group{Name: new("ldap" + model.NewId()[:8]), DisplayName: "ldap", Source: model.GroupSourceLdap, RemoteId: new(model.NewId()), AllowReference: true})
		require.Nil(t, appErr)
		_, _, err = th.SystemAdminClient.LinkGroupSyncable(context.Background(), ldap.Id, team.Id, model.GroupSyncableTypeTeam, &model.GroupSyncablePatch{AutoAdd: new(true)})
		require.NoError(t, err)
		_, resp, err = client.PatchGroupSyncable(context.Background(), ldap.Id, team.Id, model.GroupSyncableTypeTeam, &model.GroupSyncablePatch{SchemeAdmin: new(true)})
		require.Error(t, err, "nor make a synced group's members team admins")
		CheckForbiddenStatus(t, resp)
	})

	t.Run("setting someone's team roles keeps the Antimatter roles they hold, and only those", func(t *testing.T) {
		user := th.CreateUser(t)
		th.LinkUserToTeam(t, user, team)
		held := createRole(t, model.Group{DisplayName: "held"}, user.Id)
		notHeld := createRole(t, model.Group{DisplayName: "notheld"})

		_, err := th.SystemAdminClient.UpdateTeamMemberRoles(context.Background(), team.Id, user.Id, model.TeamUserRoleId+" "+model.TeamRoleName(notHeld.Id))
		require.NoError(t, err)
		roles := explicitRoles(t, team.Id, user.Id)
		assert.Contains(t, roles, model.TeamRoleName(held.Id))
		assert.NotContains(t, roles, model.TeamRoleName(notHeld.Id))
	})

	t.Run("a channel's admin only overrides what they may do themselves", func(t *testing.T) {
		channel, _, err := th.Client.CreateChannel(context.Background(), &model.Channel{DisplayName: "mine", Name: GenerateTestChannelName(), Type: model.ChannelTypeOpen, TeamId: team.Id})
		require.NoError(t, err)
		mods := createRole(t, model.Group{DisplayName: "mods"})
		path := "/channels/" + channel.Id + "/overrides"

		resp, err := put(t, th.Client, path, model.ChannelOverride{TargetType: model.ChannelOverrideTargetUser, TargetId: th.BasicUser.Id, Allow: []string{model.PermissionEditOthersPosts.Id}})
		require.Error(t, err, "no allowing themselves more")
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)

		resp, err = put(t, th.Client, path, model.ChannelOverride{TargetType: model.ChannelOverrideTargetRole, TargetId: mods.Id, Deny: []string{model.PermissionDeleteOthersPosts.Id}})
		require.Error(t, err, "no taking from moderators what they don't hold")
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)

		_, err = put(t, th.Client, path, model.ChannelOverride{TargetType: model.ChannelOverrideTargetRole, TargetId: mods.Id, Deny: []string{model.PermissionCreatePost.Id}})
		require.NoError(t, err, "what they hold, they override")

		_, err = put(t, th.SystemAdminClient, path, model.ChannelOverride{TargetType: model.ChannelOverrideTargetRole, TargetId: mods.Id, Deny: []string{model.PermissionCreatePost.Id, model.PermissionDeleteOthersPosts.Id}})
		require.NoError(t, err)
		resp, err = put(t, th.Client, path, model.ChannelOverride{TargetType: model.ChannelOverrideTargetRole, TargetId: mods.Id, Deny: []string{model.PermissionCreatePost.Id}})
		require.Error(t, err, "nor lifting a team admin's deny of what they don't hold")
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("overrides are for people of the channel's team", func(t *testing.T) {
		outsider := th.CreateUser(t)
		resp, err := put(t, th.SystemAdminClient, "/channels/"+th.BasicChannel.Id+"/overrides", model.ChannelOverride{TargetType: model.ChannelOverrideTargetUser, TargetId: outsider.Id, Allow: []string{model.PermissionCreatePost.Id}})
		require.Error(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("a role anyone can take doesn't lift its holder in the hierarchy", func(t *testing.T) {
		manager := th.CreateUser(t)
		th.LinkUserToTeam(t, manager, team)
		open := createRole(t, model.Group{DisplayName: "open", Position: 0, SelfAssignable: true})
		above := createRole(t, model.Group{DisplayName: "above", Position: 1})
		managers := createRole(t, model.Group{DisplayName: "mgrs", Position: 2}, manager.Id)
		grant(t, managers, model.PermissionManageTeamRoles.Id)
		client := loggedIn(t, manager)

		_, _, err := client.UpsertGroupMembers(context.Background(), open.Id, &model.GroupModifyMembers{UserIds: []string{manager.Id}})
		require.NoError(t, err)
		_, resp, err := client.PatchGroup(context.Background(), above.Id, &model.GroupPatch{DisplayName: new("mine now")})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("a deleted role can't be taken, joined or given permissions", func(t *testing.T) {
		user := th.CreateUser(t)
		th.LinkUserToTeam(t, user, team)
		role := createRole(t, model.Group{DisplayName: "gone", SelfAssignable: true})
		_, _, err := th.SystemAdminClient.DeleteGroup(context.Background(), role.Id)
		require.NoError(t, err)
		client := loggedIn(t, user)

		_, _, err = client.UpsertGroupMembers(context.Background(), role.Id, &model.GroupModifyMembers{UserIds: []string{user.Id}})
		require.Error(t, err)
		_, _, err = th.SystemAdminClient.UpsertGroupMembers(context.Background(), role.Id, &model.GroupModifyMembers{UserIds: []string{user.Id}})
		require.Error(t, err)
		resp, err := put(t, th.SystemAdminClient, "/groups/"+role.Id+"/team_role/permissions", map[string][]string{"permissions": {model.PermissionRemoveUserFromTeam.Id}})
		require.Error(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assert.NotContains(t, explicitRoles(t, team.Id, user.Id), model.TeamRoleName(role.Id))
	})

	t.Run("deleting a role leaves its channels to its other roles, or to anyone invited", func(t *testing.T) {
		ann, bea := th.CreateUser(t), th.CreateUser(t)
		th.LinkUserToTeam(t, ann, team)
		th.LinkUserToTeam(t, bea, team)
		alpha := createRole(t, model.Group{DisplayName: "alpha"}, ann.Id)
		beta := createRole(t, model.Group{DisplayName: "beta"}, bea.Id)
		channel := th.CreatePrivateChannel(t)
		_, err := put(t, th.SystemAdminClient, "/channels/"+channel.Id+"/role_access", map[string][]string{"group_ids": {alpha.Id, beta.Id}})
		require.NoError(t, err)
		isMember := func(user *model.User) bool {
			_, appErr := th.App.GetChannelMember(th.Context, channel.Id, user.Id)
			return appErr == nil
		}
		require.True(t, isMember(ann))
		require.True(t, isMember(bea))

		_, _, err = th.SystemAdminClient.DeleteGroup(context.Background(), alpha.Id)
		require.NoError(t, err)
		assert.False(t, isMember(ann), "alpha's people leave")
		assert.True(t, isMember(bea), "beta's stay")
		updated, appErr := th.App.GetChannel(th.Context, channel.Id)
		require.Nil(t, appErr)
		assert.True(t, updated.IsGroupConstrained())

		_, _, err = th.SystemAdminClient.DeleteGroup(context.Background(), beta.Id)
		require.NoError(t, err)
		assert.True(t, isMember(bea), "the last role going doesn't empty the channel")
		updated, appErr = th.App.GetChannel(th.Context, channel.Id)
		require.Nil(t, appErr)
		assert.False(t, updated.IsGroupConstrained(), "it's for anyone invited again")
	})

	t.Run("a new role's first members are of its team", func(t *testing.T) {
		outsider := th.CreateUser(t)
		_, resp, err := createGroupWithUsers(t, th.SystemAdminClient, &model.GroupWithUserIds{
			Group:   model.Group{Name: new("firsts" + model.NewId()[:8]), DisplayName: "firsts", Source: model.GroupSourceCustom, AllowReference: true, TeamId: team.Id},
			UserIds: []string{outsider.Id},
		})
		require.Error(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("a team's lead is one of its members, and its channel one of a team", func(t *testing.T) {
		member, stranger := th.CreateUser(t), th.CreateUser(t)
		th.LinkUserToTeam(t, member, team)
		th.LinkUserToTeam(t, stranger, team)
		group, _, err := createGroupWithUsers(t, th.SystemAdminClient, &model.GroupWithUserIds{
			Group:   model.Group{Name: new("crew" + model.NewId()[:8]), DisplayName: "crew", Source: model.GroupSourceCustom, AllowReference: true, Tag: " det ", IsTeam: true, TeamId: team.Id},
			UserIds: []string{member.Id},
		})
		require.NoError(t, err)
		assert.Equal(t, "DET", group.Tag, "tags are kept in capitals")

		_, resp, err := th.SystemAdminClient.PatchGroup(context.Background(), group.Id, &model.GroupPatch{LeadId: &stranger.Id})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)
		_, _, err = th.SystemAdminClient.PatchGroup(context.Background(), group.Id, &model.GroupPatch{LeadId: &member.Id})
		require.NoError(t, err)

		dm, _, err := th.SystemAdminClient.CreateDirectChannel(context.Background(), th.SystemAdminUser.Id, member.Id)
		require.NoError(t, err)
		_, resp, err = th.SystemAdminClient.PatchGroup(context.Background(), group.Id, &model.GroupPatch{ChannelId: &dm.Id})
		require.Error(t, err)
		CheckBadRequestStatus(t, resp)

		_, _, err = th.SystemAdminClient.DeleteGroupMembers(context.Background(), group.Id, &model.GroupModifyMembers{UserIds: []string{member.Id}})
		require.NoError(t, err)
		left, _, err := th.SystemAdminClient.GetGroup(context.Background(), group.Id, "")
		require.NoError(t, err)
		assert.Empty(t, left.LeadId, "a lead who leaves is no longer its lead")
	})
}

// createGroupWithUsers creates a custom group with its first members, as the webapp does.
func createGroupWithUsers(t *testing.T, client *model.Client4, group *model.GroupWithUserIds) (*model.Group, *http.Response, error) {
	t.Helper()
	resp, err := client.DoAPIPostJSON(context.Background(), "/groups", group)
	if err != nil {
		return nil, resp, err
	}
	defer resp.Body.Close()
	var created model.Group
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))
	return &created, resp, nil
}
