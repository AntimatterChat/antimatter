// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/app"
)

// Antimatter community attributes: the user attributes a team's admins define for its members, and their values.
// See app/community_attributes.go for who may see and set what.

func (api *API) InitCommunityAttributes() {
	api.BaseRoutes.Team.Handle("/user_attributes/fields", api.APISessionRequired(getCommunityAttributes)).Methods(http.MethodGet)
	api.BaseRoutes.Team.Handle("/user_attributes/fields", api.APISessionRequired(createCommunityAttribute)).Methods(http.MethodPost)
	api.BaseRoutes.Team.Handle("/user_attributes/fields/{field_id:[A-Za-z0-9]+}", api.APISessionRequired(patchCommunityAttribute)).Methods(http.MethodPatch)
	api.BaseRoutes.Team.Handle("/user_attributes/fields/{field_id:[A-Za-z0-9]+}", api.APISessionRequired(deleteCommunityAttribute)).Methods(http.MethodDelete)
	api.BaseRoutes.Team.Handle("/user_attributes/values/search", api.APISessionRequired(searchCommunityAttributeValues)).Methods(http.MethodPost)
	api.BaseRoutes.TeamMember.Handle("/user_attributes", api.APISessionRequired(getMemberCommunityAttributeValues)).Methods(http.MethodGet)
	api.BaseRoutes.TeamMember.Handle("/user_attributes", api.APISessionRequired(patchMemberCommunityAttributeValues)).Methods(http.MethodPatch)
}

// requireCommunityMember checks that the session is in the team (or administers the system): attributes, their
// values included, are for the team's eyes.
func requireCommunityMember(c *Context) bool {
	c.RequireTeamId()
	if c.Err != nil {
		return false
	}
	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamId, model.PermissionViewTeam) {
		c.SetPermissionError(model.PermissionViewTeam)
		return false
	}
	return true
}

// requireCommunityAttributesAdmin checks that the session administers the team's attributes.
func requireCommunityAttributesAdmin(c *Context) bool {
	c.RequireTeamId()
	if c.Err != nil {
		return false
	}
	if !c.App.SessionCanManageCommunityAttributes(*c.AppContext.Session(), c.Params.TeamId) {
		c.SetPermissionError(model.PermissionManageTeam)
		return false
	}
	return true
}

func getCommunityAttributes(c *Context, w http.ResponseWriter, r *http.Request) {
	if !requireCommunityMember(c) {
		return
	}
	rctx := app.RequestContextWithCallerID(c.AppContext, sessionCallerID(c))
	fields, appErr := c.App.GetCommunityAttributes(rctx, c.Params.TeamId)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, fields)
}

func createCommunityAttribute(c *Context, w http.ResponseWriter, r *http.Request) {
	if !requireCommunityAttributesAdmin(c) {
		return
	}
	var field *model.PropertyField
	if err := json.NewDecoder(r.Body).Decode(&field); err != nil || field == nil {
		c.SetInvalidParamWithErr("property_field", err)
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventCreateCommunityAttribute, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "team_id", c.Params.TeamId)
	model.AddEventParameterAuditableToAuditRec(auditRec, "property_field", field)

	rctx := app.RequestContextWithCallerID(c.AppContext, sessionCallerID(c))
	created, appErr := c.App.CreateCommunityAttribute(rctx, c.Params.TeamId, c.AppContext.Session().UserId, field)
	if appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.AddEventResultState(created)
	auditRec.Success()
	w.WriteHeader(http.StatusCreated)
	writeJSON(c, w, created)
}

func patchCommunityAttribute(c *Context, w http.ResponseWriter, r *http.Request) {
	if !requireCommunityAttributesAdmin(c) {
		return
	}
	c.RequireFieldId()
	if c.Err != nil {
		return
	}
	var patch *model.PropertyFieldPatch
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil || patch == nil {
		c.SetInvalidParamWithErr("property_field_patch", err)
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventPatchCommunityAttribute, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "team_id", c.Params.TeamId)
	model.AddEventParameterToAuditRec(auditRec, "field_id", c.Params.FieldId)
	model.AddEventParameterAuditableToAuditRec(auditRec, "property_field_patch", patch)

	rctx := app.RequestContextWithCallerID(c.AppContext, sessionCallerID(c))
	updated, appErr := c.App.PatchCommunityAttribute(rctx, c.Params.TeamId, c.Params.FieldId, c.AppContext.Session().UserId, patch)
	if appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.AddEventResultState(updated)
	auditRec.Success()
	writeJSON(c, w, updated)
}

func deleteCommunityAttribute(c *Context, w http.ResponseWriter, r *http.Request) {
	if !requireCommunityAttributesAdmin(c) {
		return
	}
	c.RequireFieldId()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventDeleteCommunityAttribute, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "team_id", c.Params.TeamId)
	model.AddEventParameterToAuditRec(auditRec, "field_id", c.Params.FieldId)

	rctx := app.RequestContextWithCallerID(c.AppContext, sessionCallerID(c))
	if appErr := c.App.DeleteCommunityAttribute(rctx, c.Params.TeamId, c.Params.FieldId); appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.Success()
	ReturnStatusOK(w)
}

// searchCommunityAttributeValues gives the values of some of the team's members, by user then field, as the session
// may see them: the member list asks for those it shows.
func searchCommunityAttributeValues(c *Context, w http.ResponseWriter, r *http.Request) {
	if !requireCommunityMember(c) {
		return
	}
	var search model.CommunityAttributeValuesSearch
	if err := json.NewDecoder(r.Body).Decode(&search); err != nil {
		c.SetInvalidParamWithErr("user_ids", err)
		return
	}
	for _, id := range search.UserIDs {
		if !model.IsValidId(id) {
			c.SetInvalidParam("user_ids")
			return
		}
	}
	rctx := app.RequestContextWithCallerID(c.AppContext, sessionCallerID(c))
	values, appErr := c.App.GetCommunityAttributeValues(rctx, *c.AppContext.Session(), c.Params.TeamId, search.UserIDs)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, values)
}

func getMemberCommunityAttributeValues(c *Context, w http.ResponseWriter, r *http.Request) {
	if !requireCommunityMember(c) {
		return
	}
	c.RequireUserId()
	if c.Err != nil {
		return
	}
	rctx := app.RequestContextWithCallerID(c.AppContext, sessionCallerID(c))
	values, appErr := c.App.GetCommunityAttributeValues(rctx, *c.AppContext.Session(), c.Params.TeamId, []string{c.Params.UserId})
	if appErr != nil {
		c.Err = appErr
		return
	}
	mine := values[c.Params.UserId]
	if mine == nil {
		mine = map[string]json.RawMessage{}
	}
	writeJSON(c, w, mine)
}

// patchMemberCommunityAttributeValues sets some of a member's values, {field ID: value}: the team's admins set
// anyone's, members their own (except for those the admins manage).
func patchMemberCommunityAttributeValues(c *Context, w http.ResponseWriter, r *http.Request) {
	if !requireCommunityMember(c) {
		return
	}
	c.RequireUserId()
	if c.Err != nil {
		return
	}
	var updates map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		c.SetInvalidParamWithErr("values", err)
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventPatchCommunityAttributeValues, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "team_id", c.Params.TeamId)
	model.AddEventParameterToAuditRec(auditRec, "user_id", c.Params.UserId)

	rctx := app.RequestContextWithCallerID(c.AppContext, sessionCallerID(c))
	values, appErr := c.App.PatchCommunityAttributeValues(rctx, *c.AppContext.Session(), c.Params.TeamId, c.Params.UserId, updates)
	if appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.Success()
	writeJSON(c, w, values)
}
