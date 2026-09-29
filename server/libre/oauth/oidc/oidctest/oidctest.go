// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

// Package oidctest provides an in-memory OpenID Connect provider for tests.
package oidctest

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Provider is a fake OpenID Connect provider serving a discovery document,
// a JWKS and a userinfo endpoint.
type Provider struct {
	Server *httptest.Server
	// Issuer overrides the issuer published in the discovery document.
	Issuer string

	mu       sync.Mutex
	keys     []*signingKey
	userinfo map[string]any

	JWKSRequests atomic.Int32
}

type signingKey struct {
	kid string
	key *rsa.PrivateKey
}

// New starts a fake provider. Call Close when done.
func New() *Provider {
	p := &Provider{}
	p.RotateKey()
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		base := p.Server.URL
		iss := p.Issuer
		if iss == "" {
			iss = base
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 iss,
			"authorization_endpoint": base + "/authorize",
			"token_endpoint":         base + "/token",
			"userinfo_endpoint":      base + "/userinfo",
			"jwks_uri":               base + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		p.JWKSRequests.Add(1)
		_ = json.NewEncoder(w).Encode(p.JWKS())
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		defer p.mu.Unlock()
		_ = json.NewEncoder(w).Encode(p.userinfo)
	})
	p.Server = httptest.NewServer(mux)
	return p
}

// Close stops the server.
func (p *Provider) Close() {
	p.Server.Close()
}

// URL returns the base URL, which is also the default issuer.
func (p *Provider) URL() string {
	return p.Server.URL
}

// DiscoveryURL returns the discovery document URL.
func (p *Provider) DiscoveryURL() string {
	return p.Server.URL + "/.well-known/openid-configuration"
}

// JWKSURL returns the key set URL.
func (p *Provider) JWKSURL() string {
	return p.Server.URL + "/jwks"
}

// SetUserinfo sets the userinfo response.
func (p *Provider) SetUserinfo(v map[string]any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.userinfo = v
}

// RotateKey adds a new signing key and makes it the current one. The old
// keys remain published.
func (p *Provider) RotateKey() {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	kidBytes := make([]byte, 8)
	_, _ = rand.Read(kidBytes)
	kid := base64.RawURLEncoding.EncodeToString(kidBytes)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.keys = append(p.keys, &signingKey{kid: kid, key: key})
}

// JWKS returns the published key set.
func (p *Provider) JWKS() map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	keys := make([]map[string]string, 0, len(p.keys))
	for _, k := range p.keys {
		keys = append(keys, map[string]string{
			"kty": "RSA",
			"use": "sig",
			"alg": "RS256",
			"kid": k.kid,
			"n":   base64.RawURLEncoding.EncodeToString(k.key.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(k.key.E)).Bytes()),
		})
	}
	return map[string]any{"keys": keys}
}

// Sign signs the claims with the current key. Missing iat/exp are filled in.
func (p *Provider) Sign(claims jwt.MapClaims) string {
	p.mu.Lock()
	k := p.keys[len(p.keys)-1]
	p.mu.Unlock()
	return SignWith(k.key, k.kid, claims)
}

// SignWith signs the claims with an arbitrary key (e.g. an unpublished one).
func SignWith(key *rsa.PrivateKey, kid string, claims jwt.MapClaims) string {
	if _, ok := claims["iat"]; !ok {
		claims["iat"] = time.Now().Unix()
	}
	if _, ok := claims["exp"]; !ok {
		claims["exp"] = time.Now().Add(time.Hour).Unix()
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	s, err := token.SignedString(key)
	if err != nil {
		panic(err)
	}
	return s
}
