// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

func aiMCPServerRequest(scope string) *model.AIMCPServerRequest {
	req := &model.AIMCPServerRequest{
		Name:              "Tracker",
		Slug:              "t" + strings.ToLower(model.NewId()[:12]),
		Scope:             scope,
		URL:               "https://mcp.example.com/mcp",
		Auth:              model.AIMCPAuthOAuth,
		OAuthClientId:     "client-id",
		OAuthClientSecret: "client-secret-value-1234",
	}
	if scope == model.AIConnectionScopeServer {
		req.Sharing.AllTeams = true
	}
	return req
}

// MCP servers are managed like model connections: system admins the server-wide ones, a community's admins their
// own; and no answer carries a secret.
func TestAIMCPServersPermissions(t *testing.T) {
	th, clients := setupAIConnectionsAPI(t)
	ctx := context.Background()

	t.Run("server-wide: system admins only", func(t *testing.T) {
		for name, client := range map[string]*model.Client4{"community admin": clients.communityAdmin, "member": clients.member, "guest": clients.guest} {
			_, resp, err := client.CreateAIMCPServer(ctx, aiMCPServerRequest(model.AIConnectionScopeServer))
			require.Error(t, err, name)
			CheckForbiddenStatus(t, resp)
			_, resp, err = client.GetAIMCPServers(ctx, model.AIConnectionScopeServer)
			require.Error(t, err, name)
			CheckForbiddenStatus(t, resp)
		}
		created, resp, err := clients.systemAdmin.CreateAIMCPServer(ctx, aiMCPServerRequest(model.AIConnectionScopeServer))
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
		assert.True(t, created.OAuthClientSecretSet)

		for name, client := range map[string]*model.Client4{"community admin": clients.communityAdmin, "member": clients.member} {
			_, resp, err = client.GetAIMCPServer(ctx, created.Id)
			require.Error(t, err, name)
			CheckForbiddenStatus(t, resp)
			_, resp, err = client.PatchAIMCPServer(ctx, created.Id, &model.AIMCPServerPatch{Name: model.NewPointer("Taken")})
			require.Error(t, err, name)
			CheckForbiddenStatus(t, resp)
			resp, err = client.DeleteAIMCPServer(ctx, created.Id)
			require.Error(t, err, name)
			CheckForbiddenStatus(t, resp)
		}

		for _, path := range []string{"/ai/mcp_servers/" + created.Id, "/ai/mcp_servers?scope=server", "/ai/mcp_servers"} {
			body := rawBody(t, clients.systemAdmin, http.MethodGet, path)
			assert.NotContains(t, body, "client-secret-value", "%s gives the OAuth client's secret", path)
		}
	})

	t.Run("a community's: its own admins only", func(t *testing.T) {
		created, _, err := clients.communityAdmin.CreateAIMCPServer(ctx, aiMCPServerRequest(th.BasicTeam.Id))
		require.NoError(t, err)

		_, resp, err := clients.otherAdmin.GetAIMCPServer(ctx, created.Id)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		_, resp, err = clients.otherAdmin.CreateAIMCPServer(ctx, aiMCPServerRequest(th.BasicTeam.Id))
		require.Error(t, err, "another community's admin can't add one here")
		CheckForbiddenStatus(t, resp)
		_, resp, err = clients.member.CreateAIMCPServer(ctx, aiMCPServerRequest(th.BasicTeam.Id))
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		withHeaders := aiMCPServerRequest(th.BasicTeam.Id)
		withHeaders.Auth = model.AIMCPAuthServiceAccount
		withHeaders.Headers = map[string]string{"Authorization": "Bearer shared-account-token-5678"}
		service, _, err := clients.communityAdmin.CreateAIMCPServer(ctx, withHeaders)
		require.NoError(t, err)
		assert.Equal(t, []string{"Authorization"}, service.HeaderNames)
		body := rawBody(t, clients.communityAdmin, http.MethodGet, "/ai/mcp_servers/"+service.Id)
		assert.NotContains(t, body, "shared-account-token", "a service account's headers are write-only")
	})

	t.Run("a short name is unique", func(t *testing.T) {
		req := aiMCPServerRequest(model.AIConnectionScopeServer)
		_, _, err := clients.systemAdmin.CreateAIMCPServer(ctx, req)
		require.NoError(t, err)
		_, resp, err := clients.systemAdmin.CreateAIMCPServer(ctx, req)
		require.Error(t, err)
		assert.Equal(t, http.StatusConflict, resp.StatusCode)
	})
}

// A person signs in and out for themselves only; nobody can sign someone else out, nor read a sign-in.
func TestAIMCPSignInsAreThePersons(t *testing.T) {
	th, clients := setupAIConnectionsAPI(t)
	ctx := context.Background()
	created, _, err := clients.systemAdmin.CreateAIMCPServer(ctx, aiMCPServerRequest(model.AIConnectionScopeServer))
	require.NoError(t, err)

	sealed := "not-a-real-sealed-token"
	require.NoError(t, th.App.Srv().Store().AIMCPServer().SaveUserToken(th.Context, &model.AIMCPUserToken{ServerId: created.Id, UserId: th.BasicUser.Id, TokenSealed: sealed}))

	_, err = clients.systemAdmin.DisconnectAIMCPServer(ctx, created.Id)
	require.NoError(t, err, "an admin signs themselves out")
	_, err = th.App.Srv().Store().AIMCPServer().GetUserToken(th.Context, created.Id, th.BasicUser.Id)
	require.NoError(t, err, "and nobody else")

	for _, path := range []string{"/ai/mcp_servers/" + created.Id, "/ai/mcp_servers"} {
		assert.NotContains(t, rawBody(t, clients.systemAdmin, http.MethodGet, path), sealed, "no API gives a sign-in, even sealed")
	}

	_, err = clients.member.DisconnectAIMCPServer(ctx, created.Id)
	require.NoError(t, err)
	_, err = th.App.Srv().Store().AIMCPServer().GetUserToken(th.Context, created.Id, th.BasicUser.Id)
	require.Error(t, err, "the person's own sign-out")
}

// A community admin may edit their community's MCP servers but not read their secrets: moving a server to another
// address over two patches, through another sign-in kind and back, mustn't send its saved secrets there.
func TestAIMCPServerSecretsDontFollowAKindSwitch(t *testing.T) {
	th, clients := setupAIConnectionsAPI(t)
	ctx := context.Background()
	elsewhere := "https://elsewhere.example.com/mcp"

	t.Run("service account headers", func(t *testing.T) {
		req := aiMCPServerRequest(th.BasicTeam.Id)
		req.Auth = model.AIMCPAuthServiceAccount
		req.OAuthClientId = ""
		req.OAuthClientSecret = ""
		req.Headers = map[string]string{"Authorization": "Bearer shared-account-token-5678"}
		created, _, err := clients.communityAdmin.CreateAIMCPServer(ctx, req)
		require.NoError(t, err)

		moved, _, err := clients.communityAdmin.PatchAIMCPServer(ctx, created.Id, &model.AIMCPServerPatch{Auth: model.NewPointer(model.AIMCPAuthNone), URL: &elsewhere})
		require.NoError(t, err, "a server without sign-in may go anywhere")
		assert.Empty(t, moved.HeaderNames, "without the headers")

		_, resp, err := clients.communityAdmin.PatchAIMCPServer(ctx, created.Id, &model.AIMCPServerPatch{Auth: model.NewPointer(model.AIMCPAuthServiceAccount)})
		require.Error(t, err, "back to a service account needs its headers again")
		CheckBadRequestStatus(t, resp)

		stored, appErr := th.App.GetAIMCPServer(th.Context, created.Id)
		require.Nil(t, appErr)
		assert.Equal(t, elsewhere, stored.URL)
		assert.Empty(t, stored.HeadersSealed, "the old headers are gone")
	})

	t.Run("a client secret typed in", func(t *testing.T) {
		created, _, err := clients.communityAdmin.CreateAIMCPServer(ctx, aiMCPServerRequest(th.BasicTeam.Id))
		require.NoError(t, err)
		require.True(t, created.OAuthClientSecretSet)

		_, _, err = clients.communityAdmin.PatchAIMCPServer(ctx, created.Id, &model.AIMCPServerPatch{
			Auth:    model.NewPointer(model.AIMCPAuthServiceAccount),
			URL:     &elsewhere,
			Headers: &map[string]string{"X-Mine": "my-own-value"},
		})
		require.NoError(t, err, "a service account with its own headers may go anywhere")

		back, _, err := clients.communityAdmin.PatchAIMCPServer(ctx, created.Id, &model.AIMCPServerPatch{Auth: model.NewPointer(model.AIMCPAuthOAuth)})
		require.NoError(t, err)
		assert.False(t, back.OAuthClientSecretSet, "the old client secret didn't come back with OAuth")
		assert.Empty(t, back.HeaderNames, "nor do the headers stay")

		stored, appErr := th.App.GetAIMCPServer(th.Context, created.Id)
		require.Nil(t, appErr)
		assert.Equal(t, elsewhere, stored.URL)
		assert.Empty(t, stored.OAuthClientSecretSealed)
		assert.Empty(t, stored.HeadersSealed)
	})

	t.Run("the same kind keeps its secrets", func(t *testing.T) {
		created, _, err := clients.communityAdmin.CreateAIMCPServer(ctx, aiMCPServerRequest(th.BasicTeam.Id))
		require.NoError(t, err)
		renamed, _, err := clients.communityAdmin.PatchAIMCPServer(ctx, created.Id, &model.AIMCPServerPatch{Name: model.NewPointer("Renamed")})
		require.NoError(t, err)
		assert.True(t, renamed.OAuthClientSecretSet)
	})
}

// The page an authorisation server sends people back to needs their session, and refuses a sign-in that isn't theirs.
func TestAIMCPOAuthCallback(t *testing.T) {
	_, clients := setupAIConnectionsAPI(t)

	resp, err := http.Get(clients.member.APIURL + "/ai/mcp/oauth/callback?state=x&code=y")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "no session, no sign-in")

	req, err := http.NewRequest(http.MethodGet, clients.member.APIURL+"/ai/mcp/oauth/callback?state="+model.NewId()+model.NewId()+"&code=y", nil)
	require.NoError(t, err)
	req.Header.Set(model.HeaderAuth, clients.member.AuthType+" "+clients.member.AuthToken)
	resp, err = clients.member.HTTPClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "text/html")
	assert.Equal(t, "no-store", resp.Header.Get("Cache-Control"))
	assert.Contains(t, string(body), "sign in", "a page in the person's language, not JSON")
}

// A person's tools for an agent: only an agent they may use; and only they may answer a call proposed to them.
func TestAIAgentToolsAPI(t *testing.T) {
	th, clients := setupAIConnectionsAPI(t)
	ctx := context.Background()
	endpoint := newAIEndpoint(t)
	connection, _, err := clients.systemAdmin.CreateAIConnection(ctx, func() *model.AIConnectionRequest {
		req := aiConnectionRequest(model.AIConnectionScopeServer, endpoint.URL)
		req.Sharing.AllTeams = true
		return req
	}())
	require.NoError(t, err)

	agent, appErr := th.App.CreateAIAgent(th.Context, &model.AIAgentRequest{
		Username: "tools" + strings.ToLower(model.NewId()[:8]), DisplayName: "Tools", Scope: model.AIAgentScopeServer,
		ConnectionId: connection.Id, Model: "test-model",
		Tools:  model.AIAgentToolPolicy{Enabled: true, Allowed: []string{"antimatter.*"}, AutoRun: []string{"antimatter.search_messages"}},
		Access: model.AIAgentAccess{Roles: []string{model.SystemAdminRoleId}},
	}, th.SystemAdminUser.Id)
	require.Nil(t, appErr)

	_, resp, err := clients.member.GetAIAgentTools(ctx, agent.UserId, "")
	require.Error(t, err, "an agent the person may not use")
	CheckForbiddenStatus(t, resp)

	sources, _, err := clients.systemAdmin.GetAIAgentTools(ctx, agent.UserId, "")
	require.NoError(t, err)
	require.NotEmpty(t, sources)
	assert.Equal(t, model.AIToolSourceBuiltIn, sources[0].Key)
	autoRun := map[string]bool{}
	for _, tool := range sources[0].Tools {
		autoRun[tool.Name] = tool.AutoRun
	}
	assert.True(t, autoRun["antimatter.search_messages"])
	assert.False(t, autoRun["antimatter.post_message"], "a write never runs on its own")

	_, resp, err = clients.member.AnswerAIToolCall(ctx, model.NewId(), true)
	require.Error(t, err)
	CheckNotFoundStatus(t, resp)

	t.Run("the catalogue is for the agents' managers", func(t *testing.T) {
		_, resp, err := clients.member.GetAIToolCatalogue(ctx, model.AIAgentScopeServer)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		catalogue, _, err := clients.systemAdmin.GetAIToolCatalogue(ctx, model.AIAgentScopeServer)
		require.NoError(t, err)
		require.NotEmpty(t, catalogue.Sources)
		assert.Equal(t, model.AIToolBuiltInNamespace, catalogue.Sources[0].Namespace)
		_, resp, err = clients.otherAdmin.GetAIToolCatalogue(ctx, th.BasicTeam.Id)
		require.Error(t, err, "another community's admin")
		CheckForbiddenStatus(t, resp)
	})
}
