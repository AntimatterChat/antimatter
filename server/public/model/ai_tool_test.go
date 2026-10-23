// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package model

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testAITool(namespace, name, kind string) *AITool {
	return &AITool{Name: AIToolFullName(namespace, name), Namespace: namespace, LocalName: name, Source: AIToolSourceApp, SourceId: namespace, Kind: kind}
}

func TestAIToolPatternMatches(t *testing.T) {
	todo := testAITool("com.example.todo", "add_item", AIToolKindWrite)
	nested := testAITool("com.example.todo.extra", "add_item", AIToolKindWrite)

	assert.True(t, AIToolPatternMatches("*", todo))
	assert.True(t, AIToolPatternMatches("com.example.todo.add_item", todo))
	assert.True(t, AIToolPatternMatches("com.example.todo.*", todo))
	assert.False(t, AIToolPatternMatches("com.example.*", todo), "a wildcard names one namespace, not the ones under it")
	assert.False(t, AIToolPatternMatches("com.example.todo.*", nested))
	assert.False(t, AIToolPatternMatches("com.example.todo.add", todo))
	assert.False(t, AIToolPatternMatches("", todo))
	assert.False(t, AIToolPatternMatches("*", nil))
}

func TestAIAgentToolPolicy(t *testing.T) {
	read := testAITool("notes", "find", AIToolKindRead)
	write := testAITool("notes", "append", AIToolKindWrite)
	external := testAITool("mcp.jira", "create_issue", AIToolKindExternalWrite)
	other := testAITool("todo", "list", AIToolKindRead)

	policy := &AIAgentToolPolicy{Enabled: true, Allowed: []string{"notes.*", "mcp.jira.create_issue"}, AutoRun: []string{"*"}}
	assert.True(t, policy.Allows(read))
	assert.True(t, policy.Allows(write))
	assert.True(t, policy.Allows(external))
	assert.False(t, policy.Allows(other), "a tool the policy doesn't name is forbidden")

	t.Run("only tools that read run without asking", func(t *testing.T) {
		assert.True(t, policy.AutoRuns(read))
		assert.False(t, policy.AutoRuns(write), "auto_run never covers a tool that changes something")
		assert.False(t, policy.AutoRuns(external))
		assert.False(t, policy.AutoRuns(other), "auto_run never covers a tool the policy forbids")
	})

	t.Run("auto_run must name the tool", func(t *testing.T) {
		p := &AIAgentToolPolicy{Enabled: true, Allowed: []string{"*"}, AutoRun: []string{"todo.*"}}
		assert.False(t, p.AutoRuns(read))
		assert.True(t, p.AutoRuns(other))
	})

	t.Run("a policy that's off allows nothing", func(t *testing.T) {
		p := &AIAgentToolPolicy{Enabled: false, Allowed: []string{"*"}, AutoRun: []string{"*"}}
		assert.False(t, p.Allows(read))
		assert.False(t, p.AutoRuns(read))
		var none *AIAgentToolPolicy
		assert.False(t, none.Allows(read))
	})
}

func TestAIToolChoices(t *testing.T) {
	tool := testAITool("notes", "find", AIToolKindRead)
	assert.True(t, (*AIToolChoices)(nil).Allows(tool))
	assert.True(t, (&AIToolChoices{}).Allows(tool))
	assert.False(t, (&AIToolChoices{OffSources: []string{"app:notes"}}).Allows(tool))
	assert.False(t, (&AIToolChoices{OffTools: []string{"notes.find"}}).Allows(tool))
	assert.True(t, (&AIToolChoices{OffTools: []string{"notes.other"}}).Allows(tool))

	t.Run("parse", func(t *testing.T) {
		assert.Equal(t, []string{"builtin"}, ParseAIToolChoices(`{"off_sources":["builtin"]}`).OffSources)
		assert.Empty(t, ParseAIToolChoices("not json").OffSources)
		assert.Empty(t, ParseAIToolChoices("").OffTools)
		assert.Empty(t, ParseAIToolChoices(`{"off_tools":["has space"]}`).OffTools, "an invalid value counts as none")
	})

	t.Run("the preference is checked", func(t *testing.T) {
		p := Preference{UserId: NewId(), Category: PreferenceCategoryAITools, Name: PreferenceNameAIToolChoices, Value: `{"off_sources":["mcp:` + NewId() + `"],"off_tools":[]}`}
		require.Nil(t, p.IsValid())
		p.Value = "nope"
		require.NotNil(t, p.IsValid())
		p.Value = `{}`
		p.Name = "other"
		require.NotNil(t, p.IsValid())
		many := make([]string, AIToolChoicesMaxItems+1)
		for i := range many {
			many[i] = "t.x"
		}
		data, _ := json.Marshal(AIToolChoices{OffTools: many})
		p.Name = PreferenceNameAIToolChoices
		p.Value = string(data)
		require.NotNil(t, p.IsValid())
	})
}

func TestAIToolDefinitionIsValid(t *testing.T) {
	valid := func() *AIToolDefinition {
		return &AIToolDefinition{
			Name:        "add_item",
			Title:       "Add an item",
			Description: "Adds an item to a list.",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}}}`),
			Kind:        AIToolKindWrite,
			Permissions: []AIToolPermission{{Permission: PermissionCreatePost.Id, ChannelArgument: "channel_id"}},
		}
	}
	require.Nil(t, valid().IsValid())
	for name, change := range map[string]func(d *AIToolDefinition){
		"dotted name":       func(d *AIToolDefinition) { d.Name = "a.b" },
		"empty name":        func(d *AIToolDefinition) { d.Name = "" },
		"bad kind":          func(d *AIToolDefinition) { d.Kind = "delete" },
		"schema not object": func(d *AIToolDefinition) { d.InputSchema = json.RawMessage(`[1]`) },
		"schema too large": func(d *AIToolDefinition) {
			d.InputSchema = json.RawMessage(`{"a":"` + strings.Repeat("x", AIToolSchemaMaxBytes) + `"}`)
		},
		"long description":   func(d *AIToolDefinition) { d.Description = strings.Repeat("x", AIToolDescriptionMaxRunes+1) },
		"channel and team":   func(d *AIToolDefinition) { d.Permissions[0].TeamArgument = "team_id" },
		"empty permission":   func(d *AIToolDefinition) { d.Permissions[0].Permission = "" },
		"too many permision": func(d *AIToolDefinition) { d.Permissions = make([]AIToolPermission, AIToolMaxPermissions+1) },
	} {
		t.Run(name, func(t *testing.T) {
			d := valid()
			change(d)
			assert.NotNil(t, d.IsValid())
		})
	}
	var none *AIToolDefinition
	assert.NotNil(t, none.IsValid())
}
