// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"net/http"
	"time"
	"unicode/utf8"
)

const (
	// MemberBanReasonMaxRunes bounds a ban's reason, as a timeout's.
	MemberBanReasonMaxRunes = 512

	// MemberBanPurgeMaxDuration bounds how far back a ban deletes someone's messages, as Discord's (7 days).
	MemberBanPurgeMaxDuration = 7 * 24 * time.Hour

	// MemberBanMaxDuration bounds a ban that ends: a year (a ban without an end lasts until it's lifted).
	MemberBanMaxDuration = 365 * 24 * time.Hour
)

// MemberBan keeps someone out of a community (Antimatter), as Discord's bans: they're removed from it and all its
// channels, and can't come back by any way in (open join, invite links, invitations, group sync, being added) until
// the ban ends or is lifted. Who banned them, why, when, and until when (0: until it's lifted).
type MemberBan struct {
	TeamId    string `json:"team_id"`
	UserId    string `json:"user_id"`
	Reason    string `json:"reason"`
	CreatedBy string `json:"created_by"`
	CreateAt  int64  `json:"create_at"`
	ExpireAt  int64  `json:"expire_at"`
}

// IsActive tells whether the ban still holds at a time (in milliseconds).
func (b *MemberBan) IsActive(now int64) bool {
	return b != nil && (b.ExpireAt == 0 || b.ExpireAt > now)
}

func (b *MemberBan) IsValid(now int64) *AppError {
	if !IsValidId(b.TeamId) || !IsValidId(b.UserId) {
		return NewAppError("MemberBan.IsValid", "model.member_ban.ids.app_error", nil, "", http.StatusBadRequest)
	}
	if b.ExpireAt != 0 && (b.ExpireAt <= now || b.ExpireAt > now+MemberBanMaxDuration.Milliseconds()) {
		return NewAppError("MemberBan.IsValid", "model.member_ban.expire_at.app_error", map[string]any{"MaxDays": int(MemberBanMaxDuration.Hours() / 24)}, "", http.StatusBadRequest)
	}
	if utf8.RuneCountInString(b.Reason) > MemberBanReasonMaxRunes {
		return NewAppError("MemberBan.IsValid", "model.member_ban.reason.app_error", map[string]any{"Max": MemberBanReasonMaxRunes}, "", http.StatusBadRequest)
	}
	return nil
}

// MemberBanRequest bans someone: why, until when (0: until it's lifted), and how far back to delete their messages
// in the community (0: none).
type MemberBanRequest struct {
	Reason                string `json:"reason"`
	ExpireAt              int64  `json:"expire_at"`
	DeleteMessagesSeconds int64  `json:"delete_messages_seconds"`
}

func (r *MemberBanRequest) IsValid() *AppError {
	if r.DeleteMessagesSeconds < 0 || r.DeleteMessagesSeconds > int64(MemberBanPurgeMaxDuration.Seconds()) {
		return NewAppError("MemberBanRequest.IsValid", "model.member_ban.delete_messages.app_error", map[string]any{"MaxDays": int(MemberBanPurgeMaxDuration.Hours() / 24)}, "", http.StatusBadRequest)
	}
	return nil
}

// MemberBanResult is a ban, with how many of the person's messages it deleted at once, or the job that deletes them
// when there are many.
type MemberBanResult struct {
	Ban          *MemberBan `json:"ban"`
	DeletedPosts int        `json:"deleted_posts"`
	PurgeJobId   string     `json:"purge_job_id,omitempty"`
}
