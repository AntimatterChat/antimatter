// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/app"
)

// Antimatter: messages put on top of a channel, for a while or until removed. Who may read the channel sees them;
// its admins, the community's admins and moderators put them there and take them off, as do the members of a direct
// or group message (see app.SessionCanChangeOnTop).

func (api *API) InitOnTopPosts() {
	api.BaseRoutes.Channel.Handle("/on_top", api.APISessionRequired(getOnTopPosts)).Methods(http.MethodGet)
	api.BaseRoutes.Post.Handle("/on_top", api.APISessionRequired(putPostOnTop)).Methods(http.MethodPut)
	api.BaseRoutes.Post.Handle("/on_top", api.APISessionRequired(removePostFromTop)).Methods(http.MethodDelete)
}

// getOnTopPosts lists the messages on top of a channel, the latest put there first.
func getOnTopPosts(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireChannelId()
	if c.Err != nil {
		return
	}
	channel, appErr := c.App.GetChannel(c.AppContext, c.Params.ChannelId)
	if appErr != nil {
		c.Err = appErr
		return
	}
	if ok, _ := c.App.SessionHasPermissionToReadChannel(c.AppContext, *c.AppContext.Session(), channel); !ok {
		c.SetPermissionError(model.PermissionReadChannelContent)
		return
	}
	onTop, appErr := c.App.GetOnTopPosts(c.AppContext, channel.Id)
	if appErr != nil {
		c.Err = appErr
		return
	}
	writeJSON(c, w, onTop)
}

// onTopPostAndChannel gives the post of the request and its channel, when the session may put it on top: those who
// may read the channel and post in it, so that the channel's moderation (read-only channels, overrides), timeouts and
// membership apply, and who moderate it (app.SessionCanChangeOnTop).
func onTopPostAndChannel(c *Context, auditRec *model.AuditRecord) (*model.Post, *model.Channel) {
	post, appErr := c.App.GetSinglePost(c.AppContext, c.Params.PostId, false)
	if appErr != nil {
		c.SetPermissionError(model.PermissionReadChannelContent)
		return nil, nil
	}
	auditRec.AddEventObjectType("post")
	model.AddEventParameterToAuditRec(auditRec, "channel_id", post.ChannelId)
	channel, appErr := c.App.GetChannel(c.AppContext, post.ChannelId)
	if appErr != nil {
		c.Err = appErr
		return nil, nil
	}
	if ok, _ := c.App.SessionHasPermissionToReadChannel(c.AppContext, *c.AppContext.Session(), channel); !ok {
		c.SetPermissionError(model.PermissionReadChannelContent)
		return nil, nil
	}
	if appErr := c.App.SessionCanChangeOnTop(c.AppContext, *c.AppContext.Session(), channel); appErr != nil {
		c.Err = appErr
		return nil, nil
	}
	return post, channel
}

// putPostOnTop puts a message on top of its channel for a while, or until removed; putting it there again sets a new
// time.
func putPostOnTop(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequirePostId()
	if c.Err != nil {
		return
	}
	var request model.OnTopPostRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		c.SetInvalidParamWithErr("on_top", err)
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventPutPostOnTop, model.AuditStatusFail)
	defer c.LogAuditRecWithLevel(auditRec, app.LevelContent)
	model.AddEventParameterToAuditRec(auditRec, "post_id", c.Params.PostId)
	model.AddEventParameterToAuditRec(auditRec, "duration", request.Duration)

	post, channel := onTopPostAndChannel(c, auditRec)
	if post == nil {
		return
	}
	onTop, appErr := c.App.PutPostOnTop(c.AppContext, post, channel, c.AppContext.Session().UserId, request.Duration)
	if appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.AddEventResultState(onTop)
	auditRec.Success()
	writeJSON(c, w, onTop)
}

// removePostFromTop takes a message off the top of its channel.
func removePostFromTop(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequirePostId()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventRemovePostFromTop, model.AuditStatusFail)
	defer c.LogAuditRecWithLevel(auditRec, app.LevelContent)
	model.AddEventParameterToAuditRec(auditRec, "post_id", c.Params.PostId)

	post, channel := onTopPostAndChannel(c, auditRec)
	if post == nil {
		return
	}
	if appErr := c.App.RemovePostFromTop(c.AppContext, post, channel); appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.Success()
	ReturnStatusOK(w)
}
