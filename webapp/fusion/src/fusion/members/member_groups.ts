// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useIntl} from 'react-intl';
import {shallowEqual, useSelector} from 'react-redux';

import type {UserProfile} from '@mattermost/types/users';

import {getChannelMembersInChannels} from 'mattermost-redux/selectors/entities/channels';
import {getCurrentTeamId, getMembersInTeams} from 'mattermost-redux/selectors/entities/teams';
import {getStatusForUserId} from 'mattermost-redux/selectors/entities/users';
import {isSystemAdmin} from 'mattermost-redux/utils/user_utils';

import type {GlobalState} from 'types/store';

// The Mattermost roles the member list hoists, as the mockup hoists its display roles.
export type MemberRole = 'admin' | 'mod' | 'bot' | '';

export type MemberGroup = {
    key: string;
    label: string;

    // The colour swatch of a role group (a --am-role-* token).
    color?: string;
    users: UserProfile[];
    off: boolean;
};

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
// online moderators, then everyone else online, then everyone offline.
export function useMemberGroups(users: UserProfile[], roles: Record<string, MemberRole>): MemberGroup[] {
    const {formatMessage} = useIntl();
    const statuses = useSelector((state: GlobalState) => Object.fromEntries(users.map((u) => [u.id, getStatusForUserId(state, u.id)])), shallowEqual);
    const isOnline = (u: UserProfile) => Boolean(statuses[u.id]) && statuses[u.id] !== 'offline';

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
