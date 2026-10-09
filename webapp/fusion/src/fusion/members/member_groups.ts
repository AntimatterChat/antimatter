// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useMemo} from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useSelector} from 'react-redux';

import type {Group} from '@mattermost/types/groups';
import type {UserProfile} from '@mattermost/types/users';

import {getChannelMembersInChannels} from 'mattermost-redux/selectors/entities/channels';
import {getCurrentTeamId, getMembersInTeams} from 'mattermost-redux/selectors/entities/teams';
import {getStatusForUserId} from 'mattermost-redux/selectors/entities/users';
import {isSystemAdmin} from 'mattermost-redux/utils/user_utils';

import {getMemberRolesForCurrentTeam, getTeamRoles, hoistedRole, roleColorsShown} from 'fusion/teams/roles';
import {getTeamsByUser, mainTeam} from 'fusion/teams/teams';

import type {GlobalState} from 'types/store';

// The Mattermost roles the member list hoists, as the mockup hoists its display roles.
export type MemberRole = 'admin' | 'mod' | 'bot' | '';

export type MemberGroup = {
    key: string;
    label: string;

    // The colour swatch of a role group (a --am-role-* token).
    color?: string;

    // The tag of a team group.
    tag?: string;
    users: UserProfile[];

    // Whether the whole group is offline; a team group dims its offline members one by one instead.
    off: boolean;
    offline?: ReadonlySet<string>;
};

// How the member list groups people: by role (when the admins allow it), or by team.
export type Grouping = 'roles' | 'teams';

export const ROLE_COLORS: Record<string, string> = {
    admin: 'var(--am-role-admin)',
    mod: 'var(--am-role-mod)',
    bot: 'var(--am-role-bot)',
};

// useMemberRoles gives each member's highest role: system admin, else channel or team admin ("moderator"), else bot.
export function useMemberRoles(users: UserProfile[], channelId: string): Record<string, MemberRole> {
    return useSelector((state: GlobalState) => {
        const channelMembers = getChannelMembersInChannels(state)[channelId] || {};
        const teamMembers = getMembersInTeams(state)[getCurrentTeamId(state)] || {};
        const roles: Record<string, MemberRole> = {};
        for (const u of users) {
            if (isSystemAdmin(u.roles || '')) {
                roles[u.id] = 'admin';
            } else if (channelMembers[u.id]?.scheme_admin || teamMembers[u.id]?.scheme_admin) {
                roles[u.id] = 'mod';
            } else if (u.is_bot) {
                roles[u.id] = 'bot';
            } else {
                roles[u.id] = '';
            }
        }
        return roles;
    }, shallowEqual);
}

// useMemberGroups splits a conversation's members as the mockup's member list does: the online admins, then the
// online moderators, then everyone else online, then everyone offline. By team, each team lists its members, those
// online first, under the team someone shows the tag of (else their first team); then the others, online and offline.
export function useMemberGroups(users: UserProfile[], roles: Record<string, MemberRole>, grouping: Grouping = 'roles'): MemberGroup[] {
    const {formatMessage} = useIntl();
    const statuses = useSelector((state: GlobalState) => Object.fromEntries(users.map((u) => [u.id, getStatusForUserId(state, u.id)])), shallowEqual);
    const teamsByUser = useSelector(getTeamsByUser);
    const memberRoles = useSelector(getMemberRolesForCurrentTeam);
    const teamRoles = useSelector((state: GlobalState) => getTeamRoles(state, getCurrentTeamId(state)));
    const hoisted = useMemo(() => teamRoles.filter((r) => r.hoist), [teamRoles]);
    const colorsShown = useSelector(roleColorsShown);
    const isOnline = (u: UserProfile) => Boolean(statuses[u.id]) && statuses[u.id] !== 'offline';

    if (grouping === 'teams') {
        const byTeam = new Map<string, {team: Group; users: UserProfile[]}>();
        const others: UserProfile[] = [];
        for (const u of users) {
            const team = mainTeam(u, teamsByUser);
            if (team) {
                const entry = byTeam.get(team.id) || {team, users: []};
                entry.users.push(u);
                byTeam.set(team.id, entry);
            } else {
                others.push(u);
            }
        }
        const offline = new Set(users.filter((u) => !isOnline(u)).map((u) => u.id));
        const teamGroups: MemberGroup[] = [...byTeam.values()].
            sort((a, b) => a.team.display_name.localeCompare(b.team.display_name)).
            map(({team, users: members}) => ({
                key: `team-${team.id}`,
                label: formatMessage({id: 'fusion.members.team', defaultMessage: '{team} — {count}'}, {team: team.display_name, count: members.length}),
                tag: team.tag,
                users: [...members.filter(isOnline), ...members.filter((u) => !isOnline(u))],
                off: false,
                offline,
            }));
        const othersOnline = others.filter(isOnline);
        const othersOffline = others.filter((u) => !isOnline(u));
        const rest: MemberGroup[] = [
            {key: 'online', label: formatMessage({id: 'fusion.members.online', defaultMessage: 'Online — {count}'}, {count: othersOnline.length}), users: othersOnline, off: false},
            {key: 'offline', label: formatMessage({id: 'fusion.members.offline', defaultMessage: 'Offline — {count}'}, {count: othersOffline.length}), users: othersOffline, off: true},
        ];
        return [...teamGroups, ...rest].filter((g) => g.users.length > 0);
    }

    // The team's roles shown apart come first, highest first, each with the online people whose highest such role it is.
    if (hoisted.length) {
        const online = users.filter(isOnline);
        const byRole = new Map<string, UserProfile[]>();
        const rest: UserProfile[] = [];
        for (const u of online) {
            const role = hoistedRole(memberRoles[u.id]);
            if (role) {
                byRole.set(role.id, [...(byRole.get(role.id) || []), u]);
            } else {
                rest.push(u);
            }
        }
        const offline = users.filter((u) => !isOnline(u));
        return [
            ...hoisted.map((role) => ({
                key: `role-${role.id}`,
                label: formatMessage({id: 'fusion.members.role', defaultMessage: '{role} — {count}'}, {role: role.display_name, count: byRole.get(role.id)?.length || 0}),
                color: (colorsShown && role.color) || undefined,
                users: byRole.get(role.id) || [],
                off: false,
            })),
            {key: 'online', label: formatMessage({id: 'fusion.members.online', defaultMessage: 'Online — {count}'}, {count: rest.length}), users: rest, off: false},
            {key: 'offline', label: formatMessage({id: 'fusion.members.offline', defaultMessage: 'Offline — {count}'}, {count: offline.length}), users: offline, off: true},
        ].filter((g) => g.users.length > 0);
    }

    const online = users.filter(isOnline);
    const admins = online.filter((u) => roles[u.id] === 'admin');
    const mods = online.filter((u) => roles[u.id] === 'mod');
    const rest = online.filter((u) => roles[u.id] !== 'admin' && roles[u.id] !== 'mod');
    const offline = users.filter((u) => !isOnline(u));

    const groups: MemberGroup[] = [
        {key: 'admin', label: formatMessage({id: 'fusion.members.admins', defaultMessage: 'Admin — {count}'}, {count: admins.length}), color: ROLE_COLORS.admin, users: admins, off: false},
        {key: 'mod', label: formatMessage({id: 'fusion.members.mods', defaultMessage: 'Moderators — {count}'}, {count: mods.length}), color: ROLE_COLORS.mod, users: mods, off: false},
        {key: 'online', label: formatMessage({id: 'fusion.members.online', defaultMessage: 'Online — {count}'}, {count: rest.length}), users: rest, off: false},
        {key: 'offline', label: formatMessage({id: 'fusion.members.offline', defaultMessage: 'Offline — {count}'}, {count: offline.length}), users: offline, off: true},
    ];
    return groups.filter((g) => g.users.length > 0);
}
