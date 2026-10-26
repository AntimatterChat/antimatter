// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

// emailProvider is an SSO provider giving one identity, whose address it may say it doesn't vouch for.
type emailProvider struct {
	email      string
	authData   string
	unverified bool
}

func (p *emailProvider) GetUserFromJSON(_ request.CTX, _ io.Reader, _ *model.User, _ *model.SSOSettings) (*model.User, error) {
	user := &model.User{
		Email:       p.email,
		Username:    "sso" + model.NewId()[:10],
		AuthData:    model.NewPointer(p.authData),
		AuthService: model.ServiceOffice365,
	}
	if p.unverified {
		einterfaces.MarkOAuthEmailUnverified(user)
	}
	return user, nil
}

func (p *emailProvider) GetSSOSettings(_ request.CTX, _ *model.Config, _ string) (*model.SSOSettings, error) {
	return &model.SSOSettings{}, nil
}

func (p *emailProvider) GetUserFromIdToken(_ request.CTX, _ string) (*model.User, error) {
	return nil, nil
}

func (p *emailProvider) IsSameUser(_ request.CTX, _, _ *model.User) bool {
	return true
}

// An address the identity provider doesn't vouch for (email_verified=false, or a Microsoft sign-in open to any
// tenant) creates no account, so it can't pass a community's allowed domains as a verified address; it doesn't take
// over an account with that address, doesn't replace an account's address, and isn't given to an account switching
// to single sign-on.
func TestCreateOAuthUserUnverifiedEmailCannotJoinDomainRestrictedTeam(t *testing.T) {
	th := Setup(t).InitBasic(t)
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.Office365Settings.Enable = true
		*cfg.Office365Settings.Scope = model.Office365SettingsDefaultScope
	})
	th.BasicTeam.AllowedDomains = "victim.example.com"
	_, appErr := th.App.UpdateTeam(th.BasicTeam)
	require.Nil(t, appErr)

	provider := &emailProvider{}
	previous := einterfaces.GetOAuthProvider(model.ServiceOffice365)
	einterfaces.RegisterOAuthProvider(model.ServiceOffice365, provider)
	defer einterfaces.RegisterOAuthProvider(model.ServiceOffice365, previous)

	// stored reads a user as the database has it: the store's cache doesn't follow UpdateAuthData.
	stored := func(t *testing.T, userID string) *model.User {
		t.Helper()
		th.App.Srv().Store().User().InvalidateProfileCacheForUser(userID)
		user, err := th.App.Srv().Store().User().Get(th.Context, userID)
		require.NoError(t, err)
		return user
	}

	signUp := func() (*model.User, bool, *model.AppError) {
		return th.App.CreateOAuthUser(th.Context, model.ServiceOffice365, strings.NewReader("{}"), "", "", nil)
	}

	t.Run("an unverified address creates no account", func(t *testing.T) {
		*provider = emailProvider{email: "ceo-" + model.NewId()[:8] + "@victim.example.com", authData: model.NewId(), unverified: true}
		_, created, appErr := signUp()
		require.NotNil(t, appErr)
		assert.Equal(t, "api.user.oauth.email_unverified.app_error", appErr.Id)
		assert.False(t, created)
		_, err := th.App.Srv().Store().User().GetByEmail(provider.email)
		assert.Error(t, err, "no account has the address")
	})

	t.Run("an unverified address doesn't take over an account by email", func(t *testing.T) {
		victim := th.CreateUser(t)
		_, err := th.App.Srv().Store().User().UpdateAuthData(victim.Id, model.ServiceOffice365, model.NewPointer(model.NewId()), "", false)
		require.NoError(t, err)
		*provider = emailProvider{email: victim.Email, authData: model.NewId(), unverified: true}

		_, _, appErr := signUp()
		require.NotNil(t, appErr)
		assert.Equal(t, "api.user.oauth.email_unverified.app_error", appErr.Id)
		assert.NotEqual(t, provider.authData, model.SafeDereference(stored(t, victim.Id).AuthData), "the account isn't linked to the other identity")
	})

	t.Run("an existing account still signs in and keeps its address", func(t *testing.T) {
		authData := model.NewId()
		*provider = emailProvider{email: "kept-" + model.NewId()[:8] + "@victim.example.com", authData: authData}
		user, created, appErr := signUp()
		require.Nil(t, appErr)
		require.True(t, created)
		assert.True(t, user.EmailVerified, "a verified address counts as verified")
		_, _, appErr = th.App.AddUserToTeam(th.Context, th.BasicTeam.Id, user.Id, "")
		require.Nil(t, appErr, "and passes the allowed domains")

		*provider = emailProvider{email: "other-" + model.NewId()[:8] + "@victim.example.com", authData: authData, unverified: true}
		again, created, appErr := signUp()
		require.Nil(t, appErr, "the account is found by its identity")
		assert.False(t, created)
		assert.Equal(t, user.Id, again.Id)

		require.Nil(t, th.App.UpdateOAuthUserAttrs(th.Context, strings.NewReader("{}"), again, provider, model.ServiceOffice365, nil))
		after := stored(t, user.Id)
		assert.Equal(t, user.Email, after.Email, "the unverified address doesn't replace the account's")
		assert.NotContains(t, after.Props, einterfaces.OAuthEmailUnverifiedProp)
	})

	t.Run("switching to single sign-on doesn't take an unverified address", func(t *testing.T) {
		user := th.CreateUser(t)
		*provider = emailProvider{email: "ceo-" + model.NewId()[:8] + "@victim.example.com", authData: model.NewId(), unverified: true}
		_, appErr := th.App.CompleteSwitchWithOAuth(th.Context, model.ServiceOffice365, strings.NewReader("{}"), user.Email, nil)
		require.NotNil(t, appErr)
		assert.Equal(t, "api.user.oauth.email_unverified.app_error", appErr.Id)
		after := stored(t, user.Id)
		assert.Equal(t, user.Email, after.Email)
		assert.Empty(t, after.AuthService)
	})
}
