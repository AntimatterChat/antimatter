// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

// Package oauthopenid implements the generic OpenID Connect SSO provider.
//
// The server uses this provider for the "openid" service and for any other
// SSO service (GitLab, Google, Office 365) whose scope contains "openid", so
// it resolves the endpoints of each service from its discovery document and
// verifies ID tokens against the issuer's published keys.
package oauthopenid

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
	"github.com/mattermost/mattermost/server/v8/libre/oauth/oidc"
)

const requestTimeout = 15 * time.Second

func init() {
	einterfaces.RegisterOAuthProvider(model.ServiceOpenid, New(nil))
}

// registration is what the provider learned about a configured service the
// last time its settings were resolved. It is used to pick the verification
// parameters of ID tokens, which do not say which service they belong to.
type registration struct {
	service  string
	doc      *oidc.DiscoveryDocument
	clientID string
	secret   string
	// tenant restricts Microsoft multi-tenant issuers to one tenant, if set.
	tenant string
}

// Provider is the OpenID Connect provider.
type Provider struct {
	discovery *oidc.DiscoveryCache
	keys      *oidc.KeySetCache
	altIDs    *oidc.AltIDCache
	now       func() time.Time

	mu            sync.RWMutex
	registrations map[string]*registration
}

var _ einterfaces.OAuthProvider = (*Provider)(nil)

// New creates a provider. client may be nil to use a default HTTP client.
func New(client oidc.HTTPClientFunc) *Provider {
	return &Provider{
		discovery:     oidc.NewDiscoveryCache(client),
		keys:          oidc.NewKeySetCache(client),
		altIDs:        oidc.NewAltIDCache(),
		registrations: map[string]*registration{},
	}
}

// legacyDefaults are the OAuth 2.0 endpoint defaults of the legacy providers.
// When a service has been switched to OpenID Connect they are not meaningful
// anymore and are replaced with the endpoints from the discovery document.
var legacyDefaults = map[string]bool{
	model.GoogleSettingsDefaultAuthEndpoint:       true,
	model.GoogleSettingsDefaultTokenEndpoint:      true,
	model.GoogleSettingsDefaultUserAPIEndpoint:    true,
	model.Office365SettingsDefaultAuthEndpoint:    true,
	model.Office365SettingsDefaultTokenEndpoint:   true,
	model.Office365SettingsDefaultUserAPIEndpoint: true,
}

func (p *Provider) GetSSOSettings(rctx request.CTX, config *model.Config, service string) (*model.SSOSettings, error) {
	sso := config.GetSSOService(service)
	if sso == nil {
		return nil, fmt.Errorf("unsupported SSO service %q", service)
	}
	settings := *sso

	discoveryURL := strings.TrimSpace(model.SafeDereference(sso.DiscoveryEndpoint))
	tenant := ""
	switch service {
	case model.ServiceGoogle:
		if discoveryURL == "" {
			discoveryURL = oidc.GoogleDiscoveryURL
		}
	case model.ServiceOffice365:
		tenant = strings.TrimSpace(model.SafeDereference(config.Office365Settings.DirectoryId))
		if discoveryURL == "" {
			discoveryURL = oidc.MicrosoftDiscoveryURL(tenant)
		}
		if oidc.IsMicrosoftMultiTenant(tenant) {
			tenant = ""
		}
	}

	endpoint := func(v *string) string {
		s := strings.TrimSpace(model.SafeDereference(v))
		if legacyDefaults[s] {
			return ""
		}
		return s
	}
	authEndpoint := endpoint(sso.AuthEndpoint)
	tokenEndpoint := endpoint(sso.TokenEndpoint)
	userAPIEndpoint := endpoint(sso.UserAPIEndpoint)

	reg := &registration{
		service:  service,
		clientID: model.SafeDereference(sso.Id),
		secret:   model.SafeDereference(sso.Secret),
		tenant:   tenant,
	}

	if discoveryURL == "" {
		if authEndpoint == "" || tokenEndpoint == "" || userAPIEndpoint == "" {
			return nil, errors.New("the OpenID Connect discovery endpoint is not configured")
		}
		// Manually configured endpoints: ID tokens can't be verified since
		// the issuer and its keys are unknown.
		p.register(reg)
		return &settings, nil
	}

	ctx, cancel := context.WithTimeout(rctx.Context(), requestTimeout)
	defer cancel()
	doc, err := p.discovery.Get(ctx, discoveryURL)
	if err != nil {
		return nil, err
	}
	reg.doc = doc

	if authEndpoint == "" {
		authEndpoint = doc.AuthorizationEndpoint
	}
	if tokenEndpoint == "" {
		tokenEndpoint = doc.TokenEndpoint
	}
	if userAPIEndpoint == "" {
		userAPIEndpoint = doc.UserinfoEndpoint
	}
	if userAPIEndpoint == "" {
		return nil, errors.New("the OpenID Connect provider does not publish a userinfo endpoint")
	}

	settings.DiscoveryEndpoint = model.NewPointer(discoveryURL)
	settings.AuthEndpoint = model.NewPointer(authEndpoint)
	settings.TokenEndpoint = model.NewPointer(tokenEndpoint)
	settings.UserAPIEndpoint = model.NewPointer(userAPIEndpoint)

	p.register(reg)
	return &settings, nil
}

func (p *Provider) register(reg *registration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.registrations[reg.service] = reg
}

// findRegistration returns the configured service whose issuer and client id
// match the (not yet verified) token claims.
func (p *Provider) findRegistration(claims oidc.Claims) *registration {
	iss := oidc.String(claims, "iss")
	tid := oidc.String(claims, "tid")
	aud, _ := claims.GetAudience()

	p.mu.RLock()
	defer p.mu.RUnlock()

	services := make([]string, 0, len(p.registrations))
	for s := range p.registrations {
		services = append(services, s)
	}
	slices.Sort(services)
	for _, s := range services {
		reg := p.registrations[s]
		if reg.doc == nil || reg.clientID == "" {
			continue
		}
		if reg.doc.IssuerMatches(iss, tid) && slices.Contains([]string(aud), reg.clientID) {
			return reg
		}
	}
	return nil
}

// hasManualRegistration reports whether a service configured without a
// discovery document uses the token's audience as its client id.
func (p *Provider) hasManualRegistration(claims oidc.Claims) bool {
	aud, _ := claims.GetAudience()
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, reg := range p.registrations {
		if reg.doc == nil && reg.clientID != "" && slices.Contains([]string(aud), reg.clientID) {
			return true
		}
	}
	return false
}

// GetUserFromIdToken verifies the ID token returned by the token endpoint and
// returns the identity it asserts.
func (p *Provider) GetUserFromIdToken(rctx request.CTX, idToken string) (*model.User, error) {
	unverified, err := oidc.UnverifiedClaims(idToken)
	if err != nil {
		return nil, fmt.Errorf("malformed id_token: %w", err)
	}
	reg := p.findRegistration(unverified)
	if reg == nil && p.hasManualRegistration(unverified) {
		// Without a discovery document the issuer and its keys are unknown:
		// ignore the ID token and rely on the userinfo response only.
		rctx.Logger().Debug("Ignoring the id_token of an OpenID Connect provider configured without a discovery endpoint")
		return nil, nil
	}
	if reg == nil {
		return nil, fmt.Errorf("id_token issuer %q does not match a configured OpenID Connect provider", oidc.String(unverified, "iss"))
	}

	ctx, cancel := context.WithTimeout(rctx.Context(), requestTimeout)
	defer cancel()

	opts := oidc.VerifyOptions{
		Audiences:       []string{reg.clientID},
		AuthorizedParty: reg.clientID,
		IssuerFunc: func(iss string, c oidc.Claims) bool {
			tid := oidc.String(c, "tid")
			if reg.tenant != "" && !strings.EqualFold(tid, reg.tenant) {
				return false
			}
			return reg.doc.IssuerMatches(iss, tid)
		},
		Now: p.now,
	}
	if reg.doc.JwksURI != "" {
		opts.KeySet = p.keys.Get(reg.doc.JwksURI)
	}
	if reg.secret != "" {
		opts.HMACSecret = []byte(reg.secret)
	}

	claims, err := oidc.Verify(ctx, idToken, opts)
	if err != nil {
		return nil, fmt.Errorf("invalid id_token: %w", err)
	}

	identity := oidc.IdentityFromClaims(claims)
	if identity.Subject == "" {
		return nil, errors.New("id_token has no subject")
	}
	return identity.ToUser(rctx.Logger(), authDataFor(identity), false), nil
}

// authDataFor returns the stable identifier stored as the user's AuthData.
// For Microsoft Entra ID it is the object id ("oid"), which is stable across
// applications of the tenant and matches the id used by the Office 365 OAuth
// provider, Microsoft Graph and Intune. Otherwise it is the subject.
func authDataFor(identity oidc.Identity) string {
	if identity.ObjectID != "" && identity.TenantID != "" {
		return identity.ObjectID
	}
	return identity.Subject
}

// GetUserFromJSON builds the user from the userinfo response, combined with
// the verified ID token claims when available.
func (p *Provider) GetUserFromJSON(rctx request.CTX, data io.Reader, tokenUser *model.User, settings *model.SSOSettings) (*model.User, error) {
	body, err := io.ReadAll(io.LimitReader(data, 1<<20))
	if err != nil {
		return nil, err
	}
	claims := oidc.Claims{}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&claims); err != nil {
		return nil, fmt.Errorf("invalid userinfo response: %w", err)
	}

	info := oidc.IdentityFromClaims(claims)
	if info.Subject == "" {
		// Non standard userinfo responses (e.g. Microsoft Graph /me) use "id".
		info.Subject = stringOrNumber(claims["id"])
	}

	identity := info
	if tokenIdentity, ok := oidc.IdentityFromUser(tokenUser); ok {
		if info.Subject != "" && tokenIdentity.Subject != "" && info.Subject != tokenIdentity.Subject && info.Subject != tokenIdentity.ObjectID {
			return nil, errors.New("the userinfo subject does not match the id_token subject")
		}
		// The object id and tenant are only trusted from the verified token.
		info.ObjectID, info.TenantID = "", ""
		identity = oidc.Identity{
			Subject:  tokenIdentity.Subject,
			ObjectID: tokenIdentity.ObjectID,
			TenantID: tokenIdentity.TenantID,
		}.Merge(info).Merge(tokenIdentity)
	} else {
		identity.ObjectID, identity.TenantID = "", ""
	}

	if identity.Subject == "" {
		return nil, errors.New("the OpenID Connect user has no subject")
	}

	authData := authDataFor(identity)
	email := identity.BestEmail()
	if email == "" {
		return nil, errors.New("the OpenID Connect user has no email address")
	}

	usePreferred := settings != nil && model.SafeDereference(settings.UsePreferredUsername)
	first, last := identity.Names()
	user := &model.User{
		Email:     email,
		FirstName: first,
		LastName:  last,
		Username:  oidc.Username(rctx.Logger(), identity.PreferredUsername, email, usePreferred),
		AuthData:  model.NewPointer(authData),
	}

	p.altIDs.Put(authData, identity.Subject)

	rctx.Logger().Debug("Resolved OpenID Connect user", mlog.String("auth_data", authData))
	return user, nil
}

func stringOrNumber(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case json.Number:
		return t.String()
	}
	return ""
}

func (p *Provider) IsSameUser(_ request.CTX, dbUser, oauthUser *model.User) bool {
	return oidc.SameUser(dbUser, oauthUser, p.altIDs)
}
