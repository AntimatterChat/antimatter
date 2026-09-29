// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package oidc

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// DefaultLeeway is the clock skew tolerated when validating time based claims.
const DefaultLeeway = 2 * time.Minute

// Claims are the decoded claims of a JSON Web Token.
type Claims = jwt.MapClaims

var (
	// ErrTokenExpired is returned (wrapped) when the token is expired.
	ErrTokenExpired = jwt.ErrTokenExpired
	// ErrInvalidIssuer is returned when the issuer is not accepted.
	ErrInvalidIssuer = errors.New("token issuer is not accepted")
	// ErrInvalidAudience is returned when the audience does not match.
	ErrInvalidAudience = errors.New("token audience is not accepted")
)

var asymmetricAlgs = []string{"RS256", "RS384", "RS512", "PS256", "PS384", "PS512", "ES256", "ES384", "ES512", "EdDSA"}
var hmacAlgs = []string{"HS256", "HS384", "HS512"}

// VerifyOptions configures Verify.
type VerifyOptions struct {
	// KeySet provides the provider's public signing keys.
	KeySet *KeySet
	// HMACSecret, when set, additionally allows HS* algorithms (OpenID
	// Connect providers may sign ID tokens with the client secret).
	HMACSecret []byte
	// Issuers lists the accepted issuers. IssuerFunc may be used instead.
	Issuers []string
	// IssuerFunc decides whether an issuer is accepted.
	IssuerFunc func(iss string, claims Claims) bool
	// Audiences lists the accepted audiences; the token must contain at least one.
	Audiences []string
	// AuthorizedParty, when set, must match the azp claim if the token has one.
	AuthorizedParty string
	// Leeway is the allowed clock skew; DefaultLeeway when zero.
	Leeway time.Duration
	// Now overrides the current time (tests).
	Now func() time.Time
}

// Verify checks the signature and the standard claims (exp, nbf, iat, iss,
// aud, azp) of a compact serialized JWT and returns its claims.
func Verify(ctx context.Context, raw string, opts VerifyOptions) (Claims, error) {
	if opts.KeySet == nil && len(opts.HMACSecret) == 0 {
		return nil, errors.New("no verification key configured")
	}

	algs := slices.Clone(asymmetricAlgs)
	if len(opts.HMACSecret) > 0 {
		algs = append(algs, hmacAlgs...)
	}

	leeway := opts.Leeway
	if leeway == 0 {
		leeway = DefaultLeeway
	}
	parserOpts := []jwt.ParserOption{
		jwt.WithValidMethods(algs),
		jwt.WithLeeway(leeway),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
	}
	if opts.Now != nil {
		parserOpts = append(parserOpts, jwt.WithTimeFunc(opts.Now))
	}

	claims := Claims{}
	_, err := jwt.NewParser(parserOpts...).ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		alg, _ := t.Header["alg"].(string)
		if slices.Contains(hmacAlgs, alg) {
			return opts.HMACSecret, nil
		}
		if opts.KeySet == nil {
			return nil, ErrKeyNotFound
		}
		kid, _ := t.Header["kid"].(string)
		return opts.KeySet.Key(ctx, kid, alg)
	})
	if err != nil {
		return nil, err
	}

	iss, _ := claims["iss"].(string)
	switch {
	case opts.IssuerFunc != nil:
		if !opts.IssuerFunc(iss, claims) {
			return nil, fmt.Errorf("%w: %q", ErrInvalidIssuer, iss)
		}
	case len(opts.Issuers) > 0:
		if !slices.Contains(opts.Issuers, iss) {
			return nil, fmt.Errorf("%w: %q", ErrInvalidIssuer, iss)
		}
	default:
		return nil, errors.New("no accepted issuer configured")
	}

	aud, err := claims.GetAudience()
	if err != nil {
		return nil, err
	}
	if len(opts.Audiences) == 0 {
		return nil, errors.New("no accepted audience configured")
	}
	if !slices.ContainsFunc([]string(aud), func(a string) bool { return slices.Contains(opts.Audiences, a) }) {
		return nil, fmt.Errorf("%w: %v", ErrInvalidAudience, []string(aud))
	}
	if opts.AuthorizedParty != "" {
		if azp, ok := claims["azp"].(string); ok && azp != "" && azp != opts.AuthorizedParty {
			return nil, fmt.Errorf("%w: unexpected authorized party %q", ErrInvalidAudience, azp)
		}
	}

	return claims, nil
}

// UnverifiedClaims decodes the claims of a token WITHOUT verifying it. It must
// only be used to select the verification parameters.
func UnverifiedClaims(raw string) (Claims, error) {
	claims := Claims{}
	if _, _, err := jwt.NewParser().ParseUnverified(raw, claims); err != nil {
		return nil, err
	}
	return claims, nil
}

// String returns a string claim, or "" if missing or not a string.
func String(claims Claims, name string) string {
	v, _ := claims[name].(string)
	return strings.TrimSpace(v)
}

// Bool returns a boolean claim and whether it was present. Some providers
// encode booleans as strings.
func Bool(claims Claims, name string) (value bool, present bool) {
	switch v := claims[name].(type) {
	case bool:
		return v, true
	case string:
		return strings.EqualFold(v, "true"), true
	}
	return false, false
}

// StringList returns a claim holding either a single string or a list of
// strings (space separated strings are split, as used by "scp").
func StringList(claims Claims, name string) []string {
	switch v := claims[name].(type) {
	case string:
		return strings.Fields(v)
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
