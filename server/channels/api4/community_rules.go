// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
)

// Antimatter community rules: a community's rules, which its members agree to before they post in its channels.
//
// Who may change them: the community's admins (manage_team), as for the rest of the community's settings and its
// onboarding, which the rules are part of. A permission of their own would let a role change rules it isn't held
// to, or be held to rules it changes; and those exempt from the rules are the community's admins.

func (api *API) InitCommunityRules() {
	api.BaseRoutes.Team.Handle("/rules", api.APISessionRequired(getCommunityRules)).Methods(http.MethodGet)
	api.BaseRoutes.Team.Handle("/rules", api.APISessionRequired(updateCommunityRules)).Methods(http.MethodPut)
	api.BaseRoutes.Team.Handle("/rules/accept", api.APISessionRequired(acceptCommunityRules)).Methods(http.MethodPost)
	api.BaseRoutes.Team.Handle("/rules/stats", api.APISessionRequired(getCommunityRulesStats)).Methods(http.MethodGet)
}

// getCommunityRules gives a community's rules, with whether the one asking agreed to them, to those who may see the
// community: its members, and anyone who may join it when it's open, so they read the rules before joining.
func getCommunityRules(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamId()
	if c.Err != nil {
		return
	}
	session := *c.AppContext.Session()
	if !c.App.SessionHasPermissionToTeam(session, c.Params.TeamId, model.PermissionViewTeam) {
		team, appErr := c.App.GetTeam(c.Params.TeamId)
		if appErr != nil {
			c.Err = appErr
			return
		}
		if !team.AllowOpenInvite || team.Type != model.TeamOpen || !c.App.SessionHasPermissionTo(session, model.PermissionListPublicTeams) {
			c.SetPermissionError(model.PermissionViewTeam)
			return
		}
	}
	rules, appErr := c.App.GetCommunityRulesForUser(c.AppContext, c.Params.TeamId, session.UserId)
	if appErr != nil {
		c.Err = appErr
		return
	}
	if !c.App.SessionHasPermissionToTeam(session, c.Params.TeamId, model.PermissionManageTeam) {
		rules.UpdatedBy = ""
	}
	writeJSON(c, w, rules)
}

// updateCommunityRules turns a community's rules on or off and changes them, asking everyone to agree again or not.
func updateCommunityRules(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamId()
	if c.Err != nil {
		return
	}
	var request model.CommunityRulesRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		c.SetInvalidParamWithErr("rules", err)
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventUpdateCommunityRules, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "team_id", c.Params.TeamId)
	model.AddEventParameterToAuditRec(auditRec, "enabled", request.Enabled)
	model.AddEventParameterToAuditRec(auditRec, "ask_again", request.AskAgain)

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamId, model.PermissionManageTeam) {
		c.SetPermissionError(model.PermissionManageTeam)
		return
	}
	if _, appErr := c.App.GetTeam(c.Params.TeamId); appErr != nil {
		c.Err = appErr
		return
	}
	rules, appErr := c.App.SaveCommunityRules(c.AppContext, c.Params.TeamId, c.AppContext.Session().UserId, &request)
	if appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.Success()
	model.AddEventParameterToAuditRec(auditRec, "version", rules.Version)
	writeJSON(c, w, rules)
}

// acceptCommunityRules agrees, for the one asking, to the version of a community's rules they read.
func acceptCommunityRules(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamId()
	if c.Err != nil {
		return
	}
	var request model.CommunityRulesAcceptRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		c.SetInvalidParamWithErr("version", err)
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventAcceptCommunityRules, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "team_id", c.Params.TeamId)
	model.AddEventParameterToAuditRec(auditRec, "version", request.Version)

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamId, model.PermissionViewTeam) {
		c.SetPermissionError(model.PermissionViewTeam)
		return
	}
	rules, appErr := c.App.AcceptCommunityRules(c.AppContext, c.Params.TeamId, c.AppContext.Session().UserId, request.Version)
	if appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.Success()
	rules.UpdatedBy = ""
	writeJSON(c, w, rules)
}

// getCommunityRulesStats counts, for a community's admins, the members who agreed to its current rules.
func getCommunityRulesStats(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamId()
	if c.Err != nil {
		return
	}
	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamId, model.PermissionManageTeam) {
		c.SetPermissionError(model.PermissionManageTeam)
		return
	}
	stats, appErr := c.App.GetCommunityRulesStats(c.AppContext, c.Params.TeamId)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, stats)
}
