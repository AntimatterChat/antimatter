// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/ai"
	"github.com/mattermost/mattermost/server/public/model"
)

const aiTestKey = "sk-aitest-0123456789abcdefghij"

// fakeLLM is an OpenAI-compatible endpoint for the app's tests: it answers chats with a fixed answer and usage, and
// remembers each request's path, Authorization header and body.
type fakeLLM struct {
	*httptest.Server
	mu       sync.Mutex
	answer   string
	requests []fakeLLMRequest
	// failStatus, when set, makes chats fail with this status and failMessage.
	failStatus  int
	failMessage string
	// chunks, when set, are streamed (server-sent events) to the chats that ask for a stream.
	chunks []string
	// unavailable makes every request fail with 503, the list of models too (an endpoint that's down).
	unavailable bool
	// modelLists counts the requests for the list of models (a connection's test, also the agents' status checks
	// that run in the background); they aren't in requests.
	modelLists int
}

type fakeLLMRequest struct {
	Path          string
	Authorization string
	Body          map[string]any
}

func newFakeLLM(t *testing.T, answer string) *fakeLLM {
	f := &fakeLLM{answer: answer}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		f.mu.Lock()
		if strings.HasSuffix(r.URL.Path, "/models") {
			f.modelLists++
		} else {
			f.requests = append(f.requests, fakeLLMRequest{Path: r.URL.Path, Authorization: r.Header.Get("Authorization"), Body: body})
		}
		answer, failStatus, failMessage, chunks, unavailable := f.answer, f.failStatus, f.failMessage, f.chunks, f.unavailable
		f.mu.Unlock()
		switch {
		case unavailable:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, `{"error":{"message":"down for maintenance"}}`)
		case failStatus == 0 && len(chunks) > 0 && body["stream"] == true:
			w.Header().Set("Content-Type", "text/event-stream")
			flusher, _ := w.(http.Flusher)
			for _, chunk := range chunks {
				content, _ := json.Marshal(chunk)
				fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%s}}]}\n\n", content)
				if flusher != nil {
					flusher.Flush()
				}
			}
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
			fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":12,\"completion_tokens\":6}}\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
		case failStatus != 0 && strings.HasSuffix(r.URL.Path, "/chat/completions"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(failStatus)
			msg, _ := json.Marshal(failMessage)
			fmt.Fprintf(w, `{"error":{"message":%s}}`, msg)
		case strings.HasSuffix(r.URL.Path, "/models"):
			fmt.Fprint(w, `{"data":[{"id":"test-model"}]}`)
		case strings.HasSuffix(r.URL.Path, "/audio/speech"):
			w.Header().Set("Content-Type", "audio/mpeg")
			fmt.Fprint(w, "mp3")
		default:
			content, _ := json.Marshal(answer)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"model":"test-model","choices":[{"message":{"role":"assistant","content":%s},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`, content)
		}
	}))
	t.Cleanup(f.Close)
	return f
}

// streamIn makes the endpoint stream its answers in these pieces.
func (f *fakeLLM) streamIn(chunks ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.chunks = chunks
}

// fail makes the endpoint's chats fail.
func (f *fakeLLM) fail(status int, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failStatus, f.failMessage = status, message
}

// messages gives a chat request's messages as role and content.
func (r fakeLLMRequest) messages() []map[string]any {
	list, _ := r.Body["messages"].([]any)
	out := make([]map[string]any, 0, len(list))
	for _, m := range list {
		if message, ok := m.(map[string]any); ok {
			out = append(out, message)
		}
	}
	return out
}

// schemaName gives the name of the JSON schema a chat asked for, "" for none.
func (r fakeLLMRequest) schemaName() string {
	format, _ := r.Body["response_format"].(map[string]any)
	schema, _ := format["json_schema"].(map[string]any)
	name, _ := schema["name"].(string)
	return name
}

// assertAIUsageFor checks that the model calls of a test were counted against a person.
func assertAIUsageFor(t *testing.T, th *TestHelper, userID string) {
	t.Helper()
	connections, err := th.App.Srv().Store().AIConnection().GetAll(th.Context)
	require.NoError(t, err)
	found := false
	for _, c := range connections {
		usage, appErr := th.App.GetAIConnectionUsage(th.Context, c)
		require.Nil(t, appErr)
		for _, day := range usage.Days {
			found = found || day.UserId == userID
		}
	}
	assert.True(t, found, "the call is counted against the person")
}

// setupAIFeatureModel gives a test server whose built-in features (rewrite, recaps, auto-translation) run on a
// server-wide connection to a fake model that answers answer.
func setupAIFeatureModel(t *testing.T, answer string) (*TestHelper, *fakeLLM, *model.AIConnection) {
	t.Helper()
	th := setupAITest(t)
	f := newFakeLLM(t, answer)
	c := createTestAIConnection(t, th, f, model.AIConnectionScopeServer, nil)
	th.App.UpdateConfig(func(cfg *model.Config) {
		cfg.AISettings.Rewrite.ConnectionId = model.NewPointer(c.Id)
		cfg.AISettings.Recaps.ConnectionId = model.NewPointer(c.Id)
		cfg.AISettings.AutoTranslation.ConnectionId = model.NewPointer(c.Id)
	})
	return th, f, c
}

// setUnavailable makes the endpoint down (every request fails) or up again.
func (f *fakeLLM) setUnavailable(down bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.unavailable = down
}

// modelListCount gives how many times the list of models was asked for.
func (f *fakeLLM) modelListCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.modelLists
}

func (f *fakeLLM) calls() []fakeLLMRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakeLLMRequest(nil), f.requests...)
}

// createTestAIConnection adds a connection to the fake endpoint, with the test key.
func createTestAIConnection(t *testing.T, th *TestHelper, f *fakeLLM, scope string, change func(req *model.AIConnectionRequest)) *model.AIConnection {
	t.Helper()
	req := &model.AIConnectionRequest{
		Name:     "Test " + model.NewId()[:6],
		Scope:    scope,
		Provider: model.AIProviderOpenAICompatible,
		BaseURL:  f.URL + "/v1",
		APIKey:   aiTestKey,
		Models:   model.AIConnectionModels{Chat: "test-model", Embeddings: "embed-model"},
		Audience: model.AIConnectionAudience{Everyone: true},
	}
	if scope == model.AIConnectionScopeServer {
		req.Sharing.AllTeams = true
	}
	if change != nil {
		change(req)
	}
	c, appErr := th.App.CreateAIConnection(th.Context, req, th.SystemAdminUser.Id)
	require.Nil(t, appErr)
	return c
}

func aiChat(th *TestHelper, call AICall, connectionID string) (*ai.ChatResponse, *model.AppError) {
	return th.App.AIChat(th.Context, call, ai.ChatRequest{ConnectionID: connectionID, Messages: []ai.Message{{Role: ai.RoleUser, Content: "Hello"}}})
}

func setupAITest(t *testing.T) *TestHelper {
	th := Setup(t).InitBasic(t)
	// The fake endpoints listen on 127.0.0.1, which community connections may only reach when allowed.
	th.App.UpdateConfig(func(cfg *model.Config) {
		*cfg.ServiceSettings.AllowedUntrustedInternalConnections = "127.0.0.1"
	})
	return th
}

func TestAIConnectionKeysAreSealed(t *testing.T) {
	th := setupAITest(t)
	f := newFakeLLM(t, "Hi")
	c := createTestAIConnection(t, th, f, model.AIConnectionScopeServer, func(req *model.AIConnectionRequest) {
		req.ExtraHeaders = map[string]string{"X-Org": "header-secret-value"}
	})

	assert.True(t, c.APIKeySet)
	assert.Equal(t, "ghij", c.APIKeyLast4)
	assert.Equal(t, []string{"X-Org"}, c.ExtraHeaderNames)

	stored, err := th.App.Srv().Store().AIConnection().Get(th.Context, c.Id)
	require.NoError(t, err)
	assert.NotContains(t, stored.APIKeySealed, aiTestKey, "the key is encrypted at rest")
	assert.NotContains(t, stored.ExtraHeadersSealed, "header-secret-value")

	data, err := json.Marshal(stored)
	require.NoError(t, err)
	assert.NotContains(t, string(data), aiTestKey)
	assert.NotContains(t, string(data), stored.APIKeySealed)

	// Calls use the key in clear, and the headers.
	_, appErr := aiChat(th, AICall{UserID: th.BasicUser.Id, ChannelID: th.BasicChannel.Id}, c.Id)
	require.Nil(t, appErr)
	assert.Equal(t, "Bearer "+aiTestKey, f.calls()[0].Authorization)

	t.Run("a sealed key moved to another connection doesn't open", func(t *testing.T) {
		other := createTestAIConnection(t, th, f, model.AIConnectionScopeServer, nil)
		other.APIKeySealed = stored.APIKeySealed
		_, err := th.App.Srv().Store().AIConnection().Update(th.Context, other)
		require.NoError(t, err)
		_, callErr := aiChat(th, AICall{UserID: th.BasicUser.Id}, other.Id)
		require.NotNil(t, callErr)
		assert.Equal(t, "app.ai.connection.key_unreadable.app_error", callErr.Id)
	})

	t.Run("patching keeps the key unless a new one is given", func(t *testing.T) {
		patched, appErr := th.App.PatchAIConnection(th.Context, c, &model.AIConnectionPatch{Name: model.NewPointer("Renamed")}, th.SystemAdminUser.Id)
		require.Nil(t, appErr)
		assert.True(t, patched.APIKeySet)
		assert.Equal(t, stored.APIKeySealed, patched.APIKeySealed)

		patched, appErr = th.App.PatchAIConnection(th.Context, patched, &model.AIConnectionPatch{APIKey: model.NewPointer("sk-new-key-0000000000wxyz")}, th.SystemAdminUser.Id)
		require.Nil(t, appErr)
		assert.Equal(t, "wxyz", patched.APIKeyLast4)

		patched, appErr = th.App.PatchAIConnection(th.Context, patched, &model.AIConnectionPatch{APIKey: model.NewPointer("")}, th.SystemAdminUser.Id)
		require.Nil(t, appErr)
		assert.False(t, patched.APIKeySet)
		assert.Empty(t, patched.APIKeyLast4)
	})

	assert.NotContains(t, th.LogBuffer.String(), aiTestKey, "the key never reaches the logs")
	assert.NotContains(t, th.LogBuffer.String(), "header-secret-value")
}

func TestAIProviderErrorsHideTheKey(t *testing.T) {
	th := setupAITest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, `{"error":{"message":"Incorrect API key provided: %s. You can find your API key..."}}`, aiTestKey)
	}))
	defer server.Close()
	c := createTestAIConnection(t, th, &fakeLLM{Server: server}, model.AIConnectionScopeServer, nil)

	_, appErr := aiChat(th, AICall{UserID: th.BasicUser.Id}, c.Id)
	require.NotNil(t, appErr)
	assert.Equal(t, "app.ai.call.provider_refused.app_error", appErr.Id)
	assert.NotContains(t, appErr.Error(), aiTestKey)

	result := th.App.TestAIConnection(th.Context, c)
	assert.False(t, result.OK)
	assert.Equal(t, http.StatusUnauthorized, result.StatusCode)
	assert.NotContains(t, result.Error, aiTestKey)
	assert.NotContains(t, th.LogBuffer.String(), aiTestKey)
}

func TestAIConnectionAudience(t *testing.T) {
	th := setupAITest(t)
	f := newFakeLLM(t, "Hi")

	otherTeam := th.CreateTeam(t)
	otherChannel := th.CreateChannel(t, otherTeam)
	th.LinkUserToTeam(t, th.BasicUser, otherTeam)
	th.AddUserToChannel(t, th.BasicUser, otherChannel)
	dm := th.CreateDmChannel(t, th.BasicUser2)

	t.Run("a community's connection only in that community", func(t *testing.T) {
		c := createTestAIConnection(t, th, f, th.BasicTeam.Id, nil)
		_, appErr := aiChat(th, AICall{UserID: th.BasicUser.Id, ChannelID: th.BasicChannel.Id}, c.Id)
		require.Nil(t, appErr, "in one of its channels")
		_, appErr = aiChat(th, AICall{UserID: th.BasicUser.Id, TeamID: th.BasicTeam.Id}, c.Id)
		require.Nil(t, appErr, "in the community, without a channel")

		_, appErr = aiChat(th, AICall{UserID: th.BasicUser.Id, ChannelID: otherChannel.Id}, c.Id)
		require.NotNil(t, appErr, "in another community's channel")
		assert.Equal(t, http.StatusForbidden, appErr.StatusCode)
		_, appErr = aiChat(th, AICall{UserID: th.BasicUser.Id, ChannelID: dm.Id}, c.Id)
		require.NotNil(t, appErr, "in a direct message")
		_, appErr = aiChat(th, AICall{UserID: th.BasicUser.Id}, c.Id)
		require.NotNil(t, appErr, "without context")
		_, appErr = aiChat(th, AICall{UserID: th.BasicUser.Id, ChannelID: otherChannel.Id, TeamID: th.BasicTeam.Id}, c.Id)
		require.NotNil(t, appErr, "a team ID that isn't the channel's")
		assert.Equal(t, http.StatusBadRequest, appErr.StatusCode)

		outsider := th.CreateUser(t)
		_, appErr = aiChat(th, AICall{UserID: outsider.Id, TeamID: th.BasicTeam.Id}, c.Id)
		require.NotNil(t, appErr, "someone outside the community")
	})

	t.Run("a channel the person can't read", func(t *testing.T) {
		c := createTestAIConnection(t, th, f, th.BasicTeam.Id, nil)
		private := th.CreatePrivateChannel(t, th.BasicTeam)
		require.Nil(t, th.RemoveUserFromChannel(t, th.BasicUser, private))
		_, appErr := aiChat(th, AICall{UserID: th.BasicUser.Id, ChannelID: private.Id}, c.Id)
		require.NotNil(t, appErr)
		assert.Equal(t, http.StatusForbidden, appErr.StatusCode)
	})

	t.Run("a server-wide connection where it's shared", func(t *testing.T) {
		c := createTestAIConnection(t, th, f, model.AIConnectionScopeServer, func(req *model.AIConnectionRequest) {
			req.Sharing = model.AIConnectionSharing{TeamIDs: []string{th.BasicTeam.Id}}
		})
		_, appErr := aiChat(th, AICall{UserID: th.BasicUser.Id, ChannelID: th.BasicChannel.Id}, c.Id)
		require.Nil(t, appErr)
		_, appErr = aiChat(th, AICall{UserID: th.BasicUser.Id, ChannelID: otherChannel.Id}, c.Id)
		require.NotNil(t, appErr, "a community it isn't shared with")
		_, appErr = aiChat(th, AICall{UserID: th.BasicUser.Id, ChannelID: dm.Id}, c.Id)
		require.NotNil(t, appErr, "direct messages need it shared with every community")

		shared := createTestAIConnection(t, th, f, model.AIConnectionScopeServer, nil)
		_, appErr = aiChat(th, AICall{UserID: th.BasicUser.Id, ChannelID: dm.Id}, shared.Id)
		require.Nil(t, appErr, "shared with all communities: direct messages too")
	})

	t.Run("guests only when allowed", func(t *testing.T) {
		guest := th.CreateGuest(t)
		th.LinkUserToTeam(t, guest, th.BasicTeam)
		th.AddUserToChannel(t, guest, th.BasicChannel)
		c := createTestAIConnection(t, th, f, th.BasicTeam.Id, nil)
		_, appErr := aiChat(th, AICall{UserID: guest.Id, ChannelID: th.BasicChannel.Id}, c.Id)
		require.NotNil(t, appErr, "everyone doesn't include guests")
		open := createTestAIConnection(t, th, f, th.BasicTeam.Id, func(req *model.AIConnectionRequest) { req.Audience.AllowGuests = true })
		_, appErr = aiChat(th, AICall{UserID: guest.Id, ChannelID: th.BasicChannel.Id}, open.Id)
		require.Nil(t, appErr)
	})

	t.Run("chosen groups and roles", func(t *testing.T) {
		group := th.CreateGroup(t)
		_, appErr := th.App.UpsertGroupMember(group.Id, th.BasicUser.Id)
		require.Nil(t, appErr)
		c := createTestAIConnection(t, th, f, model.AIConnectionScopeServer, func(req *model.AIConnectionRequest) {
			req.Audience = model.AIConnectionAudience{GroupIDs: []string{group.Id}}
		})
		_, appErr = aiChat(th, AICall{UserID: th.BasicUser.Id, ChannelID: th.BasicChannel.Id}, c.Id)
		require.Nil(t, appErr, "a member of the group")
		_, appErr = aiChat(th, AICall{UserID: th.BasicUser2.Id, ChannelID: th.BasicChannel.Id}, c.Id)
		require.NotNil(t, appErr, "not a member of the group")

		admins := createTestAIConnection(t, th, f, th.BasicTeam.Id, func(req *model.AIConnectionRequest) {
			req.Audience = model.AIConnectionAudience{Roles: []string{model.TeamAdminRoleId}}
		})
		_, appErr = aiChat(th, AICall{UserID: th.BasicUser2.Id, TeamID: th.BasicTeam.Id}, admins.Id)
		require.NotNil(t, appErr)
		assert.Equal(t, "app.ai.call.audience.app_error", appErr.Id)
		_, appErr = th.App.UpdateTeamMemberSchemeRoles(th.Context, th.BasicTeam.Id, th.BasicUser2.Id, false, true, true)
		require.Nil(t, appErr)
		_, appErr = aiChat(th, AICall{UserID: th.BasicUser2.Id, TeamID: th.BasicTeam.Id}, admins.Id)
		require.Nil(t, appErr, "the community's admins")
	})

	t.Run("deleted and unusable connections", func(t *testing.T) {
		c := createTestAIConnection(t, th, f, th.BasicTeam.Id, nil)
		require.Nil(t, th.App.DeleteAIConnection(th.Context, c))
		_, appErr := aiChat(th, AICall{UserID: th.BasicUser.Id, ChannelID: th.BasicChannel.Id}, c.Id)
		require.NotNil(t, appErr)
		assert.Equal(t, http.StatusNotFound, appErr.StatusCode)

		attention := createTestAIConnection(t, th, f, model.AIConnectionScopeServer, nil)
		attention.NeedsAttention = "check it"
		_, err := th.App.Srv().Store().AIConnection().Update(th.Context, attention)
		require.NoError(t, err)
		_, appErr = aiChat(th, AICall{UserID: th.BasicUser.Id}, attention.Id)
		require.NotNil(t, appErr)
		assert.Equal(t, http.StatusConflict, appErr.StatusCode)
	})

	t.Run("the connections a person may use", func(t *testing.T) {
		mine := createTestAIConnection(t, th, f, th.BasicTeam.Id, nil)
		theirs := createTestAIConnection(t, th, f, otherTeam.Id, nil)
		infos, appErr := th.App.GetAIConnectionsForUser(th.Context, th.BasicUser.Id, th.BasicChannel.Id, "")
		require.Nil(t, appErr)
		ids := []string{}
		for _, info := range infos {
			ids = append(ids, info.Id)
		}
		assert.Contains(t, ids, mine.Id)
		assert.NotContains(t, ids, theirs.Id)
		data, err := json.Marshal(infos)
		require.NoError(t, err)
		assert.NotContains(t, string(data), "api_key")
	})
}

func TestAICallPrincipal(t *testing.T) {
	th := setupAITest(t)
	f := newFakeLLM(t, "Hi")
	server := createTestAIConnection(t, th, f, model.AIConnectionScopeServer, nil)
	community := createTestAIConnection(t, th, f, th.BasicTeam.Id, nil)

	_, appErr := aiChat(th, AICall{}, server.Id)
	require.NotNil(t, appErr, "no anonymous calls")
	_, appErr = aiChat(th, AICall{UserID: "not-an-id"}, server.Id)
	require.NotNil(t, appErr)
	_, appErr = aiChat(th, AICall{Purpose: "anything"}, server.Id)
	require.NotNil(t, appErr, "only the server's own purposes")
	_, appErr = aiChat(th, AICall{UserID: th.BasicUser.Id, PluginID: "some.app", Purpose: ai.PurposeRecaps}, server.Id)
	require.NotNil(t, appErr, "an app can't call as a built-in feature")

	_, appErr = aiChat(th, AICall{Purpose: ai.PurposeAutoTranslation}, server.Id)
	require.Nil(t, appErr, "a built-in feature on a server-wide connection")
	_, appErr = aiChat(th, AICall{Purpose: ai.PurposeAutoTranslation}, community.Id)
	require.NotNil(t, appErr, "built-in features use server-wide connections only")

	deactivated := th.CreateUser(t)
	_, appErr = th.App.UpdateActive(th.Context, deactivated, false)
	require.Nil(t, appErr)
	_, appErr = aiChat(th, AICall{UserID: deactivated.Id}, server.Id)
	require.NotNil(t, appErr, "a deactivated person")
}

func TestAICallLimitsAndUsage(t *testing.T) {
	th := setupAITest(t)
	f := newFakeLLM(t, "Hi")

	t.Run("requests per minute", func(t *testing.T) {
		c := createTestAIConnection(t, th, f, model.AIConnectionScopeServer, func(req *model.AIConnectionRequest) {
			req.Limits = model.AIConnectionLimits{RequestsPerMinute: 3, PerUserRequestsPerMinute: 2}
		})
		for range 2 {
			_, appErr := aiChat(th, AICall{UserID: th.BasicUser.Id}, c.Id)
			require.Nil(t, appErr)
		}
		_, appErr := aiChat(th, AICall{UserID: th.BasicUser.Id}, c.Id)
		require.NotNil(t, appErr, "the person's rate")
		assert.Equal(t, http.StatusTooManyRequests, appErr.StatusCode)
		_, appErr = aiChat(th, AICall{UserID: th.BasicUser2.Id}, c.Id)
		require.Nil(t, appErr, "someone else still may")
		_, appErr = aiChat(th, AICall{UserID: th.SystemAdminUser.Id}, c.Id)
		require.NotNil(t, appErr, "the connection's rate")
	})

	t.Run("monthly budgets", func(t *testing.T) {
		c := createTestAIConnection(t, th, f, model.AIConnectionScopeServer, func(req *model.AIConnectionRequest) {
			req.Limits = model.AIConnectionLimits{MonthlyTokenBudget: 100, PerUserMonthlyTokenBudget: 20}
		})
		_, appErr := aiChat(th, AICall{UserID: th.BasicUser.Id}, c.Id)
		require.Nil(t, appErr)
		_, appErr = aiChat(th, AICall{UserID: th.BasicUser.Id}, c.Id)
		require.Nil(t, appErr, "15 tokens used of 20")
		_, appErr = aiChat(th, AICall{UserID: th.BasicUser.Id}, c.Id)
		require.NotNil(t, appErr, "30 tokens used: the person's budget is spent")
		assert.Equal(t, "app.ai.call.user_budget_exceeded.app_error", appErr.Id)

		require.NoError(t, th.App.Srv().Store().AIConnection().AddUsage(th.Context, &model.AIUsage{
			ConnectionId: c.Id, UserId: th.BasicUser2.Id, Day: model.AIUsageDay(time.Now()), Requests: 1, InputTokens: 100,
		}))
		_, appErr = aiChat(th, AICall{UserID: th.SystemAdminUser.Id}, c.Id)
		require.NotNil(t, appErr, "the connection's budget is spent")
		assert.Equal(t, "app.ai.call.budget_exceeded.app_error", appErr.Id)
	})

	t.Run("usage is counted per person and purpose", func(t *testing.T) {
		c := createTestAIConnection(t, th, f, model.AIConnectionScopeServer, nil)
		_, appErr := aiChat(th, AICall{UserID: th.BasicUser.Id, PluginID: "mattermost-ai"}, c.Id)
		require.Nil(t, appErr)
		_, appErr = aiChat(th, AICall{UserID: th.BasicUser.Id, Purpose: ai.PurposeRewrite}, c.Id)
		require.Nil(t, appErr)
		usage, appErr := th.App.GetAIConnectionUsage(th.Context, c)
		require.Nil(t, appErr)
		assert.Equal(t, model.AIUsageTotals{Requests: 2, InputTokens: 20, OutputTokens: 10}, usage.Totals)
		assert.Equal(t, int64(1), usage.ByPurpose["plugin:mattermost-ai"].Requests)
		assert.Equal(t, int64(1), usage.ByPurpose[string(ai.PurposeRewrite)].Requests)
	})
}

func TestAICallOtherCapabilities(t *testing.T) {
	th := setupAITest(t)
	f := newFakeLLM(t, "Hi")
	c := createTestAIConnection(t, th, f, model.AIConnectionScopeServer, nil)
	call := AICall{UserID: th.BasicUser.Id}

	_, appErr := th.App.AITranscribe(th.Context, call, ai.TranscribeRequest{ConnectionID: c.Id, Audio: []byte("x")})
	require.NotNil(t, appErr, "no transcription model on the connection")
	assert.Equal(t, "app.ai.call.no_model.app_error", appErr.Id)

	_, appErr = th.App.AIDecide(th.Context, call, ai.DecideRequest{ConnectionID: c.Id, Input: "x"})
	require.NotNil(t, appErr, "no questions")

	anthropic := createTestAIConnection(t, th, f, model.AIConnectionScopeServer, func(req *model.AIConnectionRequest) {
		req.Provider = model.AIProviderAnthropic
		req.Models = model.AIConnectionModels{Chat: "claude"}
	})
	_, appErr = th.App.AIEmbed(th.Context, call, ai.EmbedRequest{ConnectionID: anthropic.Id, Model: "x", Input: []string{"a"}})
	require.NotNil(t, appErr, "Anthropic has no embeddings")
	assert.Equal(t, "app.ai.call.unsupported.app_error", appErr.Id)
}

func TestAIConnectionReferences(t *testing.T) {
	th := setupAITest(t)
	f := newFakeLLM(t, "Hi")

	otherTeam := th.CreateTeam(t)
	foreign, err := th.App.Srv().Store().Group().Create(&model.Group{
		Name: model.NewPointer("g" + model.NewId()), DisplayName: "Foreign", Source: model.GroupSourceCustom,
		RemoteId: model.NewPointer(model.NewId()), TeamId: otherTeam.Id,
	})
	require.NoError(t, err)

	_, appErr := th.App.CreateAIConnection(th.Context, &model.AIConnectionRequest{
		Name: "x", Scope: th.BasicTeam.Id, Provider: model.AIProviderOpenAICompatible, BaseURL: f.URL,
		Audience: model.AIConnectionAudience{GroupIDs: []string{foreign.Id}},
	}, th.BasicUser.Id)
	require.NotNil(t, appErr, "a community's connection can't name another community's group")

	_, appErr = th.App.CreateAIConnection(th.Context, &model.AIConnectionRequest{
		Name: "x", Scope: model.NewId(), Provider: model.AIProviderOpenAICompatible,
	}, th.BasicUser.Id)
	require.NotNil(t, appErr, "a community that doesn't exist")

	_, appErr = th.App.CreateAIConnection(th.Context, &model.AIConnectionRequest{
		Name: "x", Scope: th.BasicTeam.Id, Provider: model.AIProviderOpenAICompatible,
		Sharing: model.AIConnectionSharing{AllTeams: true},
	}, th.BasicUser.Id)
	require.NotNil(t, appErr, "a community's connection isn't shared")

	_, appErr = th.App.CreateAIConnection(th.Context, &model.AIConnectionRequest{
		Name: "x", Scope: model.AIConnectionScopeServer, Provider: model.AIProviderOpenAICompatible,
		ExtraHeaders: map[string]string{"Host": "evil.example.com"},
	}, th.BasicUser.Id)
	require.NotNil(t, appErr, "headers the client owns")
}

func TestAICommunityConnectionsStayOffInternalAddresses(t *testing.T) {
	th := Setup(t).InitBasic(t)
	f := newFakeLLM(t, "Hi")
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.AllowedUntrustedInternalConnections = "" })

	community := createTestAIConnection(t, th, f, th.BasicTeam.Id, nil)
	_, appErr := aiChat(th, AICall{UserID: th.BasicUser.Id, ChannelID: th.BasicChannel.Id}, community.Id)
	require.NotNil(t, appErr, "a community admin's address may not reach the server's network")
	assert.Empty(t, f.calls())

	server := createTestAIConnection(t, th, f, model.AIConnectionScopeServer, nil)
	_, appErr = aiChat(th, AICall{UserID: th.BasicUser.Id, ChannelID: th.BasicChannel.Id}, server.Id)
	require.Nil(t, appErr, "a system admin's may")
}

// A person's or an app's call is audited, without what was said; the server's own features count their usage but
// write no record per call.
func TestAICallAudit(t *testing.T) {
	th := setupAITest(t)
	capture := startPluginAuditCapture(t, th)
	f := newFakeLLM(t, "The secret answer")
	c := createTestAIConnection(t, th, f, model.AIConnectionScopeServer, nil)

	_, appErr := th.App.AIChat(th.Context, AICall{UserID: th.BasicUser.Id, PluginID: "some.app", ChannelID: th.BasicChannel.Id},
		ai.ChatRequest{ConnectionID: c.Id, Messages: []ai.Message{{Role: ai.RoleUser, Content: "The secret question"}}, Operation: "summarize_thread"})
	require.Nil(t, appErr)
	records := capture.recordsFor(model.AuditEventAICall)
	require.Len(t, records, 1, "a person's call through an app is audited")
	data, err := json.Marshal(records[0])
	require.NoError(t, err)
	assert.Contains(t, string(data), c.Id)
	assert.Contains(t, string(data), "plugin:some.app")
	assert.Contains(t, string(data), th.BasicUser.Id)
	assert.NotContains(t, string(data), "The secret question", "never what was said")
	assert.NotContains(t, string(data), "The secret answer")
	assert.NotContains(t, string(data), aiTestKey)

	_, appErr = aiChat(th, AICall{Purpose: ai.PurposeAutoTranslation}, c.Id)
	require.Nil(t, appErr)
	assert.Len(t, capture.recordsFor(model.AuditEventAICall), 1, "an auto-translation call writes no audit record")
	usage, appErr := th.App.GetAIConnectionUsage(th.Context, c)
	require.Nil(t, appErr)
	assert.Equal(t, int64(1), usage.ByPurpose[string(ai.PurposeAutoTranslation)].Requests, "but its usage is counted")
}
