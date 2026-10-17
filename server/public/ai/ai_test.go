// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package ai

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPurposeIsBuiltIn(t *testing.T) {
	for _, p := range BuiltInPurposes {
		assert.True(t, p.IsBuiltIn(), p)
	}
	assert.False(t, Purpose("").IsBuiltIn())
	assert.False(t, Purpose("plugin:mattermost-ai").IsBuiltIn())
}

func TestStripMarkdownCodeFencing(t *testing.T) {
	assert.Equal(t, `{"a":1}`, StripMarkdownCodeFencing("```json\n{\"a\":1}\n```"))
	assert.Equal(t, `{"a":1}`, StripMarkdownCodeFencing("```json {\"a\":1}```"))
	assert.Equal(t, `{"a":1}`, StripMarkdownCodeFencing("```\n{\"a\":1}```"))
	assert.Equal(t, `plain`, StripMarkdownCodeFencing(`plain`))
}

func TestJSONInstruction(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"a":{"type":"string"}}}`)
	instruction := JSONOutputInstruction(schema)
	assert.Contains(t, instruction, "JSON object")
	assert.Contains(t, instruction, string(schema))

	t.Run("added to the first system message", func(t *testing.T) {
		messages := []Message{{Role: RoleSystem, Content: "Be brief."}, {Role: RoleUser, Content: "Hi"}}
		out := WithJSONInstruction(messages, schema)
		require.Len(t, out, 2)
		assert.True(t, strings.HasPrefix(out[0].Content, "Be brief.\n\n"))
		assert.Equal(t, "Be brief.", messages[0].Content, "the caller's messages are left alone")
	})

	t.Run("a new system message when there's none", func(t *testing.T) {
		out := WithJSONInstruction([]Message{{Role: RoleUser, Content: "Hi"}}, schema)
		require.Len(t, out, 2)
		assert.Equal(t, RoleSystem, out[0].Role)
	})
}

func TestTruncateMessages(t *testing.T) {
	count := func(s string) int { return len(s) }
	messages := []Message{
		{Role: RoleSystem, Content: "sys"},
		{Role: RoleUser, Content: "oldest message"},
		{Role: RoleAssistant, Content: "older"},
		{Role: RoleUser, Content: "latest"},
	}

	out, truncated := TruncateMessages(messages, 100, count)
	assert.False(t, truncated)
	assert.Equal(t, messages, out)

	out, truncated = TruncateMessages(messages, 3+6+5, count)
	assert.True(t, truncated)
	require.Len(t, out, 3)
	assert.Equal(t, "sys", out[0].Content, "the system message stays")
	assert.Equal(t, "older", out[1].Content)
	assert.Equal(t, "latest", out[2].Content)
}

func TestSanitizeProviderErrorMessage(t *testing.T) {
	msg := `401 {"error":{"message":"Incorrect API key provided: sk-proj-abcdefghijklmnop. You can find your API key at..."}} Authorization: Bearer secret-token-123 x-api-key: anthro-key-xyz custom=my-own-key-value`
	out := SanitizeProviderErrorMessage(msg, "my-own-key-value")
	for _, secret := range []string{"sk-proj-abcdefghijklmnop", "secret-token-123", "anthro-key-xyz", "my-own-key-value"} {
		assert.NotContains(t, out, secret)
	}
	assert.Contains(t, out, "401")

	assert.Equal(t, "a [U+202E]b", SanitizeNonPrintableChars("a \u202eb"))
}

func TestDecideRequestValidate(t *testing.T) {
	valid := DecideRequest{Input: "x", Questions: []DecisionQuestion{
		{Type: DecisionPredicate, Name: "urgent", Instructions: "Is it urgent?"},
		{Type: DecisionChoice, Name: "topic", Choices: []DecisionOption{{Value: "a"}, {Value: "b"}}},
		{Type: DecisionScore, Name: "tone", Levels: []DecisionLevel{{Label: "calm"}, {Label: "angry"}}},
	}}
	require.NoError(t, valid.Validate())

	for name, questions := range map[string][]DecisionQuestion{
		"none":            nil,
		"unnamed":         {{Type: DecisionPredicate}},
		"twice":           {{Type: DecisionPredicate, Name: "a"}, {Type: DecisionPredicate, Name: "a"}},
		"unknown type":    {{Type: "vibe", Name: "a"}},
		"one choice":      {{Type: DecisionChoice, Name: "a", Choices: []DecisionOption{{Value: "x"}}}},
		"same values":     {{Type: DecisionChoice, Name: "a", Choices: []DecisionOption{{Value: "x"}, {Value: "x"}}}},
		"unlabeled level": {{Type: DecisionScore, Name: "a", Levels: []DecisionLevel{{Label: "x"}, {}}}},
	} {
		req := DecideRequest{Input: "x", Questions: questions}
		assert.Error(t, req.Validate(), name)
	}
}

func TestStreamReadAll(t *testing.T) {
	events := make(chan StreamEvent, 5)
	events <- StreamEvent{Type: EventText, Text: "Hel"}
	events <- StreamEvent{Type: EventText, Text: "lo"}
	events <- StreamEvent{Type: EventToolCalls, ToolCalls: []ToolCall{{ID: "1", Name: "lookup", Arguments: json.RawMessage(`{}`)}}}
	events <- StreamEvent{Type: EventUsage, Usage: &Usage{InputTokens: 3, OutputTokens: 2}}
	events <- StreamEvent{Type: EventEnd, FinishReason: FinishReasonToolCalls}
	close(events)
	closed := false
	resp, err := NewStream(events, func() { closed = true }).ReadAll()
	require.NoError(t, err)
	assert.Equal(t, "Hello", resp.Message.Content)
	assert.Len(t, resp.Message.ToolCalls, 1)
	assert.Equal(t, int64(5), resp.Usage.Total())
	assert.Equal(t, FinishReasonToolCalls, resp.FinishReason)
	assert.True(t, closed)

	failing := make(chan StreamEvent, 1)
	failing <- StreamEvent{Type: EventError, Error: "rate limited"}
	close(failing)
	_, err = NewStream(failing, nil).ReadAll()
	assert.EqualError(t, err, "rate limited")

	resp, err = StreamFromResponse(&ChatResponse{Message: Message{Role: RoleAssistant, Content: "hi"}, FinishReason: FinishReasonStop}).ReadAll()
	require.NoError(t, err)
	assert.Equal(t, "hi", resp.Message.Content)
}
