// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useRef} from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import type {ChannelKindType, ChannelPresetId} from '@mattermost/types/channels';

import Icon from 'fusion/components/icon';
import {useIsPhone} from 'fusion/hooks/media';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import {CHANNEL_TYPES, channelPresetsOfType, channelTypeLabel, channelTypePresetsLabel} from './channel_type_presets';

// The voice-channels app's plugin id (server/public/model/plugin_constants.go's AntimatterPluginIds); the Voice
// segment needs its Fusion API, so it's disabled while the app isn't active (design §4.1).
const VOICE_CHANNELS_PLUGIN_ID = 'com.antimatterchat.voice-channels';

export type PresetPickerProps = {

    // The selected type, when it can still be changed (the create-channel dialog).
    type?: ChannelKindType;

    // The channel's fixed type (channel settings): the type switch is replaced by a line naming it.
    lockedType?: ChannelKindType;

    // The selected preset, or '' when the settings are custom (no card selected).
    value: ChannelPresetId | '';

    onTypeChange?: (type: ChannelKindType) => void;
    onChange: (preset: ChannelPresetId) => void;
};

// moveIndex is the arrow-key move of a roving radiogroup: left/right within a row, up/down by a row of `columns`.
// undefined means the key wasn't one of the four arrows, or the move would go past either end.
function moveIndex(ids: string[], current: string, key: string, columns: number): string | undefined {
    const idx = ids.indexOf(current);
    if (idx < 0) {
        return undefined;
    }
    let next = idx;
    switch (key) {
    case 'ArrowRight':
        next = idx + 1;
        break;
    case 'ArrowLeft':
        next = idx - 1;
        break;
    case 'ArrowDown':
        next = idx + columns;
        break;
    case 'ArrowUp':
        next = idx - columns;
        break;
    default:
        return undefined;
    }
    return next >= 0 && next < ids.length ? ids[next] : undefined;
}

// PresetPicker is the picker of the design's §4.1, option A: a type switch (text, voice, forum) and, under it, the
// selected type's preset cards, two to a row. One component, shared by the create-channel dialog (type and
// onTypeChange given) and the channel settings Type tab (lockedType: the type is fixed after creation, so only the
// switch is replaced by a line naming it). Both radiogroups are keyboard-navigable: arrow keys move inside one, Tab
// moves between them and out, as a native radio group's.
export default function PresetPicker({type, lockedType, value, onTypeChange, onChange}: PresetPickerProps) {
    const {formatMessage} = useIntl();
    const phone = useIsPhone();
    const voiceAvailable = useSelector((state: GlobalState) => Boolean(state.plugins.plugins[VOICE_CHANNELS_PLUGIN_ID]));
    const activeType = lockedType || type || 'text';
    const presets = channelPresetsOfType(activeType);
    const typeIds = CHANNEL_TYPES.map((t) => t.id);
    const presetIds = presets.map((p) => p.id);
    const activePresetId = value || presetIds[0];

    const typeRefs = useRef<Record<string, HTMLButtonElement | null>>({});
    const presetRefs = useRef<Record<string, HTMLButtonElement | null>>({});

    return (
        <div className={am('preset-picker', {phone})}>
            {lockedType ? (
                <p className={am('lead')}>
                    {formatMessage(
                        {id: 'fusion.presetPicker.locked', defaultMessage: '{type}. The type is set when the channel is created.', description: 'Channel settings Type tab: names the channel\'s fixed type (Text, Voice or Forum) and says it can\'t change.'},
                        {type: formatMessage(channelTypeLabel(lockedType))},
                    )}
                </p>
            ) : (
                <div className={am('field')}>
                    <span>{formatMessage({id: 'fusion.presetPicker.type', defaultMessage: 'Type', description: 'Label of the channel type switch (Text, Voice, Forum) in the create-channel dialog.'})}</span>
                    <div
                        className={am('seg', 'preset-picker-type-seg')}
                        role='radiogroup'
                        aria-label={formatMessage({id: 'fusion.presetPicker.type', defaultMessage: 'Type', description: 'Label of the channel type switch (Text, Voice, Forum) in the create-channel dialog.'})}
                    >
                        {CHANNEL_TYPES.map((t) => {
                            const disabled = t.id === 'voice' && !voiceAvailable;
                            return (
                                <button
                                    key={t.id}
                                    ref={(el) => {
                                        typeRefs.current[t.id] = el;
                                    }}
                                    type='button'
                                    role='radio'
                                    aria-checked={activeType === t.id}
                                    disabled={disabled}
                                    title={disabled ? formatMessage({id: 'fusion.presetPicker.voiceUnavailable', defaultMessage: 'Needs the Voice channels app', description: 'Tooltip on the disabled Voice segment of the channel type switch, while the voice-channels app isn\'t active.'}) : undefined}
                                    tabIndex={activeType === t.id ? 0 : -1}
                                    className={am({on: activeType === t.id})}
                                    onClick={() => onTypeChange?.(t.id)}
                                    onKeyDown={(e) => {
                                        const nextId = moveIndex(typeIds, activeType, e.key, typeIds.length);
                                        if (!nextId || (nextId === 'voice' && !voiceAvailable)) {
                                            return;
                                        }
                                        e.preventDefault();
                                        onTypeChange?.(nextId as ChannelKindType);
                                        typeRefs.current[nextId]?.focus();
                                    }}
                                >
                                    <Icon name={t.icon}/>
                                    {formatMessage(channelTypeLabel(t.id))}
                                </button>
                            );
                        })}
                    </div>
                </div>
            )}
            <div className={am('field')}>
                <span>{formatMessage(channelTypePresetsLabel(activeType))}</span>
                <div
                    className={am('preset-grid')}
                    role='radiogroup'
                    aria-label={formatMessage(channelTypePresetsLabel(activeType))}
                >
                    {presets.map((p) => (
                        <button
                            key={p.id}
                            ref={(el) => {
                                presetRefs.current[p.id] = el;
                            }}
                            type='button'
                            role='radio'
                            aria-checked={value === p.id}
                            tabIndex={p.id === activePresetId ? 0 : -1}
                            className={am('preset-card', {on: value === p.id})}
                            onClick={() => onChange(p.id)}
                            onKeyDown={(e) => {
                                const nextId = moveIndex(presetIds, activePresetId, e.key, 2);
                                if (!nextId) {
                                    return;
                                }
                                e.preventDefault();
                                onChange(nextId as ChannelPresetId);
                                presetRefs.current[nextId]?.focus();
                            }}
                        >
                            <b>{formatMessage(p.label)}</b>
                            <span>{formatMessage(p.description)}</span>
                        </button>
                    ))}
                </div>
            </div>
        </div>
    );
}
