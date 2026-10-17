// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestAgentsConfigToConnections(t *testing.T) {
	openaiID, ollamaID, claudeID, bedrockID, azureID := model.NewId(), model.NewId(), model.NewId(), model.NewId(), model.NewId()
	cfg := agentsPluginConfig{
		Services: []agentsPluginService{
			{ID: openaiID, Name: "OpenAI", Type: "openai", APIKey: "sk-openai-key-123456", OrgID: "org-1", DefaultModel: "gpt-test"},
			{ID: ollamaID, Name: "Ollama", Type: "openaicompatible", APIURL: "http://ollama:11434", DefaultModel: "qwen"},
			{ID: claudeID, Name: "Claude", Type: "anthropic", APIKey: "sk-ant-key", DefaultModel: "claude-test"},
			{ID: bedrockID, Name: "Bedrock", Type: "bedrock", AWSAccessKeyID: "AKIA", AWSSecretAccessKey: "aws-secret", DefaultModel: "anthropic.claude"},
			{ID: azureID, Name: "Azure", Type: "azure", APIURL: "https://lumen.openai.azure.com/", APIKey: "azure-key", DefaultModel: "gpt-4o"},
			{ID: "8c1f1c4e-5bd8-4d0e-9a63-5d8e0f7c2a11", Name: "Legacy", Type: "mistral", APIKey: "mistral-key"},
			{ID: model.NewId(), Name: "Mock", Type: "loadtest_mock"},
		},
		Bots: []agentsPluginBot{
			{Name: "ai", ServiceID: openaiID, Model: "gpt-bot"},
			{Name: "scribe", ServiceID: ollamaID},
		},
		DefaultBotName:      "ai",
		TranscriptGenerator: "scribe",
	}
	cfg.EmbeddingSearchConfig.EmbeddingProvider.Type = "openai"
	cfg.EmbeddingSearchConfig.EmbeddingProvider.Parameters = json.RawMessage(`{"apiKey":"sk-openai-key-123456","embeddingModel":"text-embedding-3-small"}`)

	connections, featureConnection, featureModel := agentsConfigToConnections(cfg)
	require.Len(t, connections, 6, "the load-test mock is left out")
	byName := map[string]*migratedAIConnection{}
	for _, m := range connections {
		byName[m.connection.Name] = m
		assert.True(t, m.connection.Sharing.AllTeams, "shared with every community, as the plugin's services were")
		assert.True(t, m.connection.Audience.Everyone)
		assert.Equal(t, model.AIConnectionScopeServer, m.connection.Scope)
	}

	openai := byName["OpenAI"]
	assert.Equal(t, openaiID, openai.connection.Id, "the service's ID is kept")
	assert.Equal(t, "https://api.openai.com/v1", openai.connection.BaseURL)
	assert.Equal(t, "sk-openai-key-123456", openai.apiKey)
	assert.Equal(t, "org-1", openai.headers["OpenAI-Organization"])
	assert.Equal(t, "gpt-test", openai.connection.Models.Chat)
	assert.Equal(t, "text-embedding-3-small", openai.connection.Models.Embeddings, "the embedding provider with the same address and key")
	assert.Empty(t, openai.connection.NeedsAttention)

	ollama := byName["Ollama"]
	assert.Equal(t, "http://ollama:11434/v1", ollama.connection.BaseURL)
	assert.Equal(t, "whisper-1", ollama.connection.Models.Transcription, "the transcription bot's service")

	claude := byName["Claude"]
	assert.Equal(t, model.AIProviderAnthropic, claude.connection.Provider)
	assert.Empty(t, claude.connection.NeedsAttention)

	bedrock := byName["Bedrock"]
	assert.NotEmpty(t, bedrock.connection.NeedsAttention)
	assert.Empty(t, bedrock.apiKey, "AWS credentials aren't carried over")
	for _, v := range bedrock.headers {
		assert.NotEqual(t, "aws-secret", v)
	}

	azure := byName["Azure"]
	assert.Equal(t, "https://lumen.openai.azure.com/openai/v1", azure.connection.BaseURL)
	assert.Equal(t, "azure-key", azure.headers["api-key"])
	assert.Empty(t, azure.apiKey)
	assert.NotEmpty(t, azure.connection.NeedsAttention, "an admin checks the address")

	legacy := byName["Legacy"]
	assert.True(t, model.IsValidId(legacy.connection.Id), "a legacy UUID gets a new ID")
	assert.Equal(t, "https://api.mistral.ai/v1", legacy.connection.BaseURL)

	assert.Equal(t, openaiID, featureConnection, "the default bot's service")
	assert.Equal(t, "gpt-bot", featureModel)
}

func TestAIConnectionsFromAgentsMigration(t *testing.T) {
	th := Setup(t)
	master := th.GetSqlStore().GetMaster()
	_, err := master.Exec("DROP TABLE IF EXISTS Agents_ConfigHistory")
	require.NoError(t, err)
	_, err = master.Exec(`CREATE TABLE Agents_ConfigHistory (
		ID VARCHAR(26) PRIMARY KEY, Config TEXT NOT NULL, CreateAt BIGINT NOT NULL, Active BOOLEAN NOT NULL DEFAULT false)`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := master.Exec("DROP TABLE IF EXISTS Agents_ConfigHistory")
		require.NoError(t, err)
	})

	serviceID, vertexID := model.NewId(), model.NewId()
	active := map[string]any{
		"services": []map[string]any{
			{"id": serviceID, "name": "Lumen OpenAI", "type": "openaicompatible", "apiURL": "https://llm.example.com/v1", "apiKey": "sk-active-key-0123456789", "defaultModel": "gpt-test"},
			{"id": vertexID, "name": "Vertex", "type": "vertex", "vertexAuthCredentials": "{\"private_key\":\"vertex-secret\"}", "defaultModel": "gemini"},
		},
		"bots":           []map[string]any{{"name": "ai", "serviceID": serviceID, "model": "gpt-bot"}},
		"defaultBotName": "ai",
	}
	history := map[string]any{
		"services": []map[string]any{{"id": model.NewId(), "name": "Old", "type": "openai", "apiKey": "sk-history-key-0123456789"}},
	}
	activeJSON, _ := json.Marshal(active)
	historyJSON, _ := json.Marshal(history)
	_, err = master.Exec("INSERT INTO Agents_ConfigHistory (ID, Config, CreateAt, Active) VALUES ($1, $2, 1, false), ($3, $4, 2, true)",
		model.NewId(), string(historyJSON), model.NewId(), string(activeJSON))
	require.NoError(t, err)

	_, err = th.App.Srv().Store().System().PermanentDeleteByName(aiConnectionsFromAgentsMigrationKey)
	require.NoError(t, err)
	th.App.UpdateConfig(func(cfg *model.Config) {
		cfg.AISettings.Rewrite.ConnectionId = model.NewPointer("")
		cfg.AISettings.Recaps.ConnectionId = model.NewPointer("")
	})

	require.NoError(t, th.App.Srv().doAIConnectionsFromAgentsMigration())

	c, appErr := th.App.GetAIConnection(th.Context, serviceID)
	require.Nil(t, appErr, "the service became a connection with its ID")
	assert.Equal(t, "Lumen OpenAI", c.Name)
	assert.Equal(t, "https://llm.example.com/v1", c.BaseURL)
	assert.True(t, c.APIKeySet)
	assert.NotContains(t, c.APIKeySealed, "sk-active-key", "the key is sealed on the way")
	apiKey, _, appErr := th.App.openAIConnectionSecrets(c)
	require.Nil(t, appErr)
	assert.Equal(t, "sk-active-key-0123456789", apiKey)

	vertex, appErr := th.App.GetAIConnection(th.Context, vertexID)
	require.Nil(t, appErr)
	assert.NotEmpty(t, vertex.NeedsAttention)
	assert.False(t, vertex.APIKeySet, "the service account isn't carried over")

	all, err := th.App.Srv().Store().AIConnection().GetAll(th.Context)
	require.NoError(t, err)
	for _, conn := range all {
		assert.NotEqual(t, "Old", conn.Name, "the plugin's history is never read")
	}

	cfg := th.App.Config()
	assert.Equal(t, serviceID, *cfg.AISettings.Rewrite.ConnectionId, "the rewrite uses the default bot's service")
	assert.Equal(t, "gpt-bot", *cfg.AISettings.Rewrite.Model)
	assert.Equal(t, serviceID, *cfg.AISettings.Recaps.ConnectionId)

	// The plugin's data is left as it is.
	var count int
	require.NoError(t, master.Get(&count, "SELECT COUNT(*) FROM Agents_ConfigHistory"))
	assert.Equal(t, 2, count)

	// Once only.
	require.Nil(t, th.App.DeleteAIConnection(th.Context, c))
	require.NoError(t, th.App.Srv().doAIConnectionsFromAgentsMigration())
	_, appErr = th.App.GetAIConnection(th.Context, serviceID)
	require.NotNil(t, appErr, "a second start doesn't bring a deleted connection back")
}

func TestAIConnectionsFromAgentsMigrationWithoutThePlugin(t *testing.T) {
	th := Setup(t)
	_, err := th.GetSqlStore().GetMaster().Exec("DROP TABLE IF EXISTS Agents_ConfigHistory")
	require.NoError(t, err)
	_, err = th.App.Srv().Store().System().PermanentDeleteByName(aiConnectionsFromAgentsMigrationKey)
	require.NoError(t, err)

	require.NoError(t, th.App.Srv().doAIConnectionsFromAgentsMigration())
	done, err := th.App.Srv().Store().System().GetByName(aiConnectionsFromAgentsMigrationKey)
	require.NoError(t, err, "nothing to migrate: done")
	assert.Equal(t, "true", done.Value)
}
