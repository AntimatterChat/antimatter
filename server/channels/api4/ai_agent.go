// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/app"
)

// Antimatter: agents, at /api/v4/ai/agents. System admins manage the server-wide agents; a community's admins its
// own. An agent someone made needs manage_own_agent, someone else's manage_others_agent. No answer carries a key: an
// agent names its connection by ID and name. An agent's activity is set by the agent itself (its token) or its
// managers, and read by anyone who can see the agent.

const maxAgentActivityIDs = 200

func (api *API) InitAIAgents() {
	agents := api.BaseRoutes.APIRoot.PathPrefix("/ai/agents").Subrouter()
	agents.Handle("", api.APISessionRequired(getAIAgents)).Methods(http.MethodGet)
	agents.Handle("", api.APISessionRequired(createAIAgent)).Methods(http.MethodPost)
	agents.Handle("/connections", api.APISessionRequired(getAIAgentConnections)).Methods(http.MethodGet)
	agents.Handle("/connections/{connection_id:[A-Za-z0-9]+}/models", api.APISessionRequired(getAIAgentConnectionModels)).Methods(http.MethodGet)
	agents.Handle("/activity/ids", api.APISessionRequired(getAgentActivitiesByIds)).Methods(http.MethodPost)

	agent := agents.PathPrefix("/{agent_id:[A-Za-z0-9]+}").Subrouter()
	agent.Handle("", api.APISessionRequired(getAIAgent)).Methods(http.MethodGet)
	agent.Handle("", api.APISessionRequired(patchAIAgent)).Methods(http.MethodPatch, http.MethodPut)
	agent.Handle("", api.APISessionRequired(deleteAIAgent)).Methods(http.MethodDelete)
	agent.Handle("/enable", api.APISessionRequired(enableAIAgent)).Methods(http.MethodPost)
	agent.Handle("/disable", api.APISessionRequired(disableAIAgent)).Methods(http.MethodPost)
	agent.Handle("/activity", api.APISessionRequired(getAgentActivity)).Methods(http.MethodGet)
	agent.Handle("/activity", api.APISessionRequired(setAgentActivity)).Methods(http.MethodPut)
}

// aiAgentScopePermission is the permission that manages a scope's agents, for the error message.
func aiAgentScopePermission(scope string) *model.Permission {
	if scope == model.AIAgentScopeServer {
		return model.PermissionManageSystem
	}
	return model.PermissionManageTeam
}

func validAIAgentScope(scope string) bool {
	return scope == model.AIAgentScopeServer || model.IsValidId(scope)
}

// requestedAIAgent gives the agent of the request, whoever asks.
func requestedAIAgent(c *Context, r *http.Request) *model.AIAgent {
	id := mux.Vars(r)["agent_id"]
	if !model.IsValidId(id) {
		c.SetInvalidURLParam("agent_id")
		return nil
	}
	agent, appErr := c.App.GetAIAgent(c.AppContext, id)
	if appErr != nil {
		c.Err = appErr
		return nil
	}
	return agent
}

// managedAIAgent gives the agent of the request when the session may manage it.
func managedAIAgent(c *Context, r *http.Request) *model.AIAgent {
	agent := requestedAIAgent(c, r)
	if agent == nil {
		return nil
	}
	session := *c.AppContext.Session()
	if !c.App.SessionCanManageAIAgentScope(session, agent.Scope) {
		c.SetPermissionError(aiAgentScopePermission(agent.Scope))
		return nil
	}
	if !c.App.SessionCanManageAIAgent(session, agent) {
		c.SetPermissionError(app.AIAgentPermissionFor(session, agent))
		return nil
	}
	return agent
}

// requestedScope reads and checks the scope query parameter, which the session must manage.
func requestedScope(c *Context, r *http.Request, required bool) (string, bool) {
	scope := r.URL.Query().Get("scope")
	if scope == "" && !required {
		return "", true
	}
	if !validAIAgentScope(scope) {
		c.SetInvalidParam("scope")
		return "", false
	}
	if !c.App.SessionCanManageAIAgentScope(*c.AppContext.Session(), scope) {
		c.SetPermissionError(aiAgentScopePermission(scope))
		return "", false
	}
	return scope, true
}

func getAIAgents(c *Context, w http.ResponseWriter, r *http.Request) {
	scope, ok := requestedScope(c, r, false)
	if !ok {
		return
	}
	agents, appErr := c.App.GetAIAgentsForManager(c.AppContext, *c.AppContext.Session(), scope)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, agents)
}

func createAIAgent(c *Context, w http.ResponseWriter, r *http.Request) {
	auditRec := c.MakeAuditRecord(model.AuditEventCreateAIAgent, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)

	var req model.AIAgentRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256*1024)).Decode(&req); err != nil {
		c.SetInvalidParamWithErr("agent", err)
		return
	}
	if req.Scope == "" {
		req.Scope = model.AIAgentScopeServer
	}
	if !validAIAgentScope(req.Scope) {
		c.SetInvalidParam("scope")
		return
	}
	session := *c.AppContext.Session()
	if !c.App.SessionCanManageAIAgentScope(session, req.Scope) {
		c.SetPermissionError(aiAgentScopePermission(req.Scope))
		return
	}
	if !c.App.SessionCanCreateAIAgent(session, req.Scope) {
		c.SetPermissionError(model.PermissionManageOwnAgent)
		return
	}
	if user, appErr := c.App.GetUser(c.AppContext, session.UserId); appErr != nil || user.IsBot {
		// Bots and agents don't make agents.
		c.SetPermissionError(model.PermissionManageOwnAgent)
		return
	}
	model.AddEventParameterToAuditRec(auditRec, "scope", req.Scope)
	model.AddEventParameterToAuditRec(auditRec, "external", req.External)

	details, appErr := c.App.CreateAIAgent(c.AppContext, &req, session.UserId)
	if appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.AddEventResultState(details.AIAgent)
	auditRec.AddEventObjectType("ai_agent")
	auditRec.Success()
	w.WriteHeader(http.StatusCreated)
	writeJSON(c, w, details)
}

func getAIAgent(c *Context, w http.ResponseWriter, r *http.Request) {
	agent := managedAIAgent(c, r)
	if agent == nil {
		return
	}
	details, appErr := c.App.GetAIAgentDetails(c.AppContext, agent)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, details)
}

func patchAIAgent(c *Context, w http.ResponseWriter, r *http.Request) {
	auditRec := c.MakeAuditRecord(model.AuditEventPatchAIAgent, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)

	agent := managedAIAgent(c, r)
	if agent == nil {
		return
	}
	var patch model.AIAgentPatch
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256*1024)).Decode(&patch); err != nil {
		c.SetInvalidParamWithErr("patch", err)
		return
	}
	auditRec.AddEventPriorState(agent)
	auditRec.AddEventObjectType("ai_agent")

	details, appErr := c.App.PatchAIAgent(c.AppContext, agent, &patch, c.AppContext.Session().UserId)
	if appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.AddEventResultState(details.AIAgent)
	auditRec.Success()
	writeJSON(c, w, details)
}

func deleteAIAgent(c *Context, w http.ResponseWriter, r *http.Request) {
	auditRec := c.MakeAuditRecord(model.AuditEventDeleteAIAgent, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)

	agent := managedAIAgent(c, r)
	if agent == nil {
		return
	}
	auditRec.AddEventPriorState(agent)
	auditRec.AddEventObjectType("ai_agent")
	if appErr := c.App.DeleteAIAgent(c.AppContext, agent); appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.Success()
	ReturnStatusOK(w)
}

func setAIAgentEnabled(c *Context, w http.ResponseWriter, r *http.Request, enabled bool) {
	event := model.AuditEventDisableAIAgent
	if enabled {
		event = model.AuditEventEnableAIAgent
	}
	auditRec := c.MakeAuditRecord(event, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)

	agent := managedAIAgent(c, r)
	if agent == nil {
		return
	}
	auditRec.AddEventPriorState(agent)
	auditRec.AddEventObjectType("ai_agent")
	details, appErr := c.App.SetAIAgentEnabled(c.AppContext, agent, enabled)
	if appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.Success()
	writeJSON(c, w, details)
}

func enableAIAgent(c *Context, w http.ResponseWriter, r *http.Request) {
	setAIAgentEnabled(c, w, r, true)
}

func disableAIAgent(c *Context, w http.ResponseWriter, r *http.Request) {
	setAIAgentEnabled(c, w, r, false)
}

func getAIAgentConnections(c *Context, w http.ResponseWriter, r *http.Request) {
	scope, ok := requestedScope(c, r, true)
	if !ok {
		return
	}
	infos, appErr := c.App.GetAIAgentConnectionsForScope(c.AppContext, scope)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, infos)
}

func getAIAgentConnectionModels(c *Context, w http.ResponseWriter, r *http.Request) {
	scope, ok := requestedScope(c, r, true)
	if !ok {
		return
	}
	id := mux.Vars(r)["connection_id"]
	if !model.IsValidId(id) {
		c.SetInvalidURLParam("connection_id")
		return
	}
	connection, appErr := c.App.GetAIAgentConnectionForScope(c.AppContext, scope, id)
	if appErr != nil {
		c.Err = appErr
		return
	}
	models, appErr := c.App.ListAIConnectionModels(c.AppContext, connection)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, models)
}

func getAgentActivity(c *Context, w http.ResponseWriter, r *http.Request) {
	agent := requestedAIAgent(c, r)
	if agent == nil {
		return
	}
	readerID := c.AppContext.Session().UserId
	if canSee, appErr := c.App.UserCanSeeOtherUser(c.AppContext, readerID, agent.UserId); appErr != nil || !canSee {
		c.SetPermissionError(model.PermissionViewMembers)
		return
	}
	activity, appErr := c.App.GetAgentActivity(c.AppContext, agent.UserId, readerID)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, activity)
}

func setAgentActivity(c *Context, w http.ResponseWriter, r *http.Request) {
	agent := requestedAIAgent(c, r)
	if agent == nil {
		return
	}
	if !c.App.SessionCanSetAgentActivity(*c.AppContext.Session(), agent) {
		c.SetPermissionError(app.AIAgentPermissionFor(*c.AppContext.Session(), agent))
		return
	}
	var req model.AgentActivityRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&req); err != nil {
		c.SetInvalidParamWithErr("activity", err)
		return
	}
	if req.ThreadPostId != "" && !c.App.SessionHasPermissionToChannelByPost(*c.AppContext.Session(), req.ThreadPostId, model.PermissionReadChannelContent) {
		// An agent only points at a thread it can read.
		c.SetInvalidParam("thread_post_id")
		return
	}
	activity, appErr := c.App.SetAgentActivity(c.AppContext, agent, &req)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, activity)
}

func getAgentActivitiesByIds(c *Context, w http.ResponseWriter, r *http.Request) {
	var ids []string
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&ids); err != nil {
		c.SetInvalidParamWithErr("user_ids", err)
		return
	}
	if len(ids) == 0 || len(ids) > maxAgentActivityIDs {
		c.SetInvalidParam("user_ids")
		return
	}
	for _, id := range ids {
		if !model.IsValidId(id) {
			c.SetInvalidParam("user_ids")
			return
		}
	}
	activities, appErr := c.App.GetAgentActivities(c.AppContext, ids, c.AppContext.Session().UserId)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, activities)
}
