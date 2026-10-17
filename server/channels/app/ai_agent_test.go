// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

// setupAgentTest gives a test server with a fake model behind a server-wide connection shared with every community.
func setupAgentTest(t *testing.T, answer string) (*TestHelper, *fakeLLM, *model.AIConnection) {
	t.Helper()
	th := setupAITest(t)
	f := newFakeLLM(t, answer)
	c := createTestAIConnection(t, th, f, model.AIConnectionScopeServer, nil)
	return th, f, c
}

// createTestAgent makes a built-in agent open to everyone in its scope, on a connection.
func createTestAgent(t *testing.T, th *TestHelper, scope, connectionID string, change func(req *model.AIAgentRequest)) *model.AIAgentDetails {
	t.Helper()
	req := &model.AIAgentRequest{
		Username:     "agent" + strings.ToLower(model.NewId()[:10]),
		DisplayName:  "Researcher",
		Description:  "Finds things out",
		Scope:        scope,
		ConnectionId: connectionID,
		Model:        "agent-model",
		SystemPrompt: "You are a careful researcher.",
		Access:       model.AIAgentAccess{Everyone: true},
	}
	if change != nil {
		change(req)
	}
	details, appErr := th.App.CreateAIAgent(th.Context, req, th.SystemAdminUser.Id)
	require.Nil(t, appErr)
	return details
}

func TestCreateAIAgent(t *testing.T) {
	th, _, c := setupAgentTest(t, "")

	t.Run("a server agent is a bot account of kind agent, owned by no one person", func(t *testing.T) {
		details := createTestAgent(t, th, model.AIAgentScopeServer, c.Id, nil)
		assert.True(t, details.Enabled)
		assert.Equal(t, c.Name, details.ConnectionName)
		assert.Equal(t, th.SystemAdminUser.Id, details.CreatorId)

		bot, appErr := th.App.GetBot(th.Context, details.UserId, false)
		require.Nil(t, appErr)
		assert.Equal(t, model.BotKindAgent, bot.Kind)
		assert.Equal(t, model.AIAgentBotOwnerId, bot.OwnerId, "an agent outlives the person who made it")
		assert.Equal(t, "Researcher", bot.DisplayName)

		user, appErr := th.App.GetUser(th.Context, details.UserId)
		require.Nil(t, appErr)
		assert.True(t, user.IsBot)
		assert.True(t, user.IsAgent)
	})

	t.Run("a community's agent joins its community", func(t *testing.T) {
		details := createTestAgent(t, th, th.BasicTeam.Id, c.Id, nil)
		member, appErr := th.App.GetTeamMember(th.Context, th.BasicTeam.Id, details.UserId)
		require.Nil(t, appErr)
		assert.Zero(t, member.DeleteAt)
	})

	t.Run("an external agent has no model settings", func(t *testing.T) {
		details := createTestAgent(t, th, model.AIAgentScopeServer, "", func(req *model.AIAgentRequest) {
			req.External = true
			req.Model = "ignored"
			req.SystemPrompt = "ignored"
		})
		assert.True(t, details.External)
		assert.Empty(t, details.ConnectionId)
		assert.Empty(t, details.Model)
		assert.Empty(t, details.SystemPrompt)
	})

	t.Run("a built-in agent needs a connection its scope may use", func(t *testing.T) {
		_, appErr := th.App.CreateAIAgent(th.Context, &model.AIAgentRequest{Username: "noconn" + strings.ToLower(model.NewId()[:8]), Scope: model.AIAgentScopeServer}, th.SystemAdminUser.Id)
		require.NotNil(t, appErr)

		f := newFakeLLM(t, "")
		community := createTestAIConnection(t, th, f, th.BasicTeam.Id, nil)
		_, appErr = th.App.CreateAIAgent(th.Context, &model.AIAgentRequest{Username: "wide" + strings.ToLower(model.NewId()[:8]), Scope: model.AIAgentScopeServer, ConnectionId: community.Id}, th.SystemAdminUser.Id)
		require.NotNil(t, appErr, "a server agent can't run on a community's connection")
		assert.Equal(t, "app.ai.agent.connection.app_error", appErr.Id)

		createTestAgent(t, th, th.BasicTeam.Id, community.Id, nil)

		other := th.CreateTeam(t)
		_, appErr = th.App.CreateAIAgent(th.Context, &model.AIAgentRequest{Username: "other" + strings.ToLower(model.NewId()[:8]), Scope: other.Id, ConnectionId: community.Id}, th.SystemAdminUser.Id)
		require.NotNil(t, appErr, "nor another community's agent on this community's connection")

		notShared := createTestAIConnection(t, th, f, model.AIConnectionScopeServer, func(req *model.AIConnectionRequest) {
			req.Sharing = model.AIConnectionSharing{TeamIDs: []string{th.BasicTeam.Id}}
		})
		createTestAgent(t, th, th.BasicTeam.Id, notShared.Id, nil)
		_, appErr = th.App.CreateAIAgent(th.Context, &model.AIAgentRequest{Username: "unshared" + strings.ToLower(model.NewId()[:8]), Scope: other.Id, ConnectionId: notShared.Id}, th.SystemAdminUser.Id)
		require.NotNil(t, appErr, "a server connection not shared with the community")
	})

	t.Run("an account that couldn't be saved leaves nothing behind", func(t *testing.T) {
		existing := createTestAgent(t, th, model.AIAgentScopeServer, c.Id, nil)
		_, appErr := th.App.CreateAIAgent(th.Context, &model.AIAgentRequest{Username: existing.Username, Scope: model.AIAgentScopeServer, ConnectionId: c.Id, Access: model.AIAgentAccess{Everyone: true}}, th.SystemAdminUser.Id)
		require.NotNil(t, appErr, "the username is taken")
	})
}

// Bots made through the REST API or an app are plain bots; the AI chat app's (the Agents plugin's) are agents.
func TestBotKindOnCreate(t *testing.T) {
	th := Setup(t).InitBasic(t)

	bot, appErr := th.App.CreateBot(th.Context, &model.Bot{Username: "plain" + strings.ToLower(model.NewId()[:8]), OwnerId: th.BasicUser.Id, Kind: model.BotKindAgent})
	require.Nil(t, appErr)
	assert.Equal(t, model.BotKindBot, bot.Kind, "nobody makes an agent by asking for the kind")

	aiChatBot, appErr := th.App.CreateBot(th.Context, &model.Bot{Username: "ai" + strings.ToLower(model.NewId()[:8]), OwnerId: model.AIAgentsPluginId})
	require.Nil(t, appErr)
	assert.Equal(t, model.BotKindAgent, aiChatBot.Kind, "the AI chat app's bots are agents from the start")
}

func TestPatchAIAgent(t *testing.T) {
	th, _, c := setupAgentTest(t, "")
	details := createTestAgent(t, th, model.AIAgentScopeServer, c.Id, nil)

	patched, appErr := th.App.PatchAIAgent(th.Context, details.AIAgent, &model.AIAgentPatch{
		DisplayName:  model.NewPointer("Scout"),
		Model:        model.NewPointer("other-model"),
		SystemPrompt: model.NewPointer("Be brief."),
		Reasoning:    &model.AIAgentReasoning{Enabled: true, Effort: "high"},
		MaxToolTurns: model.NewPointer(12),
		Access:       &model.AIAgentAccess{Roles: []string{model.SystemAdminRoleId}},
	}, th.BasicUser.Id)
	require.Nil(t, appErr)
	assert.Equal(t, "Scout", patched.DisplayName)
	assert.Equal(t, "other-model", patched.Model)
	assert.Equal(t, "Be brief.", patched.SystemPrompt)
	assert.Equal(t, "high", patched.Reasoning.Effort)
	assert.Equal(t, 12, patched.MaxToolTurns)
	assert.False(t, patched.Access.Everyone)
	assert.Equal(t, th.BasicUser.Id, patched.UpdaterId)
	assert.Equal(t, th.SystemAdminUser.Id, patched.CreatorId, "who made it stays")

	_, appErr = th.App.PatchAIAgent(th.Context, patched.AIAgent, &model.AIAgentPatch{Access: &model.AIAgentAccess{ChannelIDs: []string{model.NewId()}}}, th.BasicUser.Id)
	require.NotNil(t, appErr, "a channel that doesn't exist")
}

func TestDisableAndDeleteAIAgent(t *testing.T) {
	th, _, c := setupAgentTest(t, "")
	details := createTestAgent(t, th, model.AIAgentScopeServer, c.Id, nil)

	disabled, appErr := th.App.SetAIAgentEnabled(th.Context, details.AIAgent, false)
	require.Nil(t, appErr)
	assert.False(t, disabled.Enabled)
	activity, appErr := th.App.GetAgentActivity(th.Context, details.UserId, th.BasicUser.Id)
	require.Nil(t, appErr)
	assert.Equal(t, model.AgentStateOffline, activity.State, "a disabled agent is offline")

	enabled, appErr := th.App.SetAIAgentEnabled(th.Context, details.AIAgent, true)
	require.Nil(t, appErr)
	assert.True(t, enabled.Enabled)

	require.Nil(t, th.App.DeleteAIAgent(th.Context, details.AIAgent))
	_, appErr = th.App.GetAIAgent(th.Context, details.UserId)
	require.NotNil(t, appErr)
	user, appErr := th.App.GetUser(th.Context, details.UserId)
	require.Nil(t, appErr)
	assert.NotZero(t, user.DeleteAt, "its account is deactivated")
	assert.True(t, user.IsAgent, "and its messages still show an agent's")
}

// Who may talk to an agent: its scope, its access, and guests only when it allows them.
func TestAIAgentAccess(t *testing.T) {
	th, _, c := setupAgentTest(t, "")
	guest := th.CreateGuest(t)
	th.LinkUserToTeam(t, guest, th.BasicTeam)
	outsider := th.CreateUser(t)
	otherTeam := th.CreateTeam(t)
	th.LinkUserToTeam(t, outsider, otherTeam)

	allows := func(agent *model.AIAgent, user *model.User, channel *model.Channel) bool {
		return th.App.aiAgentAllows(th.Context, agent, user, channel) == nil
	}

	everyone := createTestAgent(t, th, model.AIAgentScopeServer, c.Id, nil).AIAgent
	assert.True(t, allows(everyone, th.BasicUser, th.BasicChannel))
	assert.True(t, allows(everyone, outsider, nil))
	assert.False(t, allows(everyone, guest, th.BasicChannel), "guests only when allowed")

	withGuests := createTestAgent(t, th, model.AIAgentScopeServer, c.Id, func(req *model.AIAgentRequest) { req.Access.AllowGuests = true }).AIAgent
	assert.True(t, allows(withGuests, guest, th.BasicChannel))

	community := createTestAgent(t, th, th.BasicTeam.Id, c.Id, nil).AIAgent
	assert.True(t, allows(community, th.BasicUser, th.BasicChannel))
	assert.True(t, allows(community, th.BasicUser, nil), "its members, in direct messages too")
	assert.False(t, allows(community, outsider, nil), "not someone outside its community")
	otherChannel := th.CreateChannel(t, otherTeam)
	assert.False(t, allows(community, th.BasicUser, otherChannel), "nor in another community's channel")

	private := th.CreatePrivateChannel(t, th.BasicTeam)
	members := createTestAgent(t, th, model.AIAgentScopeServer, c.Id, func(req *model.AIAgentRequest) {
		req.Access = model.AIAgentAccess{ChannelIDs: []string{private.Id}}
	}).AIAgent
	assert.True(t, allows(members, th.BasicUser, th.BasicChannel), "the channel's members")
	assert.False(t, allows(members, th.BasicUser2, th.BasicChannel))

	admins := createTestAgent(t, th, model.AIAgentScopeServer, c.Id, func(req *model.AIAgentRequest) {
		req.Access = model.AIAgentAccess{Roles: []string{model.SystemAdminRoleId}}
	}).AIAgent
	assert.True(t, allows(admins, th.SystemAdminUser, nil))
	assert.False(t, allows(admins, th.BasicUser, nil))

	people := createTestAgent(t, th, model.AIAgentScopeServer, c.Id, func(req *model.AIAgentRequest) {
		req.Access = model.AIAgentAccess{UserIDs: []string{th.BasicUser2.Id}, TeamIDs: []string{otherTeam.Id}}
	}).AIAgent
	assert.True(t, allows(people, th.BasicUser2, nil))
	assert.True(t, allows(people, outsider, nil), "the chosen community's members")
	assert.False(t, allows(people, th.BasicUser, nil))

	bot := th.CreateBot(t)
	botUser, appErr := th.App.GetUser(th.Context, bot.UserId)
	require.Nil(t, appErr)
	assert.False(t, allows(everyone, botUser, nil), "agents talk with people")
}

// /api/v4/agents keeps its shape for older apps, now from the server's agents: the built-in, enabled ones a person
// may talk to, their connection as their service.
func TestGetAgentsFromServerAgents(t *testing.T) {
	th, _, c := setupAgentTest(t, "")
	researcher := createTestAgent(t, th, model.AIAgentScopeServer, c.Id, nil)
	createTestAgent(t, th, model.AIAgentScopeServer, "", func(req *model.AIAgentRequest) { req.External = true })
	admins := createTestAgent(t, th, model.AIAgentScopeServer, c.Id, func(req *model.AIAgentRequest) {
		req.Access = model.AIAgentAccess{Roles: []string{model.SystemAdminRoleId}}
	})
	disabled := createTestAgent(t, th, model.AIAgentScopeServer, c.Id, nil)
	_, appErr := th.App.SetAIAgentEnabled(th.Context, disabled.AIAgent, false)
	require.Nil(t, appErr)

	agents, appErr := th.App.GetAgents(th.Context, th.BasicUser.Id)
	require.Nil(t, appErr)
	require.Len(t, agents, 1, "not the external one, nor the admins', nor the disabled one")
	assert.Equal(t, researcher.UserId, agents[0].ID)
	assert.Equal(t, researcher.Username, agents[0].Username)
	assert.Equal(t, c.Id, agents[0].ServiceID)
	assert.Equal(t, model.AIProviderOpenAICompatible, agents[0].ServiceType)

	agents, appErr = th.App.GetAgents(th.Context, th.SystemAdminUser.Id)
	require.Nil(t, appErr)
	ids := []string{}
	for _, a := range agents {
		ids = append(ids, a.ID)
	}
	assert.ElementsMatch(t, []string{researcher.UserId, admins.UserId}, ids)

	available, _ := th.App.GetAgentsStatus(th.Context, th.BasicUser.Id)
	assert.True(t, available)
	nobody := th.CreateGuest(t)
	available, reason := th.App.GetAgentsStatus(th.Context, nobody.Id)
	assert.False(t, available)
	assert.Equal(t, "app.agents.status.no_agents", reason)

	users, appErr := th.App.GetUsersForAgents(th.Context, th.BasicUser.Id)
	require.Nil(t, appErr)
	require.Len(t, users, 1)
	assert.True(t, users[0].IsAgent, "the users autocomplete's agents")

	services, appErr := th.App.GetLLMServices(th.Context, th.BasicUser.Id)
	require.Nil(t, appErr)
	require.NotEmpty(t, services)
	data, err := json.Marshal(services)
	require.NoError(t, err)
	assert.NotContains(t, string(data), aiTestKey, "never a key")
}

// Who manages agents: system admins the server's, a community's admins their community's, with manage_own_agent
// for their own and manage_others_agent for the others'.
func TestSessionCanManageAIAgent(t *testing.T) {
	th, _, c := setupAgentTest(t, "")
	teamAdmin := th.CreateUser(t)
	th.LinkUserToTeam(t, teamAdmin, th.BasicTeam)
	_, appErr := th.App.UpdateTeamMemberSchemeRoles(th.Context, th.BasicTeam.Id, teamAdmin.Id, false, true, true)
	require.Nil(t, appErr)
	otherAdmin := th.CreateUser(t)
	otherTeam := th.CreateTeam(t)
	th.LinkUserToTeam(t, otherAdmin, otherTeam)
	_, appErr = th.App.UpdateTeamMemberSchemeRoles(th.Context, otherTeam.Id, otherAdmin.Id, false, true, true)
	require.Nil(t, appErr)

	session := func(user *model.User) model.Session {
		s, appErr := th.App.CreateSession(th.Context, &model.Session{UserId: user.Id, Roles: user.GetRawRoles()})
		require.Nil(t, appErr)
		loaded, appErr := th.App.GetSession(s.Token)
		require.Nil(t, appErr)
		return *loaded
	}
	admin := session(th.SystemAdminUser)
	communityAdmin := session(teamAdmin)
	elsewhereAdmin := session(otherAdmin)
	member := session(th.BasicUser)

	server := createTestAgent(t, th, model.AIAgentScopeServer, c.Id, nil).AIAgent
	community := createTestAgent(t, th, th.BasicTeam.Id, c.Id, nil).AIAgent

	assert.True(t, th.App.SessionCanManageAIAgent(admin, server))
	assert.True(t, th.App.SessionCanManageAIAgent(admin, community))
	assert.False(t, th.App.SessionCanManageAIAgent(communityAdmin, server), "a community admin doesn't manage server agents")
	assert.True(t, th.App.SessionCanManageAIAgent(communityAdmin, community), "but its community's, whoever made them")
	assert.False(t, th.App.SessionCanManageAIAgent(elsewhereAdmin, community), "not another community's")
	assert.False(t, th.App.SessionCanManageAIAgent(member, community))
	assert.False(t, th.App.SessionCanManageAIAgent(member, server))

	assert.True(t, th.App.SessionCanCreateAIAgent(admin, model.AIAgentScopeServer))
	assert.False(t, th.App.SessionCanCreateAIAgent(communityAdmin, model.AIAgentScopeServer))
	assert.True(t, th.App.SessionCanCreateAIAgent(communityAdmin, th.BasicTeam.Id))
	assert.False(t, th.App.SessionCanCreateAIAgent(communityAdmin, otherTeam.Id))
	assert.False(t, th.App.SessionCanCreateAIAgent(member, th.BasicTeam.Id), "manage_own_agent alone doesn't make agents")

	// Without manage_others_agent, a community admin keeps only the agents they made.
	th.RemovePermissionFromRole(t, model.PermissionManageOthersAgent.Id, model.TeamAdminRoleId)
	defer th.AddPermissionToRole(t, model.PermissionManageOthersAgent.Id, model.TeamAdminRoleId)
	assert.False(t, th.App.SessionCanManageAIAgent(communityAdmin, community))
	own := community.Clone()
	own.CreatorId = teamAdmin.Id
	assert.True(t, th.App.SessionCanManageAIAgent(communityAdmin, own))

	assert.True(t, th.App.SessionCanManageAIAgentUser(th.Context, admin, server.UserId))
	assert.False(t, th.App.SessionCanManageAIAgentUser(th.Context, admin, th.BasicUser.Id), "a person isn't an agent")
}

func TestAIAgentPermissionsDefaults(t *testing.T) {
	roles := model.MakeDefaultRoles()
	assert.Contains(t, roles[model.TeamAdminRoleId].Permissions, model.PermissionManageOwnAgent.Id)
	assert.Contains(t, roles[model.TeamAdminRoleId].Permissions, model.PermissionManageOthersAgent.Id)
	assert.NotContains(t, roles[model.TeamUserRoleId].Permissions, model.PermissionManageOthersAgent.Id)
	assert.NotContains(t, roles[model.SystemUserRoleId].Permissions, model.PermissionManageOthersAgent.Id)
	assert.NotContains(t, roles[model.TeamGuestRoleId].Permissions, model.PermissionManageOwnAgent.Id)
}
