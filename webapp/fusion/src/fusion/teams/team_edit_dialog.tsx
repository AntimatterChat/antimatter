// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useMemo, useState} from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useDispatch, useSelector} from 'react-redux';

import type {Group} from '@mattermost/types/groups';

import {patchGroup} from 'mattermost-redux/actions/groups';
import {getMyChannels} from 'mattermost-redux/selectors/entities/channels';
import {getTeammateNameDisplaySetting} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentTeamId} from 'mattermost-redux/selectors/entities/teams';
import {getUser} from 'mattermost-redux/selectors/entities/users';
import {displayUsername} from 'mattermost-redux/utils/user_utils';

import {Dialog} from 'fusion/components/layer';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import {loadTeams} from './teams';

const TAG_MAX = 8;

// TeamEditDialog changes what makes a group an Antimatter team: its tag, what it does, its lead and its channel.
// Its name, @mention and members are edited with Mattermost's user groups.
export default function TeamEditDialog({team, onClose}: {team: Group; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const [tag, setTag] = useState(team.tag || '');
    const [description, setDescription] = useState(team.description || '');
    const [leadId, setLeadId] = useState(team.lead_id || '');
    const [channelId, setChannelId] = useState(team.channel_id || '');
    const [error, setError] = useState('');
    const [saving, setSaving] = useState(false);

    const nameSetting = useSelector(getTeammateNameDisplaySetting);
    const users = useSelector((state: GlobalState) => (team.member_ids || []).map((id) => getUser(state, id)), shallowEqual);
    const members = useMemo(() => (team.member_ids || []).
        map((id, i) => ({id, name: displayUsername(users[i], nameSetting) || id})).
        sort((a, b) => a.name.localeCompare(b.name)), [team.member_ids, users, nameSetting]);

    // The team's channel is one of its community's.
    const currentTeamId = useSelector(getCurrentTeamId);
    const communityId = team.team_id || currentTeamId;
    const myChannels = useSelector(getMyChannels);
    const channels = useMemo(() => myChannels.
        filter((c) => (c.type === 'O' || c.type === 'P') && c.delete_at === 0 && c.team_id === communityId).
        map((c) => ({id: c.id, name: c.display_name})).
        sort((a, b) => a.name.localeCompare(b.name)), [myChannels, communityId]);

    const save = async (e: React.FormEvent) => {
        e.preventDefault();
        setSaving(true);
        setError('');
        const result = await dispatch(patchGroup(team.id, {tag: tag.trim().toUpperCase(), description: description.trim(), lead_id: leadId, channel_id: channelId}));
        setSaving(false);
        if ('error' in result && result.error) {
            setError((result.error as {message?: string}).message || formatMessage({id: 'fusion.teams.saveFailed', defaultMessage: 'The team couldn\'t be saved.'}));
            return;
        }

        // A patched group comes back without its members: load the teams again.
        dispatch(loadTeams());
        onClose();
    };

    return (
        <Dialog
            label={formatMessage({id: 'fusion.teams.editTitle', defaultMessage: 'Edit {team}'}, {team: team.display_name})}
            onClose={onClose}
            onSubmit={save}
        >
            <div className={am('pane')}>
                <h2>{team.display_name}</h2>
                <p className={am('lead')}>{formatMessage({id: 'fusion.teams.editLead', defaultMessage: 'Members can show the team\'s tag next to their name. The card lists the lead and links the team\'s channel.'})}</p>
                <div className={am('field-row')}>
                    <label className={am('field', 'team-tag-field')}>
                        <span>{formatMessage({id: 'fusion.teams.tag', defaultMessage: 'Tag'})}</span>
                        <input
                            type='text'
                            value={tag}
                            maxLength={TAG_MAX}
                            placeholder={formatMessage({id: 'fusion.teams.tagPlaceholder', defaultMessage: 'e.g. DET'})}
                            onChange={(e) => setTag(e.target.value.toUpperCase())}
                        />
                        <small>{formatMessage({id: 'fusion.teams.tagHelp', defaultMessage: 'Up to {max} characters'}, {max: TAG_MAX})}</small>
                    </label>
                    <label className={am('field')}>
                        <span>{formatMessage({id: 'fusion.teams.leadLabel', defaultMessage: 'Lead'})}</span>
                        <select
                            value={leadId}
                            onChange={(e) => setLeadId(e.target.value)}
                        >
                            <option value=''>{formatMessage({id: 'fusion.teams.noLead', defaultMessage: 'No lead'})}</option>
                            {members.map((m) => (
                                <option
                                    key={m.id}
                                    value={m.id}
                                >
                                    {m.name}
                                </option>
                            ))}
                        </select>
                    </label>
                </div>
                <label className={am('field')}>
                    <span>{formatMessage({id: 'fusion.teams.channelLabel', defaultMessage: 'Team channel'})}</span>
                    <select
                        value={channelId}
                        onChange={(e) => setChannelId(e.target.value)}
                    >
                        <option value=''>{formatMessage({id: 'fusion.teams.noChannelOption', defaultMessage: 'No team channel'})}</option>
                        {channelId && !channels.some((c) => c.id === channelId) && (
                            <option value={channelId}>{formatMessage({id: 'fusion.teams.otherChannel', defaultMessage: 'The current channel (you aren\'t in it)'})}</option>
                        )}
                        {channels.map((c) => (
                            <option
                                key={c.id}
                                value={c.id}
                            >
                                {`#${c.name}`}
                            </option>
                        ))}
                    </select>
                </label>
                <label className={am('field')}>
                    <span>{formatMessage({id: 'fusion.teams.description', defaultMessage: 'What the team does'})}</span>
                    <textarea
                        rows={3}
                        value={description}
                        maxLength={1024}
                        onChange={(e) => setDescription(e.target.value)}
                    />
                </label>
                {error && (
                    <p
                        className={am('lead')}
                        role='alert'
                        style={{color: 'var(--am-danger)'}}
                    >
                        {error}
                    </p>
                )}
                <div className={am('modal-actions')}>
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={onClose}
                    >
                        {formatMessage({id: 'fusion.teams.cancel', defaultMessage: 'Cancel'})}
                    </button>
                    <button
                        className={am('btn', 'primary')}
                        disabled={saving}
                    >
                        {formatMessage({id: 'fusion.teams.save', defaultMessage: 'Save'})}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}
