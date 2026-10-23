// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package aimcp

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAuth is an MCP server's protected resource metadata and its authorisation server, on one test server.
type fakeAuth struct {
	server     *httptest.Server
	resource   string
	issuer     string
	challenge  string
	registered bool
	refreshed  bool
	tokenForm  url.Values
	basicAuth  bool
}

func newFakeAuth(t *testing.T) *fakeAuth {
	f := &fakeAuth{}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/oauth-protected-resource/mcp", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"resource": f.resource, "authorization_servers": []string{f.issuer}})
	})
	mux.HandleFunc("/.well-known/oauth-authorization-server/auth", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                           f.issuer,
			"authorization_endpoint":           f.server.URL + "/auth/authorize",
			"token_endpoint":                   f.server.URL + "/auth/token",
			"registration_endpoint":            f.server.URL + "/auth/register",
			"code_challenge_methods_supported": []string{"S256"},
		})
	})
	mux.HandleFunc("/auth/register", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, []any{"https://chat.example.com/api/v4/ai/mcp/oauth/callback"}, body["redirect_uris"])
		f.registered = true
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"client_id": "registered-client", "client_secret": "registered-secret"})
	})
	mux.HandleFunc("/auth/token", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		f.tokenForm = r.PostForm
		_, _, f.basicAuth = r.BasicAuth()
		switch r.PostForm.Get("grant_type") {
		case "authorization_code":
			sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
			if base64.RawURLEncoding.EncodeToString(sum[:]) != f.challenge || r.PostForm.Get("code") != "the-code" {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid_grant"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "access-1", "token_type": "bearer", "refresh_token": "refresh-1", "expires_in": 3600})
		case "refresh_token":
			f.refreshed = true
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "access-2", "token_type": "Bearer", "expires_in": 60})
		}
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	f.resource = f.server.URL + "/mcp"
	f.issuer = f.server.URL + "/auth"
	return f
}

func TestOAuthSignIn(t *testing.T) {
	f := newFakeAuth(t)
	ctx := context.Background()
	mcpURL, _ := url.Parse(f.server.URL + "/mcp")
	httpClient := f.server.Client()

	resource, err := DiscoverProtectedResource(ctx, httpClient, mcpURL, "")
	require.NoError(t, err)
	assert.Equal(t, []string{f.issuer}, resource.AuthorizationServers)
	assert.Equal(t, f.resource, CanonicalResource(mcpURL, resource))

	meta, err := DiscoverAuthServer(ctx, httpClient, resource.AuthorizationServers[0], mcpURL)
	require.NoError(t, err)

	redirect := "https://chat.example.com/api/v4/ai/mcp/oauth/callback"
	client, err := RegisterClient(ctx, httpClient, meta.RegistrationEndpoint, redirect, "Antimatter", "")
	require.NoError(t, err)
	assert.True(t, f.registered)
	assert.Equal(t, "registered-client", client.ClientID)

	verifier, challenge, err := NewPKCE()
	require.NoError(t, err)
	f.challenge = challenge
	authorize, err := AuthorizeURL(meta, client.ClientID, redirect, "the-state", challenge, "tools", f.resource)
	require.NoError(t, err)
	u, _ := url.Parse(authorize)
	assert.Equal(t, "S256", u.Query().Get("code_challenge_method"))
	assert.Equal(t, challenge, u.Query().Get("code_challenge"))
	assert.Equal(t, "the-state", u.Query().Get("state"))
	assert.Equal(t, f.resource, u.Query().Get("resource"), "the token is asked for this MCP server only")

	_, err = ExchangeCode(ctx, httpClient, meta, *client, "the-code", "wrong-verifier", redirect, f.resource)
	require.Error(t, err, "a code without its verifier is refused")
	assert.NotContains(t, err.Error(), "registered-secret")

	token, err := ExchangeCode(ctx, httpClient, meta, *client, "the-code", verifier, redirect, f.resource)
	require.NoError(t, err)
	assert.Equal(t, "access-1", token.AccessToken)
	assert.Equal(t, "Bearer", token.TokenType)
	assert.False(t, token.Expired(time.Now()))
	assert.Equal(t, "registered-secret", f.tokenForm.Get("client_secret"))
	assert.False(t, f.basicAuth)

	refreshed, err := Refresh(ctx, httpClient, meta, *client, token.RefreshToken, f.resource)
	require.NoError(t, err)
	assert.True(t, f.refreshed)
	assert.Equal(t, "access-2", refreshed.AccessToken)
	assert.Equal(t, "refresh-1", refreshed.RefreshToken, "a refresh that gives no new refresh token keeps the old one")
	assert.True(t, refreshed.Expired(time.Now()), "a token about to expire counts as expired")
}

func TestOAuthMetadataForAnotherServerIsRefused(t *testing.T) {
	f := newFakeAuth(t)
	f.resource = "https://elsewhere.example.com/mcp"
	mcpURL, _ := url.Parse(f.server.URL + "/mcp")
	_, err := DiscoverProtectedResource(context.Background(), f.server.Client(), mcpURL, "")
	require.Error(t, err)
}

func TestOAuthIssuerMustMatch(t *testing.T) {
	f := newFakeAuth(t)
	mcpURL, _ := url.Parse(f.server.URL + "/mcp")
	issuer := f.issuer
	f.issuer = "https://evil.example.com"
	_, err := DiscoverAuthServer(context.Background(), f.server.Client(), issuer, mcpURL)
	require.Error(t, err)
}

func TestCheckEndpoint(t *testing.T) {
	mcpURL, _ := url.Parse("http://mcp.internal:8080/mcp")
	for raw, ok := range map[string]bool{
		"https://auth.example.com/token":    true,
		"http://mcp.internal:8080/token":    true,
		"http://auth.example.com/token":     false,
		"ftp://auth.example.com":            false,
		"https://user:pw@auth.example.com/": false,
		"https://auth.example.com/#x":       false,
		"/relative":                         false,
	} {
		_, err := CheckEndpoint(raw, mcpURL)
		assert.Equal(t, ok, err == nil, fmt.Sprintf("%s: %v", raw, err))
	}
}

func TestOAuthBasicAuthWhenTheServerOnlyTakesIt(t *testing.T) {
	f := newFakeAuth(t)
	meta := &AuthServerMetadata{TokenEndpoint: f.server.URL + "/auth/token", TokenEndpointAuthMethodsSupported: []string{"client_secret_basic"}}
	_, err := Refresh(context.Background(), f.server.Client(), meta, ClientCredentials{ClientID: "c", ClientSecret: "s3cret-value"}, "refresh", "")
	require.NoError(t, err)
	assert.True(t, f.basicAuth)
	assert.Empty(t, f.tokenForm.Get("client_secret"), "the secret isn't sent twice")
}
