// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/v8/channels/app"
)

func (api *API) InitIPFiltering() {
	api.BaseRoutes.IPFiltering.Handle("", api.APISessionRequired(getIPFilters)).Methods(http.MethodGet)
	api.BaseRoutes.IPFiltering.Handle("", api.APISessionRequired(applyIPFilters)).Methods(http.MethodPost)
	api.BaseRoutes.IPFiltering.Handle("/my_ip", api.APISessionRequired(myIP)).Methods(http.MethodGet)
}

func getIPFilters(c *Context, w http.ResponseWriter, r *http.Request) {
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionSysconsoleReadIPFilters) {
		c.SetPermissionError(model.PermissionSysconsoleReadIPFilters)
		return
	}

	if err := json.NewEncoder(w).Encode(c.App.GetIPFilters()); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func applyIPFilters(c *Context, w http.ResponseWriter, r *http.Request) {
	if !c.App.SessionHasPermissionTo(*c.AppContext.Session(), model.PermissionSysconsoleWriteIPFilters) {
		c.SetPermissionError(model.PermissionSysconsoleWriteIPFilters)
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventApplyIPFilters, model.AuditStatusFail)
	defer c.LogAuditRecWithLevel(auditRec, app.LevelContent)

	var ranges model.AllowedIPRanges
	if err := json.NewDecoder(r.Body).Decode(&ranges); err != nil {
		c.SetInvalidParamWithErr("ip_filters", err)
		return
	}

	model.AddEventParameterAuditableToAuditRec(auditRec, "IPFilter", &ranges)

	applied, appErr := c.App.ApplyIPFilters(c.AppContext, ranges, c.AppContext.IPAddress())
	if appErr != nil {
		c.Err = appErr
		return
	}

	auditRec.Success()

	userID := c.AppContext.Session().UserId
	c.App.Srv().Go(func() {
		if err := c.App.SendIPFiltersChangedEmail(c.AppContext, userID); err != nil {
			c.Logger.Warn("Failed to send IP filters changed email", mlog.Err(err))
		}
	})

	if err := json.NewEncoder(w).Encode(applied); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func myIP(c *Context, w http.ResponseWriter, r *http.Request) {
	response := &model.GetIPAddressResponse{
		IP: c.AppContext.IPAddress(),
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}
