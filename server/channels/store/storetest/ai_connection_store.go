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

func TestAIConnectionStore(t *testing.T, rctx request.CTX, ss store.Store) {
	t.Run("SaveGetUpdate", func(t *testing.T) { testAIConnectionSaveGetUpdate(t, rctx, ss) })
	t.Run("Scopes", func(t *testing.T) { testAIConnectionScopes(t, rctx, ss) })
	t.Run("Delete", func(t *testing.T) { testAIConnectionDelete(t, rctx, ss) })
	t.Run("Usage", func(t *testing.T) { testAIConnectionUsage(t, rctx, ss) })
	t.Run("AnonymizeUsageByUser", func(t *testing.T) { testAIConnectionAnonymizeUsageByUser(t, rctx, ss) })
}

func testAIConnectionAnonymizeUsageByUser(t *testing.T, rctx request.CTX, ss store.Store) {
	connectionID, user, other := model.NewId(), model.NewId(), model.NewId()
	add := func(userID, purpose string, day int, in, out int64) {
		t.Helper()
		require.NoError(t, ss.AIConnection().AddUsage(rctx, &model.AIUsage{ConnectionId: connectionID, UserId: userID, Purpose: purpose, Day: day, Requests: 1, InputTokens: in, OutputTokens: out}))
	}
	add(user, "", 20261001, 10, 5)
	add(user, "rewrite", 20261002, 7, 3)
	add(other, "", 20261001, 100, 50)
	add("", "", 20261001, 1, 1) // already nobody's: theirs adds to it

	before, err := ss.AIConnection().GetUsageTotals(rctx, connectionID, 20261001)
	require.NoError(t, err)

	require.NoError(t, ss.AIConnection().AnonymizeUsageByUser(rctx, user))
	require.NoError(t, ss.AIConnection().AnonymizeUsageByUser(rctx, user), "again is fine")

	after, err := ss.AIConnection().GetUsageTotals(rctx, connectionID, 20261001)
	require.NoError(t, err)
	assert.Equal(t, before, after, "the connection's totals stay right")

	theirs, err := ss.AIConnection().GetUserUsageTotals(rctx, connectionID, user, 20261001)
	require.NoError(t, err)
	assert.Equal(t, model.AIUsageTotals{}, theirs, "nothing is theirs any more")

	nobodys, err := ss.AIConnection().GetUserUsageTotals(rctx, connectionID, "", 20261001)
	require.NoError(t, err)
	assert.Equal(t, model.AIUsageTotals{Requests: 3, InputTokens: 18, OutputTokens: 9}, nobodys)

	others, err := ss.AIConnection().GetUserUsageTotals(rctx, connectionID, other, 20261001)
	require.NoError(t, err)
	assert.Equal(t, model.AIUsageTotals{Requests: 1, InputTokens: 100, OutputTokens: 50}, others)
}

func newTestAIConnection(scope string) *model.AIConnection {
	return &model.AIConnection{
		Name:             "Connection " + model.NewId()[:6],
		Scope:            scope,
		Provider:         model.AIProviderOpenAICompatible,
		BaseURL:          "https://llm.example.com/v1",
		APIKeySealed:     "sealed-key",
		APIKeyLast4:      "wxyz",
		ExtraHeaderNames: []string{"X-Org"},
		Models:           model.AIConnectionModels{Chat: "chat-model", Embeddings: "embed-model"},
		Limits:           model.AIConnectionLimits{RequestsPerMinute: 60, MonthlyTokenBudget: 1000},
		Audience:         model.AIConnectionAudience{Everyone: true},
	}
}

func testAIConnectionSaveGetUpdate(t *testing.T, rctx request.CTX, ss store.Store) {
	team := model.NewId()
	c := newTestAIConnection(model.AIConnectionScopeServer)
	c.Sharing = model.AIConnectionSharing{TeamIDs: []string{team}}
	c.Audience.GroupIDs = []string{model.NewId()}
	c.CreatorId = model.NewId()
	saved, err := ss.AIConnection().Save(rctx, c)
	require.NoError(t, err)
	require.True(t, model.IsValidId(saved.Id))

	got, err := ss.AIConnection().Get(rctx, saved.Id)
	require.NoError(t, err)
	assert.Equal(t, saved.Name, got.Name)
	assert.Equal(t, "sealed-key", got.APIKeySealed)
	assert.True(t, got.APIKeySet)
	assert.Equal(t, "wxyz", got.APIKeyLast4)
	assert.Equal(t, []string{"X-Org"}, got.ExtraHeaderNames)
	assert.Equal(t, model.AIConnectionModels{Chat: "chat-model", Embeddings: "embed-model"}, got.Models)
	assert.Equal(t, int64(1000), got.Limits.MonthlyTokenBudget)
	assert.Equal(t, []string{team}, got.Sharing.TeamIDs)
	assert.Equal(t, model.AIDecisionsDialectOpenAI, got.DecisionsDialect)
	assert.Equal(t, c.CreatorId, got.CreatorId)

	got.Name = "Renamed"
	got.APIKeySealed = ""
	got.Scope = model.NewId() // ignored: the scope never changes
	got.Sharing = model.AIConnectionSharing{}
	got.UpdaterId = model.NewId()
	_, err = ss.AIConnection().Update(rctx, got)
	require.NoError(t, err)
	again, err := ss.AIConnection().Get(rctx, saved.Id)
	require.NoError(t, err)
	assert.Equal(t, "Renamed", again.Name)
	assert.False(t, again.APIKeySet)
	assert.Empty(t, again.APIKeyLast4, "no key, no last characters")
	assert.Equal(t, model.AIConnectionScopeServer, again.Scope)
	assert.Equal(t, got.UpdaterId, again.UpdaterId)
	assert.Equal(t, c.CreatorId, again.CreatorId)

	_, err = ss.AIConnection().Get(rctx, model.NewId())
	var nfErr *store.ErrNotFound
	assert.ErrorAs(t, err, &nfErr)

	invalid := newTestAIConnection("not a scope")
	_, err = ss.AIConnection().Save(rctx, invalid)
	assert.Error(t, err)
}

func testAIConnectionScopes(t *testing.T, rctx request.CTX, ss store.Store) {
	team, other := model.NewId(), model.NewId()
	server, err := ss.AIConnection().Save(rctx, newTestAIConnection(model.AIConnectionScopeServer))
	require.NoError(t, err)
	community, err := ss.AIConnection().Save(rctx, newTestAIConnection(team))
	require.NoError(t, err)
	elsewhere, err := ss.AIConnection().Save(rctx, newTestAIConnection(other))
	require.NoError(t, err)

	ids := func(list []*model.AIConnection) []string {
		out := []string{}
		for _, c := range list {
			out = append(out, c.Id)
		}
		return out
	}
	list, err := ss.AIConnection().GetForScopes(rctx, []string{team})
	require.NoError(t, err)
	assert.Equal(t, []string{community.Id}, ids(list))

	list, err = ss.AIConnection().GetForScopes(rctx, []string{model.AIConnectionScopeServer, team})
	require.NoError(t, err)
	assert.Contains(t, ids(list), server.Id)
	assert.Contains(t, ids(list), community.Id)
	assert.NotContains(t, ids(list), elsewhere.Id)

	list, err = ss.AIConnection().GetForScopes(rctx, nil)
	require.NoError(t, err)
	assert.Empty(t, list)

	list, err = ss.AIConnection().GetAll(rctx)
	require.NoError(t, err)
	assert.Subset(t, ids(list), []string{server.Id, community.Id, elsewhere.Id})
}

func testAIConnectionDelete(t *testing.T, rctx request.CTX, ss store.Store) {
	team := model.NewId()
	c, err := ss.AIConnection().Save(rctx, newTestAIConnection(team))
	require.NoError(t, err)
	require.NoError(t, ss.AIConnection().Delete(rctx, c.Id, model.GetMillis()))

	got, err := ss.AIConnection().Get(rctx, c.Id)
	require.NoError(t, err)
	assert.NotZero(t, got.DeleteAt)
	assert.Empty(t, got.APIKeySealed, "a deleted connection keeps no key")
	assert.Empty(t, got.ExtraHeadersSealed)

	list, err := ss.AIConnection().GetForScopes(rctx, []string{team})
	require.NoError(t, err)
	assert.Empty(t, list)

	var nfErr *store.ErrNotFound
	assert.ErrorAs(t, ss.AIConnection().Delete(rctx, c.Id, model.GetMillis()), &nfErr, "deleted once")
	_, err = ss.AIConnection().Update(rctx, got)
	assert.ErrorAs(t, err, &nfErr, "a deleted connection can't be changed")
}

func testAIConnectionUsage(t *testing.T, rctx request.CTX, ss store.Store) {
	connectionID, user, other := model.NewId(), model.NewId(), model.NewId()
	add := func(userID, purpose string, day int, in, out int64) {
		require.NoError(t, ss.AIConnection().AddUsage(rctx, &model.AIUsage{ConnectionId: connectionID, UserId: userID, Purpose: purpose, Day: day, Requests: 1, InputTokens: in, OutputTokens: out}))
	}
	add(user, "", 20260930, 1000, 1000) // last month
	add(user, "", 20261001, 10, 5)
	add(user, "", 20261001, 20, 5) // the same day adds up
	add(user, "rewrite", 20261002, 7, 3)
	add(other, "", 20261003, 100, 50)
	add("", "auto_translation", 20261003, 1, 1)

	totals, err := ss.AIConnection().GetUsageTotals(rctx, connectionID, 20261001)
	require.NoError(t, err)
	assert.Equal(t, model.AIUsageTotals{Requests: 5, InputTokens: 138, OutputTokens: 64}, totals)

	mine, err := ss.AIConnection().GetUserUsageTotals(rctx, connectionID, user, 20261001)
	require.NoError(t, err)
	assert.Equal(t, model.AIUsageTotals{Requests: 3, InputTokens: 37, OutputTokens: 13}, mine)

	none, err := ss.AIConnection().GetUsageTotals(rctx, model.NewId(), 20261001)
	require.NoError(t, err)
	assert.Equal(t, model.AIUsageTotals{}, none)

	rows, err := ss.AIConnection().GetUsage(rctx, connectionID, 20261001)
	require.NoError(t, err)
	require.Len(t, rows, 4)
	assert.Equal(t, 20261001, rows[0].Day)
	assert.Equal(t, int64(2), rows[0].Requests)
}

// TestAIConnectionAgentsPluginConfig reads the Agents plugin's active configuration from its table, and only that
// row: never the history, which keeps older keys.
func TestAIConnectionAgentsPluginConfig(t *testing.T, rctx request.CTX, ss store.Store, s SqlStore) {
	_, err := s.GetMaster().Exec("DROP TABLE IF EXISTS Agents_ConfigHistory")
	require.NoError(t, err)

	config, err := ss.AIConnection().GetAgentsPluginConfig(rctx)
	require.NoError(t, err)
	assert.Empty(t, config, "no table: nothing to migrate")

	_, err = s.GetMaster().Exec(`CREATE TABLE Agents_ConfigHistory (
		ID VARCHAR(26) PRIMARY KEY, Config TEXT NOT NULL, CreateAt BIGINT NOT NULL, Active BOOLEAN NOT NULL DEFAULT false)`)
	require.NoError(t, err)
	defer func() {
		_, dropErr := s.GetMaster().Exec("DROP TABLE IF EXISTS Agents_ConfigHistory")
		require.NoError(t, dropErr)
	}()

	config, err = ss.AIConnection().GetAgentsPluginConfig(rctx)
	require.NoError(t, err)
	assert.Empty(t, config, "no active row")

	_, err = s.GetMaster().Exec("INSERT INTO Agents_ConfigHistory (ID, Config, CreateAt, Active) VALUES ($1, $2, 1, false), ($3, $4, 2, true)",
		model.NewId(), `{"services":[{"apiKey":"old-history-key"}]}`, model.NewId(), `{"services":[{"apiKey":"current-key"}]}`)
	require.NoError(t, err)
	config, err = ss.AIConnection().GetAgentsPluginConfig(rctx)
	require.NoError(t, err)
	assert.Contains(t, config, "current-key")
	assert.NotContains(t, config, "old-history-key")
}
