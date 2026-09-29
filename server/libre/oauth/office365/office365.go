// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

// Package oauthoffice365 implements the Office 365 (Microsoft Entra ID) OAuth
// 2.0 SSO provider, which reads the user's profile from Microsoft Graph.
package oauthoffice365

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
	"github.com/mattermost/mattermost/server/v8/libre/oauth/oidc"
)

const requestTimeout = 15 * time.Second

func init() {
	einterfaces.RegisterOAuthProvider(model.ServiceOffice365, New(nil))
}

// Provider is the Office 365 OAuth 2.0 provider.
type Provider struct {
	keys    *oidc.KeySetCache
	jwksURL func(tenant string) string
	now     func() time.Time

	mu       sync.RWMutex
	clientID string
	tenant   string
}

var _ einterfaces.OAuthProvider = (*Provider)(nil)

// New creates a provider. client may be nil to use a default HTTP client.
func New(client oidc.HTTPClientFunc) *Provider {
	return &Provider{keys: oidc.NewKeySetCache(client), jwksURL: oidc.MicrosoftJWKSURL}
}

// graphUser is the Microsoft Graph /me response.
type graphUser struct {
	ID                string `json:"id"`
	DisplayName       string `json:"displayName"`
	GivenName         string `json:"givenName"`
	Surname           string `json:"surname"`
	Mail              string `json:"mail"`
	UserPrincipalName string `json:"userPrincipalName"`
}

func (p *Provider) GetUserFromJSON(rctx request.CTX, data io.Reader, tokenUser *model.User, _ *model.SSOSettings) (*model.User, error) {
	var gu graphUser
	if err := json.NewDecoder(io.LimitReader(data, 1<<20)).Decode(&gu); err != nil {
		return nil, fmt.Errorf("invalid Microsoft Graph user response: %w", err)
	}

	// The Graph user id is the Entra ID object id, which is also the "oid"
	// claim of tokens; it is the stable identifier used as AuthData.
	id := strings.TrimSpace(gu.ID)
	if id == "" {
		return nil, errors.New("the Microsoft user has no id")
	}

	tokenIdentity, hasToken := oidc.IdentityFromUser(tokenUser)
	if hasToken && tokenIdentity.ObjectID != "" && !strings.EqualFold(tokenIdentity.ObjectID, id) {
		return nil, errors.New("the Microsoft Graph profile does not belong to the authenticated user")
	}

	identity := oidc.Identity{
		Email:      strings.ToLower(strings.TrimSpace(gu.Mail)),
		GivenName:  gu.GivenName,
		FamilyName: gu.Surname,
		Name:       gu.DisplayName,
		UPN:        gu.UserPrincipalName,
	}
	if hasToken {
		identity = identity.Merge(tokenIdentity)
	}

	email := identity.BestEmail()
	if email == "" {
		return nil, errors.New("the Microsoft user has no email address")
	}
	first, last := identity.Names()

	return &model.User{
		Email:       email,
		FirstName:   first,
		LastName:    last,
		Username:    oidc.Username(rctx.Logger(), "", email, false),
		AuthData:    model.NewPointer(id),
		AuthService: model.ServiceOffice365,
	}, nil
}

func (p *Provider) GetSSOSettings(_ request.CTX, config *model.Config, _ string) (*model.SSOSettings, error) {
	settings := config.Office365Settings.SSOSettings()
	tenant := strings.TrimSpace(model.SafeDereference(config.Office365Settings.DirectoryId))
	segment := oidc.MicrosoftTenantOrCommon(tenant)

	// Use the tenant specific endpoints when a directory is configured, so
	// that only accounts of that tenant can sign in.
	endpoint := func(v *string, def, tenantURL string) *string {
		s := strings.TrimSpace(model.SafeDereference(v))
		if s == "" || (s == def && segment != "common") {
			return model.NewPointer(tenantURL)
		}
		return model.NewPointer(s)
	}
	base := oidc.MicrosoftLoginURL + "/" + segment + "/oauth2/v2.0/"
	settings.AuthEndpoint = endpoint(settings.AuthEndpoint, model.Office365SettingsDefaultAuthEndpoint, base+"authorize")
	settings.TokenEndpoint = endpoint(settings.TokenEndpoint, model.Office365SettingsDefaultTokenEndpoint, base+"token")
	settings.UserAPIEndpoint = endpoint(settings.UserAPIEndpoint, model.Office365SettingsDefaultUserAPIEndpoint, model.Office365SettingsDefaultUserAPIEndpoint)
	if strings.TrimSpace(model.SafeDereference(settings.Scope)) == "" {
		settings.Scope = model.NewPointer(model.Office365SettingsDefaultScope)
	}

	p.mu.Lock()
	p.clientID = model.SafeDereference(settings.Id)
	p.tenant = tenant
	p.mu.Unlock()

	return settings, nil
}

// GetUserFromIdToken verifies an ID token issued by the Microsoft identity
// platform (v2.0 endpoints).
func (p *Provider) GetUserFromIdToken(rctx request.CTX, idToken string) (*model.User, error) {
	p.mu.RLock()
	clientID, tenant := p.clientID, p.tenant
	p.mu.RUnlock()
	if clientID == "" {
		return nil, errors.New("the Office 365 application id is not configured")
	}

	ctx, cancel := context.WithTimeout(rctx.Context(), requestTimeout)
	defer cancel()
	claims, err := oidc.Verify(ctx, idToken, oidc.VerifyOptions{
		KeySet:          p.keys.Get(p.jwksURL(tenant)),
		Audiences:       []string{clientID},
		AuthorizedParty: clientID,
		IssuerFunc: func(iss string, c oidc.Claims) bool {
			tid := oidc.String(c, "tid")
			if tid == "" {
				return false
			}
			if !oidc.IsMicrosoftMultiTenant(tenant) && !strings.EqualFold(tid, tenant) {
				return false
			}
			return iss == oidc.MicrosoftIssuer(tid)
		},
		Now: p.now,
	})
	if err != nil {
		return nil, fmt.Errorf("invalid Microsoft id_token: %w", err)
	}

	identity := oidc.IdentityFromClaims(claims)
	if identity.ObjectID == "" {
		return nil, errors.New("the Microsoft id_token has no object id")
	}
	user := identity.ToUser(rctx.Logger(), identity.ObjectID, false)
	user.AuthService = model.ServiceOffice365
	return user, nil
}

func (p *Provider) IsSameUser(_ request.CTX, dbUser, oauthUser *model.User) bool {
	return oidc.SameUser(dbUser, oauthUser, nil)
}
