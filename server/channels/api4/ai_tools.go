// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"html/template"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/i18n"
)

// Antimatter: agents' tools (AI step 3).
//
//   - /api/v4/ai/agents/{agent_id}/tools: the tools an agent may use for the session's person, with what they turned
//     off and whether they signed in to the MCP servers that need it (their choices are a preference, ai_tools);
//   - /api/v4/ai/agents/tools/catalogue: what the managers of a scope's agents may give them;
//   - /api/v4/ai/tools/calls/{call_id}/confirm|decline: the person's answer to a tool call that waits for them; only
//     theirs (anyone else's is not found);
//   - /api/v4/ai/mcp_servers: MCP servers, managed like model connections; secrets are write-only;
//   - /api/v4/ai/mcp_servers/{id}/oauth: the session's person signs in to an MCP server (POST gives where to go) or
//     out (DELETE); /api/v4/ai/mcp/oauth/callback is where the authorisation server sends them back. A person's
//     sign-in is never given by any API, to anyone.

// InitAITools adds the routes of tool calls and MCP servers; an agent's tools and the catalogue are on the agents'
// routes (InitAIAgents).
func (api *API) InitAITools() {
	calls := api.BaseRoutes.APIRoot.PathPrefix("/ai/tools/calls/{call_id:[A-Za-z0-9]+}").Subrouter()
	calls.Handle("/confirm", api.APISessionRequired(confirmAIToolCall)).Methods(http.MethodPost)
	calls.Handle("/decline", api.APISessionRequired(declineAIToolCall)).Methods(http.MethodPost)

	servers := api.BaseRoutes.APIRoot.PathPrefix("/ai/mcp_servers").Subrouter()
	servers.Handle("", api.APISessionRequired(getAIMCPServers)).Methods(http.MethodGet)
	servers.Handle("", api.APISessionRequired(createAIMCPServer)).Methods(http.MethodPost)
	server := servers.PathPrefix("/{server_id:[A-Za-z0-9]+}").Subrouter()
	server.Handle("", api.APISessionRequired(getAIMCPServer)).Methods(http.MethodGet)
	server.Handle("", api.APISessionRequired(patchAIMCPServer)).Methods(http.MethodPatch, http.MethodPut)
	server.Handle("", api.APISessionRequired(deleteAIMCPServer)).Methods(http.MethodDelete)
	server.Handle("/oauth", api.APISessionRequired(connectAIMCPServer)).Methods(http.MethodPost)
	server.Handle("/oauth", api.APISessionRequired(disconnectAIMCPServer)).Methods(http.MethodDelete)

	api.BaseRoutes.APIRoot.Handle("/ai/mcp/oauth/callback", api.APISessionRequiredTrustRequester(completeAIMCPOAuth)).Methods(http.MethodGet)
}

func getAIAgentTools(c *Context, w http.ResponseWriter, r *http.Request) {
	agentID := mux.Vars(r)["agent_id"]
	if !model.IsValidId(agentID) {
		c.SetInvalidURLParam("agent_id")
		return
	}
	channelID := r.URL.Query().Get("channel_id")
	if channelID != "" && !model.IsValidId(channelID) {
		c.SetInvalidParam("channel_id")
		return
	}
	sources, appErr := c.App.GetAIAgentToolsForUser(c.AppContext, c.AppContext.Session().UserId, agentID, channelID)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, sources)
}

func getAIToolCatalogue(c *Context, w http.ResponseWriter, r *http.Request) {
	scope := r.URL.Query().Get("scope")
	if scope == "" {
		scope = model.AIAgentScopeServer
	}
	if !validAIAgentScope(scope) {
		c.SetInvalidParam("scope")
		return
	}
	if !c.App.SessionCanManageAIAgentScope(*c.AppContext.Session(), scope) {
		c.SetPermissionError(aiAgentScopePermission(scope))
		return
	}
	catalogue, appErr := c.App.GetAIToolCatalogue(c.AppContext, c.AppContext.Session().UserId, scope)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, catalogue)
}

func answerAIToolCall(c *Context, w http.ResponseWriter, r *http.Request, confirm bool) {
	callID := mux.Vars(r)["call_id"]
	if !model.IsValidId(callID) {
		c.SetInvalidURLParam("call_id")
		return
	}
	summary, appErr := c.App.AnswerAIToolCall(c.AppContext, c.AppContext.Session().UserId, callID, confirm)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, summary)
}

func confirmAIToolCall(c *Context, w http.ResponseWriter, r *http.Request) {
	answerAIToolCall(c, w, r, true)
}

func declineAIToolCall(c *Context, w http.ResponseWriter, r *http.Request) {
	answerAIToolCall(c, w, r, false)
}

// requestedAIMCPServer gives the MCP server of the request, whoever asks.
func requestedAIMCPServer(c *Context, r *http.Request) *model.AIMCPServer {
	id := mux.Vars(r)["server_id"]
	if !model.IsValidId(id) {
		c.SetInvalidURLParam("server_id")
		return nil
	}
	s, appErr := c.App.GetAIMCPServer(c.AppContext, id)
	if appErr != nil {
		c.Err = appErr
		return nil
	}
	return s
}

// managedAIMCPServer gives the MCP server of the request when the session may manage it.
func managedAIMCPServer(c *Context, r *http.Request) *model.AIMCPServer {
	s := requestedAIMCPServer(c, r)
	if s == nil {
		return nil
	}
	if !c.App.SessionCanManageAIConnectionScope(*c.AppContext.Session(), s.Scope) {
		c.SetPermissionError(aiConnectionScopePermission(s.Scope))
		return nil
	}
	return s
}

func getAIMCPServers(c *Context, w http.ResponseWriter, r *http.Request) {
	scope := r.URL.Query().Get("scope")
	if scope != "" && scope != model.AIConnectionScopeServer && !model.IsValidId(scope) {
		c.SetInvalidParam("scope")
		return
	}
	if scope != "" && !c.App.SessionCanManageAIConnectionScope(*c.AppContext.Session(), scope) {
		c.SetPermissionError(aiConnectionScopePermission(scope))
		return
	}
	servers, appErr := c.App.GetAIMCPServersForManager(c.AppContext, *c.AppContext.Session(), scope)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, servers)
}

func getAIMCPServer(c *Context, w http.ResponseWriter, r *http.Request) {
	s := managedAIMCPServer(c, r)
	if s == nil {
		return
	}
	writeJSON(c, w, s)
}

func createAIMCPServer(c *Context, w http.ResponseWriter, r *http.Request) {
	auditRec := c.MakeAuditRecord(model.AuditEventCreateAIMCPServer, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)

	var req model.AIMCPServerRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256*1024)).Decode(&req); err != nil {
		c.SetInvalidParamWithErr("mcp_server", err)
		return
	}
	if req.Scope == "" {
		req.Scope = model.AIConnectionScopeServer
	}
	if req.Scope != model.AIConnectionScopeServer && !model.IsValidId(req.Scope) {
		c.SetInvalidParam("scope")
		return
	}
	if !c.App.SessionCanManageAIConnectionScope(*c.AppContext.Session(), req.Scope) {
		c.SetPermissionError(aiConnectionScopePermission(req.Scope))
		return
	}
	model.AddEventParameterToAuditRec(auditRec, "scope", req.Scope)
	s, appErr := c.App.CreateAIMCPServer(c.AppContext, &req, c.AppContext.Session().UserId)
	if appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.AddEventResultState(s)
	auditRec.AddEventObjectType("ai_mcp_server")
	auditRec.Success()
	w.WriteHeader(http.StatusCreated)
	writeJSON(c, w, s)
}

func patchAIMCPServer(c *Context, w http.ResponseWriter, r *http.Request) {
	auditRec := c.MakeAuditRecord(model.AuditEventPatchAIMCPServer, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)

	s := managedAIMCPServer(c, r)
	if s == nil {
		return
	}
	var patch model.AIMCPServerPatch
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256*1024)).Decode(&patch); err != nil {
		c.SetInvalidParamWithErr("mcp_server", err)
		return
	}
	auditRec.AddEventPriorState(s)
	updated, appErr := c.App.PatchAIMCPServer(c.AppContext, s, &patch, c.AppContext.Session().UserId)
	if appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.AddEventResultState(updated)
	auditRec.AddEventObjectType("ai_mcp_server")
	auditRec.Success()
	writeJSON(c, w, updated)
}

func deleteAIMCPServer(c *Context, w http.ResponseWriter, r *http.Request) {
	auditRec := c.MakeAuditRecord(model.AuditEventDeleteAIMCPServer, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)

	s := managedAIMCPServer(c, r)
	if s == nil {
		return
	}
	auditRec.AddEventPriorState(s)
	if appErr := c.App.DeleteAIMCPServer(c.AppContext, s); appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.AddEventObjectType("ai_mcp_server")
	auditRec.Success()
	ReturnStatusOK(w)
}

func connectAIMCPServer(c *Context, w http.ResponseWriter, r *http.Request) {
	auditRec := c.MakeAuditRecord(model.AuditEventConnectAIMCPServer, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)

	s := requestedAIMCPServer(c, r)
	if s == nil {
		return
	}
	model.AddEventParameterToAuditRec(auditRec, "server_id", s.Id)
	user, appErr := c.App.GetUser(c.AppContext, c.AppContext.Session().UserId)
	if appErr != nil {
		c.Err = appErr
		return
	}
	connect, appErr := c.App.StartAIMCPOAuth(c.AppContext, user, s)
	if appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.Success()
	writeJSON(c, w, connect)
}

func disconnectAIMCPServer(c *Context, w http.ResponseWriter, r *http.Request) {
	auditRec := c.MakeAuditRecord(model.AuditEventDisconnectAIMCPServer, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)

	s := requestedAIMCPServer(c, r)
	if s == nil {
		return
	}
	model.AddEventParameterToAuditRec(auditRec, "server_id", s.Id)
	// Always the session's own sign-in: there's no way to name someone else's.
	if appErr := c.App.DisconnectAIMCPServer(c.AppContext, c.AppContext.Session().UserId, s); appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.Success()
	ReturnStatusOK(w)
}

var aiMCPOAuthPage = template.Must(template.New("mcp_oauth").Parse(`<!doctype html>
<html lang="{{.Lang}}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>{{.Title}}</title>
<style>body{margin:0;min-height:100vh;display:grid;place-items:center;font:15px/1.5 system-ui,sans-serif;background:#f4f4f7;color:#1d1c24}
@media (prefers-color-scheme:dark){body{background:#17161d;color:#e9e8f0}}main{max-width:28rem;padding:24px;text-align:center}h1{font-size:18px;margin:0 0 8px}</style>
</head><body><main><h1>{{.Title}}</h1><p>{{.Body}}</p></main><script>try{window.opener&&window.opener.postMessage({type:"antimatter_mcp_oauth",ok:{{.OK}}},"*")}catch(e){}setTimeout(function(){window.close()},1500)</script></body></html>`))

// completeAIMCPOAuth finishes a sign-in where the authorisation server sends the person back: a page in their
// language that closes itself.
func completeAIMCPOAuth(c *Context, w http.ResponseWriter, r *http.Request) {
	auditRec := c.MakeAuditRecord(model.AuditEventConnectAIMCPServer, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)

	query := r.URL.Query()
	userID := c.AppContext.Session().UserId
	locale := "en"
	if user, appErr := c.App.GetUser(c.AppContext, userID); appErr == nil && user.Locale != "" {
		locale = user.Locale
	}
	T := i18n.GetUserTranslations(locale)
	s, appErr := c.App.FinishAIMCPOAuth(c.AppContext, userID, query.Get("state"), query.Get("code"), query.Get("error"))
	name := ""
	if s != nil {
		name = s.Name
		model.AddEventParameterToAuditRec(auditRec, "server_id", s.Id)
	}
	page := map[string]any{"Lang": locale, "OK": appErr == nil}
	status := http.StatusOK
	if appErr == nil {
		auditRec.Success()
		page["Title"] = T("api.ai.mcp.oauth.connected.title", map[string]any{"Name": name})
		page["Body"] = T("api.ai.mcp.oauth.connected.body")
	} else {
		c.LogErrorByCode(appErr)
		status = http.StatusBadRequest
		page["Title"] = T("api.ai.mcp.oauth.failed.title")
		page["Body"] = T("api.ai.mcp.oauth.failed.body")
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.WriteHeader(status)
	_ = aiMCPOAuthPage.Execute(w, page)
}
