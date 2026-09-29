// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package outgoingoauth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// fakeStore mimics the SQL store, including the fact that SaveConnection
// doesn't persist the password grant credentials.
type fakeStore struct {
	mu    sync.Mutex
	conns map[string]*model.OutgoingOAuthConnection
}

func newFakeStore() *fakeStore {
	return &fakeStore{conns: map[string]*model.OutgoingOAuthConnection{}}
}

func (s *fakeStore) SaveConnection(_ request.CTX, conn *model.OutgoingOAuthConnection) (*model.OutgoingOAuthConnection, error) {
	if conn.Id != "" {
		return nil, store.NewErrInvalidInput("OutgoingOAuthConnection", "Id", conn.Id)
	}
	conn.PreSave()
	if err := conn.IsValid(); err != nil {
		return nil, err
	}
	stored := *conn
	stored.CredentialsUsername, stored.CredentialsPassword = nil, nil
	s.mu.Lock()
	s.conns[conn.Id] = &stored
	s.mu.Unlock()
	return conn, nil
}

func (s *fakeStore) UpdateConnection(_ request.CTX, conn *model.OutgoingOAuthConnection) (*model.OutgoingOAuthConnection, error) {
	conn.PreUpdate()
	if err := conn.IsValid(); err != nil {
		return nil, err
	}
	stored := *conn
	s.mu.Lock()
	s.conns[conn.Id] = &stored
	s.mu.Unlock()
	return conn, nil
}

func (s *fakeStore) GetConnection(_ request.CTX, id string) (*model.OutgoingOAuthConnection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.conns[id]
	if !ok {
		return nil, store.NewErrNotFound("OutgoingOAuthConnection", id)
	}
	cp := *c
	return &cp, nil
}

func (s *fakeStore) GetConnections(_ request.CTX, filters model.OutgoingOAuthConnectionGetConnectionsFilter) ([]*model.OutgoingOAuthConnection, error) {
	filters.SetDefaults()
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]string, 0, len(s.conns))
	for id := range s.conns {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	var out []*model.OutgoingOAuthConnection
	for _, id := range ids {
		if id <= filters.OffsetId {
			continue
		}
		c := *s.conns[id]
		out = append(out, &c)
		if len(out) == filters.Limit {
			break
		}
	}
	return out, nil
}

func (s *fakeStore) DeleteConnection(_ request.CTX, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.conns, id)
	return nil
}

func newConn(name string, audiences ...string) *model.OutgoingOAuthConnection {
	return &model.OutgoingOAuthConnection{
		CreatorId:     model.NewId(),
		Name:          name,
		ClientId:      "client",
		ClientSecret:  "secret",
		OAuthTokenURL: "https://auth.example.com/token",
		GrantType:     model.OutgoingOAuthConnectionGrantTypeClientCredentials,
		Audiences:     audiences,
	}
}

func newService(st *fakeStore) *Service {
	return New(func() store.OutgoingOAuthConnectionStore { return st }, func() *http.Client { return http.DefaultClient })
}

func TestCRUD(t *testing.T) {
	rctx := request.TestContext(t)
	st := newFakeStore()
	s := newService(st)

	saved, appErr := s.SaveConnection(rctx, newConn("one", "https://api.example.com"))
	require.Nil(t, appErr)
	require.NotEmpty(t, saved.Id)

	got, appErr := s.GetConnection(rctx, saved.Id)
	require.Nil(t, appErr)
	assert.Equal(t, "one", got.Name)

	_, appErr = s.GetConnection(rctx, model.NewId())
	require.NotNil(t, appErr)
	assert.Equal(t, http.StatusNotFound, appErr.StatusCode)

	t.Run("duplicated audience", func(t *testing.T) {
		_, appErr := s.SaveConnection(rctx, newConn("two", "https://API.example.com/"))
		require.NotNil(t, appErr)
		assert.Equal(t, "ent.outgoing_oauth_connections.save_connection.audience_duplicated", appErr.Id)
	})

	t.Run("invalid audience", func(t *testing.T) {
		_, appErr := s.SaveConnection(rctx, newConn("two", "ftp://files.example.com"))
		require.NotNil(t, appErr)
		assert.Equal(t, "ent.outgoing_oauth_connections.save_connection.audience_invalid", appErr.Id)
	})

	t.Run("invalid connection", func(t *testing.T) {
		c := newConn("", "https://other.example.com")
		_, appErr := s.SaveConnection(rctx, c)
		require.NotNil(t, appErr)
		assert.Equal(t, http.StatusBadRequest, appErr.StatusCode)
	})

	t.Run("update keeps its own audience", func(t *testing.T) {
		got.Name = "renamed"
		updated, appErr := s.UpdateConnection(rctx, got)
		require.Nil(t, appErr)
		assert.Equal(t, "renamed", updated.Name)
	})

	t.Run("password credentials are persisted", func(t *testing.T) {
		c := newConn("pwd", "https://pwd.example.com")
		c.GrantType = model.OutgoingOAuthConnectionGrantTypePassword
		c.CredentialsUsername = model.NewPointer("user")
		c.CredentialsPassword = model.NewPointer("pass")
		saved, appErr := s.SaveConnection(rctx, c)
		require.Nil(t, appErr)
		stored, appErr := s.GetConnection(rctx, saved.Id)
		require.Nil(t, appErr)
		require.NotNil(t, stored.CredentialsPassword)
		assert.Equal(t, "pass", *stored.CredentialsPassword)
	})

	conns, appErr := s.GetConnections(rctx, model.OutgoingOAuthConnectionGetConnectionsFilter{})
	require.Nil(t, appErr)
	assert.Len(t, conns, 2)

	conns = append(conns, nil)
	s.SanitizeConnections(conns)
	assert.Empty(t, conns[0].ClientSecret)

	require.Nil(t, s.DeleteConnection(rctx, saved.Id))
	_, appErr = s.GetConnection(rctx, saved.Id)
	require.NotNil(t, appErr)
}

func TestGetConnectionForAudience(t *testing.T) {
	rctx := request.TestContext(t)
	st := newFakeStore()
	s := newService(st)

	root, appErr := s.SaveConnection(rctx, newConn("root", "https://api.example.com"))
	require.Nil(t, appErr)
	hooks, appErr := s.SaveConnection(rctx, newConn("hooks", "https://api.example.com/hooks/"))
	require.Nil(t, appErr)
	// Enough connections to require paging.
	for i := range listPageSize + 5 {
		_, appErr := s.SaveConnection(rctx, newConn("filler", "https://filler"+model.NewId()+".example.com/"+string(rune('a'+i%26))))
		require.Nil(t, appErr)
	}
	last, appErr := s.SaveConnection(rctx, newConn("last", "http://localhost:8080/cmd"))
	require.Nil(t, appErr)

	for target, expected := range map[string]*model.OutgoingOAuthConnection{
		"https://api.example.com":                   root,
		"https://api.example.com/other/path?x=1":    root,
		"https://API.example.com:443/hooks":         hooks,
		"https://api.example.com/hooks/abc":         hooks,
		"https://api.example.com/hooksabc":          root,
		"http://localhost:8080/cmd/run":             last,
		"http://localhost:8080/command":             nil,
		"http://api.example.com/":                   nil,
		"https://api.example.com.evil.com/hooks":    nil,
		"https://evil.com/?https://api.example.com": nil,
		"not a url": nil,
	} {
		t.Run(target, func(t *testing.T) {
			conn, appErr := s.GetConnectionForAudience(rctx, target)
			require.Nil(t, appErr)
			if expected == nil {
				assert.Nil(t, conn)
			} else {
				require.NotNil(t, conn)
				assert.Equal(t, expected.Id, conn.Id)
			}
		})
	}
}

type tokenServer struct {
	*httptest.Server
	requests  atomic.Int32
	basicOnly bool
	postOnly  bool
	expiresIn any
}

func newTokenServer(t *testing.T) *tokenServer {
	ts := &tokenServer{expiresIn: 3600}
	ts.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ts.requests.Add(1)
		require.NoError(t, r.ParseForm())

		user, pass, hasBasic := r.BasicAuth()
		if hasBasic {
			user, _ = url.QueryUnescape(user)
			pass, _ = url.QueryUnescape(pass)
		} else {
			user, pass = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
		}
		w.Header().Set("Content-Type", "application/json")
		if (ts.postOnly && hasBasic) || (ts.basicOnly && !hasBasic) || user != "client:id" || pass != "s3cr&t" {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_client"})
			return
		}
		switch r.PostForm.Get("grant_type") {
		case "client_credentials":
		case "password":
			if r.PostForm.Get("username") != "alice" || r.PostForm.Get("password") != "pw" {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant", "error_description": "bad password"})
				return
			}
		default:
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "token-" + r.PostForm.Get("grant_type"),
			"token_type":   "bearer",
			"expires_in":   ts.expiresIn,
		})
	}))
	t.Cleanup(ts.Close)
	return ts
}

func (ts *tokenServer) conn() *model.OutgoingOAuthConnection {
	c := newConn("c", "https://api.example.com")
	c.Id = model.NewId()
	c.ClientId = "client:id"
	c.ClientSecret = "s3cr&t"
	c.OAuthTokenURL = ts.URL + "/token"
	return c
}

func TestRetrieveToken(t *testing.T) {
	rctx := request.TestContext(t)

	t.Run("client credentials with basic auth and caching", func(t *testing.T) {
		ts := newTokenServer(t)
		ts.basicOnly = true
		s := newService(newFakeStore())
		c := ts.conn()

		token, appErr := s.RetrieveTokenForConnection(rctx, c)
		require.Nil(t, appErr)
		assert.Equal(t, "Bearer token-client_credentials", token.AsHeaderValue())

		_, appErr = s.RetrieveTokenForConnection(rctx, c)
		require.Nil(t, appErr)
		assert.EqualValues(t, 1, ts.requests.Load())

		// Changing the credentials invalidates the cached token.
		c2 := *c
		c2.ClientSecret = "wrong"
		_, appErr = s.RetrieveTokenForConnection(rctx, &c2)
		require.NotNil(t, appErr)
		assert.Equal(t, http.StatusBadRequest, appErr.StatusCode)
	})

	t.Run("falls back to client_secret_post", func(t *testing.T) {
		ts := newTokenServer(t)
		ts.postOnly = true
		s := newService(newFakeStore())
		token, appErr := s.RetrieveTokenForConnection(rctx, ts.conn())
		require.Nil(t, appErr)
		assert.Equal(t, "token-client_credentials", token.AccessToken)
		assert.EqualValues(t, 2, ts.requests.Load())

		// The style is remembered.
		c := ts.conn()
		_, appErr = s.RetrieveTokenForConnection(rctx, c)
		require.Nil(t, appErr)
		assert.EqualValues(t, 3, ts.requests.Load())
	})

	t.Run("password grant", func(t *testing.T) {
		ts := newTokenServer(t)
		s := newService(newFakeStore())
		c := ts.conn()
		c.GrantType = model.OutgoingOAuthConnectionGrantTypePassword
		c.CredentialsUsername = model.NewPointer("alice")
		c.CredentialsPassword = model.NewPointer("pw")
		token, appErr := s.RetrieveTokenForConnection(rctx, c)
		require.Nil(t, appErr)
		assert.Equal(t, "token-password", token.AccessToken)

		c.CredentialsPassword = model.NewPointer("bad")
		_, appErr = s.RetrieveTokenForConnection(rctx, c)
		require.NotNil(t, appErr)
		assert.Contains(t, appErr.Error(), "bad password")

		c.CredentialsPassword = nil
		_, appErr = s.RetrieveTokenForConnection(rctx, c)
		require.NotNil(t, appErr)
	})

	t.Run("expiry", func(t *testing.T) {
		ts := newTokenServer(t)
		ts.expiresIn = "120"
		s := newService(newFakeStore())
		now := time.Now()
		s.tokens.now = func() time.Time { return now }
		c := ts.conn()

		_, appErr := s.RetrieveTokenForConnection(rctx, c)
		require.Nil(t, appErr)
		now = now.Add(80 * time.Second)
		_, appErr = s.RetrieveTokenForConnection(rctx, c)
		require.Nil(t, appErr)
		assert.EqualValues(t, 1, ts.requests.Load())
		now = now.Add(20 * time.Second)
		_, appErr = s.RetrieveTokenForConnection(rctx, c)
		require.Nil(t, appErr)
		assert.EqualValues(t, 2, ts.requests.Load())
	})

	t.Run("validation of a stored connection fills in the secrets", func(t *testing.T) {
		ts := newTokenServer(t)
		st := newFakeStore()
		s := newService(st)
		c := ts.conn()
		c.Id = ""
		saved, appErr := s.SaveConnection(rctx, c)
		require.Nil(t, appErr)

		sanitized := *saved
		sanitized.Sanitize()
		token, appErr := s.RetrieveTokenForConnection(rctx, &sanitized)
		require.Nil(t, appErr)
		assert.NotEmpty(t, token.AccessToken)
	})

	t.Run("update forgets cached tokens", func(t *testing.T) {
		ts := newTokenServer(t)
		st := newFakeStore()
		s := newService(st)
		c := ts.conn()
		c.Id = ""
		saved, appErr := s.SaveConnection(rctx, c)
		require.Nil(t, appErr)
		_, appErr = s.RetrieveTokenForConnection(rctx, saved)
		require.Nil(t, appErr)
		_, appErr = s.UpdateConnection(rctx, saved)
		require.Nil(t, appErr)
		_, appErr = s.RetrieveTokenForConnection(rctx, saved)
		require.Nil(t, appErr)
		assert.EqualValues(t, 2, ts.requests.Load())
	})

	t.Run("server error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()
		s := newService(newFakeStore())
		c := newConn("c", "https://x.example.com")
		c.OAuthTokenURL = srv.URL
		_, appErr := s.RetrieveTokenForConnection(rctx, c)
		require.NotNil(t, appErr)
		assert.True(t, strings.Contains(appErr.Error(), "500"))
	})

	t.Run("form encoded response", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/x-www-form-urlencoded")
			_, _ = w.Write([]byte("access_token=abc&token_type=bearer&expires_in=60"))
		}))
		defer srv.Close()
		s := newService(newFakeStore())
		c := newConn("c", "https://x.example.com")
		c.OAuthTokenURL = srv.URL
		token, appErr := s.RetrieveTokenForConnection(rctx, c)
		require.Nil(t, appErr)
		assert.Equal(t, "Bearer abc", token.AsHeaderValue())
	})
}
