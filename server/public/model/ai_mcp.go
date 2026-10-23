// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package model

import (
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// MCP servers (Antimatter): tools from other services, over the Model Context Protocol, for agents. System admins add
// the server-wide ones (and choose the communities they're shared with); a community's admins add its own. People
// sign in to them themselves (OAuth): the tools then act with their own account there, never with an admin's. An admin
// may instead give a server one shared account (a service account): its tools are then only those that read, and it is
// shown as such. Secrets (the OAuth client's secret, the service account's headers, the people's tokens) are encrypted
// with the AI encryption key and never leave the server.

const (
	// The ways people use an MCP server's account.
	AIMCPAuthNone           = "none"
	AIMCPAuthOAuth          = "oauth"
	AIMCPAuthServiceAccount = "service_account"

	AIMCPServerNameMaxRunes  = 64
	AIMCPServerURLMaxLen     = 1024
	AIMCPOAuthClientIDMaxLen = 512
	AIMCPOAuthSecretMaxLen   = 4096
	AIMCPOAuthScopesMaxLen   = 1024

	// The sealed fields of an MCP server and of a person's sign-in to it.
	AIMCPFieldOAuthClientSecret = "oauth_client_secret"
	AIMCPFieldHeaders           = "headers"
	AIMCPFieldUserToken         = "user_token"
	AIMCPFieldOAuthState        = "oauth_state"
)

var aiMCPSlug = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

// IsValidAIMCPSlug tells whether a slug can name an MCP server's tools ("mcp.<slug>.<tool>").
func IsValidAIMCPSlug(slug string) bool {
	return aiMCPSlug.MatchString(slug)
}

// AIMCPServer is an MCP server as its managers see it: never a secret.
type AIMCPServer struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	// Slug names its tools: "mcp.<slug>.<tool>". Unique on the server; it never changes.
	Slug  string `json:"slug"`
	Scope string `json:"scope"`
	URL   string `json:"url"`
	Auth  string `json:"auth"`

	// OAuth: the client the server signs people in with. Left empty, the server registers itself with the MCP
	// server's authorisation server (dynamic client registration) the first time someone signs in.
	OAuthClientId           string `json:"oauth_client_id"`
	OAuthClientSecretSealed string `json:"-"`
	OAuthClientSecretSet    bool   `json:"oauth_client_secret_set"`
	OAuthScopes             string `json:"oauth_scopes"`
	// OAuthRegistered says the client was registered by the server itself (and may be registered again).
	OAuthRegistered bool `json:"oauth_registered"`

	// A service account's headers (e.g. Authorization), sealed; only their names are given.
	HeadersSealed string   `json:"-"`
	HeaderNames   []string `json:"header_names"`

	// Sharing says, for a server-wide MCP server, in which communities agents may use it.
	Sharing AIConnectionSharing `json:"sharing"`

	NeedsAttention string `json:"needs_attention"`
	CreatorId      string `json:"creator_id"`
	UpdaterId      string `json:"updater_id"`
	CreateAt       int64  `json:"create_at"`
	UpdateAt       int64  `json:"update_at"`
	DeleteAt       int64  `json:"delete_at"`
}

// AIMCPServerRequest adds an MCP server. The OAuth client's secret and the headers are in clear here only.
type AIMCPServerRequest struct {
	Name              string              `json:"name"`
	Slug              string              `json:"slug"`
	Scope             string              `json:"scope"`
	URL               string              `json:"url"`
	Auth              string              `json:"auth"`
	OAuthClientId     string              `json:"oauth_client_id"`
	OAuthClientSecret string              `json:"oauth_client_secret"`
	OAuthScopes       string              `json:"oauth_scopes"`
	Headers           map[string]string   `json:"headers"`
	Sharing           AIConnectionSharing `json:"sharing"`
}

// AIMCPServerPatch changes an MCP server; a field left out is kept. The slug and the scope never change.
// OAuthClientSecret replaces the secret ("" removes it); Headers replaces every header.
type AIMCPServerPatch struct {
	Name              *string              `json:"name,omitempty"`
	URL               *string              `json:"url,omitempty"`
	Auth              *string              `json:"auth,omitempty"`
	OAuthClientId     *string              `json:"oauth_client_id,omitempty"`
	OAuthClientSecret *string              `json:"oauth_client_secret,omitempty"`
	OAuthScopes       *string              `json:"oauth_scopes,omitempty"`
	Headers           *map[string]string   `json:"headers,omitempty"`
	Sharing           *AIConnectionSharing `json:"sharing,omitempty"`
}

// IsServerScoped tells whether the MCP server is server-wide.
func (s *AIMCPServer) IsServerScoped() bool {
	return s.Scope == AIConnectionScopeServer
}

// Namespace is the namespace of its tools.
func (s *AIMCPServer) Namespace() string {
	return AIToolMCPNamespacePrefix + s.Slug
}

// SharedWithTeam tells whether agents may use the MCP server in a community ("" for direct and group messages,
// which need a server-wide one shared with all communities), as for model connections.
func (s *AIMCPServer) SharedWithTeam(teamID string) bool {
	if !s.IsServerScoped() {
		return teamID != "" && teamID == s.Scope
	}
	if s.Sharing.AllTeams {
		return true
	}
	return teamID != "" && slices.Contains(s.Sharing.TeamIDs, teamID)
}

// AIMCPAAD gives the associated data a sealed field is bound to: an MCP server's own (owner "") or a person's.
func AIMCPAAD(serverID, ownerID, field string) string {
	return "aimcp:" + serverID + ":" + ownerID + ":" + field
}

// ValidAIMCPServerURL tells whether an MCP server may have this address: http(s), without credentials or fragment.
func ValidAIMCPServerURL(raw string) bool {
	if raw == "" || len(raw) > AIMCPServerURLMaxLen {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.Fragment != "" {
		return false
	}
	return true
}

// PreSave fills in a new MCP server's ID and times.
func (s *AIMCPServer) PreSave() {
	if s.Id == "" {
		s.Id = NewId()
	}
	if s.CreateAt == 0 {
		s.CreateAt = GetMillis()
	}
	s.UpdateAt = s.CreateAt
	s.normalize()
}

// PreUpdate sets the update time.
func (s *AIMCPServer) PreUpdate() {
	s.UpdateAt = GetMillis()
	s.normalize()
}

func (s *AIMCPServer) normalize() {
	s.Name = strings.TrimSpace(s.Name)
	s.URL = strings.TrimSpace(s.URL)
	s.OAuthClientId = strings.TrimSpace(s.OAuthClientId)
	s.OAuthScopes = strings.Join(strings.Fields(s.OAuthScopes), " ")
	s.OAuthClientSecretSet = s.OAuthClientSecretSealed != ""
	if s.HeaderNames == nil {
		s.HeaderNames = []string{}
	}
	if s.Sharing.TeamIDs == nil {
		s.Sharing.TeamIDs = []string{}
	}
	s.Sharing.TeamIDs = cleanAIAgentList(s.Sharing.TeamIDs)
	if !s.IsServerScoped() {
		s.Sharing = AIConnectionSharing{TeamIDs: []string{}}
	}
	if s.Auth != AIMCPAuthOAuth {
		s.OAuthClientId = ""
		s.OAuthClientSecretSealed = ""
		s.OAuthClientSecretSet = false
		s.OAuthScopes = ""
		s.OAuthRegistered = false
	}
	if s.Auth != AIMCPAuthServiceAccount {
		// Only a service account has headers: elsewhere a header with a key would be a shared account in disguise.
		s.HeadersSealed = ""
		s.HeaderNames = []string{}
	}
}

// IsValid checks an MCP server before it's saved.
func (s *AIMCPServer) IsValid() *AppError {
	invalid := func(id, detail string) *AppError {
		return NewAppError("AIMCPServer.IsValid", id, nil, detail, http.StatusBadRequest)
	}
	if !IsValidId(s.Id) {
		return invalid("model.ai_mcp_server.id.app_error", "")
	}
	if n := utf8.RuneCountInString(s.Name); n == 0 || n > AIMCPServerNameMaxRunes {
		return invalid("model.ai_mcp_server.name.app_error", "")
	}
	if !IsValidAIMCPSlug(s.Slug) {
		return invalid("model.ai_mcp_server.slug.app_error", "slug="+s.Slug)
	}
	if s.Scope != AIConnectionScopeServer && !IsValidId(s.Scope) {
		return invalid("model.ai_mcp_server.scope.app_error", "")
	}
	if !ValidAIMCPServerURL(s.URL) {
		return invalid("model.ai_mcp_server.url.app_error", "")
	}
	switch s.Auth {
	case AIMCPAuthNone, AIMCPAuthOAuth, AIMCPAuthServiceAccount:
	default:
		return invalid("model.ai_mcp_server.auth.app_error", "auth="+s.Auth)
	}
	if len(s.OAuthClientId) > AIMCPOAuthClientIDMaxLen || strings.ContainsAny(s.OAuthClientId, "\r\n ") || len(s.OAuthScopes) > AIMCPOAuthScopesMaxLen {
		return invalid("model.ai_mcp_server.oauth.app_error", "")
	}
	if s.Auth == AIMCPAuthServiceAccount && len(s.HeaderNames) == 0 {
		// A service account is its headers.
		return invalid("model.ai_mcp_server.headers.app_error", "service account without headers")
	}
	if len(s.HeaderNames) > AIConnectionMaxHeaders {
		return invalid("model.ai_mcp_server.headers.app_error", "too many")
	}
	if !validIDs(s.Sharing.TeamIDs) {
		return invalid("model.ai_mcp_server.sharing.app_error", "")
	}
	if utf8.RuneCountInString(s.NeedsAttention) > AINeedsAttentionMaxRunes {
		return invalid("model.ai_mcp_server.needs_attention.app_error", "")
	}
	if s.CreateAt == 0 || s.UpdateAt == 0 {
		return invalid("model.ai_mcp_server.times.app_error", "")
	}
	return nil
}

// Validate checks a request's secrets (the rest is checked once it's an AIMCPServer).
func (r *AIMCPServerRequest) Validate() *AppError {
	if len(r.OAuthClientSecret) > AIMCPOAuthSecretMaxLen || strings.ContainsAny(r.OAuthClientSecret, "\r\n") {
		return NewAppError("AIMCPServer.IsValid", "model.ai_mcp_server.oauth.app_error", nil, "", http.StatusBadRequest)
	}
	return ValidateAIExtraHeaders(r.Headers)
}

// Auditable gives what an audit record may keep of an MCP server: never a secret.
func (s *AIMCPServer) Auditable() map[string]any {
	return map[string]any{
		"id":                      s.Id,
		"name":                    s.Name,
		"slug":                    s.Slug,
		"scope":                   s.Scope,
		"url":                     s.URL,
		"auth":                    s.Auth,
		"oauth_client_id":         s.OAuthClientId,
		"oauth_client_secret_set": s.OAuthClientSecretSet,
		"oauth_scopes":            s.OAuthScopes,
		"header_names":            s.HeaderNames,
		"sharing":                 s.Sharing,
		"delete_at":               s.DeleteAt,
	}
}

// AIMCPUserToken is a person's sign-in to an MCP server: their tokens, sealed (bound to the server and the person).
// It never leaves the server: no API gives it, not even to admins.
type AIMCPUserToken struct {
	ServerId    string `json:"-"`
	UserId      string `json:"-"`
	TokenSealed string `json:"-"`
	ExpiresAt   int64  `json:"-"`
	CreateAt    int64  `json:"-"`
	UpdateAt    int64  `json:"-"`
}

// AIMCPOAuthConnect is where to send a person to sign in to an MCP server.
type AIMCPOAuthConnect struct {
	AuthorizeURL string `json:"authorize_url"`
}
