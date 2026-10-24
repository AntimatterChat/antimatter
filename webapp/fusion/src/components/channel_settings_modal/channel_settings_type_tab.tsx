// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useEffect, useMemo, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Channel, ChannelForumView, ChannelKindMode, ChannelPresetId, ChannelRootPosts} from '@mattermost/types/channels';
import type {ServerError} from '@mattermost/types/errors';

import {updateChannelType} from 'mattermost-redux/actions/channels';
import {getChannelTypeInfo} from 'mattermost-redux/selectors/entities/channel_types';

import type {SaveChangesPanelState} from 'components/widgets/modals/components/save_changes_panel';
import SaveChangesPanel from 'components/widgets/modals/components/save_changes_panel';

import {channelPreset, FORUM_VIEW_OPTIONS, presetForInfo, ROOT_POSTS_OPTIONS, rootPostsLabel} from 'fusion/channel/channel_type_presets';
import PresetPicker from 'fusion/channel/preset_picker';
import {Choice} from 'fusion/modals/settings_controls';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

type Props = {
    channel: Channel;
    setAreThereUnsavedChanges?: (unsaved: boolean) => void;
    showTabSwitchError?: boolean;
};

// ChannelSettingsTypeTab is the Type tab (design §4.2): the PresetPicker with the channel's type locked, and the two
// settings a preset bundles (who starts conversations or opens topics, and a forum's default view) fine-tuned
// separately. Picking a preset card sets mode, root_posts and (for forums) view together; the controls below can then
// move away from it, which shows no card selected ("custom"). Overrides on create_root_post or create_post keep
// applying on top of whatever root_posts becomes: presets write no overrides.
function ChannelSettingsTypeTab({channel, setAreThereUnsavedChanges, showTabSwitchError}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();

    const initial = useSelector((state: GlobalState) => getChannelTypeInfo(state, channel.id));
    const [baseline, setBaseline] = useState(initial);
    const [mode, setMode] = useState<ChannelKindMode>(initial.mode);
    const [rootPosts, setRootPosts] = useState<ChannelRootPosts>(initial.root_posts);
    const [view, setView] = useState<ChannelForumView>(initial.view || 'topics');

    const [isSaving, setIsSaving] = useState(false);
    const [saveChangesPanelState, setSaveChangesPanelState] = useState<SaveChangesPanelState>();
    const [formError, setFormError] = useState('');

    const hasUnsavedChanges = mode !== baseline.mode || rootPosts !== baseline.root_posts || (initial.type === 'forum' && view !== (baseline.view || 'topics'));

    useEffect(() => {
        setAreThereUnsavedChanges?.(hasUnsavedChanges);
        if (hasUnsavedChanges) {
            setSaveChangesPanelState((current) => (current === 'saved' ? undefined : current));
        }
    }, [hasUnsavedChanges, setAreThereUnsavedChanges]);

    const selectedPreset = useMemo(
        () => presetForInfo({type: initial.type, mode, root_posts: rootPosts, view}),
        [initial.type, mode, rootPosts, view],
    );

    const handlePresetPick = useCallback((presetId: ChannelPresetId) => {
        const preset = channelPreset(presetId);
        if (!preset) {
            return;
        }
        setMode(preset.mode);
        setRootPosts(preset.root_posts);
        if (preset.view) {
            setView(preset.view);
        }
    }, []);

    const handleSave = useCallback(async () => {
        const body = {mode, root_posts: rootPosts, ...(initial.type === 'forum' ? {view} : {})};
        const {error} = await dispatch(updateChannelType(channel.id, body));
        if (error) {
            setFormError((error as ServerError).message || formatMessage({id: 'channel_settings.unknown_error', defaultMessage: 'Something went wrong.'}));
            return false;
        }
        return true;
    }, [channel.id, dispatch, formatMessage, initial.type, mode, rootPosts, view]);

    const handleSaveChanges = useCallback(async () => {
        setIsSaving(true);
        try {
            const success = await handleSave();
            if (!success) {
                setSaveChangesPanelState('error');
                return;
            }
            setBaseline({type: initial.type, mode, root_posts: rootPosts, view, preset: selectedPreset});
            setFormError('');
            setSaveChangesPanelState('saved');
        } finally {
            setIsSaving(false);
        }
    }, [handleSave, initial.type, mode, rootPosts, selectedPreset, view]);

    const handleCancel = useCallback(() => {
        setMode(baseline.mode);
        setRootPosts(baseline.root_posts);
        setView(baseline.view || 'topics');
        setFormError('');
        setSaveChangesPanelState(undefined);
    }, [baseline]);

    const handleClose = useCallback(() => {
        setSaveChangesPanelState(undefined);
    }, []);

    const showSaveChangesPanel = hasUnsavedChanges || saveChangesPanelState === 'saved';

    return (
        <div data-testid='channel-settings-type-tab'>
            <PresetPicker
                lockedType={initial.type}
                value={selectedPreset}
                onChange={handlePresetPick}
            />
            <Choice
                title={formatMessage(rootPostsLabel(initial.type))}
                value={rootPosts}
                options={ROOT_POSTS_OPTIONS.map((o): [ChannelRootPosts, string] => [o.id, formatMessage(o.label)])}
                onChange={setRootPosts}
            />
            <p className={am('lead')}>{formatMessage({id: 'fusion.channelType.overridesNote', defaultMessage: 'Roles and people can be allowed or refused in Permissions.', description: 'Channel settings Type tab: points to where overrides on who starts conversations or opens topics are set.'})}</p>
            {initial.type === 'forum' && (
                <Choice
                    title={formatMessage({id: 'fusion.channelType.defaultView', defaultMessage: 'Default view', description: 'Channel settings Type tab: the forum\'s default view setting, topics list or image board.'})}
                    value={view}
                    options={FORUM_VIEW_OPTIONS.map((o): [ChannelForumView, string] => [o.id, formatMessage(o.label)])}
                    onChange={setView}
                />
            )}
            <p className={am('lead')}>{formatMessage({id: 'fusion.channelType.switchingNote', defaultMessage: 'Your permission changes stay.', description: 'Channel settings Type tab: picking another preset keeps the channel\'s existing permission overrides.'})}</p>
            {showSaveChangesPanel && (
                <SaveChangesPanel
                    handleSubmit={handleSaveChanges}
                    handleCancel={handleCancel}
                    handleClose={handleClose}
                    tabChangeError={Boolean(formError) || showTabSwitchError}
                    state={formError ? 'error' : saveChangesPanelState}
                    saving={isSaving}
                    customErrorMessage={formError}
                />
            )}
        </div>
    );
}

export default ChannelSettingsTypeTab;
