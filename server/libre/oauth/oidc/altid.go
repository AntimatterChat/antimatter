// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package oidc

import (
	"strings"
	"sync"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

const (
	altIDTTL        = 15 * time.Minute
	altIDMaxEntries = 10000
)

// AltIDCache remembers, for a short time, the alternative identifiers that a
// provider observed for a verified identity during a login (for example the
// OpenID "sub" of a Microsoft user whose AuthData is the object id). It lets
// IsSameUser recognise accounts that were linked with the other identifier.
type AltIDCache struct {
	mu      sync.Mutex
	entries map[string]altIDEntry
	now     func() time.Time
}

type altIDEntry struct {
	ids     []string
	expires time.Time
}

// NewAltIDCache creates an empty cache.
func NewAltIDCache() *AltIDCache {
	return &AltIDCache{entries: map[string]altIDEntry{}, now: time.Now}
}

// Put records alternative identifiers for authData.
func (c *AltIDCache) Put(authData string, ids ...string) {
	var filtered []string
	for _, id := range ids {
		if id != "" && id != authData {
			filtered = append(filtered, id)
		}
	}
	if authData == "" || len(filtered) == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if len(c.entries) >= altIDMaxEntries {
		for k, e := range c.entries {
			if now.After(e.expires) {
				delete(c.entries, k)
			}
		}
		if len(c.entries) >= altIDMaxEntries {
			c.entries = map[string]altIDEntry{}
		}
	}
	c.entries[authData] = altIDEntry{ids: filtered, expires: now.Add(altIDTTL)}
}

// Get returns the alternative identifiers recorded for authData.
func (c *AltIDCache) Get(authData string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[authData]
	if !ok || c.now().After(e.expires) {
		return nil
	}
	return e.ids
}

// SameUser implements einterfaces.OAuthProvider.IsSameUser for the Libre
// providers: the accounts must use the same authentication service and the
// stored AuthData must be the identifier of the verified identity, or one of
// the alternative identifiers observed for it during this login.
func SameUser(dbUser, oauthUser *model.User, alt *AltIDCache) bool {
	if dbUser == nil || oauthUser == nil || dbUser.AuthData == nil || oauthUser.AuthData == nil {
		return false
	}
	if dbUser.AuthService != oauthUser.AuthService {
		return false
	}
	dbAuth, oauthAuth := *dbUser.AuthData, *oauthUser.AuthData
	if dbAuth == "" || oauthAuth == "" {
		return false
	}
	if strings.EqualFold(dbAuth, oauthAuth) {
		return true
	}
	if alt != nil {
		for _, id := range alt.Get(oauthAuth) {
			if dbAuth == id {
				return true
			}
		}
	}
	return false
}
