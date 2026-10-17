// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package model

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validAIConnection() *AIConnection {
	c := &AIConnection{
		Name:     "Lumen OpenAI",
		Scope:    AIConnectionScopeServer,
		Provider: AIProviderOpenAICompatible,
		BaseURL:  "https://llm.example.com/v1/",
		Models:   AIConnectionModels{Chat: "gpt-test"},
		Audience: AIConnectionAudience{Everyone: true},
	}
	c.PreSave()
	return c
}

func TestAIConnectionIsValid(t *testing.T) {
	c := validAIConnection()
	require.Nil(t, c.IsValid())
	assert.Equal(t, "https://llm.example.com/v1", c.BaseURL, "the trailing slash goes")
	assert.Equal(t, AIDecisionsDialectOpenAI, c.DecisionsDialect)

	for name, change := range map[string]func(c *AIConnection){
		"no name":               func(c *AIConnection) { c.Name = "" },
		"bad scope":             func(c *AIConnection) { c.Scope = "everyone" },
		"unknown provider":      func(c *AIConnection) { c.Provider = "bedrock" },
		"not http":              func(c *AIConnection) { c.BaseURL = "file:///etc/passwd" },
		"credentials in URL":    func(c *AIConnection) { c.BaseURL = "https://user:pass@llm.example.com" },
		"query in URL":          func(c *AIConnection) { c.BaseURL = "https://llm.example.com/v1?key=secret" },
		"unknown dialect":       func(c *AIConnection) { c.DecisionsDialect = "oracle" },
		"negative limit":        func(c *AIConnection) { c.Limits.MonthlyTokenBudget = -1 },
		"bad team":              func(c *AIConnection) { c.Sharing.TeamIDs = []string{"nope"} },
		"bad role":              func(c *AIConnection) { c.Audience.Roles = []string{"Not A Role"} },
		"Anthropic embeddings":  func(c *AIConnection) { c.Provider = AIProviderAnthropic; c.Models.Embeddings = "x" },
		"newline in a model":    func(c *AIConnection) { c.Models.Chat = "a\nb" },
		"community with shares": func(c *AIConnection) { c.Scope = NewId(); c.Sharing.AllTeams = true },
	} {
		bad := validAIConnection()
		change(bad)
		assert.NotNil(t, bad.IsValid(), name)
	}
}

func TestAIConnectionNeverMarshalsItsSecrets(t *testing.T) {
	c := validAIConnection()
	c.APIKeySealed = "sealed-api-key-value"
	c.ExtraHeadersSealed = "sealed-headers-value"
	c.APIKeyLast4 = "abcd"
	c.PreUpdate()
	data, err := json.Marshal(c)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "sealed-api-key-value")
	assert.NotContains(t, string(data), "sealed-headers-value")
	assert.Contains(t, string(data), `"api_key_set":true`)
	assert.Contains(t, string(data), `"api_key_last4":"abcd"`)

	data, err = json.Marshal(c.Auditable())
	require.NoError(t, err)
	assert.NotContains(t, string(data), "sealed-")
}

func TestAIAPIKeyLast4(t *testing.T) {
	assert.Equal(t, "cdef", AIAPIKeyLast4("sk-test-abcdef"))
	assert.Equal(t, "", AIAPIKeyLast4("short"), "a short key shows nothing")
	assert.Equal(t, "", AIAPIKeyLast4(""))
}

func TestAIConnectionSharing(t *testing.T) {
	team, other := NewId(), NewId()
	server := validAIConnection()
	assert.False(t, server.SharedWithTeam(team), "shared with no community")
	server.Sharing.TeamIDs = []string{team}
	assert.True(t, server.SharedWithTeam(team))
	assert.False(t, server.SharedWithTeam(other))
	assert.False(t, server.SharedWithTeam(""), "direct messages need it shared with every community")
	server.Sharing.AllTeams = true
	assert.True(t, server.SharedWithTeam(other))
	assert.True(t, server.SharedWithTeam(""))

	community := validAIConnection()
	community.Scope = team
	assert.True(t, community.SharedWithTeam(team))
	assert.False(t, community.SharedWithTeam(other))
	assert.False(t, community.SharedWithTeam(""))
	assert.Equal(t, team, community.TeamId())
}

func TestAIExtraHeaders(t *testing.T) {
	assert.Nil(t, ValidateAIExtraHeaders(map[string]string{"api-key": "x", "X-Org": "lumen"}))
	assert.NotNil(t, ValidateAIExtraHeaders(map[string]string{"Host": "evil"}))
	assert.NotNil(t, ValidateAIExtraHeaders(map[string]string{"content-length": "1"}))
	assert.NotNil(t, ValidateAIExtraHeaders(map[string]string{"X-A": "a\r\nInjected: b"}))
	assert.NotNil(t, ValidateAIExtraHeaders(map[string]string{"Bad Name": "x"}))
	assert.Equal(t, []string{"X-A", "a"}, AIHeaderNames(map[string]string{"a": "1", "X-A": "2"}))
}

func TestAIUsageDays(t *testing.T) {
	at := time.Date(2026, 10, 17, 23, 30, 0, 0, time.FixedZone("CEST", 2*3600))
	assert.Equal(t, 20261017, AIUsageDay(at))
	assert.Equal(t, 20261001, AIUsageMonthStart(at))
}
