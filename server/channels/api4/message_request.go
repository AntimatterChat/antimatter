// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

// updateMessageRequest acts on a direct message of the session's user that waits as a message request (Antimatter):
// POST /channels/{channel_id}/message_request with {"action": "accept" | "dismiss" | "request"}. It answers with their
// channel member, which tells where the conversation stands now. Only members act on their own requests.
func updateMessageRequest(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireChannelId()
	if c.Err != nil {
		return
	}

	var body model.MessageRequestAction
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		c.SetInvalidParamWithErr("action", err)
		return
	}

	channel, appErr := c.App.GetChannel(c.AppContext, c.Params.ChannelId)
	if appErr != nil {
		c.Err = appErr
		return
	}

	member, appErr := c.App.UpdateMessageRequest(c.AppContext, channel, c.AppContext.Session().UserId, body.Action)
	if appErr != nil {
		c.Err = appErr
		return
	}

	if err := json.NewEncoder(w).Encode(member); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}
