// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useMemo, useState} from 'react';
import {defineMessage, useIntl} from 'react-intl';
import type {MessageDescriptor} from 'react-intl';
import {shallowEqual, useDispatch, useSelector} from 'react-redux';

import type {Group} from '@mattermost/types/groups';
import type {TeamMemberListView} from '@mattermost/types/teams';
import type {UserProfile} from '@mattermost/types/users';

import {addUsersToGroup, archiveGroup, createGroupWithUserIds, patchGroup, removeUsersFromGroup} from 'mattermost-redux/actions/groups';
import {patchTeam} from 'mattermost-redux/actions/teams';
import {getMissingProfilesByIds} from 'mattermost-redux/actions/users';
import {Client4} from 'mattermost-redux/client';
import {Permissions} from 'mattermost-redux/constants';
import {haveITeamPermission} from 'mattermost-redux/selectors/entities/roles';
import {getCurrentTeam, getCurrentTeamId} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import PeopleSearch, {PersonName} from 'fusion/members/people_search';
import {Choice, Toggle} from 'fusion/modals/settings_controls';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import {getMemberRolesForCurrentTeam, getTeamRoles} from './roles';
import {loadTeams} from './teams';

// The colours offered for roles, as Discord's swatches; any #rrggbb colour can be typed too.
const SWATCHES = ['#e74c3c', '#e67e22', '#f1c40f', '#2ecc71', '#1abc9c', '#3498db', '#5865f2', '#9b59b6', '#e91e63', '#95a5a6'];

// permissions is undefined while a role's permissions aren't known (loading, or failed to load); they're only saved
// when a switch changed them.
type Draft = {display_name: string; color: string; icon: string; hoist: boolean; self_assignable: boolean; permissions?: string[]; permissionsChanged?: boolean};

// What a role can grant, as Discord's permission switches: each is on when the role holds all its permissions.
// Administrator holds them all: the server gives a role that may manage the community everything else its admins may
// do (SetTeamRolePermissions), and the other switches show on while it is.
const SWITCHES: Array<{key: string; label: MessageDescriptor; desc: MessageDescriptor; permissions: string[]}> = [
    {key: 'admin', label: defineMessage({id: 'fusion.roles.perm.admin', defaultMessage: 'Administrator'}), desc: defineMessage({id: 'fusion.roles.perm.adminDesc', defaultMessage: 'Everything the community\'s admins can do.'}), permissions: [Permissions.MANAGE_TEAM]},
    {key: 'roles', label: defineMessage({id: 'fusion.roles.perm.roles', defaultMessage: 'Manage roles'}), desc: defineMessage({id: 'fusion.roles.perm.rolesDesc', defaultMessage: 'Make and edit the roles below their own, and give them to people.'}), permissions: [Permissions.MANAGE_TEAM_ROLES]},
    {key: 'invite', label: defineMessage({id: 'fusion.roles.perm.invite', defaultMessage: 'Invite people'}), desc: defineMessage({id: 'fusion.roles.perm.inviteDesc', defaultMessage: 'Add people to the community.'}), permissions: [Permissions.ADD_USER_TO_TEAM]},
    {key: 'kick', label: defineMessage({id: 'fusion.roles.perm.kick', defaultMessage: 'Remove people'}), desc: defineMessage({id: 'fusion.roles.perm.kickDesc', defaultMessage: 'Remove people from the community.'}), permissions: [Permissions.REMOVE_USER_FROM_TEAM]},
    {key: 'public', label: defineMessage({id: 'fusion.roles.perm.public', defaultMessage: 'Create public channels'}), desc: defineMessage({id: 'fusion.roles.perm.publicDesc', defaultMessage: 'Channels anyone in the community can join.'}), permissions: [Permissions.CREATE_PUBLIC_CHANNEL]},
    {key: 'private', label: defineMessage({id: 'fusion.roles.perm.private', defaultMessage: 'Create private channels'}), desc: defineMessage({id: 'fusion.roles.perm.privateDesc', defaultMessage: 'Channels for the people invited.'}), permissions: [Permissions.CREATE_PRIVATE_CHANNEL]},
    {key: 'channels', label: defineMessage({id: 'fusion.roles.perm.channels', defaultMessage: 'Manage channels'}), desc: defineMessage({id: 'fusion.roles.perm.channelsDesc', defaultMessage: 'Rename, convert and archive channels, and manage their members and admins.'}), permissions: [Permissions.MANAGE_PUBLIC_CHANNEL_PROPERTIES, Permissions.MANAGE_PRIVATE_CHANNEL_PROPERTIES, Permissions.DELETE_PUBLIC_CHANNEL, Permissions.DELETE_PRIVATE_CHANNEL, Permissions.MANAGE_PUBLIC_CHANNEL_MEMBERS, Permissions.MANAGE_PRIVATE_CHANNEL_MEMBERS, Permissions.MANAGE_CHANNEL_ROLES, Permissions.CONVERT_PUBLIC_CHANNEL_TO_PRIVATE, Permissions.CONVERT_PRIVATE_CHANNEL_TO_PUBLIC]},
    {key: 'messages', label: defineMessage({id: 'fusion.roles.perm.messages', defaultMessage: 'Manage messages'}), desc: defineMessage({id: 'fusion.roles.perm.messagesDesc', defaultMessage: 'Edit and delete other people\'s messages, and put messages on top of channels.'}), permissions: [Permissions.DELETE_OTHERS_POSTS, Permissions.EDIT_OTHERS_POSTS]},
    {key: 'integrations', label: defineMessage({id: 'fusion.roles.perm.integrations', defaultMessage: 'Manage integrations'}), desc: defineMessage({id: 'fusion.roles.perm.integrationsDesc', defaultMessage: 'Webhooks and slash commands, theirs and other people\'s.'}), permissions: [Permissions.MANAGE_OWN_INCOMING_WEBHOOKS, Permissions.MANAGE_OWN_OUTGOING_WEBHOOKS, Permissions.MANAGE_OWN_SLASH_COMMANDS, Permissions.MANAGE_OTHERS_INCOMING_WEBHOOKS, Permissions.MANAGE_OTHERS_OUTGOING_WEBHOOKS, Permissions.MANAGE_OTHERS_SLASH_COMMANDS]},
];

const slug = (name: string) => name.toLowerCase().normalize('NFKD').replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '').slice(0, 40) || 'role';

// RolesSettings is the Roles tab of the community settings, where a team's admins set up its roles, as Discord's
// server settings: their names, colours, icons, order and members, whether each shows its members apart, and the
// team's display settings (role colours, badges, how the member list groups people). Close closes the settings.
export default function RolesSettings({onClose}: {onClose: () => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const team = useSelector(getCurrentTeam);
    const roles = useSelector((state: GlobalState) => (team ? getTeamRoles(state, team.id) : []), shallowEqual);
    const [editing, setEditing] = useState<string | 'new' | null>(null);

    // Team admins manage every role; those who may manage roles, the roles below their own highest one.
    const admin = useSelector((state: GlobalState) => Boolean(team) && haveITeamPermission(state, team!.id, Permissions.MANAGE_TEAM));

    // Roles anyone can take don't rank, as on the server.
    const myRoles = useSelector((state: GlobalState) => getMemberRolesForCurrentTeam(state)[getCurrentUserId(state)]);
    const managerTop = myRoles?.find((r) => !r.self_assignable)?.position;
    const manages = (role: Group) => admin || (managerTop !== undefined && (role.position ?? 0) > managerTop);
    const nextPosition = roles.reduce((next, r) => Math.max(next, (r.position ?? 0) + 1), 0);

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
        const fields = {display_name: draft.display_name.trim(), color: draft.color, icon: draft.icon.trim(), hoist: draft.hoist, self_assignable: draft.self_assignable};
        const lowest = admin || managerTop === undefined ? 0 : managerTop + 1;
        const result = role ? await dispatch(patchGroup(role.id, fields)) : await dispatch(createGroupWithUserIds({
            ...fields,
            name: `${slug(fields.display_name)}-${team.name}`.slice(0, 64),
            source: 'custom',
            allow_reference: true,
            team_id: team.id,
            position: Math.max(nextPosition, lowest),
            user_ids: [],
        }));
        if (fail(result)) {
            return;
        }
        const saved = (result as {data?: Group}).data;
        if (saved && draft.permissions && draft.permissionsChanged) {
            try {
                await Client4.updateTeamRolePermissions(saved.id, draft.permissions);
            } catch (error) {
                toast((error as {message?: string}).message || formatMessage({id: 'fusion.roles.failed', defaultMessage: 'The role couldn\'t be saved.'}));
                return;
            }
        }
        refresh();
        setEditing(null);
    };

    // Moving a role swaps it with its neighbour. Team admins renumber the roles 0..n in their new order, which mends
    // gaps and ties; others swap the two places (moving the role past a tie).
    const move = async (at: number, by: -1 | 1) => {
        const to = at + by;
        if (to < 0 || to >= roles.length) {
            return;
        }
        const order = [...roles];
        [order[at], order[to]] = [order[to], order[at]];
        const places: Array<[Group, number]> = admin ? order.map((r, i) => [r, i]) : [[roles[at], roles[to].position ?? 0], [roles[to], roles[at].position ?? 0]];
        if (!admin && places[0][1] === places[1][1]) {
            places[0][1] += by;
        }
        const results = await Promise.all(places.filter(([role, position]) => role.position !== position).map(([role, position]) => dispatch(patchGroup(role.id, {position}))));
        results.some(fail);
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
        <div className={am('roles-pane')}>
            <p className={am('lead')}>{formatMessage({id: 'fusion.roles.lead', defaultMessage: 'Roles give members colours. Roles shown separately get their own group in the member list; the member\'s highest role sets their name colour.'})}</p>
            {team && admin && (
                <>
                    <Toggle
                        title={formatMessage({id: 'fusion.roles.colors', defaultMessage: 'Role colours'})}
                        desc={formatMessage({id: 'fusion.roles.colorsDesc', defaultMessage: 'Names take the colour of their highest role, over everyone\'s coloured names.'})}
                        on={!team.hide_role_colors}
                        onChange={(on) => dispatch(patchTeam({id: team.id, hide_role_colors: !on}))}
                    />
                    <Toggle
                        title={formatMessage({id: 'fusion.roles.badges', defaultMessage: 'Everyone shows a team badge'})}
                        desc={formatMessage({id: 'fusion.roles.badgesDesc', defaultMessage: 'For example in a company: in this community, people who chose no badge show the tag of their first team here.'})}
                        on={Boolean(team.require_team_badges)}
                        onChange={(on) => dispatch(patchTeam({id: team.id, require_team_badges: on}))}
                    />
                    <Choice<TeamMemberListView>
                        title={formatMessage({id: 'fusion.roles.memberList', defaultMessage: 'Member list'})}
                        desc={formatMessage({id: 'fusion.roles.memberListDesc', defaultMessage: 'How the member list groups the community\'s members: by their roles, by their teams, or as each member chooses.'})}
                        value={team.member_list_view || 'choice'}
                        options={[
                            ['roles', formatMessage({id: 'fusion.roles.memberList.roles', defaultMessage: 'By roles'})],
                            ['teams', formatMessage({id: 'fusion.roles.memberList.teams', defaultMessage: 'By team'})],
                            ['choice', formatMessage({id: 'fusion.roles.memberList.choice', defaultMessage: 'Let members choose'})],
                        ]}
                        onChange={(view) => dispatch(patchTeam({id: team.id, member_list_view: view}))}
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
                                {role.self_assignable && ` · ${formatMessage({id: 'fusion.roles.openToAll', defaultMessage: 'open to all'})}`}
                                {` · @${role.name}`}
                            </span>
                        </div>
                        <button
                            type='button'
                            className={am('icon-btn')}
                            title={formatMessage({id: 'fusion.roles.up', defaultMessage: 'Move up'})}
                            aria-label={formatMessage({id: 'fusion.roles.up', defaultMessage: 'Move up'})}
                            disabled={i === 0 || !manages(role) || !manages(roles[i - 1])}
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
                            disabled={i === roles.length - 1 || !manages(role)}
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
                            disabled={!manages(role)}
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
                            disabled={!manages(role)}
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
                    {formatMessage({id: 'fusion.teamSettings.closeButton', defaultMessage: 'Close'})}
                </button>
            </div>
        </div>
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
    const [draft, setDraft] = useState<Draft>({display_name: role?.display_name || '', color: role?.color || SWATCHES[5], icon: role?.icon || '', hoist: role?.hoist ?? true, self_assignable: Boolean(role?.self_assignable), permissions: role ? undefined : []});
    const [loadFailed, setLoadFailed] = useState(false);
    const set = (patch: Partial<Draft>) => setDraft((d) => ({...d, ...patch}));

    // What the role grants, as it is now; someone who isn't a team admin only hands out what they have.
    useEffect(() => {
        if (!role) {
            return;
        }
        Client4.getTeamRole(role.id).then((r) => set({permissions: r.permissions || []})).catch(() => setLoadFailed(true));

        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [role?.id]);
    const teamId = useSelector(getCurrentTeamId);
    const held = useSelector((state: GlobalState) => {
        const admin = haveITeamPermission(state, teamId, Permissions.MANAGE_TEAM);
        return Object.fromEntries(SWITCHES.map((sw) => [sw.key, admin || sw.permissions.every((p) => haveITeamPermission(state, teamId, p))]));
    }, shallowEqual);
    const current = draft.permissions || [];
    const administrator = current.includes(Permissions.MANAGE_TEAM);
    const toggle = (permissions: string[], on: boolean) => set({permissions: on ? [...new Set([...current, ...permissions])] : current.filter((p) => !permissions.includes(p)), permissionsChanged: true});
    const loading = !draft.permissions && !loadFailed;

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
            <Toggle
                title={formatMessage({id: 'fusion.roles.selfAssignable', defaultMessage: 'Anyone can take it'})}
                desc={formatMessage({id: 'fusion.roles.selfAssignableDesc', defaultMessage: 'People of the community take or leave it themselves, in "Your roles" or through the onboarding.'})}
                on={draft.self_assignable}
                onChange={(on) => set({self_assignable: on})}
            />
            <div className={am('field', 'role-perms')}>
                <span>{formatMessage({id: 'fusion.roles.permissions', defaultMessage: 'Permissions'})}</span>
                {loadFailed && <p className={am('lead')}>{formatMessage({id: 'fusion.roles.permissionsFailed', defaultMessage: 'The role\'s permissions couldn\'t be loaded: saving leaves them as they are.'})}</p>}
                {SWITCHES.map((sw) => {
                    // Administrator includes the others: they show on, and come back as they were when it's off.
                    const implied = administrator && sw.key !== 'admin';
                    return (
                        <Toggle
                            key={sw.key}
                            title={formatMessage(sw.label)}
                            desc={formatMessage(sw.desc)}
                            on={implied || sw.permissions.every((p) => current.includes(p))}
                            disabled={implied || !held[sw.key] || !draft.permissions}
                            onChange={(on) => toggle(sw.permissions, on)}
                        />
                    );
                })}
            </div>
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
                    disabled={loading || !draft.display_name.trim() || !(/^#[0-9a-fA-F]{6}$/).test(draft.color)}
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
    useEffect(() => {
        dispatch(getMissingProfilesByIds(members));
    }, [members, dispatch]);

    const add = async (user: UserProfile) => {
        await dispatch(addUsersToGroup(role.id, [user.id]));
        onChanged();
    };
    const remove = async (userId: string) => {
        await dispatch(removeUsersFromGroup(role.id, [userId]));
        onChanged();
    };

    return (
        <div className={am('field', 'role-members')}>
            <span>{formatMessage({id: 'fusion.roles.membersTitle', defaultMessage: 'Members'})}</span>
            <PeopleSearch
                teamId={role.team_id || ''}
                exclude={members}
                placeholder={formatMessage({id: 'fusion.roles.addPeople', defaultMessage: 'Add people of the community…'})}
                onPick={add}
            />
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
