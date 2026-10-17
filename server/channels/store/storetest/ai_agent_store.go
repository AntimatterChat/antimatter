// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package storetest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

func TestAIAgentStore(t *testing.T, rctx request.CTX, ss store.Store) {
	t.Run("SaveGetUpdate", func(t *testing.T) { testAIAgentSaveGetUpdate(t, rctx, ss) })
	t.Run("External", func(t *testing.T) { testAIAgentExternal(t, rctx, ss) })
	t.Run("Scopes", func(t *testing.T) { testAIAgentScopes(t, rctx, ss) })
	t.Run("Delete", func(t *testing.T) { testAIAgentDelete(t, rctx, ss) })
	t.Run("Conversations", func(t *testing.T) { testAIAgentConversations(t, rctx, ss) })
}

func TestAgentActivityStore(t *testing.T, rctx request.CTX, ss store.Store) {
	t.Run("SaveGet", func(t *testing.T) { testAgentActivitySaveGet(t, rctx, ss) })
}

func newTestAIAgent(scope string) *model.AIAgent {
	return &model.AIAgent{
		UserId:       model.NewId(),
		Scope:        scope,
		ConnectionId: model.NewId(),
		Model:        "chat-model",
		SystemPrompt: "You are a careful researcher.",
		Tools:        model.AIAgentToolPolicy{Enabled: true, Allowed: []string{"notes.*", "search.posts"}, AutoRun: []string{"search.posts"}},
		Reasoning:    model.AIAgentReasoning{Enabled: true, Effort: model.AIAgentReasoningEffortHigh},
		MaxToolTurns: 12,
		Access:       model.AIAgentAccess{Everyone: true},
		CreatorId:    model.NewId(),
	}
}

func testAIAgentSaveGetUpdate(t *testing.T, rctx request.CTX, ss store.Store) {
	a := newTestAIAgent(model.AIAgentScopeServer)
	a.Access = model.AIAgentAccess{ChannelIDs: []string{model.NewId()}, Roles: []string{"system_admin"}, AllowGuests: true}
	saved, err := ss.AIAgent().Save(rctx, a)
	require.NoError(t, err)
	require.NotZero(t, saved.CreateAt)

	got, err := ss.AIAgent().Get(rctx, a.UserId)
	require.NoError(t, err)
	assert.Equal(t, model.AIAgentScopeServer, got.Scope)
	assert.False(t, got.External)
	assert.Equal(t, a.ConnectionId, got.ConnectionId)
	assert.Equal(t, "chat-model", got.Model)
	assert.Equal(t, "You are a careful researcher.", got.SystemPrompt)
	assert.Equal(t, []string{"notes.*", "search.posts"}, got.Tools.Allowed)
	assert.Equal(t, []string{"search.posts"}, got.Tools.AutoRun)
	assert.True(t, got.Reasoning.Enabled)
	assert.Equal(t, model.AIAgentReasoningEffortHigh, got.Reasoning.Effort)
	assert.Equal(t, 12, got.MaxToolTurns)
	assert.Equal(t, a.Access.ChannelIDs, got.Access.ChannelIDs)
	assert.Equal(t, []string{"system_admin"}, got.Access.Roles)
	assert.Equal(t, []string{}, got.Access.UserIDs, "empty lists read back as empty, not null")
	assert.True(t, got.Access.AllowGuests)
	assert.Equal(t, a.CreatorId, got.CreatorId)

	got.Model = "other-model"
	got.Scope = model.NewId() // ignored: the scope never changes
	got.UpdaterId = model.NewId()
	got.NeedsAttention = ""
	_, err = ss.AIAgent().Update(rctx, got)
	require.NoError(t, err)
	again, err := ss.AIAgent().Get(rctx, a.UserId)
	require.NoError(t, err)
	assert.Equal(t, "other-model", again.Model)
	assert.Equal(t, model.AIAgentScopeServer, again.Scope)
	assert.Equal(t, got.UpdaterId, again.UpdaterId)
	assert.Equal(t, a.CreatorId, again.CreatorId)

	var nfErr *store.ErrNotFound
	_, err = ss.AIAgent().Get(rctx, model.NewId())
	assert.ErrorAs(t, err, &nfErr)

	invalid := newTestAIAgent("not a scope")
	_, err = ss.AIAgent().Save(rctx, invalid)
	assert.Error(t, err)

	noConnection := newTestAIAgent(model.AIAgentScopeServer)
	noConnection.ConnectionId = ""
	_, err = ss.AIAgent().Save(rctx, noConnection)
	assert.Error(t, err, "a built-in agent needs a connection")

	_, err = ss.AIAgent().Save(rctx, a)
	assert.Error(t, err, "one agent per account")
}

// testAIAgentExternal: an external agent keeps no model settings.
func testAIAgentExternal(t *testing.T, rctx request.CTX, ss store.Store) {
	a := newTestAIAgent(model.AIAgentScopeServer)
	a.External = true
	_, err := ss.AIAgent().Save(rctx, a)
	require.NoError(t, err)
	got, err := ss.AIAgent().Get(rctx, a.UserId)
	require.NoError(t, err)
	assert.True(t, got.External)
	assert.Empty(t, got.ConnectionId)
	assert.Empty(t, got.Model)
	assert.Empty(t, got.SystemPrompt)
	assert.False(t, got.Tools.Enabled)
	assert.Empty(t, got.Tools.Allowed)
}

func testAIAgentScopes(t *testing.T, rctx request.CTX, ss store.Store) {
	team, other := model.NewId(), model.NewId()
	server, err := ss.AIAgent().Save(rctx, newTestAIAgent(model.AIAgentScopeServer))
	require.NoError(t, err)
	community, err := ss.AIAgent().Save(rctx, newTestAIAgent(team))
	require.NoError(t, err)
	elsewhere, err := ss.AIAgent().Save(rctx, newTestAIAgent(other))
	require.NoError(t, err)

	ids := func(list []*model.AIAgent) []string {
		out := []string{}
		for _, a := range list {
			out = append(out, a.UserId)
		}
		return out
	}
	list, err := ss.AIAgent().GetForScopes(rctx, []string{team})
	require.NoError(t, err)
	assert.Equal(t, []string{community.UserId}, ids(list))

	list, err = ss.AIAgent().GetForScopes(rctx, []string{model.AIAgentScopeServer, team})
	require.NoError(t, err)
	assert.Contains(t, ids(list), server.UserId)
	assert.Contains(t, ids(list), community.UserId)
	assert.NotContains(t, ids(list), elsewhere.UserId)

	list, err = ss.AIAgent().GetForScopes(rctx, nil)
	require.NoError(t, err)
	assert.Empty(t, list)

	list, err = ss.AIAgent().GetAll(rctx)
	require.NoError(t, err)
	assert.Subset(t, ids(list), []string{server.UserId, community.UserId, elsewhere.UserId})

	// A community's agent can't be opened to another community.
	wide := newTestAIAgent(team)
	wide.Access = model.AIAgentAccess{TeamIDs: []string{other}}
	_, err = ss.AIAgent().Save(rctx, wide)
	assert.Error(t, err)
}

func testAIAgentDelete(t *testing.T, rctx request.CTX, ss store.Store) {
	team := model.NewId()
	a, err := ss.AIAgent().Save(rctx, newTestAIAgent(team))
	require.NoError(t, err)
	require.NoError(t, ss.AIAgent().Delete(rctx, a.UserId, model.GetMillis()))

	got, err := ss.AIAgent().Get(rctx, a.UserId)
	require.NoError(t, err)
	assert.NotZero(t, got.DeleteAt)

	list, err := ss.AIAgent().GetForScopes(rctx, []string{team})
	require.NoError(t, err)
	assert.Empty(t, list)

	var nfErr *store.ErrNotFound
	assert.ErrorAs(t, ss.AIAgent().Delete(rctx, a.UserId, model.GetMillis()), &nfErr, "deleted once")
	_, err = ss.AIAgent().Update(rctx, got)
	assert.ErrorAs(t, err, &nfErr, "a deleted agent isn't updated")
}

func testAgentActivitySaveGet(t *testing.T, rctx request.CTX, ss store.Store) {
	agent, other := model.NewId(), model.NewId()
	var nfErr *store.ErrNotFound
	_, err := ss.AgentActivity().Get(rctx, agent)
	assert.ErrorAs(t, err, &nfErr)

	thread := model.NewId()
	require.NoError(t, ss.AgentActivity().Save(rctx, &model.AgentActivity{
		UserId: agent, State: model.AgentStateWorking, Text: "Running the poll tests", ThreadPostId: thread, UpdateAt: 10, ExpiresAt: 20,
	}))
	got, err := ss.AgentActivity().Get(rctx, agent)
	require.NoError(t, err)
	assert.Equal(t, &model.AgentActivity{UserId: agent, State: model.AgentStateWorking, Text: "Running the poll tests", ThreadPostId: thread, UpdateAt: 10, ExpiresAt: 20}, got)

	// Saving again replaces it.
	require.NoError(t, ss.AgentActivity().Save(rctx, &model.AgentActivity{UserId: agent, State: model.AgentStateIdle, UpdateAt: 30}))
	got, err = ss.AgentActivity().Get(rctx, agent)
	require.NoError(t, err)
	assert.Equal(t, model.AgentStateIdle, got.State)
	assert.Empty(t, got.Text)
	assert.Empty(t, got.ThreadPostId)
	assert.Zero(t, got.ExpiresAt)

	require.NoError(t, ss.AgentActivity().Save(rctx, &model.AgentActivity{UserId: other, State: model.AgentStateBlocked, Text: "Needs a key", UpdateAt: 40}))
	many, err := ss.AgentActivity().GetMany(rctx, []string{agent, other, model.NewId()})
	require.NoError(t, err)
	require.Len(t, many, 2)

	none, err := ss.AgentActivity().GetMany(rctx, nil)
	require.NoError(t, err)
	assert.Empty(t, none)

	assert.Error(t, ss.AgentActivity().Save(rctx, &model.AgentActivity{UserId: agent, State: "napping", UpdateAt: 50}), "only the known states")

	require.NoError(t, ss.AgentActivity().Delete(rctx, other))
	_, err = ss.AgentActivity().Get(rctx, other)
	assert.ErrorAs(t, err, &nfErr)
}

// TestAIAgentAgentsPluginUserAgents reads the agents people made in the Agents plugin from its table, leaving out the
// deleted ones; nothing when the table isn't there.
func TestAIAgentAgentsPluginUserAgents(t *testing.T, rctx request.CTX, ss store.Store, s SqlStore) {
	_, err := s.GetMaster().Exec("DROP TABLE IF EXISTS Agents_UserAgents")
	require.NoError(t, err)

	agents, err := ss.AIAgent().GetAgentsPluginUserAgents(rctx)
	require.NoError(t, err)
	assert.Empty(t, agents, "no table: nothing to migrate")

	_, err = s.GetMaster().Exec(`CREATE TABLE Agents_UserAgents (
		ID VARCHAR(26) PRIMARY KEY, BotUserID VARCHAR(26) NOT NULL, CreatorID VARCHAR(26) NOT NULL,
		DisplayName TEXT NOT NULL, Username TEXT NOT NULL, ServiceID TEXT NOT NULL, CustomInstructions TEXT NOT NULL,
		ChannelAccessLevel INT NOT NULL, ChannelIDs TEXT NOT NULL, UserAccessLevel INT NOT NULL, UserIDs TEXT NOT NULL,
		TeamIDs TEXT NOT NULL, AdminUserIDs TEXT NOT NULL DEFAULT '[]', EnabledTools TEXT NOT NULL DEFAULT '[]',
		Model TEXT NOT NULL, EnableVision BOOLEAN NOT NULL DEFAULT false, DisableTools BOOLEAN NOT NULL,
		ReasoningEnabled BOOLEAN NOT NULL, ReasoningEffort TEXT NOT NULL, ThinkingBudget INT NOT NULL,
		CreateAt BIGINT NOT NULL, UpdateAt BIGINT NOT NULL, DeleteAt BIGINT NOT NULL)`)
	require.NoError(t, err)
	defer func() {
		_, dropErr := s.GetMaster().Exec("DROP TABLE IF EXISTS Agents_UserAgents")
		require.NoError(t, dropErr)
	}()

	bot, creator, channel := model.NewId(), model.NewId(), model.NewId()
	_, err = s.GetMaster().Exec(`INSERT INTO Agents_UserAgents (ID, BotUserID, CreatorID, DisplayName, Username, ServiceID,
		CustomInstructions, ChannelAccessLevel, ChannelIDs, UserAccessLevel, UserIDs, TeamIDs, Model, DisableTools,
		ReasoningEnabled, ReasoningEffort, ThinkingBudget, CreateAt, UpdateAt, DeleteAt) VALUES
		($1, $2, $3, 'Researcher', 'researcher', 'svc', 'Be brief.', 1, $4, 0, '[]', '[]', 'gpt-x', false, true, 'high', 0, 1, 1, 0),
		($5, $6, $3, 'Gone', 'gone', 'svc', '', 0, '[]', 0, '[]', '[]', '', false, false, '', 0, 2, 2, 5)`,
		model.NewId(), bot, creator, `["`+channel+`"]`, model.NewId(), model.NewId())
	require.NoError(t, err)

	agents, err = ss.AIAgent().GetAgentsPluginUserAgents(rctx)
	require.NoError(t, err)
	require.Len(t, agents, 1, "the deleted one is left out")
	got := agents[0]
	assert.Equal(t, bot, got.BotUserID)
	assert.Equal(t, "researcher", got.Name)
	assert.Equal(t, "Be brief.", got.CustomInstructions)
	assert.Equal(t, 1, got.ChannelAccessLevel)
	assert.Equal(t, []string{channel}, got.ChannelIDs)
	assert.Equal(t, "gpt-x", got.Model)
	assert.True(t, got.ReasoningEnabled)
	assert.Equal(t, 0, got.MaxToolTurns, "an older table without MaxToolTurns")
	assert.Equal(t, creator, got.CreatorID)
}

// A person's conversations with agents: the threads they started in their direct messages with agents, answered or
// not; the threads elsewhere they follow that an agent replied in; never someone else's, a deleted one, or a thread
// only people replied in. The most recent first, with a cursor.
func testAIAgentConversations(t *testing.T, rctx request.CTX, ss store.Store) {
	me, agent, someone := model.NewId(), model.NewId(), model.NewId()
	dm, channel := model.NewId(), model.NewId()
	now := model.GetMillis()
	save := func(p *model.Post) *model.Post {
		t.Helper()
		saved, err := ss.Post().Save(rctx, p)
		require.NoError(t, err)
		return saved
	}
	follow := func(postID string) {
		t.Helper()
		_, err := ss.Thread().MaintainMembership(me, postID, store.ThreadMembershipOpts{Following: true, UpdateFollowing: true})
		require.NoError(t, err)
	}

	answered := save(&model.Post{UserId: me, ChannelId: dm, Message: "\n  Summarise the launch thread\nplease", CreateAt: now - 5000})
	save(&model.Post{UserId: agent, ChannelId: dm, RootId: answered.Id, Message: "Here it is.", CreateAt: now - 4000})
	waiting := save(&model.Post{UserId: me, ChannelId: dm, Message: "What changed this week?", CreateAt: now - 3000})
	save(&model.Post{UserId: agent, ChannelId: dm, Message: "I started this one myself.", CreateAt: now - 2900})
	deleted := save(&model.Post{UserId: me, ChannelId: dm, Message: "Forget it", CreateAt: now - 2800})
	require.NoError(t, ss.Post().Delete(rctx, deleted.Id, now-2700, me))

	inChannel := save(&model.Post{UserId: someone, ChannelId: channel, Message: "@agent can you plan the release?", CreateAt: now - 6000})
	save(&model.Post{UserId: agent, ChannelId: channel, RootId: inChannel.Id, Message: "Plan: ...", CreateAt: now - 1000})
	follow(inChannel.Id)
	peopleOnly := save(&model.Post{UserId: someone, ChannelId: channel, Message: "Lunch?", CreateAt: now - 7000})
	save(&model.Post{UserId: me, ChannelId: channel, RootId: peopleOnly.Id, Message: "Yes", CreateAt: now - 900})
	follow(peopleOnly.Id)

	opts := model.AIAgentConversationsOptions{UserId: me, DirectChannelIds: []string{dm}, AgentIds: []string{agent}, Limit: 10}
	got, err := ss.AIAgent().GetConversations(rctx, opts)
	require.NoError(t, err)
	ids := make([]string, 0, len(got))
	for _, c := range got {
		ids = append(ids, c.RootId)
	}
	require.Equal(t, []string{inChannel.Id, waiting.Id, answered.Id}, ids, "the most recent first")

	assert.False(t, got[0].Direct)
	assert.Contains(t, got[0].Participants, agent, "with its participants, to tell the agent")
	assert.Equal(t, int64(1), got[0].ReplyCount)
	assert.True(t, got[1].Direct)
	assert.Zero(t, got[1].ReplyCount, "not answered yet")
	assert.Equal(t, "Summarise the launch thread", got[2].Title, "the first line of its first message")
	assert.Equal(t, int64(1), got[2].ReplyCount)

	opts.Before = got[1].LastActivityAt()
	got, err = ss.AIAgent().GetConversations(rctx, opts)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, answered.Id, got[0].RootId, "only what moved before the cursor")

	opts.Before, opts.Limit = 0, 1
	got, err = ss.AIAgent().GetConversations(rctx, opts)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, inChannel.Id, got[0].RootId)

	got, err = ss.AIAgent().GetConversations(rctx, model.AIAgentConversationsOptions{UserId: someone, DirectChannelIds: []string{dm}, AgentIds: []string{agent}, Limit: 10})
	require.NoError(t, err)
	assert.Empty(t, got, "never someone else's direct message, nor a thread they don't follow")
}
