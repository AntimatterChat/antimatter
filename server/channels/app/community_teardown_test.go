// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// A community's permanent deletion takes with it what Antimatter keeps for it: its agents, MCP servers and model
// connections with their secrets, its roles and teams, its attributes. The server's and other communities' stay.
func TestPermanentDeleteTeamForgetsCommunityData(t *testing.T) {
	th, f, serverConnection := setupAgentTest(t, "")
	ss := th.Server.Store()
	team, other := th.CreateTeam(t), th.CreateTeam(t)
	th.LinkUserToTeam(t, th.BasicUser, team)

	newMCPServer := func(scope string) *model.AIMCPServer {
		t.Helper()
		server, err := ss.AIMCPServer().Save(th.Context, &model.AIMCPServer{
			Name:                    "Jira",
			Slug:                    "s" + model.NewId()[:20],
			Scope:                   scope,
			URL:                     "https://mcp.example.com/mcp",
			Auth:                    model.AIMCPAuthOAuth,
			OAuthClientSecretSealed: "sealed-secret",
		})
		require.NoError(t, err)
		require.NoError(t, ss.AIMCPServer().SaveUserToken(th.Context, &model.AIMCPUserToken{ServerId: server.Id, UserId: th.BasicUser.Id, TokenSealed: "token"}))
		return server
	}
	newRole := func(teamID string) *model.Group {
		t.Helper()
		role, err := ss.Group().Create(&model.Group{Name: new(model.NewId()), DisplayName: "Moderators", Source: model.GroupSourceCustom, AllowReference: true, TeamId: teamID})
		require.NoError(t, err)
		_, err = ss.Group().UpsertMember(role.Id, th.BasicUser.Id)
		require.NoError(t, err)
		return role
	}

	connection := createTestAIConnection(t, th, f, team.Id, nil)
	agent := createTestAgent(t, th, team.Id, connection.Id, nil).AIAgent
	mcpServer := newMCPServer(team.Id)
	role := newRole(team.Id)
	_, appErr := th.App.CreateCommunityAttribute(th.Context, team.Id, th.BasicUser.Id, &model.PropertyField{Name: "Pronouns", Type: model.PropertyFieldTypeText})
	require.Nil(t, appErr)

	othersConnection := createTestAIConnection(t, th, f, other.Id, nil)
	othersMCPServer := newMCPServer(other.Id)
	othersRole := newRole(other.Id)
	_, appErr = th.App.CreateCommunityAttribute(th.Context, other.Id, th.BasicUser.Id, &model.PropertyField{Name: "Pronouns", Type: model.PropertyFieldTypeText})
	require.Nil(t, appErr)

	require.Nil(t, th.App.PermanentDeleteTeam(th.Context, team))

	var nfErr *store.ErrNotFound
	_, err := ss.AIConnection().Get(th.Context, connection.Id)
	assert.ErrorAs(t, err, &nfErr, "its model connection is gone, with its key")
	_, err = ss.AIMCPServer().Get(th.Context, mcpServer.Id)
	assert.ErrorAs(t, err, &nfErr, "its MCP server is gone, with its secrets")
	_, err = ss.AIMCPServer().GetUserToken(th.Context, mcpServer.Id, th.BasicUser.Id)
	assert.ErrorAs(t, err, &nfErr, "and the sign-ins to it")
	_, err = ss.AIAgent().Get(th.Context, agent.UserId)
	assert.ErrorAs(t, err, &nfErr, "its agent is gone")
	bot, appErr := th.App.GetBot(th.Context, agent.UserId, true)
	require.Nil(t, appErr)
	assert.NotZero(t, bot.DeleteAt, "and its account deactivated")
	_, err = ss.Group().Get(role.Id)
	assert.ErrorAs(t, err, &nfErr, "its role is gone")
	_, err = ss.Group().GetMember(role.Id, th.BasicUser.Id)
	assert.Error(t, err, "with its members")
	attributes, appErr := th.App.GetCommunityAttributes(th.Context, team.Id)
	require.Nil(t, appErr)
	assert.Empty(t, attributes, "its attributes are gone")

	for _, id := range []string{serverConnection.Id, othersConnection.Id} {
		_, err = ss.AIConnection().Get(th.Context, id)
		assert.NoError(t, err, "the server's and other communities' connections stay")
	}
	_, err = ss.AIMCPServer().Get(th.Context, othersMCPServer.Id)
	assert.NoError(t, err)
	_, err = ss.AIMCPServer().GetUserToken(th.Context, othersMCPServer.Id, th.BasicUser.Id)
	assert.NoError(t, err)
	_, err = ss.Group().GetMember(othersRole.Id, th.BasicUser.Id)
	assert.NoError(t, err)
	attributes, appErr = th.App.GetCommunityAttributes(th.Context, other.Id)
	require.Nil(t, appErr)
	assert.Len(t, attributes, 1)
}
