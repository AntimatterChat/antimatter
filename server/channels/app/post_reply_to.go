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

// sanitizeReplyToMentionProp drops a post's reply_to_mention prop unless it is a boolean on an inline reply, so that
// it is only kept with the reply_to prop it applies to. Run it after sanitizeReplyToProp.
func sanitizeReplyToMentionProp(post *model.Post) {
	value, ok := post.GetProps()[model.PostPropsReplyToMention]
	if !ok {
		return
	}
	if _, isBool := value.(bool); isBool && post.GetReplyToProp() != "" {
		return
	}
	post.DelProp(model.PostPropsReplyToMention)
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

// populateReplyToMetadata describes, in the metadata of each of posts that is an inline reply, the message it quotes.
// known holds posts already at hand, such as the rest of a post list; the others are read in one query.
func (a *App) populateReplyToMetadata(rctx request.CTX, posts []*model.Post, known map[string]*model.Post) {
	if !a.inlineRepliesEnabled() {
		return
	}

	var missing []string
	seen := map[string]bool{}
	for _, post := range posts {
		targetID := post.GetReplyToProp()
		if targetID == "" || !model.IsValidId(targetID) || seen[targetID] {
			continue
		}
		seen[targetID] = true
		if _, ok := known[targetID]; !ok {
			missing = append(missing, targetID)
		}
	}
	if len(seen) == 0 {
		return
	}

	fetched := map[string]*model.Post{}
	switch len(missing) {
	case 0:
	case 1:
		// A single post, such as a new reply being broadcast: read from the master, as the quoted message may be new.
		if target, err := a.Srv().Store().Post().GetSingle(RequestContextWithMaster(rctx), missing[0], true); err == nil {
			fetched[target.Id] = target
		}
	default:
		if targets, err := a.Srv().Store().Post().GetPostsByIds(missing); err == nil {
			for _, target := range targets {
				fetched[target.Id] = target
			}
		}
	}

	for _, post := range posts {
		targetID := post.GetReplyToProp()
		if targetID == "" || !model.IsValidId(targetID) {
			continue
		}
		target, ok := known[targetID]
		if !ok {
			target = fetched[targetID]
		}
		if post.Metadata == nil {
			post.Metadata = &model.PostMetadata{}
		}
		post.Metadata.ReplyTo = model.NewPostReplyTo(post, targetID, target)
	}
}

// inlineReplyTargetAuthor returns the author of the message post replies to inline, for them to be notified, or ""
// when there is no one to notify: the reply's author chose not to (its reply_to_mention prop is false), or the quoted
// message was posted by an incoming webhook, and belongs to whoever set the webhook up.
func (a *App) inlineReplyTargetAuthor(rctx request.CTX, post *model.Post) string {
	targetID := post.GetReplyToProp()
	if targetID == "" || !a.inlineRepliesEnabled() || !post.ReplyToMentionsAuthor() {
		return ""
	}

	target, err := a.Srv().Store().Post().GetSingle(rctx, targetID, false)
	if err != nil || target.ChannelId != post.ChannelId || target.GetProp(model.PostPropsFromWebhook) == "true" {
		return ""
	}
	return target.UserId
}
