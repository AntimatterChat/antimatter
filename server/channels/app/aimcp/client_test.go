// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package aimcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeMCP is an MCP server for the tests: it answers initialize, tools/list (in pages of two) and tools/call, in JSON
// or as server-sent events, and checks the session and the bearer token.
type fakeMCP struct {
	t       *testing.T
	sse     bool
	token   string
	tools   []Tool
	mu      sync.Mutex
	calls   []string
	session string
	forget  bool
	gotArgs json.RawMessage
}

func (f *fakeMCP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.token != "" && r.Header.Get("Authorization") != "Bearer "+f.token {
		w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="https://mcp.example.com/.well-known/oauth-protected-resource", scope="tools:read"`)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if r.Method == http.MethodDelete {
		f.calls = append(f.calls, "DELETE")
		w.WriteHeader(http.StatusOK)
		return
	}
	assert.Contains(f.t, r.Header.Get("Accept"), "text/event-stream")
	var msg struct {
		ID     *int64          `json:"id"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	data, _ := io.ReadAll(r.Body)
	require.NoError(f.t, json.Unmarshal(data, &msg))
	f.calls = append(f.calls, msg.Method)

	if msg.Method != "initialize" {
		if f.forget {
			f.forget = false
			f.session = ""
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("Mcp-Session-Id") != f.session || f.session == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		assert.Equal(f.t, "2025-06-18", r.Header.Get("MCP-Protocol-Version"))
	}
	if msg.ID == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	var result any
	switch msg.Method {
	case "initialize":
		f.session = fmt.Sprintf("session-%d", len(f.calls))
		w.Header().Set("Mcp-Session-Id", f.session)
		result = map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]any{"name": "fake"}}
	case "tools/list":
		var params struct {
			Cursor string `json:"cursor"`
		}
		_ = json.Unmarshal(msg.Params, &params)
		start := 0
		if params.Cursor != "" {
			_, _ = fmt.Sscanf(params.Cursor, "%d", &start)
		}
		end := min(start+2, len(f.tools))
		page := map[string]any{"tools": f.tools[start:end]}
		if end < len(f.tools) {
			page["nextCursor"] = fmt.Sprint(end)
		}
		result = page
	case "tools/call":
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		_ = json.Unmarshal(msg.Params, &params)
		f.gotArgs = params.Arguments
		if params.Name == "fail" {
			f.reply(w, *msg.ID, nil, &RPCError{Code: -32602, Message: "unknown tool, token " + f.token})
			return
		}
		result = map[string]any{"content": []map[string]any{{"type": "text", "text": "ran " + params.Name}, {"type": "image", "mimeType": "image/png", "data": "AAAA"}}}
	default:
		f.reply(w, *msg.ID, nil, &RPCError{Code: -32601, Message: "no such method"})
		return
	}
	f.reply(w, *msg.ID, result, nil)
}

func (f *fakeMCP) reply(w http.ResponseWriter, id int64, result any, rpcErr *RPCError) {
	body := map[string]any{"jsonrpc": "2.0", "id": id}
	if rpcErr != nil {
		body["error"] = rpcErr
	} else {
		body["result"] = result
	}
	data, _ := json.Marshal(body)
	if f.sse {
		w.Header().Set("Content-Type", "text/event-stream")
		// A notification of the server's own first, then the answer.
		fmt.Fprintf(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/progress\",\"params\":{}}\n\n")
		fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

func readOnly(b bool) *ToolAnnotations { return &ToolAnnotations{ReadOnlyHint: &b} }

func newFakeMCP(t *testing.T, sse bool) (*fakeMCP, *httptest.Server) {
	f := &fakeMCP{t: t, sse: sse, tools: []Tool{
		{Name: "search", Description: "Search issues", Annotations: readOnly(true)},
		{Name: "create_issue", Title: "Create an issue"},
		{Name: "comment", Annotations: readOnly(false)},
	}}
	server := httptest.NewServer(f)
	t.Cleanup(server.Close)
	return f, server
}

func TestClientListAndCall(t *testing.T) {
	for _, sse := range []bool{false, true} {
		t.Run(fmt.Sprintf("sse=%v", sse), func(t *testing.T) {
			f, server := newFakeMCP(t, sse)
			c := New(server.URL, server.Client(), nil)
			tools, err := c.ListTools(context.Background())
			require.NoError(t, err)
			require.Len(t, tools, 3, "every page is read")
			assert.True(t, tools[0].ReadOnly())
			assert.False(t, tools[1].ReadOnly(), "a tool that says nothing is taken to change something")
			assert.False(t, tools[2].ReadOnly())
			assert.Equal(t, "Create an issue", tools[1].DisplayTitle())

			result, err := c.CallTool(context.Background(), "search", json.RawMessage(`{"q":"login"}`))
			require.NoError(t, err)
			assert.Equal(t, "ran search\n[image image/png not shown]", result.Text())
			assert.JSONEq(t, `{"q":"login"}`, string(f.gotArgs))

			c.Close(context.Background())
			assert.Equal(t, []string{"initialize", "notifications/initialized", "tools/list", "tools/list", "tools/call", "DELETE"}, f.calls)
		})
	}
}

func TestClientStartsAForgottenSessionAgain(t *testing.T) {
	f, server := newFakeMCP(t, false)
	c := New(server.URL, server.Client(), nil)
	_, err := c.ListTools(context.Background())
	require.NoError(t, err)
	f.forget = true
	_, err = c.CallTool(context.Background(), "search", nil)
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(f.gotArgs), "no arguments is an empty object")
}

func TestClientNeedsSignIn(t *testing.T) {
	f, server := newFakeMCP(t, false)
	f.token = "the-right-token-123"
	c := New(server.URL, server.Client(), http.Header{"Authorization": {"Bearer the-wrong-token-456"}})
	_, err := c.ListTools(context.Background())
	var unauthorized *UnauthorizedError
	require.ErrorAs(t, err, &unauthorized)
	assert.Equal(t, "https://mcp.example.com/.well-known/oauth-protected-resource", unauthorized.ResourceMetadataURL)
	assert.Equal(t, "tools:read", unauthorized.Scope)
}

func TestClientErrorsNeverCarryTheToken(t *testing.T) {
	f, server := newFakeMCP(t, false)
	f.token = "secret-token-abcdef"
	c := New(server.URL, server.Client(), http.Header{"Authorization": {"Bearer secret-token-abcdef"}})
	_, err := c.CallTool(context.Background(), "fail", nil)
	require.Error(t, err)
	var rpcErr *RPCError
	require.ErrorAs(t, err, &rpcErr)
	assert.NotContains(t, err.Error(), "secret-token-abcdef", "the server echoing the token back doesn't leak it")
}

func TestClientRefusesTooLargeAnswers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Mcp-Session-Id", "s")
		fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":{"pad":"`+strings.Repeat("x", maxResponseBytes)+`"}}`)
	}))
	defer server.Close()
	err := New(server.URL, server.Client(), nil).Initialize(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too large")
}

func TestClientRefusesAnAnswerToAnotherRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"jsonrpc":"2.0","id":99,"result":{}}`)
	}))
	defer server.Close()
	require.Error(t, New(server.URL, server.Client(), nil).Initialize(context.Background()))
}

func TestParseAuthParams(t *testing.T) {
	params := parseAuthParams(`Bearer realm="mcp", resource_metadata="https://a.example/x?y=\"z\"", scope=read`)
	assert.Equal(t, "mcp", params["realm"])
	assert.Equal(t, `https://a.example/x?y="z"`, params["resource_metadata"])
	assert.Equal(t, "read", params["scope"])
	assert.Empty(t, parseAuthParams("Bearer"))
	assert.Empty(t, parseAuthParams(""))
}

func TestCallResultText(t *testing.T) {
	r := &CallResult{Content: []Content{
		{Type: "text", Text: "one"},
		{Type: "resource", Resource: &ResourceContent{URI: "file:///a", Text: "two"}},
		{Type: "resource", Resource: &ResourceContent{URI: "file:///b"}},
		{Type: "resource_link", URI: "https://x.example"},
	}}
	assert.Equal(t, "one\ntwo\n[resource file:///b]\n[link https://x.example]", r.Text())
	assert.Equal(t, `{"a":1}`, (&CallResult{StructuredContent: json.RawMessage(`{"a":1}`)}).Text())
}
