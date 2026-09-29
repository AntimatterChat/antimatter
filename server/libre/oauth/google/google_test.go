// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package oauthgoogle

import (
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
	"github.com/mattermost/mattermost/server/v8/libre/oauth/oidc"
	"github.com/mattermost/mattermost/server/v8/libre/oauth/oidc/oidctest"
)

const peopleMe = `{
  "resourceName": "people/109876543210",
  "metadata": {"sources": [{"type": "PROFILE", "id": "109876543210"}]},
  "names": [{"metadata": {"primary": true}, "displayName": "Grace Hopper", "givenName": "Grace", "familyName": "Hopper"}],
  "emailAddresses": [
    {"metadata": {"primary": false, "verified": true}, "value": "grace@other.example.com"},
    {"metadata": {"primary": true, "verified": true}, "value": "Grace.Hopper@Example.com"}
  ]
}`

func config() *model.Config {
	cfg := &model.Config{}
	cfg.SetDefaults()
	cfg.GoogleSettings.Enable = model.NewPointer(true)
	cfg.GoogleSettings.Id = model.NewPointer("google-client")
	return cfg
}

func TestRegistered(t *testing.T) {
	assert.NotNil(t, einterfaces.GetOAuthProvider(model.ServiceGoogle))
}

func TestGetSSOSettings(t *testing.T) {
	cfg := config()
	cfg.GoogleSettings.UserAPIEndpoint = model.NewPointer("")
	settings, err := New(nil).GetSSOSettings(request.TestContext(t), cfg, model.ServiceGoogle)
	require.NoError(t, err)
	assert.Equal(t, model.GoogleSettingsDefaultUserAPIEndpoint, *settings.UserAPIEndpoint)
	assert.Equal(t, model.GoogleSettingsDefaultAuthEndpoint, *settings.AuthEndpoint)
	assert.Equal(t, "google-client", *settings.Id)
	assert.Equal(t, "", *cfg.GoogleSettings.UserAPIEndpoint)
}

func TestGetUserFromJSON(t *testing.T) {
	rctx := request.TestContext(t)
	p := New(nil)

	user, err := p.GetUserFromJSON(rctx, strings.NewReader(peopleMe), nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "109876543210", *user.AuthData)
	assert.Equal(t, model.ServiceGoogle, user.AuthService)
	assert.Equal(t, "grace.hopper@example.com", user.Email)
	assert.Equal(t, "grace.hopper", user.Username)
	assert.Equal(t, "Grace", user.FirstName)
	assert.Equal(t, "Hopper", user.LastName)

	_, err = p.GetUserFromJSON(rctx, strings.NewReader(`{"resourceName":"people/1","emailAddresses":[{"metadata":{"verified":false},"value":"x@y.com"}]}`), nil, nil)
	require.Error(t, err)
	_, err = p.GetUserFromJSON(rctx, strings.NewReader(`{"emailAddresses":[{"value":"x@y.com"}]}`), nil, nil)
	require.Error(t, err)
}

func TestGetUserFromIdToken(t *testing.T) {
	rctx := request.TestContext(t)
	idp := oidctest.New()
	defer idp.Close()

	p := New(nil)
	p.keys = oidc.NewKeySet(idp.JWKSURL(), nil)

	token := idp.Sign(jwt.MapClaims{"iss": oidc.GoogleIssuer, "aud": "google-client", "sub": "109876543210", "email": "grace.hopper@example.com", "email_verified": true})

	// The client id is learned from the settings.
	_, err := p.GetUserFromIdToken(rctx, token)
	require.Error(t, err)
	_, err = p.GetSSOSettings(rctx, config(), model.ServiceGoogle)
	require.NoError(t, err)

	tokenUser, err := p.GetUserFromIdToken(rctx, token)
	require.NoError(t, err)
	assert.Equal(t, "109876543210", *tokenUser.AuthData)

	_, err = p.GetUserFromJSON(rctx, strings.NewReader(peopleMe), tokenUser, nil)
	require.NoError(t, err)

	other := idp.Sign(jwt.MapClaims{"iss": oidc.GoogleIssuer, "aud": "google-client", "sub": "1"})
	otherUser, err := p.GetUserFromIdToken(rctx, other)
	require.NoError(t, err)
	_, err = p.GetUserFromJSON(rctx, strings.NewReader(peopleMe), otherUser, nil)
	require.Error(t, err)

	_, err = p.GetUserFromIdToken(rctx, idp.Sign(jwt.MapClaims{"iss": "https://evil.example.com", "aud": "google-client", "sub": "1"}))
	require.Error(t, err)
}
