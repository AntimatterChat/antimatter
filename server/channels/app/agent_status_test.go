// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func agentStatusOf(th *TestHelper, userID string) *model.Status {
	status, appErr := th.App.GetStatus(userID)
	if appErr != nil {
		return &model.Status{}
	}
	return status
}

// waitForAgentStatus waits for the status the server sets for an agent (some changes check its connection in the
// background).
func waitForAgentStatus(t *testing.T, th *TestHelper, userID, want string) {
	t.Helper()
	require.Eventually(t, func() bool {
		return agentStatusOf(th, userID).Status == want
	}, 10*time.Second, 20*time.Millisecond, "the agent's status becomes %s", want)
}

func TestAIAgentStatusFor(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		active, setUp, reachable bool
		want                     string
	}{
		{"on, set up, answers", true, true, true, model.StatusOnline},
		{"on, set up, doesn't answer", true, true, false, model.StatusAway},
		{"on, no usable connection", true, false, false, model.StatusOffline},
		{"off", false, true, true, model.StatusOffline},
	} {
		assert.Equal(t, tc.want, aiAgentStatusFor(tc.active, tc.setUp, tc.reachable), tc.name)
	}
}

// Agents showed offline: a built-in agent is online while its model connection answers, away while it doesn't,
// offline when it's off or has no connection, and no automatic change overrides what the server set.
func TestAgentStatusFollowsItsConnection(t *testing.T) {
	th, f, c := setupAgentTest(t, "Hi")
	agent := createTestAgent(t, th, model.AIAgentScopeServer, c.Id, nil)

	// A new agent on a connection that answers is online, as the server set it.
	waitForAgentStatus(t, th, agent.UserId, model.StatusOnline)
	assert.True(t, agentStatusOf(th, agent.UserId).Manual, "saved as set, so that nothing automatic overrides it")
	assert.Positive(t, f.modelListCount(), "its connection was checked")

	t.Run("nothing automatic overrides it", func(t *testing.T) {
		th.App.SetStatusOffline(agent.UserId, false, false)
		th.App.SetStatusAwayIfNeeded(agent.UserId, false)
		th.App.Srv().Platform().SetStatusOnline(agent.UserId, false)
		assert.Equal(t, model.StatusOnline, agentStatusOf(th, agent.UserId).Status)
	})

	t.Run("away when a test of its connection fails, online when it answers again", func(t *testing.T) {
		f.setUnavailable(true)
		assert.False(t, th.App.TestAIConnection(th.Context, c).OK)
		assert.Equal(t, model.StatusAway, agentStatusOf(th, agent.UserId).Status)

		f.setUnavailable(false)
		assert.True(t, th.App.TestAIConnection(th.Context, c).OK)
		assert.Equal(t, model.StatusOnline, agentStatusOf(th, agent.UserId).Status)
	})

	t.Run("the leader's regular check", func(t *testing.T) {
		f.setUnavailable(true)
		th.App.RefreshAIAgentStatuses(th.Context, "")
		assert.Equal(t, model.StatusAway, agentStatusOf(th, agent.UserId).Status)

		f.setUnavailable(false)
		th.App.RefreshAIAgentStatuses(th.Context, c.Id)
		assert.Equal(t, model.StatusOnline, agentStatusOf(th, agent.UserId).Status)
	})

	t.Run("offline while it's off, online when it's on again", func(t *testing.T) {
		_, appErr := th.App.SetAIAgentEnabled(th.Context, agent.AIAgent, false)
		require.Nil(t, appErr)
		assert.Equal(t, model.StatusOffline, agentStatusOf(th, agent.UserId).Status)
		th.App.RefreshAIAgentStatuses(th.Context, "")
		assert.Equal(t, model.StatusOffline, agentStatusOf(th, agent.UserId).Status, "a check doesn't bring an agent that's off online")

		_, appErr = th.App.SetAIAgentEnabled(th.Context, agent.AIAgent, true)
		require.Nil(t, appErr)
		waitForAgentStatus(t, th, agent.UserId, model.StatusOnline)
	})

	t.Run("moved to a connection that doesn't answer: away", func(t *testing.T) {
		down := newFakeLLM(t, "")
		down.setUnavailable(true)
		other := createTestAIConnection(t, th, down, model.AIConnectionScopeServer, nil)
		_, appErr := th.App.PatchAIAgent(th.Context, agent.AIAgent, &model.AIAgentPatch{ConnectionId: &other.Id}, th.SystemAdminUser.Id)
		require.Nil(t, appErr)
		waitForAgentStatus(t, th, agent.UserId, model.StatusAway)

		_, appErr = th.App.PatchAIAgent(th.Context, agent.AIAgent, &model.AIAgentPatch{ConnectionId: &c.Id}, th.SystemAdminUser.Id)
		require.Nil(t, appErr)
		waitForAgentStatus(t, th, agent.UserId, model.StatusOnline)
	})

	t.Run("offline once its connection is deleted", func(t *testing.T) {
		require.Nil(t, th.App.DeleteAIConnection(th.Context, c))
		waitForAgentStatus(t, th, agent.UserId, model.StatusOffline)
	})
}

// What an agent's call to its model says of the endpoint is its status: away when the endpoint fails, online when it
// answers again.
func TestAgentStatusFollowsItsCalls(t *testing.T) {
	th, f, c := setupAgentTest(t, "Hello")
	agent := createTestAgent(t, th, model.AIAgentScopeServer, c.Id, nil)
	waitForAgentStatus(t, th, agent.UserId, model.StatusOnline)

	f.fail(http.StatusBadGateway, "upstream down")
	post := savePostQuietly(t, th, &model.Post{UserId: th.BasicUser.Id, ChannelId: th.BasicChannel.Id, Message: "@" + agent.Username + " hi"})
	th.App.handlePostForAgents(th.Context, post, th.BasicChannel)
	waitForAgentReply(t, th, agent.UserId, post.Id)
	assert.Equal(t, model.StatusAway, agentStatusOf(th, agent.UserId).Status)

	f.fail(0, "")
	post = savePostQuietly(t, th, &model.Post{UserId: th.BasicUser.Id, ChannelId: th.BasicChannel.Id, Message: "@" + agent.Username + " again"})
	th.App.handlePostForAgents(th.Context, post, th.BasicChannel)
	waitForAgentReply(t, th, agent.UserId, post.Id)
	assert.Equal(t, model.StatusOnline, agentStatusOf(th, agent.UserId).Status)
}

func TestAICallReachedEndpoint(t *testing.T) {
	for id, want := range map[string][2]bool{
		"app.ai.call.provider_failed.app_error":       {false, true},
		"app.ai.call.timeout.app_error":               {false, true},
		"app.ai.call.provider_refused.app_error":      {false, true},
		"app.ai.call.provider_rate_limited.app_error": {true, true},
		"app.ai.call.provider_bad_request.app_error":  {true, true},
		"app.ai.call.budget.app_error":                {false, false},
		"app.ai.call.audience.app_error":              {false, false},
	} {
		reachable, known := aiCallReachedEndpoint(model.NewAppError("test", id, nil, "", http.StatusBadRequest))
		assert.Equal(t, want, [2]bool{reachable, known}, id)
	}
	reachable, known := aiCallReachedEndpoint(nil)
	assert.True(t, reachable && known, "a call that went through")
}

// Nobody sets an agent's status but the server (a built-in agent's) or the agent's own program (an external one's):
// not its managers, not system admins, not apps.
func TestSessionMaySetUserStatus(t *testing.T) {
	th, _, c := setupAgentTest(t, "")
	builtIn := createTestAgent(t, th, model.AIAgentScopeServer, c.Id, nil)
	external := createTestAgent(t, th, model.AIAgentScopeServer, "", func(req *model.AIAgentRequest) { req.External = true })

	admin := model.Session{UserId: th.SystemAdminUser.Id, Roles: model.SystemAdminRoleId + " " + model.SystemUserRoleId}
	appSession := model.Session{}

	t.Run("a built-in agent's: nobody", func(t *testing.T) {
		for name, session := range map[string]model.Session{"system admin": admin, "an app": appSession, "its own token": {UserId: builtIn.UserId}} {
			appErr := th.App.SessionMaySetUserStatus(th.Context, session, builtIn.UserId)
			require.NotNil(t, appErr, name)
			assert.Equal(t, http.StatusForbidden, appErr.StatusCode, name)
			assert.Equal(t, "api.status.agent.app_error", appErr.Id, name)
		}
	})

	t.Run("an external agent's: only its own token", func(t *testing.T) {
		assert.Nil(t, th.App.SessionMaySetUserStatus(th.Context, model.Session{UserId: external.UserId}, external.UserId))
		assert.NotNil(t, th.App.SessionMaySetUserStatus(th.Context, admin, external.UserId))
		assert.NotNil(t, th.App.SessionMaySetUserStatus(th.Context, appSession, external.UserId))
	})

	t.Run("anyone else's: as before", func(t *testing.T) {
		assert.Nil(t, th.App.SessionMaySetUserStatus(th.Context, model.Session{UserId: th.BasicUser.Id}, th.BasicUser.Id))
		assert.Nil(t, th.App.SessionMaySetUserStatus(th.Context, admin, th.BasicUser.Id))
	})

	t.Run("an external agent's status is its own connection's: the server's checks leave it", func(t *testing.T) {
		th.App.SetStatusOnline(external.UserId, false)
		th.App.RefreshAIAgentStatuses(th.Context, "")
		assert.Equal(t, model.StatusOnline, agentStatusOf(th, external.UserId).Status)
		assert.False(t, agentStatusOf(th, external.UserId).Manual)
	})

	t.Run("the app API refuses an agent's status", func(t *testing.T) {
		api := NewPluginAPI(th.App, th.Context, &model.Manifest{Id: "some.app"})
		_, appErr := api.UpdateUserStatus(builtIn.UserId, model.StatusDnd)
		require.NotNil(t, appErr)
		_, appErr = api.SetUserStatusTimedDND(builtIn.UserId, model.GetMillis()/1000+3600)
		require.NotNil(t, appErr)
		require.NotNil(t, api.UpdateUserCustomStatus(builtIn.UserId, &model.CustomStatus{Emoji: "robot", Text: "Busy"}))
		require.NotNil(t, api.RemoveUserCustomStatus(builtIn.UserId))
		_, appErr = api.UpdateUserStatus(external.UserId, model.StatusAway)
		require.NotNil(t, appErr)

		_, appErr = api.UpdateUserStatus(th.BasicUser.Id, model.StatusAway)
		require.Nil(t, appErr, "a person's, as before")
	})
}
