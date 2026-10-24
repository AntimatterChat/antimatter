// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelTypeInfoDefaults(t *testing.T) {
	assert.Equal(t, ChannelTypeInfo{Type: AMChannelTypeText, Mode: AMChannelModeChat, RootPosts: AMRootPostsMembers}, DefaultChannelTypeInfo())
	assert.True(t, DefaultChannelTypeInfo().IsDefault())
	assert.True(t, ChannelTypeInfo{}.IsDefault(), "nothing stored is a text channel")
	assert.Equal(t, ChannelPresetTextChat, ChannelTypeInfo{}.Preset())

	assert.Equal(t, ChannelTypeInfo{Type: AMChannelTypeVoice, Mode: AMChannelModeTalk, RootPosts: AMRootPostsMembers}, ChannelTypeInfo{Type: AMChannelTypeVoice}.Normalised())
	assert.Equal(t, ChannelTypeInfo{Type: AMChannelTypeForum, Mode: AMChannelModeDiscussion, RootPosts: AMRootPostsMembers, View: AMForumViewTopics}, ChannelTypeInfo{Type: AMChannelTypeForum}.Normalised())
	assert.False(t, ChannelTypeInfo{Type: AMChannelTypeVoice}.IsDefault())
	assert.False(t, ChannelTypeInfo{RootPosts: AMRootPostsModerators}.IsDefault())
}

func TestChannelTypeInfoIsValid(t *testing.T) {
	for _, p := range ChannelPresets() {
		assert.Nil(t, p.ChannelTypeInfo.IsValid(), p.Id)
	}

	for name, info := range map[string]ChannelTypeInfo{
		"an unknown type":             {Type: "wiki", Mode: AMChannelModeChat, RootPosts: AMRootPostsMembers},
		"a mode of another type":      {Type: AMChannelTypeText, Mode: AMChannelModeQA, RootPosts: AMRootPostsMembers},
		"a voice channel's chat mode": {Type: AMChannelTypeVoice, Mode: AMChannelModeChat, RootPosts: AMRootPostsMembers},
		"an unknown root posts":       {Type: AMChannelTypeText, Mode: AMChannelModeChat, RootPosts: "admins"},
		"a view on a text channel":    {Type: AMChannelTypeText, Mode: AMChannelModeChat, RootPosts: AMRootPostsMembers, View: AMForumViewTopics},
		"a view on a voice channel":   {Type: AMChannelTypeVoice, Mode: AMChannelModeStage, RootPosts: AMRootPostsMembers, View: AMForumViewImageboard},
		"an unknown forum view":       {Type: AMChannelTypeForum, Mode: AMChannelModeQA, RootPosts: AMRootPostsMembers, View: "kanban"},
	} {
		appErr := info.IsValid()
		require.NotNil(t, appErr, name)
		assert.Equal(t, "api.channel.type.invalid.app_error", appErr.Id, name)
	}
}

func TestChannelPresets(t *testing.T) {
	ids := []string{}
	for _, p := range ChannelPresets() {
		ids = append(ids, p.Id)
		assert.Equal(t, p.Id, p.ChannelTypeInfo.Preset(), "each preset's settings derive back to it")
		found, ok := ChannelPresetByID(p.Id)
		require.True(t, ok)
		assert.Equal(t, p, found)
	}
	assert.Equal(t, []string{
		ChannelPresetTextChat, ChannelPresetTextAnnouncement,
		ChannelPresetVoiceTalk, ChannelPresetVoiceStage,
		ChannelPresetForumAgora, ChannelPresetForumQA, ChannelPresetForumIdeas, ChannelPresetForumAnnouncement, ChannelPresetForumImageboard,
	}, ids, "the picker's order")

	_, ok := ChannelPresetByID("forum_wiki")
	assert.False(t, ok)

	announcement, _ := ChannelPresetByID(ChannelPresetTextAnnouncement)
	assert.Equal(t, AMRootPostsModerators, announcement.RootPosts)
	forumAnnouncement, _ := ChannelPresetByID(ChannelPresetForumAnnouncement)
	assert.Equal(t, ChannelTypeInfo{Type: AMChannelTypeForum, Mode: AMChannelModeDiscussion, RootPosts: AMRootPostsModerators, View: AMForumViewTopics}, forumAnnouncement.ChannelTypeInfo)
	imageboard, _ := ChannelPresetByID(ChannelPresetForumImageboard)
	assert.Equal(t, AMForumViewImageboard, imageboard.View)

	custom := ChannelTypeInfo{Type: AMChannelTypeForum, Mode: AMChannelModeQA, RootPosts: AMRootPostsModerators, View: AMForumViewImageboard}
	assert.Equal(t, "", custom.Preset(), "settings no preset has are custom")
}

func TestChannelTypeRequestApply(t *testing.T) {
	text := DefaultChannelTypeInfo()

	t.Run("a preset gives its bundle", func(t *testing.T) {
		info, appErr := ChannelTypeRequest{Preset: ChannelPresetForumQA}.Apply(text)
		require.Nil(t, appErr)
		assert.Equal(t, ChannelTypeInfo{Type: AMChannelTypeForum, Mode: AMChannelModeQA, RootPosts: AMRootPostsMembers, View: AMForumViewTopics}, info)
	})

	t.Run("fields go on top of the preset", func(t *testing.T) {
		info, appErr := ChannelTypeRequest{Preset: ChannelPresetForumQA, RootPosts: AMRootPostsModerators, View: AMForumViewImageboard}.Apply(text)
		require.Nil(t, appErr)
		assert.Equal(t, ChannelTypeInfo{Type: AMChannelTypeForum, Mode: AMChannelModeQA, RootPosts: AMRootPostsModerators, View: AMForumViewImageboard}, info)
		assert.Equal(t, "", info.Preset())
	})

	t.Run("a type alone takes its defaults", func(t *testing.T) {
		info, appErr := ChannelTypeRequest{Type: AMChannelTypeForum}.Apply(text)
		require.Nil(t, appErr)
		assert.Equal(t, ChannelPresetForumAgora, info.Preset())
		info, appErr = ChannelTypeRequest{Type: AMChannelTypeVoice}.Apply(ChannelTypeInfo{Type: AMChannelTypeForum, Mode: AMChannelModeQA, View: AMForumViewImageboard}.Normalised())
		require.Nil(t, appErr)
		assert.Equal(t, ChannelPresetVoiceTalk, info.Preset(), "the forum's mode and view don't come along")
	})

	t.Run("fields change the base", func(t *testing.T) {
		stage := ChannelTypeInfo{Type: AMChannelTypeVoice, Mode: AMChannelModeStage}.Normalised()
		info, appErr := ChannelTypeRequest{Mode: AMChannelModeTalk}.Apply(stage)
		require.Nil(t, appErr)
		assert.Equal(t, ChannelPresetVoiceTalk, info.Preset())
		info, appErr = ChannelTypeRequest{RootPosts: AMRootPostsModerators}.Apply(text)
		require.Nil(t, appErr)
		assert.Equal(t, ChannelPresetTextAnnouncement, info.Preset())
	})

	for name, req := range map[string]ChannelTypeRequest{
		"an unknown preset":           {Preset: "text_wiki"},
		"a preset of another type":    {Preset: ChannelPresetVoiceStage, Type: AMChannelTypeForum},
		"an unknown type":             {Type: "wiki"},
		"a mode not of the type":      {Mode: AMChannelModeStage},
		"a view on a text channel":    {View: AMForumViewTopics},
		"an unknown root posts value": {RootPosts: "everyone"},
	} {
		_, appErr := req.Apply(text)
		require.NotNil(t, appErr, name)
		assert.Equal(t, "api.channel.type.invalid.app_error", appErr.Id, name)
	}
}

func TestChannelTypeProps(t *testing.T) {
	t.Run("a creation's props", func(t *testing.T) {
		req, appErr := ChannelTypeRequestFromProps(map[string]any{ChannelPropPreset: " forum_ideas ", ChannelPropRootPosts: AMRootPostsModerators, "channel_mentions": "kept"})
		require.Nil(t, appErr)
		assert.Equal(t, ChannelTypeRequest{Preset: ChannelPresetForumIdeas, RootPosts: AMRootPostsModerators}, req)

		_, appErr = ChannelTypeRequestFromProps(map[string]any{ChannelPropType: 3})
		require.NotNil(t, appErr, "only strings")

		req, appErr = ChannelTypeRequestFromProps(nil)
		require.Nil(t, appErr)
		assert.True(t, req.IsEmpty())
	})

	t.Run("am_ props are recognised and dropped", func(t *testing.T) {
		props := map[string]any{ChannelPropPreset: ChannelPresetVoiceTalk, "am_forged": "x", "channel_mentions": "kept"}
		assert.True(t, HasChannelTypeProps(props))
		RemoveChannelTypeProps(props)
		assert.Equal(t, map[string]any{"channel_mentions": "kept"}, props)
		assert.False(t, HasChannelTypeProps(props))
	})

	t.Run("filling in a channel", func(t *testing.T) {
		channel := &Channel{}
		channel.AddChannelTypeProps(ChannelTypeInfo{Type: AMChannelTypeForum, Mode: AMChannelModeQA, RootPosts: AMRootPostsMembers, View: AMForumViewTopics})
		assert.Equal(t, map[string]any{
			ChannelPropType: AMChannelTypeForum, ChannelPropMode: AMChannelModeQA, ChannelPropRootPosts: AMRootPostsMembers,
			ChannelPropPreset: ChannelPresetForumQA, ChannelPropView: AMForumViewTopics,
		}, channel.Props)

		channel.AddChannelTypeProps(DefaultChannelTypeInfo())
		assert.Equal(t, map[string]any{
			ChannelPropType: AMChannelTypeText, ChannelPropMode: AMChannelModeChat, ChannelPropRootPosts: AMRootPostsMembers,
			ChannelPropPreset: ChannelPresetTextChat,
		}, channel.Props, "text channels carry their type too, and no view")
	})

	t.Run("stored field values", func(t *testing.T) {
		assert.Equal(t, DefaultChannelTypeInfo(), ChannelTypeInfoFromFieldValues(nil))
		assert.Equal(t, ChannelPresetTextAnnouncement, ChannelTypeInfoFromFieldValues(map[string]string{ChannelTypeFieldRootPosts: AMRootPostsModerators}).Preset())
		assert.Equal(t, ChannelPresetVoiceTalk, ChannelTypeInfoFromFieldValues(map[string]string{ChannelTypeFieldType: AMChannelTypeVoice}).Preset())
		assert.Equal(t, ChannelPresetForumImageboard, ChannelTypeInfoFromFieldValues(map[string]string{ChannelTypeFieldType: AMChannelTypeForum, ChannelTypeFieldView: AMForumViewImageboard}).Preset())
		assert.Equal(t, "", ChannelTypeInfoFromFieldValues(map[string]string{ChannelTypeFieldView: AMForumViewImageboard}).View, "a view on a text channel is ignored")

		forum := ChannelTypeInfo{Type: AMChannelTypeForum, Mode: AMChannelModeIdeas, RootPosts: AMRootPostsMembers, View: AMForumViewTopics}
		assert.Equal(t, forum, ChannelTypeInfoFromFieldValues(forum.FieldValues()))
		assert.NotContains(t, DefaultChannelTypeInfo().FieldValues(), ChannelTypeFieldView)
	})
}

func TestCreateRootPostIsOverrideOnly(t *testing.T) {
	assert.Contains(t, ChannelOverridePermissions(), PermissionCreateRootPost.Id, "overrides can allow or deny it")
	for _, p := range AllPermissions {
		assert.NotEqual(t, PermissionCreateRootPost.Id, p.Id, "no role holds it, so no role is given it")
	}
	for name, role := range MakeDefaultRoles() {
		assert.NotContains(t, role.Permissions, PermissionCreateRootPost.Id, name)
	}
}
