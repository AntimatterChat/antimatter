// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"net/http"
	"strings"
	"unicode/utf8"
)

// CommunityRulesMaxRunes bounds a community's rules, as Markdown.
const CommunityRulesMaxRunes = 16000

// CommunityRules are the rules a community's members agree to before they post in its channels (Antimatter), as
// Discord's rules screening. Version goes up each time an admin saves them asking everyone to agree again; someone
// may post while the rules are on only once they agreed to the current version. Direct and group messages, bots,
// webhooks and the community's admins aren't held by them.
type CommunityRules struct {
	TeamId    string `json:"team_id"`
	Enabled   bool   `json:"enabled"`
	Text      string `json:"text"`
	Version   int64  `json:"version"`
	UpdatedBy string `json:"updated_by"`
	UpdateAt  int64  `json:"update_at"`

	// For the one asking (filled in by the API, never stored): the latest version they agreed to (0 for none), and
	// whether they must agree before they may post (the rules are on, they haven't agreed to this version, and
	// they aren't exempt).
	AcceptedVersion int64 `json:"accepted_version" db:"-"`
	MustAccept      bool  `json:"must_accept" db:"-"`
}

// IsActive tells whether the rules hold someone back from posting until they agree.
func (r *CommunityRules) IsActive() bool {
	return r != nil && r.Enabled && r.Version > 0
}

// CommunityRulesRequest saves a community's rules: on or off, their text, and whether everyone must agree again.
type CommunityRulesRequest struct {
	Enabled  bool   `json:"enabled"`
	Text     string `json:"text"`
	AskAgain bool   `json:"ask_again"`
}

func (r *CommunityRulesRequest) PreSave() {
	r.Text = strings.TrimSpace(r.Text)
}

func (r *CommunityRulesRequest) IsValid() *AppError {
	if utf8.RuneCountInString(r.Text) > CommunityRulesMaxRunes {
		return NewAppError("CommunityRulesRequest.IsValid", "model.community_rules.text.app_error", map[string]any{"Max": CommunityRulesMaxRunes}, "", http.StatusBadRequest)
	}
	if r.Enabled && r.Text == "" {
		return NewAppError("CommunityRulesRequest.IsValid", "model.community_rules.empty.app_error", nil, "", http.StatusBadRequest)
	}
	return nil
}

// CommunityRulesAcceptRequest agrees to a version of a community's rules: the one the member read. Agreeing to a
// version that's no longer the current one is refused, so nobody agrees to rules they haven't seen.
type CommunityRulesAcceptRequest struct {
	Version int64 `json:"version"`
}

// CommunityRulesAcceptance is someone's agreement to a version of a community's rules, kept for each version they
// agreed to.
type CommunityRulesAcceptance struct {
	TeamId     string `json:"team_id"`
	UserId     string `json:"user_id"`
	Version    int64  `json:"version"`
	AcceptedAt int64  `json:"accepted_at"`
}

// CommunityRulesStats tells a community's admins how many of its members agreed to the current version of its rules.
// Members are the active people of the community (bots and deactivated accounts left out).
type CommunityRulesStats struct {
	TeamId        string `json:"team_id"`
	Version       int64  `json:"version"`
	AcceptedCount int64  `json:"accepted_count"`
	MemberCount   int64  `json:"member_count"`
}
