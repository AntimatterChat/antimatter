// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package api4

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

const apiAITestKey = "sk-apitest-0123456789abcdefwxyz"

// newAIEndpoint is an OpenAI-compatible endpoint that lists one model and answers chats.
func newAIEndpoint(t *testing.T) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/models") {
			fmt.Fprint(w, `{"data":[{"id":"test-model"}]}`)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"content":"Hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`)
	}))
	t.Cleanup(server.Close)
	return server
}

func aiConnectionRequest(scope, url string) *model.AIConnectionRequest {
	return &model.AIConnectionRequest{
		Name:         "Lumen " + model.NewId()[:6],
		Scope:        scope,
		Provider:     model.AIProviderOpenAICompatible,
		BaseURL:      url + "/v1",
		APIKey:       apiAITestKey,
		ExtraHeaders: map[string]string{"X-Org": "header-secret-value"},
		Models:       model.AIConnectionModels{Chat: "test-model"},
		Audience:     model.AIConnectionAudience{Everyone: true},
	}
}

// rawBody gets an API answer as text, whatever its status, to check what it carries.
func rawBody(t *testing.T, client *model.Client4, method, path string) string {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, client.APIURL+path, nil)
	require.NoError(t, err)
	req.Header.Set(model.HeaderAuth, client.AuthType+" "+client.AuthToken)
	req.Header.Set(model.HeaderRequestedWith, model.HeaderRequestedWithXML)
	resp, err := client.HTTPClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Less(t, resp.StatusCode, 300, "%s %s: %s", method, path, data)
	return string(data)
}

type aiTestClients struct {
	systemAdmin, communityAdmin, otherAdmin, member, guest *model.Client4
	otherTeam                                              *model.Team
}

func setupAIConnectionsAPI(t *testing.T) (*TestHelper, aiTestClients) {
	th := Setup(t).InitBasic(t)
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.AllowedUntrustedInternalConnections = "127.0.0.1"
		*cfg.GuestAccountsSettings.Enable = true
	})

	th.LinkUserToTeam(t, th.TeamAdminUser, th.BasicTeam)
	th.UpdateUserToTeamAdmin(t, th.TeamAdminUser, th.BasicTeam)
	communityAdmin := th.CreateClient()
	th.LoginTeamAdminWithClient(t, communityAdmin)

	otherTeam := th.CreateTeam(t)
	otherAdminUser := th.CreateUser(t)
	th.LinkUserToTeam(t, otherAdminUser, otherTeam)
	th.UpdateUserToTeamAdmin(t, otherAdminUser, otherTeam)
	otherAdmin := th.CreateClient()
	_, _, err := otherAdmin.Login(context.Background(), otherAdminUser.Email, otherAdminUser.Password)
	require.NoError(t, err)

	// The guest is in the basic team.
	_, guest := th.CreateGuestAndClient(t)

	return th, aiTestClients{
		systemAdmin:    th.SystemAdminClient,
		communityAdmin: communityAdmin,
		otherAdmin:     otherAdmin,
		member:         th.Client,
		guest:          guest,
		otherTeam:      otherTeam,
	}
}

func TestAIConnectionsPermissions(t *testing.T) {
	th, clients := setupAIConnectionsAPI(t)
	endpoint := newAIEndpoint(t)
	ctx := context.Background()

	t.Run("server-wide connections: system admins only", func(t *testing.T) {
		for name, client := range map[string]*model.Client4{"community admin": clients.communityAdmin, "member": clients.member, "guest": clients.guest} {
			_, resp, err := client.CreateAIConnection(ctx, aiConnectionRequest(model.AIConnectionScopeServer, endpoint.URL))
			require.Error(t, err, name)
			CheckForbiddenStatus(t, resp)
			_, resp, err = client.GetAIConnections(ctx, model.AIConnectionScopeServer)
			require.Error(t, err, name)
			CheckForbiddenStatus(t, resp)
		}

		created, resp, err := clients.systemAdmin.CreateAIConnection(ctx, aiConnectionRequest(model.AIConnectionScopeServer, endpoint.URL))
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)
		assert.True(t, created.APIKeySet)
		assert.Equal(t, "wxyz", created.APIKeyLast4)

		for name, client := range map[string]*model.Client4{"community admin": clients.communityAdmin, "member": clients.member, "guest": clients.guest} {
			_, resp, err = client.GetAIConnection(ctx, created.Id)
			require.Error(t, err, name)
			CheckForbiddenStatus(t, resp)
			_, resp, err = client.PatchAIConnection(ctx, created.Id, &model.AIConnectionPatch{Name: model.NewPointer("Taken")})
			require.Error(t, err, name)
			CheckForbiddenStatus(t, resp)
			resp, err = client.DeleteAIConnection(ctx, created.Id)
			require.Error(t, err, name)
			CheckForbiddenStatus(t, resp)
			_, resp, err = client.TestAIConnection(ctx, created.Id)
			require.Error(t, err, name)
			CheckForbiddenStatus(t, resp)
			_, resp, err = client.GetAIConnectionModels(ctx, created.Id)
			require.Error(t, err, name)
			CheckForbiddenStatus(t, resp)
			_, resp, err = client.GetAIConnectionUsage(ctx, created.Id)
			require.Error(t, err, name)
			CheckForbiddenStatus(t, resp)
		}

		got, _, err := clients.systemAdmin.GetAIConnection(ctx, created.Id)
		require.NoError(t, err)
		assert.Equal(t, created.Name, got.Name)
		result, _, err := clients.systemAdmin.TestAIConnection(ctx, created.Id)
		require.NoError(t, err)
		assert.True(t, result.OK, result.Error)
		models, _, err := clients.systemAdmin.GetAIConnectionModels(ctx, created.Id)
		require.NoError(t, err)
		require.Len(t, models, 1)
		_, _, err = clients.systemAdmin.GetAIConnectionUsage(ctx, created.Id)
		require.NoError(t, err)
		_, _, err = clients.systemAdmin.PatchAIConnection(ctx, created.Id, &model.AIConnectionPatch{Name: model.NewPointer("Renamed")})
		require.NoError(t, err)
		_, err = clients.systemAdmin.DeleteAIConnection(ctx, created.Id)
		require.NoError(t, err)
		_, resp, err = clients.systemAdmin.GetAIConnection(ctx, created.Id)
		require.Error(t, err)
		CheckNotFoundStatus(t, resp)
	})

	t.Run("a community's connections: its admins, and only theirs", func(t *testing.T) {
		for name, client := range map[string]*model.Client4{"another community's admin": clients.otherAdmin, "member": clients.member, "guest": clients.guest} {
			_, resp, err := client.CreateAIConnection(ctx, aiConnectionRequest(th.BasicTeam.Id, endpoint.URL))
			require.Error(t, err, name)
			CheckForbiddenStatus(t, resp)
		}

		created, resp, err := clients.communityAdmin.CreateAIConnection(ctx, aiConnectionRequest(th.BasicTeam.Id, endpoint.URL))
		require.NoError(t, err)
		CheckCreatedStatus(t, resp)

		list, _, err := clients.communityAdmin.GetAIConnections(ctx, th.BasicTeam.Id)
		require.NoError(t, err)
		require.Len(t, list, 1)
		list, _, err = clients.communityAdmin.GetAIConnections(ctx, "")
		require.NoError(t, err)
		for _, c := range list {
			assert.Equal(t, th.BasicTeam.Id, c.Scope, "a community admin sees their community's only")
		}
		_, resp, err = clients.communityAdmin.GetAIConnections(ctx, clients.otherTeam.Id)
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)

		for name, client := range map[string]*model.Client4{"another community's admin": clients.otherAdmin, "member": clients.member, "guest": clients.guest} {
			_, resp, err = client.GetAIConnection(ctx, created.Id)
			require.Error(t, err, name)
			CheckForbiddenStatus(t, resp)
			_, resp, err = client.PatchAIConnection(ctx, created.Id, &model.AIConnectionPatch{APIKey: model.NewPointer("sk-stolen-0000000000")})
			require.Error(t, err, name)
			CheckForbiddenStatus(t, resp)
			resp, err = client.DeleteAIConnection(ctx, created.Id)
			require.Error(t, err, name)
			CheckForbiddenStatus(t, resp)
		}

		_, resp, err = clients.communityAdmin.PatchAIConnection(ctx, created.Id, &model.AIConnectionPatch{Sharing: &model.AIConnectionSharing{AllTeams: true}})
		require.Error(t, err, "a community's connection isn't shared with others")
		CheckBadRequestStatus(t, resp)

		_, _, err = clients.systemAdmin.GetAIConnection(ctx, created.Id)
		require.NoError(t, err, "system admins manage every community's too")
		_, err = clients.communityAdmin.DeleteAIConnection(ctx, created.Id)
		require.NoError(t, err)
	})

	t.Run("testing before saving needs the scope's permission", func(t *testing.T) {
		_, resp, err := clients.member.TestAIConnectionRequest(ctx, aiConnectionRequest(model.AIConnectionScopeServer, endpoint.URL))
		require.Error(t, err)
		CheckForbiddenStatus(t, resp)
		result, _, err := clients.communityAdmin.TestAIConnectionRequest(ctx, aiConnectionRequest(th.BasicTeam.Id, endpoint.URL))
		require.NoError(t, err)
		assert.True(t, result.OK, result.Error)
		list, _, err := clients.systemAdmin.GetAIConnections(ctx, th.BasicTeam.Id)
		require.NoError(t, err)
		assert.Empty(t, list, "a test saves nothing")
	})

	t.Run("anyone lists what they may use", func(t *testing.T) {
		shared, _, err := clients.systemAdmin.CreateAIConnection(ctx, func() *model.AIConnectionRequest {
			req := aiConnectionRequest(model.AIConnectionScopeServer, endpoint.URL)
			req.Sharing = model.AIConnectionSharing{AllTeams: true}
			return req
		}())
		require.NoError(t, err)
		infos, _, err := clients.member.GetUsableAIConnections(ctx, th.BasicChannel.Id, "")
		require.NoError(t, err)
		ids := []string{}
		for _, info := range infos {
			ids = append(ids, info.Id)
		}
		assert.Contains(t, ids, shared.Id)
		infos, _, err = clients.guest.GetUsableAIConnections(ctx, "", th.BasicTeam.Id)
		require.NoError(t, err)
		assert.Empty(t, infos, "guests aren't in the audience unless allowed")
	})
}

func TestAIConnectionsNeverGiveTheKey(t *testing.T) {
	th, clients := setupAIConnectionsAPI(t)
	endpoint := newAIEndpoint(t)
	ctx := context.Background()

	created, _, err := clients.systemAdmin.CreateAIConnection(ctx, aiConnectionRequest(model.AIConnectionScopeServer, endpoint.URL))
	require.NoError(t, err)
	community, _, err := clients.communityAdmin.CreateAIConnection(ctx, aiConnectionRequest(th.BasicTeam.Id, endpoint.URL))
	require.NoError(t, err)

	for _, path := range []string{
		"/ai/connections",
		"/ai/connections?scope=server",
		"/ai/connections/" + created.Id,
		"/ai/connections/" + community.Id,
		"/ai/connections/" + created.Id + "/usage",
		"/ai/connections/usable?team_id=" + th.BasicTeam.Id,
	} {
		client := clients.systemAdmin
		if strings.Contains(path, "/usable") {
			// The system admin isn't in the community: its admin lists what they may use there.
			client = clients.communityAdmin
		}
		body := rawBody(t, client, http.MethodGet, path)
		assert.NotContains(t, body, apiAITestKey, path)
		assert.NotContains(t, body, "header-secret-value", path)
		assert.NotContains(t, body, "sealed", path)
	}
	body := rawBody(t, clients.systemAdmin, http.MethodPost, "/ai/connections/"+created.Id+"/test")
	assert.NotContains(t, body, apiAITestKey)

	patched, _, err := clients.systemAdmin.PatchAIConnection(ctx, created.Id, &model.AIConnectionPatch{Name: model.NewPointer("Renamed")})
	require.NoError(t, err)
	assert.True(t, patched.APIKeySet, "a patch without a key keeps it")
	assert.Equal(t, []string{"X-Org"}, patched.ExtraHeaderNames)

	// The config the console reads doesn't carry them either, nor do the logs.
	cfg, _, err := clients.systemAdmin.GetConfig(ctx)
	require.NoError(t, err)
	assert.NotContains(t, fmt.Sprintf("%+v", cfg.AISettings), apiAITestKey)
	assert.NotContains(t, th.LogBuffer.String(), apiAITestKey)
	assert.NotContains(t, th.LogBuffer.String(), "header-secret-value")
}
