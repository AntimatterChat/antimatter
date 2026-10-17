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

// moveThreadToChannel is Antimatter's POST /posts/{post_id}/move_to_channel:
// it moves the thread of the post to another channel of the same community
// (see App.MoveThreadToChannel for who may, and what happens). Upstream's
// /posts/{post_id}/move stays behind its retired MoveThreadsEnabled flag.
func moveThreadToChannel(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequirePostId()
	if c.Err != nil {
		return
	}

	var params model.MoveThreadToChannelParams
	if jsonErr := json.NewDecoder(r.Body).Decode(&params); jsonErr != nil {
		c.SetInvalidParamWithErr("channel_id", jsonErr)
		return
	}
	if !model.IsValidId(params.ChannelId) {
		c.SetInvalidParam("channel_id")
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventMoveThread, model.AuditStatusFail)
	defer c.LogAuditRecWithLevel(auditRec, app.LevelContent)
	model.AddEventParameterToAuditRec(auditRec, "original_post_id", c.Params.PostId)
	model.AddEventParameterToAuditRec(auditRec, "to_channel_id", params.ChannelId)
	model.AddEventParameterToAuditRec(auditRec, "flatten", params.Flatten)

	// Who can't read the message can't learn anything from the answer either.
	if ok, _ := c.App.SessionHasPermissionToReadPost(c.AppContext, *c.AppContext.Session(), c.Params.PostId); !ok {
		c.SetPermissionError(model.PermissionReadChannelContent)
		return
	}

	user, appErr := c.App.GetUser(c.AppContext, c.AppContext.Session().UserId)
	if appErr != nil {
		c.Err = appErr
		return
	}

	moved, appErr := c.App.MoveThreadToChannel(c.AppContext, c.Params.PostId, params.ChannelId, params.Flatten, user)
	if appErr != nil {
		c.Err = appErr
		return
	}

	auditRec.Success()
	model.AddEventParameterToAuditRec(auditRec, "new_post_id", moved.Id)

	moved = c.App.PreparePostForClient(c.AppContext, moved, &model.PreparePostForClientOpts{IncludePriority: true})
	moved.StripActionIntegrations()
	if err := json.NewEncoder(w).Encode(moved); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}
