// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package outgoingoauth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/mattermost/mattermost/server/public/model"
)

const (
	// expiryDelta is subtracted from the token lifetime so that a token is
	// never used right before it expires.
	expiryDelta = 30 * time.Second
	// defaultTokenLifetime is used when the server doesn't return expires_in.
	defaultTokenLifetime = 5 * time.Minute
	tokenRequestTimeout  = 30 * time.Second
	maxTokenResponseSize = 1 << 20
)

type authStyle int

const (
	authStyleUnknown authStyle = iota
	// authStyleHeader sends the client credentials with HTTP Basic auth
	// (client_secret_basic, preferred by RFC 6749).
	authStyleHeader
	// authStyleParams sends them in the request body (client_secret_post).
	authStyleParams
)

// tokenError is returned when the authorization server refused the request.
type tokenError struct {
	Status      int
	Code        string
	Description string
}

func (e *tokenError) Error() string {
	msg := fmt.Sprintf("token endpoint returned status %d", e.Status)
	if e.Code != "" {
		msg += ": " + e.Code
	}
	if e.Description != "" {
		msg += " (" + e.Description + ")"
	}
	return msg
}

type cachedToken struct {
	connID  string
	token   model.OutgoingOAuthConnectionToken
	expires time.Time
}

type tokenSource struct {
	client func() *http.Client
	now    func() time.Time
	group  singleflight.Group

	mu     sync.Mutex
	cache  map[string]cachedToken
	styles map[string]authStyle
}

func newTokenSource(client func() *http.Client) *tokenSource {
	return &tokenSource{
		client: client,
		now:    time.Now,
		cache:  map[string]cachedToken{},
		styles: map[string]authStyle{},
	}
}

// cacheKey identifies a connection configuration, so that a token is never
// reused after the credentials or the token URL change.
func cacheKey(c *model.OutgoingOAuthConnection) string {
	h := sha256.New()
	for _, part := range []string{
		c.Id, c.OAuthTokenURL, string(c.GrantType), c.ClientId, c.ClientSecret,
		model.SafeDereference(c.CredentialsUsername), model.SafeDereference(c.CredentialsPassword),
	} {
		h.Write([]byte(strconv.Itoa(len(part))))
		h.Write([]byte{':'})
		h.Write([]byte(part))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (ts *tokenSource) forget(connID string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	for k, v := range ts.cache {
		if v.connID == connID {
			delete(ts.cache, k)
		}
	}
}

func (ts *tokenSource) token(ctx context.Context, c *model.OutgoingOAuthConnection) (*model.OutgoingOAuthConnectionToken, error) {
	key := cacheKey(c)

	ts.mu.Lock()
	if cached, ok := ts.cache[key]; ok {
		if ts.now().Before(cached.expires) {
			ts.mu.Unlock()
			t := cached.token
			return &t, nil
		}
		delete(ts.cache, key)
	}
	ts.mu.Unlock()

	v, err, _ := ts.group.Do(key, func() (any, error) {
		// Don't let one caller's cancellation fail everyone waiting on the same fetch.
		fetchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), tokenRequestTimeout)
		defer cancel()

		token, lifetime, err := ts.fetch(fetchCtx, c)
		if err != nil {
			return nil, err
		}
		expires := ts.now().Add(lifetime - expiryDelta)
		if lifetime <= expiryDelta {
			expires = ts.now().Add(lifetime / 2)
		}

		ts.mu.Lock()
		ts.pruneLocked()
		ts.cache[key] = cachedToken{connID: c.Id, token: *token, expires: expires}
		ts.mu.Unlock()
		return token, nil
	})
	if err != nil {
		return nil, err
	}
	t := *v.(*model.OutgoingOAuthConnectionToken)
	return &t, nil
}

func (ts *tokenSource) pruneLocked() {
	now := ts.now()
	for k, v := range ts.cache {
		if !now.Before(v.expires) {
			delete(ts.cache, k)
		}
	}
}

func (ts *tokenSource) fetch(ctx context.Context, c *model.OutgoingOAuthConnection) (*model.OutgoingOAuthConnectionToken, time.Duration, error) {
	ts.mu.Lock()
	style := ts.styles[c.OAuthTokenURL]
	ts.mu.Unlock()

	if style != authStyleUnknown {
		return ts.request(ctx, c, style)
	}

	// Like most OAuth clients, try HTTP Basic first and fall back to sending
	// the credentials in the body when the server rejects it.
	token, lifetime, err := ts.request(ctx, c, authStyleHeader)
	if err == nil {
		ts.rememberStyle(c.OAuthTokenURL, authStyleHeader)
		return token, lifetime, nil
	}
	var te *tokenError
	if !errors.As(err, &te) || (te.Status != http.StatusBadRequest && te.Status != http.StatusUnauthorized) {
		return nil, 0, err
	}
	token, lifetime, err2 := ts.request(ctx, c, authStyleParams)
	if err2 == nil {
		ts.rememberStyle(c.OAuthTokenURL, authStyleParams)
		return token, lifetime, nil
	}
	return nil, 0, err2
}

func (ts *tokenSource) rememberStyle(tokenURL string, style authStyle) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.styles[tokenURL] = style
}

func (ts *tokenSource) request(ctx context.Context, c *model.OutgoingOAuthConnection, style authStyle) (*model.OutgoingOAuthConnectionToken, time.Duration, error) {
	form := url.Values{}
	form.Set("grant_type", string(c.GrantType))
	if c.GrantType == model.OutgoingOAuthConnectionGrantTypePassword {
		form.Set("username", model.SafeDereference(c.CredentialsUsername))
		form.Set("password", model.SafeDereference(c.CredentialsPassword))
	}
	if style == authStyleParams {
		form.Set("client_id", c.ClientId)
		form.Set("client_secret", c.ClientSecret)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.OAuthTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if style == authStyleHeader {
		// RFC 6749 section 2.3.1: the credentials are form-encoded before being used for Basic auth.
		req.SetBasicAuth(url.QueryEscape(c.ClientId), url.QueryEscape(c.ClientSecret))
	}

	resp, err := ts.client().Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTokenResponseSize))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read the token response: %w", err)
	}

	fields, parseErr := parseTokenResponse(resp.Header.Get("Content-Type"), body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		te := &tokenError{Status: resp.StatusCode}
		if parseErr == nil {
			te.Code = fields.str("error")
			te.Description = fields.str("error_description")
		}
		return nil, 0, te
	}
	if parseErr != nil {
		return nil, 0, fmt.Errorf("invalid token response: %w", parseErr)
	}
	if code := fields.str("error"); code != "" {
		return nil, 0, &tokenError{Status: resp.StatusCode, Code: code, Description: fields.str("error_description")}
	}

	accessToken := fields.str("access_token")
	if accessToken == "" {
		return nil, 0, errors.New("the token response has no access_token")
	}
	tokenType := fields.str("token_type")
	if tokenType == "" || strings.EqualFold(tokenType, "bearer") {
		tokenType = "Bearer"
	}

	lifetime := defaultTokenLifetime
	if secs, ok := fields.seconds("expires_in"); ok && secs > 0 {
		lifetime = time.Duration(secs) * time.Second
	}

	return &model.OutgoingOAuthConnectionToken{AccessToken: accessToken, TokenType: tokenType}, lifetime, nil
}

type tokenFields map[string]any

func (f tokenFields) str(name string) string {
	switch v := f[name].(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	}
	return ""
}

func (f tokenFields) seconds(name string) (int64, bool) {
	switch v := f[name].(type) {
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return n, true
		}
		if fl, err := v.Float64(); err == nil {
			return int64(fl), true
		}
	case string:
		if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
			return n, true
		}
	}
	return 0, false
}

// parseTokenResponse decodes a JSON token response, or a form encoded one
// (returned by some older servers).
func parseTokenResponse(contentType string, body []byte) (tokenFields, error) {
	mediaType, _, _ := mime.ParseMediaType(contentType)
	if mediaType != "application/x-www-form-urlencoded" {
		fields := tokenFields{}
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.UseNumber()
		err := decoder.Decode(&fields)
		if err == nil {
			return fields, nil
		}
		if !bytes.Contains(body, []byte("=")) {
			return nil, err
		}
	}

	values, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, err
	}
	fields := tokenFields{}
	for k := range values {
		fields[k] = values.Get(k)
	}
	return fields, nil
}
