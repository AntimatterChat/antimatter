// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package model

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validTestAIMCPServer() *AIMCPServer {
	s := &AIMCPServer{Name: "Jira", Slug: "jira", Scope: AIConnectionScopeServer, URL: "https://mcp.example.com/mcp", Auth: AIMCPAuthOAuth}
	s.PreSave()
	return s
}

func TestAIMCPServerIsValid(t *testing.T) {
	require.Nil(t, validTestAIMCPServer().IsValid())

	for name, change := range map[string]func(s *AIMCPServer){
		"no name":              func(s *AIMCPServer) { s.Name = "" },
		"long name":            func(s *AIMCPServer) { s.Name = strings.Repeat("a", AIMCPServerNameMaxRunes+1) },
		"bad slug":             func(s *AIMCPServer) { s.Slug = "Jira Cloud" },
		"dotted slug":          func(s *AIMCPServer) { s.Slug = "a.b" },
		"bad scope":            func(s *AIMCPServer) { s.Scope = "everywhere" },
		"no url":               func(s *AIMCPServer) { s.URL = "" },
		"ftp url":              func(s *AIMCPServer) { s.URL = "ftp://mcp.example.com" },
		"url with credentials": func(s *AIMCPServer) { s.URL = "https://user:pass@mcp.example.com" },
		"bad auth":             func(s *AIMCPServer) { s.Auth = "password" },
		"client id with space": func(s *AIMCPServer) { s.OAuthClientId = "a b" },
		"bad sharing":          func(s *AIMCPServer) { s.Sharing.TeamIDs = []string{"x"} },
		"no times":             func(s *AIMCPServer) { s.CreateAt = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			s := validTestAIMCPServer()
			change(s)
			assert.NotNil(t, s.IsValid())
		})
	}

	t.Run("a service account needs its headers", func(t *testing.T) {
		s := validTestAIMCPServer()
		s.Auth = AIMCPAuthServiceAccount
		s.PreUpdate()
		assert.NotNil(t, s.IsValid())
		s.HeadersSealed = "sealed"
		s.HeaderNames = []string{"Authorization"}
		s.PreUpdate()
		assert.Nil(t, s.IsValid())
	})
}

func TestAIMCPServerNormalize(t *testing.T) {
	t.Run("only a service account keeps headers", func(t *testing.T) {
		s := validTestAIMCPServer()
		s.HeadersSealed = "sealed"
		s.HeaderNames = []string{"Authorization"}
		s.PreUpdate()
		assert.Empty(t, s.HeadersSealed)
		assert.Empty(t, s.HeaderNames)
	})
	t.Run("only OAuth keeps a client", func(t *testing.T) {
		s := validTestAIMCPServer()
		s.OAuthClientId = "client"
		s.OAuthClientSecretSealed = "sealed"
		s.Auth = AIMCPAuthNone
		s.PreUpdate()
		assert.Empty(t, s.OAuthClientId)
		assert.Empty(t, s.OAuthClientSecretSealed)
		assert.False(t, s.OAuthClientSecretSet)
	})
	t.Run("a community's server has no sharing", func(t *testing.T) {
		s := validTestAIMCPServer()
		s.Scope = NewId()
		s.Sharing = AIConnectionSharing{AllTeams: true}
		s.PreUpdate()
		assert.False(t, s.Sharing.AllTeams)
	})
}

func TestAIMCPServerSharedWithTeam(t *testing.T) {
	team := NewId()
	s := validTestAIMCPServer()
	assert.False(t, s.SharedWithTeam(team))
	assert.False(t, s.SharedWithTeam(""))
	s.Sharing.TeamIDs = []string{team}
	assert.True(t, s.SharedWithTeam(team))
	assert.False(t, s.SharedWithTeam(""), "direct messages need a server shared with every community")
	s.Sharing.AllTeams = true
	assert.True(t, s.SharedWithTeam(""))

	community := validTestAIMCPServer()
	community.Scope = team
	assert.True(t, community.SharedWithTeam(team))
	assert.False(t, community.SharedWithTeam(NewId()))
	assert.False(t, community.SharedWithTeam(""))
}

func TestAIMCPSecretsNeverInJSON(t *testing.T) {
	s := validTestAIMCPServer()
	s.OAuthClientSecretSealed = "sealed-secret"
	s.HeadersSealed = "sealed-headers"
	data, err := json.Marshal(s)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "sealed-secret")
	assert.NotContains(t, string(data), "sealed-headers")

	token := &AIMCPUserToken{ServerId: NewId(), UserId: NewId(), TokenSealed: "sealed-token"}
	data, err = json.Marshal(token)
	require.NoError(t, err)
	assert.Equal(t, "{}", string(data), "a person's sign-in has nothing to give")

	for _, auditable := range []map[string]any{s.Auditable()} {
		data, err := json.Marshal(auditable)
		require.NoError(t, err)
		assert.NotContains(t, string(data), "sealed")
	}
}
