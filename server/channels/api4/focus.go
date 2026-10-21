// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

// Focus mode (Antimatter): someone's own, and only theirs. Unlike the status (which system admins may set for anyone),
// nobody else may start, end or read it: its summary tells what someone was sent, in direct messages and private
// channels included. An admin who must change someone's status still can, which ends their focus.
func (api *API) InitFocus() {
	api.BaseRoutes.User.Handle("/focus", api.APISessionRequired(getFocus)).Methods(http.MethodGet)
	api.BaseRoutes.User.Handle("/focus", api.APISessionRequired(startFocus)).Methods(http.MethodPost)
	api.BaseRoutes.User.Handle("/focus", api.APISessionRequired(endFocus)).Methods(http.MethodDelete)
	api.BaseRoutes.User.Handle("/focus/summary", api.APISessionRequired(getFocusSummary)).Methods(http.MethodGet)
	api.BaseRoutes.User.Handle("/focus/summary/read", api.APISessionRequired(markFocusSummaryRead)).Methods(http.MethodPost)
}

// requireOwnFocus lets through only the user themself ("me" or their own id).
func requireOwnFocus(c *Context) bool {
	c.RequireUserId()
	if c.Err != nil {
		return false
	}
	if c.Params.UserId != c.AppContext.Session().UserId {
		c.SetPermissionError(model.PermissionEditOtherUsers)
		return false
	}
	return true
}

func writeFocusJSON(c *Context, w http.ResponseWriter, v any) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func getFocus(c *Context, w http.ResponseWriter, r *http.Request) {
	if !requireOwnFocus(c) {
		return
	}
	state, appErr := c.App.GetFocus(c.AppContext, c.Params.UserId)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeFocusJSON(c, w, state)
}

func startFocus(c *Context, w http.ResponseWriter, r *http.Request) {
	if !requireOwnFocus(c) {
		return
	}
	var focusRequest model.FocusRequest
	if err := json.NewDecoder(r.Body).Decode(&focusRequest); err != nil {
		c.SetInvalidParamWithErr("focus", err)
		return
	}
	state, appErr := c.App.StartFocus(c.AppContext, c.Params.UserId, &focusRequest)
	if appErr != nil {
		c.Err = appErr
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeFocusJSON(c, w, state)
}

func endFocus(c *Context, w http.ResponseWriter, r *http.Request) {
	if !requireOwnFocus(c) {
		return
	}
	state, appErr := c.App.EndFocus(c.AppContext, c.Params.UserId)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeFocusJSON(c, w, state)
}

func focusWindowFromQuery(c *Context, r *http.Request) *model.FocusWindow {
	query := r.URL.Query()
	since, err := strconv.ParseInt(query.Get("since"), 10, 64)
	if err != nil {
		c.SetInvalidURLParam("since")
		return nil
	}
	until, err := strconv.ParseInt(query.Get("until"), 10, 64)
	if err != nil {
		c.SetInvalidURLParam("until")
		return nil
	}
	return &model.FocusWindow{Since: since, Until: until}
}

func getFocusSummary(c *Context, w http.ResponseWriter, r *http.Request) {
	if !requireOwnFocus(c) {
		return
	}
	window := focusWindowFromQuery(c, r)
	if c.Err != nil {
		return
	}
	summary, appErr := c.App.GetFocusSummary(c.AppContext, c.Params.UserId, window)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeFocusJSON(c, w, summary)
}

func markFocusSummaryRead(c *Context, w http.ResponseWriter, r *http.Request) {
	if !requireOwnFocus(c) {
		return
	}
	var window model.FocusWindow
	if err := json.NewDecoder(r.Body).Decode(&window); err != nil {
		c.SetInvalidParamWithErr("window", err)
		return
	}
	if appErr := c.App.MarkFocusSummaryRead(c.AppContext, c.Params.UserId, c.AppContext.Session().Id, &window); appErr != nil {
		c.Err = appErr
		return
	}
	ReturnStatusOK(w)
}
