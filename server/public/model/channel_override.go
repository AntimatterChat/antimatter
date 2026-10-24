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
// see a channel stays its membership (see role-gated channels). create_root_post is only ever given by overrides.
func ChannelOverridePermissions() []string {
	return []string{
		PermissionCreatePost.Id,
		PermissionCreateRootPost.Id,
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

// CategoryOverrideCategoryMaxLength bounds a category override's category: a server category's name, as compared
// (CategoryOverrideKey).
const CategoryOverrideCategoryMaxLength = 255

// CategoryOverride is an override set on a server category (Antimatter), as Discord's category permissions: the
// channels filed in it inherit it, unless they have their own override for the same role or person.
type CategoryOverride struct {
	TeamId     string   `json:"team_id"`
	Category   string   `json:"category"`
	TargetType string   `json:"target_type"`
	TargetId   string   `json:"target_id"`
	Allow      []string `json:"allow"`
	Deny       []string `json:"deny"`
	UpdateAt   int64    `json:"update_at"`
}

// CategoryOverrideKey is how a category override names its category: the server category's name, trimmed and in
// lower case, as names compare.
func CategoryOverrideKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// AsChannelOverride is the override a channel of the category inherits.
func (o *CategoryOverride) AsChannelOverride(channelID string) *ChannelOverride {
	return &ChannelOverride{ChannelId: channelID, TargetType: o.TargetType, TargetId: o.TargetId, Allow: o.Allow, Deny: o.Deny, UpdateAt: o.UpdateAt}
}

func (o *CategoryOverride) IsValid() *AppError {
	if !IsValidId(o.TeamId) || o.Category == "" || len(o.Category) > CategoryOverrideCategoryMaxLength {
		return NewAppError("CategoryOverride.IsValid", "model.channel_override.ids.app_error", nil, "", http.StatusBadRequest)
	}
	// The same rules as a channel's.
	return o.AsChannelOverride(o.TeamId).IsValid()
}

func (o *CategoryOverride) IsEmpty() bool {
	return len(o.Allow) == 0 && len(o.Deny) == 0
}

// EffectiveChannelOverrides are the overrides that apply in a channel: those of its server category, then the
// channel's own on top, permission by permission: what the channel allows or denies for a role or person wins over
// what its category says of the same permission; the rest of the category's stands.
func EffectiveChannelOverrides(channelID string, category []*CategoryOverride, own []*ChannelOverride) []*ChannelOverride {
	effective := make([]*ChannelOverride, 0, len(category)+len(own))
	for _, c := range category {
		inherited := c.AsChannelOverride(channelID)
		inherited.Allow = slices.Clone(c.Allow)
		inherited.Deny = slices.Clone(c.Deny)
		effective = append(effective, inherited)
	}
	for _, o := range own {
		at := slices.IndexFunc(effective, func(e *ChannelOverride) bool { return e.TargetType == o.TargetType && e.TargetId == o.TargetId })
		if at < 0 {
			effective = append(effective, o)
			continue
		}
		merged := effective[at]
		merged.Allow = slices.DeleteFunc(merged.Allow, func(p string) bool { return slices.Contains(o.Deny, p) || slices.Contains(o.Allow, p) })
		merged.Deny = slices.DeleteFunc(merged.Deny, func(p string) bool { return slices.Contains(o.Allow, p) || slices.Contains(o.Deny, p) })
		merged.Allow = append(merged.Allow, o.Allow...)
		merged.Deny = append(merged.Deny, o.Deny...)
		merged.UpdateAt = max(merged.UpdateAt, o.UpdateAt)
	}
	return effective
}
