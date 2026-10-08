// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"net/http"
	"slices"
	"strings"
)

const (
	ChannelOverrideTargetRole = "role"
	ChannelOverrideTargetUser = "user"
)

// ChannelOverride changes what a role (a group of the channel's team) or one person may do in a channel, as Discord's
// permission overwrites: permissions allowed there on top of what they have, and permissions denied there.
type ChannelOverride struct {
	ChannelId  string   `json:"channel_id"`
	TargetType string   `json:"target_type"`
	TargetId   string   `json:"target_id"`
	Allow      []string `json:"allow"`
	Deny       []string `json:"deny"`
	UpdateAt   int64    `json:"update_at"`
}

// ChannelOverridePermissions are the permissions an override may allow or deny: what one does in a channel. Who can
// see a channel stays its membership (see role-gated channels).
func ChannelOverridePermissions() []string {
	return []string{
		PermissionCreatePost.Id,
		PermissionAddReaction.Id,
		PermissionRemoveReaction.Id,
		PermissionUploadFile.Id,
		PermissionUseChannelMentions.Id,
		PermissionUseGroupMentions.Id,
		PermissionDeleteOthersPosts.Id,
		PermissionEditOthersPosts.Id,
		PermissionManagePublicChannelMembers.Id,
		PermissionManagePrivateChannelMembers.Id,
		PermissionManagePublicChannelProperties.Id,
		PermissionManagePrivateChannelProperties.Id,
	}
}

func (o *ChannelOverride) IsValid() *AppError {
	if !IsValidId(o.ChannelId) || !IsValidId(o.TargetId) {
		return NewAppError("ChannelOverride.IsValid", "model.channel_override.ids.app_error", nil, "", http.StatusBadRequest)
	}
	if o.TargetType != ChannelOverrideTargetRole && o.TargetType != ChannelOverrideTargetUser {
		return NewAppError("ChannelOverride.IsValid", "model.channel_override.target_type.app_error", nil, "", http.StatusBadRequest)
	}
	allowed := ChannelOverridePermissions()
	for _, p := range append(slices.Clone(o.Allow), o.Deny...) {
		if !slices.Contains(allowed, p) {
			return NewAppError("ChannelOverride.IsValid", "model.channel_override.permission.app_error", map[string]any{"Permission": p}, "", http.StatusBadRequest)
		}
	}
	for _, p := range o.Allow {
		if slices.Contains(o.Deny, p) {
			return NewAppError("ChannelOverride.IsValid", "model.channel_override.both.app_error", map[string]any{"Permission": p}, "", http.StatusBadRequest)
		}
	}
	return nil
}

// IsEmpty tells whether the override changes nothing (and can go).
func (o *ChannelOverride) IsEmpty() bool {
	return len(o.Allow) == 0 && len(o.Deny) == 0
}

// ApplyChannelOverrides resolves a permission in a channel the way Discord does, from what someone has without the
// channel's overrides: the overrides of their roles first (a deny takes it away, then an allow gives it back), then
// their own override.
func ApplyChannelOverrides(has bool, permission string, roleIDs []string, userID string, overrides []*ChannelOverride) bool {
	denied, allowed := false, false
	for _, o := range overrides {
		if o.TargetType == ChannelOverrideTargetRole && slices.Contains(roleIDs, o.TargetId) {
			denied = denied || slices.Contains(o.Deny, permission)
			allowed = allowed || slices.Contains(o.Allow, permission)
		}
	}
	if denied {
		has = false
	}
	if allowed {
		has = true
	}
	for _, o := range overrides {
		if o.TargetType == ChannelOverrideTargetUser && o.TargetId == userID {
			if slices.Contains(o.Deny, permission) {
				has = false
			}
			if slices.Contains(o.Allow, permission) {
				has = true
			}
		}
	}
	return has
}

// TeamRoleGroupIDs gives the groups of the Antimatter roles among someone's team roles.
func TeamRoleGroupIDs(teamRoles string) []string {
	var ids []string
	for r := range strings.FieldsSeq(teamRoles) {
		if id, ok := strings.CutPrefix(r, TeamRoleName("")); ok {
			ids = append(ids, id)
		}
	}
	return ids
}
