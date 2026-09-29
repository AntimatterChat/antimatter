// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

// Package outgoingoauth implements outgoing OAuth connections: OAuth 2.0
// client credentials used by the server to obtain access tokens that are sent
// along with outgoing webhooks and slash command requests whose URL matches
// one of the connection's audiences.
package outgoingoauth

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

const listPageSize = 100

func init() {
	app.RegisterOutgoingOAuthConnectionInterface(func(a *app.App) einterfaces.OutgoingOAuthConnectionInterface {
		return New(
			func() store.OutgoingOAuthConnectionStore { return a.Srv().Store().OutgoingOAuthConnection() },
			func() *http.Client { return a.HTTPService().MakeClient(false) },
		)
	})
}

// Service implements einterfaces.OutgoingOAuthConnectionInterface.
type Service struct {
	store  func() store.OutgoingOAuthConnectionStore
	tokens *tokenSource
}

var _ einterfaces.OutgoingOAuthConnectionInterface = (*Service)(nil)

// New creates the service. storeFn and clientFn are resolved lazily on each
// use.
func New(storeFn func() store.OutgoingOAuthConnectionStore, clientFn func() *http.Client) *Service {
	return &Service{store: storeFn, tokens: newTokenSource(clientFn)}
}

func (s *Service) GetConnection(rctx request.CTX, id string) (*model.OutgoingOAuthConnection, *model.AppError) {
	conn, err := s.store().GetConnection(rctx, id)
	if err != nil {
		var nfErr *store.ErrNotFound
		if errors.As(err, &nfErr) {
			return nil, model.NewAppError("GetConnection", "ent.outgoing_oauth_connections.get_connection.not_found.app_error", nil, "id="+id, http.StatusNotFound).Wrap(err)
		}
		return nil, model.NewAppError("GetConnection", "ent.outgoing_oauth_connections.get_connection.app_error", nil, "id="+id, http.StatusInternalServerError).Wrap(err)
	}
	return conn, nil
}

func (s *Service) GetConnections(rctx request.CTX, filters model.OutgoingOAuthConnectionGetConnectionsFilter) ([]*model.OutgoingOAuthConnection, *model.AppError) {
	filters.SetDefaults()
	conns, err := s.store().GetConnections(rctx, filters)
	if err != nil {
		return nil, model.NewAppError("GetConnections", "ent.outgoing_oauth_connections.get_connections.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return conns, nil
}

// allConnections pages through every stored connection.
func (s *Service) allConnections(rctx request.CTX) ([]*model.OutgoingOAuthConnection, error) {
	var all []*model.OutgoingOAuthConnection
	filters := model.OutgoingOAuthConnectionGetConnectionsFilter{Limit: listPageSize}
	for {
		page, err := s.store().GetConnections(rctx, filters)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(page) < filters.Limit {
			return all, nil
		}
		filters.OffsetId = page[len(page)-1].Id
	}
}

// checkAudiences validates the audiences of conn and makes sure no other
// connection already claims one of them.
func (s *Service) checkAudiences(rctx request.CTX, where, action string, conn *model.OutgoingOAuthConnection) *model.AppError {
	for _, audience := range conn.Audiences {
		if _, err := parseAudience(audience); err != nil {
			return model.NewAppError(where, "ent.outgoing_oauth_connections."+action+".audience_invalid", map[string]any{"Error": err.Error()}, "", http.StatusBadRequest)
		}
	}

	existing, err := s.allConnections(rctx)
	if err != nil {
		return model.NewAppError(where, "ent.outgoing_oauth_connections.get_connections.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	for _, other := range existing {
		if other == nil || other.Id == conn.Id {
			continue
		}
		for _, audience := range conn.Audiences {
			for _, otherAudience := range other.Audiences {
				if normalizeAudience(audience) == normalizeAudience(otherAudience) {
					return model.NewAppError(where, "ent.outgoing_oauth_connections."+action+".audience_duplicated", map[string]any{"Audience": audience}, "", http.StatusBadRequest)
				}
			}
		}
	}
	return nil
}

func storeError(where, id string, err error) *model.AppError {
	var appErr *model.AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	var invErr *store.ErrInvalidInput
	if errors.As(err, &invErr) {
		return model.NewAppError(where, id, map[string]any{"Error": err.Error()}, "", http.StatusBadRequest).Wrap(err)
	}
	return model.NewAppError(where, id, map[string]any{"Error": err.Error()}, "", http.StatusInternalServerError).Wrap(err)
}

func (s *Service) SaveConnection(rctx request.CTX, conn *model.OutgoingOAuthConnection) (*model.OutgoingOAuthConnection, *model.AppError) {
	if conn == nil {
		return nil, model.NewAppError("SaveConnection", "ent.outgoing_oauth_connections.save_connection.app_error", map[string]any{"Error": "missing connection"}, "", http.StatusBadRequest)
	}
	trimAudiences(conn)
	if appErr := s.checkAudiences(rctx, "SaveConnection", "save_connection", conn); appErr != nil {
		return nil, appErr
	}

	saved, err := s.store().SaveConnection(rctx, conn)
	if err != nil {
		return nil, storeError("SaveConnection", "ent.outgoing_oauth_connections.save_connection.app_error", err)
	}

	// The store does not persist the password grant credentials on insert,
	// so write them with an update right away.
	if saved.CredentialsUsername != nil || saved.CredentialsPassword != nil {
		if saved, err = s.store().UpdateConnection(rctx, saved); err != nil {
			return nil, storeError("SaveConnection", "ent.outgoing_oauth_connections.save_connection.app_error", err)
		}
	}
	return saved, nil
}

func (s *Service) UpdateConnection(rctx request.CTX, conn *model.OutgoingOAuthConnection) (*model.OutgoingOAuthConnection, *model.AppError) {
	if conn == nil {
		return nil, model.NewAppError("UpdateConnection", "ent.outgoing_oauth_connections.update_connection.app_error", map[string]any{"Error": "missing connection"}, "", http.StatusBadRequest)
	}
	trimAudiences(conn)
	if appErr := s.checkAudiences(rctx, "UpdateConnection", "update_connection", conn); appErr != nil {
		return nil, appErr
	}

	updated, err := s.store().UpdateConnection(rctx, conn)
	if err != nil {
		return nil, storeError("UpdateConnection", "ent.outgoing_oauth_connections.update_connection.app_error", err)
	}
	s.tokens.forget(conn.Id)
	return updated, nil
}

func (s *Service) DeleteConnection(rctx request.CTX, id string) *model.AppError {
	if err := s.store().DeleteConnection(rctx, id); err != nil {
		return model.NewAppError("DeleteConnection", "ent.outgoing_oauth_connections.delete_connection.app_error", nil, "id="+id, http.StatusInternalServerError).Wrap(err)
	}
	s.tokens.forget(id)
	return nil
}

func (s *Service) SanitizeConnection(conn *model.OutgoingOAuthConnection) {
	if conn != nil {
		conn.Sanitize()
	}
}

func (s *Service) SanitizeConnections(conns []*model.OutgoingOAuthConnection) {
	for _, conn := range conns {
		s.SanitizeConnection(conn)
	}
}

// GetConnectionForAudience returns the connection whose audience matches the
// URL, preferring the most specific audience. It returns nil (and no error)
// when no connection matches, in which case requests are sent unauthenticated.
func (s *Service) GetConnectionForAudience(rctx request.CTX, rawURL string) (*model.OutgoingOAuthConnection, *model.AppError) {
	target, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || target.Host == "" {
		return nil, nil
	}

	conns, err := s.allConnections(rctx)
	if err != nil {
		return nil, model.NewAppError("GetConnectionForAudience", "ent.outgoing_oauth_connections.get_connection_for_audience.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	var best *model.OutgoingOAuthConnection
	bestScore := -1
	for _, conn := range conns {
		if conn == nil {
			continue
		}
		for _, audience := range conn.Audiences {
			if score := audienceMatch(audience, target); score > bestScore {
				best, bestScore = conn, score
			}
		}
	}
	return best, nil
}

func trimAudiences(conn *model.OutgoingOAuthConnection) {
	audiences := make(model.StringArray, 0, len(conn.Audiences))
	for _, a := range conn.Audiences {
		if a = strings.TrimSpace(a); a != "" {
			audiences = append(audiences, a)
		}
	}
	conn.Audiences = audiences
}

func parseAudience(audience string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(audience))
	if err != nil {
		return nil, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("the audience must be an http or https URL")
	}
	if u.Host == "" {
		return nil, errors.New("the audience must have a host")
	}
	if u.User != nil {
		return nil, errors.New("the audience must not contain credentials")
	}
	return u, nil
}

func normalizeAudience(audience string) string {
	u, err := parseAudience(audience)
	if err != nil {
		return audience
	}
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host) + strings.TrimSuffix(u.EscapedPath(), "/") + "?" + u.RawQuery
}

// audienceMatch returns -1 when the audience doesn't cover the target URL, and
// otherwise a score that grows with the audience's specificity. An audience
// covers a URL with the same scheme and host whose path is the audience path
// or below it (on a path segment boundary).
func audienceMatch(audience string, target *url.URL) int {
	a, err := parseAudience(audience)
	if err != nil {
		return -1
	}
	if !strings.EqualFold(a.Scheme, target.Scheme) || !strings.EqualFold(hostWithPort(a), hostWithPort(target)) {
		return -1
	}

	aPath := strings.TrimSuffix(a.EscapedPath(), "/")
	tPath := target.EscapedPath()
	if aPath != "" && tPath != aPath && !strings.HasPrefix(tPath, aPath+"/") {
		return -1
	}
	score := len(aPath) * 2
	if a.RawQuery != "" {
		if a.RawQuery != target.RawQuery {
			return -1
		}
		score++
	}
	return score
}

func hostWithPort(u *url.URL) string {
	port := u.Port()
	if port == "" {
		switch strings.ToLower(u.Scheme) {
		case "http":
			port = "80"
		case "https":
			port = "443"
		}
	}
	return u.Hostname() + ":" + port
}

// RetrieveTokenForConnection obtains an access token for the connection,
// reusing a cached token until shortly before it expires.
//
// When validating an edited connection, the client doesn't know the stored
// secrets (they are sanitized), so missing secrets are taken from the stored
// connection with the same id.
func (s *Service) RetrieveTokenForConnection(rctx request.CTX, conn *model.OutgoingOAuthConnection) (*model.OutgoingOAuthConnectionToken, *model.AppError) {
	if conn == nil {
		return nil, authError(http.StatusBadRequest, errors.New("missing connection"))
	}

	c := *conn
	if c.Id != "" && (c.ClientSecret == "" || (c.GrantType == model.OutgoingOAuthConnectionGrantTypePassword && model.SafeDereference(c.CredentialsPassword) == "")) {
		if stored, err := s.store().GetConnection(rctx, c.Id); err == nil {
			if c.ClientSecret == "" {
				c.ClientSecret = stored.ClientSecret
			}
			if model.SafeDereference(c.CredentialsPassword) == "" {
				c.CredentialsPassword = stored.CredentialsPassword
			}
			if model.SafeDereference(c.CredentialsUsername) == "" {
				c.CredentialsUsername = stored.CredentialsUsername
			}
		}
	}

	if err := validateForToken(&c); err != nil {
		return nil, authError(http.StatusBadRequest, err)
	}

	token, err := s.tokens.token(rctx.Context(), &c)
	if err != nil {
		status := http.StatusBadRequest
		var te *tokenError
		if !errors.As(err, &te) {
			status = http.StatusInternalServerError
		}
		return nil, authError(status, err)
	}
	return token, nil
}

func validateForToken(c *model.OutgoingOAuthConnection) error {
	if c.ClientId == "" {
		return errors.New("the client id is required")
	}
	if c.ClientSecret == "" {
		return errors.New("the client secret is required")
	}
	if !model.IsValidHTTPURL(c.OAuthTokenURL) {
		return errors.New("the token URL is not a valid http or https URL")
	}
	if appErr := c.HasValidGrantType(); appErr != nil {
		if !c.GrantType.IsValid() {
			return errors.New("unsupported grant type")
		}
		return errors.New("the password grant requires a username and a password")
	}
	return nil
}

func authError(status int, err error) *model.AppError {
	return model.NewAppError("RetrieveTokenForConnection", "ent.outgoing_oauth_connections.authenticate.app_error", map[string]any{"Error": err.Error()}, "", status).Wrap(err)
}
