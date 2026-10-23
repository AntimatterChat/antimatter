// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

// Package aimcp is the server's MCP client (Antimatter): it lists and calls the tools of an MCP server over the Model
// Context Protocol's Streamable HTTP transport (JSON-RPC 2.0 over POST, answered in JSON or as server-sent events),
// and signs people in to it with OAuth 2.1 (authorization code with PKCE, the server's metadata found as the MCP
// authorization spec says, dynamic client registration when no client was set up).
//
// It knows nothing of who may use what: the app (channels/app/ai_mcp*.go) picks the HTTP client (the same network
// rules as the model connections'), the person whose sign-in is used, and what is shown. Nothing here logs; errors
// never carry the secrets they were given.
package aimcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/mattermost/mattermost/server/public/ai"
)

const (
	// ProtocolVersion is the version of MCP the client asks for; it accepts the server's own answer.
	ProtocolVersion = "2025-06-18"

	maxResponseBytes  = 8 * 1024 * 1024
	maxSSELineBytes   = 4 * 1024 * 1024
	maxErrorBodyBytes = 4 * 1024
	// MaxTools bounds the tools read from one server, MaxToolPages the pages of its list.
	MaxTools     = 256
	MaxToolPages = 10

	headerSessionID       = "Mcp-Session-Id"
	headerProtocolVersion = "MCP-Protocol-Version"
)

// ClientName is how the server introduces itself to MCP servers.
var ClientName = "Antimatter"

// Tool is a tool as an MCP server lists it.
type Tool struct {
	Name        string           `json:"name"`
	Title       string           `json:"title,omitempty"`
	Description string           `json:"description,omitempty"`
	InputSchema json.RawMessage  `json:"inputSchema,omitempty"`
	Annotations *ToolAnnotations `json:"annotations,omitempty"`
}

// ToolAnnotations are what the server says of a tool. They are hints from the server: a tool is only taken to read
// when it says so (readOnlyHint true); anything else is taken to change something.
type ToolAnnotations struct {
	Title           string `json:"title,omitempty"`
	ReadOnlyHint    *bool  `json:"readOnlyHint,omitempty"`
	DestructiveHint *bool  `json:"destructiveHint,omitempty"`
	IdempotentHint  *bool  `json:"idempotentHint,omitempty"`
	OpenWorldHint   *bool  `json:"openWorldHint,omitempty"`
}

// ReadOnly tells whether the server says the tool only reads.
func (t *Tool) ReadOnly() bool {
	return t.Annotations != nil && t.Annotations.ReadOnlyHint != nil && *t.Annotations.ReadOnlyHint
}

// DisplayTitle is the tool's title, else its annotations', else its name.
func (t *Tool) DisplayTitle() string {
	switch {
	case t.Title != "":
		return t.Title
	case t.Annotations != nil && t.Annotations.Title != "":
		return t.Annotations.Title
	default:
		return t.Name
	}
}

// Content is a part of a tool's result.
type Content struct {
	Type     string           `json:"type"`
	Text     string           `json:"text,omitempty"`
	MimeType string           `json:"mimeType,omitempty"`
	URI      string           `json:"uri,omitempty"`
	Name     string           `json:"name,omitempty"`
	Resource *ResourceContent `json:"resource,omitempty"`
}

// ResourceContent is an embedded resource of a result.
type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
}

// CallResult is a tool's answer.
type CallResult struct {
	Content           []Content       `json:"content"`
	StructuredContent json.RawMessage `json:"structuredContent,omitempty"`
	IsError           bool            `json:"isError,omitempty"`
}

// Text gives the result as text for a model: its text parts, embedded text resources, and a line for what isn't
// text (images, audio), whose data isn't passed on.
func (r *CallResult) Text() string {
	var b strings.Builder
	add := func(s string) {
		if s == "" {
			return
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(s)
	}
	for _, c := range r.Content {
		switch c.Type {
		case "text":
			add(c.Text)
		case "resource":
			if c.Resource != nil {
				if c.Resource.Text != "" {
					add(c.Resource.Text)
				} else {
					add("[resource " + c.Resource.URI + "]")
				}
			}
		case "resource_link":
			add("[link " + c.URI + "]")
		case "image", "audio":
			add("[" + c.Type + " " + c.MimeType + " not shown]")
		}
	}
	if b.Len() == 0 && len(r.StructuredContent) > 0 {
		add(string(r.StructuredContent))
	}
	return b.String()
}

// UnauthorizedError is a server's answer that it needs a sign-in (401), with where its metadata is when it says.
type UnauthorizedError struct {
	ResourceMetadataURL string
	Scope               string
}

func (e *UnauthorizedError) Error() string {
	return "the MCP server needs a sign-in"
}

// RPCError is an error the server answered a request with.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("the MCP server answered with an error %d: %s", e.Code, e.Message)
}

// Client talks to one MCP server for one sign-in. It isn't safe for concurrent use: make one per use.
type Client struct {
	url        string
	httpClient *http.Client
	headers    http.Header
	secrets    []string

	sessionID   string
	protocol    string
	initialized bool
	nextID      atomic.Int64
}

// New makes a client of the server at url, sending headers (a person's bearer token, or a service account's headers)
// with every request. Their values are kept out of every error.
func New(url string, httpClient *http.Client, headers http.Header) *Client {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	c := &Client{url: url, httpClient: httpClient, headers: headers.Clone()}
	for _, values := range headers {
		for _, v := range values {
			c.secrets = append(c.secrets, v)
			if token, ok := strings.CutPrefix(v, "Bearer "); ok {
				c.secrets = append(c.secrets, token)
			}
		}
	}
	return c
}

func (c *Client) sanitize(message string) string {
	return ai.SanitizeProviderErrorMessage(message, c.secrets...)
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      *int64 `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// Initialize starts a session with the server.
func (c *Client) Initialize(ctx context.Context) error {
	var result struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	params := map[string]any{
		"protocolVersion": ProtocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": ClientName, "version": "1"},
	}
	if err := c.call(ctx, "initialize", params, &result, false); err != nil {
		return err
	}
	c.protocol = result.ProtocolVersion
	if c.protocol == "" {
		c.protocol = ProtocolVersion
	}
	c.initialized = true
	return c.notify(ctx, "notifications/initialized")
}

func (c *Client) ensureInitialized(ctx context.Context) error {
	if c.initialized {
		return nil
	}
	return c.Initialize(ctx)
}

// ListTools gives the server's tools (at most MaxTools).
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	if err := c.ensureInitialized(ctx); err != nil {
		return nil, err
	}
	var tools []Tool
	cursor := ""
	for page := 0; page < MaxToolPages; page++ {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		var result struct {
			Tools      []Tool `json:"tools"`
			NextCursor string `json:"nextCursor"`
		}
		if err := c.call(ctx, "tools/list", params, &result, true); err != nil {
			return nil, err
		}
		tools = append(tools, result.Tools...)
		if len(tools) >= MaxTools {
			return tools[:MaxTools], nil
		}
		if result.NextCursor == "" {
			break
		}
		cursor = result.NextCursor
	}
	return tools, nil
}

// CallTool runs a tool with its arguments (a JSON object).
func (c *Client) CallTool(ctx context.Context, name string, arguments json.RawMessage) (*CallResult, error) {
	if err := c.ensureInitialized(ctx); err != nil {
		return nil, err
	}
	if len(arguments) == 0 {
		arguments = json.RawMessage(`{}`)
	}
	var result CallResult
	if err := c.call(ctx, "tools/call", map[string]any{"name": name, "arguments": arguments}, &result, true); err != nil {
		return nil, err
	}
	return &result, nil
}

// Close ends the session (best effort).
func (c *Client) Close(ctx context.Context) {
	if c.sessionID == "" {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.url, nil)
	if err != nil {
		return
	}
	c.setHeaders(req)
	if resp, err := c.httpClient.Do(req); err == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxErrorBodyBytes))
		resp.Body.Close()
	}
	c.sessionID = ""
}

func (c *Client) setHeaders(req *http.Request) {
	for name, values := range c.headers {
		for _, v := range values {
			req.Header.Add(name, v)
		}
	}
	if c.sessionID != "" {
		req.Header.Set(headerSessionID, c.sessionID)
	}
	if c.protocol != "" {
		req.Header.Set(headerProtocolVersion, c.protocol)
	}
}

func (c *Client) post(ctx context.Context, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New(c.sanitize(err.Error()))
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	c.setHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, errors.New(c.sanitize(err.Error()))
	}
	return resp, nil
}

// statusError reads a failed response into an error.
func (c *Client) statusError(resp *http.Response) error {
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return parseUnauthorized(resp.Header.Get("WWW-Authenticate"))
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
	message := strings.TrimSpace(string(data))
	if message == "" {
		message = http.StatusText(resp.StatusCode)
	}
	return errors.New(c.sanitize(fmt.Sprintf("the MCP server answered %d: %s", resp.StatusCode, message)))
}

func (c *Client) notify(ctx context.Context, method string) error {
	body, err := json.Marshal(rpcRequest{JSONRPC: "2.0", Method: method})
	if err != nil {
		return err
	}
	resp, err := c.post(ctx, body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return c.statusError(resp)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxErrorBodyBytes))
	resp.Body.Close()
	return nil
}

// call sends a request and reads its result into out. A session the server forgot (404) is started again once when
// retry is set.
func (c *Client) call(ctx context.Context, method string, params, out any, retry bool) error {
	id := c.nextID.Add(1)
	body, err := json.Marshal(rpcRequest{JSONRPC: "2.0", ID: &id, Method: method, Params: params})
	if err != nil {
		return err
	}
	resp, err := c.post(ctx, body)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNotFound && c.sessionID != "" && retry {
		resp.Body.Close()
		c.sessionID, c.protocol, c.initialized = "", "", false
		if err := c.Initialize(ctx); err != nil {
			return err
		}
		return c.call(ctx, method, params, out, false)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.statusError(resp)
	}
	if session := resp.Header.Get(headerSessionID); session != "" && method == "initialize" {
		c.sessionID = session
	}
	msg, err := c.readResponse(resp, id)
	if err != nil {
		return err
	}
	if msg.Error != nil {
		msg.Error.Message = c.sanitize(msg.Error.Message)
		return msg.Error
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(msg.Result, out); err != nil {
		return fmt.Errorf("the MCP server's answer to %s isn't valid: %w", method, err)
	}
	return nil
}

// readResponse reads the answer to request id: a JSON body, or the event of a server-sent event stream that carries
// it (the server's own requests and notifications on the stream are skipped).
func (c *Client) readResponse(resp *http.Response, id int64) (*rpcMessage, error) {
	defer resp.Body.Close()
	mediaType, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	want := fmt.Sprint(id)
	if mediaType == "text/event-stream" {
		var found *rpcMessage
		err := readSSE(io.LimitReader(resp.Body, maxResponseBytes), func(data string) bool {
			var msg rpcMessage
			if json.Unmarshal([]byte(data), &msg) != nil {
				return true
			}
			if string(msg.ID) == want && msg.Method == "" {
				found = &msg
				return false
			}
			return true
		})
		if err != nil {
			return nil, errors.New(c.sanitize("the MCP server's stream failed: " + err.Error()))
		}
		if found == nil {
			return nil, errors.New("the MCP server's stream ended without an answer")
		}
		return found, nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, errors.New(c.sanitize(err.Error()))
	}
	if len(data) > maxResponseBytes {
		return nil, errors.New("the MCP server's answer is too large")
	}
	var msg rpcMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, errors.New("the MCP server's answer isn't JSON-RPC")
	}
	if string(msg.ID) != want {
		return nil, errors.New("the MCP server answered another request")
	}
	return &msg, nil
}

// readSSE calls onData with the data of each event until it returns false or the stream ends.
func readSSE(r io.Reader, onData func(string) bool) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), maxSSELineBytes)
	var data []string
	dispatch := func() bool {
		if len(data) == 0 {
			return true
		}
		ok := onData(strings.Join(data, "\n"))
		data = nil
		return ok
	}
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			if !dispatch() {
				return nil
			}
		case strings.HasPrefix(line, "data:"):
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	dispatch()
	return nil
}

// parseUnauthorized reads a 401's WWW-Authenticate header: Bearer resource_metadata="..." scope="...".
func parseUnauthorized(header string) *UnauthorizedError {
	e := &UnauthorizedError{}
	params := parseAuthParams(header)
	e.ResourceMetadataURL = params["resource_metadata"]
	e.Scope = params["scope"]
	return e
}

// parseAuthParams reads the parameters of a Bearer challenge.
func parseAuthParams(header string) map[string]string {
	out := map[string]string{}
	rest := strings.TrimSpace(header)
	if scheme, after, ok := strings.Cut(rest, " "); ok && strings.EqualFold(scheme, "Bearer") {
		rest = after
	} else if strings.EqualFold(rest, "Bearer") {
		return out
	}
	for rest != "" {
		rest = strings.TrimLeft(rest, " ,")
		name, after, ok := strings.Cut(rest, "=")
		if !ok {
			break
		}
		name = strings.ToLower(strings.TrimSpace(name))
		after = strings.TrimLeft(after, " ")
		var value string
		if strings.HasPrefix(after, `"`) {
			end := 1
			var b strings.Builder
			for end < len(after) && after[end] != '"' {
				if after[end] == '\\' && end+1 < len(after) {
					end++
				}
				b.WriteByte(after[end])
				end++
			}
			value = b.String()
			if end < len(after) {
				end++
			}
			rest = after[end:]
		} else {
			value, rest, _ = strings.Cut(after, ",")
			value = strings.TrimSpace(value)
		}
		out[name] = value
	}
	return out
}
