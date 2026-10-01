// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';
import type {UserProfile} from '@mattermost/types/users';

import {addChannelMembers} from 'mattermost-redux/actions/channels';
import {getProfilesNotInChannel, searchProfiles} from 'mattermost-redux/actions/users';
import {Permissions} from 'mattermost-redux/constants';
import {getTeammateNameDisplaySetting} from 'mattermost-redux/selectors/entities/preferences';
import {haveITeamPermission} from 'mattermost-redux/selectors/entities/roles';
import {displayUsername} from 'mattermost-redux/utils/user_utils';

import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import {Dialog} from 'fusion/components/layer';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

const PAGE = 50;

type Props = {
    channel: Channel;
    onInvite: () => void;
    onClose: () => void;
};

// AddPeopleDialog adds members of the team to a channel: the mockup's "Add people" dialog (search, a list of people
// to tick, "Invite someone new" for people who aren't in the team yet).
export default function AddPeopleDialog({channel, onInvite, onClose}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const nameDisplay = useSelector(getTeammateNameDisplaySetting);
    const canInvite = useSelector((state: GlobalState) => haveITeamPermission(state, channel.team_id, Permissions.ADD_USER_TO_TEAM) || haveITeamPermission(state, channel.team_id, Permissions.INVITE_GUEST));
    const [query, setQuery] = useState('');
    const [people, setPeople] = useState<UserProfile[] | null>(null);
    const [picked, setPicked] = useState<Map<string, UserProfile>>(new Map());
    const [saving, setSaving] = useState(false);
    const [error, setError] = useState('');
    const inputRef = useRef<HTMLInputElement>(null);

    useEffect(() => {
        inputRef.current?.focus();
    }, []);

    // The team's people outside the channel, searched on the server as you type.
    useEffect(() => {
        let cancelled = false;
        const term = query.trim().replace(/^@/, '');
        const timer = setTimeout(async () => {
            const result = term ? await dispatch(searchProfiles(term, {not_in_channel_id: channel.id, team_id: channel.team_id, group_constrained: channel.group_constrained, allow_inactive: false})) : await dispatch(getProfilesNotInChannel(channel.team_id, channel.id, Boolean(channel.group_constrained), 0, PAGE));
            if (!cancelled && result && 'data' in result && result.data) {
                setPeople(result.data.filter((u) => !u.delete_at));
            }
        }, term ? 250 : 0);
        return () => {
            cancelled = true;
            clearTimeout(timer);
        };
    }, [query, channel, dispatch]);

    const where = '#' + channel.display_name;
    const toggle = (user: UserProfile) => {
        setPicked((p) => {
            const next = new Map(p);
            if (next.has(user.id)) {
                next.delete(user.id);
            } else {
                next.set(user.id, user);
            }
            return next;
        });
    };

    const add = async () => {
        if (!picked.size || saving) {
            return;
        }
        setSaving(true);
        const result = await dispatch(addChannelMembers(channel.id, [...picked.keys()]));
        setSaving(false);
        if (result && 'error' in result && result.error) {
            setError((result.error as {message?: string}).message || formatMessage({id: 'fusion.addPeople.failed', defaultMessage: 'Could not add them. Try again.'}));
            return;
        }
        const names = [...picked.values()].map((u) => displayUsername(u, nameDisplay));
        toast(formatMessage({id: 'fusion.toast.added', defaultMessage: 'Added {names} to {where}'}, {names: names.join(', '), where}));
        onClose();
    };

    let list;
    if (people === null) {
        list = <p className={am('pick-empty')}>{formatMessage({id: 'fusion.addPeople.loading', defaultMessage: 'Loading…'})}</p>;
    } else if (people.length === 0) {
        list = (
            <p className={am('pick-empty')}>
                {query.trim() ? formatMessage({id: 'fusion.addPeople.noMatch', defaultMessage: 'No one matches “{query}”.'}, {query: query.trim()}) : formatMessage({id: 'fusion.addPeople.everyone', defaultMessage: 'Everyone in the team is already here.'})}
                {canInvite && ' ' + formatMessage({id: 'fusion.addPeople.useInvite', defaultMessage: 'Use “Invite someone new” to bring people in.'})}
            </p>
        );
    } else {
        list = people.map((u) => (
            <button
                key={u.id}
                type='button'
                className={am('pick-row')}
                role='checkbox'
                aria-checked={picked.has(u.id)}
                onClick={() => toggle(u)}
            >
                <Avatar
                    userId={u.id}
                    status={true}
                />
                <span className={am('who')}>
                    <b>{displayUsername(u, nameDisplay)}</b>
                    <span>{[`@${u.username}`, u.position].filter(Boolean).join(' · ')}</span>
                </span>
                <span className={am('box')}>
                    <Icon
                        name='check'
                        size='xs'
                    />
                </span>
            </button>
        ));
    }

    const title = formatMessage({id: 'fusion.addPeople.title', defaultMessage: 'Add people to {where}'}, {where});
    return (
        <Dialog
            label={title}
            onClose={onClose}
            onSubmit={add}
        >
            <div className={am('pane')}>
                <h2>{title}</h2>
                <p className={am('lead')}>
                    {channel.type === 'P' ? formatMessage({id: 'fusion.addPeople.leadPrivate', defaultMessage: "They'll see the channel history and get notified. It's a private channel: only members can see it."}) : formatMessage({id: 'fusion.addPeople.lead', defaultMessage: "They'll see the channel history and get notified."})}
                </p>
                <label className={am('dm-find', 'add-find')}>
                    <Icon
                        name='search'
                        size='sm'
                    />
                    <input
                        ref={inputRef}
                        value={query}
                        placeholder={formatMessage({id: 'fusion.addPeople.search', defaultMessage: 'Search by name or @username'})}
                        aria-label={formatMessage({id: 'fusion.addPeople.searchLabel', defaultMessage: 'Search people'})}
                        autoComplete='off'
                        onChange={(e) => setQuery(e.target.value)}
                    />
                </label>
                <div
                    className={am('pick-list')}
                    role='group'
                    aria-label={formatMessage({id: 'fusion.addPeople.people', defaultMessage: 'People'})}
                >
                    {list}
                </div>
                {error && (
                    <p
                        className={am('pick-empty')}
                        role='alert'
                        style={{color: 'var(--am-danger)'}}
                    >
                        {error}
                    </p>
                )}
                <div className={am('modal-actions')}>
                    {canInvite && (
                        <button
                            type='button'
                            className={am('btn')}
                            onClick={onInvite}
                        >
                            <Icon
                                name='link'
                                size='sm'
                            />
                            {formatMessage({id: 'fusion.addPeople.invite', defaultMessage: 'Invite someone new'})}
                        </button>
                    )}
                    <span className={am('grow')}/>
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={onClose}
                    >
                        {formatMessage({id: 'fusion.addPeople.cancel', defaultMessage: 'Cancel'})}
                    </button>
                    <button
                        className={am('btn', 'primary')}
                        disabled={!picked.size || saving}
                    >
                        {picked.size ? formatMessage({id: 'fusion.addPeople.addCount', defaultMessage: 'Add {count}'}, {count: picked.size}) : formatMessage({id: 'fusion.addPeople.add', defaultMessage: 'Add'})}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}
