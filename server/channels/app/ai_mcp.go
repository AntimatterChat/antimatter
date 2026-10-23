// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app/aimcp"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// MCP servers (Antimatter): sources of tools for agents. System admins (manage_ai_connections) manage the server-wide
// ones and choose the communities they're shared with; a community's admins (manage_team_ai_connections) its own, as
// for model connections. Their requests follow the model connections' network rules: a server-wide one is a system
// admin's address and may be on the local network, a community's may not reach internal addresses (unless
// ServiceSettings.AllowedUntrustedInternalConnections allows them); no request follows a redirect. The addresses an
// MCP server's own metadata gives (its sign-in) are never trusted that way, but for the server-wide one's own host.
//
// People sign in to them themselves (OAuth): a tool then runs with the person's own account there. Each sign-in is
// sealed with the AI encryption key, bound to the server and the person, and only ever opened for that person's own
// calls: no API gives it, to anyone. An admin may instead give a server a shared service account (its headers,
// sealed): only its tools that read are then offered.

const (
	aiMCPListTimeout  = 30 * time.Second
	aiMCPCallTimeout  = 2 * time.Minute
	aiMCPToolsTTL     = 5 * time.Minute
	aiMCPOAuthTimeout = 30 * time.Second
	// aiMCPOAuthStateSeconds bounds a sign-in: the person has this long to finish it.
	aiMCPOAuthStateSeconds = 10 * 60
	aiMCPOAuthStatePrefix  = "mcpoauth_"

	// AIMCPOAuthCallbackPath is where authorisation servers send people back to, under the site URL.
	AIMCPOAuthCallbackPath = "/api/v4/ai/mcp/oauth/callback"
)

var errAIMCPNotSignedIn = errors.New("not signed in to the MCP server")

// aiMCPState is the node's cache of MCP servers' tool lists, per server and sign-in ("" for a server without one, or
// a person's ID).
type aiMCPState struct {
	mu    sync.Mutex
	tools map[string]*aiMCPToolsEntry
}

type aiMCPToolsEntry struct {
	tools   []aimcp.Tool
	expires time.Time
}

func aiMCPToolsKey(serverID, userID string) string {
	return serverID + ":" + userID
}

func (s *aiMCPState) get(key string) ([]aimcp.Tool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.tools[key]
	if !ok || time.Now().After(entry.expires) {
		return nil, false
	}
	return entry.tools, true
}

func (s *aiMCPState) put(key string, tools []aimcp.Tool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tools == nil {
		s.tools = map[string]*aiMCPToolsEntry{}
	}
	s.tools[key] = &aiMCPToolsEntry{tools: tools, expires: time.Now().Add(aiMCPToolsTTL)}
}

// forget drops a server's lists (userID "") or one person's.
func (s *aiMCPState) forget(serverID, userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key := range s.tools {
		if userID == "" && strings.HasPrefix(key, serverID+":") || key == aiMCPToolsKey(serverID, userID) {
			delete(s.tools, key)
		}
	}
}

func (a *App) aiMCP() *aiMCPState {
	return &a.Srv().ai.mcp
}

// GetAIMCPServer gives an MCP server that isn't deleted.
func (a *App) GetAIMCPServer(rctx request.CTX, id string) (*model.AIMCPServer, *model.AppError) {
	s, err := a.Srv().Store().AIMCPServer().Get(rctx, id)
	if err != nil {
		return nil, aiMCPStoreError("GetAIMCPServer", err)
	}
	if s.DeleteAt != 0 {
		return nil, model.NewAppError("GetAIMCPServer", "app.ai.mcp.not_found.app_error", nil, "", http.StatusNotFound)
	}
	return s, nil
}

func aiMCPStoreError(where string, err error) *model.AppError {
	var appErr *model.AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	var nfErr *store.ErrNotFound
	if errors.As(err, &nfErr) {
		return model.NewAppError(where, "app.ai.mcp.not_found.app_error", nil, "", http.StatusNotFound).Wrap(err)
	}
	var conflict *store.ErrConflict
	if errors.As(err, &conflict) {
		return model.NewAppError(where, "app.ai.mcp.slug_taken.app_error", nil, "", http.StatusConflict).Wrap(err)
	}
	return model.NewAppError(where, "app.ai.mcp.save.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
}

// GetAIMCPServersForManager lists the MCP servers a session manages: those of a scope, or, with an empty scope,
// every one it may manage.
func (a *App) GetAIMCPServersForManager(rctx request.CTX, session model.Session, scope string) ([]*model.AIMCPServer, *model.AppError) {
	var scopes []string
	switch {
	case scope != "":
		if !a.SessionCanManageAIConnectionScope(session, scope) {
			return nil, model.NewAppError("GetAIMCPServersForManager", "api.context.permissions.app_error", nil, "", http.StatusForbidden)
		}
		scopes = []string{scope}
	default:
		if a.SessionHasPermissionTo(session, model.PermissionManageAIConnections) {
			list, err := a.Srv().Store().AIMCPServer().GetAll(rctx)
			if err != nil {
				return nil, aiMCPStoreError("GetAIMCPServersForManager", err)
			}
			return list, nil
		}
		teams, appErr := a.GetTeamsForUser(session.UserId)
		if appErr != nil {
			return nil, appErr
		}
		for _, team := range teams {
			if a.SessionCanManageAIConnectionScope(session, team.Id) {
				scopes = append(scopes, team.Id)
			}
		}
	}
	list, err := a.Srv().Store().AIMCPServer().GetForScopes(rctx, scopes)
	if err != nil {
		return nil, aiMCPStoreError("GetAIMCPServersForManager", err)
	}
	return list, nil
}

// sealAIMCPServerSecrets encrypts an MCP server's OAuth client secret and service account headers, bound to it.
func (a *App) sealAIMCPServerSecrets(s *model.AIMCPServer, clientSecret *string, headers *map[string]string) *model.AppError {
	if clientSecret == nil && headers == nil {
		return nil
	}
	keyring, appErr := a.Srv().aiKeyring()
	if appErr != nil {
		return appErr
	}
	if clientSecret != nil {
		s.OAuthClientSecretSealed = ""
		if secret := strings.TrimSpace(*clientSecret); secret != "" {
			sealed, err := keyring.Seal(secret, model.AIMCPAAD(s.Id, "", model.AIMCPFieldOAuthClientSecret))
			if err != nil {
				return model.NewAppError("sealAIMCPServerSecrets", "app.ai.encryption_key.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
			}
			s.OAuthClientSecretSealed = sealed
		}
	}
	if headers != nil {
		s.HeadersSealed = ""
		s.HeaderNames = model.AIHeaderNames(*headers)
		if len(*headers) > 0 {
			data, err := json.Marshal(*headers)
			if err != nil {
				return model.NewAppError("sealAIMCPServerSecrets", "app.ai.encryption_key.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
			}
			sealed, err := keyring.Seal(string(data), model.AIMCPAAD(s.Id, "", model.AIMCPFieldHeaders))
			if err != nil {
				return model.NewAppError("sealAIMCPServerSecrets", "app.ai.encryption_key.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
			}
			s.HeadersSealed = sealed
		}
	}
	return nil
}

// openAIMCPServerSecret decrypts one of an MCP server's own sealed fields, for a call only.
func (a *App) openAIMCPServerSecret(s *model.AIMCPServer, sealed, field string) (string, *model.AppError) {
	if sealed == "" {
		return "", nil
	}
	keyring, appErr := a.Srv().aiKeyring()
	if appErr != nil {
		return "", appErr
	}
	plain, err := keyring.Open(sealed, model.AIMCPAAD(s.Id, "", field))
	if err != nil {
		return "", model.NewAppError("openAIMCPServerSecret", "app.ai.mcp.secret_unreadable.app_error", nil, "server_id="+s.Id, http.StatusInternalServerError).Wrap(err)
	}
	return plain, nil
}

func (a *App) checkAIMCPServerReferences(s *model.AIMCPServer) *model.AppError {
	invalid := func(detail string) *model.AppError {
		return model.NewAppError("checkAIMCPServerReferences", "app.ai.mcp.references.app_error", nil, detail, http.StatusBadRequest)
	}
	if !s.IsServerScoped() {
		if team, err := a.Srv().Store().Team().Get(s.Scope); err != nil || team.DeleteAt != 0 {
			return invalid("scope=" + s.Scope)
		}
	}
	for _, teamID := range s.Sharing.TeamIDs {
		if team, err := a.Srv().Store().Team().Get(teamID); err != nil || team.DeleteAt != 0 {
			return invalid("team_id=" + teamID)
		}
	}
	return nil
}

// CreateAIMCPServer adds an MCP server. The caller checked that its creator may manage its scope.
func (a *App) CreateAIMCPServer(rctx request.CTX, req *model.AIMCPServerRequest, creatorID string) (*model.AIMCPServer, *model.AppError) {
	if appErr := req.Validate(); appErr != nil {
		return nil, appErr
	}
	s := &model.AIMCPServer{
		Id:            model.NewId(),
		Name:          req.Name,
		Slug:          strings.TrimSpace(req.Slug),
		Scope:         req.Scope,
		URL:           req.URL,
		Auth:          req.Auth,
		OAuthClientId: req.OAuthClientId,
		OAuthScopes:   req.OAuthScopes,
		Sharing:       req.Sharing,
		CreatorId:     creatorID,
		UpdaterId:     creatorID,
	}
	secret := req.OAuthClientSecret
	headers := req.Headers
	if headers == nil {
		headers = map[string]string{}
	}
	if appErr := a.sealAIMCPServerSecrets(s, &secret, &headers); appErr != nil {
		return nil, appErr
	}
	s.PreSave()
	if appErr := s.IsValid(); appErr != nil {
		return nil, appErr
	}
	if appErr := a.checkAIMCPServerReferences(s); appErr != nil {
		return nil, appErr
	}
	saved, err := a.Srv().Store().AIMCPServer().Save(rctx, s)
	if err != nil {
		return nil, aiMCPStoreError("CreateAIMCPServer", err)
	}
	return saved, nil
}

// PatchAIMCPServer changes an MCP server. A new address or sign-in kind is another account: everyone's sign-in to it
// is forgotten, and a client the server registered itself is registered again.
func (a *App) PatchAIMCPServer(rctx request.CTX, s *model.AIMCPServer, patch *model.AIMCPServerPatch, updaterID string) (*model.AIMCPServer, *model.AppError) {
	updated := *s
	updated.HeaderNames = slices.Clone(s.HeaderNames)
	if patch.Name != nil {
		updated.Name = *patch.Name
	}
	if patch.URL != nil {
		updated.URL = strings.TrimSpace(*patch.URL)
	}
	if patch.Auth != nil {
		updated.Auth = *patch.Auth
	}
	if patch.OAuthClientId != nil {
		updated.OAuthClientId = *patch.OAuthClientId
		updated.OAuthRegistered = false
	}
	if patch.OAuthScopes != nil {
		updated.OAuthScopes = *patch.OAuthScopes
	}
	if patch.Sharing != nil {
		updated.Sharing = *patch.Sharing
	}
	if patch.OAuthClientSecret != nil {
		if appErr := (&model.AIMCPServerRequest{OAuthClientSecret: *patch.OAuthClientSecret}).Validate(); appErr != nil {
			return nil, appErr
		}
	}
	if patch.Headers != nil {
		if appErr := model.ValidateAIExtraHeaders(*patch.Headers); appErr != nil {
			return nil, appErr
		}
	}
	anotherAccount := updated.URL != s.URL || updated.Auth != s.Auth
	if anotherAccount && updated.OAuthRegistered {
		updated.OAuthClientId = ""
		updated.OAuthClientSecretSealed = ""
		updated.OAuthRegistered = false
	}
	if appErr := a.sealAIMCPServerSecrets(&updated, patch.OAuthClientSecret, patch.Headers); appErr != nil {
		return nil, appErr
	}
	updated.NeedsAttention = ""
	updated.UpdaterId = updaterID
	updated.PreUpdate()
	if appErr := updated.IsValid(); appErr != nil {
		return nil, appErr
	}
	if appErr := a.checkAIMCPServerReferences(&updated); appErr != nil {
		return nil, appErr
	}
	saved, err := a.Srv().Store().AIMCPServer().Update(rctx, &updated)
	if err != nil {
		return nil, aiMCPStoreError("PatchAIMCPServer", err)
	}
	if anotherAccount {
		if err := a.Srv().Store().AIMCPServer().DeleteUserTokensForServer(rctx, saved.Id); err != nil {
			rctx.Logger().Warn("Couldn't forget the sign-ins to an MCP server whose address changed", mlog.String("server_id", saved.Id), mlog.Err(err))
		}
	}
	a.aiMCP().forget(saved.Id, "")
	a.publishAIToolsChanged("")
	return saved, nil
}

// DeleteAIMCPServer deletes an MCP server, its secrets and everyone's sign-in to it.
func (a *App) DeleteAIMCPServer(rctx request.CTX, s *model.AIMCPServer) *model.AppError {
	if err := a.Srv().Store().AIMCPServer().Delete(rctx, s.Id, model.GetMillis()); err != nil {
		return aiMCPStoreError("DeleteAIMCPServer", err)
	}
	a.aiMCP().forget(s.Id, "")
	a.publishAIToolsChanged("")
	return nil
}

// publishAIToolsChanged tells a person's clients (everyone's, for userID "") that the tools they may use changed: a
// sign-in, or an MCP server.
func (a *App) publishAIToolsChanged(userID string) {
	message := model.NewWebSocketEvent(model.WebsocketEventAIToolsChanged, "", "", userID, nil, "")
	message.GetBroadcast().ReliableClusterSend = true
	a.Publish(message)
}

// aiMCPAvailableIn tells whether agents may use an MCP server in a community ("" for direct and group messages).
func aiMCPAvailableIn(s *model.AIMCPServer, agent *model.AIAgent, teamID string) bool {
	if s.DeleteAt != 0 || s.NeedsAttention != "" {
		return false
	}
	if !s.IsServerScoped() && (agent == nil || agent.IsServerScoped() || s.Scope != agent.Scope) {
		// A community's server is for its own community's agents only: never another community's, nor a
		// server-wide agent's.
		return false
	}
	return s.SharedWithTeam(teamID)
}

// aiMCPServerUsableBy tells whether a person may use (and sign in to) an MCP server at all: a community's for its
// members, a server-wide one for the members of a community it's shared with.
func (a *App) aiMCPServerUsableBy(rctx request.CTX, s *model.AIMCPServer, user *model.User) bool {
	if user == nil || user.DeleteAt != 0 || user.IsBot {
		return false
	}
	if !s.IsServerScoped() {
		member, appErr := a.GetTeamMember(rctx, s.Scope, user.Id)
		return appErr == nil && member.DeleteAt == 0
	}
	if s.Sharing.AllTeams {
		return true
	}
	for _, teamID := range s.Sharing.TeamIDs {
		if member, appErr := a.GetTeamMember(rctx, teamID, user.Id); appErr == nil && member.DeleteAt == 0 {
			return true
		}
	}
	return false
}

type aiRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f aiRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func aiNoRedirects(*http.Request, []*http.Request) error {
	return http.ErrUseLastResponse
}

// aiMCPHTTPClient gives the client of an MCP server's own requests: as for its scope's model connections.
func (a *App) aiMCPHTTPClient(s *model.AIMCPServer) *http.Client {
	return &http.Client{Transport: a.HTTPService().MakeTransport(s.IsServerScoped()), CheckRedirect: aiNoRedirects}
}

// aiMCPOAuthHTTPClient gives the client of a sign-in's requests: to addresses an MCP server's metadata gave, which
// may not reach internal addresses, but for a server-wide server's own host (a system admin's address).
func (a *App) aiMCPOAuthHTTPClient(s *model.AIMCPServer) *http.Client {
	untrusted := a.HTTPService().MakeTransport(false)
	trusted := a.HTTPService().MakeTransport(true)
	own, _ := url.Parse(s.URL)
	return &http.Client{
		Transport: aiRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if s.IsServerScoped() && own != nil && strings.EqualFold(req.URL.Host, own.Host) {
				return trusted.RoundTrip(req)
			}
			return untrusted.RoundTrip(req)
		}),
		CheckRedirect: aiNoRedirects,
	}
}

// aiMCPSignIn is a person's sign-in to an MCP server, as sealed: the token, and what refreshing it needs.
type aiMCPSignIn struct {
	Token              aimcp.Token `json:"token"`
	ClientID           string      `json:"client_id"`
	ClientSecret       string      `json:"client_secret,omitempty"`
	Issuer             string      `json:"issuer"`
	TokenEndpoint      string      `json:"token_endpoint"`
	RevocationEndpoint string      `json:"revocation_endpoint,omitempty"`
	AuthMethods        []string    `json:"auth_methods,omitempty"`
	Resource           string      `json:"resource"`
}

func (si *aiMCPSignIn) meta() *aimcp.AuthServerMetadata {
	return &aimcp.AuthServerMetadata{Issuer: si.Issuer, TokenEndpoint: si.TokenEndpoint, RevocationEndpoint: si.RevocationEndpoint, TokenEndpointAuthMethodsSupported: si.AuthMethods}
}

func (si *aiMCPSignIn) client() aimcp.ClientCredentials {
	return aimcp.ClientCredentials{ClientID: si.ClientID, ClientSecret: si.ClientSecret}
}

// saveAIMCPSignIn seals and keeps a person's sign-in, bound to the server and the person.
func (a *App) saveAIMCPSignIn(rctx request.CTX, serverID, userID string, si *aiMCPSignIn) *model.AppError {
	keyring, appErr := a.Srv().aiKeyring()
	if appErr != nil {
		return appErr
	}
	data, err := json.Marshal(si)
	if err != nil {
		return model.NewAppError("saveAIMCPSignIn", "app.ai.encryption_key.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	sealed, err := keyring.Seal(string(data), model.AIMCPAAD(serverID, userID, model.AIMCPFieldUserToken))
	if err != nil {
		return model.NewAppError("saveAIMCPSignIn", "app.ai.encryption_key.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if err := a.Srv().Store().AIMCPServer().SaveUserToken(rctx, &model.AIMCPUserToken{ServerId: serverID, UserId: userID, TokenSealed: sealed, ExpiresAt: si.Token.ExpiresAt}); err != nil {
		return aiMCPStoreError("saveAIMCPSignIn", err)
	}
	return nil
}

// loadAIMCPSignIn opens a person's own sign-in to an MCP server: only ever theirs, read by the server and their ID.
func (a *App) loadAIMCPSignIn(rctx request.CTX, serverID, userID string) (*aiMCPSignIn, error) {
	token, err := a.Srv().Store().AIMCPServer().GetUserToken(rctx, serverID, userID)
	if err != nil {
		var nfErr *store.ErrNotFound
		if errors.As(err, &nfErr) {
			return nil, errAIMCPNotSignedIn
		}
		return nil, err
	}
	if token.UserId != userID || token.ServerId != serverID {
		return nil, errAIMCPNotSignedIn
	}
	keyring, appErr := a.Srv().aiKeyring()
	if appErr != nil {
		return nil, appErr
	}
	plain, err := keyring.Open(token.TokenSealed, model.AIMCPAAD(serverID, userID, model.AIMCPFieldUserToken))
	if err != nil {
		// Sealed for another person or server, or with another key: of no use.
		return nil, errAIMCPNotSignedIn
	}
	var si aiMCPSignIn
	if err := json.Unmarshal([]byte(plain), &si); err != nil || si.Token.AccessToken == "" {
		return nil, errAIMCPNotSignedIn
	}
	return &si, nil
}

// aiMCPHeadersFor gives the headers of a call for a person: none, the service account's, or the person's own bearer
// token (refreshed when it expired; a refresh the server refuses forgets the sign-in).
func (a *App) aiMCPHeadersFor(rctx request.CTX, s *model.AIMCPServer, userID string) (http.Header, error) {
	switch s.Auth {
	case model.AIMCPAuthNone:
		return http.Header{}, nil
	case model.AIMCPAuthServiceAccount:
		raw, appErr := a.openAIMCPServerSecret(s, s.HeadersSealed, model.AIMCPFieldHeaders)
		if appErr != nil {
			return nil, appErr
		}
		values := map[string]string{}
		if raw != "" {
			if err := json.Unmarshal([]byte(raw), &values); err != nil {
				return nil, err
			}
		}
		headers := http.Header{}
		for name, value := range values {
			headers.Set(name, value)
		}
		return headers, nil
	case model.AIMCPAuthOAuth:
		si, err := a.loadAIMCPSignIn(rctx, s.Id, userID)
		if err != nil {
			return nil, err
		}
		if si.Token.Expired(time.Now()) {
			if si.Token.RefreshToken == "" {
				return nil, errAIMCPNotSignedIn
			}
			ctx, cancel := context.WithTimeout(rctx.Context(), aiMCPOAuthTimeout)
			defer cancel()
			refreshed, err := aimcp.Refresh(ctx, a.aiMCPOAuthHTTPClient(s), si.meta(), si.client(), si.Token.RefreshToken, si.Resource)
			if err != nil {
				var refused *aimcp.TokenRefusedError
				if errors.As(err, &refused) {
					_ = a.Srv().Store().AIMCPServer().DeleteUserToken(rctx, s.Id, userID)
					a.aiMCP().forget(s.Id, userID)
					a.publishAIToolsChanged(userID)
					return nil, errAIMCPNotSignedIn
				}
				return nil, err
			}
			si.Token = *refreshed
			if appErr := a.saveAIMCPSignIn(rctx, s.Id, userID, si); appErr != nil {
				return nil, appErr
			}
		}
		return http.Header{"Authorization": {"Bearer " + si.Token.AccessToken}}, nil
	}
	return nil, errors.New("unknown sign-in")
}

// aiMCPToolsFor lists an MCP server's tools for a person (with their sign-in, or the server's own), cached a few
// minutes. A service account's list keeps only the tools that read.
func (a *App) aiMCPToolsFor(rctx request.CTX, s *model.AIMCPServer, userID string) ([]aimcp.Tool, error) {
	owner := ""
	if s.Auth == model.AIMCPAuthOAuth {
		owner = userID
	}
	key := aiMCPToolsKey(s.Id, owner)
	if tools, ok := a.aiMCP().get(key); ok {
		return tools, nil
	}
	headers, err := a.aiMCPHeadersFor(rctx, s, userID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(rctx.Context(), aiMCPListTimeout)
	defer cancel()
	client := aimcp.New(s.URL, a.aiMCPHTTPClient(s), headers)
	defer client.Close(ctx)
	tools, err := client.ListTools(ctx)
	if err != nil {
		var unauthorized *aimcp.UnauthorizedError
		if errors.As(err, &unauthorized) && s.Auth == model.AIMCPAuthOAuth {
			// The server no longer takes the person's sign-in.
			_ = a.Srv().Store().AIMCPServer().DeleteUserToken(rctx, s.Id, userID)
			return nil, errAIMCPNotSignedIn
		}
		return nil, err
	}
	if s.Auth == model.AIMCPAuthServiceAccount {
		tools = slices.DeleteFunc(tools, func(t aimcp.Tool) bool { return !t.ReadOnly() })
	}
	a.aiMCP().put(key, tools)
	return tools, nil
}

// callAIMCPTool runs an MCP server's tool for a person, with their own sign-in (or the server's).
func (a *App) callAIMCPTool(rctx request.CTX, s *model.AIMCPServer, userID, remoteName string, arguments json.RawMessage) (*aimcp.CallResult, error) {
	headers, err := a.aiMCPHeadersFor(rctx, s, userID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(rctx.Context(), aiMCPCallTimeout)
	defer cancel()
	client := aimcp.New(s.URL, a.aiMCPHTTPClient(s), headers)
	defer client.Close(ctx)
	return client.CallTool(ctx, remoteName, arguments)
}

// aiMCPOAuthState is a sign-in under way, sealed in the key-value store until the person comes back.
type aiMCPOAuthState struct {
	UserId             string   `json:"user_id"`
	ServerId           string   `json:"server_id"`
	Verifier           string   `json:"verifier"`
	RedirectURI        string   `json:"redirect_uri"`
	ClientID           string   `json:"client_id"`
	ClientSecret       string   `json:"client_secret,omitempty"`
	Issuer             string   `json:"issuer"`
	TokenEndpoint      string   `json:"token_endpoint"`
	RevocationEndpoint string   `json:"revocation_endpoint,omitempty"`
	AuthMethods        []string `json:"auth_methods,omitempty"`
	Resource           string   `json:"resource"`
}

func aiMCPOAuthError(where, detail string, status int) *model.AppError {
	return model.NewAppError(where, "app.ai.mcp.oauth.app_error", nil, detail, status)
}

// aiMCPOAuthClient gives the OAuth client of a sign-in: the one an admin set up, else one the server registers with
// the authorisation server (once, kept on the MCP server).
func (a *App) aiMCPOAuthClient(rctx request.CTX, s *model.AIMCPServer, meta *aimcp.AuthServerMetadata, redirectURI string) (*aimcp.ClientCredentials, *model.AppError) {
	const where = "aiMCPOAuthClient"
	if s.OAuthClientId != "" {
		secret, appErr := a.openAIMCPServerSecret(s, s.OAuthClientSecretSealed, model.AIMCPFieldOAuthClientSecret)
		if appErr != nil {
			return nil, appErr
		}
		return &aimcp.ClientCredentials{ClientID: s.OAuthClientId, ClientSecret: secret}, nil
	}
	if meta.RegistrationEndpoint == "" {
		return nil, aiMCPOAuthError(where, "no client and no registration endpoint", http.StatusConflict)
	}
	ctx, cancel := context.WithTimeout(rctx.Context(), aiMCPOAuthTimeout)
	defer cancel()
	creds, err := aimcp.RegisterClient(ctx, a.aiMCPOAuthHTTPClient(s), meta.RegistrationEndpoint, redirectURI, aimcp.ClientName, s.OAuthScopes)
	if err != nil {
		return nil, aiMCPOAuthError(where, err.Error(), http.StatusBadGateway)
	}
	updated := *s
	updated.OAuthClientId = creds.ClientID
	updated.OAuthRegistered = true
	secret := creds.ClientSecret
	if appErr := a.sealAIMCPServerSecrets(&updated, &secret, nil); appErr != nil {
		return nil, appErr
	}
	if _, err := a.Srv().Store().AIMCPServer().Update(rctx, &updated); err != nil {
		rctx.Logger().Warn("Couldn't keep the OAuth client registered for an MCP server", mlog.String("server_id", s.Id), mlog.Err(err))
	}
	return creds, nil
}

// StartAIMCPOAuth starts a person's sign-in to an MCP server: it finds the server's authorisation server, registers
// a client if none was set up, and gives where to send the person. The sign-in is bound to them: only they can finish
// it, once, within ten minutes.
func (a *App) StartAIMCPOAuth(rctx request.CTX, user *model.User, s *model.AIMCPServer) (*model.AIMCPOAuthConnect, *model.AppError) {
	const where = "StartAIMCPOAuth"
	if s.Auth != model.AIMCPAuthOAuth {
		return nil, aiMCPOAuthError(where, "the server has no per-person sign-in", http.StatusBadRequest)
	}
	if !a.aiMCPServerUsableBy(rctx, s, user) {
		return nil, model.NewAppError(where, "app.ai.mcp.not_found.app_error", nil, "", http.StatusNotFound)
	}
	siteURL := strings.TrimRight(a.GetSiteURL(), "/")
	if siteURL == "" {
		return nil, model.NewAppError(where, "app.ai.mcp.oauth_site_url.app_error", nil, "", http.StatusConflict)
	}
	redirectURI := siteURL + AIMCPOAuthCallbackPath
	mcpURL, err := url.Parse(s.URL)
	if err != nil {
		return nil, aiMCPOAuthError(where, "invalid address", http.StatusBadRequest)
	}
	ctx, cancel := context.WithTimeout(rctx.Context(), aiMCPOAuthTimeout)
	defer cancel()

	// The server's 401 says where its metadata is; else it's at the well-known address.
	metadataURL := ""
	probe := aimcp.New(s.URL, a.aiMCPHTTPClient(s), nil)
	var unauthorized *aimcp.UnauthorizedError
	if probeErr := probe.Initialize(ctx); errors.As(probeErr, &unauthorized) {
		metadataURL = unauthorized.ResourceMetadataURL
	}
	probe.Close(ctx)
	oauthClient := a.aiMCPOAuthHTTPClient(s)
	resource, err := aimcp.DiscoverProtectedResource(ctx, oauthClient, mcpURL, metadataURL)
	if err != nil {
		return nil, aiMCPOAuthError(where, err.Error(), http.StatusBadGateway)
	}
	meta, err := aimcp.DiscoverAuthServer(ctx, oauthClient, resource.AuthorizationServers[0], mcpURL)
	if err != nil {
		return nil, aiMCPOAuthError(where, err.Error(), http.StatusBadGateway)
	}
	creds, appErr := a.aiMCPOAuthClient(rctx, s, meta, redirectURI)
	if appErr != nil {
		return nil, appErr
	}
	verifier, challenge, err := aimcp.NewPKCE()
	if err != nil {
		return nil, aiMCPOAuthError(where, err.Error(), http.StatusInternalServerError)
	}
	state := model.NewId() + model.NewId()
	pending := aiMCPOAuthState{
		UserId: user.Id, ServerId: s.Id, Verifier: verifier, RedirectURI: redirectURI,
		ClientID: creds.ClientID, ClientSecret: creds.ClientSecret, Issuer: meta.Issuer, TokenEndpoint: meta.TokenEndpoint,
		RevocationEndpoint: meta.RevocationEndpoint, AuthMethods: meta.TokenEndpointAuthMethodsSupported,
		Resource: aimcp.CanonicalResource(mcpURL, resource),
	}
	keyring, appErr := a.Srv().aiKeyring()
	if appErr != nil {
		return nil, appErr
	}
	data, _ := json.Marshal(pending)
	sealed, err := keyring.Seal(string(data), model.AIMCPAAD(s.Id, user.Id, model.AIMCPFieldOAuthState+":"+state))
	if err != nil {
		return nil, model.NewAppError(where, "app.ai.encryption_key.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	// The key names the server and the person, so that the callback can open it bound to them.
	if appErr := a.SetPluginKeyWithExpiry(agentRuntimeKVNamespace, aiMCPOAuthStatePrefix+state, []byte(s.Id+":"+user.Id+":"+sealed), aiMCPOAuthStateSeconds); appErr != nil {
		return nil, appErr
	}
	scope := s.OAuthScopes
	authorizeURL, err := aimcp.AuthorizeURL(meta, creds.ClientID, redirectURI, state, challenge, scope, pending.Resource)
	if err != nil {
		return nil, aiMCPOAuthError(where, err.Error(), http.StatusBadGateway)
	}
	return &model.AIMCPOAuthConnect{AuthorizeURL: authorizeURL}, nil
}

// FinishAIMCPOAuth finishes a sign-in when the person comes back from the authorisation server: the state must be
// one this person started (a sign-in someone else started is refused, so nobody can tie their account there to
// another person), used once; the code is turned into a token kept for them.
func (a *App) FinishAIMCPOAuth(rctx request.CTX, userID, state, code, oauthError string) (*model.AIMCPServer, *model.AppError) {
	const where = "FinishAIMCPOAuth"
	if len(state) != 52 || !model.IsValidId(state[:26]) || !model.IsValidId(state[26:]) {
		return nil, aiMCPOAuthError(where, "invalid state", http.StatusBadRequest)
	}
	key := aiMCPOAuthStatePrefix + state
	raw, appErr := a.GetPluginKey(agentRuntimeKVNamespace, key)
	if appErr != nil || raw == nil {
		return nil, model.NewAppError(where, "app.ai.mcp.oauth_expired.app_error", nil, "", http.StatusBadRequest)
	}
	serverID, rest, _ := strings.Cut(string(raw), ":")
	ownerID, sealed, _ := strings.Cut(rest, ":")
	if ownerID != userID {
		// Someone else's sign-in: left for them, untouched.
		return nil, model.NewAppError(where, "app.ai.mcp.oauth_expired.app_error", nil, "another person's state", http.StatusForbidden)
	}
	if deleted, _ := a.CompareAndDeletePluginKey(rctx, agentRuntimeKVNamespace, key, raw); !deleted {
		return nil, model.NewAppError(where, "app.ai.mcp.oauth_expired.app_error", nil, "already used", http.StatusBadRequest)
	}
	keyring, appErr := a.Srv().aiKeyring()
	if appErr != nil {
		return nil, appErr
	}
	plain, err := keyring.Open(sealed, model.AIMCPAAD(serverID, userID, model.AIMCPFieldOAuthState+":"+state))
	if err != nil {
		return nil, model.NewAppError(where, "app.ai.mcp.oauth_expired.app_error", nil, "unreadable state", http.StatusBadRequest)
	}
	var pending aiMCPOAuthState
	if json.Unmarshal([]byte(plain), &pending) != nil || pending.UserId != userID || pending.ServerId != serverID {
		return nil, model.NewAppError(where, "app.ai.mcp.oauth_expired.app_error", nil, "invalid state", http.StatusBadRequest)
	}
	s, appErr := a.GetAIMCPServer(rctx, serverID)
	if appErr != nil {
		return nil, appErr
	}
	user, appErr := a.GetUser(rctx, userID)
	if appErr != nil || s.Auth != model.AIMCPAuthOAuth || !a.aiMCPServerUsableBy(rctx, s, user) {
		return nil, model.NewAppError(where, "app.ai.mcp.not_found.app_error", nil, "", http.StatusNotFound)
	}
	if oauthError != "" || code == "" {
		return s, aiMCPOAuthError(where, "the authorisation server said "+oauthError, http.StatusBadRequest)
	}
	ctx, cancel := context.WithTimeout(rctx.Context(), aiMCPOAuthTimeout)
	defer cancel()
	si := &aiMCPSignIn{
		ClientID: pending.ClientID, ClientSecret: pending.ClientSecret, Issuer: pending.Issuer,
		TokenEndpoint: pending.TokenEndpoint, RevocationEndpoint: pending.RevocationEndpoint,
		AuthMethods: pending.AuthMethods, Resource: pending.Resource,
	}
	token, err := aimcp.ExchangeCode(ctx, a.aiMCPOAuthHTTPClient(s), si.meta(), si.client(), code, pending.Verifier, pending.RedirectURI, pending.Resource)
	if err != nil {
		return s, aiMCPOAuthError(where, err.Error(), http.StatusBadGateway)
	}
	si.Token = *token
	if appErr := a.saveAIMCPSignIn(rctx, s.Id, userID, si); appErr != nil {
		return s, appErr
	}
	a.aiMCP().forget(s.Id, userID)
	a.publishAIToolsChanged(userID)
	return s, nil
}

// DisconnectAIMCPServer forgets a person's own sign-in to an MCP server (asking the server to forget the token too).
func (a *App) DisconnectAIMCPServer(rctx request.CTX, userID string, s *model.AIMCPServer) *model.AppError {
	if si, err := a.loadAIMCPSignIn(rctx, s.Id, userID); err == nil {
		ctx, cancel := context.WithTimeout(rctx.Context(), aiMCPOAuthTimeout)
		aimcp.Revoke(ctx, a.aiMCPOAuthHTTPClient(s), si.meta(), si.client(), si.Token.RefreshToken)
		aimcp.Revoke(ctx, a.aiMCPOAuthHTTPClient(s), si.meta(), si.client(), si.Token.AccessToken)
		cancel()
	}
	if err := a.Srv().Store().AIMCPServer().DeleteUserToken(rctx, s.Id, userID); err != nil {
		return aiMCPStoreError("DisconnectAIMCPServer", err)
	}
	a.aiMCP().forget(s.Id, userID)
	a.publishAIToolsChanged(userID)
	return nil
}

// aiMCPSignedIn tells whether a person signed in to an MCP server (whether their sign-in still works is only known
// when it's used).
func (a *App) aiMCPSignedIn(rctx request.CTX, serverID, userID string) bool {
	_, err := a.Srv().Store().AIMCPServer().GetUserToken(rctx, serverID, userID)
	return err == nil
}
