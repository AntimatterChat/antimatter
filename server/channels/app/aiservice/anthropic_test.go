// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package aiservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/ai"
)

const anthropicTestKey = "sk-ant-test-0123456789abcdefghijklmnop"

func newAnthropic(t *testing.T, url string) ai.Provider {
	t.Helper()
	p, err := New(Config{Provider: ProviderAnthropic, BaseURL: url + "/v1", APIKey: anthropicTestKey})
	require.NoError(t, err)
	return p
}

func writeAnthropicSSE(w http.ResponseWriter, events ...[2]string) {
	w.Header().Set("Content-Type", "text/event-stream")
	flusher, _ := w.(http.Flusher)
	for _, e := range events {
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e[0], e[1])
		if flusher != nil {
			flusher.Flush()
		}
	}
}

func TestAnthropicChat(t *testing.T) {
	f := newFakeEndpoint(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
		fmt.Fprint(w, `{"model":"claude-test","content":[{"type":"text","text":"Hello"},{"type":"tool_use","id":"tu_1","name":"lookup","input":{"q":"gdp"}}],"stop_reason":"tool_use","usage":{"input_tokens":20,"output_tokens":9}}`)
	})
	resp, err := newAnthropic(t, f.URL).Chat(context.Background(), ai.ChatRequest{
		Model: "claude-test",
		Messages: []ai.Message{
			{Role: ai.RoleSystem, Content: "Be brief."},
			{Role: ai.RoleUser, Content: "GDP?"},
			{Role: ai.RoleAssistant, ToolCalls: []ai.ToolCall{{ID: "tu_0", Name: "lookup", Arguments: json.RawMessage(`{"q":"x"}`)}}},
			{Role: ai.RoleTool, ToolCallID: "tu_0", Content: "none"},
			{Role: ai.RoleUser, Content: "Again?"},
		},
		Tools:      []ai.Tool{{Name: "lookup", Description: "Look it up", Parameters: json.RawMessage(`{"type":"object","properties":{"q":{"type":"string"}}}`)}},
		ToolChoice: ai.ToolChoiceRequired,
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello", resp.Message.Content)
	require.Len(t, resp.Message.ToolCalls, 1)
	assert.JSONEq(t, `{"q":"gdp"}`, string(resp.Message.ToolCalls[0].Arguments))
	assert.Equal(t, ai.FinishReasonToolCalls, resp.FinishReason)
	assert.Equal(t, ai.Usage{InputTokens: 20, OutputTokens: 9}, resp.Usage)

	req := f.last()
	assert.Equal(t, "/v1/messages", req.Path)
	assert.Equal(t, anthropicTestKey, req.Header.Get("x-api-key"))
	assert.Equal(t, anthropicVersion, req.Header.Get("anthropic-version"))
	assert.Empty(t, req.Header.Get("Authorization"))
	assert.Equal(t, "Be brief.", req.Body["system"], "system messages go to the system field")
	assert.Equal(t, float64(anthropicDefaultMaxTokens), req.Body["max_tokens"], "max_tokens is required")
	assert.Equal(t, map[string]any{"type": "any"}, req.Body["tool_choice"])
	messages := req.Body["messages"].([]any)
	require.Len(t, messages, 3, "the tool result and the next question share the user's turn")
	userTurn := messages[2].(map[string]any)
	assert.Equal(t, "user", userTurn["role"])
	blocks := userTurn["content"].([]any)
	assert.Equal(t, "tool_result", blocks[0].(map[string]any)["type"])
	assert.Equal(t, "tu_0", blocks[0].(map[string]any)["tool_use_id"])
	assert.Equal(t, "tool_use", messages[1].(map[string]any)["content"].([]any)[0].(map[string]any)["type"])
	tools := req.Body["tools"].([]any)
	assert.NotNil(t, tools[0].(map[string]any)["input_schema"])
}

func TestAnthropicStructuredOutput(t *testing.T) {
	t.Run("an object schema is a forced tool", func(t *testing.T) {
		f := newFakeEndpoint(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
			fmt.Fprint(w, `{"content":[{"type":"tool_use","id":"tu","name":"structured_output","input":{"highlights":["a"],"action_items":[]}}],"stop_reason":"tool_use","usage":{"input_tokens":1,"output_tokens":1}}`)
		})
		schema := json.RawMessage(`{"type":"object","properties":{"highlights":{"type":"array"},"action_items":{"type":"array"}}}`)
		resp, err := newAnthropic(t, f.URL).Chat(context.Background(), ai.ChatRequest{Model: "c", Messages: []ai.Message{{Role: ai.RoleUser, Content: "sum up"}}, JSONSchema: schema})
		require.NoError(t, err)
		assert.JSONEq(t, `{"highlights":["a"],"action_items":[]}`, resp.Message.Content)
		assert.Empty(t, resp.Message.ToolCalls)
		assert.Equal(t, ai.FinishReasonStop, resp.FinishReason)
		body := f.last().Body
		assert.Equal(t, map[string]any{"type": "tool", "name": anthropicOutputTool}, body["tool_choice"])
	})

	t.Run("another schema is wrapped in an object", func(t *testing.T) {
		f := newFakeEndpoint(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
			fmt.Fprint(w, `{"content":[{"type":"tool_use","id":"tu","name":"structured_output","input":{"value":["x","y"]}}],"stop_reason":"tool_use"}`)
		})
		resp, err := newAnthropic(t, f.URL).Chat(context.Background(), ai.ChatRequest{Model: "c", Messages: []ai.Message{{Role: ai.RoleUser, Content: "list"}}, JSONSchema: json.RawMessage(`{"type":"array","items":{"type":"string"}}`)})
		require.NoError(t, err)
		assert.JSONEq(t, `["x","y"]`, resp.Message.Content)
	})

	t.Run("with the caller's tools, the schema goes to the instructions", func(t *testing.T) {
		f := newFakeEndpoint(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
			fmt.Fprint(w, "{\"content\":[{\"type\":\"text\",\"text\":\"```json\\n{\\\"a\\\":1}\\n```\"}],\"stop_reason\":\"end_turn\"}")
		})
		resp, err := newAnthropic(t, f.URL).Chat(context.Background(), ai.ChatRequest{
			Model: "c", Messages: []ai.Message{{Role: ai.RoleUser, Content: "go"}},
			Tools:      []ai.Tool{{Name: "lookup"}},
			JSONSchema: json.RawMessage(`{"type":"object"}`),
		})
		require.NoError(t, err)
		assert.JSONEq(t, `{"a":1}`, resp.Message.Content)
		assert.Contains(t, f.last().Body["system"], "JSON")
	})
}

func TestAnthropicChatStream(t *testing.T) {
	f := newFakeEndpoint(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
		writeAnthropicSSE(w,
			[2]string{"message_start", `{"type":"message_start","message":{"model":"claude-test","usage":{"input_tokens":25,"output_tokens":1}}}`},
			[2]string{"content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`},
			[2]string{"ping", `{"type":"ping"}`},
			[2]string{"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Let me "}}`},
			[2]string{"content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"check."}}`},
			[2]string{"content_block_stop", `{"type":"content_block_stop","index":0}`},
			[2]string{"content_block_start", `{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"tu_1","name":"lookup","input":{}}}`},
			[2]string{"content_block_delta", `{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"q\": \"g"}}`},
			[2]string{"content_block_delta", `{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"dp\"}"}}`},
			[2]string{"content_block_stop", `{"type":"content_block_stop","index":1}`},
			[2]string{"message_delta", `{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":15}}`},
			[2]string{"message_stop", `{"type":"message_stop"}`},
		)
	})
	stream, err := newAnthropic(t, f.URL).ChatStream(context.Background(), ai.ChatRequest{Model: "claude-test", Messages: []ai.Message{{Role: ai.RoleUser, Content: "GDP?"}}, Tools: []ai.Tool{{Name: "lookup"}}})
	require.NoError(t, err)
	resp, err := stream.ReadAll()
	require.NoError(t, err)
	assert.Equal(t, "Let me check.", resp.Message.Content)
	require.Len(t, resp.Message.ToolCalls, 1)
	assert.Equal(t, "tu_1", resp.Message.ToolCalls[0].ID)
	assert.JSONEq(t, `{"q":"gdp"}`, string(resp.Message.ToolCalls[0].Arguments))
	assert.Equal(t, ai.FinishReasonToolCalls, resp.FinishReason)
	assert.Equal(t, ai.Usage{InputTokens: 25, OutputTokens: 15}, resp.Usage)
	assert.Equal(t, true, f.last().Body["stream"])
}

func TestAnthropicStreamError(t *testing.T) {
	f := newFakeEndpoint(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
		writeAnthropicSSE(w, [2]string{"error", `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`})
	})
	stream, err := newAnthropic(t, f.URL).ChatStream(context.Background(), ai.ChatRequest{Model: "c"})
	require.NoError(t, err)
	_, err = stream.ReadAll()
	assert.EqualError(t, err, "Overloaded")
}

func TestAnthropicUnsupported(t *testing.T) {
	p := newAnthropic(t, "http://127.0.0.1:1")
	_, err := p.Embed(context.Background(), ai.EmbedRequest{})
	assert.True(t, errors.Is(err, ai.ErrUnsupported))
	_, err = p.Transcribe(context.Background(), ai.TranscribeRequest{})
	assert.True(t, errors.Is(err, ai.ErrUnsupported))
	_, err = p.Decide(context.Background(), ai.DecideRequest{})
	assert.True(t, errors.Is(err, ai.ErrUnsupported))
}

func TestAnthropicRefusedKeyIsRedacted(t *testing.T) {
	f := newFakeEndpoint(t, func(w http.ResponseWriter, r *http.Request, body map[string]any) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key `+anthropicTestKey+`"}}`)
	})
	_, err := newAnthropic(t, f.URL).Chat(context.Background(), ai.ChatRequest{Model: "c"})
	var pe *ai.ProviderError
	require.ErrorAs(t, err, &pe)
	assert.Equal(t, http.StatusUnauthorized, pe.StatusCode)
	assert.NotContains(t, pe.Error(), anthropicTestKey)
}
