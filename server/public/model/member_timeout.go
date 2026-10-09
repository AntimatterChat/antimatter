// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"net/http"
	"time"
	"unicode/utf8"
)

const (
	// MemberTimeoutMaxDuration bounds a timeout, as Discord's.
	MemberTimeoutMaxDuration = 28 * 24 * time.Hour

	// MemberTimeoutReasonMaxRunes bounds a timeout's reason.
	MemberTimeoutReasonMaxRunes = 512
)

// MemberTimeout keeps someone of a team from speaking in it until a time (Antimatter), as Discord's timeouts: no
// messages, reactions, files or mentions in the team's channels. Direct messages aren't affected.
type MemberTimeout struct {
	TeamId    string `json:"team_id"`
	UserId    string `json:"user_id"`
	Until     int64  `json:"until"`
	Reason    string `json:"reason"`
	CreatedBy string `json:"created_by"`
	CreateAt  int64  `json:"create_at"`
}

// MemberTimeoutPermissions are what a timeout takes away in its team's channels.
func MemberTimeoutPermissions() []string {
	return []string{
		PermissionCreatePost.Id,
		PermissionAddReaction.Id,
		PermissionUploadFile.Id,
		PermissionUseChannelMentions.Id,
		PermissionUseGroupMentions.Id,
	}
}

// IsActive tells whether the timeout still holds at a time (in milliseconds).
func (t *MemberTimeout) IsActive(now int64) bool {
	return t != nil && t.Until > now
}

func (t *MemberTimeout) IsValid(now int64) *AppError {
	if !IsValidId(t.TeamId) || !IsValidId(t.UserId) {
		return NewAppError("MemberTimeout.IsValid", "model.member_timeout.ids.app_error", nil, "", http.StatusBadRequest)
	}
	if t.Until <= now || t.Until > now+MemberTimeoutMaxDuration.Milliseconds() {
		return NewAppError("MemberTimeout.IsValid", "model.member_timeout.until.app_error", map[string]any{"MaxDays": int(MemberTimeoutMaxDuration.Hours() / 24)}, "", http.StatusBadRequest)
	}
	if utf8.RuneCountInString(t.Reason) > MemberTimeoutReasonMaxRunes {
		return NewAppError("MemberTimeout.IsValid", "model.member_timeout.reason.app_error", map[string]any{"Max": MemberTimeoutReasonMaxRunes}, "", http.StatusBadRequest)
	}
	return nil
}

// MemberTimeoutRequest times someone out until a time, or lifts their timeout (until 0).
type MemberTimeoutRequest struct {
	Until  int64  `json:"until"`
	Reason string `json:"reason"`
}

// MemberModeration is what moderators see of someone in a team (the mod view): when they joined it, their timeout,
// and how much they've posted in it lately.
type MemberModeration struct {
	TeamId        string         `json:"team_id"`
	UserId        string         `json:"user_id"`
	JoinedAt      int64          `json:"joined_at"`
	Timeout       *MemberTimeout `json:"timeout,omitempty"`
	PostsLastDay  int64          `json:"posts_last_day"`
	PostsLastWeek int64          `json:"posts_last_week"`
}
