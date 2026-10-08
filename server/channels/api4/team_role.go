// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

// getTeamRole gives the Mattermost role an Antimatter role grants, with its permissions, to its team's members.
func getTeamRole(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireGroupId()
	if c.Err != nil {
		return
	}

	group, appErr := c.App.GetGroup(c.Params.GroupId, nil, nil)
	if appErr != nil {
		c.Err = appErr
		return
	}
	if !group.IsRole() {
		c.Err = model.NewAppError("getTeamRole", "app.team_role.not_a_role.app_error", nil, "", http.StatusBadRequest)
		return
	}
	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), group.TeamId, model.PermissionViewTeam) {
		c.SetPermissionError(model.PermissionViewTeam)
		return
	}

	role, appErr := c.App.GetTeamRole(c.AppContext, group)
	if appErr != nil {
		c.Err = appErr
		return
	}
	if err := json.NewEncoder(w).Encode(role); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

type teamRolePermissions struct {
	Permissions []string `json:"permissions"`
}

// updateTeamRolePermissions sets what an Antimatter role grants, for those who may manage it.
func updateTeamRolePermissions(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireGroupId()
	if c.Err != nil {
		return
	}

	var body teamRolePermissions
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		c.SetInvalidParamWithErr("permissions", err)
		return
	}

	group, appErr := c.App.GetGroup(c.Params.GroupId, nil, nil)
	if appErr != nil {
		c.Err = appErr
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventUpdateTeamRolePermissions, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "group_id", group.Id)
	model.AddEventParameterToAuditRec(auditRec, "permissions", body.Permissions)

	if !c.App.SessionHasPermissionToGroup(*c.AppContext.Session(), group.Id, model.PermissionEditCustomGroup) {
		c.SetPermissionError(model.PermissionManageTeamRoles)
		return
	}

	role, appErr := c.App.SetTeamRolePermissions(c.AppContext, *c.AppContext.Session(), group, body.Permissions)
	if appErr != nil {
		c.Err = appErr
		return
	}

	auditRec.Success()
	if err := json.NewEncoder(w).Encode(role); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}
