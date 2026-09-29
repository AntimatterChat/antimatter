// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

// Package oauthgoogle implements the Google OAuth 2.0 SSO provider, which
// reads the user's profile from the Google People API.
package oauthgoogle

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

var googleIssuers = []string{oidc.GoogleIssuer, "accounts.google.com"}

func init() {
	einterfaces.RegisterOAuthProvider(model.ServiceGoogle, New(nil))
}

// Provider is the Google OAuth 2.0 provider.
type Provider struct {
	keys *oidc.KeySet
	now  func() time.Time

	mu       sync.RWMutex
	clientID string
}

var _ einterfaces.OAuthProvider = (*Provider)(nil)

// New creates a provider. client may be nil to use a default HTTP client.
func New(client oidc.HTTPClientFunc) *Provider {
	return &Provider{
		keys: oidc.NewKeySet(oidc.GoogleJWKSURL, client),
	}
}

// People API response, see https://developers.google.com/people/api/rest/v1/people
type fieldMetadata struct {
	Primary  bool  `json:"primary"`
	Verified *bool `json:"verified"`
	Source   struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	} `json:"source"`
}

type googleUser struct {
	ResourceName string `json:"resourceName"`
	Names        []struct {
		Metadata    fieldMetadata `json:"metadata"`
		DisplayName string        `json:"displayName"`
		GivenName   string        `json:"givenName"`
		FamilyName  string        `json:"familyName"`
	} `json:"names"`
	EmailAddresses []struct {
		Metadata fieldMetadata `json:"metadata"`
		Value    string        `json:"value"`
	} `json:"emailAddresses"`
	Metadata struct {
		Sources []struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		} `json:"sources"`
	} `json:"metadata"`

	// OpenID Connect userinfo fields, in case the user API endpoint was
	// pointed at Google's userinfo endpoint.
	Sub   string `json:"sub"`
	Email string `json:"email"`
}

func (gu *googleUser) id() string {
	for _, s := range gu.Metadata.Sources {
		if s.Type == "PROFILE" && s.ID != "" {
			return s.ID
		}
	}
	if id, ok := strings.CutPrefix(gu.ResourceName, "people/"); ok && id != "" {
		return id
	}
	return gu.Sub
}

func (gu *googleUser) email() (string, error) {
	var fallback string
	for _, e := range gu.EmailAddresses {
		if e.Value == "" || (e.Metadata.Verified != nil && !*e.Metadata.Verified) {
			continue
		}
		if e.Metadata.Primary {
			return strings.ToLower(e.Value), nil
		}
		if fallback == "" {
			fallback = e.Value
		}
	}
	if fallback == "" {
		fallback = gu.Email
	}
	if fallback == "" {
		return "", errors.New("the Google account has no verified email address")
	}
	return strings.ToLower(fallback), nil
}

func (gu *googleUser) names() (first, last string) {
	for i, n := range gu.Names {
		if n.Metadata.Primary || i == len(gu.Names)-1 {
			if n.GivenName != "" || n.FamilyName != "" {
				return n.GivenName, n.FamilyName
			}
			return oidc.SplitName(n.DisplayName)
		}
	}
	return "", ""
}

func (p *Provider) GetUserFromJSON(rctx request.CTX, data io.Reader, tokenUser *model.User, _ *model.SSOSettings) (*model.User, error) {
	var gu googleUser
	if err := json.NewDecoder(io.LimitReader(data, 1<<20)).Decode(&gu); err != nil {
		return nil, fmt.Errorf("invalid Google user response: %w", err)
	}

	id := gu.id()
	if id == "" {
		return nil, errors.New("the Google user has no id")
	}

	tokenIdentity, hasToken := oidc.IdentityFromUser(tokenUser)
	if hasToken && tokenIdentity.Subject != "" && tokenIdentity.Subject != id {
		return nil, errors.New("the Google profile does not belong to the authenticated user")
	}

	email, err := gu.email()
	if err != nil {
		if !hasToken || tokenIdentity.Email == "" {
			return nil, err
		}
		email = tokenIdentity.Email
	}

	first, last := gu.names()
	if first == "" && last == "" && hasToken {
		first, last = tokenIdentity.Names()
	}

	user := &model.User{
		Email:       email,
		FirstName:   first,
		LastName:    last,
		Username:    oidc.Username(rctx.Logger(), "", email, false),
		AuthData:    model.NewPointer(id),
		AuthService: model.ServiceGoogle,
	}
	return user, nil
}

func (p *Provider) GetSSOSettings(_ request.CTX, config *model.Config, _ string) (*model.SSOSettings, error) {
	settings := config.GoogleSettings
	orDefault := func(v *string, def string) *string {
		if s := strings.TrimSpace(model.SafeDereference(v)); s != "" {
			return model.NewPointer(s)
		}
		return model.NewPointer(def)
	}
	settings.AuthEndpoint = orDefault(settings.AuthEndpoint, model.GoogleSettingsDefaultAuthEndpoint)
	settings.TokenEndpoint = orDefault(settings.TokenEndpoint, model.GoogleSettingsDefaultTokenEndpoint)
	settings.UserAPIEndpoint = orDefault(settings.UserAPIEndpoint, model.GoogleSettingsDefaultUserAPIEndpoint)
	settings.Scope = orDefault(settings.Scope, model.GoogleSettingsDefaultScope)

	p.mu.Lock()
	p.clientID = model.SafeDereference(settings.Id)
	p.mu.Unlock()

	return &settings, nil
}

// GetUserFromIdToken verifies an ID token issued by Google (returned by the
// token endpoint when the "openid" or "email" scopes are granted).
func (p *Provider) GetUserFromIdToken(rctx request.CTX, idToken string) (*model.User, error) {
	p.mu.RLock()
	clientID := p.clientID
	p.mu.RUnlock()
	if clientID == "" {
		return nil, errors.New("the Google client id is not configured")
	}

	ctx, cancel := context.WithTimeout(rctx.Context(), requestTimeout)
	defer cancel()
	claims, err := oidc.Verify(ctx, idToken, oidc.VerifyOptions{
		KeySet:          p.keys,
		Issuers:         googleIssuers,
		Audiences:       []string{clientID},
		AuthorizedParty: clientID,
		Now:             p.now,
	})
	if err != nil {
		return nil, fmt.Errorf("invalid Google id_token: %w", err)
	}

	identity := oidc.IdentityFromClaims(claims)
	if identity.Subject == "" {
		return nil, errors.New("the Google id_token has no subject")
	}
	if verified, present := oidc.Bool(claims, "email_verified"); present && !verified {
		identity.Email = ""
	}
	user := identity.ToUser(rctx.Logger(), identity.Subject, false)
	user.AuthService = model.ServiceGoogle
	return user, nil
}

func (p *Provider) IsSameUser(_ request.CTX, dbUser, oauthUser *model.User) bool {
	return oidc.SameUser(dbUser, oauthUser, nil)
}
