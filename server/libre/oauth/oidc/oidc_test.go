// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package oidc_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/libre/oauth/oidc"
	"github.com/mattermost/mattermost/server/v8/libre/oauth/oidc/oidctest"
)

func baseClaims(p *oidctest.Provider) jwt.MapClaims {
	return jwt.MapClaims{"iss": p.URL(), "aud": "client", "sub": "user-1"}
}

func verifyOpts(p *oidctest.Provider, ks *oidc.KeySet) oidc.VerifyOptions {
	return oidc.VerifyOptions{KeySet: ks, Issuers: []string{p.URL()}, Audiences: []string{"client"}}
}

func TestVerify(t *testing.T) {
	p := oidctest.New()
	defer p.Close()
	ks := oidc.NewKeySet(p.JWKSURL(), nil)
	ctx := context.Background()

	t.Run("valid token", func(t *testing.T) {
		claims, err := oidc.Verify(ctx, p.Sign(baseClaims(p)), verifyOpts(p, ks))
		require.NoError(t, err)
		assert.Equal(t, "user-1", oidc.String(claims, "sub"))
	})

	t.Run("audience list", func(t *testing.T) {
		c := baseClaims(p)
		c["aud"] = []string{"other", "client"}
		c["azp"] = "client"
		_, err := oidc.Verify(ctx, p.Sign(c), verifyOpts(p, ks))
		require.NoError(t, err)
	})

	t.Run("wrong audience", func(t *testing.T) {
		c := baseClaims(p)
		c["aud"] = "someone-else"
		_, err := oidc.Verify(ctx, p.Sign(c), verifyOpts(p, ks))
		require.ErrorIs(t, err, oidc.ErrInvalidAudience)
	})

	t.Run("wrong authorized party", func(t *testing.T) {
		c := baseClaims(p)
		c["azp"] = "someone-else"
		opts := verifyOpts(p, ks)
		opts.AuthorizedParty = "client"
		_, err := oidc.Verify(ctx, p.Sign(c), opts)
		require.ErrorIs(t, err, oidc.ErrInvalidAudience)
	})

	t.Run("wrong issuer", func(t *testing.T) {
		c := baseClaims(p)
		c["iss"] = "https://evil.example.com"
		_, err := oidc.Verify(ctx, p.Sign(c), verifyOpts(p, ks))
		require.ErrorIs(t, err, oidc.ErrInvalidIssuer)
	})

	t.Run("expired", func(t *testing.T) {
		c := baseClaims(p)
		c["exp"] = time.Now().Add(-time.Hour).Unix()
		c["iat"] = time.Now().Add(-2 * time.Hour).Unix()
		_, err := oidc.Verify(ctx, p.Sign(c), verifyOpts(p, ks))
		require.ErrorIs(t, err, oidc.ErrTokenExpired)
	})

	t.Run("missing expiry", func(t *testing.T) {
		signed := p.Sign(jwt.MapClaims{"iss": p.URL(), "aud": "client", "sub": "x", "exp": nil})
		_, err := oidc.Verify(ctx, signed, verifyOpts(p, ks))
		require.Error(t, err)
	})

	t.Run("unknown signing key", func(t *testing.T) {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		_, err = oidc.Verify(ctx, oidctest.SignWith(key, "unknown", baseClaims(p)), verifyOpts(p, ks))
		require.Error(t, err)
	})

	t.Run("forged token with a published kid", func(t *testing.T) {
		valid := p.Sign(baseClaims(p))
		header, _, _ := cut3(valid)
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		forged := oidctest.SignWith(key, kidOf(t, header), baseClaims(p))
		_, err = oidc.Verify(ctx, forged, verifyOpts(p, ks))
		require.Error(t, err)
	})

	t.Run("alg none is refused", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodNone, baseClaims(p))
		s, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
		require.NoError(t, err)
		_, err = oidc.Verify(ctx, s, verifyOpts(p, ks))
		require.Error(t, err)
	})

	t.Run("HMAC only when a secret is configured", func(t *testing.T) {
		c := baseClaims(p)
		c["exp"] = time.Now().Add(time.Hour).Unix()
		s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte("secret"))
		require.NoError(t, err)

		_, err = oidc.Verify(ctx, s, verifyOpts(p, ks))
		require.Error(t, err)

		opts := verifyOpts(p, ks)
		opts.HMACSecret = []byte("secret")
		_, err = oidc.Verify(ctx, s, opts)
		require.NoError(t, err)

		opts.HMACSecret = []byte("other")
		_, err = oidc.Verify(ctx, s, opts)
		require.Error(t, err)
	})
}

func cut3(token string) (string, string, string) {
	var parts [3]string
	i := 0
	start := 0
	for j := 0; j < len(token) && i < 2; j++ {
		if token[j] == '.' {
			parts[i] = token[start:j]
			start = j + 1
			i++
		}
	}
	parts[2] = token[start:]
	return parts[0], parts[1], parts[2]
}

func kidOf(t *testing.T, header string) string {
	b, err := base64.RawURLEncoding.DecodeString(header)
	require.NoError(t, err)
	var h map[string]any
	require.NoError(t, json.Unmarshal(b, &h))
	return h["kid"].(string)
}

func TestKeySetRollover(t *testing.T) {
	p := oidctest.New()
	defer p.Close()
	ks := oidc.NewKeySet(p.JWKSURL(), nil)
	ctx := context.Background()

	_, err := oidc.Verify(ctx, p.Sign(baseClaims(p)), verifyOpts(p, ks))
	require.NoError(t, err)
	require.EqualValues(t, 1, p.JWKSRequests.Load())

	// Cached: no new fetch.
	_, err = oidc.Verify(ctx, p.Sign(baseClaims(p)), verifyOpts(p, ks))
	require.NoError(t, err)
	require.EqualValues(t, 1, p.JWKSRequests.Load())

	// The provider rolls its key over: the unknown kid triggers a refresh.
	ks.MinRefreshInterval = 0
	p.RotateKey()
	_, err = oidc.Verify(ctx, p.Sign(baseClaims(p)), verifyOpts(p, ks))
	require.NoError(t, err)
	require.EqualValues(t, 2, p.JWKSRequests.Load())

	// Unknown kids don't cause a refresh storm.
	ks.MinRefreshInterval = time.Hour
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	for range 5 {
		_, err = oidc.Verify(ctx, oidctest.SignWith(key, "bogus", baseClaims(p)), verifyOpts(p, ks))
		require.Error(t, err)
	}
	require.EqualValues(t, 2, p.JWKSRequests.Load())
}

func TestECKeys(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	pub, err := key.PublicKey.Bytes()
	require.NoError(t, err)
	jwks := map[string]any{"keys": []map[string]string{{
		"kty": "EC", "crv": "P-256", "kid": "ec1",
		"x": base64.RawURLEncoding.EncodeToString(pub[1:33]),
		"y": base64.RawURLEncoding.EncodeToString(pub[33:]),
	}}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	defer srv.Close()

	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": "iss", "aud": "client", "sub": "s", "exp": time.Now().Add(time.Minute).Unix(),
	})
	token.Header["kid"] = "ec1"
	s, err := token.SignedString(key)
	require.NoError(t, err)

	_, err = oidc.Verify(context.Background(), s, oidc.VerifyOptions{
		KeySet: oidc.NewKeySet(srv.URL, nil), Issuers: []string{"iss"}, Audiences: []string{"client"},
	})
	require.NoError(t, err)
}

func TestDiscovery(t *testing.T) {
	p := oidctest.New()
	defer p.Close()

	cache := oidc.NewDiscoveryCache(nil)
	doc, err := cache.Get(context.Background(), p.DiscoveryURL())
	require.NoError(t, err)
	assert.Equal(t, p.URL(), doc.Issuer)
	assert.Equal(t, p.URL()+"/userinfo", doc.UserinfoEndpoint)

	// Served from cache even when the provider goes away.
	p.Close()
	doc2, err := cache.Get(context.Background(), p.DiscoveryURL())
	require.NoError(t, err)
	assert.Same(t, doc, doc2)

	_, err = oidc.NewDiscoveryCache(nil).Get(context.Background(), p.DiscoveryURL())
	require.Error(t, err)
}

func TestIssuerTemplate(t *testing.T) {
	doc := &oidc.DiscoveryDocument{Issuer: "https://login.microsoftonline.com/{tenantid}/v2.0"}
	assert.True(t, doc.IssuerMatches("https://login.microsoftonline.com/abc/v2.0", "abc"))
	assert.False(t, doc.IssuerMatches("https://login.microsoftonline.com/abc/v2.0", "def"))
	assert.False(t, doc.IssuerMatches("https://login.microsoftonline.com/abc/v2.0", ""))
}

func TestIdentity(t *testing.T) {
	id := oidc.IdentityFromClaims(oidc.Claims{
		"sub": "s", "oid": "o", "tid": "t", "preferred_username": "John.Doe@Example.com", "name": "John Middle Doe",
	})
	assert.Equal(t, "john.doe@example.com", id.BestEmail())
	first, last := id.Names()
	assert.Equal(t, "John", first)
	assert.Equal(t, "Middle Doe", last)

	u := id.ToUser(nil, "o", true)
	assert.Equal(t, "john.doe", u.Username)
	back, ok := oidc.IdentityFromUser(u)
	require.True(t, ok)
	assert.Equal(t, id, back)

	_, ok = oidc.IdentityFromUser(&model.User{})
	assert.False(t, ok)
}

func TestSameUser(t *testing.T) {
	alt := oidc.NewAltIDCache()
	db := &model.User{AuthService: "openid", AuthData: model.NewPointer("legacy-sub")}
	oauth := &model.User{AuthService: "openid", AuthData: model.NewPointer("object-id")}
	assert.False(t, oidc.SameUser(db, oauth, alt))

	alt.Put("object-id", "legacy-sub")
	assert.True(t, oidc.SameUser(db, oauth, alt))

	other := &model.User{AuthService: "gitlab", AuthData: model.NewPointer("legacy-sub")}
	assert.False(t, oidc.SameUser(other, oauth, alt))
	assert.False(t, oidc.SameUser(&model.User{AuthService: "openid"}, oauth, alt))
	assert.True(t, oidc.SameUser(&model.User{AuthService: "openid", AuthData: model.NewPointer("OBJECT-ID")}, oauth, nil))
}
