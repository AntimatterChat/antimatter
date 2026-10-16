// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// DefaultDiscoveryTTL is how long a discovery document is cached.
const DefaultDiscoveryTTL = time.Hour

// MicrosoftTenantPlaceholder is the placeholder used by the Microsoft identity
// platform in the issuer of its multi-tenant ("common") discovery documents.
const MicrosoftTenantPlaceholder = "{tenantid}"

// DiscoveryDocument holds the subset of the OpenID Provider Metadata used by
// Mattermost.
type DiscoveryDocument struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
	JwksURI               string `json:"jwks_uri"`
}

// IssuerMatches reports whether iss is the issuer of this document. It
// supports the Microsoft "{tenantid}" template when tid is provided.
func (d *DiscoveryDocument) IssuerMatches(iss, tid string) bool {
	if d.Issuer == iss {
		return true
	}
	if tid != "" && strings.Contains(d.Issuer, MicrosoftTenantPlaceholder) {
		return strings.ReplaceAll(d.Issuer, MicrosoftTenantPlaceholder, tid) == iss
	}
	return false
}

type discoveryEntry struct {
	doc       *DiscoveryDocument
	fetchedAt time.Time
}

// DiscoveryCache fetches and caches OpenID Connect discovery documents.
type DiscoveryCache struct {
	client HTTPClientFunc
	ttl    time.Duration
	now    func() time.Time

	mu      sync.Mutex
	entries map[string]discoveryEntry
}

// NewDiscoveryCache creates an empty discovery cache.
func NewDiscoveryCache(client HTTPClientFunc) *DiscoveryCache {
	if client == nil {
		client = DefaultHTTPClient
	}
	return &DiscoveryCache{client: client, ttl: DefaultDiscoveryTTL, now: time.Now, entries: map[string]discoveryEntry{}}
}

// Get returns the discovery document at url, using the cache when fresh. When
// a refresh fails, a previously fetched document is returned instead.
func (c *DiscoveryCache) Get(ctx context.Context, url string) (*DiscoveryDocument, error) {
	c.mu.Lock()
	entry, ok := c.entries[url]
	c.mu.Unlock()
	if ok && c.now().Sub(entry.fetchedAt) < c.ttl {
		return entry.doc, nil
	}

	doc, err := FetchDiscovery(ctx, c.client(), url)
	if err != nil {
		if ok {
			return entry.doc, nil
		}
		return nil, err
	}

	c.mu.Lock()
	c.entries[url] = discoveryEntry{doc: doc, fetchedAt: c.now()}
	c.mu.Unlock()
	return doc, nil
}

// FetchDiscovery downloads and validates a discovery document.
func FetchDiscovery(ctx context.Context, client *http.Client, url string) (*DiscoveryDocument, error) {
	body, err := getDocument(ctx, client, url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch discovery document from %s: %w", url, err)
	}
	var doc DiscoveryDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("invalid discovery document at %s: %w", url, err)
	}
	if doc.Issuer == "" {
		return nil, errors.New("discovery document is missing the issuer")
	}
	if doc.AuthorizationEndpoint == "" || doc.TokenEndpoint == "" {
		return nil, errors.New("discovery document is missing the authorization or token endpoint")
	}
	return &doc, nil
}
