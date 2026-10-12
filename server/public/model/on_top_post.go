// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"net/http"
	"time"
)

const (
	// OnTopPostsMaxPerChannel bounds how many messages a channel keeps on top at once.
	OnTopPostsMaxPerChannel = 10

	// OnTopPostMaxDuration bounds how long a message stays on top, unless it stays until removed.
	OnTopPostMaxDuration = 30 * 24 * time.Hour
)

// OnTopPost is a message put on top of its channel (Antimatter): shown above the channel's messages, with who put it
// there, until it expires (ExpiresAt, 0 for until removed) or someone removes it.
type OnTopPost struct {
	PostId    string `json:"post_id"`
	ChannelId string `json:"channel_id"`
	CreatedBy string `json:"created_by"`
	CreateAt  int64  `json:"create_at"`
	ExpiresAt int64  `json:"expires_at"`
}

// IsActive tells whether the message is still on top at a time (in milliseconds).
func (o *OnTopPost) IsActive(now int64) bool {
	return o != nil && (o.ExpiresAt == 0 || o.ExpiresAt > now)
}

func (o *OnTopPost) Auditable() map[string]any {
	return map[string]any{
		"post_id":    o.PostId,
		"channel_id": o.ChannelId,
		"created_by": o.CreatedBy,
		"create_at":  o.CreateAt,
		"expires_at": o.ExpiresAt,
	}
}

// OnTopPostRequest puts a message on top for a while (Duration, in milliseconds), or until removed (0).
type OnTopPostRequest struct {
	Duration int64 `json:"duration"`
}

func (r *OnTopPostRequest) IsValid() *AppError {
	if r.Duration < 0 || r.Duration > OnTopPostMaxDuration.Milliseconds() {
		return NewAppError("OnTopPostRequest.IsValid", "model.on_top_post.duration.app_error", map[string]any{"MaxDays": int(OnTopPostMaxDuration.Hours() / 24)}, "", http.StatusBadRequest)
	}
	return nil
}
