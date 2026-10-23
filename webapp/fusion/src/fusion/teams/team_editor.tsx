// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import React, {useEffect, useMemo, useState} from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useDispatch, useSelector} from 'react-redux';

import type {Group, GroupTeamPatch} from '@mattermost/types/groups';
import type {UserProfile} from '@mattermost/types/users';

import {addUsersToGroup, createGroupWithUserIds, patchGroup, removeUsersFromGroup} from 'mattermost-redux/actions/groups';
import {getMissingProfilesByIds} from 'mattermost-redux/actions/users';
import {getMyChannels} from 'mattermost-redux/selectors/entities/channels';
import {getTeammateNameDisplaySetting} from 'mattermost-redux/selectors/entities/preferences';
import {getUser} from 'mattermost-redux/selectors/entities/users';
import {displayUsername} from 'mattermost-redux/utils/user_utils';

import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import PeopleSearch, {PersonName} from 'fusion/members/people_search';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import {loadTeams} from './teams';

export const TEAM_TAG_MAX = 8;

// What the editor changes: what makes a group an Antimatter team (its name, mention, tag, lead, channel and what it
// does) and its members.
export type TeamDraft = {
    display_name: string;
    name: string;
    tag: string;
    lead_id: string;
    channel_id: string;
    description: string;
    member_ids: string[];
};

// mentionFromName is the @mention a new team's name suggests.
export function mentionFromName(name: string): string {
    return name.toLowerCase().normalize('NFKD').replace(/\p{M}/gu, '').replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '').slice(0, 64);
}

export function draftOf(team?: Group): TeamDraft {
    return {
        display_name: team?.display_name || '',
        name: team?.name || '',
        tag: team?.tag || '',
        lead_id: team?.lead_id || '',
        channel_id: team?.channel_id || '',
        description: team?.description || '',
        member_ids: [...(team?.member_ids || [])],
    };
}

export function draftChanged(a: TeamDraft, b: TeamDraft): boolean {
    const members = (d: TeamDraft) => [...d.member_ids].sort().join(',');
    return a.display_name !== b.display_name || a.name !== b.name || a.tag !== b.tag || a.lead_id !== b.lead_id ||
        a.channel_id !== b.channel_id || a.description !== b.description || members(a) !== members(b);
}

type Props = {

    // The team edited; none to create one in the community.
    team?: Group;
    communityId: string;
    onDone: () => void;
    onCancel: () => void;
    setUnsaved?: (unsaved: boolean) => void;
};

// TeamEditor creates or edits a team of a community: its name and @mention, the tag its members can show next to their
// name (its badge), its lead (one of its members), its channel, what it does, and its members. Nothing is saved until
// Save: members are added first, then the team changed (its lead must be a member), then members removed.
export default function TeamEditor({team, communityId, onDone, onCancel, setUnsaved}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const saved = useMemo(() => draftOf(team), [team]);
    const [draft, setDraft] = useState<TeamDraft>(saved);
    const [mentionEdited, setMentionEdited] = useState(Boolean(team));
    const [error, setError] = useState('');
    const [saving, setSaving] = useState(false);
    const set = (patch: Partial<TeamDraft>) => setDraft((d) => ({...d, ...patch}));

    const changed = draftChanged(draft, saved);
    useEffect(() => {
        setUnsaved?.(changed);
    }, [changed, setUnsaved]);
    useEffect(() => () => setUnsaved?.(false), [setUnsaved]);

    useEffect(() => {
        dispatch(getMissingProfilesByIds(draft.member_ids));
    }, [draft.member_ids, dispatch]);

    const nameSetting = useSelector(getTeammateNameDisplaySetting);
    const users = useSelector((state: GlobalState) => draft.member_ids.map((id) => getUser(state, id)), shallowEqual);
    const members = useMemo(() => draft.member_ids.
        map((id, i) => ({id, name: displayUsername(users[i], nameSetting) || id})).
        sort((a, b) => a.name.localeCompare(b.name)), [draft.member_ids, users, nameSetting]);

    // The team's channel is one of its community's.
    const myChannels = useSelector(getMyChannels);
    const channels = useMemo(() => myChannels.
        filter((c) => (c.type === 'O' || c.type === 'P') && c.delete_at === 0 && c.team_id === communityId).
        map((c) => ({id: c.id, name: c.display_name})).
        sort((a, b) => a.name.localeCompare(b.name)), [myChannels, communityId]);

    const setName = (displayName: string) => set(mentionEdited ? {display_name: displayName} : {display_name: displayName, name: mentionFromName(displayName)});
    const addMember = (user: UserProfile) => set({member_ids: [...draft.member_ids, user.id]});
    const removeMember = (userId: string) => set({member_ids: draft.member_ids.filter((id) => id !== userId), lead_id: draft.lead_id === userId ? '' : draft.lead_id});

    const failed = (result: unknown) => {
        const err = (result as {error?: {message?: string}})?.error;
        if (err) {
            setError(err.message || formatMessage({id: 'fusion.teams.saveFailed', defaultMessage: 'The team couldn\'t be saved.'}));
            return true;
        }
        return false;
    };

    const save = async () => {
        setSaving(true);
        setError('');
        const fields = {
            display_name: draft.display_name.trim(),
            name: draft.name.trim(),
            tag: draft.tag.trim().toUpperCase(),
            lead_id: draft.lead_id,
            channel_id: draft.channel_id,
            description: draft.description.trim(),
        };
        try {
            if (!team) {
                const result = await dispatch(createGroupWithUserIds({
                    ...fields,
                    source: 'custom',
                    allow_reference: true,
                    team_id: communityId,
                    is_team: true,
                    user_ids: draft.member_ids,
                }));
                if (failed(result)) {
                    return;
                }
            } else {
                const added = draft.member_ids.filter((id) => !saved.member_ids.includes(id));
                const removed = saved.member_ids.filter((id) => !draft.member_ids.includes(id));
                if (added.length && failed(await dispatch(addUsersToGroup(team.id, added)))) {
                    return;
                }
                const patch: GroupTeamPatch = Object.fromEntries(Object.entries(fields).filter(([key, value]) => value !== saved[key as keyof TeamDraft]));
                if (Object.keys(patch).length && failed(await dispatch(patchGroup(team.id, patch)))) {
                    return;
                }
                if (removed.length && failed(await dispatch(removeUsersFromGroup(team.id, removed)))) {
                    return;
                }
            }

            // A patched group comes back without its members: load the teams again.
            dispatch(loadTeams());
            onDone();
        } finally {
            setSaving(false);
        }
    };

    const valid = Boolean(draft.display_name.trim()) && (/^[a-z0-9]([a-z0-9._-]*[a-z0-9])?$/).test(draft.name.trim());
    return (
        <div className={am('role-editor', 'team-editor')}>
            <div className={am('field-row')}>
                <label className={am('field')}>
                    <span>{formatMessage({id: 'fusion.teams.name', defaultMessage: 'Name'})}</span>
                    <input
                        type='text'
                        value={draft.display_name}
                        maxLength={128}
                        placeholder={formatMessage({id: 'fusion.teams.namePlaceholder', defaultMessage: 'e.g. Detector team'})}
                        onChange={(e) => setName(e.target.value)}
                    />
                </label>
                <label className={am('field')}>
                    <span>{formatMessage({id: 'fusion.teams.mention', defaultMessage: 'Mention'})}</span>
                    <input
                        type='text'
                        value={draft.name}
                        maxLength={64}
                        spellCheck={false}
                        onChange={(e) => {
                            setMentionEdited(true);
                            set({name: e.target.value.toLowerCase()});
                        }}
                    />
                    <small>{formatMessage({id: 'fusion.teams.mentionHelp', defaultMessage: '@{name} notifies its members'}, {name: draft.name || '…'})}</small>
                </label>
            </div>
            <div className={am('field-row')}>
                <label className={am('field', 'team-tag-field')}>
                    <span>{formatMessage({id: 'fusion.teams.badge', defaultMessage: 'Badge'})}</span>
                    <input
                        type='text'
                        value={draft.tag}
                        maxLength={TEAM_TAG_MAX}
                        placeholder={formatMessage({id: 'fusion.teams.tagPlaceholder', defaultMessage: 'e.g. DET'})}
                        onChange={(e) => set({tag: e.target.value.toUpperCase()})}
                    />
                    <small>{formatMessage({id: 'fusion.teams.tagHelp', defaultMessage: 'Up to {max} characters'}, {max: TEAM_TAG_MAX})}</small>
                </label>
                <label className={am('field')}>
                    <span>{formatMessage({id: 'fusion.teams.leadLabel', defaultMessage: 'Lead'})}</span>
                    <select
                        value={draft.lead_id}
                        onChange={(e) => set({lead_id: e.target.value})}
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
                    value={draft.channel_id}
                    onChange={(e) => set({channel_id: e.target.value})}
                >
                    <option value=''>{formatMessage({id: 'fusion.teams.noChannelOption', defaultMessage: 'No team channel'})}</option>
                    {draft.channel_id && !channels.some((c) => c.id === draft.channel_id) && (
                        <option value={draft.channel_id}>{formatMessage({id: 'fusion.teams.otherChannel', defaultMessage: 'The current channel (you aren\'t in it)'})}</option>
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
                    rows={2}
                    value={draft.description}
                    maxLength={1024}
                    onChange={(e) => set({description: e.target.value})}
                />
            </label>
            <div className={am('field', 'role-members')}>
                <span>{formatMessage({id: 'fusion.teams.membersTitle', defaultMessage: 'Members'})}</span>
                <PeopleSearch
                    teamId={communityId}
                    exclude={draft.member_ids}
                    placeholder={formatMessage({id: 'fusion.teams.addPeople', defaultMessage: 'Add people of the community…'})}
                    onPick={addMember}
                />
                <div className={am('role-people')}>
                    {draft.member_ids.map((id) => (
                        <span
                            key={id}
                            className={am('role-person')}
                        >
                            <Avatar
                                userId={id}
                                size='sm'
                            />
                            <PersonName userId={id}/>
                            <button
                                type='button'
                                aria-label={formatMessage({id: 'fusion.teams.removePerson', defaultMessage: 'Remove from the team'})}
                                onClick={() => removeMember(id)}
                            >
                                <Icon
                                    name='x'
                                    size='xs'
                                />
                            </button>
                        </span>
                    ))}
                </div>
            </div>
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
                    onClick={onCancel}
                >
                    {formatMessage({id: 'fusion.teams.cancel', defaultMessage: 'Cancel'})}
                </button>
                <button
                    type='button'
                    className={am('btn', 'primary')}
                    disabled={saving || !valid || (Boolean(team) && !changed)}
                    onClick={save}
                >
                    {team ? formatMessage({id: 'fusion.teams.save', defaultMessage: 'Save'}) : formatMessage({id: 'fusion.teams.create', defaultMessage: 'Create team'})}
                </button>
            </div>
        </div>
    );
}
