// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useMemo, useRef, useState} from 'react';
import {defineMessage, useIntl} from 'react-intl';
import type {MessageDescriptor} from 'react-intl';
import {shallowEqual, useSelector} from 'react-redux';

import type {ChannelOverride} from '@mattermost/types/channel_overrides';
import type {Channel} from '@mattermost/types/channels';

import {Client4} from 'mattermost-redux/client';
import {Permissions} from 'mattermost-redux/constants';
import {getTeammateNameDisplaySetting} from 'mattermost-redux/selectors/entities/preferences';
import {getUser} from 'mattermost-redux/selectors/entities/users';
import {displayUsername} from 'mattermost-redux/utils/user_utils';

import {Dialog} from 'fusion/components/layer';
import {useToast} from 'fusion/shell/toast_context';
import {getTeamRoles} from 'fusion/teams/roles';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

// What an override can change in a channel, each line covering the permissions it stands for.
const LINES: Array<{key: string; label: MessageDescriptor; permissions: string[]}> = [
    {key: 'post', label: defineMessage({id: 'fusion.overrides.post', defaultMessage: 'Send messages'}), permissions: [Permissions.CREATE_POST]},
    {key: 'react', label: defineMessage({id: 'fusion.overrides.react', defaultMessage: 'Add reactions'}), permissions: [Permissions.ADD_REACTION, Permissions.REMOVE_REACTION]},
    {key: 'files', label: defineMessage({id: 'fusion.overrides.files', defaultMessage: 'Attach files'}), permissions: [Permissions.UPLOAD_FILE]},
    {key: 'channelMentions', label: defineMessage({id: 'fusion.overrides.channelMentions', defaultMessage: 'Mention @channel, @all and @here'}), permissions: [Permissions.USE_CHANNEL_MENTIONS]},
    {key: 'groupMentions', label: defineMessage({id: 'fusion.overrides.groupMentions', defaultMessage: 'Mention groups and roles'}), permissions: [Permissions.USE_GROUP_MENTIONS]},
    {key: 'messages', label: defineMessage({id: 'fusion.overrides.messages', defaultMessage: 'Manage messages'}), permissions: [Permissions.DELETE_OTHERS_POSTS, Permissions.EDIT_OTHERS_POSTS]},
    {key: 'members', label: defineMessage({id: 'fusion.overrides.members', defaultMessage: 'Manage members'}), permissions: [Permissions.MANAGE_PUBLIC_CHANNEL_MEMBERS, Permissions.MANAGE_PRIVATE_CHANNEL_MEMBERS]},
    {key: 'settings', label: defineMessage({id: 'fusion.overrides.settings', defaultMessage: 'Manage the channel'}), permissions: [Permissions.MANAGE_PUBLIC_CHANNEL_PROPERTIES, Permissions.MANAGE_PRIVATE_CHANNEL_PROPERTIES]},
];

type State = 'deny' | 'inherit' | 'allow';

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

// ChannelPermissionsDialog is a channel's permissions, as Discord's: who can see it (for a private channel, possibly
// some roles only), and what roles or people may do here on top of, or less than, what they have elsewhere.
export default function ChannelPermissionsDialog({channel, onClose}: {channel: Channel; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const toast = useToast();
    const roles = useSelector((state: GlobalState) => getTeamRoles(state, channel.team_id));
    const [overrides, setOverrides] = useState<ChannelOverride[]>([]);
    const [access, setAccess] = useState<string[]>([]);
    const [loaded, setLoaded] = useState(false);
    const [loadFailed, setLoadFailed] = useState(false);
    const [adding, setAdding] = useState('');
    const isPrivate = channel.type === 'P';

    useEffect(() => {
        Promise.all([Client4.getChannelOverrides(channel.id), isPrivate ? Client4.getChannelRoleAccess(channel.id) : Promise.resolve({group_ids: []})]).
            then(([o, a]) => {
                setOverrides(o);
                setAccess(a.group_ids);
                setLoaded(true);
            }).
            catch(() => {
                setLoadFailed(true);
                setLoaded(true);
            });
    }, [channel.id, isPrivate]);

    // Nothing is edited over a list that couldn't be loaded: saving would overwrite what's there.
    const editable = loaded && !loadFailed;

    const fail = (error: unknown) => toast((error as {message?: string}).message || formatMessage({id: 'fusion.overrides.failed', defaultMessage: 'The permissions couldn\'t be saved.'}));

    // Edits apply at once and are saved one after the other, each from the latest list, so that quick clicks don't
    // undo each other; a failed save is undone.
    const latest = useRef<ChannelOverride[]>([]);
    const queue = useRef<Promise<unknown>>(Promise.resolve());
    const show = (list: ChannelOverride[]) => {
        latest.current = list;
        setOverrides(list);
    };
    useEffect(() => {
        latest.current = overrides;
    }, [overrides]);

    // put shows an override in place of the one before; an empty one goes, unless it's kept (a row being undone).
    const put = (next: ChannelOverride, keep = false) => {
        const at = latest.current.findIndex((o) => o.target_type === next.target_type && o.target_id === next.target_id);
        const others = latest.current.filter((_, i) => i !== at);
        if (!keep && !next.allow.length && !next.deny.length) {
            show(others);
        } else {
            show(at < 0 ? [...others, next] : [...others.slice(0, at), next, ...others.slice(at)]);
        }
    };

    const changeOverride = (target: ChannelOverride, change: (o: ChannelOverride) => ChannelOverride) => {
        queue.current = queue.current.then(async () => {
            const known = latest.current.find((o) => o.target_type === target.target_type && o.target_id === target.target_id) || target;
            const next = change(known);
            put(next);
            try {
                await Client4.saveChannelOverride(channel.id, {target_type: next.target_type, target_id: next.target_id, allow: next.allow, deny: next.deny});
            } catch (error) {
                put(known, true);
                fail(error);
            }
        });
    };

    const accessQueue = useRef<Promise<unknown>>(Promise.resolve());
    const accessLatest = useRef<string[]>([]);
    accessLatest.current = access;
    const toggleAccess = (groupId: string) => {
        accessQueue.current = accessQueue.current.then(async () => {
            const current = accessLatest.current;
            const next = current.includes(groupId) ? current.filter((id) => id !== groupId) : [...current, groupId];
            accessLatest.current = next;
            setAccess(next);
            try {
                const result = await Client4.setChannelRoleAccess(channel.id, next);
                accessLatest.current = result.group_ids;
                setAccess(result.group_ids);
            } catch (error) {
                accessLatest.current = current;
                setAccess(current);
                fail(error);
            }
        });
    };

    const listedRoles = new Set(overrides.filter((o) => o.target_type === 'role').map((o) => o.target_id));
    const rows = useMemo(() => [...overrides].sort((a, b) => a.target_type.localeCompare(b.target_type)), [overrides]);

    return (
        <Dialog
            label={formatMessage({id: 'fusion.overrides.title', defaultMessage: 'Permissions of #{channel}'}, {channel: channel.display_name})}
            onClose={onClose}
            small={false}
            className={am('wide')}
        >
            <div className={am('pane', 'roles-pane')}>
                <h2>{formatMessage({id: 'fusion.overrides.title', defaultMessage: 'Permissions of #{channel}'}, {channel: channel.display_name})}</h2>
                <p className={am('lead')}>{formatMessage({id: 'fusion.overrides.lead', defaultMessage: 'Here, roles and people can be allowed more, or less, than they have elsewhere. A person\'s own setting wins over their roles\'; the team\'s admins aren\'t affected.'})}</p>
                {loadFailed && (
                    <p
                        className={am('lead')}
                        role='alert'
                    >
                        {formatMessage({id: 'fusion.overrides.loadFailed', defaultMessage: 'The channel\'s permissions couldn\'t be loaded. Close this and try again.'})}
                    </p>
                )}
                <h3 className={am('section-title')}>{formatMessage({id: 'fusion.overrides.whoSees', defaultMessage: 'Who can see it'})}</h3>
                {isPrivate ? (
                    <div className={am('role-access')}>
                        <p className={am('lead')}>{access.length ? formatMessage({id: 'fusion.overrides.rolesOnly', defaultMessage: 'Only people with these roles: they join it, and others leave.'}) : formatMessage({id: 'fusion.overrides.invited', defaultMessage: 'People invited. Choose roles to keep it for them only.'})}</p>
                        {roles.map((role) => (
                            <label
                                key={role.id}
                                className={am('check-row')}
                            >
                                <input
                                    type='checkbox'
                                    checked={access.includes(role.id)}
                                    disabled={!editable}
                                    onChange={() => toggleAccess(role.id)}
                                />
                                <i
                                    className={am('role-dot')}
                                    style={{background: role.color || 'var(--am-muted)'}}
                                />
                                {role.display_name}
                            </label>
                        ))}
                        {!roles.length && <p className={am('lead')}>{formatMessage({id: 'fusion.overrides.noRoles', defaultMessage: 'The team has no roles yet.'})}</p>}
                    </div>
                ) : (
                    <p className={am('lead')}>{formatMessage({id: 'fusion.overrides.public', defaultMessage: 'Everyone in the team: it\'s public. Make it private to keep it for some roles.'})}</p>
                )}
                <h3 className={am('section-title')}>{formatMessage({id: 'fusion.overrides.what', defaultMessage: 'What they can do here'})}</h3>
                {rows.map((o) => (
                    <OverrideRow
                        key={o.target_type + o.target_id}
                        override={o}
                        disabled={!editable}
                        onChange={(change) => changeOverride(o, change)}
                    />
                ))}
                <div className={am('field')}>
                    <select
                        value={adding}
                        disabled={!editable}
                        aria-label={formatMessage({id: 'fusion.overrides.addRole', defaultMessage: 'Add a role'})}
                        onChange={(e) => {
                            const id = e.target.value;
                            setAdding('');
                            if (id) {
                                show([...latest.current, {channel_id: channel.id, target_type: 'role', target_id: id, allow: [], deny: [], update_at: 0}]);
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
                </div>
                <div className={am('modal-actions')}>
                    <button
                        type='button'
                        className={am('btn', 'primary')}
                        onClick={onClose}
                    >
                        {formatMessage({id: 'fusion.overrides.done', defaultMessage: 'Done'})}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}

function OverrideRow({override, disabled, onChange}: {override: ChannelOverride; disabled: boolean; onChange: (change: (o: ChannelOverride) => ChannelOverride) => void}) {
    const {formatMessage} = useIntl();
    const role = useSelector((state: GlobalState) => (override.target_type === 'role' ? state.entities.groups.groups[override.target_id] : undefined));
    const name = useSelector((state: GlobalState) => (override.target_type === 'user' ? displayUsername(getUser(state, override.target_id), getTeammateNameDisplaySetting(state)) : ''), shallowEqual);
    const states: Array<[State, string, string]> = [
        ['deny', '✕', formatMessage({id: 'fusion.overrides.deny', defaultMessage: 'Denied here'})],
        ['inherit', '/', formatMessage({id: 'fusion.overrides.inherit', defaultMessage: 'As elsewhere'})],
        ['allow', '✓', formatMessage({id: 'fusion.overrides.allow', defaultMessage: 'Allowed here'})],
    ];
    return (
        <div className={am('override-row')}>
            <div className={am('override-head')}>
                {role && (
                    <i
                        className={am('role-dot')}
                        style={{background: role.color || 'var(--am-muted)'}}
                    />
                )}
                <b style={role?.color ? {color: role.color} : undefined}>{role ? role.display_name : name || override.target_id}</b>
                <button
                    type='button'
                    className={am('btn', 'small')}
                    disabled={disabled}
                    onClick={() => onChange((o) => ({...o, allow: [], deny: []}))}
                >
                    {formatMessage({id: 'fusion.overrides.remove', defaultMessage: 'Remove'})}
                </button>
            </div>
            {LINES.map((line) => {
                const current = lineState(override, line.permissions);
                return (
                    <div
                        key={line.key}
                        className={am('override-line')}
                    >
                        <span>{formatMessage(line.label)}</span>
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
                                    className={am(state, {on: current === state})}
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
