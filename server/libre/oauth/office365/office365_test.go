// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package oauthoffice365

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

const graphMe = `{"id":"3f2c1a5e-0000-4000-8000-000000000001","displayName":"Ada Lovelace","givenName":"Ada","surname":"Lovelace","mail":"Ada.Lovelace@Contoso.com","userPrincipalName":"ada@contoso.onmicrosoft.com"}`

func config(directoryID string) *model.Config {
	cfg := &model.Config{}
	cfg.SetDefaults()
	cfg.Office365Settings.Enable = model.NewPointer(true)
	cfg.Office365Settings.Id = model.NewPointer("app-id")
	cfg.Office365Settings.DirectoryId = model.NewPointer(directoryID)
	return cfg
}

func TestRegistered(t *testing.T) {
	assert.NotNil(t, einterfaces.GetOAuthProvider(model.ServiceOffice365))
}

func TestGetSSOSettings(t *testing.T) {
	rctx := request.TestContext(t)
	p := New(nil)

	settings, err := p.GetSSOSettings(rctx, config(""), model.ServiceOffice365)
	require.NoError(t, err)
	assert.Equal(t, model.Office365SettingsDefaultAuthEndpoint, *settings.AuthEndpoint)
	assert.Equal(t, model.Office365SettingsDefaultUserAPIEndpoint, *settings.UserAPIEndpoint)

	settings, err = p.GetSSOSettings(rctx, config("tenant-1"), model.ServiceOffice365)
	require.NoError(t, err)
	assert.Equal(t, "https://login.microsoftonline.com/tenant-1/oauth2/v2.0/authorize", *settings.AuthEndpoint)
	assert.Equal(t, "https://login.microsoftonline.com/tenant-1/oauth2/v2.0/token", *settings.TokenEndpoint)
	assert.Equal(t, "app-id", *settings.Id)

	cfg := config("tenant-1")
	cfg.Office365Settings.AuthEndpoint = model.NewPointer("https://proxy.example.com/authorize")
	settings, err = p.GetSSOSettings(rctx, cfg, model.ServiceOffice365)
	require.NoError(t, err)
	assert.Equal(t, "https://proxy.example.com/authorize", *settings.AuthEndpoint)
}

func TestGetUserFromJSON(t *testing.T) {
	rctx := request.TestContext(t)
	p := New(nil)

	user, err := p.GetUserFromJSON(rctx, strings.NewReader(graphMe), nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "3f2c1a5e-0000-4000-8000-000000000001", *user.AuthData)
	assert.Equal(t, model.ServiceOffice365, user.AuthService)
	assert.Equal(t, "ada.lovelace@contoso.com", user.Email)
	assert.Equal(t, "ada.lovelace", user.Username)
	assert.Equal(t, "Ada", user.FirstName)
	assert.Equal(t, "Lovelace", user.LastName)

	// Fallback to the UPN when there is no mailbox.
	user, err = p.GetUserFromJSON(rctx, strings.NewReader(`{"id":"x","userPrincipalName":"Bob@Contoso.com"}`), nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "bob@contoso.com", user.Email)

	_, err = p.GetUserFromJSON(rctx, strings.NewReader(`{"mail":"a@b.com"}`), nil, nil)
	require.Error(t, err)
	_, err = p.GetUserFromJSON(rctx, strings.NewReader(`{"id":"x"}`), nil, nil)
	require.Error(t, err)
}

func TestGetUserFromIdToken(t *testing.T) {
	rctx := request.TestContext(t)
	idp := oidctest.New()
	defer idp.Close()

	p := New(nil)
	p.jwksURL = func(string) string { return idp.JWKSURL() }
	_, err := p.GetSSOSettings(rctx, config("tenant-1"), model.ServiceOffice365)
	require.NoError(t, err)

	claims := func(tid string) jwt.MapClaims {
		return jwt.MapClaims{
			"iss": "https://login.microsoftonline.com/" + tid + "/v2.0", "aud": "app-id", "tid": tid,
			"sub": "pairwise", "oid": "3f2c1a5e-0000-4000-8000-000000000001", "preferred_username": "ada@contoso.com",
		}
	}

	tokenUser, err := p.GetUserFromIdToken(rctx, idp.Sign(claims("tenant-1")))
	require.NoError(t, err)
	assert.Equal(t, "3f2c1a5e-0000-4000-8000-000000000001", *tokenUser.AuthData)

	user, err := p.GetUserFromJSON(rctx, strings.NewReader(graphMe), tokenUser, nil)
	require.NoError(t, err)
	assert.Equal(t, *tokenUser.AuthData, *user.AuthData)

	_, err = p.GetUserFromJSON(rctx, strings.NewReader(`{"id":"other","mail":"x@y.com"}`), tokenUser, nil)
	require.Error(t, err)

	_, err = p.GetUserFromIdToken(rctx, idp.Sign(claims("tenant-2")))
	require.Error(t, err)

	c := claims("tenant-1")
	c["aud"] = "other-app"
	_, err = p.GetUserFromIdToken(rctx, idp.Sign(c))
	require.Error(t, err)
}
