// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/mattermost/mattermost/server/public/model"
)

// Antimatter: model connections, at /api/v4/ai/connections. System admins (manage_ai_connections) manage the
// server-wide ones; a community's admins (manage_team_ai_connections) their community's own. Keys are write-only:
// no answer ever carries one, only api_key_set and api_key_last4. Anyone signed in lists the connections they may
// use where they are (/usable).

func (api *API) InitAIConnections() {
	connections := api.BaseRoutes.APIRoot.PathPrefix("/ai/connections").Subrouter()
	connections.Handle("", api.APISessionRequired(getAIConnections)).Methods(http.MethodGet)
	connections.Handle("", api.APISessionRequired(createAIConnection)).Methods(http.MethodPost)
	connections.Handle("/test", api.APISessionRequired(testAIConnectionRequest)).Methods(http.MethodPost)
	connections.Handle("/models", api.APISessionRequired(getAIConnectionRequestModels)).Methods(http.MethodPost)
	connections.Handle("/usable", api.APISessionRequired(getUsableAIConnections)).Methods(http.MethodGet)

	connection := connections.PathPrefix("/{connection_id:[A-Za-z0-9]+}").Subrouter()
	connection.Handle("", api.APISessionRequired(getAIConnection)).Methods(http.MethodGet)
	connection.Handle("", api.APISessionRequired(patchAIConnection)).Methods(http.MethodPatch, http.MethodPut)
	connection.Handle("", api.APISessionRequired(deleteAIConnection)).Methods(http.MethodDelete)
	connection.Handle("/test", api.APISessionRequired(testAIConnection)).Methods(http.MethodPost)
	connection.Handle("/models", api.APISessionRequired(getAIConnectionModels)).Methods(http.MethodGet)
	connection.Handle("/usage", api.APISessionRequired(getAIConnectionUsage)).Methods(http.MethodGet)
}

// aiConnectionScopePermission is the permission that manages a scope, for the error message.
func aiConnectionScopePermission(scope string) *model.Permission {
	if scope == model.AIConnectionScopeServer {
		return model.PermissionManageAIConnections
	}
	return model.PermissionManageTeamAIConnections
}

// managedAIConnection gives the connection of the request when the session may manage it.
func managedAIConnection(c *Context, r *http.Request) *model.AIConnection {
	id := mux.Vars(r)["connection_id"]
	if !model.IsValidId(id) {
		c.SetInvalidURLParam("connection_id")
		return nil
	}
	connection, appErr := c.App.GetAIConnection(c.AppContext, id)
	if appErr != nil {
		c.Err = appErr
		return nil
	}
	if !c.App.SessionCanManageAIConnectionScope(*c.AppContext.Session(), connection.Scope) {
		c.SetPermissionError(aiConnectionScopePermission(connection.Scope))
		return nil
	}
	return connection
}

func getAIConnections(c *Context, w http.ResponseWriter, r *http.Request) {
	scope := r.URL.Query().Get("scope")
	if scope != "" && scope != model.AIConnectionScopeServer && !model.IsValidId(scope) {
		c.SetInvalidParam("scope")
		return
	}
	if scope != "" && !c.App.SessionCanManageAIConnectionScope(*c.AppContext.Session(), scope) {
		c.SetPermissionError(aiConnectionScopePermission(scope))
		return
	}
	connections, appErr := c.App.GetAIConnectionsForManager(c.AppContext, *c.AppContext.Session(), scope)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, connections)
}

func getUsableAIConnections(c *Context, w http.ResponseWriter, r *http.Request) {
	channelID, teamID := r.URL.Query().Get("channel_id"), r.URL.Query().Get("team_id")
	if channelID != "" && !model.IsValidId(channelID) {
		c.SetInvalidParam("channel_id")
		return
	}
	if teamID != "" && !model.IsValidId(teamID) {
		c.SetInvalidParam("team_id")
		return
	}
	infos, appErr := c.App.GetAIConnectionsForUser(c.AppContext, c.AppContext.Session().UserId, channelID, teamID)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, infos)
}

func decodeAIConnectionRequest(c *Context, w http.ResponseWriter, r *http.Request) *model.AIConnectionRequest {
	var req model.AIConnectionRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256*1024)).Decode(&req); err != nil {
		c.SetInvalidParamWithErr("connection", err)
		return nil
	}
	if req.Scope == "" {
		req.Scope = model.AIConnectionScopeServer
	}
	if req.Scope != model.AIConnectionScopeServer && !model.IsValidId(req.Scope) {
		c.SetInvalidParam("scope")
		return nil
	}
	if !c.App.SessionCanManageAIConnectionScope(*c.AppContext.Session(), req.Scope) {
		c.SetPermissionError(aiConnectionScopePermission(req.Scope))
		return nil
	}
	return &req
}

func createAIConnection(c *Context, w http.ResponseWriter, r *http.Request) {
	auditRec := c.MakeAuditRecord(model.AuditEventCreateAIConnection, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)

	req := decodeAIConnectionRequest(c, w, r)
	if req == nil {
		return
	}
	model.AddEventParameterToAuditRec(auditRec, "scope", req.Scope)
	connection, appErr := c.App.CreateAIConnection(c.AppContext, req, c.AppContext.Session().UserId)
	if appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.AddEventResultState(connection)
	auditRec.AddEventObjectType("ai_connection")
	auditRec.Success()
	w.WriteHeader(http.StatusCreated)
	writeJSON(c, w, connection)
}

func testAIConnectionRequest(c *Context, w http.ResponseWriter, r *http.Request) {
	auditRec := c.MakeAuditRecord(model.AuditEventTestAIConnection, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)

	req := decodeAIConnectionRequest(c, w, r)
	if req == nil {
		return
	}
	model.AddEventParameterToAuditRec(auditRec, "scope", req.Scope)
	model.AddEventParameterToAuditRec(auditRec, "base_url", req.BaseURL)
	result, appErr := c.App.TestAIConnectionRequest(c.AppContext, req)
	if appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.Success()
	writeJSON(c, w, result)
}

// getAIConnectionRequestModels lists the models of a connection before it's saved, for the form's model fields: the
// same body, permission and outgoing-request rules as a test before saving. Nothing is stored.
func getAIConnectionRequestModels(c *Context, w http.ResponseWriter, r *http.Request) {
	auditRec := c.MakeAuditRecord(model.AuditEventTestAIConnection, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)

	req := decodeAIConnectionRequest(c, w, r)
	if req == nil {
		return
	}
	model.AddEventParameterToAuditRec(auditRec, "scope", req.Scope)
	model.AddEventParameterToAuditRec(auditRec, "base_url", req.BaseURL)
	model.AddEventParameterToAuditRec(auditRec, "operation", "models")
	models, appErr := c.App.ListAIConnectionRequestModels(c.AppContext, req)
	if appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.Success()
	writeJSON(c, w, models)
}

func getAIConnection(c *Context, w http.ResponseWriter, r *http.Request) {
	connection := managedAIConnection(c, r)
	if connection == nil {
		return
	}
	writeJSON(c, w, connection)
}

func patchAIConnection(c *Context, w http.ResponseWriter, r *http.Request) {
	auditRec := c.MakeAuditRecord(model.AuditEventPatchAIConnection, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)

	connection := managedAIConnection(c, r)
	if connection == nil {
		return
	}
	var patch model.AIConnectionPatch
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256*1024)).Decode(&patch); err != nil {
		c.SetInvalidParamWithErr("patch", err)
		return
	}
	auditRec.AddEventPriorState(connection)
	auditRec.AddEventObjectType("ai_connection")
	model.AddEventParameterToAuditRec(auditRec, "api_key_changed", patch.APIKey != nil)
	model.AddEventParameterToAuditRec(auditRec, "extra_headers_changed", patch.ExtraHeaders != nil)

	updated, appErr := c.App.PatchAIConnection(c.AppContext, connection, &patch, c.AppContext.Session().UserId)
	if appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.AddEventResultState(updated)
	auditRec.Success()
	writeJSON(c, w, updated)
}

func deleteAIConnection(c *Context, w http.ResponseWriter, r *http.Request) {
	auditRec := c.MakeAuditRecord(model.AuditEventDeleteAIConnection, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)

	connection := managedAIConnection(c, r)
	if connection == nil {
		return
	}
	auditRec.AddEventPriorState(connection)
	auditRec.AddEventObjectType("ai_connection")
	if appErr := c.App.DeleteAIConnection(c.AppContext, connection); appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.Success()
	ReturnStatusOK(w)
}

func testAIConnection(c *Context, w http.ResponseWriter, r *http.Request) {
	auditRec := c.MakeAuditRecord(model.AuditEventTestAIConnection, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)

	connection := managedAIConnection(c, r)
	if connection == nil {
		return
	}
	model.AddEventParameterToAuditRec(auditRec, "connection_id", connection.Id)
	result := c.App.TestAIConnection(c.AppContext, connection)
	auditRec.Success()
	writeJSON(c, w, result)
}

func getAIConnectionModels(c *Context, w http.ResponseWriter, r *http.Request) {
	connection := managedAIConnection(c, r)
	if connection == nil {
		return
	}
	models, appErr := c.App.ListAIConnectionModels(c.AppContext, connection)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, models)
}

func getAIConnectionUsage(c *Context, w http.ResponseWriter, r *http.Request) {
	connection := managedAIConnection(c, r)
	if connection == nil {
		return
	}
	usage, appErr := c.App.GetAIConnectionUsage(c.AppContext, connection)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, usage)
}
