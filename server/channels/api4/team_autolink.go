// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"net/http"
	"unicode/utf8"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/v8/channels/app"
)

// Antimatter: a team's autolinks are set with the team patch API (Team.Autolinks); this previews them.
func (api *API) InitTeamAutolink() {
	api.BaseRoutes.Team.Handle("/autolinks/preview", api.APISessionRequired(previewTeamAutolinks)).Methods(http.MethodPost)
}

const autolinkPreviewMaxLength = 4000

type autolinkPreviewRequest struct {
	Autolinks model.TeamAutolinks `json:"autolinks"`
	Text      string              `json:"text"`
	ChannelID string              `json:"channel_id"`
}

// previewTeamAutolinks shows what a set of autolinks (being edited, so not saved yet) makes of a text, to those who
// manage the team's settings.
func previewTeamAutolinks(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireTeamId()
	if c.Err != nil {
		return
	}
	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), c.Params.TeamId, model.PermissionManageTeam) {
		c.SetPermissionError(model.PermissionManageTeam)
		return
	}
	var req autolinkPreviewRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, model.TeamAutolinksMaxLength+2*autolinkPreviewMaxLength)).Decode(&req); err != nil {
		c.SetInvalidParamWithErr("autolinks", err)
		return
	}
	if utf8.RuneCountInString(req.Text) > autolinkPreviewMaxLength {
		c.SetInvalidParam("text")
		return
	}
	if appErr := req.Autolinks.IsValid(); appErr != nil {
		c.Err = appErr
		return
	}
	if err := json.NewEncoder(w).Encode(map[string]string{"text": app.PreviewTeamAutolinks(&req.Autolinks, req.Text, req.ChannelID)}); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}
