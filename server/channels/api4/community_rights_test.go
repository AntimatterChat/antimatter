// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/app"
)

// Antimatter: who may do what in a community by default, by kind of community: archiving channels, putting messages
// on top, the presets' moderators and the roles with Administrator on. A regression here hands out (or takes away)
// power on every server, so each case is spelt out.

type rightsHelper struct {
	th   *TestHelper
	team *model.Team
}

func newRightsHelper(t *testing.T, kind string) *rightsHelper {
	th := Setup(t).InitBasic(t)
	team := th.CreateTeamWithClient(t, th.SystemAdminClient)
	if kind != "" {
		patched, _, err := th.SystemAdminClient.PatchTeam(context.Background(), team.Id, &model.TeamPatch{Kind: model.NewPointer(kind)})
		require.NoError(t, err)
		require.Equal(t, kind, patched.Kind)
		team = patched
	}
	return &rightsHelper{th: th, team: team}
}

// member is someone of the community, logged in.
func (h *rightsHelper) member(t *testing.T) (*model.User, *model.Client4) {
	t.Helper()
	user := h.th.CreateUser(t)
	h.th.LinkUserToTeam(t, user, h.team)
	client := h.th.CreateClient()
	_, _, err := client.Login(context.Background(), user.Email, user.Password)
	require.NoError(t, err)
	return user, client
}

// admin is one of the community's admins, logged in.
func (h *rightsHelper) admin(t *testing.T) (*model.User, *model.Client4) {
	t.Helper()
	user := h.th.CreateUser(t)
	h.th.LinkUserToTeam(t, user, h.team)
	_, appErr := h.th.App.UpdateTeamMemberSchemeRoles(h.th.Context, h.team.Id, user.Id, false, true, true)
	require.Nil(t, appErr)
	client := h.th.CreateClient()
	_, _, err := client.Login(context.Background(), user.Email, user.Password)
	require.NoError(t, err)
	return user, client
}

// withRole is a member holding a new role of the community that grants permissions.
func (h *rightsHelper) withRole(t *testing.T, permissions ...string) (*model.User, *model.Client4, *model.Group) {
	t.Helper()
	id := model.NewId()[:8]
	role, _, err := h.th.SystemAdminClient.CreateGroup(context.Background(), &model.Group{
		Name: new("role" + id), DisplayName: "Moderators " + id, Source: model.GroupSourceCustom, AllowReference: true, TeamId: h.team.Id,
	})
	require.NoError(t, err)
	h.grant(t, role, permissions...)
	user, client := h.member(t)
	_, _, err = h.th.SystemAdminClient.UpsertGroupMembers(context.Background(), role.Id, &model.GroupModifyMembers{UserIds: []string{user.Id}})
	require.NoError(t, err)
	return user, client, role
}

func (h *rightsHelper) grant(t *testing.T, role *model.Group, permissions ...string) {
	t.Helper()
	body, err := json.Marshal(map[string][]string{"permissions": permissions})
	require.NoError(t, err)
	resp, err := h.th.SystemAdminClient.DoAPIPut(context.Background(), "/groups/"+role.Id+"/team_role/permissions", string(body))
	require.NoError(t, err)
	resp.Body.Close()
}

func (h *rightsHelper) rolePermissions(t *testing.T, role *model.Group) []string {
	t.Helper()
	resp, err := h.th.SystemAdminClient.DoAPIGet(context.Background(), "/groups/"+role.Id+"/team_role", "")
	require.NoError(t, err)
	defer resp.Body.Close()
	var r model.Role
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&r))
	return r.Permissions
}

// channel is a channel of the community created by client (who is then its admin), with the people given in it.
func (h *rightsHelper) channel(t *testing.T, client *model.Client4, channelType model.ChannelType, users ...*model.User) *model.Channel {
	t.Helper()
	channel, _, err := client.CreateChannel(context.Background(), &model.Channel{TeamId: h.team.Id, Name: GenerateTestChannelName(), DisplayName: "Channel", Type: channelType})
	require.NoError(t, err)
	for _, u := range users {
		h.th.AddUserToChannel(t, u, channel)
	}
	return channel
}

func (h *rightsHelper) archived(t *testing.T, channelID string) bool {
	t.Helper()
	channel, appErr := h.th.App.GetChannel(h.th.Context, channelID)
	require.Nil(t, appErr)
	return channel.DeleteAt != 0
}

func TestCommunityKind(t *testing.T) {
	mainHelper.Parallel(t)
	h := newRightsHelper(t, "")
	ctx := context.Background()

	assert.Equal(t, model.TeamKindCommunity, h.team.Kind, "a new community is of the community kind")

	_, member := h.member(t)
	_, resp, err := member.PatchTeam(ctx, h.team.Id, &model.TeamPatch{Kind: model.NewPointer(model.TeamKindWorkplace)})
	require.Error(t, err, "members don't change the kind")
	CheckForbiddenStatus(t, resp)

	_, resp, err = h.th.SystemAdminClient.PatchTeam(ctx, h.team.Id, &model.TeamPatch{Kind: model.NewPointer("castle")})
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	_, admin := h.admin(t)
	patched, _, err := admin.PatchTeam(ctx, h.team.Id, &model.TeamPatch{Kind: model.NewPointer(model.TeamKindWorkplace)})
	require.NoError(t, err, "the community's admins change it")
	assert.Equal(t, model.TeamKindWorkplace, patched.Kind)
	fetched, _, err := member.GetTeam(ctx, h.team.Id, "")
	require.NoError(t, err)
	assert.Equal(t, model.TeamKindWorkplace, fetched.Kind, "everyone sees it")

	patched, _, err = admin.PatchTeam(ctx, h.team.Id, &model.TeamPatch{Kind: model.NewPointer(model.TeamKindCommunity)})
	require.NoError(t, err)
	assert.Equal(t, model.TeamKindCommunity, patched.Kind)
}

func TestArchiveChannelRights(t *testing.T) {
	mainHelper.Parallel(t)
	ctx := context.Background()

	t.Run("in a community, members don't archive channels, even those they created", func(t *testing.T) {
		h := newRightsHelper(t, model.TeamKindCommunity)
		user, client := h.member(t)
		other, _ := h.member(t)
		for _, channelType := range []model.ChannelType{model.ChannelTypeOpen, model.ChannelTypePrivate} {
			mine := h.channel(t, client, channelType)
			resp, err := client.DeleteChannel(ctx, mine.Id)
			require.Error(t, err, "not their own (%s)", channelType)
			CheckForbiddenStatus(t, resp)

			theirs := h.channel(t, h.th.SystemAdminClient, channelType, user, other)
			resp, err = client.DeleteChannel(ctx, theirs.Id)
			require.Error(t, err, "nor others' (%s)", channelType)
			CheckForbiddenStatus(t, resp)
			assert.False(t, h.archived(t, mine.Id))
			assert.False(t, h.archived(t, theirs.Id))
		}
	})

	t.Run("in a community, its admins, system admins and moderators who manage channels archive them", func(t *testing.T) {
		h := newRightsHelper(t, model.TeamKindCommunity)
		user, client := h.member(t)
		_, admin := h.admin(t)
		_, manager, _ := h.withRole(t, model.PermissionManagePublicChannelProperties.Id, model.PermissionManagePrivateChannelProperties.Id,
			model.PermissionDeletePublicChannel.Id, model.PermissionDeletePrivateChannel.Id)
		_, moderator, _ := h.withRole(t, app.ModeratorPresetPermissions()...)

		for _, channelType := range []model.ChannelType{model.ChannelTypeOpen, model.ChannelTypePrivate} {
			channel := h.channel(t, client, channelType, user)
			resp, err := moderator.DeleteChannel(ctx, channel.Id)
			require.Error(t, err, "the presets' moderators don't archive channels (%s)", channelType)
			CheckForbiddenStatus(t, resp)

			_, err = admin.DeleteChannel(ctx, channel.Id)
			require.NoError(t, err, "the community's admins archive any channel, without being in it (%s)", channelType)
			assert.True(t, h.archived(t, channel.Id))

			channel = h.channel(t, client, channelType, user)
			_, err = manager.DeleteChannel(ctx, channel.Id)
			require.NoError(t, err, "a role with Manage channels archives (%s)", channelType)

			channel = h.channel(t, client, channelType, user)
			_, err = h.th.SystemAdminClient.DeleteChannel(ctx, channel.Id)
			require.NoError(t, err, "system admins archive (%s)", channelType)
		}
	})

	t.Run("in a workplace community, members archive the channels they created, not the others", func(t *testing.T) {
		h := newRightsHelper(t, model.TeamKindWorkplace)
		user, client := h.member(t)
		other, otherClient := h.member(t)
		for _, channelType := range []model.ChannelType{model.ChannelTypeOpen, model.ChannelTypePrivate} {
			mine := h.channel(t, client, channelType, other)
			resp, err := otherClient.DeleteChannel(ctx, mine.Id)
			require.Error(t, err, "a member who didn't create it doesn't (%s)", channelType)
			CheckForbiddenStatus(t, resp)

			_, err = client.DeleteChannel(ctx, mine.Id)
			require.NoError(t, err, "its creator does (%s)", channelType)
			assert.True(t, h.archived(t, mine.Id))

			theirs := h.channel(t, h.th.SystemAdminClient, channelType, user)
			resp, err = client.DeleteChannel(ctx, theirs.Id)
			require.Error(t, err, "nor another's channel (%s)", channelType)
			CheckForbiddenStatus(t, resp)

			// Someone the channel's admins made one of them archives it too.
			_, appErr := h.th.App.UpdateChannelMemberSchemeRoles(h.th.Context, theirs.Id, user.Id, false, true, true)
			require.Nil(t, appErr)
			_, err = client.DeleteChannel(ctx, theirs.Id)
			require.NoError(t, err, "the channel's admins do (%s)", channelType)
		}
	})

	t.Run("in a workplace community, the default channels stay with its admins", func(t *testing.T) {
		h := newRightsHelper(t, model.TeamKindWorkplace)
		user, client := h.member(t)
		townSquare, appErr := h.th.App.GetChannelByName(h.th.Context, model.DefaultChannelName, h.team.Id, false)
		require.Nil(t, appErr)
		_, appErr = h.th.App.UpdateChannelMemberSchemeRoles(h.th.Context, townSquare.Id, user.Id, false, true, true)
		require.Nil(t, appErr)
		resp, err := client.DeleteChannel(ctx, townSquare.Id)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("a member role a system admin gave archiving to still archives", func(t *testing.T) {
		h := newRightsHelper(t, model.TeamKindCommunity)
		defaults := h.th.SaveDefaultRolePermissions(t)
		defer h.th.RestoreDefaultRolePermissions(t, defaults)
		h.th.AddPermissionToRole(t, model.PermissionDeletePublicChannel.Id, model.ChannelUserRoleId)

		user, client := h.member(t)
		channel := h.channel(t, h.th.SystemAdminClient, model.ChannelTypeOpen, user)
		_, err := client.DeleteChannel(ctx, channel.Id)
		require.NoError(t, err)
	})
}

func TestOnTopRights(t *testing.T) {
	mainHelper.Parallel(t)
	ctx := context.Background()

	for _, kind := range []string{model.TeamKindCommunity, model.TeamKindWorkplace} {
		t.Run("in a "+kind+" community", func(t *testing.T) {
			h := newRightsHelper(t, kind)
			user, client := h.member(t)
			channelAdmin, channelAdminClient := h.member(t)
			admin, adminClient := h.admin(t)
			moderator, moderatorClient, _ := h.withRole(t, app.ModeratorPresetPermissions()...)
			channel := h.channel(t, h.th.SystemAdminClient, model.ChannelTypeOpen, user, channelAdmin, admin, moderator)
			_, appErr := h.th.App.UpdateChannelMemberSchemeRoles(h.th.Context, channel.Id, channelAdmin.Id, false, true, true)
			require.Nil(t, appErr)
			post := h.th.CreatePostWithClient(t, client, channel)

			_, resp, err := client.PutPostOnTop(ctx, post.Id, 0)
			require.Error(t, err, "members don't put messages on top")
			CheckForbiddenStatus(t, resp)

			// Created by a member, a channel is theirs to admin: they put messages on top of it.
			own := h.channel(t, client, model.ChannelTypeOpen)
			ownPost := h.th.CreatePostWithClient(t, client, own)
			_, _, err = client.PutPostOnTop(ctx, ownPost.Id, 0)
			require.NoError(t, err, "the channel's admins do")

			for name, c := range map[string]*model.Client4{"the channel's admins": channelAdminClient, "the community's admins": adminClient, "its moderators": moderatorClient, "system admins": h.th.SystemAdminClient} {
				_, _, err = c.PutPostOnTop(ctx, post.Id, time.Hour.Milliseconds())
				require.NoError(t, err, "%s put messages on top", name)
				resp, err = client.RemovePostFromTop(ctx, post.Id)
				require.Error(t, err, "members don't take them off")
				CheckForbiddenStatus(t, resp)
				_, err = c.RemovePostFromTop(ctx, post.Id)
				require.NoError(t, err, "%s take them off", name)
			}
		})
	}

	t.Run("in a direct message, its members do", func(t *testing.T) {
		h := newRightsHelper(t, "")
		dm := h.th.CreateDmChannel(t, h.th.BasicUser2)
		post := h.th.CreatePostWithClient(t, h.th.Client, dm)
		_, _, err := h.th.Client.PutPostOnTop(ctx, post.Id, 0)
		require.NoError(t, err)
	})
}

func TestArchivedChannelIsLeftAlone(t *testing.T) {
	mainHelper.Parallel(t)
	h := newRightsHelper(t, "")
	ctx := context.Background()
	user, _ := h.member(t)
	channel := h.channel(t, h.th.SystemAdminClient, model.ChannelTypeOpen, user)
	post := h.th.CreatePostWithClient(t, h.th.SystemAdminClient, channel)
	_, err := h.th.SystemAdminClient.DeleteChannel(ctx, channel.Id)
	require.NoError(t, err)
	admin := h.th.SystemAdminClient

	_, resp, err := admin.PutPostOnTop(ctx, post.Id, 0)
	require.Error(t, err, "nothing goes on top")
	CheckBadRequestStatus(t, resp)

	_, resp, err = admin.UpdateChannelPrivacy(ctx, channel.Id, model.ChannelTypePrivate)
	require.Error(t, err, "it isn't made private")
	CheckBadRequestStatus(t, resp)

	resp, err = admin.UpdateChannelRoles(ctx, channel.Id, user.Id, model.ChannelUserRoleId+" "+model.ChannelAdminRoleId)
	require.Error(t, err, "nobody is made its admin")
	CheckBadRequestStatus(t, resp)

	body, err := json.Marshal(model.ChannelOverride{TargetType: model.ChannelOverrideTargetUser, TargetId: user.Id, Deny: []string{model.PermissionCreatePost.Id}})
	require.NoError(t, err)
	httpResp, err := admin.DoAPIPut(ctx, "/channels/"+channel.Id+"/overrides", string(body))
	require.Error(t, err, "its permissions stay")
	assert.Equal(t, http.StatusBadRequest, httpResp.StatusCode)

	httpResp, err = admin.DoAPIPut(ctx, "/channels/"+channel.Id+"/role_access", `{"group_ids":[]}`)
	require.Error(t, err, "who sees it stays")
	assert.Equal(t, http.StatusBadRequest, httpResp.StatusCode)

	_, resp, err = admin.PatchChannel(ctx, channel.Id, &model.ChannelPatch{DisplayName: model.NewPointer("renamed")})
	require.Error(t, err, "it isn't renamed")
	CheckBadRequestStatus(t, resp)

	require.NoError(t, h.th.App.SetPhase2PermissionsMigrationStatus(true))
	scheme, _, err := admin.CreateScheme(ctx, &model.Scheme{DisplayName: "Archive", Name: model.NewId(), Scope: model.SchemeScopeChannel})
	require.NoError(t, err)
	_, err = admin.UpdateChannelScheme(ctx, channel.Id, scheme.Id)
	require.Error(t, err, "its scheme stays")

	resp, err = admin.PinPost(ctx, post.Id)
	require.Error(t, err, "nothing is pinned")
	CheckBadRequestStatus(t, resp)

	// It still takes new members, to read it (an auditor going through old information), and stays read-only.
	auditor, auditorClient := h.member(t)
	_, _, err = admin.AddChannelMember(ctx, channel.Id, auditor.Id)
	require.NoError(t, err, "an archived channel still takes new members")
	posts, _, err := auditorClient.GetPostsForChannel(ctx, channel.Id, 0, 60, "", false, false)
	require.NoError(t, err, "they read its history")
	assert.Contains(t, posts.Order, post.Id)
	_, _, err = auditorClient.CreatePost(ctx, &model.Post{ChannelId: channel.Id, Message: "late"})
	require.Error(t, err, "nobody posts in it")
	_, _, err = auditorClient.SaveReaction(ctx, &model.Reaction{UserId: auditor.Id, PostId: post.Id, EmojiName: "smile"})
	require.Error(t, err, "nor reacts")
}

func TestModeratorPresetAndAdministratorRoles(t *testing.T) {
	mainHelper.Parallel(t)
	ctx := context.Background()

	t.Run("the presets' moderators time people out and remove them, below them", func(t *testing.T) {
		h := newRightsHelper(t, "")
		_, moderator, _ := h.withRole(t, app.ModeratorPresetPermissions()...)
		target, _ := h.member(t)
		admin, _ := h.admin(t)

		data, err := json.Marshal(model.MemberTimeoutRequest{Until: model.GetMillis() + time.Hour.Milliseconds(), Reason: "flooding"})
		require.NoError(t, err)
		httpResp, err := moderator.DoAPIPut(ctx, "/teams/"+h.team.Id+"/members/"+target.Id+"/timeout", string(data))
		require.NoError(t, err, "a moderator times a member out")
		httpResp.Body.Close()

		httpResp, err = moderator.DoAPIPut(ctx, "/teams/"+h.team.Id+"/members/"+admin.Id+"/timeout", string(data))
		require.Error(t, err, "not the community's admins")
		assert.Equal(t, http.StatusForbidden, httpResp.StatusCode)

		_, err = moderator.RemoveTeamMember(ctx, h.team.Id, target.Id)
		require.NoError(t, err, "a moderator removes a member")
		resp, err := moderator.RemoveTeamMember(ctx, h.team.Id, admin.Id)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("Administrator grants what the community's admins may do, nothing system-wide", func(t *testing.T) {
		h := newRightsHelper(t, "")
		holder, holderClient, role := h.withRole(t, model.PermissionManageTeam.Id)

		permissions := h.rolePermissions(t, role)
		for _, p := range []*model.Permission{
			model.PermissionManageTeam, model.PermissionManageTeamRoles, model.PermissionRemoveUserFromTeam,
			model.PermissionManageChannelRoles, model.PermissionDeleteOthersPosts,
			model.PermissionDeletePublicChannel, model.PermissionDeletePrivateChannel,
			model.PermissionConvertPublicChannelToPrivate, model.PermissionManageOthersIncomingWebhooks,
			model.PermissionManagePublicChannelBanner, model.PermissionManagePrivateChannelBanner,
			model.PermissionManageChannelJoinRequests, model.PermissionManagePrivateChannelDiscoverability,
			model.PermissionManagePublicChannelAutoTranslation, model.PermissionManagePrivateChannelAutoTranslation,
		} {
			assert.Contains(t, permissions, p.Id)
		}
		for _, p := range []*model.Permission{model.PermissionBypassIncomingWebhookChannelLock, model.PermissionManageChannelAccessRules, model.PermissionManageTeamAccessRules} {
			assert.NotContains(t, permissions, p.Id, "the community's admins keep it")
		}
		for _, p := range permissions {
			perm := slices.IndexFunc(model.AllPermissions, func(x *model.Permission) bool { return x.Id == p })
			require.NotEqual(t, -1, perm, p)
			assert.NotEqual(t, model.PermissionScopeSystem, model.AllPermissions[perm].Scope, "%s is a system permission", p)
		}
		expected, appErr := h.th.App.TeamAdminRolePermissions(h.th.Context, h.team.Id)
		require.Nil(t, appErr)
		for _, p := range expected {
			assert.Contains(t, permissions, p)
		}
		for _, p := range permissions {
			assert.True(t, slices.Contains(model.TeamRolePermissions(), p), "%s isn't a permission a role may hold", p)
		}
		assert.NotContains(t, permissions, model.PermissionManageSystem.Id)
		assert.NotContains(t, permissions, model.PermissionManageRoles.Id)

		// They act as the community's admins: on others' messages and channels they aren't in.
		user, client := h.member(t)
		channel := h.channel(t, client, model.ChannelTypeOpen, holder)
		post := h.th.CreatePostWithClient(t, client, channel)
		_, err := holderClient.DeletePost(ctx, post.Id)
		require.NoError(t, err)
		_, err = holderClient.DeleteChannel(ctx, h.channel(t, client, model.ChannelTypePrivate, user).Id)
		require.NoError(t, err)

		// Without it, a role holds what it's given and nothing more.
		h.grant(t, role, model.PermissionDeleteOthersPosts.Id)
		assert.Equal(t, []string{model.PermissionDeleteOthersPosts.Id}, h.rolePermissions(t, role))
	})
}

// Antimatter: a community's admins choose how its member list groups people, or let each member choose.
func TestCommunityMemberListView(t *testing.T) {
	mainHelper.Parallel(t)
	h := newRightsHelper(t, "")
	ctx := context.Background()
	assert.Equal(t, model.TeamMemberListViewChoice, h.team.MemberListView, "members choose by default")

	_, member := h.member(t)
	_, resp, err := member.PatchTeam(ctx, h.team.Id, &model.TeamPatch{MemberListView: model.NewPointer(model.TeamMemberListViewRoles)})
	require.Error(t, err, "members don't set it")
	CheckForbiddenStatus(t, resp)

	_, resp, err = h.th.SystemAdminClient.PatchTeam(ctx, h.team.Id, &model.TeamPatch{MemberListView: model.NewPointer("status")})
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)

	_, admin := h.admin(t)
	for _, view := range []string{model.TeamMemberListViewTeams, model.TeamMemberListViewRoles, model.TeamMemberListViewChoice} {
		patched, _, err := admin.PatchTeam(ctx, h.team.Id, &model.TeamPatch{MemberListView: model.NewPointer(view)})
		require.NoError(t, err, view)
		assert.Equal(t, view, patched.MemberListView)
		fetched, _, err := member.GetTeam(ctx, h.team.Id, "")
		require.NoError(t, err)
		assert.Equal(t, view, fetched.MemberListView, "members see it")
	}

	// Patching something else keeps it.
	_, _, err = admin.PatchTeam(ctx, h.team.Id, &model.TeamPatch{MemberListView: model.NewPointer(model.TeamMemberListViewTeams)})
	require.NoError(t, err)
	patched, _, err := admin.PatchTeam(ctx, h.team.Id, &model.TeamPatch{Description: model.NewPointer("about")})
	require.NoError(t, err)
	assert.Equal(t, model.TeamMemberListViewTeams, patched.MemberListView)
	assert.Equal(t, model.TeamKindCommunity, patched.Kind)
}

// Antimatter: communities made through the API are of the server's default kind, unless created with one.
func TestCreateTeamKind(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	ctx := context.Background()
	newTeam := func(kind string) *model.Team {
		return &model.Team{DisplayName: "Kind", Name: GenerateTestTeamName(), Email: th.GenerateTestEmail(), Type: model.TeamOpen, Kind: kind}
	}

	team, _, err := th.Client.CreateTeam(ctx, newTeam(""))
	require.NoError(t, err)
	assert.Equal(t, model.TeamKindCommunity, team.Kind)

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.TeamSettings.DefaultCommunityKind = model.TeamKindWorkplace })
	team, _, err = th.Client.CreateTeam(ctx, newTeam(""))
	require.NoError(t, err)
	assert.Equal(t, model.TeamKindWorkplace, team.Kind)
	fetched, _, err := th.Client.GetTeam(ctx, team.Id, "")
	require.NoError(t, err)
	assert.Equal(t, model.TeamKindWorkplace, fetched.Kind)

	team, _, err = th.Client.CreateTeam(ctx, newTeam(model.TeamKindCommunity))
	require.NoError(t, err)
	assert.Equal(t, model.TeamKindCommunity, team.Kind, "the kind asked for wins")

	_, resp, err := th.Client.CreateTeam(ctx, newTeam("castle"))
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)
}
