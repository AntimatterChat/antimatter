// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

// Package notification implements ID-loaded push notifications.
//
// With PushNotificationContents set to "id_loaded", the push proxy only ever
// sees ids. When the mobile app receives such a notification it acknowledges
// it to the server, which answers with the full notification built here.
package notification

import (
	"errors"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/channels/store"
	"github.com/mattermost/mattermost/server/v8/einterfaces"
)

func init() {
	app.RegisterNotificationInterface(func(a *app.App) einterfaces.NotificationInterface {
		return &Notification{app: a}
	})
}

// Notification implements einterfaces.NotificationInterface.
type Notification struct {
	app *app.App
}

var _ einterfaces.NotificationInterface = (*Notification)(nil)

// CheckLicense always succeeds: every feature is available in Mattermost Libre.
func (n *Notification) CheckLicense() *model.AppError {
	return nil
}

// GetNotificationMessage builds the full push notification for the post
// referenced by an ID-loaded notification acknowledgement.
//
// The caller has already checked that userID may read the post.
func (n *Notification) GetNotificationMessage(rctx request.CTX, ack *model.PushNotificationAck, userID string) (*model.PushNotification, *model.AppError) {
	a := n.app
	if ack == nil || ack.PostId == "" {
		return nil, model.NewAppError("GetNotificationMessage", "api.push_notification.id_loaded.fetch.app_error", nil, "missing post id", http.StatusBadRequest)
	}

	// Read the stored post, exactly as the regular push notification path
	// does; app.GetSinglePost would reveal burn-on-read posts to the user.
	post, err := a.Srv().Store().Post().GetSingle(rctx, ack.PostId, false)
	if err != nil {
		status := http.StatusInternalServerError
		var nfErr *store.ErrNotFound
		if errors.As(err, &nfErr) {
			status = http.StatusNotFound
		}
		return nil, model.NewAppError("GetNotificationMessage", "app.post.get.app_error", nil, "", status).Wrap(err)
	}

	channel, appErr := a.GetChannel(rctx, post.ChannelId)
	if appErr != nil {
		return nil, appErr
	}

	user, appErr := a.GetUser(rctx, userID)
	if appErr != nil {
		return nil, appErr
	}

	sender, appErr := a.GetUser(rctx, post.UserId)
	if appErr != nil {
		return nil, appErr
	}

	notification := &app.PostNotification{
		Channel: channel,
		Post:    post,
		Sender:  sender,
	}
	if channel.Type == model.ChannelTypeGroup {
		profileMap, pErr := a.Srv().Store().User().GetAllProfilesInChannel(rctx, channel.Id, true)
		if pErr != nil {
			return nil, model.NewAppError("GetNotificationMessage", "app.user.get_profiles.app_error", nil, "", http.StatusInternalServerError).Wrap(pErr)
		}
		notification.ProfileMap = profileMap
	}

	nameFormat := a.GetNotificationNameFormat(user)
	channelName := notification.GetChannelName(nameFormat, user.Id)
	senderName := notification.GetSenderName(nameFormat, model.SafeDereference(a.Config().ServiceSettings.EnablePostUsernameOverride))

	replyToThreadType := ""
	if post.RootId != "" && a.IsCRTEnabledForUser(rctx, user.Id) {
		replyToThreadType = model.CommentsNotifyCRT
	}

	// Mentions only influence the generic messages, not the full contents.
	msg, appErr := a.BuildPushNotificationMessage(rctx, model.FullNotification, post, user, channel, channelName, senderName, false, false, replyToThreadType)
	if appErr != nil {
		return nil, appErr
	}

	msg.AckId = ack.Id
	msg.Platform = ack.ClientPlatform
	msg.ServerId = a.ServerId()
	return msg, nil
}
