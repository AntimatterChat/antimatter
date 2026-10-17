// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package api4

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

// Message actions (Antimatter): outgoing webhooks triggered from the message menu. Listing and running them needs
// only read access to the channel or message; creating them needs the outgoing webhook permissions.
func (api *API) InitMessageAction() {
	api.BaseRoutes.Channel.Handle("/message_actions", api.APISessionRequired(getChannelMessageActions)).Methods(http.MethodGet)
	api.BaseRoutes.Post.Handle("/message_actions/{hook_id:[A-Za-z0-9]+}", api.APISessionRequired(runMessageAction)).Methods(http.MethodPost)

	// An outgoing webhook's uploaded icon: the icon of its message action, and of the posts it answers with.
	api.BaseRoutes.OutgoingHook.Handle("/icon", api.APISessionRequired(getOutgoingHookIcon)).Methods(http.MethodGet)
	api.BaseRoutes.OutgoingHook.Handle("/icon", api.APISessionRequired(setOutgoingHookIcon)).Methods(http.MethodPost)
}

func getChannelMessageActions(c *Context, w http.ResponseWriter, r *http.Request) {
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

	user, appErr := c.App.GetUser(c.AppContext, c.AppContext.Session().UserId)
	if appErr != nil {
		c.Err = appErr
		return
	}

	actions, appErr := c.App.GetMessageActionsForChannel(c.AppContext, channel, user)
	if appErr != nil {
		c.Err = appErr
		return
	}

	if err := json.NewEncoder(w).Encode(actions); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func runMessageAction(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequirePostId().RequireHookId()
	if c.Err != nil {
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventRunMessageAction, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	model.AddEventParameterToAuditRec(auditRec, "hook_id", c.Params.HookId)
	model.AddEventParameterToAuditRec(auditRec, "post_id", c.Params.PostId)

	// Only people who can read the message may send it.
	post, appErr, _ := c.App.GetPostIfAuthorized(c.AppContext, c.Params.PostId, c.AppContext.Session(), false)
	if appErr != nil {
		c.Err = appErr
		return
	}

	channel, appErr := c.App.GetChannel(c.AppContext, post.ChannelId)
	if appErr != nil {
		c.Err = appErr
		return
	}
	auditRec.AddMeta("channel_id", channel.Id)

	user, appErr := c.App.GetUser(c.AppContext, c.AppContext.Session().UserId)
	if appErr != nil {
		c.Err = appErr
		return
	}

	result, appErr := c.App.RunMessageAction(c.AppContext, c.Params.HookId, post, channel, user)
	if appErr != nil {
		c.Err = appErr
		return
	}

	auditRec.Success()

	if err := json.NewEncoder(w).Encode(result); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}

func getOutgoingHookIcon(c *Context, w http.ResponseWriter, r *http.Request) {
	c.RequireHookId()
	if c.Err != nil {
		return
	}

	hook, appErr := c.App.GetOutgoingWebhook(c.Params.HookId)
	if appErr != nil {
		c.Err = appErr
		return
	}

	// The icon is shown in the message menus of the hook's community.
	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), hook.TeamId, model.PermissionViewTeam) {
		c.SetPermissionError(model.PermissionViewTeam)
		return
	}

	data, appErr := c.App.GetOutgoingWebhookIcon(hook)
	if appErr != nil {
		c.Err = appErr
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if _, err := w.Write(data); err != nil {
		c.Logger.Warn("Error while writing the outgoing webhook's icon", mlog.Err(err))
	}
}

func setOutgoingHookIcon(c *Context, w http.ResponseWriter, r *http.Request) {
	defer func() {
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			c.Logger.Warn("Error discarding request body", mlog.Err(err))
		}
	}()

	c.RequireHookId()
	if c.Err != nil {
		return
	}

	hook, appErr := c.App.GetOutgoingWebhook(c.Params.HookId)
	if appErr != nil {
		c.Err = appErr
		return
	}

	auditRec := c.MakeAuditRecord(model.AuditEventSetOutgoingHookIcon, model.AuditStatusFail)
	defer c.LogAuditRec(auditRec)
	auditRec.AddMeta("hook_id", hook.Id)
	auditRec.AddMeta("team_id", hook.TeamId)

	// The same permissions as updating the hook.
	if !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), hook.TeamId, model.PermissionManageOwnOutgoingWebhooks) {
		c.SetPermissionError(model.PermissionManageOwnOutgoingWebhooks)
		return
	}
	if c.AppContext.Session().UserId != hook.CreatorId && !c.App.SessionHasPermissionToTeam(*c.AppContext.Session(), hook.TeamId, model.PermissionManageOthersOutgoingWebhooks) {
		c.SetPermissionError(model.PermissionManageOthersOutgoingWebhooks)
		return
	}

	if *c.App.Config().FileSettings.DriverName == "" {
		c.Err = model.NewAppError("setOutgoingHookIcon", "api.user.upload_profile_user.storage.app_error", nil, "", http.StatusNotImplemented)
		return
	}

	if r.ContentLength > *c.App.Config().FileSettings.MaxFileSize {
		c.Err = model.NewAppError("setOutgoingHookIcon", "api.outgoing_webhook.icon.too_large.app_error", nil, "", http.StatusRequestEntityTooLarge)
		return
	}

	if err := r.ParseMultipartForm(*c.App.Config().FileSettings.MaxFileSize); err != nil {
		c.Err = model.NewAppError("setOutgoingHookIcon", "api.user.upload_profile_user.parse.app_error", nil, "", http.StatusBadRequest).Wrap(err)
		return
	}

	images := r.MultipartForm.File["image"]
	if len(images) == 0 || images[0] == nil {
		c.Err = model.NewAppError("setOutgoingHookIcon", "api.user.upload_profile_user.no_file.app_error", nil, "", http.StatusBadRequest)
		return
	}

	file, err := images[0].Open()
	if err != nil {
		c.Err = model.NewAppError("setOutgoingHookIcon", "api.user.upload_profile_user.open.app_error", nil, "", http.StatusBadRequest).Wrap(err)
		return
	}
	defer file.Close()

	updated, appErr := c.App.SetOutgoingWebhookIcon(c.AppContext, hook, file)
	if appErr != nil {
		c.Err = appErr
		return
	}

	auditRec.Success()

	if err := json.NewEncoder(w).Encode(updated); err != nil {
		c.Logger.Warn("Error while writing response", mlog.Err(err))
	}
}
