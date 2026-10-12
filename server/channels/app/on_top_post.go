// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"errors"
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// Antimatter: messages put on top of a channel. Several can be there at once, each for a while or until removed;
// who may pin a message to the channel and post in it may put it on top or take it off. Expired or deleted ones are
// left out when read, and forgotten the next time a message of the channel is put on top.

// GetOnTopPosts gives the messages on top of a channel, the latest put there first.
func (a *App) GetOnTopPosts(rctx request.CTX, channelID string) ([]*model.OnTopPost, *model.AppError) {
	onTop, err := a.Srv().Store().OnTopPost().GetForChannel(rctx, channelID, model.GetMillis())
	if err != nil {
		return nil, model.NewAppError("GetOnTopPosts", "app.on_top_post.get.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	return onTop, nil
}

// checkCanPutOnTop tells whether a message may go on top of its channel, or come off it: as for pinning, not in an
// archived channel or a restricted direct message, and not burn-on-read or system messages.
func (a *App) checkCanPutOnTop(rctx request.CTX, post *model.Post, channel *model.Channel) *model.AppError {
	if channel.DeleteAt != 0 {
		return model.NewAppError("checkCanPutOnTop", "app.on_top_post.archived_channel.app_error", nil, "", http.StatusBadRequest)
	}
	if post.Type == model.PostTypeBurnOnRead || post.IsSystemMessage() {
		return model.NewAppError("checkCanPutOnTop", "app.on_top_post.post_type.app_error", nil, "", http.StatusBadRequest)
	}
	restrictDM, appErr := a.CheckIfChannelIsRestrictedDM(rctx, channel)
	if appErr != nil {
		return appErr
	}
	if restrictDM {
		return model.NewAppError("checkCanPutOnTop", "app.on_top_post.restricted_dm.app_error", nil, "", http.StatusBadRequest)
	}
	return nil
}

// PutPostOnTop puts a message on top of its channel for a while (in milliseconds), or until removed (0). Putting it
// there again sets a new time, and brings it first.
func (a *App) PutPostOnTop(rctx request.CTX, post *model.Post, channel *model.Channel, userID string, duration int64) (*model.OnTopPost, *model.AppError) {
	if appErr := (&model.OnTopPostRequest{Duration: duration}).IsValid(); appErr != nil {
		return nil, appErr
	}
	if appErr := a.checkCanPutOnTop(rctx, post, channel); appErr != nil {
		return nil, appErr
	}

	now := model.GetMillis()
	onTop := &model.OnTopPost{PostId: post.Id, ChannelId: channel.Id, CreatedBy: userID, CreateAt: now}
	if duration > 0 {
		onTop.ExpiresAt = now + duration
	}
	// The store forgets the stale ones, counts and saves at once, so that concurrent puts keep to the limit.
	onTop, err := a.Srv().Store().OnTopPost().Save(rctx, onTop, model.OnTopPostsMaxPerChannel)
	if err != nil {
		var limitErr *store.ErrLimitExceeded
		if errors.As(err, &limitErr) {
			return nil, model.NewAppError("PutPostOnTop", "app.on_top_post.too_many.app_error", map[string]any{"Max": model.OnTopPostsMaxPerChannel}, "", http.StatusBadRequest).Wrap(err)
		}
		return nil, model.NewAppError("PutPostOnTop", "app.on_top_post.save.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	message := model.NewWebSocketEvent(model.WebsocketEventPostPutOnTop, "", channel.Id, "", nil, "")
	message.Add("on_top", onTop)
	a.Publish(message)
	return onTop, nil
}

// RemovePostFromTop takes a message off the top of its channel.
func (a *App) RemovePostFromTop(rctx request.CTX, post *model.Post, channel *model.Channel) *model.AppError {
	if appErr := a.checkCanPutOnTop(rctx, post, channel); appErr != nil {
		return appErr
	}
	if err := a.Srv().Store().OnTopPost().Delete(rctx, post.Id); err != nil {
		var nfErr *store.ErrNotFound
		if errors.As(err, &nfErr) {
			return model.NewAppError("RemovePostFromTop", "app.on_top_post.not_on_top.app_error", nil, "", http.StatusNotFound).Wrap(err)
		}
		return model.NewAppError("RemovePostFromTop", "app.on_top_post.delete.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	a.publishRemovedFromTop(post.Id, channel.Id)
	return nil
}

func (a *App) publishRemovedFromTop(postID, channelID string) {
	message := model.NewWebSocketEvent(model.WebsocketEventPostRemovedFromTop, "", channelID, "", nil, "")
	message.Add("post_id", postID)
	message.Add("channel_id", channelID)
	a.Publish(message)
}

// removeDeletedPostFromTop takes a deleted message off the top of its channel, if it was there.
func (a *App) removeDeletedPostFromTop(rctx request.CTX, post *model.Post) {
	if err := a.Srv().Store().OnTopPost().Delete(rctx, post.Id); err != nil {
		var nfErr *store.ErrNotFound
		if !errors.As(err, &nfErr) {
			rctx.Logger().Warn("Failed to take a deleted message off the top of its channel", mlog.String("post_id", post.Id), mlog.Err(err))
		}
		return
	}
	a.publishRemovedFromTop(post.Id, post.ChannelId)
}
