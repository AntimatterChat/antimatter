// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package model

import (
	"net/http"
	"net/textproto"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// A model connection is the only place an AI provider's address and key live (Antimatter). System admins manage the
// server-wide ones, which they may share with chosen communities or all of them; a community's admins manage its
// own, used only in it. The key is write-only: it is stored encrypted, and the API only says whether one is set and
// gives its last four characters.

const (
	// AIConnectionScopeServer is the scope of a server-wide connection; any other scope is a team (community) ID.
	AIConnectionScopeServer = "server"

	AIProviderOpenAICompatible = "openai_compatible"
	AIProviderAnthropic        = "anthropic"

	AIDecisionsDialectOpenAI    = "openai"
	AIDecisionsDialectSystemOne = "systemone"

	AIConnectionNameMaxRunes   = 64
	AIConnectionBaseURLMaxLen  = 1024
	AIConnectionAPIKeyMaxLen   = 8192
	AIConnectionMaxHeaders     = 16
	AIConnectionHeaderValueMax = 4096
	AIConnectionMaxListItems   = 256
	AIModelNameMaxLength       = 256
	AINeedsAttentionMaxRunes   = 1024
)

// AIConnectionModels are a connection's default model for each capability; empty means the capability isn't set up.
type AIConnectionModels struct {
	Chat          string `json:"chat"`
	Embeddings    string `json:"embeddings"`
	Transcription string `json:"transcription"`
	Decisions     string `json:"decisions"`
}

// AIConnectionLimits bound a connection's use; 0 means no limit. Requests per minute are counted on each server node;
// token budgets per calendar month (UTC), for the whole cluster.
type AIConnectionLimits struct {
	RequestsPerMinute         int   `json:"requests_per_minute"`
	MonthlyTokenBudget        int64 `json:"monthly_token_budget"`
	PerUserRequestsPerMinute  int   `json:"per_user_requests_per_minute"`
	PerUserMonthlyTokenBudget int64 `json:"per_user_monthly_token_budget"`
}

// AIConnectionSharing says in which communities people may use a server-wide connection: all of them, or those
// listed. Direct and group messages, which belong to no community, need all of them. A community's own connection
// has no sharing.
type AIConnectionSharing struct {
	AllTeams bool     `json:"all_teams"`
	TeamIDs  []string `json:"team_ids"`
}

// AIConnectionAudience says who may use a connection where it's available: everyone, or the members of chosen
// groups (Antimatter teams and roles) and the holders of chosen built-in roles (system_admin, team_admin...).
// Guests may only when AllowGuests is on, even with Everyone. The server's own features (rewrite, recaps,
// translation, speech) aren't bound by it: an admin chose the connection for them.
type AIConnectionAudience struct {
	Everyone    bool     `json:"everyone"`
	GroupIDs    []string `json:"group_ids"`
	Roles       []string `json:"roles"`
	AllowGuests bool     `json:"allow_guests"`
}

// AIConnection is a model connection as the API gives it: never its key or header values.
type AIConnection struct {
	Id       string `json:"id"`
	Name     string `json:"name"`
	Scope    string `json:"scope"`
	Provider string `json:"provider"`
	BaseURL  string `json:"base_url"`

	// APIKeySealed and ExtraHeadersSealed are encrypted (aiservice.Keyring); they never leave the server.
	APIKeySealed       string `json:"-"`
	ExtraHeadersSealed string `json:"-"`

	APIKeySet        bool     `json:"api_key_set"`
	APIKeyLast4      string   `json:"api_key_last4"`
	ExtraHeaderNames []string `json:"extra_header_names"`

	Models           AIConnectionModels   `json:"models"`
	DecisionsDialect string               `json:"decisions_dialect"`
	Limits           AIConnectionLimits   `json:"limits"`
	Sharing          AIConnectionSharing  `json:"sharing"`
	Audience         AIConnectionAudience `json:"audience"`

	// NeedsAttention, when set, says why the connection can't be used until an admin fixes it (e.g. an Agents
	// plugin service of a type Antimatter doesn't speak). Saving any change clears it.
	NeedsAttention string `json:"needs_attention"`

	CreatorId string `json:"creator_id"`
	UpdaterId string `json:"updater_id"`
	CreateAt  int64  `json:"create_at"`
	UpdateAt  int64  `json:"update_at"`
	DeleteAt  int64  `json:"delete_at"`
}

// AIConnectionRequest creates a connection, or tests one before it's saved. APIKey and ExtraHeaders are in clear
// here only: they're encrypted before being stored.
type AIConnectionRequest struct {
	Name             string               `json:"name"`
	Scope            string               `json:"scope"`
	Provider         string               `json:"provider"`
	BaseURL          string               `json:"base_url"`
	APIKey           string               `json:"api_key"`
	ExtraHeaders     map[string]string    `json:"extra_headers"`
	Models           AIConnectionModels   `json:"models"`
	DecisionsDialect string               `json:"decisions_dialect"`
	Limits           AIConnectionLimits   `json:"limits"`
	Sharing          AIConnectionSharing  `json:"sharing"`
	Audience         AIConnectionAudience `json:"audience"`
}

// AIConnectionPatch changes a connection. A field left out is kept; APIKey replaces the key ("" removes it) and
// ExtraHeaders replaces all the headers. The scope never changes.
type AIConnectionPatch struct {
	Name             *string               `json:"name,omitempty"`
	Provider         *string               `json:"provider,omitempty"`
	BaseURL          *string               `json:"base_url,omitempty"`
	APIKey           *string               `json:"api_key,omitempty"`
	ExtraHeaders     *map[string]string    `json:"extra_headers,omitempty"`
	Models           *AIConnectionModels   `json:"models,omitempty"`
	DecisionsDialect *string               `json:"decisions_dialect,omitempty"`
	Limits           *AIConnectionLimits   `json:"limits,omitempty"`
	Sharing          *AIConnectionSharing  `json:"sharing,omitempty"`
	Audience         *AIConnectionAudience `json:"audience,omitempty"`
}

// AIConnectionTestResult is the answer of a connection test: whether the endpoint and key work, and how long the
// call took. Error is the endpoint's message, without the key.
type AIConnectionTestResult struct {
	OK         bool   `json:"ok"`
	Error      string `json:"error,omitempty"`
	StatusCode int    `json:"status_code,omitempty"`
	LatencyMs  int64  `json:"latency_ms"`
	ModelCount int    `json:"model_count"`
}

// AIConnectionInfo is what someone who may use a connection (or an app acting for them) sees of it.
type AIConnectionInfo struct {
	Id           string             `json:"id"`
	Name         string             `json:"name"`
	Scope        string             `json:"scope"`
	Provider     string             `json:"provider"`
	Models       AIConnectionModels `json:"models"`
	Capabilities []string           `json:"capabilities"`
}

// IsServerScoped tells whether the connection is server-wide.
func (c *AIConnection) IsServerScoped() bool {
	return c.Scope == AIConnectionScopeServer
}

// TeamId is the community of a community connection, "" for a server-wide one.
func (c *AIConnection) TeamId() string {
	if c.IsServerScoped() {
		return ""
	}
	return c.Scope
}

// Capabilities lists what the connection can do: chat always, the rest with the OpenAI-compatible API.
func (c *AIConnection) Capabilities() []string {
	if c.Provider == AIProviderAnthropic {
		return []string{"chat"}
	}
	return []string{"chat", "embeddings", "transcription", "decisions"}
}

// Info gives what users of the connection see.
func (c *AIConnection) Info() *AIConnectionInfo {
	return &AIConnectionInfo{Id: c.Id, Name: c.Name, Scope: c.Scope, Provider: c.Provider, Models: c.Models, Capabilities: c.Capabilities()}
}

// SharedWithTeam tells whether people may use a server-wide connection in a community ("" for direct and group
// messages, which need it shared with all of them). A community's connection is for its own community only.
func (c *AIConnection) SharedWithTeam(teamID string) bool {
	if !c.IsServerScoped() {
		return teamID != "" && teamID == c.Scope
	}
	if c.Sharing.AllTeams {
		return true
	}
	return teamID != "" && slices.Contains(c.Sharing.TeamIDs, teamID)
}

// AIAPIKeyLast4 gives the last four characters of a key, shown in place of the key; none for a short key.
func AIAPIKeyLast4(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	n := utf8.RuneCountInString(key)
	if n <= 8 {
		// A short key: showing four of its characters would show half of it.
		return ""
	}
	runes := []rune(key)
	return string(runes[n-4:])
}

// AIConnectionAAD gives the associated data a connection's sealed field is bound to.
func AIConnectionAAD(connectionID, field string) string {
	return "aiconnection:" + connectionID + ":" + field
}

// The fields of a connection that are sealed.
const (
	AIConnectionFieldAPIKey       = "api_key"
	AIConnectionFieldExtraHeaders = "extra_headers"
)

// forbiddenAIHeaders are headers a connection may not set: the HTTP client owns them.
var forbiddenAIHeaders = []string{"Host", "Content-Length", "Content-Type", "Transfer-Encoding", "Connection", "Upgrade", "Te", "Trailer", "Cookie"}

// IsValidAIHeaderName tells whether a connection may send a header of this name.
func IsValidAIHeaderName(name string) bool {
	if name == "" || len(name) > 128 {
		return false
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", r)) {
			return false
		}
	}
	return !slices.Contains(forbiddenAIHeaders, textproto.CanonicalMIMEHeaderKey(name))
}

// ValidateAIExtraHeaders checks headers a connection would send.
func ValidateAIExtraHeaders(headers map[string]string) *AppError {
	if len(headers) > AIConnectionMaxHeaders {
		return NewAppError("AIConnection.IsValid", "model.ai_connection.extra_headers.app_error", nil, "too many", http.StatusBadRequest)
	}
	for name, value := range headers {
		if !IsValidAIHeaderName(name) || len(value) > AIConnectionHeaderValueMax || strings.ContainsAny(value, "\r\n") {
			return NewAppError("AIConnection.IsValid", "model.ai_connection.extra_headers.app_error", nil, "header="+name, http.StatusBadRequest)
		}
	}
	return nil
}

// AIHeaderNames gives the sorted names of headers.
func AIHeaderNames(headers map[string]string) []string {
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// ValidAIBaseURL tells whether a connection may have this base URL: empty (the provider's default), or http(s)
// without credentials, query or fragment.
func ValidAIBaseURL(raw string) bool {
	return isValidAIBaseURL(raw)
}

func isValidAIBaseURL(raw string) bool {
	if raw == "" {
		return true
	}
	if len(raw) > AIConnectionBaseURLMaxLen {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" {
		return false
	}
	return true
}

func validIDs(ids []string) bool {
	if len(ids) > AIConnectionMaxListItems {
		return false
	}
	for _, id := range ids {
		if !IsValidId(id) {
			return false
		}
	}
	return true
}

func aiConnectionError(field string) *AppError {
	return NewAppError("AIConnection.IsValid", "model.ai_connection."+field+".app_error", nil, "", http.StatusBadRequest)
}

// PreSave fills in a new connection's ID and times, and normalises its lists.
func (c *AIConnection) PreSave() {
	if c.Id == "" {
		c.Id = NewId()
	}
	if c.CreateAt == 0 {
		c.CreateAt = GetMillis()
	}
	c.UpdateAt = c.CreateAt
	c.normalize()
}

// PreUpdate sets the update time and normalises the lists.
func (c *AIConnection) PreUpdate() {
	c.UpdateAt = GetMillis()
	c.normalize()
}

func (c *AIConnection) normalize() {
	c.Name = strings.TrimSpace(c.Name)
	c.BaseURL = strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if c.DecisionsDialect == "" {
		c.DecisionsDialect = AIDecisionsDialectOpenAI
	}
	clean := func(list []string) []string {
		out := slices.Clone(list)
		slices.Sort(out)
		out = slices.Compact(out)
		if out == nil {
			out = []string{}
		}
		return out
	}
	c.Sharing.TeamIDs = clean(c.Sharing.TeamIDs)
	c.Audience.GroupIDs = clean(c.Audience.GroupIDs)
	c.Audience.Roles = clean(c.Audience.Roles)
	c.ExtraHeaderNames = clean(c.ExtraHeaderNames)
	c.APIKeySet = c.APIKeySealed != ""
	if !c.APIKeySet {
		c.APIKeyLast4 = ""
	}
}

// IsValid checks a connection before it's saved.
func (c *AIConnection) IsValid() *AppError {
	if !IsValidId(c.Id) {
		return aiConnectionError("id")
	}
	if n := utf8.RuneCountInString(c.Name); n == 0 || n > AIConnectionNameMaxRunes {
		return aiConnectionError("name")
	}
	if c.Scope != AIConnectionScopeServer && !IsValidId(c.Scope) {
		return aiConnectionError("scope")
	}
	if c.Provider != AIProviderOpenAICompatible && c.Provider != AIProviderAnthropic {
		return aiConnectionError("provider")
	}
	if !isValidAIBaseURL(c.BaseURL) {
		return aiConnectionError("base_url")
	}
	if c.DecisionsDialect != AIDecisionsDialectOpenAI && c.DecisionsDialect != AIDecisionsDialectSystemOne {
		return aiConnectionError("decisions_dialect")
	}
	for _, m := range []string{c.Models.Chat, c.Models.Embeddings, c.Models.Transcription, c.Models.Decisions} {
		if len(m) > AIModelNameMaxLength || strings.ContainsAny(m, "\r\n") {
			return aiConnectionError("models")
		}
	}
	if c.Provider == AIProviderAnthropic && (c.Models.Embeddings != "" || c.Models.Transcription != "" || c.Models.Decisions != "") {
		return aiConnectionError("models_anthropic")
	}
	l := c.Limits
	if l.RequestsPerMinute < 0 || l.MonthlyTokenBudget < 0 || l.PerUserRequestsPerMinute < 0 || l.PerUserMonthlyTokenBudget < 0 {
		return aiConnectionError("limits")
	}
	if !c.IsServerScoped() && (c.Sharing.AllTeams || len(c.Sharing.TeamIDs) > 0) {
		return aiConnectionError("sharing_community")
	}
	if !validIDs(c.Sharing.TeamIDs) || !validIDs(c.Audience.GroupIDs) {
		return aiConnectionError("ids")
	}
	if len(c.Audience.Roles) > AIConnectionMaxListItems {
		return aiConnectionError("roles")
	}
	for _, role := range c.Audience.Roles {
		if !IsValidRoleName(role) {
			return aiConnectionError("roles")
		}
	}
	if len(c.APIKeyLast4) > 16 || len(c.ExtraHeaderNames) > AIConnectionMaxHeaders {
		return aiConnectionError("api_key")
	}
	if utf8.RuneCountInString(c.NeedsAttention) > AINeedsAttentionMaxRunes {
		return aiConnectionError("needs_attention")
	}
	if c.CreateAt == 0 || c.UpdateAt == 0 {
		return aiConnectionError("times")
	}
	return nil
}

// Validate checks a request's own fields (not the permissions or the community's groups).
func (r *AIConnectionRequest) Validate() *AppError {
	if len(r.APIKey) > AIConnectionAPIKeyMaxLen || strings.ContainsAny(r.APIKey, "\r\n") {
		return aiConnectionError("api_key")
	}
	return ValidateAIExtraHeaders(r.ExtraHeaders)
}

// Auditable gives what an audit record may keep of a connection: never a key or a header value.
func (c *AIConnection) Auditable() map[string]any {
	return map[string]any{
		"id":                 c.Id,
		"name":               c.Name,
		"scope":              c.Scope,
		"provider":           c.Provider,
		"base_url":           c.BaseURL,
		"api_key_set":        c.APIKeySet,
		"extra_header_names": c.ExtraHeaderNames,
		"models":             c.Models,
		"limits":             c.Limits,
		"sharing":            c.Sharing,
		"audience":           c.Audience,
		"delete_at":          c.DeleteAt,
	}
}

// AIUsage counts what one person (or "" for a built-in feature without one) used of a connection, for one purpose,
// on one day (UTC, as 20261017).
type AIUsage struct {
	ConnectionId string `json:"connection_id"`
	UserId       string `json:"user_id"`
	Purpose      string `json:"purpose"`
	Day          int    `json:"day"`
	Requests     int64  `json:"requests"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
}

// AIUsageTotals adds up usage.
type AIUsageTotals struct {
	Requests     int64 `json:"requests"`
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

// Tokens are the tokens in and out.
func (t AIUsageTotals) Tokens() int64 {
	return t.InputTokens + t.OutputTokens
}

// AIConnectionUsage is a connection's usage this month, for its admins: the month's totals, per day and per
// purpose, and its budget.
type AIConnectionUsage struct {
	ConnectionId       string                   `json:"connection_id"`
	Month              int                      `json:"month"` // 202610
	Totals             AIUsageTotals            `json:"totals"`
	MonthlyTokenBudget int64                    `json:"monthly_token_budget"`
	Days               []*AIUsage               `json:"days"`
	ByPurpose          map[string]AIUsageTotals `json:"by_purpose"`
}

// AIUsageDay gives the usage day of a time, in UTC (20261017).
func AIUsageDay(t time.Time) int {
	t = t.UTC()
	return t.Year()*10000 + int(t.Month())*100 + t.Day()
}

// AIUsageMonthStart gives the first usage day of the month of a time (20261001).
func AIUsageMonthStart(t time.Time) int {
	t = t.UTC()
	return t.Year()*10000 + int(t.Month())*100 + 1
}
