// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/mattermost/mattermost/server/public/model"
)

// Antimatter moderation: a team's timeouts, the mod view of one of its members, its bans and its invite links.

func (api *API) InitModeration() {
	api.BaseRoutes.Team.Handle("/timeouts", api.APISessionRequired(getTeamTimeouts)).Methods(http.MethodGet)
	api.BaseRoutes.TeamMember.Handle("/timeout", api.APISessionRequired(setMemberTimeout)).Methods(http.MethodPut)
	api.BaseRoutes.TeamMember.Handle("/moderation", api.APISessionRequired(getMemberModeration)).Methods(http.MethodGet)

	api.BaseRoutes.Team.Handle("/bans", api.APISessionRequired(getTeamBans)).Methods(http.MethodGet)
	api.BaseRoutes.Team.Handle("/bans/{user_id:[A-Za-z0-9]+}", api.APISessionRequired(banTeamMember)).Methods(http.MethodPut)
	api.BaseRoutes.Team.Handle("/bans/{user_id:[A-Za-z0-9]+}", api.APISessionRequired(unbanTeamMember)).Methods(http.MethodDelete)

	api.BaseRoutes.Team.Handle("/invite_links", api.APISessionRequired(getTeamInviteLinks)).Methods(http.MethodGet)
	api.BaseRoutes.Team.Handle("/invite_links", api.APISessionRequired(createTeamInviteLink)).Methods(http.MethodPost)
	api.BaseRoutes.Team.Handle("/invite_links/{link_id:[A-Za-z0-9]+}", api.APISessionRequired(revokeTeamInviteLink)).Methods(http.MethodDelete)
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

// getTeamBans lists who's banned from a team, for those who may ban (its admins, and those who may remove members).
func getTeamBans(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamId()
	if c.Err != nil {
		return
	}
	if !c.App.SessionCanSeeBans(*c.AppContext.Session(), c.Params.TeamId) {
		c.SetPermissionError(model.PermissionRemoveUserFromTeam)
		return
	}
	bans, appErr := c.App.GetTeamBans(c.AppContext, c.Params.TeamId)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, bans)
}

// banTeamMember bans someone from a team (removing them from it), and may delete their recent messages there.
func banTeamMember(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamId().RequireUserId()
	if c.Err != nil {
		return
	}
	var request model.MemberBanRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		c.SetInvalidParamWithErr("ban", err)
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventBanTeamMember, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "team_id", c.Params.TeamId)
	model.AddEventParameterToAuditRec(auditRec, "user_id", c.Params.UserId)
	model.AddEventParameterToAuditRec(auditRec, "expire_at", request.ExpireAt)
	model.AddEventParameterToAuditRec(auditRec, "delete_messages_seconds", request.DeleteMessagesSeconds)

	result, appErr := c.App.BanMember(c.AppContext, *c.AppContext.Session(), c.Params.TeamId, c.Params.UserId, &request)
	if appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.Success()
	auditRec.AddMeta("deleted_posts", result.DeletedPosts)
	if result.PurgeJobId != "" {
		auditRec.AddMeta("purge_job_id", result.PurgeJobId)
	}
	writeJSON(c, w, result)
}

// unbanTeamMember lifts someone's ban from a team.
func unbanTeamMember(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamId().RequireUserId()
	if c.Err != nil {
		return
	}
	if !c.App.SessionCanSeeBans(*c.AppContext.Session(), c.Params.TeamId) {
		c.SetPermissionError(model.PermissionRemoveUserFromTeam)
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventUnbanTeamMember, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "team_id", c.Params.TeamId)
	model.AddEventParameterToAuditRec(auditRec, "user_id", c.Params.UserId)

	if appErr := c.App.UnbanMember(c.AppContext, *c.AppContext.Session(), c.Params.TeamId, c.Params.UserId); appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.Success()
	ReturnStatusOK(w)
}

// getTeamInviteLinks lists a team's invite links: all of them for its admins, their own for the others who may invite.
func getTeamInviteLinks(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamId()
	if c.Err != nil {
		return
	}
	links, appErr := c.App.GetTeamInviteLinks(c.AppContext, *c.AppContext.Session(), c.Params.TeamId)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, links)
}

// createTeamInviteLink makes one of a team's invite links, for those who may invite to it.
func createTeamInviteLink(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamId()
	if c.Err != nil {
		return
	}
	var request model.TeamInviteLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		c.SetInvalidParamWithErr("invite_link", err)
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventCreateTeamInviteLink, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "team_id", c.Params.TeamId)
	model.AddEventParameterToAuditRec(auditRec, "expire_at", request.ExpireAt)
	model.AddEventParameterToAuditRec(auditRec, "max_uses", request.MaxUses)

	link, appErr := c.App.CreateTeamInviteLink(c.AppContext, *c.AppContext.Session(), c.Params.TeamId, &request)
	if appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.Success()
	auditRec.AddMeta("link_id", link.Id)
	w.WriteHeader(http.StatusCreated)
	writeJSON(c, w, link)
}

// revokeTeamInviteLink revokes one of a team's invite links: its creator, or the team's admins.
func revokeTeamInviteLink(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamId()
	if c.Err != nil {
		return
	}
	linkID := mux.Vars(r)["link_id"]
	if !model.IsValidId(linkID) {
		c.SetInvalidURLParam("link_id")
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventRevokeTeamInviteLink, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "team_id", c.Params.TeamId)
	model.AddEventParameterToAuditRec(auditRec, "link_id", linkID)

	if appErr := c.App.RevokeTeamInviteLink(c.AppContext, *c.AppContext.Session(), c.Params.TeamId, linkID); appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.Success()
	ReturnStatusOK(w)
}
