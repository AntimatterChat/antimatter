// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/ai"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/app/aimcp"
)

// toolTestAgent makes a server-wide agent with a tool policy.
func toolTestAgent(t *testing.T, th *TestHelper, connectionID string, policy model.AIAgentToolPolicy) *model.AIAgentDetails {
	t.Helper()
	return createTestAgent(t, th, model.AIAgentScopeServer, connectionID, func(req *model.AIAgentRequest) { req.Tools = policy })
}

// toolTestTurn is a turn of an agent for a person in a channel, as the runtime gives it to the tool runner.
func toolTestTurn(t *testing.T, th *TestHelper, agent *model.AIAgentDetails, asker *model.User, channel *model.Channel) *AgentTurnContext {
	t.Helper()
	post := savePostQuietly(t, th, &model.Post{UserId: asker.Id, ChannelId: channel.Id, Message: "@" + agent.Username + " help"})
	return &AgentTurnContext{Agent: agent.AIAgent, AgentUser: mustGetUser(t, th, agent.UserId), Asker: asker, Channel: channel, RootID: post.Id, PostID: post.Id}
}

func toolNames(tools []ai.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	return names
}

func runTestTool(th *TestHelper, turn *AgentTurnContext, name string, args any) AgentToolOutcome {
	raw, _ := json.Marshal(args)
	return th.App.agentToolRunner().Run(th.Context, turn, ai.ToolCall{ID: "call_" + model.NewId()[:8], Name: name, Arguments: raw})
}

func TestAIToolModelName(t *testing.T) {
	taken := map[string]*aiResolvedTool{}
	assert.Equal(t, "antimatter__search_messages", aiToolModelName("antimatter.search_messages", taken))
	assert.Equal(t, "com__example__todo__add-item", aiToolModelName("com.example.todo.add-item", taken))
	long := aiToolModelName("mcp."+strings.Repeat("x", 30)+"."+strings.Repeat("y", 60), taken)
	assert.LessOrEqual(t, len(long), 64, "providers take 64 characters at most")
	taken["antimatter__read_channel"] = &aiResolvedTool{}
	assert.Equal(t, "antimatter__read_channel_2", aiToolModelName("antimatter.read_channel", taken), "unique in a turn")
}

// An agent only gets the tools its policy allows, and a call of any other is refused, whatever the model asks.
func TestAIToolPolicyIsEnforced(t *testing.T) {
	th, _, c := setupAgentTest(t, "ok")
	agent := toolTestAgent(t, th, c.Id, model.AIAgentToolPolicy{Enabled: true, Allowed: []string{"antimatter.read_channel"}, AutoRun: []string{"*"}})
	turn := toolTestTurn(t, th, agent, th.BasicUser, th.BasicChannel)
	runner := th.App.agentToolRunner()

	assert.Equal(t, []string{"antimatter__read_channel"}, toolNames(runner.Tools(th.Context, turn)))

	outcome := runTestTool(th, turn, "antimatter__post_message", map[string]any{"channel_id": th.BasicChannel.Id, "message": "pwned"})
	assert.Contains(t, outcome.Result, "There is no tool")
	assert.Nil(t, outcome.Pending, "a forbidden tool isn't even proposed")

	outcome = runTestTool(th, turn, "antimatter__read_channel", map[string]any{"channel_id": th.BasicChannel.Id})
	assert.Contains(t, outcome.Result, "Result of antimatter.read_channel")

	t.Run("an agent whose tools are off has none", func(t *testing.T) {
		off := toolTestAgent(t, th, c.Id, model.AIAgentToolPolicy{Enabled: false, Allowed: []string{"*"}, AutoRun: []string{"*"}})
		assert.Empty(t, runner.Tools(th.Context, toolTestTurn(t, th, off, th.BasicUser, th.BasicChannel)))
	})

	t.Run("the policy is checked again when the call runs", func(t *testing.T) {
		agent := toolTestAgent(t, th, c.Id, model.AIAgentToolPolicy{Enabled: true, Allowed: []string{"antimatter.*"}, AutoRun: []string{"*"}})
		changed := toolTestTurn(t, th, agent, th.BasicUser, th.BasicChannel)
		require.NotEmpty(t, runner.Tools(th.Context, changed))
		changed.Agent = changed.Agent.Clone()
		changed.Agent.Tools.Allowed = []string{"antimatter.lookup_user"}
		outcome := runTestTool(th, changed, "antimatter__read_channel", map[string]any{"channel_id": th.BasicChannel.Id})
		assert.Equal(t, "This tool isn't available.", outcome.Result)
	})

	t.Run("arguments must be an object", func(t *testing.T) {
		outcome := th.App.agentToolRunner().Run(th.Context, turn, ai.ToolCall{ID: "x", Name: "antimatter__read_channel", Arguments: json.RawMessage(`["a"]`)})
		assert.Equal(t, "The arguments must be a JSON object.", outcome.Result)
	})
}

// Tools act as the person who asked, never as the agent: a channel only the agent's account (or someone else) can
// read stays unreadable to the person's tools.
func TestAIToolsActAsTheAsker(t *testing.T) {
	th, _, c := setupAgentTest(t, "ok")
	agent := toolTestAgent(t, th, c.Id, model.AIAgentToolPolicy{Enabled: true, Allowed: []string{"antimatter.*"}, AutoRun: []string{"*"}})

	// Only BasicUser2 (its creator) and the agent's own account are in it.
	secret := th.CreatePrivateChannel(t, th.BasicTeam, func(ch *model.Channel) { ch.CreatorId = th.BasicUser2.Id })
	agentUser := mustGetUser(t, th, agent.UserId)
	th.LinkUserToTeam(t, agentUser, th.BasicTeam)
	th.AddUserToChannel(t, agentUser, secret)
	th.CreatePost(t, secret, func(p *model.Post) {
		p.UserId = th.BasicUser2.Id
		p.Message = "The merger code name is BLUEBIRD."
	})

	turn := toolTestTurn(t, th, agent, th.BasicUser, th.BasicChannel)
	th.App.agentToolRunner().Tools(th.Context, turn)
	for _, args := range []map[string]any{{"channel_id": secret.Id}, {"channel_name": secret.Name}} {
		outcome := runTestTool(th, turn, "antimatter__read_channel", args)
		assert.NotContains(t, outcome.Result, "BLUEBIRD", "the agent's own membership gives the person nothing")
		assert.Contains(t, outcome.Result, "The tool failed")
	}
	outcome := runTestTool(th, turn, "antimatter__search_messages", map[string]any{"terms": "BLUEBIRD"})
	assert.NotContains(t, outcome.Result, "merger")

	other := toolTestTurn(t, th, agent, th.BasicUser2, th.BasicChannel)
	th.App.agentToolRunner().Tools(th.Context, other)
	outcome = runTestTool(th, other, "antimatter__read_channel", map[string]any{"channel_id": secret.Id})
	assert.Contains(t, outcome.Result, "BLUEBIRD", "someone who may read it reads it")
}

// A tool that changes something never runs on its own, whatever auto_run says: it waits for the person, who alone
// may answer, once, with the arguments they were shown.
func TestAIToolWritesNeedTheAskersConfirmation(t *testing.T) {
	th, _, c := setupAgentTest(t, "Done.")
	agent := toolTestAgent(t, th, c.Id, model.AIAgentToolPolicy{Enabled: true, Allowed: []string{"antimatter.post_message"}, AutoRun: []string{"*"}})
	turn := toolTestTurn(t, th, agent, th.BasicUser, th.BasicChannel)
	th.App.agentToolRunner().Tools(th.Context, turn)
	countMine := func(message string) int {
		list, appErr := th.App.GetPostsPage(th.Context, model.GetPostsOptions{ChannelId: th.BasicChannel.Id, PerPage: 100})
		require.Nil(t, appErr)
		n := 0
		for _, p := range list.Posts {
			if p.Message == message && p.UserId == th.BasicUser.Id {
				n++
			}
		}
		return n
	}

	outcome := runTestTool(th, turn, "antimatter__post_message", map[string]any{"channel_id": th.BasicChannel.Id, "message": "Release is on Friday"})
	require.NotNil(t, outcome.Pending)
	assert.Equal(t, model.AIToolCallPending, outcome.Pending.Status)
	assert.Equal(t, th.BasicUser.Id, outcome.Pending.UserId)
	assert.Contains(t, outcome.Pending.Arguments, "Release is on Friday", "the person sees what will be done")
	assert.Zero(t, countMine("Release is on Friday"), "nothing ran yet")

	_, appErr := th.App.AnswerAIToolCall(th.Context, th.BasicUser2.Id, outcome.Pending.Id, true)
	require.NotNil(t, appErr, "nobody else may confirm it")
	assert.Equal(t, http.StatusNotFound, appErr.StatusCode)
	assert.Zero(t, countMine("Release is on Friday"))

	summary, appErr := th.App.AnswerAIToolCall(th.Context, th.BasicUser.Id, outcome.Pending.Id, true)
	require.Nil(t, appErr)
	assert.Equal(t, model.AIToolCallDone, summary.Status)
	assert.Equal(t, 1, countMine("Release is on Friday"), "posted as the person")

	list, appErr := th.App.GetPostsPage(th.Context, model.GetPostsOptions{ChannelId: th.BasicChannel.Id, PerPage: 100})
	require.Nil(t, appErr)
	for _, p := range list.Posts {
		if p.Message == "Release is on Friday" {
			assert.Equal(t, agent.UserId, p.GetProp(model.PostPropAgentPostedFor), "marked as posted through the agent")
		}
	}

	_, appErr = th.App.AnswerAIToolCall(th.Context, th.BasicUser.Id, outcome.Pending.Id, true)
	require.NotNil(t, appErr, "a confirmation is used once")
	assert.Equal(t, 1, countMine("Release is on Friday"))

	t.Run("declined, it doesn't run", func(t *testing.T) {
		outcome := runTestTool(th, turn, "antimatter__post_message", map[string]any{"channel_id": th.BasicChannel.Id, "message": "Not this one"})
		require.NotNil(t, outcome.Pending)
		summary, appErr := th.App.AnswerAIToolCall(th.Context, th.BasicUser.Id, outcome.Pending.Id, false)
		require.Nil(t, appErr)
		assert.Equal(t, model.AIToolCallDeclined, summary.Status)
		assert.Zero(t, countMine("Not this one"))
	})

	t.Run("a call the person may no longer make is refused when confirmed", func(t *testing.T) {
		outcome := runTestTool(th, turn, "antimatter__post_message", map[string]any{"channel_id": th.BasicChannel.Id, "message": "Too late"})
		require.NotNil(t, outcome.Pending)
		allowed := model.AIAgentToolPolicy{Enabled: true, Allowed: []string{"antimatter.read_channel"}}
		_, appErr := th.App.PatchAIAgent(th.Context, agent.AIAgent, &model.AIAgentPatch{Tools: &allowed}, th.SystemAdminUser.Id)
		require.Nil(t, appErr)
		_, appErr = th.App.AnswerAIToolCall(th.Context, th.BasicUser.Id, outcome.Pending.Id, true)
		require.NotNil(t, appErr)
		assert.Equal(t, http.StatusForbidden, appErr.StatusCode)
		assert.Zero(t, countMine("Too late"))
	})

	t.Run("the person's own permissions: no posting where they can't", func(t *testing.T) {
		agent := toolTestAgent(t, th, c.Id, model.AIAgentToolPolicy{Enabled: true, Allowed: []string{"antimatter.post_message"}})
		turn := toolTestTurn(t, th, agent, th.BasicUser, th.BasicChannel)
		th.App.agentToolRunner().Tools(th.Context, turn)
		private := th.CreatePrivateChannel(t, th.BasicTeam, func(ch *model.Channel) { ch.CreatorId = th.BasicUser2.Id })
		outcome := runTestTool(th, turn, "antimatter__post_message", map[string]any{"channel_id": private.Id, "message": "Sneaky"})
		require.NotNil(t, outcome.Pending)
		summary, appErr := th.App.AnswerAIToolCall(th.Context, th.BasicUser.Id, outcome.Pending.Id, true)
		require.Nil(t, appErr)
		assert.Equal(t, model.AIToolCallFailed, summary.Status)
	})
}

// What a person turns off for themselves is gone from every agent's tools.
func TestAIToolChoicesOfThePerson(t *testing.T) {
	th, _, c := setupAgentTest(t, "ok")
	agent := toolTestAgent(t, th, c.Id, model.AIAgentToolPolicy{Enabled: true, Allowed: []string{"antimatter.*"}, AutoRun: []string{"*"}})
	setChoices := func(choices model.AIToolChoices) {
		data, _ := json.Marshal(choices)
		require.Nil(t, th.App.UpdatePreferences(th.Context, th.BasicUser.Id, model.Preferences{{UserId: th.BasicUser.Id, Category: model.PreferenceCategoryAITools, Name: model.PreferenceNameAIToolChoices, Value: string(data)}}))
	}

	setChoices(model.AIToolChoices{OffTools: []string{"antimatter.search_messages"}})
	turn := toolTestTurn(t, th, agent, th.BasicUser, th.BasicChannel)
	names := toolNames(th.App.agentToolRunner().Tools(th.Context, turn))
	assert.NotContains(t, names, "antimatter__search_messages")
	assert.Contains(t, names, "antimatter__read_channel")

	setChoices(model.AIToolChoices{OffSources: []string{model.AIToolSourceBuiltIn}})
	outcome := runTestTool(th, turn, "antimatter__read_channel", map[string]any{"channel_id": th.BasicChannel.Id})
	assert.Equal(t, "This tool isn't available.", outcome.Result, "turned off since the turn started")
	assert.Empty(t, th.App.agentToolRunner().Tools(th.Context, toolTestTurn(t, th, agent, th.BasicUser, th.BasicChannel)))

	other := toolTestTurn(t, th, agent, th.BasicUser2, th.BasicChannel)
	assert.NotEmpty(t, th.App.agentToolRunner().Tools(th.Context, other), "someone's choices are theirs only")

	sources, appErr := th.App.GetAIAgentToolsForUser(th.Context, th.BasicUser.Id, agent.UserId, th.BasicChannel.Id)
	require.Nil(t, appErr)
	require.NotEmpty(t, sources)
	assert.False(t, sources[0].On, "the panel shows what they turned off")
}

// fakeMCPServer is an MCP server (JSON answers) whose tools need a person's own bearer token, a service account's
// header, or nothing; it can also be an OAuth authorisation server for its sign-in.
type fakeMCPServer struct {
	*httptest.Server
	mu sync.Mutex
	// tokens are the bearer tokens it takes, and whose they are; none: every request is taken.
	tokens   map[string]string
	header   string
	tools    []aimcp.Tool
	seenAuth []string
	called   []string
	codes    map[string]string
	verifier map[string]string
	// answer, when set, gives a call's text (by the tool, its arguments and whose token it came with).
	answer func(name string, arguments json.RawMessage, who string) string
}

func newFakeMCPServer(t *testing.T) *fakeMCPServer {
	f := &fakeMCPServer{tokens: map[string]string{}, codes: map[string]string{}, verifier: map[string]string{}}
	yes, no := true, false
	f.tools = []aimcp.Tool{
		{Name: "search_issues", Description: "Find issues", Annotations: &aimcp.ToolAnnotations{ReadOnlyHint: &yes}},
		{Name: "create.issue", Description: "Create an issue", Annotations: &aimcp.ToolAnnotations{ReadOnlyHint: &no}},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", f.serveMCP)
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/mcp", http.StatusTemporaryRedirect)
	})
	mux.HandleFunc("/.well-known/oauth-protected-resource/mcp", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"resource": f.URL + "/mcp", "authorization_servers": []string{f.URL}})
	})
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer": f.URL, "authorization_endpoint": f.URL + "/authorize", "token_endpoint": f.URL + "/token",
			"registration_endpoint": f.URL + "/register", "code_challenge_methods_supported": []string{"S256"},
		})
	})
	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"client_id": "registered", "client_secret": "registered-secret-1234"})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.mu.Lock()
		who, ok := f.codes[r.PostForm.Get("code")]
		delete(f.codes, r.PostForm.Get("code"))
		f.mu.Unlock()
		if !ok || r.PostForm.Get("code_verifier") == "" || r.PostForm.Get("client_id") != "registered" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid_grant"})
			return
		}
		token := "token-of-" + who
		f.mu.Lock()
		f.tokens[token] = who
		f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": token, "token_type": "Bearer", "expires_in": 3600})
	})
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	return f
}

func (f *fakeMCPServer) serveMCP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	auth := r.Header.Get("Authorization")
	if f.header != "" {
		auth = r.Header.Get("X-Service-Key")
	}
	f.seenAuth = append(f.seenAuth, auth)
	who := ""
	switch {
	case f.header != "":
		if auth != f.header {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	case len(f.tokens) > 0 || len(f.codes) > 0:
		var ok bool
		who, ok = f.tokens[strings.TrimPrefix(auth, "Bearer ")]
		if !ok {
			w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="`+f.URL+`/.well-known/oauth-protected-resource/mcp"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}
	if r.Method == http.MethodDelete {
		return
	}
	var msg struct {
		ID     *int64          `json:"id"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	data, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(data, &msg)
	if msg.ID == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	var result any
	switch msg.Method {
	case "initialize":
		w.Header().Set("Mcp-Session-Id", "s1")
		result = map[string]any{"protocolVersion": aimcp.ProtocolVersion}
	case "tools/list":
		result = map[string]any{"tools": f.tools}
	case "tools/call":
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		_ = json.Unmarshal(msg.Params, &params)
		f.called = append(f.called, params.Name)
		text := "ran " + params.Name + " as " + who
		if f.answer != nil {
			text = f.answer(params.Name, params.Arguments, who)
		}
		result = map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}
	}
	out, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": *msg.ID, "result": result})
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(out)
}

func (f *fakeMCPServer) takeToken(token, who string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokens[token] = who
}

func (f *fakeMCPServer) authsSeen() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.seenAuth...)
}

func createTestMCPServer(t *testing.T, th *TestHelper, f *fakeMCPServer, change func(req *model.AIMCPServerRequest)) *model.AIMCPServer {
	t.Helper()
	req := &model.AIMCPServerRequest{
		Name: "Tracker", Slug: "t" + strings.ToLower(model.NewId()[:12]), Scope: model.AIConnectionScopeServer, URL: f.URL + "/mcp",
		Auth: model.AIMCPAuthOAuth, Sharing: model.AIConnectionSharing{AllTeams: true},
	}
	if change != nil {
		change(req)
	}
	s, appErr := th.App.CreateAIMCPServer(th.Context, req, th.SystemAdminUser.Id)
	require.Nil(t, appErr)
	return s
}

func signInToTestMCPServer(t *testing.T, th *TestHelper, s *model.AIMCPServer, userID, token string) {
	t.Helper()
	require.Nil(t, th.App.saveAIMCPSignIn(th.Context, s.Id, userID, &aiMCPSignIn{Token: aimcp.Token{AccessToken: token, TokenType: "Bearer"}}))
}

// An MCP server's tools run with the asker's own sign-in there: someone without one gets no tool (and the agent is
// told they could sign in), never someone else's.
func TestAIMCPToolsUseTheAskersOwnSignIn(t *testing.T) {
	th, _, c := setupAgentTest(t, "ok")
	f := newFakeMCPServer(t)
	s := createTestMCPServer(t, th, f, nil)
	f.takeToken("maya-token-123456", "maya")
	signInToTestMCPServer(t, th, s, th.BasicUser.Id, "maya-token-123456")
	agent := toolTestAgent(t, th, c.Id, model.AIAgentToolPolicy{Enabled: true, Allowed: []string{s.Namespace() + ".*"}, AutoRun: []string{"*"}})

	maya := toolTestTurn(t, th, agent, th.BasicUser, th.BasicChannel)
	names := toolNames(th.App.agentToolRunner().Tools(th.Context, maya))
	searchName := aiToolModelName(s.Namespace()+".search_issues", map[string]*aiResolvedTool{})
	createName := aiToolModelName(s.Namespace()+".create_issue", map[string]*aiResolvedTool{})
	assert.ElementsMatch(t, []string{searchName, createName}, names, "the tool's name keeps only what tool names may have")

	outcome := runTestTool(th, maya, searchName, map[string]any{"q": "login"})
	assert.Contains(t, outcome.Result, "ran search_issues as maya")
	outcome = runTestTool(th, maya, createName, map[string]any{"title": "Bug"})
	require.NotNil(t, outcome.Pending, "an MCP tool that doesn't say it only reads waits for the person")

	sam := toolTestTurn(t, th, agent, th.BasicUser2, th.BasicChannel)
	assert.Empty(t, th.App.agentToolRunner().Tools(th.Context, sam), "no sign-in, no tools")
	assert.Contains(t, th.App.agentToolRunner().Instructions(sam), s.Name, "the agent can tell them to sign in")
	for _, auth := range f.authsSeen() {
		assert.NotEqual(t, "", auth)
		assert.Equal(t, "Bearer maya-token-123456", auth, "only maya's turns ever sent maya's token")
	}

	t.Run("another person can't confirm with their sign-in either", func(t *testing.T) {
		_, appErr := th.App.AnswerAIToolCall(th.Context, th.BasicUser2.Id, outcome.Pending.Id, true)
		require.NotNil(t, appErr)
	})

	t.Run("the panel says who signed in", func(t *testing.T) {
		// Outside any channel (BasicUser2 isn't in the basic channel): the server is shared with every community.
		sources, appErr := th.App.GetAIAgentToolsForUser(th.Context, th.BasicUser2.Id, agent.UserId, "")
		require.Nil(t, appErr)
		var found *model.AIToolSourceInfo
		for _, source := range sources {
			if source.SourceId == s.Id {
				found = source
			}
		}
		require.NotNil(t, found, "a server they could sign in to is listed")
		assert.False(t, found.Connected)
		assert.Empty(t, found.Tools)
	})
}

// A person's sign-in is sealed, bound to them and the server, and opened for nobody else.
func TestAIMCPSignInsAreSealedAndOnlyTheirs(t *testing.T) {
	th, _, _ := setupAgentTest(t, "ok")
	f := newFakeMCPServer(t)
	s := createTestMCPServer(t, th, f, nil)
	signInToTestMCPServer(t, th, s, th.BasicUser.Id, "maya-secret-token-987654")

	row, err := th.App.Srv().Store().AIMCPServer().GetUserToken(th.Context, s.Id, th.BasicUser.Id)
	require.NoError(t, err)
	assert.NotContains(t, row.TokenSealed, "maya-secret-token", "never stored in clear")

	si, err := th.App.loadAIMCPSignIn(th.Context, s.Id, th.BasicUser.Id)
	require.NoError(t, err)
	assert.Equal(t, "maya-secret-token-987654", si.Token.AccessToken)

	_, err = th.App.loadAIMCPSignIn(th.Context, s.Id, th.BasicUser2.Id)
	assert.ErrorIs(t, err, errAIMCPNotSignedIn)

	// Even moved to someone else's row (a database edit), it opens for nobody.
	require.NoError(t, th.App.Srv().Store().AIMCPServer().SaveUserToken(th.Context, &model.AIMCPUserToken{ServerId: s.Id, UserId: th.BasicUser2.Id, TokenSealed: row.TokenSealed}))
	_, err = th.App.loadAIMCPSignIn(th.Context, s.Id, th.BasicUser2.Id)
	assert.ErrorIs(t, err, errAIMCPNotSignedIn, "a sign-in is bound to its person")

	t.Run("the server's own secrets are write-only", func(t *testing.T) {
		withSecrets := createTestMCPServer(t, th, f, func(req *model.AIMCPServerRequest) {
			req.Auth = model.AIMCPAuthServiceAccount
			req.Headers = map[string]string{"X-Service-Key": "service-key-value-1234"}
		})
		data, err := json.Marshal(withSecrets)
		require.NoError(t, err)
		assert.NotContains(t, string(data), "service-key-value")
		assert.Contains(t, string(data), "X-Service-Key", "only the header's name")
		list, appErr := th.App.GetAIMCPServersForManager(th.Context, model.Session{UserId: th.SystemAdminUser.Id, Roles: th.SystemAdminUser.Roles}, "")
		require.Nil(t, appErr)
		data, err = json.Marshal(list)
		require.NoError(t, err)
		assert.NotContains(t, string(data), "service-key-value")
		assert.NotContains(t, string(data), "maya-secret-token", "admins never see a person's sign-in")
	})

	t.Run("the support packet carries no sign-in", func(t *testing.T) {
		noProfile := time.Duration(0)
		for _, file := range th.App.GenerateSupportPacket(th.Context, &model.SupportPacketOptions{IncludeLogs: true, CPUProfileDuration: &noProfile}) {
			assert.NotContains(t, string(file.Body), "maya-secret-token", "%s carries a sign-in", file.Filename)
		}
	})

	t.Run("deactivating someone forgets their sign-ins", func(t *testing.T) {
		user := th.CreateUser(t)
		signInToTestMCPServer(t, th, s, user.Id, "their-token-123456")
		_, appErr := th.App.UpdateActive(th.Context, user, false)
		require.Nil(t, appErr)
		_, err := th.App.loadAIMCPSignIn(th.Context, s.Id, user.Id)
		assert.ErrorIs(t, err, errAIMCPNotSignedIn)
	})
}

// A service account (an admin's shared account) only ever offers the tools that read.
func TestAIMCPServiceAccountOnlyReads(t *testing.T) {
	th, _, c := setupAgentTest(t, "ok")
	f := newFakeMCPServer(t)
	f.header = "service-key-value-1234"
	s := createTestMCPServer(t, th, f, func(req *model.AIMCPServerRequest) {
		req.Auth = model.AIMCPAuthServiceAccount
		req.Headers = map[string]string{"X-Service-Key": "service-key-value-1234"}
	})
	agent := toolTestAgent(t, th, c.Id, model.AIAgentToolPolicy{Enabled: true, Allowed: []string{"*"}, AutoRun: []string{"*"}})
	turn := toolTestTurn(t, th, agent, th.BasicUser, th.BasicChannel)
	names := toolNames(th.App.agentToolRunner().Tools(th.Context, turn))
	assert.Contains(t, names, aiToolModelName(s.Namespace()+".search_issues", map[string]*aiResolvedTool{}))
	assert.NotContains(t, names, aiToolModelName(s.Namespace()+".create_issue", map[string]*aiResolvedTool{}), "never a write through a shared account")
}

// MCP servers follow the model connections' network rules: a community's may not reach internal addresses unless
// allowed, and no request follows a redirect.
func TestAIMCPNetworkRules(t *testing.T) {
	th, _, _ := setupAgentTest(t, "ok")
	f := newFakeMCPServer(t)
	community := createTestMCPServer(t, th, f, func(req *model.AIMCPServerRequest) {
		req.Scope = th.BasicTeam.Id
		req.Auth = model.AIMCPAuthNone
		req.Sharing = model.AIConnectionSharing{}
	})

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.AllowedUntrustedInternalConnections = "" })
	_, err := th.App.aiMCPToolsFor(th.Context, community, th.BasicUser.Id)
	require.Error(t, err, "a community's server can't reach 127.0.0.1 unless allowed")

	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.AllowedUntrustedInternalConnections = "127.0.0.1" })
	tools, err := th.App.aiMCPToolsFor(th.Context, community, th.BasicUser.Id)
	require.NoError(t, err)
	assert.Len(t, tools, 2)

	redirected := createTestMCPServer(t, th, f, func(req *model.AIMCPServerRequest) {
		req.Auth = model.AIMCPAuthNone
		req.URL = f.URL + "/redirect"
	})
	_, err = th.App.aiMCPToolsFor(th.Context, redirected, th.BasicUser.Id)
	require.Error(t, err, "no redirect is followed")

	t.Run("a sign-in's other addresses are never trusted", func(t *testing.T) {
		th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.AllowedUntrustedInternalConnections = "" })
		defer th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.AllowedUntrustedInternalConnections = "127.0.0.1" })
		server := createTestMCPServer(t, th, f, func(req *model.AIMCPServerRequest) {
			req.URL = strings.Replace(f.URL, "127.0.0.1", "localhost", 1) + "/mcp"
		})
		client := th.App.aiMCPOAuthHTTPClient(server)
		resp, err := client.Get(f.URL + "/.well-known/oauth-authorization-server")
		if resp != nil {
			resp.Body.Close()
		}
		require.Error(t, err, "an address other than the server's own host is checked like a community's")
	})

	t.Run("a community's server is its own agents' only", func(t *testing.T) {
		assert.True(t, aiMCPAvailableIn(community, &model.AIAgent{Scope: th.BasicTeam.Id}, th.BasicTeam.Id))
		assert.False(t, aiMCPAvailableIn(community, &model.AIAgent{Scope: model.NewId()}, th.BasicTeam.Id))
		assert.False(t, aiMCPAvailableIn(community, &model.AIAgent{Scope: model.AIAgentScopeServer}, th.BasicTeam.Id))
	})
}

// Signing in: the state is the person's own, used once; someone else finishing it gets nothing.
func TestAIMCPOAuthSignIn(t *testing.T) {
	th, _, _ := setupAgentTest(t, "ok")
	th.App.UpdateConfig(func(cfg *model.Config) { *cfg.ServiceSettings.SiteURL = "http://localhost:8065" })
	f := newFakeMCPServer(t)
	s := createTestMCPServer(t, th, f, nil)

	connect, appErr := th.App.StartAIMCPOAuth(th.Context, th.BasicUser, s)
	require.Nil(t, appErr)
	authorizeURL, err := url.Parse(connect.AuthorizeURL)
	require.NoError(t, err)
	authorize := authorizeURL.Query()
	assert.Equal(t, "S256", authorize.Get("code_challenge_method"))
	assert.Equal(t, "http://localhost:8065"+AIMCPOAuthCallbackPath, authorize.Get("redirect_uri"))
	state := authorize.Get("state")

	saved, appErr := th.App.GetAIMCPServer(th.Context, s.Id)
	require.Nil(t, appErr)
	assert.Equal(t, "registered", saved.OAuthClientId, "the client the server registered is kept")
	assert.True(t, saved.OAuthRegistered)

	f.mu.Lock()
	f.codes["the-code"] = "maya"
	f.mu.Unlock()

	_, appErr = th.App.FinishAIMCPOAuth(th.Context, th.BasicUser2.Id, state, "the-code", "")
	require.NotNil(t, appErr, "another person can't finish someone's sign-in")
	assert.False(t, th.App.aiMCPSignedIn(th.Context, s.Id, th.BasicUser2.Id))

	_, appErr = th.App.FinishAIMCPOAuth(th.Context, th.BasicUser.Id, state, "the-code", "")
	require.Nil(t, appErr, "the person still can")
	si, err := th.App.loadAIMCPSignIn(th.Context, s.Id, th.BasicUser.Id)
	require.NoError(t, err)
	assert.Equal(t, "token-of-maya", si.Token.AccessToken)

	_, appErr = th.App.FinishAIMCPOAuth(th.Context, th.BasicUser.Id, state, "the-code", "")
	require.NotNil(t, appErr, "a state is used once")

	t.Run("disconnecting forgets their sign-in only", func(t *testing.T) {
		signInToTestMCPServer(t, th, s, th.BasicUser2.Id, "sam-token-123456")
		require.Nil(t, th.App.DisconnectAIMCPServer(th.Context, th.BasicUser.Id, s))
		assert.False(t, th.App.aiMCPSignedIn(th.Context, s.Id, th.BasicUser.Id))
		assert.True(t, th.App.aiMCPSignedIn(th.Context, s.Id, th.BasicUser2.Id))
	})

	t.Run("a new address forgets everyone's sign-in", func(t *testing.T) {
		newURL := f.URL + "/mcp?v=2"
		_, appErr := th.App.PatchAIMCPServer(th.Context, saved, &model.AIMCPServerPatch{URL: &newURL}, th.SystemAdminUser.Id)
		require.Nil(t, appErr)
		assert.False(t, th.App.aiMCPSignedIn(th.Context, s.Id, th.BasicUser2.Id))
	})
}

func TestRegisterAIAppTools(t *testing.T) {
	th := Setup(t)
	valid := []*model.AIToolDefinition{{Name: "add_item", Title: "Add", Kind: model.AIToolKindWrite}}
	assert.NotNil(t, th.App.RegisterAIAppTools(model.AIToolBuiltInNamespace, "1", valid), "an app can't name its tools like the server's")
	assert.NotNil(t, th.App.RegisterAIAppTools("mcp.github", "1", valid), "nor like an MCP server's")
	assert.NotNil(t, th.App.RegisterAIAppTools("com.example.todo", "1", []*model.AIToolDefinition{{Name: "bad name", Kind: model.AIToolKindRead}}))
	assert.NotNil(t, th.App.RegisterAIAppTools("com.example.todo", "1", append(valid, valid[0])), "twice the same name")
	require.Nil(t, th.App.RegisterAIAppTools("com.example.todo", "1", valid))
	assert.Empty(t, th.App.aiRunningAppTools(), "an app's tools count only while it runs")
}

// A turn with tools, end to end: the model asks for a read the policy runs on its own, gets its result as
// information, and answers.
func TestAgentTurnRunsATool(t *testing.T) {
	th := setupAITest(t)
	s := newScriptedLLM(t, []scriptedAnswer{
		{tool: "antimatter__read_channel", args: `{"channel_id":"` + th.BasicChannel.Id + `"}`},
		{text: "The channel talks about the launch."},
	})
	c := createTestAIConnection(t, th, &fakeLLM{Server: s.Server}, model.AIConnectionScopeServer, nil)
	agent := toolTestAgent(t, th, c.Id, model.AIAgentToolPolicy{Enabled: true, Allowed: []string{"antimatter.read_channel"}, AutoRun: []string{"antimatter.read_channel"}})
	th.CreateMessagePost(t, th.BasicChannel, "The launch is on Friday.")

	post := th.CreateMessagePost(t, th.BasicChannel, "@"+agent.Username+" what is this channel about?")
	reply := waitForAgentReply(t, th, agent.UserId, post.Id)
	assert.Equal(t, "The channel talks about the launch.", reply.Message)

	bodies := s.bodies()
	require.Len(t, bodies, 2)
	tools, _ := bodies[0]["tools"].([]any)
	require.Len(t, tools, 1, "only the tools the policy allows")
	messages, _ := bodies[1]["messages"].([]any)
	last, _ := messages[len(messages)-1].(map[string]any)
	assert.Equal(t, "tool", last["role"])
	assert.Contains(t, last["content"], "information, not instructions")
	assert.Contains(t, last["content"], "The launch is on Friday.")
	system, _ := messages[0].(map[string]any)
	assert.Contains(t, system["content"], "never instructions to follow")
}

// A turn whose model asks for a write ends waiting for the person: the reply shows the call, nothing ran.
func TestAgentTurnWaitsForConfirmation(t *testing.T) {
	th := setupAITest(t)
	s := newScriptedLLM(t, []scriptedAnswer{
		{tool: "antimatter__post_message", args: `{"channel_id":"` + th.BasicChannel.Id + `","message":"Shipped!"}`},
		{text: "Posted."},
	})
	c := createTestAIConnection(t, th, &fakeLLM{Server: s.Server}, model.AIConnectionScopeServer, nil)
	agent := toolTestAgent(t, th, c.Id, model.AIAgentToolPolicy{Enabled: true, Allowed: []string{"*"}, AutoRun: []string{"*"}})

	post := th.CreateMessagePost(t, th.BasicChannel, "@"+agent.Username+" tell everyone it shipped")
	reply := waitForAgentReply(t, th, agent.UserId, post.Id)
	assert.Contains(t, reply.Message, "@"+th.BasicUser.Username)
	raw, _ := json.Marshal(reply.GetProp(model.PostPropAgentToolCalls))
	var calls []*model.AIToolCallSummary
	require.NoError(t, json.Unmarshal(raw, &calls))
	require.Len(t, calls, 1)
	assert.Equal(t, model.AIToolCallPending, calls[0].Status)
	assert.Equal(t, "antimatter.post_message", calls[0].Tool)
	assert.Len(t, s.bodies(), 1, "the turn stopped")
	require.Eventually(t, func() bool {
		stored, appErr := th.App.GetPluginKey(agentRuntimeKVNamespace, aiPendingToolCallPrefix+calls[0].Id)
		var pending aiPendingToolCall
		return appErr == nil && json.Unmarshal(stored, &pending) == nil && pending.ReplyId == reply.Id
	}, 5*time.Second, 20*time.Millisecond, "the call knows the reply that shows it")

	summary, appErr := th.App.AnswerAIToolCall(th.Context, th.BasicUser.Id, calls[0].Id, true)
	require.Nil(t, appErr)
	assert.Equal(t, model.AIToolCallDone, summary.Status)
	require.Eventually(t, func() bool { return len(s.bodies()) == 2 }, 10*time.Second, 50*time.Millisecond, "the agent goes on with the result")
	updated, appErr := th.App.GetSinglePost(th.Context, reply.Id, false)
	require.Nil(t, appErr)
	raw, _ = json.Marshal(updated.GetProp(model.PostPropAgentToolCalls))
	require.NoError(t, json.Unmarshal(raw, &calls))
	assert.Equal(t, model.AIToolCallDone, calls[0].Status, "the reply shows it ran")
}

type scriptedAnswer struct {
	text string
	tool string
	args string
}

// scriptedLLM is an OpenAI-compatible streaming endpoint that gives its answers in order: text, or a tool call.
type scriptedLLM struct {
	*httptest.Server
	mu       sync.Mutex
	script   []scriptedAnswer
	received []map[string]any
}

func newScriptedLLM(t *testing.T, script []scriptedAnswer) *scriptedLLM {
	s := &scriptedLLM{script: script}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/models") {
			fmt.Fprint(w, `{"data":[{"id":"agent-model"}]}`)
			return
		}
		var body map[string]any
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		s.mu.Lock()
		n := len(s.received)
		s.received = append(s.received, body)
		answer := scriptedAnswer{text: "That's all."}
		if n < len(s.script) {
			answer = s.script[n]
		}
		s.mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		if answer.tool != "" {
			args, _ := json.Marshal(answer.args)
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_%d\",\"type\":\"function\",\"function\":{\"name\":%q,\"arguments\":%s}}]}}]}\n\n", n, answer.tool, args)
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n")
		} else {
			content, _ := json.Marshal(answer.text)
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%s}}]}\n\n", content)
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *scriptedLLM) bodies() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]map[string]any(nil), s.received...)
}
