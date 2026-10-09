// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

// Antimatter channel overrides (what a role or a person may do in a channel) and role-gated channels (who sees it).

func (api *API) InitChannelOverrides() {
	api.BaseRoutes.Channel.Handle("/overrides", api.APISessionRequired(getChannelOverrides)).Methods(http.MethodGet)
	api.BaseRoutes.Channel.Handle("/overrides", api.APISessionRequired(saveChannelOverride)).Methods(http.MethodPut)
	api.BaseRoutes.Channel.Handle("/role_access", api.APISessionRequired(getChannelRoleAccess)).Methods(http.MethodGet)
	api.BaseRoutes.Channel.Handle("/role_access", api.APISessionRequired(setChannelRoleAccess)).Methods(http.MethodPut)
}

// channelForOverrides gets the channel, for those who can read it; a direct or group message has no overrides.
func channelForOverrides(c *Context) *model.Channel {
	c.RequireChannelId()
	if c.Err != nil {
		return nil
	}
	channel, appErr := c.App.GetChannel(c.AppContext, c.Params.ChannelId)
	if appErr != nil {
		c.Err = appErr
		return nil
	}
	if channel.TeamId == "" {
		c.Err = model.NewAppError("channelForOverrides", "app.channel_override.team_channel_only.app_error", nil, "", http.StatusBadRequest)
		return nil
	}
	if ok, _ := c.App.SessionHasPermissionToReadChannel(c.AppContext, *c.AppContext.Session(), channel); !ok {
		c.SetPermissionError(model.PermissionReadChannelContent)
		return nil
	}
	return channel
}

// canManageChannelAccess: the channel's admins (who manage its roles), or the team's admins.
func canManageChannelAccess(c *Context, channel *model.Channel) bool {
	if c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), channel.TeamId, model.PermissionManageTeam) {
		return true
	}
	ok, _ := c.App.SessionHasPermissionToChannel(c.AppContext, *c.AppContext.Session(), channel.Id, model.PermissionManageChannelRoles)
	return ok
}

func writeJSON(c *Context, w http.ResponseWriter, v any) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func getChannelOverrides(c *Context, w http.ResponseWriter, r *http.Request) {
	channel := channelForOverrides(c)
	if channel == nil {
		return
	}
	overrides, appErr := c.App.GetChannelOverrides(c.AppContext, channel.Id)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, overrides)
}

func saveChannelOverride(c *Context, w http.ResponseWriter, r *http.Request) {
	channel := channelForOverrides(c)
	if channel == nil {
		return
	}
	var override model.ChannelOverride
	if err := json.NewDecoder(r.Body).Decode(&override); err != nil {
		c.SetInvalidParamWithErr("override", err)
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventSaveChannelOverride, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "channel_id", channel.Id)
	model.AddEventParameterToAuditRec(auditRec, "target_id", override.TargetId)

	if !canManageChannelAccess(c, channel) {
		c.SetPermissionError(model.PermissionManageChannelRoles)
		return
	}
	// The channel's admins who aren't the team's change only what they may do themselves there, as Discord's
	// overwrites: no allowing themselves more, nor taking from moderators what they don't hold.
	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), channel.TeamId, model.PermissionManageTeam) {
		changed, appErr := c.App.ChangedOverridePermissions(c.AppContext, channel.Id, &override)
		if appErr != nil {
			c.Err = appErr
			return
		}
		for _, p := range changed {
			if ok, _ := c.App.SessionHasPermissionToChannel(c.AppContext, *c.AppContext.Session(), channel.Id, &model.Permission{Id: p}); !ok {
				c.SetPermissionError(&model.Permission{Id: p})
				return
			}
		}
	}
	saved, appErr := c.App.SaveChannelOverride(c.AppContext, channel, &override)
	if appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.Success()
	writeJSON(c, w, saved)
}

type channelRoleAccess struct {
	GroupIds []string `json:"group_ids"`
}

func getChannelRoleAccess(c *Context, w http.ResponseWriter, r *http.Request) {
	channel := channelForOverrides(c)
	if channel == nil {
		return
	}
	ids, appErr := c.App.GetChannelRoleAccess(channel)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, channelRoleAccess{GroupIds: ids})
}

func setChannelRoleAccess(c *Context, w http.ResponseWriter, r *http.Request) {
	channel := channelForOverrides(c)
	if channel == nil {
		return
	}
	var body channelRoleAccess
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		c.SetInvalidParamWithErr("group_ids", err)
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventSetChannelRoleAccess, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "channel_id", channel.Id)
	model.AddEventParameterToAuditRec(auditRec, "group_ids", body.GroupIds)

	if !canManageChannelAccess(c, channel) {
		c.SetPermissionError(model.PermissionManageChannelRoles)
		return
	}
	if appErr := c.App.SetChannelRoleAccess(c.AppContext, channel, body.GroupIds); appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.Success()
	ids, appErr := c.App.GetChannelRoleAccess(channel)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, channelRoleAccess{GroupIds: ids})
}
