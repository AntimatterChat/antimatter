// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app/properties"
)

// Antimatter channel types (model/channel_type.go): text, voice or forum, a mode and settings. They're stored as
// property values of the protected group model.ChannelTypePropertyGroupName, one row per setting that isn't its
// default, keyed by the channel; plain text channels (every channel made before channel types) have none. Every node
// keeps the typed channels in memory (Channels.channelTypes), loaded at startup and kept up to date on each write
// (clusterEventInvalidateChannelType for the other nodes), so that checks on every post cost no query.

// clusterEventInvalidateChannelType tells the other nodes a channel's type settings changed; its data is the channel,
// whose entry they reload.
const clusterEventInvalidateChannelType = model.ClusterEvent("inv_channel_type")

// channelTypesVoiceMigrationKey marks the one-time migration of the voice-channels app's channels done.
const channelTypesVoiceMigrationKey = "AmChannelTypesVoiceMigrated"

// The voice-channels app, whose channels became voice channels, and the prefix of its keys for them (vc_<channel id>).
const (
	voiceChannelsPluginID      = "com.antimatterchat.voice-channels"
	voiceChannelsKVPrefix      = "vc_"
	channelTypeValuesPageSize  = 1000
	voiceChannelsKVListPerPage = 200
)

// doSetupChannelTypeProperties registers the channel types' property group and its fields, then loads the typed
// channels into memory. It's idempotent: it runs at every start.
func (s *Server) doSetupChannelTypeProperties() error {
	group, err := s.propertyService.RegisterPropertyGroup(&model.PropertyGroup{Name: model.ChannelTypePropertyGroupName, Version: model.PropertyGroupVersionV2})
	if err != nil {
		return fmt.Errorf("failed to register the channel type property group: %w", err)
	}
	rctx := properties.SystemCallerContext(request.EmptyContext(s.Log()))

	fieldIDs := make(map[string]string, len(model.ChannelTypeFields()))
	for _, name := range model.ChannelTypeFields() {
		field, getErr := s.propertyService.GetPropertyFieldByNameForObjectType(rctx, group.ID, "", model.PropertyFieldObjectTypeChannel, name)
		if getErr != nil {
			field, err = s.propertyService.CreatePropertyField(rctx, &model.PropertyField{
				GroupID:           group.ID,
				Name:              name,
				Type:              model.PropertyFieldTypeText,
				ObjectType:        model.PropertyFieldObjectTypeChannel,
				TargetType:        "system",
				Protected:         true,
				PermissionField:   model.NewPointer(model.PermissionLevelNone),
				PermissionValues:  model.NewPointer(model.PermissionLevelNone),
				PermissionOptions: model.NewPointer(model.PermissionLevelNone),
			})
			if err != nil {
				// Another node may have made it meanwhile.
				var retryErr error
				if field, retryErr = s.propertyService.GetPropertyFieldByNameForObjectType(rctx, group.ID, "", model.PropertyFieldObjectTypeChannel, name); retryErr != nil {
					return fmt.Errorf("failed to create the channel type field %q: %w", name, err)
				}
			}
		}
		fieldIDs[name] = field.ID
	}

	ch := s.Channels()
	ch.channelTypeGroupID = group.ID
	ch.channelTypeFieldIDs = fieldIDs
	ch.channelTypeFieldNames = make(map[string]string, len(fieldIDs))
	for name, id := range fieldIDs {
		ch.channelTypeFieldNames[id] = name
	}
	return ch.loadAllChannelTypes(rctx)
}

// loadAllChannelTypes reads every typed channel's settings, a page at a time.
func (ch *Channels) loadAllChannelTypes(rctx request.CTX) error {
	opts := model.PropertyValueSearchOpts{
		GroupID:    ch.channelTypeGroupID,
		TargetType: model.PropertyValueTargetTypeChannel,
		PerPage:    channelTypeValuesPageSize,
	}
	byChannel := map[string]map[string]string{}
	for {
		page, err := ch.srv.propertyService.SearchPropertyValues(rctx, ch.channelTypeGroupID, opts)
		if err != nil {
			return fmt.Errorf("failed to load the channel types: %w", err)
		}
		for _, v := range page {
			ch.collectChannelTypeValue(rctx, byChannel, v)
		}
		if len(page) < channelTypeValuesPageSize {
			break
		}
		last := page[len(page)-1]
		opts.Cursor = model.PropertyValueSearchCursor{PropertyValueID: last.ID, CreateAt: last.CreateAt}
	}

	ch.channelTypes.Clear()
	for channelID, values := range byChannel {
		ch.storeChannelType(channelID, model.ChannelTypeInfoFromFieldValues(values))
	}
	return nil
}

// collectChannelTypeValue adds a stored value to the values of its channel, by field name.
func (ch *Channels) collectChannelTypeValue(rctx request.CTX, byChannel map[string]map[string]string, v *model.PropertyValue) {
	name, ok := ch.channelTypeFieldNames[v.FieldID]
	if !ok {
		return
	}
	var value string
	if err := json.Unmarshal(v.Value, &value); err != nil {
		rctx.Logger().Warn("Failed to read a channel type value", mlog.String("channel_id", v.TargetID), mlog.String("field", name), mlog.Err(err))
		return
	}
	if byChannel[v.TargetID] == nil {
		byChannel[v.TargetID] = map[string]string{}
	}
	byChannel[v.TargetID][name] = value
}

// storeChannelType keeps a channel's settings in memory; plain text channels aren't kept.
func (ch *Channels) storeChannelType(channelID string, info model.ChannelTypeInfo) {
	if info.IsDefault() {
		ch.channelTypes.Delete(channelID)
		return
	}
	ch.channelTypes.Store(channelID, info)
}

// reloadChannelType reads one channel's settings again, from the master database, which a replica may lag behind.
func (ch *Channels) reloadChannelType(rctx request.CTX, channelID string) error {
	if ch.channelTypeGroupID == "" {
		return nil
	}
	values, err := ch.srv.propertyService.SearchPropertyValues(RequestContextWithMaster(rctx), ch.channelTypeGroupID, model.PropertyValueSearchOpts{
		GroupID:    ch.channelTypeGroupID,
		TargetType: model.PropertyValueTargetTypeChannel,
		TargetIDs:  []string{channelID},
		PerPage:    len(model.ChannelTypeFields()) * 2,
	})
	if err != nil {
		return err
	}
	byChannel := map[string]map[string]string{}
	for _, v := range values {
		ch.collectChannelTypeValue(rctx, byChannel, v)
	}
	ch.storeChannelType(channelID, model.ChannelTypeInfoFromFieldValues(byChannel[channelID]))
	return nil
}

// clusterInvalidateChannelTypeHandler reloads the channel another node changed the type settings of.
func (ch *Channels) clusterInvalidateChannelTypeHandler(msg *model.ClusterMessage) {
	channelID := string(msg.Data)
	if !model.IsValidId(channelID) {
		return
	}
	rctx := request.EmptyContext(ch.srv.Log())
	if err := ch.reloadChannelType(rctx, channelID); err != nil {
		rctx.Logger().Warn("Failed to reload a channel's type", mlog.String("channel_id", channelID), mlog.Err(err))
	}
}

func (a *App) sendChannelTypeInvalidation(channelID string) {
	if cluster := a.Srv().Platform().Cluster(); cluster != nil {
		cluster.SendClusterMessage(&model.ClusterMessage{
			Event:    clusterEventInvalidateChannelType,
			SendType: model.ClusterSendReliable,
			Data:     []byte(channelID),
		})
	}
}

// GetChannelTypeInfo gives a channel's type and settings, from memory: a plain text channel's for one without stored
// settings, direct and group messages included.
func (a *App) GetChannelTypeInfo(channelID string) model.ChannelTypeInfo {
	if v, ok := a.Channels().channelTypes.Load(channelID); ok {
		return v.(model.ChannelTypeInfo)
	}
	return model.DefaultChannelTypeInfo()
}

// FillInChannelTypes puts each team channel's type and settings in its props (am_type, am_mode, am_root_posts,
// am_preset, and am_forum_view for forums), text channels included; direct and group messages get nothing.
func (a *App) FillInChannelTypes(channels ...*model.Channel) {
	for _, channel := range channels {
		if channel == nil || channel.IsGroupOrDirect() || channel.TeamId == "" {
			continue
		}
		channel.AddChannelTypeProps(a.GetChannelTypeInfo(channel.Id))
	}
}

// FillInChannelTypesWithTeamData is FillInChannelTypes for the lists that carry the channels' teams.
func (a *App) FillInChannelTypesWithTeamData(channels model.ChannelListWithTeamData) {
	for i := range channels {
		a.FillInChannelTypes(&channels[i].Channel)
	}
}

// channelTypeForCreate reads a new channel's type from its props (am_preset, then the am_ fields on top), and drops
// every am_ prop it carried. A channel without any is a plain text channel.
func channelTypeForCreate(channel *model.Channel) (model.ChannelTypeInfo, *model.AppError) {
	if !model.HasChannelTypeProps(channel.Props) {
		return model.DefaultChannelTypeInfo(), nil
	}
	if channel.IsGroupOrDirect() {
		return model.ChannelTypeInfo{}, model.NewAppError("CreateChannel", "api.channel.type.direct.app_error", nil, "", http.StatusBadRequest)
	}
	req, appErr := model.ChannelTypeRequestFromProps(channel.Props)
	if appErr != nil {
		return model.ChannelTypeInfo{}, appErr
	}
	model.RemoveChannelTypeProps(channel.Props)
	return req.Apply(model.DefaultChannelTypeInfo())
}

// isVoiceChannelsAppActive tells whether the voice-channels app is active. A voice channel needs it: it sets up and
// enforces the channel's calls (§3.4 of the design), so creating one without the app would leave a voice channel
// with no call. Plugins unavailable (disabled by config, or not yet initialised) counts as inactive: fail closed.
func (a *App) isVoiceChannelsAppActive() bool {
	env := a.GetPluginsEnvironment()
	return env != nil && env.IsActive(voiceChannelsPluginID)
}

// checkVoiceChannelsAppForCreate refuses creating a voice channel while the voice-channels app isn't active, from
// every creation path (they all reach App.CreateChannel).
func (a *App) checkVoiceChannelsAppForCreate(info model.ChannelTypeInfo) *model.AppError {
	if info.Type != model.AMChannelTypeVoice || a.isVoiceChannelsAppActive() {
		return nil
	}
	return model.NewAppError("CreateChannel", "api.channel.type.voice_unavailable.app_error", nil, "", http.StatusBadRequest)
}

// writeChannelType stores a channel's settings: with onlyNonDefault (a new channel, which has no rows), only those
// that aren't their defaults; otherwise every one the type has, so that rows written before follow. It then updates
// this node's memory and tells the others.
func (a *App) writeChannelType(rctx request.CTX, channelID, userID string, info model.ChannelTypeInfo, onlyNonDefault bool) *model.AppError {
	ch := a.Channels()
	if ch.channelTypeGroupID == "" {
		return model.NewAppError("writeChannelType", "app.channel.type.save.app_error", nil, "channel types aren't set up", http.StatusInternalServerError)
	}
	// What a missing row means: text, and the type's own defaults for the rest.
	defaults := model.ChannelTypeInfo{Type: info.Type}.Normalised().FieldValues()
	defaults[model.ChannelTypeFieldType] = model.AMChannelTypeText
	values := []*model.PropertyValue{}
	for name, value := range info.FieldValues() {
		if onlyNonDefault && value == defaults[name] {
			continue
		}
		valueJSON, err := json.Marshal(value)
		if err != nil {
			return model.NewAppError("writeChannelType", "app.channel.type.save.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
		values = append(values, &model.PropertyValue{
			GroupID:    ch.channelTypeGroupID,
			FieldID:    ch.channelTypeFieldIDs[name],
			TargetID:   channelID,
			TargetType: model.PropertyValueTargetTypeChannel,
			Value:      valueJSON,
			CreatedBy:  userID,
			UpdatedBy:  userID,
		})
	}
	if len(values) > 0 {
		if _, err := a.Srv().propertyService.UpsertPropertyValues(rctx, values); err != nil {
			return model.NewAppError("writeChannelType", "app.channel.type.save.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
	}
	ch.storeChannelType(channelID, info)
	a.sendChannelTypeInvalidation(channelID)
	return nil
}

// forgetChannelType removes a permanently deleted channel's settings.
func (a *App) forgetChannelType(rctx request.CTX, channelID string) {
	ch := a.Channels()
	if ch.channelTypeGroupID == "" {
		return
	}
	if _, typed := ch.channelTypes.Load(channelID); typed {
		if err := a.Srv().propertyService.DeletePropertyValuesForTarget(rctx, ch.channelTypeGroupID, model.PropertyValueTargetTypeChannel, channelID); err != nil {
			rctx.Logger().Warn("Failed to delete the type of a deleted channel", mlog.String("channel_id", channelID), mlog.Err(err))
		}
	}
	ch.channelTypes.Delete(channelID)
	a.sendChannelTypeInvalidation(channelID)
}

// setChannelTypeOnCreate stores a new channel's type, and removes the channel again if that fails, as for its
// attribute values (setChannelPropertyValuesOnCreate): nobody has been told the channel exists yet.
func (a *App) setChannelTypeOnCreate(rctx request.CTX, channel *model.Channel, info model.ChannelTypeInfo) *model.AppError {
	if info.IsDefault() {
		return nil
	}
	appErr := a.writeChannelType(rctx, channel.Id, channel.CreatorId, info, true)
	if appErr == nil {
		return nil
	}
	if deleteErr := a.PermanentDeleteChannel(rctx, channel); deleteErr != nil {
		rctx.Logger().Error("Failed to remove a channel after its type could not be written", mlog.String("channel_id", channel.Id), mlog.Err(deleteErr))
	}
	return appErr
}

// UpdateChannelType changes a team channel's mode and settings (a preset of its type, then the fields given). The type
// itself is fixed: a request for another one is refused. The channel comes back filled in, as the channel_updated
// event sends it.
func (a *App) UpdateChannelType(rctx request.CTX, channel *model.Channel, req model.ChannelTypeRequest, userID string) (*model.Channel, *model.AppError) {
	if channel.IsGroupOrDirect() || channel.TeamId == "" {
		return nil, model.NewAppError("UpdateChannelType", "api.channel.type.direct.app_error", nil, "", http.StatusBadRequest)
	}
	if req.IsEmpty() {
		return nil, model.NewAppError("UpdateChannelType", "api.channel.type.invalid.app_error", nil, "", http.StatusBadRequest)
	}
	current := a.GetChannelTypeInfo(channel.Id)
	if req.Type != "" && req.Type != current.Type {
		return nil, model.NewAppError("UpdateChannelType", "api.channel.type.fixed.app_error", nil, "", http.StatusBadRequest)
	}
	if req.Preset != "" {
		if preset, ok := model.ChannelPresetByID(req.Preset); ok && preset.Type != current.Type {
			return nil, model.NewAppError("UpdateChannelType", "api.channel.type.fixed.app_error", nil, "", http.StatusBadRequest)
		}
	}
	info, appErr := req.Apply(current)
	if appErr != nil {
		return nil, appErr
	}
	if info.Type != current.Type {
		return nil, model.NewAppError("UpdateChannelType", "api.channel.type.fixed.app_error", nil, "", http.StatusBadRequest)
	}
	if appErr = a.writeChannelType(rctx, channel.Id, userID, info, false); appErr != nil {
		return nil, appErr
	}

	updated := channel.DeepCopy()
	a.FillInChannelTypes(updated)
	a.publishChannelUpdated(updated)
	return updated, nil
}

// publishChannelUpdated sends a channel (filled in) to those who see it.
func (a *App) publishChannelUpdated(channel *model.Channel) {
	channelJSON, err := json.Marshal(channel)
	if err != nil {
		a.Log().Warn("Failed to encode a channel", mlog.String("channel_id", channel.Id), mlog.Err(err))
		return
	}
	message := model.NewWebSocketEvent(model.WebsocketEventChannelUpdated, "", channel.Id, "", nil, "")
	message.Add("channel", string(channelJSON))
	a.Publish(message)
}

// doChannelTypesVoiceMigration makes the voice-channels app's channels (its keys vc_<channel id>) voice channels,
// once, whether the app is enabled or not; channels that no longer exist are skipped. It touches nothing else.
func (s *Server) doChannelTypesVoiceMigration() error {
	if done, err := s.migrationDone(channelTypesVoiceMigrationKey); done || err != nil {
		return err
	}

	a := New(ServerConnector(s.Channels()))
	rctx := request.EmptyContext(s.Log())
	migrated := 0
	for offset := 0; ; offset += voiceChannelsKVListPerPage {
		keys, err := s.Store().Plugin().List(voiceChannelsPluginID, offset, voiceChannelsKVListPerPage)
		if err != nil {
			return fmt.Errorf("failed to list the voice channels: %w", err)
		}
		for _, key := range keys {
			channelID, ok := strings.CutPrefix(key, voiceChannelsKVPrefix)
			if !ok || !model.IsValidId(channelID) {
				continue
			}
			channel, getErr := s.Store().Channel().Get(channelID, false)
			if getErr != nil || channel.IsGroupOrDirect() || channel.TeamId == "" {
				continue
			}
			if a.GetChannelTypeInfo(channelID).Type == model.AMChannelTypeVoice {
				continue
			}
			info := model.ChannelTypeInfo{Type: model.AMChannelTypeVoice}.Normalised()
			if appErr := a.writeChannelType(rctx, channelID, "", info, true); appErr != nil {
				return fmt.Errorf("failed to make channel %s a voice channel: %w", channelID, appErr)
			}
			migrated++
		}
		if len(keys) < voiceChannelsKVListPerPage {
			break
		}
	}

	rctx.Logger().Info("Voice channels migrated to the voice type", mlog.Int("channels", migrated))
	return s.markMigrationDone(channelTypesVoiceMigrationKey)
}
