// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package model

import (
	"net/http"
	"slices"
	"strings"
	"unicode/utf8"
)

// Agents (Antimatter) are bot accounts of kind "agent": people mention them, message them and hand them tasks like a
// coworker. A built-in agent runs in the server's agent runtime with its settings (a model connection and model, a
// system prompt, its tools, who may talk to it); an external agent is a program elsewhere that signs in with the
// account's token and answers like a person would, with no runtime in the server. System admins manage the
// server-wide agents; a community's admins manage their community's own.

const (
	// BotKindBot is a plain bot account; BotKindAgent an agent.
	BotKindBot   = "bot"
	BotKindAgent = "agent"

	// AIAgentScopeServer is the scope of a server-wide agent; any other scope is a team (community) ID.
	AIAgentScopeServer = AIConnectionScopeServer

	// AIAgentBotOwnerId owns the bot accounts of the agents made in Antimatter. It isn't a user, so an agent outlives
	// the person who made it (bots owned by a user are disabled with them), and the agent's creator is kept in its
	// settings instead.
	AIAgentBotOwnerId = "antimatter-agents"

	// AIAgentsPluginId owned the bots of the Agents plugin (now the AI chat app); they became agents.
	AIAgentsPluginId = "mattermost-ai"

	AIAgentSystemPromptMaxRunes = 32000
	AIAgentDefaultMaxToolTurns  = 30
	AIAgentMaxToolTurns         = 250
	AIAgentMaxListItems         = 256
	AIAgentToolNameMaxLength    = 128
	AIAgentReasoningMaxBudget   = 128000

	AIAgentReasoningEffortMinimal = "minimal"
	AIAgentReasoningEffortLow     = "low"
	AIAgentReasoningEffortMedium  = "medium"
	AIAgentReasoningEffortHigh    = "high"
)

// AIAgentToolPolicy says which tools an agent may use, and which of them run without asking the person first. Tool
// names are namespaced by their app ("todo.add_item"); "todo.*" names every tool of an app and "*" every tool. Only
// tools that read may run without asking: a tool that changes something always needs the person's confirmation,
// whatever AutoRun says (the tool registry enforces it).
type AIAgentToolPolicy struct {
	Enabled bool     `json:"enabled"`
	Allowed []string `json:"allowed"`
	AutoRun []string `json:"auto_run"`
}

// AIAgentReasoning asks models that can reason before answering to do so: an effort for OpenAI-style models, a token
// budget for Anthropic's (0: the server picks one).
type AIAgentReasoning struct {
	Enabled      bool   `json:"enabled"`
	Effort       string `json:"effort"`
	BudgetTokens int    `json:"budget_tokens"`
}

// AIAgentAccess says who may talk to an agent: everyone in its scope, or the people of chosen communities, the
// members of chosen channels, the holders of chosen roles (system_admin, team_admin...) and chosen people. Guests
// only when AllowGuests is on, even with Everyone. A community's agent is only for its community's members.
type AIAgentAccess struct {
	Everyone    bool     `json:"everyone"`
	TeamIDs     []string `json:"team_ids"`
	ChannelIDs  []string `json:"channel_ids"`
	Roles       []string `json:"roles"`
	UserIDs     []string `json:"user_ids"`
	AllowGuests bool     `json:"allow_guests"`
}

// AIAgent is an agent's settings, keyed by its bot account's user ID.
type AIAgent struct {
	UserId string `json:"user_id"`
	Scope  string `json:"scope"`

	// External agents are programs outside the server: they sign in with the account's token and the runtime never
	// answers for them; their model settings are empty.
	External bool `json:"external"`

	ConnectionId string            `json:"connection_id"`
	Model        string            `json:"model"`
	SystemPrompt string            `json:"system_prompt"`
	Tools        AIAgentToolPolicy `json:"tools"`
	Reasoning    AIAgentReasoning  `json:"reasoning"`
	MaxToolTurns int               `json:"max_tool_turns"`
	Access       AIAgentAccess     `json:"access"`

	// NeedsAttention, when set, says what an admin should review (an agent migrated from the Agents plugin whose
	// access couldn't be carried over as it was). Saving any change clears it.
	NeedsAttention string `json:"needs_attention"`

	CreatorId string `json:"creator_id"`
	UpdaterId string `json:"updater_id"`
	CreateAt  int64  `json:"create_at"`
	UpdateAt  int64  `json:"update_at"`
	DeleteAt  int64  `json:"delete_at"`
}

// AIAgentDetails is an agent as its managers see it: its settings and its account. No key ever appears here: the
// connection is named by its ID and name only.
type AIAgentDetails struct {
	*AIAgent
	Username       string `json:"username"`
	DisplayName    string `json:"display_name"`
	Description    string `json:"description"`
	LastIconUpdate int64  `json:"last_icon_update"`
	// Enabled is the account's state: a disabled agent's account is deactivated, and nobody can talk to it.
	Enabled        bool   `json:"enabled"`
	ConnectionName string `json:"connection_name"`
}

// AIAgentRequest creates an agent and its account.
type AIAgentRequest struct {
	Username     string            `json:"username"`
	DisplayName  string            `json:"display_name"`
	Description  string            `json:"description"`
	Scope        string            `json:"scope"`
	External     bool              `json:"external"`
	ConnectionId string            `json:"connection_id"`
	Model        string            `json:"model"`
	SystemPrompt string            `json:"system_prompt"`
	Tools        AIAgentToolPolicy `json:"tools"`
	Reasoning    AIAgentReasoning  `json:"reasoning"`
	MaxToolTurns int               `json:"max_tool_turns"`
	Access       AIAgentAccess     `json:"access"`
}

// AIAgentPatch changes an agent. A field left out is kept; the scope and whether it's external never change.
type AIAgentPatch struct {
	Username     *string            `json:"username,omitempty"`
	DisplayName  *string            `json:"display_name,omitempty"`
	Description  *string            `json:"description,omitempty"`
	ConnectionId *string            `json:"connection_id,omitempty"`
	Model        *string            `json:"model,omitempty"`
	SystemPrompt *string            `json:"system_prompt,omitempty"`
	Tools        *AIAgentToolPolicy `json:"tools,omitempty"`
	Reasoning    *AIAgentReasoning  `json:"reasoning,omitempty"`
	MaxToolTurns *int               `json:"max_tool_turns,omitempty"`
	Access       *AIAgentAccess     `json:"access,omitempty"`
}

// IsServerScoped tells whether the agent is server-wide.
func (a *AIAgent) IsServerScoped() bool {
	return a.Scope == AIAgentScopeServer
}

// TeamId is the community of a community agent, "" for a server-wide one.
func (a *AIAgent) TeamId() string {
	if a.IsServerScoped() {
		return ""
	}
	return a.Scope
}

// EffectiveMaxToolTurns is the agent's tool turns, the default when unset.
func (a *AIAgent) EffectiveMaxToolTurns() int {
	if a.MaxToolTurns <= 0 {
		return AIAgentDefaultMaxToolTurns
	}
	return min(a.MaxToolTurns, AIAgentMaxToolTurns)
}

// Clone gives a copy of the agent with its own lists.
func (a *AIAgent) Clone() *AIAgent {
	c := *a
	c.Tools.Allowed = slices.Clone(a.Tools.Allowed)
	c.Tools.AutoRun = slices.Clone(a.Tools.AutoRun)
	c.Access.TeamIDs = slices.Clone(a.Access.TeamIDs)
	c.Access.ChannelIDs = slices.Clone(a.Access.ChannelIDs)
	c.Access.Roles = slices.Clone(a.Access.Roles)
	c.Access.UserIDs = slices.Clone(a.Access.UserIDs)
	return &c
}

// PreSave fills in a new agent's times and normalises its lists.
func (a *AIAgent) PreSave() {
	if a.CreateAt == 0 {
		a.CreateAt = GetMillis()
	}
	a.UpdateAt = a.CreateAt
	a.normalize()
}

// PreUpdate sets the update time and normalises the lists.
func (a *AIAgent) PreUpdate() {
	a.UpdateAt = GetMillis()
	a.normalize()
}

func cleanAIAgentList(list []string) []string {
	out := make([]string, 0, len(list))
	for _, item := range list {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func (a *AIAgent) normalize() {
	a.Model = strings.TrimSpace(a.Model)
	a.SystemPrompt = strings.TrimSpace(a.SystemPrompt)
	a.Reasoning.Effort = strings.ToLower(strings.TrimSpace(a.Reasoning.Effort))
	a.Tools.Allowed = cleanAIAgentList(a.Tools.Allowed)
	a.Tools.AutoRun = cleanAIAgentList(a.Tools.AutoRun)
	a.Access.TeamIDs = cleanAIAgentList(a.Access.TeamIDs)
	a.Access.ChannelIDs = cleanAIAgentList(a.Access.ChannelIDs)
	a.Access.Roles = cleanAIAgentList(a.Access.Roles)
	a.Access.UserIDs = cleanAIAgentList(a.Access.UserIDs)
	if a.External {
		// An external agent has no runtime: nothing of the model's applies to it.
		a.ConnectionId = ""
		a.Model = ""
		a.SystemPrompt = ""
		a.Tools = AIAgentToolPolicy{Allowed: []string{}, AutoRun: []string{}}
		a.Reasoning = AIAgentReasoning{}
		a.MaxToolTurns = 0
	}
}

// IsValidAIAgentToolName tells whether a name is a tool name, an app's wildcard ("todo.*") or "*".
func IsValidAIAgentToolName(name string) bool {
	if name == "" || len(name) > AIAgentToolNameMaxLength {
		return false
	}
	if name == "*" {
		return true
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("._-*:/", r)) {
			return false
		}
	}
	return !strings.HasPrefix(name, ".") && !strings.Contains(name, "..")
}

func aiAgentError(field string) *AppError {
	return NewAppError("AIAgent.IsValid", "model.ai_agent."+field+".app_error", nil, "", http.StatusBadRequest)
}

func validAIAgentIDs(ids []string) bool {
	if len(ids) > AIAgentMaxListItems {
		return false
	}
	for _, id := range ids {
		if !IsValidId(id) {
			return false
		}
	}
	return true
}

// IsValid checks an agent's settings before they're saved.
func (a *AIAgent) IsValid() *AppError {
	if !IsValidId(a.UserId) {
		return aiAgentError("user_id")
	}
	if a.Scope != AIAgentScopeServer && !IsValidId(a.Scope) {
		return aiAgentError("scope")
	}
	if a.External {
		if a.ConnectionId != "" || a.Model != "" || a.SystemPrompt != "" {
			return aiAgentError("external")
		}
	} else if !IsValidId(a.ConnectionId) {
		return aiAgentError("connection_id")
	}
	if len(a.Model) > AIModelNameMaxLength || strings.ContainsAny(a.Model, "\r\n") {
		return aiAgentError("model")
	}
	if utf8.RuneCountInString(a.SystemPrompt) > AIAgentSystemPromptMaxRunes {
		return aiAgentError("system_prompt")
	}
	if len(a.Tools.Allowed) > AIAgentMaxListItems || len(a.Tools.AutoRun) > AIAgentMaxListItems {
		return aiAgentError("tools")
	}
	for _, name := range append(slices.Clone(a.Tools.Allowed), a.Tools.AutoRun...) {
		if !IsValidAIAgentToolName(name) {
			return aiAgentError("tools")
		}
	}
	switch a.Reasoning.Effort {
	case "", AIAgentReasoningEffortMinimal, AIAgentReasoningEffortLow, AIAgentReasoningEffortMedium, AIAgentReasoningEffortHigh:
	default:
		return aiAgentError("reasoning")
	}
	if a.Reasoning.BudgetTokens < 0 || a.Reasoning.BudgetTokens > AIAgentReasoningMaxBudget {
		return aiAgentError("reasoning")
	}
	if a.MaxToolTurns < 0 || a.MaxToolTurns > AIAgentMaxToolTurns {
		return aiAgentError("max_tool_turns")
	}
	if !validAIAgentIDs(a.Access.TeamIDs) || !validAIAgentIDs(a.Access.ChannelIDs) || !validAIAgentIDs(a.Access.UserIDs) {
		return aiAgentError("access")
	}
	if len(a.Access.Roles) > AIAgentMaxListItems {
		return aiAgentError("access")
	}
	for _, role := range a.Access.Roles {
		if !IsValidRoleName(role) {
			return aiAgentError("access")
		}
	}
	if !a.IsServerScoped() && slices.ContainsFunc(a.Access.TeamIDs, func(id string) bool { return id != a.Scope }) {
		// A community's agent can't be opened to other communities.
		return aiAgentError("access_community")
	}
	if utf8.RuneCountInString(a.NeedsAttention) > AINeedsAttentionMaxRunes {
		return aiAgentError("needs_attention")
	}
	if a.CreateAt == 0 || a.UpdateAt == 0 {
		return aiAgentError("times")
	}
	return nil
}

// Auditable gives what an audit record keeps of an agent: its settings without the system prompt's text.
func (a *AIAgent) Auditable() map[string]any {
	return map[string]any{
		"user_id":            a.UserId,
		"scope":              a.Scope,
		"external":           a.External,
		"connection_id":      a.ConnectionId,
		"model":              a.Model,
		"system_prompt_size": utf8.RuneCountInString(a.SystemPrompt),
		"tools":              a.Tools,
		"reasoning":          a.Reasoning,
		"max_tool_turns":     a.MaxToolTurns,
		"access":             a.Access,
		"delete_at":          a.DeleteAt,
	}
}

// Agent activity: what an agent is doing now, shown under its name and sent live to everyone who can see it.

const (
	AgentStateIdle     = "idle"
	AgentStateThinking = "thinking"
	AgentStateWorking  = "working"
	// AgentStateWaiting: the agent waits for someone (an answer, a confirmation).
	AgentStateWaiting = "waiting"
	// AgentStateBlocked: the agent can't go on without help.
	AgentStateBlocked = "blocked"
	AgentStateOffline = "offline"

	AgentActivityTextMaxRunes = 200
	// AgentActivityMaxSeconds bounds how long a state lasts without being renewed; busy states expire after
	// AgentActivityDefaultSeconds unless the agent says otherwise, so a stopped agent doesn't look busy forever.
	AgentActivityMaxSeconds     = 24 * 60 * 60
	AgentActivityDefaultSeconds = 10 * 60
)

// AgentActivity is an agent's current state, a few words on the task, and the thread it works in.
type AgentActivity struct {
	UserId       string `json:"user_id"`
	State        string `json:"state"`
	Text         string `json:"text"`
	ThreadPostId string `json:"thread_post_id"`
	UpdateAt     int64  `json:"update_at"`
	// ExpiresAt, when set, is when the state stops holding: the agent is then idle.
	ExpiresAt int64 `json:"expires_at"`
}

// AgentActivityRequest sets an agent's activity. ExpiresInSeconds of 0 keeps a busy state for
// AgentActivityDefaultSeconds; idle and offline never expire.
type AgentActivityRequest struct {
	State            string `json:"state"`
	Text             string `json:"text"`
	ThreadPostId     string `json:"thread_post_id"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
}

// IsValidAgentState tells whether s is one of the agent states.
func IsValidAgentState(s string) bool {
	switch s {
	case AgentStateIdle, AgentStateThinking, AgentStateWorking, AgentStateWaiting, AgentStateBlocked, AgentStateOffline:
		return true
	}
	return false
}

// IsBusyAgentState tells whether a state is one that expires.
func IsBusyAgentState(s string) bool {
	return s != AgentStateIdle && s != AgentStateOffline
}

// Validate checks a request and gives the activity it sets.
func (r *AgentActivityRequest) Validate(userID string, now int64) (*AgentActivity, *AppError) {
	state := strings.TrimSpace(r.State)
	if !IsValidAgentState(state) {
		return nil, NewAppError("AgentActivity.IsValid", "model.agent_activity.state.app_error", nil, "", http.StatusBadRequest)
	}
	text := strings.TrimSpace(SanitizeUnicode(r.Text))
	if strings.ContainsAny(text, "\r\n") || utf8.RuneCountInString(text) > AgentActivityTextMaxRunes {
		return nil, NewAppError("AgentActivity.IsValid", "model.agent_activity.text.app_error", nil, "", http.StatusBadRequest)
	}
	if r.ThreadPostId != "" && !IsValidId(r.ThreadPostId) {
		return nil, NewAppError("AgentActivity.IsValid", "model.agent_activity.thread_post_id.app_error", nil, "", http.StatusBadRequest)
	}
	if r.ExpiresInSeconds < 0 || r.ExpiresInSeconds > AgentActivityMaxSeconds {
		return nil, NewAppError("AgentActivity.IsValid", "model.agent_activity.expires.app_error", nil, "", http.StatusBadRequest)
	}
	activity := &AgentActivity{UserId: userID, State: state, Text: text, ThreadPostId: r.ThreadPostId, UpdateAt: now}
	if IsBusyAgentState(state) {
		seconds := r.ExpiresInSeconds
		if seconds == 0 {
			seconds = AgentActivityDefaultSeconds
		}
		activity.ExpiresAt = now + int64(seconds)*1000
	} else {
		activity.ThreadPostId = ""
	}
	return activity, nil
}

// Effective gives the activity as it holds at now: an expired busy state is idle again.
func (a *AgentActivity) Effective(now int64) *AgentActivity {
	if a.ExpiresAt != 0 && now >= a.ExpiresAt {
		return &AgentActivity{UserId: a.UserId, State: AgentStateIdle, UpdateAt: a.ExpiresAt}
	}
	c := *a
	return &c
}

// WithoutThread gives a copy without the thread, for those who may not read it.
func (a *AgentActivity) WithoutThread() *AgentActivity {
	c := *a
	c.ThreadPostId = ""
	return &c
}

// AIAgentsPluginAgent is an agent of the Agents plugin (now the AI chat app) as its configuration or its table
// Agents_UserAgents keeps it, with the configuration's JSON names; the one-time migration makes server agents of them.
// The access levels are the plugin's: 0 all, 1 allow list, 2 block list, 3 none (users also 4, attribute-based).
type AIAgentsPluginAgent struct {
	BotUserID          string   `json:"botUserID"`
	Name               string   `json:"name"`
	DisplayName        string   `json:"displayName"`
	CustomInstructions string   `json:"customInstructions"`
	ServiceID          string   `json:"serviceID"`
	Model              string   `json:"model"`
	DisableTools       bool     `json:"disableTools"`
	ChannelAccessLevel int      `json:"channelAccessLevel"`
	ChannelIDs         []string `json:"channelIDs"`
	UserAccessLevel    int      `json:"userAccessLevel"`
	UserIDs            []string `json:"userIDs"`
	TeamIDs            []string `json:"teamIDs"`
	ReasoningEnabled   bool     `json:"reasoningEnabled"`
	ReasoningEffort    string   `json:"reasoningEffort"`
	ThinkingBudget     int      `json:"thinkingBudget"`
	MaxToolTurns       int      `json:"maxToolTurns"`
	CreatorID          string   `json:"creatorID"`
}
