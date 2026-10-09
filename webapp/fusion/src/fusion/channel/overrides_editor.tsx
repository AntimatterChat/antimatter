// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useMemo, useRef, useState} from 'react';
import {defineMessage, useIntl} from 'react-intl';
import type {MessageDescriptor} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {ChannelOverride} from '@mattermost/types/channel_overrides';

import {getMissingProfilesByIds} from 'mattermost-redux/actions/users';
import {Permissions} from 'mattermost-redux/constants';

import Avatar from 'fusion/components/avatar';
import PeopleSearch, {PersonName} from 'fusion/members/people_search';
import {useToast} from 'fusion/shell/toast_context';
import {getTeamRoles} from 'fusion/teams/roles';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

// An override as edited: for a role or a person, the permissions allowed and denied (a channel's, or a category's).
export type Override = Pick<ChannelOverride, 'target_type' | 'target_id' | 'allow' | 'deny'>;

// What an override can change, each line covering the permissions it stands for.
export const LINES: Array<{key: string; label: MessageDescriptor; permissions: string[]}> = [
    {key: 'post', label: defineMessage({id: 'fusion.overrides.post', defaultMessage: 'Send messages'}), permissions: [Permissions.CREATE_POST]},
    {key: 'react', label: defineMessage({id: 'fusion.overrides.react', defaultMessage: 'Add reactions'}), permissions: [Permissions.ADD_REACTION, Permissions.REMOVE_REACTION]},
    {key: 'files', label: defineMessage({id: 'fusion.overrides.files', defaultMessage: 'Attach files'}), permissions: [Permissions.UPLOAD_FILE]},
    {key: 'channelMentions', label: defineMessage({id: 'fusion.overrides.channelMentions', defaultMessage: 'Mention @channel, @all and @here'}), permissions: [Permissions.USE_CHANNEL_MENTIONS]},
    {key: 'groupMentions', label: defineMessage({id: 'fusion.overrides.groupMentions', defaultMessage: 'Mention groups and roles'}), permissions: [Permissions.USE_GROUP_MENTIONS]},
    {key: 'messages', label: defineMessage({id: 'fusion.overrides.messages', defaultMessage: 'Manage messages'}), permissions: [Permissions.DELETE_OTHERS_POSTS, Permissions.EDIT_OTHERS_POSTS]},
    {key: 'members', label: defineMessage({id: 'fusion.overrides.members', defaultMessage: 'Manage members'}), permissions: [Permissions.MANAGE_PUBLIC_CHANNEL_MEMBERS, Permissions.MANAGE_PRIVATE_CHANNEL_MEMBERS]},
    {key: 'settings', label: defineMessage({id: 'fusion.overrides.settings', defaultMessage: 'Manage the channel'}), permissions: [Permissions.MANAGE_PUBLIC_CHANNEL_PROPERTIES, Permissions.MANAGE_PRIVATE_CHANNEL_PROPERTIES]},
];

export type State = 'deny' | 'inherit' | 'allow';

export function lineState(override: Pick<ChannelOverride, 'allow' | 'deny'>, permissions: string[]): State {
    if (permissions.every((p) => override.deny.includes(p))) {
        return 'deny';
    }
    if (permissions.every((p) => override.allow.includes(p))) {
        return 'allow';
    }
    return 'inherit';
}

export function setLine<T extends Pick<ChannelOverride, 'allow' | 'deny'>>(override: T, permissions: string[], state: State): T {
    const allow = override.allow.filter((p) => !permissions.includes(p));
    const deny = override.deny.filter((p) => !permissions.includes(p));
    return {...override, allow: state === 'allow' ? [...allow, ...permissions] : allow, deny: state === 'deny' ? [...deny, ...permissions] : deny};
}

const sameTarget = (a: Override, b: Override) => a.target_type === b.target_type && a.target_id === b.target_id;

// useOverrideList keeps a list of overrides as they're edited: loaded once, each change shown at once and saved after
// the ones before, from the latest list, so that quick clicks don't undo each other; a failed save is undone. Nothing
// is edited over a list that couldn't be loaded: saving would overwrite what's there.
export function useOverrideList(load: () => Promise<Override[]>, save: (o: Override) => Promise<unknown>, deps: unknown[]) {
    const {formatMessage} = useIntl();
    const toast = useToast();
    const [overrides, setOverrides] = useState<Override[]>([]);
    const [loaded, setLoaded] = useState(false);
    const [loadFailed, setLoadFailed] = useState(false);
    const latest = useRef<Override[]>([]);
    const queue = useRef<Promise<unknown>>(Promise.resolve());

    const show = (list: Override[]) => {
        latest.current = list;
        setOverrides(list);
    };

    useEffect(() => {
        let gone = false;
        load().then((list) => {
            if (!gone) {
                show(list);
                setLoaded(true);
            }
        }).catch(() => {
            if (!gone) {
                setLoadFailed(true);
                setLoaded(true);
            }
        });
        return () => {
            gone = true;
        };
    }, deps); // eslint-disable-line react-hooks/exhaustive-deps

    // put shows an override in place of the one before; an empty one goes, unless it's kept (a row being undone).
    const put = (next: Override, keep = false) => {
        const at = latest.current.findIndex((o) => sameTarget(o, next));
        const others = latest.current.filter((_, i) => i !== at);
        if (!keep && !next.allow.length && !next.deny.length) {
            show(others);
        } else {
            show(at < 0 ? [...others, next] : [...others.slice(0, at), next, ...others.slice(at)]);
        }
    };

    const change = (target: Override, edit: (o: Override) => Override) => {
        queue.current = queue.current.then(async () => {
            const known = latest.current.find((o) => sameTarget(o, target)) || {...target, allow: [], deny: []};
            const next = edit(known);
            put(next);
            try {
                await save(next);
            } catch (error) {
                put(known, true);
                toast((error as {message?: string}).message || formatMessage({id: 'fusion.overrides.failed', defaultMessage: 'The permissions couldn\'t be saved.'}));
            }
        });
    };

    // A role or person added shows with nothing set yet: their first change saves them.
    const add = (target: Override) => {
        if (!latest.current.some((o) => sameTarget(o, target))) {
            show([...latest.current, target]);
        }
    };

    return {overrides, loaded, loadFailed, editable: loaded && !loadFailed, change, add};
}

type EditorProps = {
    teamId: string;
    overrides: Override[];

    // What a category gives the channel (the channel's own winning, line by line); shown beside each line.
    inherited?: Override[];
    editable: boolean;
    onChange: (target: Override, edit: (o: Override) => Override) => void;
    onAdd: (target: Override) => void;
};

// OverridesEditor lists the roles and people with an override, each line denied, as elsewhere, or allowed, and adds
// roles and people.
export function OverridesEditor({teamId, overrides, inherited = [], editable, onChange, onAdd}: EditorProps) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const roles = useSelector((state: GlobalState) => getTeamRoles(state, teamId));

    // The rows: the overrides set, and those only inherited, roles first.
    const rows = useMemo(() => {
        const list = [...overrides];
        for (const o of inherited) {
            if (!list.some((r) => sameTarget(r, o))) {
                list.push({target_type: o.target_type, target_id: o.target_id, allow: [], deny: []});
            }
        }
        return list.sort((a, b) => a.target_type.localeCompare(b.target_type));
    }, [overrides, inherited]);
    const listedRoles = new Set(rows.filter((o) => o.target_type === 'role').map((o) => o.target_id));
    const people = useMemo(() => rows.filter((o) => o.target_type === 'user').map((o) => o.target_id), [rows]);

    useEffect(() => {
        if (people.length) {
            dispatch(getMissingProfilesByIds(people));
        }
    }, [people, dispatch]);

    return (
        <>
            {rows.map((o) => (
                <OverrideRow
                    key={o.target_type + o.target_id}
                    override={o}
                    inherited={inherited.find((i) => sameTarget(i, o))}
                    disabled={!editable}
                    onChange={(edit) => onChange(o, edit)}
                />
            ))}
            <div className={am('override-add')}>
                <select
                    value=''
                    disabled={!editable}
                    aria-label={formatMessage({id: 'fusion.overrides.addRole', defaultMessage: 'Add a role'})}
                    onChange={(e) => {
                        if (e.target.value) {
                            onAdd({target_type: 'role', target_id: e.target.value, allow: [], deny: []});
                        }
                    }}
                >
                    <option value=''>{formatMessage({id: 'fusion.overrides.addRole', defaultMessage: 'Add a role'})}</option>
                    {roles.filter((r) => !listedRoles.has(r.id)).map((r) => (
                        <option
                            key={r.id}
                            value={r.id}
                        >
                            {r.display_name}
                        </option>
                    ))}
                </select>
                <div className={am('field', 'role-members')}>
                    <PeopleSearch
                        teamId={teamId}
                        exclude={people}
                        disabled={!editable}
                        placeholder={formatMessage({id: 'fusion.overrides.addPerson', defaultMessage: 'Add a person…'})}
                        onPick={(user) => onAdd({target_type: 'user', target_id: user.id, allow: [], deny: []})}
                    />
                </div>
            </div>
        </>
    );
}

function OverrideRow({override, inherited, disabled, onChange}: {override: Override; inherited?: Override; disabled: boolean; onChange: (edit: (o: Override) => Override) => void}) {
    const {formatMessage} = useIntl();
    const role = useSelector((state: GlobalState) => (override.target_type === 'role' ? state.entities.groups.groups[override.target_id] : undefined));
    const states: Array<[State, string, string]> = [
        ['deny', '✕', formatMessage({id: 'fusion.overrides.deny', defaultMessage: 'Denied here'})],
        ['inherit', '/', formatMessage({id: 'fusion.overrides.inherit', defaultMessage: 'As elsewhere'})],
        ['allow', '✓', formatMessage({id: 'fusion.overrides.allow', defaultMessage: 'Allowed here'})],
    ];
    const set = override.allow.length > 0 || override.deny.length > 0;
    return (
        <div className={am('override-row')}>
            <div className={am('override-head')}>
                {role && (
                    <i
                        className={am('role-dot')}
                        style={{background: role.color || 'var(--am-muted)'}}
                    />
                )}
                {override.target_type === 'user' && (
                    <Avatar
                        userId={override.target_id}
                        size='sm'
                    />
                )}
                <b style={role?.color ? {color: role.color} : undefined}>
                    {role ? role.display_name : <PersonName userId={override.target_id}/>}
                </b>
                {inherited && (
                    <small className={am('override-from')}>
                        {set ? formatMessage({id: 'fusion.overrides.alsoCategory', defaultMessage: 'also set on the category'}) : formatMessage({id: 'fusion.overrides.fromCategory', defaultMessage: 'from the category'})}
                    </small>
                )}
                {set && (
                    <button
                        type='button'
                        className={am('btn', 'small')}
                        disabled={disabled}
                        onClick={() => onChange((o) => ({...o, allow: [], deny: []}))}
                    >
                        {inherited ? formatMessage({id: 'fusion.overrides.reset', defaultMessage: 'As the category'}) : formatMessage({id: 'fusion.overrides.remove', defaultMessage: 'Remove'})}
                    </button>
                )}
            </div>
            {LINES.map((line) => {
                const current = lineState(override, line.permissions);
                const fromCategory = inherited ? lineState(inherited, line.permissions) : 'inherit';
                return (
                    <div
                        key={line.key}
                        className={am('override-line')}
                    >
                        <span>
                            {formatMessage(line.label)}
                            {current === 'inherit' && fromCategory !== 'inherit' && (
                                <small className={am('override-from')}>
                                    {fromCategory === 'deny' ? formatMessage({id: 'fusion.overrides.categoryDenies', defaultMessage: 'denied by the category'}) : formatMessage({id: 'fusion.overrides.categoryAllows', defaultMessage: 'allowed by the category'})}
                                </small>
                            )}
                        </span>
                        <div
                            className={am('tri')}
                            role='radiogroup'
                            aria-label={formatMessage(line.label)}
                        >
                            {states.map(([state, glyph, label]) => (
                                <button
                                    key={state}
                                    type='button'
                                    role='radio'
                                    aria-checked={current === state}
                                    title={label}
                                    aria-label={label}
                                    className={am(state, {on: current === state, from: current === 'inherit' && state === fromCategory && state !== 'inherit'})}
                                    disabled={disabled}
                                    onClick={() => onChange((o) => setLine(o, line.permissions, state))}
                                >
                                    {glyph}
                                </button>
                            ))}
                        </div>
                    </div>
                );
            })}
        </div>
    );
}
