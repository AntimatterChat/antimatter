// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
)

// Antimatter moderation: a team's timeouts, and the mod view of one of its members.

func (api *API) InitModeration() {
	api.BaseRoutes.Team.Handle("/timeouts", api.APISessionRequired(getTeamTimeouts)).Methods(http.MethodGet)
	api.BaseRoutes.TeamMember.Handle("/timeout", api.APISessionRequired(setMemberTimeout)).Methods(http.MethodPut)
	api.BaseRoutes.TeamMember.Handle("/moderation", api.APISessionRequired(getMemberModeration)).Methods(http.MethodGet)
}

// getTeamTimeouts lists who's timed out in a team, for its members: their clients keep them from speaking, and show
// it.
func getTeamTimeouts(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamId()
	if c.Err != nil {
		return
	}
	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamId, model.PermissionViewTeam) {
		c.SetPermissionError(model.PermissionViewTeam)
		return
	}
	timeouts, appErr := c.App.GetTeamTimeouts(c.AppContext, c.Params.TeamId)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, timeouts)
}

// setMemberTimeout times one of a team's members out, or lifts their timeout.
func setMemberTimeout(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamId().RequireUserId()
	if c.Err != nil {
		return
	}
	var request model.MemberTimeoutRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		c.SetInvalidParamWithErr("timeout", err)
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventSetMemberTimeout, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "team_id", c.Params.TeamId)
	model.AddEventParameterToAuditRec(auditRec, "user_id", c.Params.UserId)
	model.AddEventParameterToAuditRec(auditRec, "until", request.Until)

	timeout, appErr := c.App.SetMemberTimeout(c.AppContext, *c.AppContext.Session(), c.Params.TeamId, c.Params.UserId, &request)
	if appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.Success()
	if timeout == nil {
		ReturnStatusOK(w)
		return
	}
	writeJSON(c, w, timeout)
}

// getMemberModeration gives the mod view of one of a team's members, for its moderators.
func getMemberModeration(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamId().RequireUserId()
	if c.Err != nil {
		return
	}
	if !c.App.SessionCanModerateTeam(*c.AppContext.Session(), c.Params.TeamId) {
		c.SetPermissionError(model.PermissionRemoveUserFromTeam)
		return
	}
	moderation, appErr := c.App.GetMemberModeration(c.AppContext, c.Params.TeamId, c.Params.UserId)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, moderation)
}
