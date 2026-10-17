// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validTestAIAgent() *AIAgent {
	a := &AIAgent{UserId: NewId(), Scope: AIAgentScopeServer, ConnectionId: NewId(), Model: "m", Access: AIAgentAccess{Everyone: true}}
	a.PreSave()
	return a
}

func TestAIAgentIsValid(t *testing.T) {
	require.Nil(t, validTestAIAgent().IsValid())

	for name, change := range map[string]func(a *AIAgent){
		"bad user id":          func(a *AIAgent) { a.UserId = "x" },
		"bad scope":            func(a *AIAgent) { a.Scope = "everywhere" },
		"no connection":        func(a *AIAgent) { a.ConnectionId = "" },
		"model with a newline": func(a *AIAgent) { a.Model = "a\nb" },
		"prompt too long":      func(a *AIAgent) { a.SystemPrompt = strings.Repeat("a", AIAgentSystemPromptMaxRunes+1) },
		"bad tool name":        func(a *AIAgent) { a.Tools.Allowed = []string{"rm -rf"} },
		"bad auto-run name":    func(a *AIAgent) { a.Tools.AutoRun = []string{"../x"} },
		"bad effort":           func(a *AIAgent) { a.Reasoning.Effort = "extreme" },
		"negative budget":      func(a *AIAgent) { a.Reasoning.BudgetTokens = -1 },
		"too many tool turns":  func(a *AIAgent) { a.MaxToolTurns = AIAgentMaxToolTurns + 1 },
		"bad channel id":       func(a *AIAgent) { a.Access.ChannelIDs = []string{"general"} },
		"bad role":             func(a *AIAgent) { a.Access.Roles = []string{"Not A Role!"} },
		"no times":             func(a *AIAgent) { a.CreateAt = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			a := validTestAIAgent()
			change(a)
			assert.NotNil(t, a.IsValid())
		})
	}

	t.Run("a community's agent stays in its community", func(t *testing.T) {
		a := validTestAIAgent()
		a.Scope = NewId()
		a.Access.TeamIDs = []string{a.Scope}
		assert.Nil(t, a.IsValid())
		a.Access.TeamIDs = []string{a.Scope, NewId()}
		assert.NotNil(t, a.IsValid())
	})

	t.Run("tool names", func(t *testing.T) {
		for _, ok := range []string{"*", "todo.*", "todo.add_item", "github.search-issues", "mcp:server/tool"} {
			assert.True(t, IsValidAIAgentToolName(ok), ok)
		}
		for _, bad := range []string{"", ".hidden", "a..b", "with space", strings.Repeat("a", AIAgentToolNameMaxLength+1)} {
			assert.False(t, IsValidAIAgentToolName(bad), bad)
		}
	})
}

// An external agent has no runtime: its model settings are dropped, and it needs no connection.
func TestAIAgentExternalKeepsNoModelSettings(t *testing.T) {
	a := &AIAgent{
		UserId: NewId(), Scope: AIAgentScopeServer, External: true, ConnectionId: NewId(), Model: "m", SystemPrompt: "p",
		Tools:     AIAgentToolPolicy{Enabled: true, Allowed: []string{"*"}, AutoRun: []string{"*"}},
		Reasoning: AIAgentReasoning{Enabled: true}, MaxToolTurns: 3,
	}
	a.PreSave()
	require.Nil(t, a.IsValid())
	assert.Empty(t, a.ConnectionId)
	assert.Empty(t, a.Model)
	assert.Empty(t, a.SystemPrompt)
	assert.False(t, a.Tools.Enabled)
	assert.Empty(t, a.Tools.Allowed)
	assert.Empty(t, a.Tools.AutoRun)
	assert.Zero(t, a.MaxToolTurns)
}

func TestAIAgentNormalize(t *testing.T) {
	a := &AIAgent{UserId: NewId(), Scope: AIAgentScopeServer, ConnectionId: NewId(), Model: " m ",
		Reasoning: AIAgentReasoning{Effort: " High "},
		Tools:     AIAgentToolPolicy{Allowed: []string{"b.x", " a.y ", "b.x", ""}},
	}
	a.PreSave()
	assert.Equal(t, "m", a.Model)
	assert.Equal(t, AIAgentReasoningEffortHigh, a.Reasoning.Effort)
	assert.Equal(t, []string{"a.y", "b.x"}, a.Tools.Allowed)
	assert.Equal(t, []string{}, a.Access.ChannelIDs)
	assert.Equal(t, AIAgentDefaultMaxToolTurns, a.EffectiveMaxToolTurns())
	a.MaxToolTurns = 7
	assert.Equal(t, 7, a.EffectiveMaxToolTurns())
}

// The audit record never keeps the system prompt's text.
func TestAIAgentAuditableLeavesOutThePrompt(t *testing.T) {
	a := validTestAIAgent()
	a.SystemPrompt = "the secret sauce"
	for _, v := range a.Auditable() {
		if s, ok := v.(string); ok {
			assert.NotContains(t, s, "secret sauce")
		}
	}
	assert.Equal(t, len("the secret sauce"), a.Auditable()["system_prompt_size"])
}

func TestAgentActivityRequest(t *testing.T) {
	user := NewId()
	now := int64(1_000_000)

	activity, appErr := (&AgentActivityRequest{State: AgentStateWorking, Text: " Running the poll tests "}).Validate(user, now)
	require.Nil(t, appErr)
	assert.Equal(t, "Running the poll tests", activity.Text)
	assert.Equal(t, now+AgentActivityDefaultSeconds*1000, activity.ExpiresAt, "a busy state expires by default")

	activity, appErr = (&AgentActivityRequest{State: AgentStateWaiting, ExpiresInSeconds: 30}).Validate(user, now)
	require.Nil(t, appErr)
	assert.Equal(t, now+30_000, activity.ExpiresAt)

	thread := NewId()
	activity, appErr = (&AgentActivityRequest{State: AgentStateIdle, ThreadPostId: thread, ExpiresInSeconds: 30}).Validate(user, now)
	require.Nil(t, appErr)
	assert.Zero(t, activity.ExpiresAt, "idle never expires")
	assert.Empty(t, activity.ThreadPostId, "an idle agent works in no thread")

	for name, req := range map[string]AgentActivityRequest{
		"unknown state":  {State: "napping"},
		"text too long":  {State: AgentStateWorking, Text: strings.Repeat("a", AgentActivityTextMaxRunes+1)},
		"text with line": {State: AgentStateWorking, Text: "a\nb"},
		"bad thread":     {State: AgentStateWorking, ThreadPostId: "thread"},
		"negative":       {State: AgentStateWorking, ExpiresInSeconds: -1},
		"too long":       {State: AgentStateWorking, ExpiresInSeconds: AgentActivityMaxSeconds + 1},
	} {
		t.Run(name, func(t *testing.T) {
			_, appErr := req.Validate(user, now)
			assert.NotNil(t, appErr)
		})
	}
}

func TestAgentActivityEffective(t *testing.T) {
	a := &AgentActivity{UserId: NewId(), State: AgentStateWorking, Text: "x", ThreadPostId: NewId(), UpdateAt: 1, ExpiresAt: 100}
	assert.Equal(t, AgentStateWorking, a.Effective(99).State)
	expired := a.Effective(100)
	assert.Equal(t, AgentStateIdle, expired.State, "an expired busy state is idle")
	assert.Empty(t, expired.Text)
	assert.Empty(t, expired.ThreadPostId)
	assert.Empty(t, a.WithoutThread().ThreadPostId)
	assert.NotEmpty(t, a.ThreadPostId, "WithoutThread copies")
}

func TestBotKind(t *testing.T) {
	b := &Bot{Username: "agentsmith", OwnerId: NewId()}
	b.PreSave()
	assert.Equal(t, BotKindBot, b.Kind, "a bot is a plain bot unless made an agent")
	assert.False(t, b.IsAgent())
	b.Kind = BotKindAgent
	assert.True(t, b.IsAgent())
	assert.Nil(t, b.IsValidCreate())
	b.Kind = "robot"
	assert.NotNil(t, b.IsValidCreate())
}
