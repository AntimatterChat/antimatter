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

func TestAgentsPluginAgentToAIAgent(t *testing.T) {
	connection := model.NewId()
	exists := func(id string) bool { return id == connection }
	user, channel, team, creator := model.NewId(), model.NewId(), model.NewId(), model.NewId()

	t.Run("open to everyone, with its service, prompt and reasoning", func(t *testing.T) {
		agent := agentsPluginAgentToAIAgent(user, &model.AIAgentsPluginAgent{
			ServiceID: connection, Model: "gpt-x", CustomInstructions: "Be kind.",
			ReasoningEnabled: true, ReasoningEffort: "High", ThinkingBudget: 4096, MaxToolTurns: 900, CreatorID: creator,
		}, exists)
		assert.Equal(t, user, agent.UserId)
		assert.Equal(t, model.AIAgentScopeServer, agent.Scope)
		assert.Equal(t, connection, agent.ConnectionId)
		assert.Equal(t, "gpt-x", agent.Model)
		assert.Equal(t, "Be kind.", agent.SystemPrompt)
		assert.True(t, agent.Reasoning.Enabled)
		assert.Equal(t, model.AIAgentReasoningEffortHigh, agent.Reasoning.Effort)
		assert.Equal(t, 4096, agent.Reasoning.BudgetTokens)
		assert.Equal(t, model.AIAgentMaxToolTurns, agent.MaxToolTurns, "clamped")
		assert.True(t, agent.Tools.Enabled)
		assert.True(t, agent.Access.Everyone)
		assert.Equal(t, creator, agent.CreatorId)
		assert.Empty(t, agent.NeedsAttention)
		agent.PreSave()
		assert.Nil(t, agent.IsValid())
	})

	t.Run("an allow list of people and communities", func(t *testing.T) {
		agent := agentsPluginAgentToAIAgent(user, &model.AIAgentsPluginAgent{ServiceID: connection, UserAccessLevel: 1, UserIDs: []string{user, "not-an-id"}, TeamIDs: []string{team}}, exists)
		assert.False(t, agent.Access.Everyone)
		assert.Equal(t, []string{user}, agent.Access.UserIDs, "only valid IDs")
		assert.Equal(t, []string{team}, agent.Access.TeamIDs)
		assert.Empty(t, agent.NeedsAttention)
	})

	t.Run("where it answered becomes who may talk to it, and needs attention", func(t *testing.T) {
		agent := agentsPluginAgentToAIAgent(user, &model.AIAgentsPluginAgent{ServiceID: connection, ChannelAccessLevel: 1, ChannelIDs: []string{channel}}, exists)
		assert.False(t, agent.Access.Everyone, "never wider than the plugin's")
		assert.Equal(t, []string{channel}, agent.Access.ChannelIDs)
		assert.NotEmpty(t, agent.NeedsAttention)
	})

	for name, src := range map[string]*model.AIAgentsPluginAgent{
		"a block list of people":    {ServiceID: connection, UserAccessLevel: 2, UserIDs: []string{user}},
		"nobody":                    {ServiceID: connection, UserAccessLevel: 3},
		"attribute-based access":    {ServiceID: connection, UserAccessLevel: 4},
		"a block list of channels":  {ServiceID: connection, ChannelAccessLevel: 2, ChannelIDs: []string{channel}},
		"no channel":                {ServiceID: connection, ChannelAccessLevel: 3},
		"people allowed, no places": {ServiceID: connection, UserAccessLevel: 1, UserIDs: []string{user}, ChannelAccessLevel: 3},
	} {
		t.Run(name+" becomes nobody, and needs attention", func(t *testing.T) {
			agent := agentsPluginAgentToAIAgent(user, src, exists)
			assert.False(t, agent.Access.Everyone)
			assert.Empty(t, agent.Access.UserIDs)
			assert.Empty(t, agent.Access.TeamIDs)
			assert.Empty(t, agent.Access.ChannelIDs)
			assert.NotEmpty(t, agent.NeedsAttention)
		})
	}

	t.Run("a service that didn't become a connection", func(t *testing.T) {
		agent := agentsPluginAgentToAIAgent(user, &model.AIAgentsPluginAgent{ServiceID: model.NewId()}, exists)
		assert.Empty(t, agent.ConnectionId)
		assert.Contains(t, agent.NeedsAttention, "connection")
		agent.PreSave()
		assert.Nil(t, agent.IsValid(), "saved, needing attention")
	})
}

// makePluginBot makes a bot owned by the Agents plugin as an older server did: a plain bot.
func makePluginBot(t *testing.T, th *TestHelper, username string) *model.Bot {
	t.Helper()
	bot := &model.Bot{Username: username, DisplayName: strings.ToUpper(username), OwnerId: model.AIAgentsPluginId, Kind: model.BotKindBot}
	user, err := th.App.Srv().Store().User().Save(th.Context, model.UserFromBot(bot))
	require.NoError(t, err)
	bot.UserId = user.Id
	saved, err := th.App.Srv().Store().Bot().Save(bot)
	require.NoError(t, err)
	require.Equal(t, model.BotKindBot, saved.Kind)
	return saved
}

func TestAgentsFromBotsMigration(t *testing.T) {
	th, _, c := setupAgentTest(t, "")
	master := th.GetSqlStore().GetMaster()
	for _, table := range []string{"Agents_ConfigHistory", "Agents_UserAgents"} {
		_, err := master.Exec("DROP TABLE IF EXISTS " + table)
		require.NoError(t, err)
	}
	_, err := master.Exec(`CREATE TABLE Agents_ConfigHistory (
		ID VARCHAR(26) PRIMARY KEY, Config TEXT NOT NULL, CreateAt BIGINT NOT NULL, Active BOOLEAN NOT NULL DEFAULT false)`)
	require.NoError(t, err)
	_, err = master.Exec(`CREATE TABLE Agents_UserAgents (
		ID VARCHAR(26) PRIMARY KEY, BotUserID VARCHAR(26) NOT NULL, CreatorID VARCHAR(26) NOT NULL,
		DisplayName TEXT NOT NULL, Username TEXT NOT NULL, ServiceID TEXT NOT NULL, CustomInstructions TEXT NOT NULL,
		ChannelAccessLevel INT NOT NULL, ChannelIDs TEXT NOT NULL, UserAccessLevel INT NOT NULL, UserIDs TEXT NOT NULL,
		TeamIDs TEXT NOT NULL, Model TEXT NOT NULL, DisableTools BOOLEAN NOT NULL, ReasoningEnabled BOOLEAN NOT NULL,
		ReasoningEffort TEXT NOT NULL, ThinkingBudget INT NOT NULL, MaxToolTurns INT NOT NULL DEFAULT 0,
		CreateAt BIGINT NOT NULL, UpdateAt BIGINT NOT NULL, DeleteAt BIGINT NOT NULL)`)
	require.NoError(t, err)
	t.Cleanup(func() {
		for _, table := range []string{"Agents_ConfigHistory", "Agents_UserAgents"} {
			_, dropErr := master.Exec("DROP TABLE IF EXISTS " + table)
			require.NoError(t, dropErr)
		}
	})

	configured := makePluginBot(t, th, "ai"+strings.ToLower(model.NewId()[:8]))
	custom := makePluginBot(t, th, "scout"+strings.ToLower(model.NewId()[:8]))
	unknown := makePluginBot(t, th, "spare"+strings.ToLower(model.NewId()[:8]))
	plain, appErr := th.App.CreateBot(th.Context, &model.Bot{Username: "release" + strings.ToLower(model.NewId()[:8]), OwnerId: th.BasicUser.Id})
	require.Nil(t, appErr)

	config, _ := json.Marshal(map[string]any{
		"bots": []map[string]any{{"name": configured.Username, "serviceID": c.Id, "model": "gpt-x", "customInstructions": "Be kind.", "channelAccessLevel": 0, "userAccessLevel": 0}},
	})
	_, err = master.Exec("INSERT INTO Agents_ConfigHistory (ID, Config, CreateAt, Active) VALUES ($1, $2, 1, true)", model.NewId(), string(config))
	require.NoError(t, err)
	channel := th.BasicChannel.Id
	_, err = master.Exec(`INSERT INTO Agents_UserAgents (ID, BotUserID, CreatorID, DisplayName, Username, ServiceID, CustomInstructions,
		ChannelAccessLevel, ChannelIDs, UserAccessLevel, UserIDs, TeamIDs, Model, DisableTools, ReasoningEnabled, ReasoningEffort,
		ThinkingBudget, MaxToolTurns, CreateAt, UpdateAt, DeleteAt)
		VALUES ($1, $2, $3, 'Scout', $4, $5, 'Look things up.', 1, $6, 0, '[]', '[]', 'claude-x', true, false, '', 0, 7, 1, 1, 0)`,
		model.NewId(), custom.UserId, th.BasicUser.Id, custom.Username, c.Id, `["`+channel+`"]`)
	require.NoError(t, err)

	_, err = th.App.Srv().Store().System().PermanentDeleteByName(agentsFromBotsMigrationKey)
	require.NoError(t, err)
	require.NoError(t, th.App.Srv().doAgentsFromBotsMigration())

	for _, bot := range []*model.Bot{configured, custom, unknown} {
		got, getErr := th.App.GetBot(th.Context, bot.UserId, false)
		require.Nil(t, getErr)
		assert.Equal(t, model.BotKindAgent, got.Kind, "the plugin's bot %s is an agent", bot.Username)
		user, getErr := th.App.GetUser(th.Context, bot.UserId)
		require.Nil(t, getErr)
		assert.True(t, user.IsAgent, "and its account says so")
	}
	got, appErr := th.App.GetBot(th.Context, plain.UserId, false)
	require.Nil(t, appErr)
	assert.Equal(t, model.BotKindBot, got.Kind, "someone else's bot stays a bot")

	agent, appErr := th.App.GetAIAgent(th.Context, configured.UserId)
	require.Nil(t, appErr, "the configuration's bot has an agent's settings")
	assert.Equal(t, c.Id, agent.ConnectionId)
	assert.Equal(t, "gpt-x", agent.Model)
	assert.Equal(t, "Be kind.", agent.SystemPrompt)
	assert.True(t, agent.Access.Everyone)
	assert.Empty(t, agent.NeedsAttention)

	agent, appErr = th.App.GetAIAgent(th.Context, custom.UserId)
	require.Nil(t, appErr, "the agent someone made in the plugin too")
	assert.Equal(t, "claude-x", agent.Model)
	assert.Equal(t, "Look things up.", agent.SystemPrompt)
	assert.False(t, agent.Tools.Enabled, "its tools were off")
	assert.Equal(t, 7, agent.MaxToolTurns)
	assert.Equal(t, th.BasicUser.Id, agent.CreatorId)
	assert.Equal(t, []string{channel}, agent.Access.ChannelIDs)
	assert.NotEmpty(t, agent.NeedsAttention, "where it answered changed meaning")

	agent, appErr = th.App.GetAIAgent(th.Context, unknown.UserId)
	require.Nil(t, appErr, "a bot the plugin no longer configures is an agent nobody talks to")
	assert.False(t, agent.Access.Everyone)
	assert.NotEmpty(t, agent.NeedsAttention)

	done, err := th.App.Srv().Store().System().GetByName(agentsFromBotsMigrationKey)
	require.NoError(t, err)
	assert.Equal(t, "true", done.Value)

	// Once only: a second start changes nothing.
	require.Nil(t, th.App.DeleteAIAgent(th.Context, agent))
	require.NoError(t, th.App.Srv().doAgentsFromBotsMigration())
	_, appErr = th.App.GetAIAgent(th.Context, unknown.UserId)
	require.NotNil(t, appErr, "a deleted agent isn't brought back")
}

// A fresh server ends up as an upgraded one: the AI chat app's bots are agents either way.
func TestAgentsFromBotsFreshEqualsUpgraded(t *testing.T) {
	th := Setup(t).InitBasic(t)
	_, err := th.GetSqlStore().GetMaster().Exec("DROP TABLE IF EXISTS Agents_ConfigHistory")
	require.NoError(t, err)

	// Upgraded: a bot the plugin made on an older server, then the migration.
	older := makePluginBot(t, th, "older"+strings.ToLower(model.NewId()[:8]))
	_, err = th.App.Srv().Store().System().PermanentDeleteByName(agentsFromBotsMigrationKey)
	require.NoError(t, err)
	require.NoError(t, th.App.Srv().doAgentsFromBotsMigration())

	// Fresh: the migration already done, the app makes its bot afterwards.
	newer, appErr := th.App.CreateBot(th.Context, &model.Bot{Username: "newer" + strings.ToLower(model.NewId()[:8]), OwnerId: model.AIAgentsPluginId})
	require.Nil(t, appErr)

	for _, id := range []string{older.UserId, newer.UserId} {
		bot, appErr := th.App.GetBot(th.Context, id, false)
		require.Nil(t, appErr)
		assert.Equal(t, model.BotKindAgent, bot.Kind)
		user, appErr := th.App.GetUser(th.Context, id)
		require.Nil(t, appErr)
		assert.True(t, user.IsAgent)
	}
}

// Antimatter: on upgrade, the community's admins get the management of their community's agents; nobody else gets
// anything new.
func TestAntimatterTeamAgentPermissionsMigration(t *testing.T) {
	th := SetupWithStoreMock(t)
	migrationMap, err := th.App.getAntimatterTeamAgentPermissionsMigration()
	require.NoError(t, err)

	teamAdmin := &model.Role{Name: model.TeamAdminRoleId, Permissions: []string{model.PermissionManageTeam.Id}}
	teamUser := &model.Role{Name: model.TeamUserRoleId, Permissions: []string{model.PermissionViewTeam.Id}}
	systemUser := &model.Role{Name: model.SystemUserRoleId, Permissions: []string{model.PermissionManageOwnAgent.Id}}
	roles := []*model.Role{teamAdmin, teamUser, systemUser}
	for _, role := range roles {
		for _, transformation := range migrationMap {
			if transformation.On(role, map[string]map[string]bool{}) {
				role.Permissions = append(role.Permissions, transformation.Add...)
			}
		}
	}
	assert.ElementsMatch(t, []string{model.PermissionManageTeam.Id, model.PermissionManageOwnAgent.Id, model.PermissionManageOthersAgent.Id}, teamAdmin.Permissions)
	assert.ElementsMatch(t, []string{model.PermissionViewTeam.Id}, teamUser.Permissions, "members get nothing")
	assert.ElementsMatch(t, []string{model.PermissionManageOwnAgent.Id}, systemUser.Permissions)
}
