// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package oauthopenid

import (
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
	"github.com/mattermost/mattermost/server/v8/libre/oauth/oidc/oidctest"
)

func setup(t *testing.T) (*Provider, *oidctest.Provider, *model.Config, request.CTX) {
	t.Helper()
	idp := oidctest.New()
	t.Cleanup(idp.Close)

	cfg := &model.Config{}
	cfg.SetDefaults()
	cfg.OpenIdSettings.Enable = model.NewPointer(true)
	cfg.OpenIdSettings.Id = model.NewPointer("client-id")
	cfg.OpenIdSettings.Secret = model.NewPointer("secret")
	cfg.OpenIdSettings.DiscoveryEndpoint = model.NewPointer(idp.DiscoveryURL())

	return New(nil), idp, cfg, request.TestContext(t)
}

func TestRegistered(t *testing.T) {
	assert.NotNil(t, einterfaces.GetOAuthProvider(model.ServiceOpenid))
}

func TestGetSSOSettings(t *testing.T) {
	p, idp, cfg, rctx := setup(t)

	settings, err := p.GetSSOSettings(rctx, cfg, model.ServiceOpenid)
	require.NoError(t, err)
	assert.Equal(t, idp.URL()+"/authorize", *settings.AuthEndpoint)
	assert.Equal(t, idp.URL()+"/token", *settings.TokenEndpoint)
	assert.Equal(t, idp.URL()+"/userinfo", *settings.UserAPIEndpoint)
	assert.Equal(t, "client-id", *settings.Id)
	// The configuration itself is not modified.
	assert.Equal(t, "", *cfg.OpenIdSettings.AuthEndpoint)

	t.Run("explicit endpoints win", func(t *testing.T) {
		cfg.OpenIdSettings.UserAPIEndpoint = model.NewPointer("https://custom/userinfo")
		defer func() { cfg.OpenIdSettings.UserAPIEndpoint = model.NewPointer("") }()
		settings, err := p.GetSSOSettings(rctx, cfg, model.ServiceOpenid)
		require.NoError(t, err)
		assert.Equal(t, "https://custom/userinfo", *settings.UserAPIEndpoint)
	})

	t.Run("legacy defaults are replaced for converted services", func(t *testing.T) {
		cfg.Office365Settings.Scope = model.NewPointer("openid profile email")
		cfg.Office365Settings.DiscoveryEndpoint = model.NewPointer(idp.DiscoveryURL())
		settings, err := p.GetSSOSettings(rctx, cfg, model.ServiceOffice365)
		require.NoError(t, err)
		assert.Equal(t, idp.URL()+"/userinfo", *settings.UserAPIEndpoint)
		assert.Equal(t, idp.URL()+"/authorize", *settings.AuthEndpoint)
	})

	t.Run("discovery failure", func(t *testing.T) {
		cfg.OpenIdSettings.DiscoveryEndpoint = model.NewPointer(idp.URL() + "/missing")
		defer func() { cfg.OpenIdSettings.DiscoveryEndpoint = model.NewPointer(idp.DiscoveryURL()) }()
		_, err := p.GetSSOSettings(rctx, cfg, model.ServiceOpenid)
		require.Error(t, err)
	})

	t.Run("no discovery", func(t *testing.T) {
		cfg.OpenIdSettings.DiscoveryEndpoint = model.NewPointer("")
		defer func() { cfg.OpenIdSettings.DiscoveryEndpoint = model.NewPointer(idp.DiscoveryURL()) }()
		_, err := p.GetSSOSettings(rctx, cfg, model.ServiceOpenid)
		require.Error(t, err)
	})
}

func TestLoginFlow(t *testing.T) {
	p, idp, cfg, rctx := setup(t)
	settings, err := p.GetSSOSettings(rctx, cfg, model.ServiceOpenid)
	require.NoError(t, err)

	idToken := idp.Sign(jwt.MapClaims{
		"iss": idp.URL(), "aud": "client-id", "sub": "subject-1",
		"email": "Jane@Example.com", "given_name": "Jane", "family_name": "Doe",
		"preferred_username": "jdoe",
	})
	tokenUser, err := p.GetUserFromIdToken(rctx, idToken)
	require.NoError(t, err)
	require.NotNil(t, tokenUser)
	assert.Equal(t, "subject-1", *tokenUser.AuthData)

	user, err := p.GetUserFromJSON(rctx, strings.NewReader(`{"sub":"subject-1","name":"Jane Q Doe"}`), tokenUser, settings)
	require.NoError(t, err)
	assert.Equal(t, "subject-1", *user.AuthData)
	assert.Equal(t, "jane@example.com", user.Email)
	assert.Equal(t, "Jane", user.FirstName)
	assert.Equal(t, "Doe", user.LastName)
	assert.Equal(t, "jane", user.Username)
	assert.Empty(t, user.Props)

	settings.UsePreferredUsername = model.NewPointer(true)
	user, err = p.GetUserFromJSON(rctx, strings.NewReader(`{"sub":"subject-1"}`), tokenUser, settings)
	require.NoError(t, err)
	assert.Equal(t, "jdoe", user.Username)

	t.Run("userinfo subject mismatch", func(t *testing.T) {
		_, err := p.GetUserFromJSON(rctx, strings.NewReader(`{"sub":"attacker","email":"a@b.com"}`), tokenUser, settings)
		require.Error(t, err)
	})

	t.Run("without id token", func(t *testing.T) {
		user, err := p.GetUserFromJSON(rctx, strings.NewReader(`{"sub":"s2","email":"x@example.com","oid":"forged","tid":"forged"}`), nil, settings)
		require.NoError(t, err)
		// oid is only trusted from a verified token
		assert.Equal(t, "s2", *user.AuthData)
	})

	t.Run("missing email", func(t *testing.T) {
		_, err := p.GetUserFromJSON(rctx, strings.NewReader(`{"sub":"s3"}`), nil, settings)
		require.Error(t, err)
	})
}

func TestIdTokenRejections(t *testing.T) {
	p, idp, cfg, rctx := setup(t)
	_, err := p.GetSSOSettings(rctx, cfg, model.ServiceOpenid)
	require.NoError(t, err)

	for name, claims := range map[string]jwt.MapClaims{
		"wrong audience": {"iss": idp.URL(), "aud": "other", "sub": "s"},
		"unknown issuer": {"iss": "https://other.example.com", "aud": "client-id", "sub": "s"},
		"no subject":     {"iss": idp.URL(), "aud": "client-id"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := p.GetUserFromIdToken(rctx, idp.Sign(claims))
			require.Error(t, err)
		})
	}

	t.Run("malformed", func(t *testing.T) {
		_, err := p.GetUserFromIdToken(rctx, "not-a-jwt")
		require.Error(t, err)
	})
}

func TestMicrosoftIdentity(t *testing.T) {
	p, idp, cfg, rctx := setup(t)
	idp.Issuer = "https://login.example.com/{tenantid}/v2.0"
	cfg.Office365Settings.Enable = model.NewPointer(true)
	cfg.Office365Settings.Id = model.NewPointer("ms-client")
	cfg.Office365Settings.Scope = model.NewPointer("openid profile email")
	cfg.Office365Settings.DiscoveryEndpoint = model.NewPointer(idp.DiscoveryURL())
	cfg.Office365Settings.DirectoryId = model.NewPointer("tenant-a")

	settings, err := p.GetSSOSettings(rctx, cfg, model.ServiceOffice365)
	require.NoError(t, err)

	claims := func(tid string) jwt.MapClaims {
		return jwt.MapClaims{
			"iss": "https://login.example.com/" + tid + "/v2.0", "aud": "ms-client", "sub": "pairwise-sub",
			"oid": "object-id", "tid": tid, "preferred_username": "john@contoso.com", "name": "John Smith",
		}
	}

	tokenUser, err := p.GetUserFromIdToken(rctx, idp.Sign(claims("tenant-a")))
	require.NoError(t, err)
	assert.Equal(t, "object-id", *tokenUser.AuthData)

	// Tokens of other tenants are refused.
	_, err = p.GetUserFromIdToken(rctx, idp.Sign(claims("tenant-b")))
	require.Error(t, err)

	user, err := p.GetUserFromJSON(rctx, strings.NewReader(`{"sub":"pairwise-sub","name":"John Smith"}`), tokenUser, settings)
	require.NoError(t, err)
	assert.Equal(t, "object-id", *user.AuthData)
	assert.Equal(t, "john@contoso.com", user.Email)
	assert.Equal(t, "john", user.Username)

	// A user linked with the OpenID subject is recognised.
	user.AuthService = model.ServiceOffice365
	dbUser := &model.User{AuthService: model.ServiceOffice365, AuthData: model.NewPointer("pairwise-sub")}
	assert.True(t, p.IsSameUser(rctx, dbUser, user))
	dbUser.AuthData = model.NewPointer("someone-else")
	assert.False(t, p.IsSameUser(rctx, dbUser, user))
}

func TestManualEndpoints(t *testing.T) {
	p, idp, cfg, rctx := setup(t)
	cfg.OpenIdSettings.DiscoveryEndpoint = model.NewPointer("")
	cfg.OpenIdSettings.AuthEndpoint = model.NewPointer(idp.URL() + "/authorize")
	cfg.OpenIdSettings.TokenEndpoint = model.NewPointer(idp.URL() + "/token")
	cfg.OpenIdSettings.UserAPIEndpoint = model.NewPointer(idp.URL() + "/userinfo")

	settings, err := p.GetSSOSettings(rctx, cfg, model.ServiceOpenid)
	require.NoError(t, err)
	assert.Equal(t, idp.URL()+"/userinfo", *settings.UserAPIEndpoint)

	tokenUser, err := p.GetUserFromIdToken(rctx, idp.Sign(jwt.MapClaims{"iss": idp.URL(), "aud": "client-id", "sub": "s"}))
	require.NoError(t, err)
	assert.Nil(t, tokenUser)

	_, err = p.GetUserFromIdToken(rctx, idp.Sign(jwt.MapClaims{"iss": idp.URL(), "aud": "other", "sub": "s"}))
	require.Error(t, err)
}
