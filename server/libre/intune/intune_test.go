// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package intune

import (
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/libre/oauth/oidc/oidctest"
)

const (
	tenant   = "11111111-2222-3333-4444-555555555555"
	clientID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	objectID = "99999999-8888-7777-6666-555555555555"
)

type fakeApp struct {
	cfg     *model.Config
	users   []*model.User
	created []*model.User
}

func (f *fakeApp) Config() *model.Config { return f.cfg }

func (f *fakeApp) GetUserByAuth(authData *string, authService string) (*model.User, *model.AppError) {
	for _, u := range f.users {
		if u.AuthService == authService && u.AuthData != nil && *u.AuthData == *authData {
			return u, nil
		}
	}
	return nil, model.NewAppError("GetUserByAuth", app.MissingAuthAccountError, nil, "", http.StatusInternalServerError)
}

func (f *fakeApp) GetUserByEmail(email string) (*model.User, *model.AppError) {
	for _, u := range f.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, model.NewAppError("GetUserByEmail", app.MissingAccountError, nil, "", http.StatusNotFound)
}

func (f *fakeApp) GetUserByUsername(username string) (*model.User, *model.AppError) {
	for _, u := range f.users {
		if u.Username == username {
			return u, nil
		}
	}
	return nil, model.NewAppError("GetUserByUsername", "app.user.get_by_username.app_error", nil, "", http.StatusNotFound)
}

func (f *fakeApp) CreateUser(_ request.CTX, user *model.User) (*model.User, *model.AppError) {
	user.Id = model.NewId()
	f.users = append(f.users, user)
	f.created = append(f.created, user)
	return user, nil
}

func setup(t *testing.T, authService string) (*Intune, *fakeApp, *oidctest.Provider) {
	idp := oidctest.New()
	t.Cleanup(idp.Close)

	cfg := &model.Config{}
	cfg.SetDefaults()
	cfg.IntuneSettings.Enable = model.NewPointer(true)
	cfg.IntuneSettings.TenantId = model.NewPointer(tenant)
	cfg.IntuneSettings.ClientId = model.NewPointer(clientID)
	cfg.IntuneSettings.AuthService = model.NewPointer(authService)
	cfg.Office365Settings.Enable = model.NewPointer(true)
	cfg.SamlSettings.Enable = model.NewPointer(true)

	fa := &fakeApp{cfg: cfg}
	in := New(fa, nil)
	in.jwksURL = func(string) string { return idp.JWKSURL() }
	return in, fa, idp
}

func token(idp *oidctest.Provider, overrides jwt.MapClaims) string {
	claims := jwt.MapClaims{
		"iss":                "https://login.microsoftonline.com/" + tenant + "/v2.0",
		"aud":                clientID,
		"tid":                tenant,
		"oid":                objectID,
		"scp":                "login.mattermost",
		"preferred_username": "Jane.Doe@Contoso.com",
		"given_name":         "Jane",
		"family_name":        "Doe",
	}
	for k, v := range overrides {
		if v == nil {
			delete(claims, k)
		} else {
			claims[k] = v
		}
	}
	return idp.Sign(claims)
}

func TestIsConfigured(t *testing.T) {
	in, fa, _ := setup(t, model.ServiceOffice365)
	assert.True(t, in.IsConfigured())

	fa.cfg.Office365Settings.Enable = model.NewPointer(false)
	assert.False(t, in.IsConfigured())

	fa.cfg.IntuneSettings.AuthService = model.NewPointer(model.UserAuthServiceSaml)
	assert.True(t, in.IsConfigured())

	fa.cfg.IntuneSettings.TenantId = model.NewPointer("not-a-uuid")
	assert.False(t, in.IsConfigured())

	fa.cfg.IntuneSettings.Enable = model.NewPointer(false)
	assert.False(t, in.IsConfigured())
	_, appErr := in.Login(request.TestContext(t), "x")
	require.NotNil(t, appErr)
	assert.Equal(t, "ent.intune.login.not_configured.app_error", appErr.Id)
}

func TestLoginExistingUser(t *testing.T) {
	rctx := request.TestContext(t)
	for _, service := range []string{model.ServiceOffice365, model.UserAuthServiceSaml} {
		t.Run(service, func(t *testing.T) {
			in, fa, idp := setup(t, service)
			existing := &model.User{Id: model.NewId(), AuthService: service, AuthData: model.NewPointer(objectID), Email: "jane.doe@contoso.com"}
			fa.users = append(fa.users, existing)

			user, appErr := in.Login(rctx, token(idp, nil))
			require.Nil(t, appErr)
			assert.Equal(t, existing.Id, user.Id)

			// v1.0 access tokens
			user, appErr = in.Login(rctx, token(idp, jwt.MapClaims{"iss": "https://sts.windows.net/" + tenant + "/", "aud": "api://" + clientID}))
			require.Nil(t, appErr)
			assert.Equal(t, existing.Id, user.Id)
		})
	}
}

func TestLoginCreatesOffice365User(t *testing.T) {
	rctx := request.TestContext(t)
	in, fa, idp := setup(t, model.ServiceOffice365)
	fa.users = append(fa.users, &model.User{Id: model.NewId(), Username: "jane.doe", Email: "other@contoso.com"})

	user, appErr := in.Login(rctx, token(idp, nil))
	require.Nil(t, appErr)
	require.Len(t, fa.created, 1)
	assert.Equal(t, "jane.doe@contoso.com", user.Email)
	assert.Equal(t, "jane.doe0", user.Username)
	assert.Equal(t, "Jane", user.FirstName)
	assert.Equal(t, model.ServiceOffice365, user.AuthService)
	assert.Equal(t, objectID, *user.AuthData)
	assert.True(t, user.EmailVerified)

	t.Run("creation disabled", func(t *testing.T) {
		in, fa, idp := setup(t, model.ServiceOffice365)
		fa.cfg.TeamSettings.EnableUserCreation = model.NewPointer(false)
		_, appErr := in.Login(rctx, token(idp, nil))
		require.NotNil(t, appErr)
		assert.Equal(t, "ent.intune.login.account_not_found.app_error", appErr.Id)
	})

	t.Run("email used by another account", func(t *testing.T) {
		in, fa, idp := setup(t, model.ServiceOffice365)
		fa.users = append(fa.users, &model.User{Id: model.NewId(), Email: "jane.doe@contoso.com"})
		_, appErr := in.Login(rctx, token(idp, nil))
		require.NotNil(t, appErr)
		assert.Equal(t, "api.user.create_oauth_user.already_attached.app_error", appErr.Id)
		assert.Empty(t, fa.created)
	})
}

func TestLoginSamlRequiresExistingUser(t *testing.T) {
	in, fa, idp := setup(t, model.UserAuthServiceSaml)
	_, appErr := in.Login(request.TestContext(t), token(idp, nil))
	require.NotNil(t, appErr)
	assert.Equal(t, "ent.intune.login.account_not_found.app_error", appErr.Id)
	assert.Empty(t, fa.created)
}

func TestLoginRejectsInvalidTokens(t *testing.T) {
	rctx := request.TestContext(t)
	in, _, idp := setup(t, model.ServiceOffice365)

	for name, tc := range map[string]struct {
		overrides jwt.MapClaims
		errID     string
	}{
		"other tenant issuer": {jwt.MapClaims{"iss": "https://login.microsoftonline.com/00000000-0000-0000-0000-000000000000/v2.0"}, "ent.intune.validate_token.invalid_tenant_id.app_error"},
		"other tenant claim":  {jwt.MapClaims{"tid": "00000000-0000-0000-0000-000000000000"}, "ent.intune.validate_token.invalid_tenant_id.app_error"},
		"other audience":      {jwt.MapClaims{"aud": "00000003-0000-0000-c000-000000000000"}, "ent.intune.validate_token.invalid_token.app_error"},
		"wrong scope":         {jwt.MapClaims{"scp": "User.Read"}, "ent.intune.validate_token.invalid_token.app_error"},
		"app-only token":      {jwt.MapClaims{"scp": nil, "roles": []string{"x"}}, "ent.intune.validate_token.invalid_token.app_error"},
		"missing oid":         {jwt.MapClaims{"oid": nil}, "ent.intune.validate_token.missing_claims.app_error"},
		"expired":             {jwt.MapClaims{"exp": time.Now().Add(-time.Hour).Unix(), "iat": time.Now().Add(-2 * time.Hour).Unix()}, "ent.intune.validate_token.token_expired.app_error"},
		"scope among several": {jwt.MapClaims{"scp": "openid login.mattermost"}, ""},
	} {
		t.Run(name, func(t *testing.T) {
			_, appErr := in.Login(rctx, token(idp, tc.overrides))
			if tc.errID == "" {
				require.Nil(t, appErr)
				return
			}
			require.NotNil(t, appErr)
			assert.Equal(t, tc.errID, appErr.Id)
		})
	}

	_, appErr := in.Login(rctx, "garbage")
	require.NotNil(t, appErr)
	assert.Equal(t, "ent.intune.validate_token.invalid_token.app_error", appErr.Id)
}

func TestJWKSUnavailable(t *testing.T) {
	in, _, idp := setup(t, model.ServiceOffice365)
	tok := token(idp, nil)
	in.jwksURL = func(string) string { return idp.URL() + "/missing" }
	_, appErr := in.Login(request.TestContext(t), tok)
	require.NotNil(t, appErr)
	assert.Equal(t, "ent.intune.validate_token.jwks_init.app_error", appErr.Id)
}
