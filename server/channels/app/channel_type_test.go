// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"maps"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/v8/channels/app/imports"
)

// Antimatter channel types: how a channel's type and settings are stored, kept in memory, filled in and changed.

// createTypedChannel makes a public channel of the basic team with a preset, through App.CreateChannel, with nobody
// in it.
func createTypedChannel(t *testing.T, th *TestHelper, preset string) *model.Channel {
	t.Helper()
	channel, appErr := th.App.CreateChannel(th.Context, &model.Channel{
		DisplayName: "Typed",
		Name:        "typed-" + model.NewId(),
		Type:        model.ChannelTypeOpen,
		TeamId:      th.BasicTeam.Id,
		CreatorId:   th.BasicUser.Id,
		Props:       map[string]any{model.ChannelPropPreset: preset},
	}, false)
	require.Nil(t, appErr)
	return channel
}

// withoutChannelTypeProps gives a channel's props but the channel type ones (nil when nothing else is left), for tests
// about the other props.
func withoutChannelTypeProps(props map[string]any) map[string]any {
	rest := maps.Clone(props)
	model.RemoveChannelTypeProps(rest)
	if len(rest) == 0 {
		return nil
	}
	return rest
}

// storedChannelTypeValues reads a channel's rows, by field name.
func storedChannelTypeValues(t *testing.T, th *TestHelper, channelID string) map[string]string {
	t.Helper()
	ch := th.App.Channels()
	values, err := th.App.Srv().propertyService.SearchPropertyValues(th.Context, ch.channelTypeGroupID, model.PropertyValueSearchOpts{
		GroupID: ch.channelTypeGroupID, TargetType: model.PropertyValueTargetTypeChannel, TargetIDs: []string{channelID}, PerPage: 20,
	})
	require.NoError(t, err)
	byName := map[string]string{}
	for _, v := range values {
		var s string
		require.NoError(t, json.Unmarshal(v.Value, &s))
		byName[ch.channelTypeFieldNames[v.FieldID]] = s
	}
	return byName
}

func TestChannelTypesSetup(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t)

	group, appErr := th.App.GetPropertyGroup(th.Context, model.ChannelTypePropertyGroupName)
	require.Nil(t, appErr)
	assert.Equal(t, model.PropertyGroupVersionV2, group.Version)
	assert.Equal(t, group.ID, th.App.Channels().channelTypeGroupID)

	fields, appErr := th.App.SearchPropertyFields(th.Context, group.ID, model.PropertyFieldSearchOpts{PerPage: 100})
	require.Nil(t, appErr)
	names := []string{}
	for _, f := range fields {
		names = append(names, f.Name)
		assert.True(t, f.Protected, "only the server writes them")
		assert.Equal(t, model.PropertyFieldObjectTypeChannel, f.ObjectType)
		assert.Equal(t, f.ID, th.App.Channels().channelTypeFieldIDs[f.Name])
	}
	assert.ElementsMatch(t, model.ChannelTypeFields(), names)

	// Again, as at every start: nothing new.
	require.NoError(t, th.Server.doSetupChannelTypeProperties())
	again, appErr := th.App.SearchPropertyFields(th.Context, group.ID, model.PropertyFieldSearchOpts{PerPage: 100})
	require.Nil(t, appErr)
	assert.Len(t, again, len(fields))
}

func TestCreateChannelWithType(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	t.Run("a preset is stored as its non-default settings and filled in", func(t *testing.T) {
		channel := createTypedChannel(t, th, model.ChannelPresetForumQA)
		assert.Equal(t, model.AMChannelTypeForum, channel.Props[model.ChannelPropType])
		assert.Equal(t, model.AMChannelModeQA, channel.Props[model.ChannelPropMode])
		assert.Equal(t, model.AMRootPostsMembers, channel.Props[model.ChannelPropRootPosts])
		assert.Equal(t, model.AMForumViewTopics, channel.Props[model.ChannelPropView])
		assert.Equal(t, model.ChannelPresetForumQA, channel.Props[model.ChannelPropPreset])

		assert.Equal(t, map[string]string{model.ChannelTypeFieldType: model.AMChannelTypeForum, model.ChannelTypeFieldMode: model.AMChannelModeQA}, storedChannelTypeValues(t, th, channel.Id))
		assert.Equal(t, model.ChannelPresetForumQA, th.App.GetChannelTypeInfo(channel.Id).Preset())

		read, appErr := th.App.GetChannel(th.Context, channel.Id)
		require.Nil(t, appErr)
		require.Nil(t, th.App.FillInChannelProps(th.Context, read))
		assert.Equal(t, model.ChannelPresetForumQA, read.Props[model.ChannelPropPreset], "every read fills it in")
	})

	t.Run("the fields go on top of the preset; other am_ props are dropped", func(t *testing.T) {
		channel, appErr := th.App.CreateChannel(th.Context, &model.Channel{
			DisplayName: "Board", Name: "board-" + model.NewId(), Type: model.ChannelTypePrivate, TeamId: th.BasicTeam.Id, CreatorId: th.BasicUser.Id,
			Props: map[string]any{model.ChannelPropPreset: model.ChannelPresetForumAgora, model.ChannelPropView: model.AMForumViewImageboard, "am_forged": "x"},
		}, false)
		require.Nil(t, appErr)
		assert.Equal(t, model.ChannelPresetForumImageboard, channel.Props[model.ChannelPropPreset])
		assert.NotContains(t, channel.Props, "am_forged")
		assert.Equal(t, model.ChannelTypePrivate, channel.Type, "privacy stays the channel's own switch")
	})

	t.Run("a plain text channel stores nothing", func(t *testing.T) {
		channel := createTypedChannel(t, th, model.ChannelPresetTextChat)
		assert.Empty(t, storedChannelTypeValues(t, th, channel.Id))
		assert.Equal(t, model.AMChannelTypeText, channel.Props[model.ChannelPropType])

		plain := th.CreateChannel(t, th.BasicTeam)
		assert.Empty(t, storedChannelTypeValues(t, th, plain.Id))
		assert.Equal(t, model.DefaultChannelTypeInfo(), th.App.GetChannelTypeInfo(plain.Id))
	})

	t.Run("an announcement channel stores who starts conversations only", func(t *testing.T) {
		channel := createTypedChannel(t, th, model.ChannelPresetTextAnnouncement)
		assert.Equal(t, map[string]string{model.ChannelTypeFieldRootPosts: model.AMRootPostsModerators}, storedChannelTypeValues(t, th, channel.Id))
	})

	t.Run("invalid settings make no channel", func(t *testing.T) {
		name := "bad-" + model.NewId()
		for _, props := range []map[string]any{
			{model.ChannelPropPreset: "forum_wiki"},
			{model.ChannelPropType: model.AMChannelTypeText, model.ChannelPropMode: model.AMChannelModeStage},
			{model.ChannelPropView: model.AMForumViewTopics},
			{model.ChannelPropType: true},
		} {
			_, appErr := th.App.CreateChannel(th.Context, &model.Channel{
				DisplayName: "Bad", Name: name, Type: model.ChannelTypeOpen, TeamId: th.BasicTeam.Id, CreatorId: th.BasicUser.Id, Props: props,
			}, false)
			require.NotNil(t, appErr, props)
			assert.Equal(t, "api.channel.type.invalid.app_error", appErr.Id)
			assert.Equal(t, http.StatusBadRequest, appErr.StatusCode)
		}
		_, appErr := th.App.GetChannelByName(th.Context, name, th.BasicTeam.Id, true)
		require.NotNil(t, appErr, "nothing was made")
	})

	t.Run("a failed write removes the channel again", func(t *testing.T) {
		ch := th.App.Channels()
		saved := ch.channelTypeFieldIDs
		broken := map[string]string{}
		for name := range saved {
			broken[name] = "not-an-id"
		}
		ch.channelTypeFieldIDs = broken
		name := "rollback-" + model.NewId()
		_, appErr := th.App.CreateChannel(th.Context, &model.Channel{
			DisplayName: "Rollback", Name: name, Type: model.ChannelTypeOpen, TeamId: th.BasicTeam.Id, CreatorId: th.BasicUser.Id,
			Props: map[string]any{model.ChannelPropPreset: model.ChannelPresetVoiceStage},
		}, true)
		ch.channelTypeFieldIDs = saved
		require.NotNil(t, appErr)
		assert.Equal(t, "app.channel.type.save.app_error", appErr.Id)
		_, appErr = th.App.GetChannelByName(th.Context, name, th.BasicTeam.Id, true)
		require.NotNil(t, appErr, "no half-made channel stays")
	})

	t.Run("permanently deleting a channel forgets its type", func(t *testing.T) {
		channel := createTypedChannel(t, th, model.ChannelPresetVoiceStage)
		require.Nil(t, th.App.PermanentDeleteChannel(th.Context, channel))
		assert.Empty(t, storedChannelTypeValues(t, th, channel.Id))
		assert.Equal(t, model.DefaultChannelTypeInfo(), th.App.GetChannelTypeInfo(channel.Id))
	})
}

func TestChannelTypeFillIn(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	forum := createTypedChannel(t, th, model.ChannelPresetForumIdeas)
	text := th.BasicChannel

	t.Run("lists and single channels", func(t *testing.T) {
		list := model.ChannelList{forum.DeepCopy(), text.DeepCopy()}
		for _, c := range list {
			c.Props = nil
		}
		require.Nil(t, th.App.FillInChannelsProps(th.Context, list))
		assert.Equal(t, model.ChannelPresetForumIdeas, list[0].Props[model.ChannelPropPreset])
		assert.Equal(t, model.AMChannelTypeText, list[1].Props[model.ChannelPropType], "text channels carry their type too")
	})

	t.Run("direct and group messages get nothing", func(t *testing.T) {
		dm, appErr := th.App.GetOrCreateDirectChannel(th.Context, th.BasicUser.Id, th.BasicUser2.Id)
		require.Nil(t, appErr)
		th.App.FillInChannelTypes(dm)
		assert.NotContains(t, dm.Props, model.ChannelPropType)
	})

	t.Run("the plugin API", func(t *testing.T) {
		api := th.SetupPluginAPI()
		got, appErr := api.GetChannel(forum.Id)
		require.Nil(t, appErr)
		assert.Equal(t, model.AMChannelModeIdeas, got.Props[model.ChannelPropMode])
		got, appErr = api.GetChannelByName(th.BasicTeam.Id, forum.Name, false)
		require.Nil(t, appErr)
		assert.Equal(t, model.AMChannelTypeForum, got.Props[model.ChannelPropType])
	})

	t.Run("channel_updated carries it", func(t *testing.T) {
		updated, appErr := th.App.UpdateChannel(th.Context, forum.DeepCopy())
		require.Nil(t, appErr)
		assert.Equal(t, model.ChannelPresetForumIdeas, updated.Props[model.ChannelPropPreset])
	})
}

func TestChannelTypeCacheAndCluster(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)
	channel := th.CreateChannel(t, th.BasicTeam)
	ch := th.App.Channels()

	// Another node makes it a stage: the rows change under this one, which only learns of it by the cluster message.
	valueJSON, err := json.Marshal(model.AMChannelTypeVoice)
	require.NoError(t, err)
	modeJSON, err := json.Marshal(model.AMChannelModeStage)
	require.NoError(t, err)
	_, err = th.App.Srv().propertyService.UpsertPropertyValues(th.Context, []*model.PropertyValue{
		{GroupID: ch.channelTypeGroupID, FieldID: ch.channelTypeFieldIDs[model.ChannelTypeFieldType], TargetID: channel.Id, TargetType: model.PropertyValueTargetTypeChannel, Value: valueJSON},
		{GroupID: ch.channelTypeGroupID, FieldID: ch.channelTypeFieldIDs[model.ChannelTypeFieldMode], TargetID: channel.Id, TargetType: model.PropertyValueTargetTypeChannel, Value: modeJSON},
	})
	require.NoError(t, err)
	assert.Equal(t, model.AMChannelTypeText, th.App.GetChannelTypeInfo(channel.Id).Type, "checks read memory, not the database")

	ch.clusterInvalidateChannelTypeHandler(&model.ClusterMessage{Event: clusterEventInvalidateChannelType, Data: []byte(channel.Id)})
	assert.Equal(t, model.ChannelPresetVoiceStage, th.App.GetChannelTypeInfo(channel.Id).Preset())

	ch.clusterInvalidateChannelTypeHandler(&model.ClusterMessage{Event: clusterEventInvalidateChannelType, Data: []byte("not an id")})

	// A restart loads every typed channel.
	ch.channelTypes.Delete(channel.Id)
	require.NoError(t, ch.loadAllChannelTypes(th.Context))
	assert.Equal(t, model.ChannelPresetVoiceStage, th.App.GetChannelTypeInfo(channel.Id).Preset())
}

func TestUpdateChannelType(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	t.Run("another preset of the type", func(t *testing.T) {
		channel := createTypedChannel(t, th, model.ChannelPresetForumAgora)
		updated, appErr := th.App.UpdateChannelType(th.Context, channel, model.ChannelTypeRequest{Preset: model.ChannelPresetForumIdeas}, th.BasicUser.Id)
		require.Nil(t, appErr)
		assert.Equal(t, model.ChannelPresetForumIdeas, updated.Props[model.ChannelPropPreset])
		assert.Equal(t, model.AMChannelModeIdeas, th.App.GetChannelTypeInfo(channel.Id).Mode)

		updated, appErr = th.App.UpdateChannelType(th.Context, channel, model.ChannelTypeRequest{RootPosts: model.AMRootPostsModerators, View: model.AMForumViewImageboard}, th.BasicUser.Id)
		require.Nil(t, appErr)
		assert.Equal(t, "", updated.Props[model.ChannelPropPreset], "custom settings")
		assert.Equal(t, model.ChannelTypeInfo{Type: model.AMChannelTypeForum, Mode: model.AMChannelModeIdeas, RootPosts: model.AMRootPostsModerators, View: model.AMForumViewImageboard}, th.App.GetChannelTypeInfo(channel.Id))
	})

	t.Run("going back to the defaults updates the rows", func(t *testing.T) {
		channel := createTypedChannel(t, th, model.ChannelPresetTextAnnouncement)
		_, appErr := th.App.UpdateChannelType(th.Context, channel, model.ChannelTypeRequest{Preset: model.ChannelPresetTextChat}, th.BasicUser.Id)
		require.Nil(t, appErr)
		assert.Equal(t, model.DefaultChannelTypeInfo(), th.App.GetChannelTypeInfo(channel.Id))
		require.NoError(t, th.App.Channels().reloadChannelType(th.Context, channel.Id))
		assert.Equal(t, model.DefaultChannelTypeInfo(), th.App.GetChannelTypeInfo(channel.Id), "what's stored says so too")
	})

	t.Run("the type is fixed", func(t *testing.T) {
		channel := createTypedChannel(t, th, model.ChannelPresetVoiceTalk)
		for _, req := range []model.ChannelTypeRequest{
			{Preset: model.ChannelPresetForumAgora},
			{Type: model.AMChannelTypeText},
			{Preset: model.ChannelPresetTextChat, Type: model.AMChannelTypeText},
		} {
			_, appErr := th.App.UpdateChannelType(th.Context, channel, req, th.BasicUser.Id)
			require.NotNil(t, appErr, req)
			assert.Equal(t, "api.channel.type.fixed.app_error", appErr.Id)
		}
		assert.Equal(t, model.ChannelPresetVoiceTalk, th.App.GetChannelTypeInfo(channel.Id).Preset())

		text := th.CreateChannel(t, th.BasicTeam)
		_, appErr := th.App.UpdateChannelType(th.Context, text, model.ChannelTypeRequest{Preset: model.ChannelPresetVoiceStage}, th.BasicUser.Id)
		require.NotNil(t, appErr, "nor does a text channel become a voice one")
		assert.Equal(t, "api.channel.type.fixed.app_error", appErr.Id)
	})

	t.Run("invalid settings and empty requests are refused", func(t *testing.T) {
		channel := createTypedChannel(t, th, model.ChannelPresetTextChat)
		for _, req := range []model.ChannelTypeRequest{{}, {View: model.AMForumViewTopics}, {Mode: model.AMChannelModeQA}, {Preset: "nope"}} {
			_, appErr := th.App.UpdateChannelType(th.Context, channel, req, th.BasicUser.Id)
			require.NotNil(t, appErr, req)
			assert.Equal(t, "api.channel.type.invalid.app_error", appErr.Id)
		}
	})

	t.Run("direct messages have no type", func(t *testing.T) {
		dm, appErr := th.App.GetOrCreateDirectChannel(th.Context, th.BasicUser.Id, th.BasicUser2.Id)
		require.Nil(t, appErr)
		_, appErr = th.App.UpdateChannelType(th.Context, dm, model.ChannelTypeRequest{Preset: model.ChannelPresetTextAnnouncement}, th.BasicUser.Id)
		require.NotNil(t, appErr)
		assert.Equal(t, "api.channel.type.direct.app_error", appErr.Id)
	})
}

func TestChannelTypesVoiceMigration(t *testing.T) {
	th := Setup(t).InitBasic(t)

	voice := th.CreateChannel(t, th.BasicTeam)
	archived := th.CreateChannel(t, th.BasicTeam)
	require.Nil(t, th.App.DeleteChannel(th.Context, archived, th.BasicUser.Id))
	gone := model.NewId()
	plain := th.CreateChannel(t, th.BasicTeam)
	for _, key := range []string{voiceChannelsKVPrefix + voice.Id, voiceChannelsKVPrefix + archived.Id, voiceChannelsKVPrefix + gone, "deaf_" + plain.Id} {
		_, err := th.Server.Store().Plugin().SaveOrUpdate(&model.PluginKeyValue{PluginId: voiceChannelsPluginID, Key: key, Value: []byte(`{}`)})
		require.NoError(t, err)
	}
	_, err := th.Server.Store().System().PermanentDeleteByName(channelTypesVoiceMigrationKey)
	require.NoError(t, err)

	require.NoError(t, th.Server.doChannelTypesVoiceMigration())
	assert.Equal(t, model.ChannelPresetVoiceTalk, th.App.GetChannelTypeInfo(voice.Id).Preset())
	assert.Equal(t, map[string]string{model.ChannelTypeFieldType: model.AMChannelTypeVoice}, storedChannelTypeValues(t, th, voice.Id), "the type only: talk and members are the defaults")
	assert.Equal(t, model.AMChannelTypeVoice, th.App.GetChannelTypeInfo(archived.Id).Type, "an archived channel still exists")
	assert.Empty(t, storedChannelTypeValues(t, th, gone), "a channel that's gone is skipped")
	assert.Equal(t, model.DefaultChannelTypeInfo(), th.App.GetChannelTypeInfo(plain.Id), "other keys of the app aren't voice channels")

	done, err := th.Server.migrationDone(channelTypesVoiceMigrationKey)
	require.NoError(t, err)
	assert.True(t, done)

	// Idempotent: run again (done), and again after forgetting it was done.
	require.NoError(t, th.Server.doChannelTypesVoiceMigration())
	_, err = th.Server.Store().System().PermanentDeleteByName(channelTypesVoiceMigrationKey)
	require.NoError(t, err)
	require.NoError(t, th.Server.doChannelTypesVoiceMigration())
	assert.Equal(t, map[string]string{model.ChannelTypeFieldType: model.AMChannelTypeVoice}, storedChannelTypeValues(t, th, voice.Id))
}

func TestChannelTypeExportImport(t *testing.T) {
	mainHelper.Parallel(t)
	th := Setup(t).InitBasic(t)

	t.Run("the export line carries a typed channel's settings", func(t *testing.T) {
		forum := &model.ChannelForExport{Channel: model.Channel{Name: "f", TeamId: th.BasicTeam.Id}, TeamName: th.BasicTeam.Name}
		line := importLineFromChannel(forum, model.ChannelTypeInfo{Type: model.AMChannelTypeForum, Mode: model.AMChannelModeQA, RootPosts: model.AMRootPostsMembers, View: model.AMForumViewTopics})
		require.NotNil(t, line.Channel.AMType)
		assert.Equal(t, model.AMChannelTypeForum, *line.Channel.AMType)
		assert.Equal(t, model.AMChannelModeQA, *line.Channel.AMMode)
		assert.Equal(t, model.AMRootPostsMembers, *line.Channel.AMRootPosts)
		assert.Equal(t, model.AMForumViewTopics, *line.Channel.AMForumView)

		line = importLineFromChannel(forum, model.DefaultChannelTypeInfo())
		assert.Nil(t, line.Channel.AMType, "a plain text channel's line is as before")
		assert.Nil(t, line.Channel.AMForumView)
	})

	t.Run("an import line makes a typed channel", func(t *testing.T) {
		name := "imported-" + model.NewId()
		data := &imports.ChannelImportData{
			Team: &th.BasicTeam.Name, Name: &name, DisplayName: model.NewPointer("Imported"), Type: model.NewPointer(model.ChannelTypeOpen),
			AMType: model.NewPointer(model.AMChannelTypeForum), AMMode: model.NewPointer(model.AMChannelModeIdeas), AMForumView: model.NewPointer(model.AMForumViewImageboard),
		}
		require.Nil(t, th.App.importChannel(th.Context, data, false))
		channel, appErr := th.App.GetChannelByName(th.Context, name, th.BasicTeam.Id, false)
		require.Nil(t, appErr)
		assert.Equal(t, model.ChannelTypeInfo{Type: model.AMChannelTypeForum, Mode: model.AMChannelModeIdeas, RootPosts: model.AMRootPostsMembers, View: model.AMForumViewImageboard}, th.App.GetChannelTypeInfo(channel.Id))

		// Again, with other settings: they're taken, the type stays.
		data.AMMode, data.AMForumView = model.NewPointer(model.AMChannelModeQA), nil
		require.Nil(t, th.App.importChannel(th.Context, data, false))
		assert.Equal(t, model.ChannelPresetForumQA, th.App.GetChannelTypeInfo(channel.Id).Preset())

		data.AMType, data.AMMode = model.NewPointer(model.AMChannelTypeVoice), nil
		appErr = th.App.importChannel(th.Context, data, false)
		require.NotNil(t, appErr, "an import can't change the type")
		assert.Equal(t, "api.channel.type.fixed.app_error", appErr.Id)
	})

	t.Run("invalid settings fail validation", func(t *testing.T) {
		name := "invalid-" + model.NewId()
		data := &imports.ChannelImportData{
			Team: &th.BasicTeam.Name, Name: &name, DisplayName: model.NewPointer("Invalid"), Type: model.NewPointer(model.ChannelTypeOpen),
			AMType: model.NewPointer(model.AMChannelTypeText), AMMode: model.NewPointer(model.AMChannelModeStage),
		}
		appErr := imports.ValidateChannelImportData(data)
		require.NotNil(t, appErr)
		assert.Equal(t, "app.import.validate_channel_import_data.am_type_invalid.error", appErr.Id)
		require.NotNil(t, th.App.importChannel(th.Context, data, true), "a dry run says so too")
	})
}
