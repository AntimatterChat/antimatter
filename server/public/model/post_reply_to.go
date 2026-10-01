// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

// PostReplyToMessageMaxRunes is how much of the quoted message's text PostReplyTo carries.
const PostReplyToMessageMaxRunes = 300

// PostReplyTo describes the message a post replies to inline (the post's PostPropsReplyTo prop), so that clients can
// quote it above the reply without fetching it. The server fills it in the post's metadata when it sends the post.
type PostReplyTo struct {
	PostId string `json:"post_id"`
	UserId string `json:"user_id,omitempty"`

	// Message is the start of the quoted message's text, at most PostReplyToMessageMaxRunes long.
	Message  string `json:"message,omitempty"`
	Type     string `json:"type,omitempty"`
	CreateAt int64  `json:"create_at,omitempty"`
	EditAt   int64  `json:"edit_at,omitempty"`

	// FileCount is how many files the quoted message has, for clients to describe a message without text.
	FileCount int `json:"file_count,omitempty"`

	// OverrideUsername is the name an incoming webhook posted the quoted message under.
	OverrideUsername string `json:"override_username,omitempty"`

	// Deleted is set when the quoted message was deleted or can't be shown with the reply (the other fields are
	// then empty but PostId).
	Deleted bool `json:"deleted,omitempty"`
}

// GetReplyToProp returns the id of the message the post replies to inline, or "" when it isn't an inline reply.
func (o *Post) GetReplyToProp() string {
	if val, ok := o.GetProp(PostPropsReplyTo).(string); ok {
		return val
	}
	return ""
}

// NewPostReplyTo describes target, the message reply quotes. A missing or deleted target, one from another channel
// (the reply's thread was moved) or a burn-on-read message is reported as deleted, so that its text isn't shown to
// readers of the reply who may not see it.
func NewPostReplyTo(reply *Post, targetID string, target *Post) *PostReplyTo {
	if target == nil || target.Id != targetID || target.DeleteAt != 0 || target.ChannelId != reply.ChannelId || target.Type == PostTypeBurnOnRead {
		return &PostReplyTo{PostId: targetID, Deleted: true}
	}

	message := target.Message
	if runes := []rune(message); len(runes) > PostReplyToMessageMaxRunes {
		message = string(runes[:PostReplyToMessageMaxRunes-1]) + "…"
	}

	replyTo := &PostReplyTo{
		PostId:    target.Id,
		UserId:    target.UserId,
		Message:   message,
		Type:      target.Type,
		CreateAt:  target.CreateAt,
		EditAt:    target.EditAt,
		FileCount: len(target.FileIds),
	}
	if target.GetProp(PostPropsFromWebhook) == "true" {
		replyTo.OverrideUsername, _ = target.GetProp(PostPropsOverrideUsername).(string)
	}
	return replyTo
}
