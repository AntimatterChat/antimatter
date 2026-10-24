// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"net/http"
	"time"
)

const (
	// TeamInviteLinkMaxUses bounds how many times a link may be used (0: as often as one likes).
	TeamInviteLinkMaxUses = 10000

	// TeamInviteLinkMaxDuration bounds a link that ends: a year (0: it doesn't end).
	TeamInviteLinkMaxDuration = 365 * 24 * time.Hour

	// TeamInviteLinksMaxPerTeam bounds a community's links, used up and ended ones included: revoke some to make more.
	TeamInviteLinksMaxPerTeam = 200
)

// TeamInviteLink is one of a community's invite links (Antimatter), besides its permanent invite code: made by
// someone allowed to invite, it may end at a time and allow a number of uses, and is revoked by deleting it. Its id
// is the code in the link (/signup_user_complete/?id=<id>), as the permanent one's.
type TeamInviteLink struct {
	Id        string `json:"id"`
	TeamId    string `json:"team_id"`
	CreatorId string `json:"creator_id"`
	CreateAt  int64  `json:"create_at"`
	ExpireAt  int64  `json:"expire_at"`
	MaxUses   int    `json:"max_uses"`
	Uses      int    `json:"uses"`
}

// IsExpired tells whether the link has ended at a time (in milliseconds).
func (l *TeamInviteLink) IsExpired(now int64) bool {
	return l.ExpireAt != 0 && l.ExpireAt <= now
}

// IsUsedUp tells whether the link has been used as many times as it allows.
func (l *TeamInviteLink) IsUsedUp() bool {
	return l.MaxUses > 0 && l.Uses >= l.MaxUses
}

func (l *TeamInviteLink) IsValid(now int64) *AppError {
	if !IsValidId(l.Id) || !IsValidId(l.TeamId) || !IsValidId(l.CreatorId) {
		return NewAppError("TeamInviteLink.IsValid", "model.team_invite_link.ids.app_error", nil, "", http.StatusBadRequest)
	}
	if l.ExpireAt != 0 && (l.ExpireAt <= now || l.ExpireAt > now+TeamInviteLinkMaxDuration.Milliseconds()) {
		return NewAppError("TeamInviteLink.IsValid", "model.team_invite_link.expire_at.app_error", map[string]any{"MaxDays": int(TeamInviteLinkMaxDuration.Hours() / 24)}, "", http.StatusBadRequest)
	}
	if l.MaxUses < 0 || l.MaxUses > TeamInviteLinkMaxUses {
		return NewAppError("TeamInviteLink.IsValid", "model.team_invite_link.max_uses.app_error", map[string]any{"Max": TeamInviteLinkMaxUses}, "", http.StatusBadRequest)
	}
	return nil
}

// TeamInviteLinkRequest makes an invite link: until when (0: it doesn't end) and how many uses (0: no limit).
type TeamInviteLinkRequest struct {
	ExpireAt int64 `json:"expire_at"`
	MaxUses  int   `json:"max_uses"`
}
