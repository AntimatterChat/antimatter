// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useSelector} from 'react-redux';

import type {Group} from '@mattermost/types/groups';

import {createSelector} from 'mattermost-redux/selectors/create_selector';
import {getCurrentTeam, getCurrentTeamId} from 'mattermost-redux/selectors/entities/teams';

import type {GlobalState} from 'types/store';

// Antimatter roles, as Discord's: groups of one team, with a colour for their members' names, an icon, a place among
// the team's roles (0 = highest) and, for some, their own group in the member list. A person's highest role with a
// colour colours their name, unless the team hides role colours.

const NO_ROLES: Group[] = [];

export function isRole(group: Group): boolean {
    return Boolean(group.team_id);
}

export function byPosition(a: Group, b: Group): number {
    return ((a.position ?? 0) - (b.position ?? 0)) || a.display_name.localeCompare(b.display_name);
}

// getRolesByTeam gives each team's roles, highest first.
export const getRolesByTeam = createSelector(
    'fusionGetRolesByTeam',
    (state: GlobalState) => state.entities.groups.groups,
    (groups): Record<string, Group[]> => {
        const byTeam: Record<string, Group[]> = {};
        for (const group of Object.values(groups)) {
            if (group.source === 'custom' && group.delete_at === 0 && group.team_id) {
                (byTeam[group.team_id] ||= []).push(group);
            }
        }
        for (const roles of Object.values(byTeam)) {
            roles.sort(byPosition);
        }
        return byTeam;
    },
);

export function getTeamRoles(state: GlobalState, teamId: string): Group[] {
    return getRolesByTeam(state)[teamId] || NO_ROLES;
}

// getMemberRolesForCurrentTeam gives each person's roles in the current team, highest first.
export const getMemberRolesForCurrentTeam = createSelector(
    'fusionGetMemberRolesForCurrentTeam',
    (state: GlobalState) => getTeamRoles(state, getCurrentTeamId(state)),
    (roles): Record<string, Group[]> => {
        const byUser: Record<string, Group[]> = {};
        for (const role of roles) {
            for (const userId of role.member_ids || []) {
                (byUser[userId] ||= []).push(role);
            }
        }
        return byUser;
    },
);

export function roleColorsShown(state: GlobalState): boolean {
    return !getCurrentTeam(state)?.hide_role_colors;
}

// getRoleColor is the colour of someone's highest coloured role in the current team, when the team shows role colours.
export function getRoleColor(state: GlobalState, userId: string): string | undefined {
    if (!roleColorsShown(state)) {
        return undefined;
    }
    return (getMemberRolesForCurrentTeam(state)[userId] || NO_ROLES).find((r) => r.color)?.color;
}

export function useRoleColor(userId: string | undefined): string | undefined {
    return useSelector((state: GlobalState) => (userId ? getRoleColor(state, userId) : undefined));
}

export function useMemberRoles(userId: string): Group[] {
    return useSelector((state: GlobalState) => getMemberRolesForCurrentTeam(state)[userId] || NO_ROLES);
}

// hoistedRole is the highest of someone's roles shown apart in the member list.
export function hoistedRole(roles: Group[] | undefined): Group | undefined {
    return roles?.find((r) => r.hoist);
}
