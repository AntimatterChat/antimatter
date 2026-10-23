// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

package api4

import (
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
)

// Sign-in warnings (Antimatter): a user's sessions warn them when someone signs in to their account (the
// session_created event). "Disconnect" uses the revoke-session API; "It's me" confirms the sign-in here, which takes
// the warning off all their sessions. Only the user themself may confirm one of their sign-ins: not a system admin,
// who isn't sent the warning either.
func (api *API) InitSessionWarnings() {
	api.BaseRoutes.User.Handle("/sessions/acknowledge", api.APISessionRequired(acknowledgeSession)).Methods(http.MethodPost)
}

func acknowledgeSession(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireUserId()
	if c.Err != nil {
		return
	}
	if c.Params.UserId != c.AppContext.Session().UserId {
		c.SetPermissionError(model.PermissionEditOtherUsers)
		return
	}

	props := model.MapFromJSON(r.Body)
	sessionID := props["session_id"]
	if !model.IsValidId(sessionID) {
		c.SetInvalidParam("session_id")
		return
	}

	if appErr := c.App.AcknowledgeSession(c.AppContext, c.Params.UserId, sessionID); appErr != nil {
		c.Err = appErr
		return
	}

	ReturnStatusOK(w)
}
