// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package model

import (
	"encoding/json"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// Tools (Antimatter): what an agent may do besides talking. The server keeps a registry of them: its own (built in),
// the tools apps register, and the tools of the MCP servers admins add. A tool always runs as the person who asked
// the agent, with their permissions and their own sign-in to other services, never as the agent or an admin.
//
// A tool's full name is its namespace and its own name: "antimatter.search_messages" for the server's,
// "<app id>.<name>" for an app's, "mcp.<server>.<name>" for an MCP server's. Agents' tool policies
// (AIAgentToolPolicy) name tools this way, with "<namespace>.*" for every tool of a namespace and "*" for all.

const (
	// AIToolKindRead looks something up; AIToolKindWrite changes something here; AIToolKindExternalWrite changes
	// something in another service. Only tools that read may run without asking the person first.
	AIToolKindRead          = "read"
	AIToolKindWrite         = "write"
	AIToolKindExternalWrite = "external_write"

	// The sources of tools.
	AIToolSourceBuiltIn = "builtin"
	AIToolSourceApp     = "app"
	AIToolSourceMCP     = "mcp"

	// AIToolBuiltInNamespace is the namespace of the server's own tools.
	AIToolBuiltInNamespace = "antimatter"
	// AIToolMCPNamespacePrefix starts the namespace of an MCP server's tools ("mcp.<slug>").
	AIToolMCPNamespacePrefix = "mcp."

	AIToolLocalNameMaxLength   = 64
	AIToolTitleMaxRunes        = 128
	AIToolDescriptionMaxRunes  = 4096
	AIToolSchemaMaxBytes       = 64 * 1024
	AIToolMaxPerSource         = 128
	AIToolMaxPermissions       = 8
	AIToolArgumentsMaxBytes    = 64 * 1024
	AIToolResultMaxRunes       = 32000
	AIToolCallDisplayMaxRunes  = 4000
	AIToolChoicesMaxItems      = 512
	AIToolCallConfirmationDays = 1

	// PreferenceCategoryAITools keeps a person's tool choices (PreferenceNameAIToolChoices, an AIToolChoices).
	PreferenceCategoryAITools   = "ai_tools"
	PreferenceNameAIToolChoices = "choices"

	// PostPropAgentToolCalls lists, on an agent's reply, the tool calls it proposed that wait for (or got) the
	// person's answer ([]AIToolCallSummary).
	PostPropAgentToolCalls = "agent_tool_calls"
	// PostPropAgentPostedFor marks a message a person posted with a tool, through an agent (the agent's user ID).
	PostPropAgentPostedFor = "agent_posted_for"
)

// The states of a tool call that waits for the person.
const (
	AIToolCallPending  = "pending"
	AIToolCallDone     = "done"
	AIToolCallFailed   = "failed"
	AIToolCallDeclined = "declined"
	AIToolCallExpired  = "expired"
)

var aiToolLocalName = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// IsValidAIToolLocalName tells whether a name is a tool's own name (without its namespace).
func IsValidAIToolLocalName(name string) bool {
	return aiToolLocalName.MatchString(name)
}

// IsValidAIToolKind tells whether a kind is one of the kinds of tools.
func IsValidAIToolKind(kind string) bool {
	switch kind {
	case AIToolKindRead, AIToolKindWrite, AIToolKindExternalWrite:
		return true
	}
	return false
}

// AIToolPermission is a permission a tool needs, checked as the person before the tool runs: on the channel named
// by the argument ChannelArgument, on the community named by TeamArgument, else on the server.
type AIToolPermission struct {
	Permission      string `json:"permission"`
	ChannelArgument string `json:"channel_argument,omitempty"`
	TeamArgument    string `json:"team_argument,omitempty"`
}

// AIToolDefinition is a tool as an app declares it (plugin API RegisterAITools). Its name is the app's own; the
// server adds the app's ID as its namespace.
type AIToolDefinition struct {
	Name        string             `json:"name"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	InputSchema json.RawMessage    `json:"input_schema"`
	Kind        string             `json:"kind"`
	Permissions []AIToolPermission `json:"permissions,omitempty"`
}

// IsValid checks an app's tool.
func (d *AIToolDefinition) IsValid() *AppError {
	invalid := func(detail string) *AppError {
		return NewAppError("AIToolDefinition.IsValid", "model.ai_tool.definition.app_error", nil, detail, http.StatusBadRequest)
	}
	if d == nil {
		return invalid("nil")
	}
	if !IsValidAIToolLocalName(d.Name) {
		return invalid("name=" + d.Name)
	}
	if utf8.RuneCountInString(d.Title) > AIToolTitleMaxRunes || utf8.RuneCountInString(d.Description) > AIToolDescriptionMaxRunes {
		return invalid("too long: " + d.Name)
	}
	if !IsValidAIToolKind(d.Kind) {
		return invalid("kind=" + d.Kind)
	}
	if appErr := ValidateAIToolSchema(d.InputSchema); appErr != nil {
		return appErr
	}
	if len(d.Permissions) > AIToolMaxPermissions {
		return invalid("too many permissions: " + d.Name)
	}
	for _, p := range d.Permissions {
		if p.Permission == "" || len(p.Permission) > 128 || (p.ChannelArgument != "" && p.TeamArgument != "") {
			return invalid("permission of " + d.Name)
		}
		if len(p.ChannelArgument) > AIToolLocalNameMaxLength || len(p.TeamArgument) > AIToolLocalNameMaxLength {
			return invalid("permission argument of " + d.Name)
		}
	}
	return nil
}

// ValidateAIToolSchema checks a tool's input schema: empty (no arguments) or a JSON object of at most
// AIToolSchemaMaxBytes.
func ValidateAIToolSchema(schema json.RawMessage) *AppError {
	if len(schema) == 0 {
		return nil
	}
	if len(schema) > AIToolSchemaMaxBytes {
		return NewAppError("ValidateAIToolSchema", "model.ai_tool.schema.app_error", nil, "too large", http.StatusBadRequest)
	}
	var object map[string]any
	if err := json.Unmarshal(schema, &object); err != nil {
		return NewAppError("ValidateAIToolSchema", "model.ai_tool.schema.app_error", nil, "", http.StatusBadRequest).Wrap(err)
	}
	return nil
}

// AITool is a tool as the registry has it.
type AITool struct {
	// Name is the full name, "<namespace>.<local name>".
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	LocalName string `json:"local_name"`
	// Source is builtin, app or mcp; SourceId the app's ID or the MCP server's.
	Source      string             `json:"source"`
	SourceId    string             `json:"source_id"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	InputSchema json.RawMessage    `json:"input_schema,omitempty"`
	Kind        string             `json:"kind"`
	Permissions []AIToolPermission `json:"permissions,omitempty"`
}

// IsRead tells whether the tool only reads.
func (t *AITool) IsRead() bool {
	return t.Kind == AIToolKindRead
}

// SourceKey names the tool's source for a person's choices: "builtin", "app:<id>" or "mcp:<server id>".
func (t *AITool) SourceKey() string {
	return AIToolSourceKey(t.Source, t.SourceId)
}

// AIToolSourceKey names a source of tools for a person's choices.
func AIToolSourceKey(source, sourceID string) string {
	if source == AIToolSourceBuiltIn {
		return AIToolSourceBuiltIn
	}
	return source + ":" + sourceID
}

// AIToolFullName gives a tool's full name.
func AIToolFullName(namespace, localName string) string {
	return namespace + "." + localName
}

// AIToolPatternMatches tells whether a policy's entry names a tool: the tool's full name, "<namespace>.*" for every
// tool of its namespace, or "*". Namespaces may contain dots (app IDs, "mcp.<server>"): "a.*" names the tools of the
// namespace "a" only, never those of "a.b".
func AIToolPatternMatches(pattern string, tool *AITool) bool {
	if tool == nil || pattern == "" {
		return false
	}
	if pattern == "*" {
		return true
	}
	if namespace, ok := strings.CutSuffix(pattern, ".*"); ok {
		return namespace == tool.Namespace
	}
	return pattern == tool.Name
}

// Allows tells whether an agent's policy lets it use a tool.
func (p *AIAgentToolPolicy) Allows(tool *AITool) bool {
	if p == nil || !p.Enabled || tool == nil {
		return false
	}
	return slices.ContainsFunc(p.Allowed, func(pattern string) bool { return AIToolPatternMatches(pattern, tool) })
}

// AutoRuns tells whether a tool the policy allows runs without asking the person first: only a tool that reads, and
// only when the policy says so. A tool that changes something always waits for the person's confirmation.
func (p *AIAgentToolPolicy) AutoRuns(tool *AITool) bool {
	if !p.Allows(tool) || !tool.IsRead() {
		return false
	}
	return slices.ContainsFunc(p.AutoRun, func(pattern string) bool { return AIToolPatternMatches(pattern, tool) })
}

// AIToolChoices are the tools a person turned off for themselves, for every agent: whole sources (SourceKey) and
// single tools (full names). Everything an agent's policy allows is on unless turned off here; nothing here can turn
// on what the policy doesn't allow.
type AIToolChoices struct {
	OffSources []string `json:"off_sources"`
	OffTools   []string `json:"off_tools"`
}

// Allows tells whether the person keeps a tool on.
func (c *AIToolChoices) Allows(tool *AITool) bool {
	if c == nil {
		return true
	}
	return !slices.Contains(c.OffSources, tool.SourceKey()) && !slices.Contains(c.OffTools, tool.Name)
}

// IsValid checks a person's choices.
func (c *AIToolChoices) IsValid() *AppError {
	if len(c.OffSources) > AIToolChoicesMaxItems || len(c.OffTools) > AIToolChoicesMaxItems {
		return NewAppError("AIToolChoices.IsValid", "model.ai_tool.choices.app_error", nil, "too many", http.StatusBadRequest)
	}
	for _, item := range append(slices.Clone(c.OffSources), c.OffTools...) {
		if item == "" || len(item) > AIAgentToolNameMaxLength || strings.ContainsAny(item, " \t\r\n") {
			return NewAppError("AIToolChoices.IsValid", "model.ai_tool.choices.app_error", nil, "", http.StatusBadRequest)
		}
	}
	return nil
}

// ParseAIToolChoices reads a person's choices from their preference; a value that isn't valid counts as none.
func ParseAIToolChoices(value string) *AIToolChoices {
	choices := &AIToolChoices{}
	if value == "" {
		return choices
	}
	if err := json.Unmarshal([]byte(value), choices); err != nil || choices.IsValid() != nil {
		return &AIToolChoices{}
	}
	return choices
}

// AIToolInfo is a tool as a person sees it for an agent: whether they keep it on, and whether it runs without asking.
type AIToolInfo struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Kind        string `json:"kind"`
	On          bool   `json:"on"`
	AutoRun     bool   `json:"auto_run"`
}

// AIToolSourceInfo is a source of an agent's tools as a person sees it: the server's own tools, an app's, or an MCP
// server's, with whether they keep it on and, for an MCP server where everyone signs in, whether they did.
type AIToolSourceInfo struct {
	Key    string `json:"key"`
	Source string `json:"source"`
	// SourceId is the app's ID or the MCP server's; Name is shown.
	SourceId string `json:"source_id"`
	Name     string `json:"name"`
	On       bool   `json:"on"`
	// Auth is an MCP server's sign-in: AIMCPAuthNone, AIMCPAuthOAuth (each person signs in) or
	// AIMCPAuthServiceAccount (one shared account an admin set up: its tools only read).
	Auth string `json:"auth,omitempty"`
	// Connected says whether the person signed in to an MCP server that needs it.
	Connected bool `json:"connected"`
	// Unavailable is set when the source's tools couldn't be listed (the server didn't answer).
	Unavailable bool          `json:"unavailable"`
	Tools       []*AIToolInfo `json:"tools"`
}

// AIToolCatalogue is what an agent's managers may give it: every source and tool they may name in its policy.
type AIToolCatalogue struct {
	Sources []*AIToolCatalogueSource `json:"sources"`
}

// AIToolCatalogueSource is a source of tools for an agent's policy: its namespace (for "<namespace>.*") and the tools
// it's known to have (an MCP server where each person signs in lists its tools to signed-in people only).
type AIToolCatalogueSource struct {
	Key       string        `json:"key"`
	Source    string        `json:"source"`
	SourceId  string        `json:"source_id"`
	Name      string        `json:"name"`
	Namespace string        `json:"namespace"`
	Auth      string        `json:"auth,omitempty"`
	Tools     []*AIToolInfo `json:"tools"`
}

// AIToolCallSummary is what an agent's reply shows of a tool call that waits for the person: the tool, its arguments
// as the model gave them (shortened), and where it stands.
type AIToolCallSummary struct {
	Id        string `json:"id"`
	Tool      string `json:"tool"`
	Title     string `json:"title"`
	Kind      string `json:"kind"`
	Arguments string `json:"arguments"`
	Status    string `json:"status"`
	// UserId is the person who must confirm it.
	UserId string `json:"user_id"`
}

// AIToolExecution is a call of an app's tool (the ExecuteAITool hook): the tool's own name, its arguments, the person
// it runs for (verified by the server: the app acts with their permissions only), and where.
type AIToolExecution struct {
	CallId    string          `json:"call_id"`
	Tool      string          `json:"tool"`
	Arguments json.RawMessage `json:"arguments"`
	UserId    string          `json:"user_id"`
	ChannelId string          `json:"channel_id"`
	TeamId    string          `json:"team_id"`
	RootId    string          `json:"root_id"`
	AgentId   string          `json:"agent_id"`
}

// AIToolResult is what a tool gives back: text for the model, or what went wrong.
type AIToolResult struct {
	Text  string `json:"text"`
	Error string `json:"error,omitempty"`
}
