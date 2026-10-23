// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useSelector} from 'react-redux';

import type {Group} from '@mattermost/types/groups';

import {createSelector} from 'mattermost-redux/selectors/create_selector';
import {getCurrentTeam, getCurrentTeamId, getTeam} from 'mattermost-redux/selectors/entities/teams';

import type {GlobalState} from 'types/store';

// Antimatter roles, as Discord's: groups of one team, with a colour for their members' names, an icon, a place among
// the team's roles (0 = highest) and, for some, their own group in the member list. A person's highest role with a
// colour colours their name, unless the team hides role colours.

const NO_ROLES: Group[] = [];

// isRole tells a role from the community's teams (groups of one team too, marked is_team).
export function isRole(group: Group): boolean {
    return Boolean(group.team_id) && !group.is_team;
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
            if (group.source === 'custom' && group.delete_at === 0 && isRole(group)) {
                (byTeam[group.team_id!] ||= []).push(group);
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

// A community's default role, which everyone in it has (but guests and bots), as Discord's @everyone: what members with
// no other role look like and may do. Nobody gives it nor takes it away, and it isn't listed among someone's roles.
export function isDefaultRole(state: GlobalState, role: Group): boolean {
    return Boolean(role.team_id) && getTeam(state, role.team_id!)?.default_role_id === role.id;
}

// getGivenTeamRoles gives a team's roles but its default role: those given to some of its members, highest first.
export const getGivenTeamRoles = createSelector(
    'fusionGetGivenTeamRoles',
    (state: GlobalState, teamId: string) => getTeamRoles(state, teamId),
    (state: GlobalState, teamId: string) => getTeam(state, teamId)?.default_role_id,
    (roles, defaultRoleId): Group[] => (defaultRoleId && roles.some((r) => r.id === defaultRoleId) ? roles.filter((r) => r.id !== defaultRoleId) : roles),
);

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

// getGivenMemberRolesForCurrentTeam gives each person's roles in the current team, but its default role.
const getGivenMemberRolesForCurrentTeam = createSelector(
    'fusionGetGivenMemberRolesForCurrentTeam',
    getMemberRolesForCurrentTeam,
    (state: GlobalState) => getCurrentTeam(state)?.default_role_id,
    (byUser, defaultRoleId): Record<string, Group[]> => {
        if (!defaultRoleId) {
            return byUser;
        }
        const given: Record<string, Group[]> = {};
        for (const [userId, roles] of Object.entries(byUser)) {
            const others = roles.filter((r) => r.id !== defaultRoleId);
            if (others.length) {
                given[userId] = others;
            }
        }
        return given;
    },
);

// useMemberRoles gives someone's roles in the current team, highest first, but its default role, which everyone has.
export function useMemberRoles(userId: string): Group[] {
    return useSelector((state: GlobalState) => getGivenMemberRolesForCurrentTeam(state)[userId] || NO_ROLES);
}

export function getGivenRolesOf(state: GlobalState, userId: string): Group[] {
    return getGivenMemberRolesForCurrentTeam(state)[userId] || NO_ROLES;
}

// hoistedRole is the highest of someone's roles shown apart in the member list.
export function hoistedRole(roles: Group[] | undefined): Group | undefined {
    return roles?.find((r) => r.hoist);
}
