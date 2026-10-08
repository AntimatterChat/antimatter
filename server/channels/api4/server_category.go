// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

// getServerCategories lists a team's server categories (Antimatter), in order, for its members.
func getServerCategories(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamId()
	if c.Err != nil {
		return
	}

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamId, model.PermissionViewTeam) {
		c.SetPermissionError(model.PermissionViewTeam)
		return
	}

	categories, appErr := c.App.GetServerCategories(c.AppContext, c.Params.TeamId)
	if appErr != nil {
		c.Err = appErr
		return
	}

	if err := json.NewEncoder(w).Encode(categories); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

// updateServerCategories replaces a team's server categories; team admins only.
func updateServerCategories(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamId()
	if c.Err != nil {
		return
	}

	var update model.ServerCategoriesUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		c.SetInvalidParamWithErr("server_categories", err)
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventUpdateServerCategories, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "team_id", c.Params.TeamId)

	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamId, model.PermissionManageTeam) {
		c.SetPermissionError(model.PermissionManageTeam)
		return
	}

	categories, appErr := c.App.UpdateServerCategories(c.AppContext, c.Params.TeamId, &update)
	if appErr != nil {
		c.Err = appErr
		return
	}

	auditRec.Success()
	if err := json.NewEncoder(w).Encode(categories); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}
