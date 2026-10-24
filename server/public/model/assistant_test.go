// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The assistant's preferences: a channel's switch (on or off, named by the channel) and the muted tools.
func TestAssistantPreferences(t *testing.T) {
	userID, channelID := NewId(), NewId()
	valid := []Preference{
		{UserId: userID, Category: PreferenceCategoryAssistant, Name: channelID, Value: AssistantChannelOff},
		{UserId: userID, Category: PreferenceCategoryAssistant, Name: channelID, Value: AssistantChannelOn},
		{UserId: userID, Category: PreferenceCategoryAssistant, Name: PreferenceNameAssistantMutedTools, Value: `{"tools":["mcp.grafana.query_prometheus"]}`},
	}
	for _, p := range valid {
		assert.Nil(t, p.IsValid(), "%s=%s", p.Name, p.Value)
	}
	invalid := []Preference{
		{UserId: userID, Category: PreferenceCategoryAssistant, Name: channelID, Value: "maybe"},
		{UserId: userID, Category: PreferenceCategoryAssistant, Name: "not-an-id", Value: AssistantChannelOff},
		{UserId: userID, Category: PreferenceCategoryAssistant, Name: PreferenceNameAssistantMutedTools, Value: `{"tools":["has space"]}`},
		{UserId: userID, Category: PreferenceCategoryAssistant, Name: PreferenceNameAssistantMutedTools, Value: `not json`},
	}
	for _, p := range invalid {
		assert.NotNil(t, p.IsValid(), "%s=%s", p.Name, p.Value)
	}

	muted := ParseAssistantMutedTools(`{"tools":["a.b"]}`)
	assert.True(t, muted.Mutes("a.b"))
	assert.False(t, muted.Mutes("a.c"))
	assert.Empty(t, ParseAssistantMutedTools("broken").Tools, "a broken value mutes nothing")
}

func TestAssistantCommunitySettingsIsValid(t *testing.T) {
	assert.Nil(t, (&AssistantCommunitySettings{}).IsValid())
	assert.Nil(t, (&AssistantCommunitySettings{Enabled: NewPointer(false), Engine: AssistantEngineModel, ConnectionId: NewId()}).IsValid())
	assert.NotNil(t, (&AssistantCommunitySettings{Engine: "chat"}).IsValid())
	assert.NotNil(t, (&AssistantCommunitySettings{ConnectionId: "x"}).IsValid())
}

// A pattern's slots become groups that capture the entity's index; plain regular-expression braces stay.
func TestCompileAssistantPatternExpression(t *testing.T) {
	re, slots, err := CompileAssistantPatternExpression(`\bcpu.{0,40}on {host|service} and {date}`)
	assert.NoError(t, err)
	assert.Equal(t, []string{"slot__0", "slot__1"}, slots)
	assert.True(t, re.MatchString("CPU usage on ⟦service:3⟧ and ⟦date:4⟧"))
	assert.False(t, re.MatchString("cpu on ⟦person:3⟧ and ⟦date:4⟧"), "a slot takes only its types")
}
