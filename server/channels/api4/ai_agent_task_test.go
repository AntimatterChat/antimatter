// Copyright (c) 2026-present Antimatter contributors.
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

// setupAIChatAPI gives an agent everyone may use, one only system admins may use, and a private channel with a
// message the member can't read.
func setupAIChatAPI(t *testing.T) (*TestHelper, aiTestClients, *model.AIAgentDetails, *model.AIAgentDetails, *model.Post) {
	th, clients, connection := setupAIAgentsAPI(t)
	ctx := context.Background()
	open, _, err := clients.systemAdmin.CreateAIAgent(ctx, aiAgentRequest(model.AIAgentScopeServer, connection.Id))
	require.NoError(t, err)
	adminsOnlyReq := aiAgentRequest(model.AIAgentScopeServer, connection.Id)
	adminsOnlyReq.Access = model.AIAgentAccess{Roles: []string{model.SystemAdminRoleId}}
	adminsOnly, _, err := clients.systemAdmin.CreateAIAgent(ctx, adminsOnlyReq)
	require.NoError(t, err)

	secret, appErr := th.App.CreateChannel(th.Context, &model.Channel{
		DisplayName: "Board", Name: "board-" + model.NewId()[:8], Type: model.ChannelTypePrivate,
		TeamId: th.BasicTeam.Id, CreatorId: th.SystemAdminUser.Id,
	}, true)
	require.Nil(t, appErr)
	secretPost, _, appErr := th.App.CreatePost(th.Context, &model.Post{UserId: th.SystemAdminUser.Id, ChannelId: secret.Id, Message: "The merger code name is BLUEBIRD."}, secret, model.CreatePostFlags{})
	require.Nil(t, appErr)
	return th, clients, open, adminsOnly, secretPost
}

// The AI chat's task API: a person asks an agent they may use about what they can read; the request is their own
// message in their direct message with the agent.
func TestAIAgentTasksAPI(t *testing.T) {
	th, clients, open, adminsOnly, secretPost := setupAIChatAPI(t)
	ctx := context.Background()

	request, resp, err := clients.member.StartAIAgentTask(ctx, open.UserId, &model.AIAgentTaskRequest{Task: model.AIAgentTaskSummarizeThread, PostId: th.BasicPost.Id})
	require.NoError(t, err)
	CheckCreatedStatus(t, resp)
	assert.Equal(t, th.BasicUser.Id, request.UserId)
	dm, appErr := th.App.GetChannel(th.Context, request.ChannelId)
	require.Nil(t, appErr)
	assert.Equal(t, model.ChannelTypeDirect, dm.Type)
	assert.Equal(t, open.UserId, dm.GetOtherUserIdForDM(th.BasicUser.Id))

	_, resp, err = clients.member.StartAIAgentTask(ctx, open.UserId, &model.AIAgentTaskRequest{Task: model.AIAgentTaskSummarizeChannel, ChannelId: th.BasicChannel.Id, Since: model.GetMillis() - 86400000})
	require.NoError(t, err)
	CheckCreatedStatus(t, resp)

	t.Run("refused: an agent the person may not use", func(t *testing.T) {
		_, resp, err := clients.member.StartAIAgentTask(ctx, adminsOnly.UserId, &model.AIAgentTaskRequest{Task: model.AIAgentTaskSummarizeThread, PostId: th.BasicPost.Id})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		_, resp, err = clients.systemAdmin.StartAIAgentTask(ctx, adminsOnly.UserId, &model.AIAgentTaskRequest{Task: model.AIAgentTaskSummarizeThread, PostId: th.BasicPost.Id})
		require.NoError(t, err, "a system admin may")
		CheckCreatedStatus(t, resp)
	})

	t.Run("refused: what the person can't read, as if it weren't there", func(t *testing.T) {
		_, resp, err := clients.member.StartAIAgentTask(ctx, open.UserId, &model.AIAgentTaskRequest{Task: model.AIAgentTaskSummarizeThread, PostId: secretPost.Id})
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
		_, resp, err = clients.member.StartAIAgentTask(ctx, open.UserId, &model.AIAgentTaskRequest{Task: model.AIAgentTaskAskChannel, ChannelId: secretPost.ChannelId, Question: "code name?"})
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("refused: guests, unless the agent allows them", func(t *testing.T) {
		_, resp, err := clients.guest.StartAIAgentTask(ctx, open.UserId, &model.AIAgentTaskRequest{Task: model.AIAgentTaskSummarizeChannel, ChannelId: th.BasicChannel.Id})
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
	})

	t.Run("invalid requests", func(t *testing.T) {
		for name, req := range map[string]*model.AIAgentTaskRequest{
			"an unknown task":            {Task: "delete_everything", PostId: th.BasicPost.Id},
			"a thread task's channel":    {Task: model.AIAgentTaskSummarizeThread, ChannelId: th.BasicChannel.Id},
			"an empty question":          {Task: model.AIAgentTaskAskChannel, ChannelId: th.BasicChannel.Id},
			"a transcription that isn't": {Task: model.AIAgentTaskSummarizeTranscription, PostId: th.BasicPost.Id},
		} {
			_, resp, err := clients.member.StartAIAgentTask(ctx, open.UserId, req)
			require.Error(t, err, name)
			CheckBadRequestStatus(t, resp)
		}
		_, resp, err := clients.member.StartAIAgentTask(ctx, model.NewId(), &model.AIAgentTaskRequest{Task: model.AIAgentTaskSummarizeThread, PostId: th.BasicPost.Id})
		require.Error(t, err, "not an agent")
		assert.GreaterOrEqual(t, resp.StatusCode, 400)
	})
}

// The AI chat's conversations API lists the person's own, and refuses an agent they may not use.
func TestAIAgentConversationsAPI(t *testing.T) {
	th, clients, open, adminsOnly, _ := setupAIChatAPI(t)
	ctx := context.Background()

	request, _, err := clients.member.StartAIAgentTask(ctx, open.UserId, &model.AIAgentTaskRequest{Task: model.AIAgentTaskSummarizeThread, PostId: th.BasicPost.Id})
	require.NoError(t, err)

	conversations, resp, err := clients.member.GetAIAgentConversations(ctx, "", 0, 0)
	require.NoError(t, err)
	CheckOKStatus(t, resp)
	found := false
	for _, c := range conversations {
		if c.RootId == request.Id {
			found = true
			assert.Equal(t, open.UserId, c.AgentId)
			assert.True(t, c.Direct)
			assert.NotEmpty(t, c.Title)
		}
	}
	assert.True(t, found, "the conversation the task started")

	conversations, _, err = clients.member.GetAIAgentConversations(ctx, open.UserId, 0, 10)
	require.NoError(t, err)
	assert.NotEmpty(t, conversations)

	_, resp, err = clients.member.GetAIAgentConversations(ctx, adminsOnly.UserId, 0, 0)
	require.Error(t, err, "an agent the person may not use")
	CheckForbiddenStatus(t, resp)

	others, _, err := clients.communityAdmin.GetAIAgentConversations(ctx, "", 0, 0)
	require.NoError(t, err)
	for _, c := range others {
		assert.NotEqual(t, request.Id, c.RootId, "never someone else's")
	}

	for _, path := range []string{
		"/ai/agents/conversations?agent_id=nope",
		"/ai/agents/conversations?before=-1",
		"/ai/agents/conversations?before=soon",
		"/ai/agents/conversations?per_page=0",
		"/ai/agents/conversations?per_page=101",
	} {
		body := rawBody(t, clients.member, "GET", path)
		assert.Contains(t, body, `"status_code":400`, path)
	}
}

// Security regression: the agents a person may talk to (what the AI chat's picker lists) leave out the agents they
// may not use, and never carry an agent's prompt.
func TestUsableAgentsListOnlyAllowed(t *testing.T) {
	th, clients, open, adminsOnly, _ := setupAIChatAPI(t)

	ids := func(client *model.Client4) []string {
		var list []model.BridgeAgentInfo
		require.NoError(t, json.Unmarshal([]byte(rawBody(t, client, "GET", "/agents")), &list))
		out := []string{}
		for _, a := range list {
			out = append(out, a.ID)
		}
		return out
	}
	member := ids(clients.member)
	assert.Contains(t, member, open.UserId)
	assert.NotContains(t, member, adminsOnly.UserId)
	admin := ids(clients.systemAdmin)
	assert.Contains(t, admin, adminsOnly.UserId)
	assert.NotContains(t, ids(clients.guest), open.UserId, "guests only where an agent allows them")
	assert.NotContains(t, rawBody(t, th.Client, "GET", "/agents"), "Be careful.")
}
