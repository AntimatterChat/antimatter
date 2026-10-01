// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// Inline replies answer one message in the channel, quoting it above the reply, outside of the thread model: the
// reply's reply_to prop holds the id of the message it answers.

func (a *App) inlineRepliesEnabled() bool {
	return model.SafeDereference(a.Config().ServiceSettings.EnableInlineReplies)
}

// sanitizeReplyToProp drops a post's reply_to prop unless it names a message the post may quote. On an edit
// (oldPost set), an unchanged reference is kept even if the quoted message was deleted since, and a changed one that
// isn't valid is put back to what it was.
func (a *App) sanitizeReplyToProp(rctx request.CTX, post, oldPost *model.Post) {
	value, ok := post.GetProps()[model.PostPropsReplyTo]
	if !ok {
		return
	}

	var oldValue string
	if oldPost != nil {
		oldValue = oldPost.GetReplyToProp()
		if newValue, isString := value.(string); isString && newValue == oldValue {
			return
		}
	}

	targetID, _ := value.(string)
	if a.inlineRepliesEnabled() && a.isValidReplyTarget(rctx, post, targetID) {
		return
	}

	rctx.Logger().Debug("Dropping an invalid inline reply reference", mlog.String("post_id", post.Id), mlog.String("reply_to", targetID))
	if oldValue != "" {
		post.AddProp(model.PostPropsReplyTo, oldValue)
	} else {
		post.DelProp(model.PostPropsReplyTo)
	}
}

// isValidReplyTarget tells whether post may quote the message targetID: an existing message of the same channel
// (of the same thread, when post is a thread reply) that people can read and reply to.
func (a *App) isValidReplyTarget(rctx request.CTX, post *model.Post, targetID string) bool {
	if !model.IsValidId(targetID) || targetID == post.Id || post.IsSystemMessage() {
		return false
	}

	// Read from the master: people reply to messages that were just posted.
	target, err := a.Srv().Store().Post().GetSingle(RequestContextWithMaster(rctx), targetID, false)
	if err != nil {
		return false
	}

	if target.ChannelId != post.ChannelId || target.IsSystemMessage() || target.Type == model.PostTypeBurnOnRead {
		return false
	}

	if post.RootId != "" && target.Id != post.RootId && target.RootId != post.RootId {
		return false
	}

	return true
}
