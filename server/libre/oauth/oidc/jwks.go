// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

// Package oidc contains the OpenID Connect building blocks shared by the
// Mattermost Libre SSO providers: discovery document retrieval, JSON Web Key
// Set handling with key rollover, and ID/access token verification.
package oidc

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultKeySetTTL is how long a fetched key set is considered fresh.
	DefaultKeySetTTL = 24 * time.Hour
	// MinKeySetRefreshInterval rate-limits forced refreshes triggered by an
	// unknown key id, so a flood of bogus tokens cannot hammer the provider.
	MinKeySetRefreshInterval = time.Minute

	maxDocumentSize = 1 << 20
)

var (
	// ErrKeyNotFound is returned when no key in the set matches a token.
	ErrKeyNotFound = errors.New("no matching signing key found in the key set")
	// ErrKeySetUnavailable is returned when the key set could not be fetched
	// and no previously fetched keys are available.
	ErrKeySetUnavailable = errors.New("the signing key set is unavailable")
)

// HTTPClientFunc returns the HTTP client used for outgoing requests.
type HTTPClientFunc func() *http.Client

var defaultHTTPClient = &http.Client{Timeout: 30 * time.Second}

// DefaultHTTPClient returns a client with a sane timeout.
func DefaultHTTPClient() *http.Client {
	return defaultHTTPClient
}

type jsonWebKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

type publicKey struct {
	kid string
	alg string
	key crypto.PublicKey
}

// parseJWKS parses a JSON Web Key Set document and returns the usable
// signature verification keys. Keys of unsupported types are skipped.
func parseJWKS(data []byte) ([]publicKey, error) {
	var set struct {
		Keys []jsonWebKey `json:"keys"`
	}
	if err := json.Unmarshal(data, &set); err != nil {
		return nil, fmt.Errorf("invalid JWKS document: %w", err)
	}

	keys := make([]publicKey, 0, len(set.Keys))
	for _, k := range set.Keys {
		if k.Use != "" && k.Use != "sig" {
			continue
		}
		pk, err := k.publicKey()
		if err != nil || pk == nil {
			continue
		}
		keys = append(keys, publicKey{kid: k.Kid, alg: k.Alg, key: pk})
	}
	if len(keys) == 0 {
		return nil, errors.New("JWKS document contains no usable signing keys")
	}
	return keys, nil
}

func b64(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(stripPadding(s))
}

func stripPadding(s string) string {
	for len(s) > 0 && s[len(s)-1] == '=' {
		s = s[:len(s)-1]
	}
	return s
}

func (k *jsonWebKey) publicKey() (crypto.PublicKey, error) {
	switch k.Kty {
	case "RSA":
		nb, err := b64(k.N)
		if err != nil {
			return nil, err
		}
		eb, err := b64(k.E)
		if err != nil {
			return nil, err
		}
		if len(nb) == 0 || len(eb) == 0 || len(eb) > 4 {
			return nil, errors.New("invalid RSA key parameters")
		}
		e := 0
		for _, b := range eb {
			e = e<<8 | int(b)
		}
		pk := &rsa.PublicKey{N: new(big.Int).SetBytes(nb), E: e}
		if pk.N.BitLen() < 2048 {
			return nil, errors.New("RSA key too small")
		}
		return pk, nil
	case "EC":
		var curve elliptic.Curve
		var size int
		switch k.Crv {
		case "P-256":
			curve, size = elliptic.P256(), 32
		case "P-384":
			curve, size = elliptic.P384(), 48
		case "P-521":
			curve, size = elliptic.P521(), 66
		default:
			return nil, fmt.Errorf("unsupported curve %q", k.Crv)
		}
		xb, err := b64(k.X)
		if err != nil {
			return nil, err
		}
		yb, err := b64(k.Y)
		if err != nil {
			return nil, err
		}
		if len(xb) > size || len(yb) > size {
			return nil, errors.New("invalid EC key parameters")
		}
		raw := make([]byte, 1+2*size)
		raw[0] = 4
		copy(raw[1+size-len(xb):1+size], xb)
		copy(raw[1+2*size-len(yb):], yb)
		return ecdsa.ParseUncompressedPublicKey(curve, raw)
	case "OKP":
		if k.Crv != "Ed25519" {
			return nil, fmt.Errorf("unsupported curve %q", k.Crv)
		}
		xb, err := b64(k.X)
		if err != nil {
			return nil, err
		}
		if len(xb) != ed25519.PublicKeySize {
			return nil, errors.New("invalid Ed25519 key")
		}
		return ed25519.PublicKey(xb), nil
	}
	return nil, fmt.Errorf("unsupported key type %q", k.Kty)
}

// KeySet is a remote JSON Web Key Set. It is refreshed periodically and on
// demand when a token references a key id it does not know yet, which is how
// providers roll signing keys over.
type KeySet struct {
	// MinRefreshInterval rate-limits refreshes triggered by unknown key ids.
	MinRefreshInterval time.Duration

	url    string
	client HTTPClientFunc
	ttl    time.Duration
	now    func() time.Time

	mu          sync.Mutex
	keys        []publicKey
	fetchedAt   time.Time
	lastAttempt time.Time
}

// NewKeySet creates a key set backed by the given JWKS URL.
func NewKeySet(url string, client HTTPClientFunc) *KeySet {
	if client == nil {
		client = DefaultHTTPClient
	}
	return &KeySet{MinRefreshInterval: MinKeySetRefreshInterval, url: url, client: client, ttl: DefaultKeySetTTL, now: time.Now}
}

// URL returns the JWKS URL of the key set.
func (ks *KeySet) URL() string {
	return ks.url
}

// Key returns the key matching the given key id and algorithm, refreshing the
// set when needed.
func (ks *KeySet) Key(ctx context.Context, kid, alg string) (crypto.PublicKey, error) {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	now := ks.now()
	stale := ks.keys == nil || now.Sub(ks.fetchedAt) > ks.ttl
	if !stale {
		if k := findKey(ks.keys, kid, alg); k != nil {
			return k, nil
		}
	}

	// Refresh when stale, or when the key id is unknown (rollover), subject to rate limiting.
	if stale || now.Sub(ks.lastAttempt) >= ks.MinRefreshInterval {
		ks.lastAttempt = now
		keys, err := ks.fetch(ctx)
		if err != nil {
			if ks.keys == nil {
				return nil, fmt.Errorf("%w: %w", ErrKeySetUnavailable, err)
			}
			// Keep using the previous keys if the provider is temporarily unreachable.
		} else {
			ks.keys = keys
			ks.fetchedAt = now
		}
	}

	if k := findKey(ks.keys, kid, alg); k != nil {
		return k, nil
	}
	return nil, ErrKeyNotFound
}

func findKey(keys []publicKey, kid, alg string) crypto.PublicKey {
	var candidates []publicKey
	for _, k := range keys {
		if kid != "" && k.kid != kid {
			continue
		}
		if k.alg != "" && alg != "" && k.alg != alg {
			continue
		}
		if !keyMatchesAlg(k.key, alg) {
			continue
		}
		candidates = append(candidates, k)
	}
	// Without a key id we only accept an unambiguous match.
	if len(candidates) == 1 || (kid != "" && len(candidates) > 0) {
		return candidates[0].key
	}
	return nil
}

func keyMatchesAlg(key crypto.PublicKey, alg string) bool {
	if alg == "" {
		return true
	}
	switch key.(type) {
	case *rsa.PublicKey:
		return strings.HasPrefix(alg, "RS") || strings.HasPrefix(alg, "PS")
	case *ecdsa.PublicKey:
		return strings.HasPrefix(alg, "ES")
	case ed25519.PublicKey:
		return alg == "EdDSA"
	}
	return false
}

func (ks *KeySet) fetch(ctx context.Context) ([]publicKey, error) {
	body, err := getDocument(ctx, ks.client(), ks.url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS from %s: %w", ks.url, err)
	}
	return parseJWKS(body)
}

// KeySetCache shares KeySet instances per URL so that key rollover state and
// cached keys survive across requests.
type KeySetCache struct {
	client HTTPClientFunc
	mu     sync.Mutex
	sets   map[string]*KeySet
}

// NewKeySetCache creates an empty cache.
func NewKeySetCache(client HTTPClientFunc) *KeySetCache {
	return &KeySetCache{client: client, sets: map[string]*KeySet{}}
}

// Get returns the key set for the URL, creating it if needed.
func (c *KeySetCache) Get(url string) *KeySet {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ks, ok := c.sets[url]; ok {
		return ks
	}
	ks := NewKeySet(url, c.client)
	c.sets[url] = ks
	return ks
}

func getDocument(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := readLimited(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}
	return body, nil
}

func readLimited(resp *http.Response) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxDocumentSize+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxDocumentSize {
		return nil, errors.New("response document is too large")
	}
	return body, nil
}
