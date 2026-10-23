// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/utils/testutils"
)

func aiAgentRequest(scope, connectionID string) *model.AIAgentRequest {
	return &model.AIAgentRequest{
		Username:     "agent" + strings.ToLower(model.NewId()[:10]),
		DisplayName:  "Researcher",
		Scope:        scope,
		ConnectionId: connectionID,
		Model:        "test-model",
		SystemPrompt: "Be careful.",
		Access:       model.AIAgentAccess{Everyone: true},
	}
}

// setupAIAgentsAPI gives the clients of the connections' tests and a server-wide connection shared with every
// community, whose key the answers must never carry.
func setupAIAgentsAPI(t *testing.T) (*TestHelper, aiTestClients, *model.AIConnection) {
	th, clients := setupAIConnectionsAPI(t)
	endpoint := newAIEndpoint(t)
	req := aiConnectionRequest(model.AIConnectionScopeServer, endpoint.URL)
	req.Sharing = model.AIConnectionSharing{AllTeams: true}
	connection, resp, err := clients.systemAdmin.CreateAIConnection(context.Background(), req)
	require.NoError(t, err)
	CheckCreatedStatus(t, resp)
	return th, clients, connection
}

// The permission matrix: system admins manage server agents; a community's admins their community's (theirs and
// the others'); nobody else manages any.
func TestAIAgentsPermissions(t *testing.T) {
	th, clients, connection := setupAIAgentsAPI(t)
	ctx := context.Background()

	t.Run("server agents: system admins only", func(t *testing.T) {
		for name, client := range map[string]*model.Client4{"community admin": clients.communityAdmin, "member": clients.member, "guest": clients.guest} {
			_, resp, err := client.CreateAIAgent(ctx, aiAgentRequest(model.AIAgentScopeServer, connection.Id))
			require.Error(t, err, name)
			CheckForbiddenStatus(t, resp)
			_, resp, err = client.GetAIAgents(ctx, model.AIAgentScopeServer)
			require.Error(t, err, name)
			CheckForbiddenStatus(t, resp)
		}

		created, resp, err := clients.systemAdmin.CreateAIAgent(ctx, aiAgentRequest(model.AIAgentScopeServer, connection.Id))
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
		assert.Equal(t, model.AIAgentScopeServer, created.Scope)
		assert.True(t, created.Enabled)

		for name, client := range map[string]*model.Client4{"community admin": clients.communityAdmin, "member": clients.member, "guest": clients.guest} {
			_, resp, err = client.GetAIAgent(ctx, created.UserId)
			require.Error(t, err, name)
			CheckForbiddenStatus(t, resp)
			_, resp, err = client.PatchAIAgent(ctx, created.UserId, &model.AIAgentPatch{DisplayName: model.NewPointer("Taken")})
			require.Error(t, err, name)
			CheckForbiddenStatus(t, resp)
			_, resp, err = client.SetAIAgentEnabled(ctx, created.UserId, false)
			require.Error(t, err, name)
			CheckForbiddenStatus(t, resp)
			resp, err = client.DeleteAIAgent(ctx, created.UserId)
			require.Error(t, err, name)
			CheckForbiddenStatus(t, resp)
		}

		patched, _, err := clients.systemAdmin.PatchAIAgent(ctx, created.UserId, &model.AIAgentPatch{DisplayName: model.NewPointer("Scout")})
		require.NoError(t, err)
		assert.Equal(t, "Scout", patched.DisplayName)
		list, _, err := clients.systemAdmin.GetAIAgents(ctx, "")
		require.NoError(t, err)
		ids := []string{}
		for _, a := range list {
			ids = append(ids, a.UserId)
		}
		assert.Contains(t, ids, created.UserId)
	})

	t.Run("a community's agents: its admins", func(t *testing.T) {
		for name, client := range map[string]*model.Client4{"member": clients.member, "guest": clients.guest, "another community's admin": clients.otherAdmin} {
			_, resp, err := client.CreateAIAgent(ctx, aiAgentRequest(th.BasicTeam.Id, connection.Id))
			require.Error(t, err, name)
			CheckForbiddenStatus(t, resp)
		}

		mine, resp, err := clients.communityAdmin.CreateAIAgent(ctx, aiAgentRequest(th.BasicTeam.Id, connection.Id))
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
		assert.Equal(t, th.TeamAdminUser.Id, mine.CreatorId)
		theirs, _, err := clients.systemAdmin.CreateAIAgent(ctx, aiAgentRequest(th.BasicTeam.Id, connection.Id))
		require.NoError(t, err)

		for _, agent := range []*model.AIAgentDetails{mine, theirs} {
			_, _, err = clients.communityAdmin.PatchAIAgent(ctx, agent.UserId, &model.AIAgentPatch{SystemPrompt: model.NewPointer("Be brief.")})
			require.NoError(t, err, "the community's admin manages its agents, whoever made them")
			_, _, err = clients.systemAdmin.GetAIAgent(ctx, agent.UserId)
			require.NoError(t, err)
			for name, client := range map[string]*model.Client4{"member": clients.member, "guest": clients.guest, "another community's admin": clients.otherAdmin} {
				_, resp, err = client.GetAIAgent(ctx, agent.UserId)
				require.Error(t, err, name)
				CheckForbiddenStatus(t, resp)
				_, resp, err = client.PatchAIAgent(ctx, agent.UserId, &model.AIAgentPatch{SystemPrompt: model.NewPointer("pwned")})
				require.Error(t, err, name)
				CheckForbiddenStatus(t, resp)
			}
		}

		list, _, err := clients.communityAdmin.GetAIAgents(ctx, th.BasicTeam.Id)
		require.NoError(t, err)
		assert.Len(t, list, 2)
		list, _, err = clients.communityAdmin.GetAIAgents(ctx, "")
		require.NoError(t, err)
		for _, a := range list {
			assert.Equal(t, th.BasicTeam.Id, a.Scope, "a community admin's list has no server agent")
		}
		_, resp, err = clients.otherAdmin.GetAIAgents(ctx, th.BasicTeam.Id)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		list, _, err = clients.member.GetAIAgents(ctx, "")
		require.NoError(t, err)
		assert.Empty(t, list, "a member manages none")

		_, _, err = clients.communityAdmin.SetAIAgentEnabled(ctx, theirs.UserId, false)
		require.NoError(t, err)
		resp, err = clients.communityAdmin.DeleteAIAgent(ctx, theirs.UserId)
		require.NoError(t, err)
		CheckOKStatus(t, resp)
	})

	t.Run("without manage_others_agent, a community's admin keeps their own", func(t *testing.T) {
		theirs, _, err := clients.systemAdmin.CreateAIAgent(ctx, aiAgentRequest(th.BasicTeam.Id, connection.Id))
		require.NoError(t, err)
		mine, _, err := clients.communityAdmin.CreateAIAgent(ctx, aiAgentRequest(th.BasicTeam.Id, connection.Id))
		require.NoError(t, err)
		th.RemovePermissionFromRole(t, model.PermissionManageOthersAgent.Id, model.TeamAdminRoleId)
		defer th.AddPermissionToRole(t, model.PermissionManageOthersAgent.Id, model.TeamAdminRoleId)

		_, resp, err := clients.communityAdmin.PatchAIAgent(ctx, theirs.UserId, &model.AIAgentPatch{SystemPrompt: model.NewPointer("x")})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		_, _, err = clients.communityAdmin.PatchAIAgent(ctx, mine.UserId, &model.AIAgentPatch{SystemPrompt: model.NewPointer("x")})
		require.NoError(t, err)
	})

	t.Run("connections an agent may use", func(t *testing.T) {
		infos, _, err := clients.communityAdmin.GetAIAgentConnections(ctx, th.BasicTeam.Id)
		require.NoError(t, err)
		ids := []string{}
		for _, info := range infos {
			ids = append(ids, info.Id)
		}
		assert.Contains(t, ids, connection.Id, "a server connection shared with the community")
		_, resp, err := clients.communityAdmin.GetAIAgentConnections(ctx, model.AIAgentScopeServer)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		_, resp, err = clients.member.GetAIAgentConnections(ctx, th.BasicTeam.Id)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		models, _, err := clients.communityAdmin.GetAIAgentConnectionModels(ctx, th.BasicTeam.Id, connection.Id)
		require.NoError(t, err)
		require.NotEmpty(t, models)
		assert.Equal(t, "test-model", models[0].ID)
	})
}

// No answer about agents carries a connection's key, or its headers' values.
func TestAIAgentsReturnNoSecret(t *testing.T) {
	th, clients, connection := setupAIAgentsAPI(t)
	ctx := context.Background()
	created, _, err := clients.systemAdmin.CreateAIAgent(ctx, aiAgentRequest(model.AIAgentScopeServer, connection.Id))
	require.NoError(t, err)
	assert.Equal(t, connection.Name, created.ConnectionName)

	for _, path := range []string{
		"/ai/agents",
		"/ai/agents/" + created.UserId,
		"/ai/agents/connections?scope=server",
		"/agents",
		"/llmservices",
		"/users/" + created.UserId,
	} {
		body := rawBody(t, clients.systemAdmin, "GET", path)
		assert.NotContains(t, body, apiAITestKey, path)
		assert.NotContains(t, body, "header-secret-value", path)
	}
	body := rawBody(t, th.Client, "GET", "/agents")
	assert.NotContains(t, body, apiAITestKey)
	assert.NotContains(t, body, "Be careful.", "a person talking to an agent doesn't see its prompt")
}

// An agent's activity: set by the agent's own token or its managers, read by whoever can see it; the thread only for
// who may read it.
func TestAgentActivityAPI(t *testing.T) {
	th, clients, connection := setupAIAgentsAPI(t)
	ctx := context.Background()
	created, _, err := clients.systemAdmin.CreateAIAgent(ctx, aiAgentRequest(model.AIAgentScopeServer, connection.Id))
	require.NoError(t, err)

	token, appErr := th.App.CreateUserAccessToken(th.Context, &model.UserAccessToken{UserId: created.UserId, Description: "agent"})
	require.Nil(t, appErr)
	agentClient := th.CreateClient()
	agentClient.SetToken(token.Token)

	private := th.CreatePrivateChannel(t)
	thread := th.CreatePostWithClient(t, th.Client, private)
	_, resp, err := agentClient.SetAgentActivity(ctx, created.UserId, &model.AgentActivityRequest{State: model.AgentStateWorking, Text: "Reading", ThreadPostId: thread.Id})
	require.Error(t, err, "an agent only points at a thread it can read")
	CheckBadRequestStatus(t, resp)

	agentUser, appErr := th.App.GetUser(th.Context, created.UserId)
	require.Nil(t, appErr)
	th.LinkUserToTeam(t, agentUser, th.BasicTeam)
	th.AddUserToChannel(t, agentUser, private)
	activity, _, err := agentClient.SetAgentActivity(ctx, created.UserId, &model.AgentActivityRequest{State: model.AgentStateWorking, Text: "Reading", ThreadPostId: thread.Id})
	require.NoError(t, err, "the agent's own token")
	assert.Equal(t, model.AgentStateWorking, activity.State)

	_, resp, err = th.Client.SetAgentActivity(ctx, created.UserId, &model.AgentActivityRequest{State: model.AgentStateIdle})
	require.Error(t, err, "a member can't speak for the agent")
	CheckForbiddenStatus(t, resp)
	_, _, err = clients.systemAdmin.SetAgentActivity(ctx, created.UserId, &model.AgentActivityRequest{State: model.AgentStateWorking, Text: "Reading", ThreadPostId: thread.Id})
	require.NoError(t, err, "its manager can")

	got, _, err := th.Client.GetAgentActivity(ctx, created.UserId)
	require.NoError(t, err)
	assert.Equal(t, "Reading", got.Text)
	assert.Equal(t, thread.Id, got.ThreadPostId, "the thread's reader sees it")

	got, _, err = th.SystemAdminClient.GetAgentActivity(ctx, created.UserId)
	require.NoError(t, err)
	assert.Equal(t, thread.Id, got.ThreadPostId)

	outsider := th.CreateUser(t)
	th.LinkUserToTeam(t, outsider, th.BasicTeam)
	outsiderClient := th.CreateClient()
	_, _, err = outsiderClient.Login(ctx, outsider.Email, outsider.Password)
	require.NoError(t, err)
	got, _, err = outsiderClient.GetAgentActivity(ctx, created.UserId)
	require.NoError(t, err)
	assert.Equal(t, "Reading", got.Text)
	assert.Empty(t, got.ThreadPostId, "who can't read the thread doesn't see it")

	list, _, err := outsiderClient.GetAgentActivitiesByIds(ctx, []string{created.UserId, th.BasicUser.Id})
	require.NoError(t, err)
	require.Len(t, list, 1, "only agents")
	assert.Empty(t, list[0].ThreadPostId)

	_, resp, err = th.Client.GetAgentActivitiesByIds(ctx, []string{"nope"})
	require.Error(t, err)
	CheckBadRequestStatus(t, resp)
}

// An agent's managers set its avatar through the users' image API; others can't.
func TestAIAgentAvatar(t *testing.T) {
	th, clients, connection := setupAIAgentsAPI(t)
	ctx := context.Background()
	community, _, err := clients.systemAdmin.CreateAIAgent(ctx, aiAgentRequest(th.BasicTeam.Id, connection.Id))
	require.NoError(t, err)

	data, err := testutils.ReadTestFile("test.png")
	require.NoError(t, err)

	resp, err := clients.member.SetProfileImage(ctx, community.UserId, data)
	require.Error(t, err, "a member can't")
	CheckForbiddenStatus(t, resp)
	resp, err = clients.otherAdmin.SetProfileImage(ctx, community.UserId, data)
	require.Error(t, err, "nor another community's admin")
	CheckForbiddenStatus(t, resp)

	resp, err = clients.communityAdmin.SetProfileImage(ctx, community.UserId, data)
	require.NoError(t, err, "the community's admin manages its agent")
	CheckOKStatus(t, resp)

	// Managing an agent isn't managing a person.
	resp, err = clients.communityAdmin.SetProfileImage(ctx, th.BasicUser2.Id, data)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)
}

// A bot made through the bots API is a plain bot, whatever kind it asks for.
func TestCreateBotIsNeverAnAgent(t *testing.T) {
	th, clients, _ := setupAIAgentsAPI(t)
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.EnableBotAccountCreation = true })
	body, err := json.Marshal(map[string]any{"username": "plain" + strings.ToLower(model.NewId()[:8]), "kind": model.BotKindAgent})
	require.NoError(t, err)
	r, err := clients.systemAdmin.DoAPIPost(context.Background(), "/bots", string(body))
	require.NoError(t, err)
	defer r.Body.Close()
	var bot model.Bot
	require.NoError(t, json.NewDecoder(r.Body).Decode(&bot))
	assert.Equal(t, model.BotKindBot, bot.Kind)
	user, appErr := th.App.GetUser(th.Context, bot.UserId)
	require.Nil(t, appErr)
	assert.False(t, user.IsAgent)
}

// An agent's status is the server's: nobody sets it through the API, not even a system admin, while people still set
// their own.
func TestAgentStatusIsTheServers(t *testing.T) {
	th, clients, connection := setupAIAgentsAPI(t)
	ctx := context.Background()
	agent, _, err := clients.systemAdmin.CreateAIAgent(ctx, aiAgentRequest(model.AIAgentScopeServer, connection.Id))
	require.NoError(t, err)

	// The server sets it from its connection, which answers.
	require.Eventually(t, func() bool {
		status, appErr := th.App.GetStatus(agent.UserId)
		return appErr == nil && status.Status == model.StatusOnline
	}, 10*time.Second, 20*time.Millisecond)

	for _, status := range []string{model.StatusOffline, model.StatusAway, model.StatusDnd, model.StatusOnline} {
		_, resp, err := clients.systemAdmin.UpdateUserStatus(ctx, agent.UserId, &model.Status{UserId: agent.UserId, Status: status})
		require.Error(t, err, status)
		CheckForbiddenStatus(t, resp)
	}
	_, resp, err := clients.systemAdmin.UpdateUserCustomStatus(ctx, agent.UserId, &model.CustomStatus{Emoji: "robot", Text: "On a break"})
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)
	resp, err = clients.systemAdmin.RemoveUserCustomStatus(ctx, agent.UserId)
	require.Error(t, err)
	CheckForbiddenStatus(t, resp)

	status, appErr := th.App.GetStatus(agent.UserId)
	require.Nil(t, appErr)
	assert.Equal(t, model.StatusOnline, status.Status, "unchanged")

	// People still set their own.
	_, _, err = clients.member.UpdateUserStatus(ctx, th.BasicUser.Id, &model.Status{UserId: th.BasicUser.Id, Status: model.StatusDnd})
	require.NoError(t, err)
}
