// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {CommunityNameDisplay} from '@mattermost/types/community_profiles';

import {patchTeam, removeTeamIcon, setTeamIcon} from 'mattermost-redux/actions/teams';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {isCustomGroupsEnabled} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';

import {Choice, Section} from 'fusion/modals/settings_controls';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';
import Constants from 'utils/constants';
import {imageURLForTeam} from 'utils/utils';

import type {GlobalState} from 'types/store';

import ChannelsSettings from './channels_settings';
import {TeamLayoutSettings} from './team_layout_dialog';

const ICON_TYPES = ['image/jpeg', 'image/png', 'image/bmp'];

type Props = {
    setUnsaved?: (unsaved: boolean) => void;
};

// GeneralSettings is the General tab of the community settings: its name, description and icon; then everything
// members do with its channels and categories: its kind, whether everyone is in all its channels, where people land,
// where joining and leaving are told, and its layout (presets and templates of categories and roles).
export default function GeneralSettings({setUnsaved}: Props) {
    const {formatMessage} = useIntl();
    const customGroups = useSelector(isCustomGroupsEnabled);
    return (
        <div className={am('general-pane')}>
            <CommunityInfo setUnsaved={setUnsaved}/>
            <Section title={formatMessage({id: 'fusion.teamGeneral.people', defaultMessage: 'People'})}/>
            <NameDisplaySetting/>
            <Section title={formatMessage({id: 'fusion.teamGeneral.channels', defaultMessage: 'Channels'})}/>
            <ChannelsSettings/>
            {customGroups && (
                <>
                    <Section title={formatMessage({id: 'fusion.layout.title', defaultMessage: 'Community layout'})}/>
                    <TeamLayoutSettings embedded={true}/>
                </>
            )}
        </div>
    );
}

// NameDisplaySetting is how the community shows people in its channels, member list, mentions and cards: by
// username, nickname (their nickname there, else their global one, else their full name) or full name, for everyone
// in it. Each member's own choice still applies to direct and group messages. Until the admins choose, the server's
// default applies. Whoever hides their full name and nickname from the community (its privacy settings) shows as their
// username there whatever this is.
export function NameDisplaySetting() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const team = useSelector(getCurrentTeam);
    const serverDefault = useSelector((state: GlobalState) => getConfig(state).TeammateNameDisplay) || 'username';
    if (!team) {
        return null;
    }
    const value = (team.name_display || serverDefault) as CommunityNameDisplay;
    return (
        <Choice<CommunityNameDisplay>
            title={formatMessage({id: 'fusion.teamGeneral.nameDisplay', defaultMessage: 'Show people as'})}
            desc={formatMessage({id: 'fusion.teamGeneral.nameDisplayDesc', defaultMessage: 'For everyone, in this community\'s channels, member list, mentions and cards; a nickname set for the community comes first. Members choose for their direct and group messages. Those who hide their name from the community show as their username.'})}
            value={value}
            options={[
                ['username', formatMessage({id: 'fusion.settings.namesUsername', defaultMessage: 'Username'})],
                ['nickname_full_name', formatMessage({id: 'fusion.settings.namesNickname', defaultMessage: 'Nickname'})],
                ['full_name', formatMessage({id: 'fusion.settings.namesFull', defaultMessage: 'Full name'})],
            ]}
            onChange={async (next) => {
                const result = await dispatch(patchTeam({id: team.id, name_display: next}));
                if ('error' in result && result.error) {
                    toast((result.error as {message?: string}).message || formatMessage({id: 'fusion.teamChannels.failed', defaultMessage: 'The setting couldn\'t be saved.'}));
                } else {
                    toast(formatMessage({id: 'fusion.teamGeneral.nameDisplaySaved', defaultMessage: 'People show {display, select, username {by username} nickname_full_name {by nickname} other {by full name}} in this community'}, {display: next}));
                }
            }}
        />
    );
}

// CommunityInfo edits the community's name and description, saved together, and its icon, saved as soon as it's picked.
export function CommunityInfo({setUnsaved}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const team = useSelector(getCurrentTeam);
    const maxFileSize = parseInt(useSelector((state: GlobalState) => getConfig(state).MaxFileSize) || '', 10);
    const [name, setName] = useState(team?.display_name || '');
    const [description, setDescription] = useState(team?.description || '');
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');
    const [iconBusy, setIconBusy] = useState(false);
    const fileRef = useRef<HTMLInputElement>(null);

    // What was saved, until the community comes back changed from the server.
    const [justSaved, setJustSaved] = useState<{name: string; description: string} | null>(null);
    useEffect(() => setJustSaved(null), [team?.display_name, team?.description]);
    const savedName = justSaved ? justSaved.name : team?.display_name || '';
    const savedDescription = justSaved ? justSaved.description : team?.description || '';
    const changed = name !== savedName || description !== savedDescription;
    useEffect(() => {
        setUnsaved?.(changed);
    }, [changed, setUnsaved]);
    useEffect(() => () => setUnsaved?.(false), [setUnsaved]);

    if (!team) {
        return null;
    }

    const trimmed = name.trim();
    const nameProblem = trimmed.length < Constants.MIN_TEAMNAME_LENGTH ? formatMessage({id: 'fusion.teamGeneral.nameLength', defaultMessage: 'The name needs {min} to {max} characters.'}, {min: Constants.MIN_TEAMNAME_LENGTH, max: Constants.MAX_TEAMNAME_LENGTH}) : '';

    const save = async () => {
        if (nameProblem) {
            setError(nameProblem);
            return;
        }
        setBusy(true);
        setError('');
        const result = await dispatch(patchTeam({id: team.id, display_name: trimmed, description: description.trim()}));
        setBusy(false);
        if ('error' in result && result.error) {
            setError((result.error as {message?: string}).message || formatMessage({id: 'fusion.teamGeneral.failed', defaultMessage: 'The community couldn\'t be saved.'}));
            return;
        }
        setName(trimmed);
        setDescription(description.trim());
        setJustSaved({name: trimmed, description: description.trim()});
        toast(formatMessage({id: 'fusion.teamGeneral.saved', defaultMessage: 'Saved'}));
    };

    const pickIcon = async (file: File | undefined) => {
        if (!file) {
            return;
        }
        if (!ICON_TYPES.includes(file.type)) {
            toast(formatMessage({id: 'fusion.teamGeneral.iconType', defaultMessage: 'The icon must be a BMP, JPG or PNG image.'}));
            return;
        }
        if (maxFileSize && file.size > maxFileSize) {
            toast(formatMessage({id: 'fusion.teamGeneral.iconSize', defaultMessage: 'That image is too large.'}));
            return;
        }
        setIconBusy(true);
        const result = await dispatch(setTeamIcon(team.id, file));
        setIconBusy(false);
        if ('error' in result && result.error) {
            toast((result.error as {message?: string}).message || formatMessage({id: 'fusion.teamGeneral.iconFailed', defaultMessage: 'The icon couldn\'t be saved.'}));
        }
    };

    const dropIcon = async () => {
        setIconBusy(true);
        const result = await dispatch(removeTeamIcon(team.id));
        setIconBusy(false);
        if ('error' in result && result.error) {
            toast((result.error as {message?: string}).message || formatMessage({id: 'fusion.teamGeneral.iconFailed', defaultMessage: 'The icon couldn\'t be saved.'}));
        }
    };

    const icon = imageURLForTeam(team);
    const initials = (team.display_name || team.name).slice(0, 2);
    return (
        <div className={am('community-info')}>
            <div className={am('community-icon-row')}>
                <span
                    className={am('community-icon')}
                    aria-hidden={true}
                >
                    {icon ? (
                        <img
                            src={icon}
                            alt=''
                        />
                    ) : initials}
                </span>
                <div className={am('community-icon-actions')}>
                    <b>{formatMessage({id: 'fusion.teamGeneral.icon', defaultMessage: 'Icon'})}</b>
                    <span>{formatMessage({id: 'fusion.teamGeneral.iconDesc', defaultMessage: 'Shown in the list of communities. A square BMP, JPG or PNG image.'})}</span>
                    <div>
                        <button
                            type='button'
                            className={am('btn', 'small')}
                            disabled={iconBusy}
                            onClick={() => fileRef.current?.click()}
                        >
                            {icon ? formatMessage({id: 'fusion.teamGeneral.iconChange', defaultMessage: 'Change'}) : formatMessage({id: 'fusion.teamGeneral.iconUpload', defaultMessage: 'Upload'})}
                        </button>
                        {icon && (
                            <button
                                type='button'
                                className={am('btn', 'small')}
                                disabled={iconBusy}
                                onClick={dropIcon}
                            >
                                {formatMessage({id: 'fusion.teamGeneral.iconRemove', defaultMessage: 'Remove'})}
                            </button>
                        )}
                    </div>
                </div>
                <input
                    ref={fileRef}
                    type='file'
                    accept={Constants.ACCEPT_STATIC_IMAGE}
                    style={{display: 'none'}}
                    aria-label={formatMessage({id: 'fusion.teamGeneral.icon', defaultMessage: 'Icon'})}
                    onChange={(e) => {
                        pickIcon(e.target.files?.[0]);
                        e.target.value = '';
                    }}
                />
            </div>
            <label className={am('field')}>
                <span>{formatMessage({id: 'fusion.teamGeneral.name', defaultMessage: 'Name'})}</span>
                <input
                    type='text'
                    value={name}
                    maxLength={Constants.MAX_TEAMNAME_LENGTH}
                    onChange={(e) => setName(e.target.value)}
                />
            </label>
            <label className={am('field')}>
                <span>{formatMessage({id: 'fusion.teamGeneral.description', defaultMessage: 'Description'})}</span>
                <input
                    type='text'
                    value={description}
                    maxLength={Constants.MAX_TEAMDESCRIPTION_LENGTH}
                    placeholder={formatMessage({id: 'fusion.teamGeneral.descriptionPlaceholder', defaultMessage: 'What the community is about, in a few words'})}
                    onChange={(e) => setDescription(e.target.value)}
                />
            </label>
            {error && (
                <p
                    className={am('field-error')}
                    role='alert'
                >
                    {error}
                </p>
            )}
            <div className={am('modal-actions')}>
                <button
                    type='button'
                    className={am('btn')}
                    disabled={!changed || busy}
                    onClick={() => {
                        setName(savedName);
                        setDescription(savedDescription);
                        setError('');
                    }}
                >
                    {formatMessage({id: 'fusion.teamGeneral.undo', defaultMessage: 'Undo changes'})}
                </button>
                <button
                    type='button'
                    className={am('btn', 'primary')}
                    disabled={!changed || busy}
                    onClick={save}
                >
                    {formatMessage({id: 'fusion.teamGeneral.save', defaultMessage: 'Save'})}
                </button>
            </div>
        </div>
    );
}
