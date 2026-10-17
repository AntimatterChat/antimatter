// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/i18n"
	"github.com/mattermost/mattermost/server/v8/channels/app/platform"
)

func TestSetAndGetAgentActivity(t *testing.T) {
	th, _, c := setupAgentTest(t, "")
	agent := createTestAgent(t, th, model.AIAgentScopeServer, c.Id, nil).AIAgent

	activity, appErr := th.App.GetAgentActivity(th.Context, agent.UserId, th.BasicUser.Id)
	require.Nil(t, appErr)
	assert.Equal(t, model.AgentStateIdle, activity.State, "an agent that never said anything is idle")

	// A thread in a private channel BasicUser2 isn't in.
	thread := th.CreatePost(t, th.CreatePrivateChannel(t, th.BasicTeam))
	set, appErr := th.App.SetAgentActivity(th.Context, agent, &model.AgentActivityRequest{State: model.AgentStateWorking, Text: "Running the poll tests", ThreadPostId: thread.Id})
	require.Nil(t, appErr)
	assert.NotZero(t, set.ExpiresAt, "a busy state expires")

	activity, appErr = th.App.GetAgentActivity(th.Context, agent.UserId, th.BasicUser.Id)
	require.Nil(t, appErr)
	assert.Equal(t, model.AgentStateWorking, activity.State)
	assert.Equal(t, "Running the poll tests", activity.Text)
	assert.Equal(t, thread.Id, activity.ThreadPostId, "who may read the thread sees it")

	activity, appErr = th.App.GetAgentActivity(th.Context, agent.UserId, th.BasicUser2.Id)
	require.Nil(t, appErr)
	assert.Equal(t, model.AgentStateWorking, activity.State)
	assert.Empty(t, activity.ThreadPostId, "who may not read the thread doesn't")

	_, appErr = th.App.SetAgentActivity(th.Context, agent, &model.AgentActivityRequest{State: "napping"})
	require.NotNil(t, appErr)

	// An expired busy state is idle.
	require.NoError(t, th.App.Srv().Store().AgentActivity().Save(th.Context, &model.AgentActivity{UserId: agent.UserId, State: model.AgentStateBlocked, Text: "Needs a key", UpdateAt: 1, ExpiresAt: 2}))
	activity, appErr = th.App.GetAgentActivity(th.Context, agent.UserId, th.BasicUser.Id)
	require.Nil(t, appErr)
	assert.Equal(t, model.AgentStateIdle, activity.State)
	assert.Empty(t, activity.Text)

	activities, appErr := th.App.GetAgentActivities(th.Context, []string{agent.UserId, th.BasicUser2.Id, model.NewId()}, th.BasicUser.Id)
	require.Nil(t, appErr)
	require.Len(t, activities, 1, "only agents have an activity")
	assert.Equal(t, agent.UserId, activities[0].UserId)
}

func TestSessionCanSetAgentActivity(t *testing.T) {
	th, _, c := setupAgentTest(t, "")
	agent := createTestAgent(t, th, model.AIAgentScopeServer, c.Id, nil).AIAgent

	assert.True(t, th.App.SessionCanSetAgentActivity(model.Session{UserId: agent.UserId, Roles: model.SystemUserRoleId}, agent), "the agent itself")
	assert.True(t, th.App.SessionCanSetAgentActivity(model.Session{UserId: th.SystemAdminUser.Id, Roles: model.SystemAdminRoleId + " " + model.SystemUserRoleId}, agent), "its manager")
	assert.False(t, th.App.SessionCanSetAgentActivity(model.Session{UserId: th.BasicUser.Id, Roles: model.SystemUserRoleId}, agent), "anyone else")

	other := createTestAgent(t, th, model.AIAgentScopeServer, c.Id, nil).AIAgent
	assert.False(t, th.App.SessionCanSetAgentActivity(model.Session{UserId: other.UserId, Roles: model.SystemUserRoleId}, agent), "nor another agent")
}

// agent_activity_changed reaches those who can see the agent: everyone for a server agent, a community's members for
// its agent, and a guest only for an agent they share a channel with, as user_updated does; never the thread.
func TestAgentActivityBroadcast(t *testing.T) {
	th, _, c := setupAgentTest(t, "")
	server := createTestAgent(t, th, model.AIAgentScopeServer, c.Id, nil).AIAgent
	community := createTestAgent(t, th, th.BasicTeam.Id, c.Id, nil).AIAgent

	guest := th.CreateGuest(t)
	th.LinkUserToTeam(t, guest, th.BasicTeam)
	guestChannel := th.CreateChannel(t, th.BasicTeam)
	th.AddUserToChannel(t, guest, guestChannel)
	outsider := th.CreateUser(t)
	th.LinkUserToTeam(t, outsider, th.CreateTeam(t))

	conn := func(user *model.User, guest bool) *platform.WebConn {
		session := &model.Session{UserId: user.Id, Roles: user.GetRawRoles()}
		if guest {
			session.AddProp(model.SessionPropIsGuest, "true")
		} else {
			session.AddProp(model.SessionPropIsGuest, "false")
		}
		saved, appErr := th.App.CreateSession(th.Context, session)
		require.Nil(t, appErr)
		wc := &platform.WebConn{Platform: th.Server.Platform(), Suite: th.App, UserId: user.Id, T: i18n.T}
		wc.SetConnectionID(model.NewId())
		wc.SetSession(saved)
		wc.SetSessionToken(saved.Token)
		wc.SetSessionExpiresAt(saved.ExpiresAt)
		return wc
	}
	member := conn(th.BasicUser, false)
	outsiderConn := conn(outsider, false)
	guestConn := conn(guest, true)

	thread := th.CreatePost(t, th.BasicChannel)
	activity := &model.AgentActivity{UserId: server.UserId, State: model.AgentStateWorking, Text: "Reading", ThreadPostId: thread.Id, UpdateAt: model.GetMillis()}

	serverEvent := agentActivityEvent(server, activity)
	data := serverEvent.GetData()
	sent, ok := data["activity"].(*model.AgentActivity)
	require.True(t, ok)
	assert.Empty(t, sent.ThreadPostId, "the thread is never sent live")
	assert.Equal(t, "Reading", sent.Text)
	assert.NotEmpty(t, activity.ThreadPostId, "the saved activity keeps it")

	assert.True(t, member.ShouldSendEvent(serverEvent), "a member sees a server agent's activity")
	assert.True(t, outsiderConn.ShouldSendEvent(serverEvent), "so does anyone on the server")
	assert.False(t, guestConn.ShouldSendEvent(serverEvent), "a guest doesn't, for an agent they can't see")

	serverUser := mustGetUser(t, th, server.UserId)
	th.LinkUserToTeam(t, serverUser, th.BasicTeam)
	th.AddUserToChannel(t, serverUser, guestChannel)
	guestConn.InvalidateCache()
	th.App.Srv().Platform().InvalidateCacheForUser(guest.Id)
	assert.True(t, guestConn.ShouldSendEvent(agentActivityEvent(server, activity)), "a guest does once they share a channel with it")

	communityEvent := agentActivityEvent(community, &model.AgentActivity{UserId: community.UserId, State: model.AgentStateIdle, UpdateAt: model.GetMillis()})
	assert.Equal(t, th.BasicTeam.Id, communityEvent.GetBroadcast().TeamId)
	assert.True(t, member.ShouldSendEvent(communityEvent), "a community's agent: its members")
	assert.False(t, outsiderConn.ShouldSendEvent(communityEvent), "not someone outside the community")
	assert.False(t, guestConn.ShouldSendEvent(communityEvent), "nor a guest of the community who can't see it")
	th.AddUserToChannel(t, mustGetUser(t, th, community.UserId), guestChannel)
	guestConn.InvalidateCache()
	th.App.Srv().Platform().InvalidateCacheForUser(guest.Id)
	assert.True(t, guestConn.ShouldSendEvent(communityEvent), "a guest sharing a channel with it")
}

func mustGetUser(t *testing.T, th *TestHelper, id string) *model.User {
	t.Helper()
	user, appErr := th.App.GetUser(th.Context, id)
	require.Nil(t, appErr)
	return user
}
