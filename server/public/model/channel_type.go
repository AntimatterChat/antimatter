// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"net/http"
	"slices"
	"strings"
)

// Antimatter channel types: a team channel is text, voice or forum (its type, fixed when it's made), and works in one
// of its type's modes, with settings on top: who starts conversations (opens topics, in a forum) and a forum's default
// view. Presets are named bundles of these that people pick; they aren't stored, the matching one is derived from the
// settings. Privacy stays the channel's Type (public or private), for every type. Direct and group messages have none
// of this.
//
// The settings are stored as property values (ChannelTypePropertyGroupName), one per setting that isn't its default,
// and filled into every team channel's props when it's read (ChannelPropType and the others).

// ChannelTypePropertyGroupName is the property group of the channels' types and settings; only the server writes it.
const ChannelTypePropertyGroupName = "antimatter_channel_type"

// The property fields of the group, each a JSON string.
const (
	ChannelTypeFieldType      = "type"
	ChannelTypeFieldMode      = "mode"
	ChannelTypeFieldRootPosts = "root_posts"
	ChannelTypeFieldView      = "view"
)

// ChannelTypeFields are the group's fields.
func ChannelTypeFields() []string {
	return []string{ChannelTypeFieldType, ChannelTypeFieldMode, ChannelTypeFieldRootPosts, ChannelTypeFieldView}
}

// The channel props a team channel carries when it's read (always, text channels included: a channel without
// ChannelPropType wasn't filled in), and that its creation may carry. ChannelPropPreset is derived: the matching
// preset, "" for custom settings.
const (
	ChannelPropType      = "am_type"
	ChannelPropMode      = "am_mode"
	ChannelPropRootPosts = "am_root_posts"
	ChannelPropView      = "am_forum_view"
	ChannelPropPreset    = "am_preset"

	channelPropPrefix = "am_"
)

// The types.
const (
	AMChannelTypeText  = "text"
	AMChannelTypeVoice = "voice"
	AMChannelTypeForum = "forum"
)

// The modes: what a preset brings that a setting can't (votes, an accepted answer, a status; speakers and audience).
const (
	AMChannelModeChat       = "chat"
	AMChannelModeTalk       = "talk"
	AMChannelModeStage      = "stage"
	AMChannelModeDiscussion = "discussion"
	AMChannelModeQA         = "qa"
	AMChannelModeIdeas      = "ideas"
)

// Who starts conversations (text, voice) or opens topics (forum).
const (
	AMRootPostsMembers    = "members"
	AMRootPostsModerators = "moderators"
)

// A forum's default view.
const (
	AMForumViewTopics     = "topics"
	AMForumViewImageboard = "imageboard"
)

// The presets' ids.
const (
	ChannelPresetTextChat          = "text_chat"
	ChannelPresetTextAnnouncement  = "text_announcement"
	ChannelPresetVoiceTalk         = "voice_talk"
	ChannelPresetVoiceStage        = "voice_stage"
	ChannelPresetForumAgora        = "forum_agora"
	ChannelPresetForumQA           = "forum_qa"
	ChannelPresetForumIdeas        = "forum_ideas"
	ChannelPresetForumAnnouncement = "forum_announcement"
	ChannelPresetForumImageboard   = "forum_imageboard"
)

// channelTypeModes gives each type's modes, its default first.
var channelTypeModes = map[string][]string{
	AMChannelTypeText:  {AMChannelModeChat},
	AMChannelTypeVoice: {AMChannelModeTalk, AMChannelModeStage},
	AMChannelTypeForum: {AMChannelModeDiscussion, AMChannelModeQA, AMChannelModeIdeas},
}

// ChannelTypeInfo is a team channel's type and settings, normalised (the defaults filled in). View is set for forums
// only.
type ChannelTypeInfo struct {
	Type      string `json:"type"`
	Mode      string `json:"mode"`
	RootPosts string `json:"root_posts"`
	View      string `json:"view,omitempty"`
}

// ChannelPreset is a named bundle of a type, a mode and settings, as the picker offers it.
type ChannelPreset struct {
	Id string `json:"id"`
	ChannelTypeInfo
}

// ChannelTypeRequest is a channel type's settings as a creation's props or a change (PUT /channels/{id}/type) give
// them: a preset first, then the fields given on top of it.
type ChannelTypeRequest struct {
	Preset    string `json:"preset,omitempty"`
	Type      string `json:"type,omitempty"`
	Mode      string `json:"mode,omitempty"`
	RootPosts string `json:"root_posts,omitempty"`
	View      string `json:"view,omitempty"`
}

// DefaultChannelTypeInfo is what a channel without stored settings is: a text channel to chat in, where members
// start conversations. Every channel made before channel types is one.
func DefaultChannelTypeInfo() ChannelTypeInfo {
	return ChannelTypeInfo{Type: AMChannelTypeText, Mode: AMChannelModeChat, RootPosts: AMRootPostsMembers}
}

// ChannelPresets gives the presets in the picker's order: by type, then by expected use.
func ChannelPresets() []ChannelPreset {
	preset := func(id, channelType, mode, rootPosts, view string) ChannelPreset {
		return ChannelPreset{Id: id, ChannelTypeInfo: ChannelTypeInfo{Type: channelType, Mode: mode, RootPosts: rootPosts, View: view}}
	}
	return []ChannelPreset{
		preset(ChannelPresetTextChat, AMChannelTypeText, AMChannelModeChat, AMRootPostsMembers, ""),
		preset(ChannelPresetTextAnnouncement, AMChannelTypeText, AMChannelModeChat, AMRootPostsModerators, ""),
		preset(ChannelPresetVoiceTalk, AMChannelTypeVoice, AMChannelModeTalk, AMRootPostsMembers, ""),
		preset(ChannelPresetVoiceStage, AMChannelTypeVoice, AMChannelModeStage, AMRootPostsMembers, ""),
		preset(ChannelPresetForumAgora, AMChannelTypeForum, AMChannelModeDiscussion, AMRootPostsMembers, AMForumViewTopics),
		preset(ChannelPresetForumQA, AMChannelTypeForum, AMChannelModeQA, AMRootPostsMembers, AMForumViewTopics),
		preset(ChannelPresetForumIdeas, AMChannelTypeForum, AMChannelModeIdeas, AMRootPostsMembers, AMForumViewTopics),
		preset(ChannelPresetForumAnnouncement, AMChannelTypeForum, AMChannelModeDiscussion, AMRootPostsModerators, AMForumViewTopics),
		preset(ChannelPresetForumImageboard, AMChannelTypeForum, AMChannelModeDiscussion, AMRootPostsMembers, AMForumViewImageboard),
	}
}

// ChannelPresetByID finds a preset.
func ChannelPresetByID(id string) (ChannelPreset, bool) {
	for _, p := range ChannelPresets() {
		if p.Id == id {
			return p, true
		}
	}
	return ChannelPreset{}, false
}

func channelTypeInvalid(details string) *AppError {
	return NewAppError("ChannelTypeInfo.IsValid", "api.channel.type.invalid.app_error", nil, details, http.StatusBadRequest)
}

// Normalised fills in the defaults of what's missing: text, the type's first mode, members, and the topics view for a
// forum. It doesn't drop what doesn't fit (IsValid refuses it).
func (i ChannelTypeInfo) Normalised() ChannelTypeInfo {
	if i.Type == "" {
		i.Type = AMChannelTypeText
	}
	if i.Mode == "" {
		if modes, ok := channelTypeModes[i.Type]; ok {
			i.Mode = modes[0]
		}
	}
	if i.RootPosts == "" {
		i.RootPosts = AMRootPostsMembers
	}
	if i.Type == AMChannelTypeForum && i.View == "" {
		i.View = AMForumViewTopics
	}
	return i
}

// IsValid checks a normalised info: a known type, a mode of that type, a known root posts setting, and a known view
// on a forum only.
func (i ChannelTypeInfo) IsValid() *AppError {
	modes, ok := channelTypeModes[i.Type]
	if !ok {
		return channelTypeInvalid("type=" + i.Type)
	}
	if !slices.Contains(modes, i.Mode) {
		return channelTypeInvalid("mode=" + i.Mode)
	}
	if i.RootPosts != AMRootPostsMembers && i.RootPosts != AMRootPostsModerators {
		return channelTypeInvalid("root_posts=" + i.RootPosts)
	}
	if i.Type == AMChannelTypeForum {
		if i.View != AMForumViewTopics && i.View != AMForumViewImageboard {
			return channelTypeInvalid("view=" + i.View)
		}
	} else if i.View != "" {
		return channelTypeInvalid("view on a channel that isn't a forum")
	}
	return nil
}

// IsDefault tells whether the info is a plain text channel's (nothing to store).
func (i ChannelTypeInfo) IsDefault() bool {
	return i.Normalised() == DefaultChannelTypeInfo()
}

// Preset gives the id of the preset whose settings these are, or "" for custom settings.
func (i ChannelTypeInfo) Preset() string {
	n := i.Normalised()
	for _, p := range ChannelPresets() {
		if p.ChannelTypeInfo == n {
			return p.Id
		}
	}
	return ""
}

// FieldValues gives the value of each field the info sets (the view only for a forum).
func (i ChannelTypeInfo) FieldValues() map[string]string {
	values := map[string]string{
		ChannelTypeFieldType:      i.Type,
		ChannelTypeFieldMode:      i.Mode,
		ChannelTypeFieldRootPosts: i.RootPosts,
	}
	if i.Type == AMChannelTypeForum {
		values[ChannelTypeFieldView] = i.View
	}
	return values
}

// ChannelTypeInfoFromFieldValues builds the info stored as field values; missing fields are their defaults.
func ChannelTypeInfoFromFieldValues(values map[string]string) ChannelTypeInfo {
	info := ChannelTypeInfo{
		Type:      values[ChannelTypeFieldType],
		Mode:      values[ChannelTypeFieldMode],
		RootPosts: values[ChannelTypeFieldRootPosts],
	}
	info = info.Normalised()
	if info.Type == AMChannelTypeForum {
		info.View = values[ChannelTypeFieldView]
		info = info.Normalised()
	}
	return info
}

// IsEmpty tells whether the request asks for nothing.
func (r ChannelTypeRequest) IsEmpty() bool {
	return r == ChannelTypeRequest{}
}

// Apply gives the settings the request leads to from base: its preset's bundle first, its type (whose settings that
// don't fit fall back to the type's defaults), then its fields on top. The result is normalised and checked.
func (r ChannelTypeRequest) Apply(base ChannelTypeInfo) (ChannelTypeInfo, *AppError) {
	info := base.Normalised()
	if r.Preset != "" {
		preset, ok := ChannelPresetByID(r.Preset)
		if !ok {
			return ChannelTypeInfo{}, channelTypeInvalid("preset=" + r.Preset)
		}
		if r.Type != "" && r.Type != preset.Type {
			return ChannelTypeInfo{}, channelTypeInvalid("the preset is of another type")
		}
		info = preset.ChannelTypeInfo
	}
	if r.Type != "" && r.Type != info.Type {
		modes, ok := channelTypeModes[r.Type]
		if !ok {
			return ChannelTypeInfo{}, channelTypeInvalid("type=" + r.Type)
		}
		info.Type = r.Type
		if !slices.Contains(modes, info.Mode) {
			info.Mode = ""
		}
		if info.Type != AMChannelTypeForum {
			info.View = ""
		}
	}
	if r.Mode != "" {
		info.Mode = r.Mode
	}
	if r.RootPosts != "" {
		info.RootPosts = r.RootPosts
	}
	if r.View != "" {
		info.View = r.View
	}
	info = info.Normalised()
	if appErr := info.IsValid(); appErr != nil {
		return ChannelTypeInfo{}, appErr
	}
	return info, nil
}

// ChannelTypeRequestFromProps reads a channel's creation props: am_preset, then am_type, am_mode, am_root_posts and
// am_forum_view on top. Values that aren't strings are refused.
func ChannelTypeRequestFromProps(props map[string]any) (ChannelTypeRequest, *AppError) {
	var r ChannelTypeRequest
	for key, dst := range map[string]*string{
		ChannelPropPreset:    &r.Preset,
		ChannelPropType:      &r.Type,
		ChannelPropMode:      &r.Mode,
		ChannelPropRootPosts: &r.RootPosts,
		ChannelPropView:      &r.View,
	} {
		value, ok := props[key]
		if !ok || value == nil {
			continue
		}
		s, ok := value.(string)
		if !ok {
			return ChannelTypeRequest{}, channelTypeInvalid(key + " isn't a string")
		}
		*dst = strings.TrimSpace(s)
	}
	return r, nil
}

// HasChannelTypeProps tells whether props carry any am_ prop.
func HasChannelTypeProps(props map[string]any) bool {
	for key := range props {
		if strings.HasPrefix(key, channelPropPrefix) {
			return true
		}
	}
	return false
}

// RemoveChannelTypeProps drops every am_ prop (what a creation carried, or a filling in).
func RemoveChannelTypeProps(props map[string]any) {
	for key := range props {
		if strings.HasPrefix(key, channelPropPrefix) {
			delete(props, key)
		}
	}
}

// AddChannelTypeProps fills a team channel's props with its type and settings, and the preset they match.
func (o *Channel) AddChannelTypeProps(info ChannelTypeInfo) {
	o.AddProp(ChannelPropType, info.Type)
	o.AddProp(ChannelPropMode, info.Mode)
	o.AddProp(ChannelPropRootPosts, info.RootPosts)
	o.AddProp(ChannelPropPreset, info.Preset())
	if info.Type == AMChannelTypeForum {
		o.AddProp(ChannelPropView, info.View)
	} else {
		delete(o.Props, ChannelPropView)
	}
}
