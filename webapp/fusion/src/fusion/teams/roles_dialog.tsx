// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useMemo, useState} from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useDispatch, useSelector} from 'react-redux';

import type {Group} from '@mattermost/types/groups';
import type {UserProfile} from '@mattermost/types/users';

import {addUsersToGroup, archiveGroup, createGroupWithUserIds, patchGroup, removeUsersFromGroup} from 'mattermost-redux/actions/groups';
import {patchTeam} from 'mattermost-redux/actions/teams';
import {getMissingProfilesByIds} from 'mattermost-redux/actions/users';
import {Client4} from 'mattermost-redux/client';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {getUser} from 'mattermost-redux/selectors/entities/users';

import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import {Dialog} from 'fusion/components/layer';
import {useDisplayName} from 'fusion/hooks/users';
import {Toggle} from 'fusion/modals/settings_controls';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import {getTeamRoles} from './roles';
import {loadTeams} from './teams';

// The colours offered for roles, as Discord's swatches; any #rrggbb colour can be typed too.
const SWATCHES = ['#e74c3c', '#e67e22', '#f1c40f', '#2ecc71', '#1abc9c', '#3498db', '#5865f2', '#9b59b6', '#e91e63', '#95a5a6'];

type Draft = {display_name: string; color: string; icon: string; hoist: boolean};

const slug = (name: string) => name.toLowerCase().normalize('NFKD').replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '').slice(0, 40) || 'role';

// RolesDialog is where a team's admins set up its roles, as Discord's server settings: their names, colours, icons,
// order and members, whether each shows its members apart, and the team's display settings (role colours, badges).
export default function RolesDialog({onClose}: {onClose: () => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const team = useSelector(getCurrentTeam);
    const roles = useSelector((state: GlobalState) => (team ? getTeamRoles(state, team.id) : []), shallowEqual);
    const [editing, setEditing] = useState<string | 'new' | null>(null);

    const refresh = () => dispatch(loadTeams());
    const fail = (result: unknown) => {
        const error = (result as {error?: {message?: string}}).error;
        if (error) {
            toast(error.message || formatMessage({id: 'fusion.roles.failed', defaultMessage: 'The role couldn\'t be saved.'}));
            return true;
        }
        return false;
    };

    const save = async (draft: Draft, role?: Group) => {
        if (!team) {
            return;
        }
        const fields = {display_name: draft.display_name.trim(), color: draft.color, icon: draft.icon.trim(), hoist: draft.hoist};
        const result = role ? await dispatch(patchGroup(role.id, fields)) : await dispatch(createGroupWithUserIds({
            ...fields,
            name: `${slug(fields.display_name)}-${team.name}`.slice(0, 64),
            source: 'custom',
            allow_reference: true,
            team_id: team.id,
            position: roles.length,
            user_ids: [],
        }));
        if (!fail(result)) {
            refresh();
            setEditing(null);
        }
    };

    // Moving a role swaps its place with its neighbour's; places are written for both, from their order here.
    const move = async (at: number, by: -1 | 1) => {
        const to = at + by;
        if (to < 0 || to >= roles.length) {
            return;
        }
        await dispatch(patchGroup(roles[at].id, {position: to}));
        await dispatch(patchGroup(roles[to].id, {position: at}));
        refresh();
    };

    const remove = async (role: Group) => {
        if (!fail(await dispatch(archiveGroup(role.id)))) {
            toast(formatMessage({id: 'fusion.roles.deleted', defaultMessage: 'Deleted the role {role}'}, {role: role.display_name}));
            refresh();
        }
    };

    const editedRole = roles.find((r) => r.id === editing);
    return (
        <Dialog
            label={formatMessage({id: 'fusion.roles.title', defaultMessage: 'Roles'})}
            onClose={onClose}
            small={false}
        >
            <div className={am('pane', 'roles-pane')}>
                <h2>{formatMessage({id: 'fusion.roles.title', defaultMessage: 'Roles'})}</h2>
                <p className={am('lead')}>{formatMessage({id: 'fusion.roles.lead', defaultMessage: 'Roles give members colours. Roles shown separately get their own group in the member list; the member\'s highest role sets their name colour.'})}</p>
                {team && (
                    <>
                        <Toggle
                            title={formatMessage({id: 'fusion.roles.colors', defaultMessage: 'Role colours'})}
                            desc={formatMessage({id: 'fusion.roles.colorsDesc', defaultMessage: 'Names take the colour of their highest role, over everyone\'s coloured names.'})}
                            on={!team.hide_role_colors}
                            onChange={(on) => dispatch(patchTeam({id: team.id, hide_role_colors: !on}))}
                        />
                        <Toggle
                            title={formatMessage({id: 'fusion.roles.badges', defaultMessage: 'Everyone shows a team badge'})}
                            desc={formatMessage({id: 'fusion.roles.badgesDesc', defaultMessage: 'For example in a company: people who chose no badge show their first team\'s.'})}
                            on={Boolean(team.require_team_badges)}
                            onChange={(on) => dispatch(patchTeam({id: team.id, require_team_badges: on}))}
                        />
                    </>
                )}
                <div className={am('role-list')}>
                    {roles.map((role, i) => (
                        <div
                            key={role.id}
                            className={am('role-row')}
                        >
                            <i
                                className={am('role-dot')}
                                style={{background: role.color || 'var(--am-muted)'}}
                            />
                            <div className={am('grow')}>
                                <b style={role.color ? {color: role.color} : undefined}>{role.display_name}</b>
                                <span>
                                    {formatMessage({id: 'fusion.roles.members', defaultMessage: '{count, plural, one {# member} other {# members}}'}, {count: role.member_ids?.length || 0})}
                                    {role.hoist && ` · ${formatMessage({id: 'fusion.roles.shownApart', defaultMessage: 'shown separately'})}`}
                                    {` · @${role.name}`}
                                </span>
                            </div>
                            <button
                                type='button'
                                className={am('icon-btn')}
                                title={formatMessage({id: 'fusion.roles.up', defaultMessage: 'Move up'})}
                                aria-label={formatMessage({id: 'fusion.roles.up', defaultMessage: 'Move up'})}
                                disabled={i === 0}
                                onClick={() => move(i, -1)}
                            >
                                <Icon
                                    name='chev'
                                    size='sm'
                                    className={am('flip')}
                                />
                            </button>
                            <button
                                type='button'
                                className={am('icon-btn')}
                                title={formatMessage({id: 'fusion.roles.down', defaultMessage: 'Move down'})}
                                aria-label={formatMessage({id: 'fusion.roles.down', defaultMessage: 'Move down'})}
                                disabled={i === roles.length - 1}
                                onClick={() => move(i, 1)}
                            >
                                <Icon
                                    name='chev'
                                    size='sm'
                                />
                            </button>
                            <button
                                type='button'
                                className={am('icon-btn')}
                                title={formatMessage({id: 'fusion.roles.edit', defaultMessage: 'Edit the role'})}
                                aria-label={formatMessage({id: 'fusion.roles.edit', defaultMessage: 'Edit the role'})}
                                onClick={() => setEditing(editing === role.id ? null : role.id)}
                            >
                                <Icon
                                    name='pen'
                                    size='sm'
                                />
                            </button>
                            <button
                                type='button'
                                className={am('icon-btn', 'danger')}
                                title={formatMessage({id: 'fusion.roles.delete', defaultMessage: 'Delete the role'})}
                                aria-label={formatMessage({id: 'fusion.roles.delete', defaultMessage: 'Delete the role'})}
                                onClick={() => remove(role)}
                            >
                                <Icon
                                    name='trash'
                                    size='sm'
                                />
                            </button>
                        </div>
                    ))}
                    {!roles.length && <p className={am('lead')}>{formatMessage({id: 'fusion.roles.none', defaultMessage: 'No roles yet.'})}</p>}
                </div>
                {editing && (
                    <RoleEditor
                        key={editing}
                        role={editedRole}
                        onSave={(draft) => save(draft, editedRole)}
                        onCancel={() => setEditing(null)}
                        onMembersChanged={refresh}
                    />
                )}
                <div className={am('modal-actions')}>
                    {!editing && (
                        <button
                            type='button'
                            className={am('btn')}
                            onClick={() => setEditing('new')}
                        >
                            <Icon
                                name='plus'
                                size='sm'
                            />
                            {formatMessage({id: 'fusion.roles.create', defaultMessage: 'Create role'})}
                        </button>
                    )}
                    <button
                        type='button'
                        className={am('btn', 'primary')}
                        onClick={onClose}
                    >
                        {formatMessage({id: 'fusion.roles.done', defaultMessage: 'Done'})}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}

type EditorProps = {
    role?: Group;
    onSave: (draft: Draft) => void;
    onCancel: () => void;
    onMembersChanged: () => void;
};

// RoleEditor edits a role's look and, once it exists, its members.
function RoleEditor({role, onSave, onCancel, onMembersChanged}: EditorProps) {
    const {formatMessage} = useIntl();
    const [draft, setDraft] = useState<Draft>({display_name: role?.display_name || '', color: role?.color || SWATCHES[5], icon: role?.icon || '', hoist: role?.hoist ?? true});
    const set = (patch: Partial<Draft>) => setDraft({...draft, ...patch});

    return (
        <div className={am('role-editor')}>
            <label className={am('field')}>
                <span>{formatMessage({id: 'fusion.roles.name', defaultMessage: 'Name'})}</span>
                <input
                    type='text'
                    value={draft.display_name}
                    maxLength={64}
                    placeholder={formatMessage({id: 'fusion.roles.namePlaceholder', defaultMessage: 'e.g. Moderators'})}
                    onChange={(e) => set({display_name: e.target.value})}
                />
            </label>
            <div className={am('field')}>
                <span>{formatMessage({id: 'fusion.roles.color', defaultMessage: 'Colour'})}</span>
                <div
                    className={am('swatches')}
                    role='radiogroup'
                    aria-label={formatMessage({id: 'fusion.roles.color', defaultMessage: 'Colour'})}
                >
                    {SWATCHES.map((c) => (
                        <button
                            key={c}
                            type='button'
                            role='radio'
                            aria-checked={draft.color === c}
                            aria-label={c}
                            className={am('swatch', {on: draft.color === c})}
                            style={{background: c}}
                            onClick={() => set({color: c})}
                        />
                    ))}
                    <input
                        type='text'
                        value={draft.color}
                        maxLength={7}
                        aria-label={formatMessage({id: 'fusion.roles.hex', defaultMessage: 'Colour code'})}
                        onChange={(e) => set({color: e.target.value})}
                    />
                </div>
            </div>
            <Toggle
                title={formatMessage({id: 'fusion.roles.hoist', defaultMessage: 'Show members separately'})}
                desc={formatMessage({id: 'fusion.roles.hoistDesc', defaultMessage: 'Online members with this role get their own group in the member list.'})}
                on={draft.hoist}
                onChange={(hoist) => set({hoist})}
            />
            {role && (
                <RoleMembers
                    role={role}
                    onChanged={onMembersChanged}
                />
            )}
            <div className={am('modal-actions')}>
                <button
                    type='button'
                    className={am('btn')}
                    onClick={onCancel}
                >
                    {formatMessage({id: 'fusion.roles.cancel', defaultMessage: 'Cancel'})}
                </button>
                <button
                    type='button'
                    className={am('btn', 'primary')}
                    disabled={!draft.display_name.trim() || !(/^#[0-9a-fA-F]{6}$/).test(draft.color)}
                    onClick={() => onSave(draft)}
                >
                    {role ? formatMessage({id: 'fusion.roles.save', defaultMessage: 'Save'}) : formatMessage({id: 'fusion.roles.createIt', defaultMessage: 'Create role'})}
                </button>
            </div>
        </div>
    );
}

// RoleMembers lists a role's members, with a search to add people of the team.
function RoleMembers({role, onChanged}: {role: Group; onChanged: () => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const members = useMemo(() => role.member_ids || [], [role.member_ids]);
    const [query, setQuery] = useState('');
    const [found, setFound] = useState<UserProfile[]>([]);

    useEffect(() => {
        dispatch(getMissingProfilesByIds(members));
    }, [members, dispatch]);

    useEffect(() => {
        const term = query.trim();
        if (!term) {
            setFound([]);
            return undefined;
        }
        let stale = false;
        const timer = setTimeout(async () => {
            try {
                const result = await Client4.autocompleteUsers(term, role.team_id || '', '', {limit: 8});
                if (!stale) {
                    setFound(result.users.filter((u) => !members.includes(u.id) && !u.delete_at));
                }
            } catch {
                // Nothing found then.
            }
        }, 200);
        return () => {
            stale = true;
            clearTimeout(timer);
        };
    }, [query, role.team_id, members]);

    const add = async (user: UserProfile) => {
        await dispatch(addUsersToGroup(role.id, [user.id]));
        setQuery('');
        onChanged();
    };
    const remove = async (userId: string) => {
        await dispatch(removeUsersFromGroup(role.id, [userId]));
        onChanged();
    };

    return (
        <div className={am('field', 'role-members')}>
            <span>{formatMessage({id: 'fusion.roles.membersTitle', defaultMessage: 'Members'})}</span>
            <input
                type='text'
                value={query}
                placeholder={formatMessage({id: 'fusion.roles.addPeople', defaultMessage: 'Add people of the team…'})}
                onChange={(e) => setQuery(e.target.value)}
            />
            {found.length > 0 && (
                <div className={am('role-found')}>
                    {found.map((u) => (
                        <button
                            key={u.id}
                            type='button'
                            onClick={() => add(u)}
                        >
                            <Avatar
                                userId={u.id}
                                size='sm'
                            />
                            <PersonName userId={u.id}/>
                            <Icon
                                name='plus'
                                size='sm'
                            />
                        </button>
                    ))}
                </div>
            )}
            <div className={am('role-people')}>
                {members.map((id) => (
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
                            aria-label={formatMessage({id: 'fusion.roles.removePerson', defaultMessage: 'Remove from the role'})}
                            onClick={() => remove(id)}
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
    );
}

function PersonName({userId}: {userId: string}) {
    const user = useSelector((state: GlobalState) => getUser(state, userId));
    return <span className={am('nm')}>{useDisplayName(user)}</span>;
}
