// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package storetest

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

func TestAIMCPServerStore(t *testing.T, rctx request.CTX, ss store.Store) {
	t.Run("SaveGetUpdate", func(t *testing.T) { testAIMCPServerSaveGetUpdate(t, rctx, ss) })
	t.Run("SlugIsUnique", func(t *testing.T) { testAIMCPServerSlugIsUnique(t, rctx, ss) })
	t.Run("Scopes", func(t *testing.T) { testAIMCPServerScopes(t, rctx, ss) })
	t.Run("UserTokens", func(t *testing.T) { testAIMCPServerUserTokens(t, rctx, ss) })
	t.Run("DeleteForgetsSignIns", func(t *testing.T) { testAIMCPServerDeleteForgetsSignIns(t, rctx, ss) })
}

func newTestAIMCPServer(scope string) *model.AIMCPServer {
	return &model.AIMCPServer{
		Name:  "Jira",
		Slug:  "s" + model.NewId()[:20],
		Scope: scope,
		URL:   "https://mcp.example.com/mcp",
		Auth:  model.AIMCPAuthOAuth,
	}
}

func testAIMCPServerSaveGetUpdate(t *testing.T, rctx request.CTX, ss store.Store) {
	s := newTestAIMCPServer(model.AIConnectionScopeServer)
	s.OAuthClientId = "client"
	s.OAuthClientSecretSealed = "sealed-secret"
	s.OAuthScopes = " read  write "
	s.Sharing = model.AIConnectionSharing{TeamIDs: []string{model.NewId()}}
	saved, err := ss.AIMCPServer().Save(rctx, s)
	require.NoError(t, err)
	require.True(t, model.IsValidId(saved.Id))

	got, err := ss.AIMCPServer().Get(rctx, saved.Id)
	require.NoError(t, err)
	assert.Equal(t, "client", got.OAuthClientId)
	assert.Equal(t, "sealed-secret", got.OAuthClientSecretSealed)
	assert.True(t, got.OAuthClientSecretSet)
	assert.Equal(t, "read write", got.OAuthScopes)
	assert.Equal(t, s.Sharing.TeamIDs, got.Sharing.TeamIDs)
	assert.Equal(t, []string{}, got.HeaderNames)

	got.Name = "Jira Cloud"
	got.Slug = "changed"
	got.Auth = model.AIMCPAuthServiceAccount
	got.HeadersSealed = "sealed-headers"
	got.HeaderNames = []string{"Authorization"}
	_, err = ss.AIMCPServer().Update(rctx, got)
	require.NoError(t, err)

	again, err := ss.AIMCPServer().Get(rctx, saved.Id)
	require.NoError(t, err)
	assert.Equal(t, "Jira Cloud", again.Name)
	assert.Equal(t, saved.Slug, again.Slug, "the slug never changes")
	assert.Equal(t, "sealed-headers", again.HeadersSealed)
	assert.Equal(t, []string{"Authorization"}, again.HeaderNames)
	assert.Empty(t, again.OAuthClientSecretSealed, "a service account has no OAuth client")

	_, err = ss.AIMCPServer().Get(rctx, model.NewId())
	var nf *store.ErrNotFound
	require.True(t, errors.As(err, &nf))
}

func testAIMCPServerSlugIsUnique(t *testing.T, rctx request.CTX, ss store.Store) {
	first := newTestAIMCPServer(model.AIConnectionScopeServer)
	_, err := ss.AIMCPServer().Save(rctx, first)
	require.NoError(t, err)

	second := newTestAIMCPServer(model.NewId())
	second.Slug = first.Slug
	_, err = ss.AIMCPServer().Save(rctx, second)
	var conflict *store.ErrConflict
	require.True(t, errors.As(err, &conflict), "two MCP servers can't name their tools alike")

	require.NoError(t, ss.AIMCPServer().Delete(rctx, first.Id, model.GetMillis()))
	second.Id = ""
	_, err = ss.AIMCPServer().Save(rctx, second)
	require.NoError(t, err, "a deleted server's slug is free again")
}

func testAIMCPServerScopes(t *testing.T, rctx request.CTX, ss store.Store) {
	team := model.NewId()
	server, err := ss.AIMCPServer().Save(rctx, newTestAIMCPServer(model.AIConnectionScopeServer))
	require.NoError(t, err)
	community, err := ss.AIMCPServer().Save(rctx, newTestAIMCPServer(team))
	require.NoError(t, err)
	other, err := ss.AIMCPServer().Save(rctx, newTestAIMCPServer(model.NewId()))
	require.NoError(t, err)

	list, err := ss.AIMCPServer().GetForScopes(rctx, []string{model.AIConnectionScopeServer, team})
	require.NoError(t, err)
	ids := []string{}
	for _, s := range list {
		ids = append(ids, s.Id)
	}
	assert.Contains(t, ids, server.Id)
	assert.Contains(t, ids, community.Id)
	assert.NotContains(t, ids, other.Id)

	none, err := ss.AIMCPServer().GetForScopes(rctx, nil)
	require.NoError(t, err)
	assert.Empty(t, none)

	require.NoError(t, ss.AIMCPServer().Delete(rctx, community.Id, model.GetMillis()))
	list, err = ss.AIMCPServer().GetForScopes(rctx, []string{team})
	require.NoError(t, err)
	assert.Empty(t, list)
	var nf *store.ErrNotFound
	require.True(t, errors.As(ss.AIMCPServer().Delete(rctx, community.Id, model.GetMillis()), &nf))
}

func testAIMCPServerUserTokens(t *testing.T, rctx request.CTX, ss store.Store) {
	server, err := ss.AIMCPServer().Save(rctx, newTestAIMCPServer(model.AIConnectionScopeServer))
	require.NoError(t, err)
	maya, sam := model.NewId(), model.NewId()

	require.NoError(t, ss.AIMCPServer().SaveUserToken(rctx, &model.AIMCPUserToken{ServerId: server.Id, UserId: maya, TokenSealed: "maya-1", ExpiresAt: 10}))
	require.NoError(t, ss.AIMCPServer().SaveUserToken(rctx, &model.AIMCPUserToken{ServerId: server.Id, UserId: maya, TokenSealed: "maya-2", ExpiresAt: 20}))
	require.Error(t, ss.AIMCPServer().SaveUserToken(rctx, &model.AIMCPUserToken{ServerId: server.Id, UserId: sam}), "no token, nothing to keep")

	token, err := ss.AIMCPServer().GetUserToken(rctx, server.Id, maya)
	require.NoError(t, err)
	assert.Equal(t, "maya-2", token.TokenSealed, "a new sign-in replaces the old one")
	assert.EqualValues(t, 20, token.ExpiresAt)

	_, err = ss.AIMCPServer().GetUserToken(rctx, server.Id, sam)
	var nf *store.ErrNotFound
	require.True(t, errors.As(err, &nf), "someone else's sign-in is never given for another person")

	ids, err := ss.AIMCPServer().GetUserTokenServerIds(rctx, maya)
	require.NoError(t, err)
	assert.Equal(t, []string{server.Id}, ids)
	ids, err = ss.AIMCPServer().GetUserTokenServerIds(rctx, sam)
	require.NoError(t, err)
	assert.Empty(t, ids)

	require.NoError(t, ss.AIMCPServer().DeleteUserToken(rctx, server.Id, maya))
	require.NoError(t, ss.AIMCPServer().DeleteUserToken(rctx, server.Id, maya), "again is fine")
	_, err = ss.AIMCPServer().GetUserToken(rctx, server.Id, maya)
	require.True(t, errors.As(err, &nf))

	require.NoError(t, ss.AIMCPServer().SaveUserToken(rctx, &model.AIMCPUserToken{ServerId: server.Id, UserId: sam, TokenSealed: "sam"}))
	require.NoError(t, ss.AIMCPServer().DeleteUserTokensForUser(rctx, sam))
	_, err = ss.AIMCPServer().GetUserToken(rctx, server.Id, sam)
	require.True(t, errors.As(err, &nf))
}

func testAIMCPServerDeleteForgetsSignIns(t *testing.T, rctx request.CTX, ss store.Store) {
	s := newTestAIMCPServer(model.AIConnectionScopeServer)
	s.OAuthClientSecretSealed = "sealed-secret"
	server, err := ss.AIMCPServer().Save(rctx, s)
	require.NoError(t, err)
	user := model.NewId()
	require.NoError(t, ss.AIMCPServer().SaveUserToken(rctx, &model.AIMCPUserToken{ServerId: server.Id, UserId: user, TokenSealed: "token"}))

	require.NoError(t, ss.AIMCPServer().Delete(rctx, server.Id, model.GetMillis()))
	_, err = ss.AIMCPServer().GetUserToken(rctx, server.Id, user)
	var nf *store.ErrNotFound
	require.True(t, errors.As(err, &nf), "deleting a server forgets everyone's sign-in to it")
	deleted, err := ss.AIMCPServer().Get(rctx, server.Id)
	require.NoError(t, err)
	assert.NotZero(t, deleted.DeleteAt)
	assert.Empty(t, deleted.OAuthClientSecretSealed, "and its secrets")
}
