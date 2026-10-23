// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package aimcp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/ai"
)

// Signing people in to an MCP server, as the MCP authorization spec (2025-06-18) says: the server's protected
// resource metadata (RFC 9728) names its authorisation server, whose metadata (RFC 8414, or OpenID Connect discovery)
// gives the endpoints; the person signs in there with the authorization code flow and PKCE (S256), the token is asked
// for this MCP server only (RFC 8707's resource), and, when no client was set up, the server registers itself
// (RFC 7591).

const maxMetadataBytes = 256 * 1024

// ProtectedResourceMetadata is an MCP server's OAuth metadata (RFC 9728).
type ProtectedResourceMetadata struct {
	Resource             string   `json:"resource"`
	AuthorizationServers []string `json:"authorization_servers"`
	ScopesSupported      []string `json:"scopes_supported,omitempty"`
}

// AuthServerMetadata is an authorisation server's metadata (RFC 8414).
type AuthServerMetadata struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	RegistrationEndpoint              string   `json:"registration_endpoint,omitempty"`
	RevocationEndpoint                string   `json:"revocation_endpoint,omitempty"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported,omitempty"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported,omitempty"`
	ScopesSupported                   []string `json:"scopes_supported,omitempty"`
}

// Token is a person's sign-in: what the authorisation server gave.
type Token struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
	// ExpiresAt is when the access token stops working, in milliseconds; 0 when the server didn't say.
	ExpiresAt int64 `json:"expires_at,omitempty"`
}

// Expired tells whether the access token has stopped working (or will within a minute).
func (t *Token) Expired(now time.Time) bool {
	return t.ExpiresAt != 0 && now.Add(time.Minute).UnixMilli() >= t.ExpiresAt
}

// TokenRefusedError is the authorisation server refusing a token request (a code or refresh token it no longer
// takes): the person has to sign in again. Other errors (the network) may pass.
type TokenRefusedError struct {
	Reason string
}

func (e *TokenRefusedError) Error() string {
	return "the token request was refused: " + e.Reason
}

// ClientCredentials are the OAuth client the server signs people in with.
type ClientCredentials struct {
	ClientID     string
	ClientSecret string
}

// CheckEndpoint checks an address an MCP server's metadata gave: https, or http on the MCP server's own host (a
// server on the local network); never with credentials or a fragment.
func CheckEndpoint(raw string, mcpURL *url.URL) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" {
		return nil, errors.New("the MCP server's metadata gives an invalid address")
	}
	switch u.Scheme {
	case "https":
	case "http":
		if mcpURL == nil || !strings.EqualFold(u.Host, mcpURL.Host) {
			return nil, errors.New("the MCP server's metadata gives an address without https")
		}
	default:
		return nil, errors.New("the MCP server's metadata gives an invalid address")
	}
	return u, nil
}

func getJSON(ctx context.Context, httpClient *http.Client, raw string, out any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set(headerProtocolVersion, ProtocolVersion)
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, fmt.Errorf("%s answered %d", raw, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxMetadataBytes+1))
	if err != nil {
		return resp.StatusCode, err
	}
	if len(data) > maxMetadataBytes {
		return resp.StatusCode, errors.New("the metadata is too large")
	}
	return resp.StatusCode, json.Unmarshal(data, out)
}

// wellKnown gives the well-known addresses of a document for a URL with a path: with the path after the
// well-known name first, then at the host's root.
func wellKnown(u *url.URL, name string) []string {
	base := url.URL{Scheme: u.Scheme, Host: u.Host}
	path := strings.TrimRight(u.EscapedPath(), "/")
	var out []string
	if path != "" {
		out = append(out, base.String()+"/.well-known/"+name+path)
	}
	return append(out, base.String()+"/.well-known/"+name)
}

// DiscoverProtectedResource reads an MCP server's protected resource metadata: at metadataURL when its 401 said, else
// at its well-known addresses.
func DiscoverProtectedResource(ctx context.Context, httpClient *http.Client, mcpURL *url.URL, metadataURL string) (*ProtectedResourceMetadata, error) {
	var candidates []string
	if metadataURL != "" {
		u, err := CheckEndpoint(metadataURL, mcpURL)
		if err != nil {
			return nil, err
		}
		candidates = []string{u.String()}
	} else {
		candidates = wellKnown(mcpURL, "oauth-protected-resource")
	}
	var lastErr error
	for _, candidate := range candidates {
		var meta ProtectedResourceMetadata
		if _, err := getJSON(ctx, httpClient, candidate, &meta); err != nil {
			lastErr = err
			continue
		}
		if len(meta.AuthorizationServers) == 0 {
			lastErr = errors.New("the MCP server names no authorisation server")
			continue
		}
		if meta.Resource != "" {
			// The metadata must be the MCP server's own: a token asked for another resource would go elsewhere.
			resource, err := url.Parse(meta.Resource)
			if err != nil || !strings.EqualFold(resource.Scheme, mcpURL.Scheme) || !strings.EqualFold(resource.Host, mcpURL.Host) {
				return nil, errors.New("the MCP server's OAuth metadata is for another server")
			}
		}
		return &meta, nil
	}
	return nil, fmt.Errorf("the MCP server's OAuth metadata can't be read: %w", lastErr)
}

// DiscoverAuthServer reads an authorisation server's metadata (RFC 8414, then OpenID Connect discovery), and checks
// that it names itself and has the endpoints and PKCE the sign-in needs.
func DiscoverAuthServer(ctx context.Context, httpClient *http.Client, issuer string, mcpURL *url.URL) (*AuthServerMetadata, error) {
	u, err := CheckEndpoint(issuer, mcpURL)
	if err != nil {
		return nil, err
	}
	candidates := wellKnown(u, "oauth-authorization-server")
	candidates = append(candidates, wellKnown(u, "openid-configuration")...)
	if path := strings.TrimRight(u.EscapedPath(), "/"); path != "" {
		candidates = append(candidates, (&url.URL{Scheme: u.Scheme, Host: u.Host}).String()+path+"/.well-known/openid-configuration")
	}
	var lastErr error
	for _, candidate := range candidates {
		var meta AuthServerMetadata
		if _, err := getJSON(ctx, httpClient, candidate, &meta); err != nil {
			lastErr = err
			continue
		}
		if strings.TrimRight(meta.Issuer, "/") != strings.TrimRight(issuer, "/") {
			lastErr = errors.New("the authorisation server's metadata names another issuer")
			continue
		}
		for _, endpoint := range []string{meta.AuthorizationEndpoint, meta.TokenEndpoint} {
			if _, err := CheckEndpoint(endpoint, mcpURL); err != nil {
				return nil, err
			}
		}
		for _, endpoint := range []string{meta.RegistrationEndpoint, meta.RevocationEndpoint} {
			if endpoint == "" {
				continue
			}
			if _, err := CheckEndpoint(endpoint, mcpURL); err != nil {
				return nil, err
			}
		}
		if len(meta.CodeChallengeMethodsSupported) > 0 && !slices.Contains(meta.CodeChallengeMethodsSupported, "S256") {
			return nil, errors.New("the authorisation server doesn't support PKCE with S256")
		}
		return &meta, nil
	}
	return nil, fmt.Errorf("the authorisation server's metadata can't be read: %w", lastErr)
}

// RegisterClient registers the server as a client of an authorisation server (RFC 7591), for the redirect URI.
func RegisterClient(ctx context.Context, httpClient *http.Client, endpoint, redirectURI, clientName, scope string) (*ClientCredentials, error) {
	body := map[string]any{
		"client_name":                clientName,
		"redirect_uris":              []string{redirectURI},
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "client_secret_post",
	}
	if scope != "" {
		body["scope"] = scope
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("the client registration failed: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxMetadataBytes))
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the client registration was refused (%d)", resp.StatusCode)
	}
	var out struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.ClientID == "" {
		return nil, errors.New("the client registration gave no client")
	}
	return &ClientCredentials{ClientID: out.ClientID, ClientSecret: out.ClientSecret}, nil
}

// NewPKCE gives a code verifier and its S256 challenge.
func NewPKCE() (verifier, challenge string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

// AuthorizeURL gives where to send the person to sign in.
func AuthorizeURL(meta *AuthServerMetadata, clientID, redirectURI, state, challenge, scope, resource string) (string, error) {
	u, err := url.Parse(meta.AuthorizationEndpoint)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	if scope != "" {
		q.Set("scope", scope)
	}
	if resource != "" {
		q.Set("resource", resource)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// tokenRequest asks the token endpoint for a token, sending the client's secret in the body (client_secret_post)
// unless the server only takes it as basic authentication.
func tokenRequest(ctx context.Context, httpClient *http.Client, meta *AuthServerMetadata, client ClientCredentials, form url.Values) (*Token, error) {
	basic := client.ClientSecret != "" && len(meta.TokenEndpointAuthMethodsSupported) > 0 &&
		!slices.Contains(meta.TokenEndpointAuthMethodsSupported, "client_secret_post") &&
		slices.Contains(meta.TokenEndpointAuthMethodsSupported, "client_secret_basic")
	form.Set("client_id", client.ClientID)
	if client.ClientSecret != "" && !basic {
		form.Set("client_secret", client.ClientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, meta.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if basic {
		req.SetBasicAuth(url.QueryEscape(client.ClientID), url.QueryEscape(client.ClientSecret))
	}
	secrets := []string{client.ClientSecret, form.Get("code"), form.Get("refresh_token"), form.Get("code_verifier")}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, errors.New(ai.SanitizeProviderErrorMessage("the token request failed: "+err.Error(), secrets...))
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxMetadataBytes))
	if resp.StatusCode != http.StatusOK {
		var failure struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &failure)
		reason := failure.Error
		if reason == "" {
			reason = http.StatusText(resp.StatusCode)
		}
		return nil, &TokenRefusedError{Reason: ai.SanitizeProviderErrorMessage(reason, secrets...)}
	}
	var out struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		RefreshToken string `json:"refresh_token"`
		Scope        string `json:"scope"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.AccessToken == "" {
		return nil, errors.New("the token request gave no token")
	}
	if out.TokenType != "" && !strings.EqualFold(out.TokenType, "Bearer") {
		return nil, errors.New("the token request gave a token that isn't a bearer token")
	}
	token := &Token{AccessToken: out.AccessToken, TokenType: "Bearer", RefreshToken: out.RefreshToken, Scope: out.Scope}
	if out.ExpiresIn > 0 {
		token.ExpiresAt = time.Now().Add(time.Duration(out.ExpiresIn) * time.Second).UnixMilli()
	}
	return token, nil
}

// ExchangeCode turns the code of a sign-in into a token.
func ExchangeCode(ctx context.Context, httpClient *http.Client, meta *AuthServerMetadata, client ClientCredentials, code, verifier, redirectURI, resource string) (*Token, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("code_verifier", verifier)
	form.Set("redirect_uri", redirectURI)
	if resource != "" {
		form.Set("resource", resource)
	}
	return tokenRequest(ctx, httpClient, meta, client, form)
}

// Refresh gives a new token for a refresh token; the new one keeps the old refresh token when it gives none.
func Refresh(ctx context.Context, httpClient *http.Client, meta *AuthServerMetadata, client ClientCredentials, refreshToken, resource string) (*Token, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	if resource != "" {
		form.Set("resource", resource)
	}
	token, err := tokenRequest(ctx, httpClient, meta, client, form)
	if err != nil {
		return nil, err
	}
	if token.RefreshToken == "" {
		token.RefreshToken = refreshToken
	}
	return token, nil
}

// Revoke asks the authorisation server to forget a token (RFC 7009), best effort.
func Revoke(ctx context.Context, httpClient *http.Client, meta *AuthServerMetadata, client ClientCredentials, token string) {
	if meta.RevocationEndpoint == "" || token == "" {
		return
	}
	form := url.Values{}
	form.Set("token", token)
	form.Set("client_id", client.ClientID)
	if client.ClientSecret != "" {
		form.Set("client_secret", client.ClientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, meta.RevocationEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if resp, err := httpClient.Do(req); err == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxErrorBodyBytes))
		resp.Body.Close()
	}
}

// CanonicalResource gives an MCP server's resource identifier for its tokens (RFC 8707): its metadata's, else its
// URL without query or fragment.
func CanonicalResource(mcpURL *url.URL, meta *ProtectedResourceMetadata) string {
	if meta != nil && meta.Resource != "" {
		return meta.Resource
	}
	u := url.URL{Scheme: strings.ToLower(mcpURL.Scheme), Host: strings.ToLower(mcpURL.Host), Path: mcpURL.Path}
	return u.String()
}
