// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

// Package intune implements Microsoft Intune MAM sign-in: the mobile app
// authenticates with Microsoft Entra ID through MSAL and hands the server an
// access token for the "login.mattermost" scope of the configured app
// registration. The token is verified against the tenant's published signing
// keys and mapped to the Mattermost user whose AuthData is the Entra ID
// object id, for the configured authentication service (Office 365 or SAML).
package intune

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
	"github.com/mattermost/mattermost/server/v8/libre/oauth/oidc"
)

const (
	// Scope is the delegated permission the mobile app requests, exposed by
	// the app registration as api://<client id>/login.mattermost.
	Scope = "login.mattermost"

	requestTimeout = 15 * time.Second
)

func init() {
	app.RegisterIntuneInterface(func(a *app.App) einterfaces.IntuneInterface {
		return New(a, func() *http.Client { return a.HTTPService().MakeClient(false) })
	})
}

// AppAPI is the subset of the app used by the Intune login.
type AppAPI interface {
	Config() *model.Config
	GetUserByAuth(authData *string, authService string) (*model.User, *model.AppError)
	GetUserByEmail(email string) (*model.User, *model.AppError)
	GetUserByUsername(username string) (*model.User, *model.AppError)
	CreateUser(rctx request.CTX, user *model.User) (*model.User, *model.AppError)
}

// Intune implements einterfaces.IntuneInterface.
type Intune struct {
	app     AppAPI
	keys    *oidc.KeySetCache
	jwksURL func(tenant string) string
	now     func() time.Time
}

var _ einterfaces.IntuneInterface = (*Intune)(nil)

// New creates the Intune login service.
func New(a AppAPI, client oidc.HTTPClientFunc) *Intune {
	return &Intune{app: a, keys: oidc.NewKeySetCache(client), jwksURL: oidc.MicrosoftJWKSURL}
}

type settings struct {
	tenant      string
	clientID    string
	authService string
}

func (in *Intune) settings() (settings, bool) {
	cfg := in.app.Config()
	s := cfg.IntuneSettings
	if !model.SafeDereference(s.Enable) || s.IsValid() != nil {
		return settings{}, false
	}
	out := settings{
		tenant:      strings.ToLower(strings.TrimSpace(model.SafeDereference(s.TenantId))),
		clientID:    strings.ToLower(strings.TrimSpace(model.SafeDereference(s.ClientId))),
		authService: model.SafeDereference(s.AuthService),
	}
	switch out.authService {
	case model.ServiceOffice365:
		if !model.SafeDereference(cfg.Office365Settings.Enable) {
			return settings{}, false
		}
	case model.UserAuthServiceSaml:
		if !model.SafeDereference(cfg.SamlSettings.Enable) {
			return settings{}, false
		}
	default:
		return settings{}, false
	}
	return out, true
}

// IsConfigured reports whether Intune MAM is enabled with a valid configuration
// and its authentication service is enabled.
func (in *Intune) IsConfigured() bool {
	_, ok := in.settings()
	return ok
}

func appError(id string, status int, err error) *model.AppError {
	appErr := model.NewAppError("Intune.Login", id, nil, "", status)
	if err != nil {
		appErr = appErr.Wrap(err)
	}
	return appErr
}

// validate verifies the access token and returns its claims.
func (in *Intune) validate(rctx request.CTX, s settings, accessToken string) (oidc.Claims, *model.AppError) {
	ctx, cancel := context.WithTimeout(rctx.Context(), requestTimeout)
	defer cancel()

	// Access tokens are v2.0 tokens when the app registration sets
	// accessTokenAcceptedVersion to 2, v1.0 tokens otherwise; accept both.
	issuers := []string{
		oidc.MicrosoftIssuer(s.tenant),
		"https://sts.windows.net/" + s.tenant + "/",
	}
	claims, err := oidc.Verify(ctx, accessToken, oidc.VerifyOptions{
		KeySet:    in.keys.Get(in.jwksURL(s.tenant)),
		Audiences: []string{s.clientID, "api://" + s.clientID},
		IssuerFunc: func(iss string, _ oidc.Claims) bool {
			return slices.ContainsFunc(issuers, func(i string) bool { return strings.EqualFold(i, iss) })
		},
		Now: in.now,
	})
	switch {
	case err == nil:
	case errors.Is(err, oidc.ErrTokenExpired):
		return nil, appError("ent.intune.validate_token.token_expired.app_error", http.StatusUnauthorized, err)
	case errors.Is(err, oidc.ErrKeySetUnavailable):
		return nil, appError("ent.intune.validate_token.jwks_init.app_error", http.StatusInternalServerError, err)
	case errors.Is(err, oidc.ErrInvalidIssuer):
		return nil, appError("ent.intune.validate_token.invalid_tenant_id.app_error", http.StatusUnauthorized, err)
	default:
		return nil, appError("ent.intune.validate_token.invalid_token.app_error", http.StatusUnauthorized, err)
	}

	if tid := oidc.String(claims, "tid"); !strings.EqualFold(tid, s.tenant) {
		return nil, appError("ent.intune.validate_token.invalid_tenant_id.app_error", http.StatusUnauthorized, errors.New("unexpected tenant "+tid))
	}

	// Only delegated tokens issued for the Mattermost login scope are
	// accepted (this rejects app-only tokens, which have no "scp").
	if !slices.Contains(oidc.StringList(claims, "scp"), Scope) {
		return nil, appError("ent.intune.validate_token.invalid_token.app_error", http.StatusUnauthorized, errors.New("the access token was not issued for the "+Scope+" scope"))
	}

	if oidc.String(claims, "oid") == "" {
		return nil, appError("ent.intune.validate_token.missing_claims.app_error", http.StatusBadRequest, errors.New("the access token has no oid claim"))
	}
	return claims, nil
}

// Login authenticates a user with an MSAL access token.
func (in *Intune) Login(rctx request.CTX, accessToken string) (*model.User, *model.AppError) {
	s, ok := in.settings()
	if !ok {
		return nil, appError("ent.intune.login.not_configured.app_error", http.StatusBadRequest, nil)
	}

	claims, appErr := in.validate(rctx, s, accessToken)
	if appErr != nil {
		rctx.Logger().Warn("Intune access token validation failed", mlog.Err(appErr))
		return nil, appErr
	}

	identity := oidc.IdentityFromClaims(claims)
	oid := identity.ObjectID

	user, appErr := in.app.GetUserByAuth(model.NewPointer(oid), s.authService)
	if appErr == nil {
		return user, nil
	}
	if appErr.Id != app.MissingAuthAccountError {
		return nil, appErr
	}

	if s.authService != model.ServiceOffice365 {
		// SAML accounts are provisioned by SAML sign-in, whose Id attribute
		// must be the Entra ID object id.
		return nil, appError("ent.intune.login.account_not_found.app_error", http.StatusNotFound, errors.New("no "+s.authService+" user with this object id"))
	}
	return in.createOffice365User(rctx, identity)
}

func (in *Intune) createOffice365User(rctx request.CTX, identity oidc.Identity) (*model.User, *model.AppError) {
	cfg := in.app.Config()
	if !model.SafeDereference(cfg.TeamSettings.EnableUserCreation) {
		return nil, appError("ent.intune.login.account_not_found.app_error", http.StatusNotFound, errors.New("user creation is disabled"))
	}

	email := identity.BestEmail()
	if email == "" {
		return nil, appError("ent.intune.validate_token.missing_claims.app_error", http.StatusBadRequest, errors.New("the access token has no email, upn or preferred_username claim"))
	}

	if existing, appErr := in.app.GetUserByEmail(email); appErr == nil {
		auth := existing.AuthService
		if auth == "" {
			auth = model.UserAuthServiceEmail
		}
		return nil, model.NewAppError("Intune.Login", "api.user.create_oauth_user.already_attached.app_error", map[string]any{"Service": "Office 365", "Auth": auth}, "email="+email, http.StatusBadRequest)
	} else if appErr.StatusCode != http.StatusNotFound {
		return nil, appErr
	}

	first, last := identity.Names()
	username := oidc.Username(rctx.Logger(), identity.PreferredUsername, email, false)
	base := username
	for i := 0; ; i++ {
		if _, appErr := in.app.GetUserByUsername(username); appErr != nil {
			if appErr.StatusCode != http.StatusNotFound {
				return nil, appErr
			}
			break
		}
		username = base + strconv.Itoa(i)
	}

	user := &model.User{
		Email:         email,
		EmailVerified: true,
		Username:      username,
		FirstName:     first,
		LastName:      last,
		AuthService:   model.ServiceOffice365,
		AuthData:      model.NewPointer(identity.ObjectID),
	}
	created, appErr := in.app.CreateUser(rctx, user)
	if appErr != nil {
		return nil, appErr
	}
	rctx.Logger().Info("Created user from an Intune sign-in", mlog.String("user_id", created.Id))
	return created, nil
}
