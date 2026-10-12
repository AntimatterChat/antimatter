// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useEffect} from 'react';
import {useDispatch, useSelector} from 'react-redux';

import type {Group} from '@mattermost/types/groups';
import type {UserProfile} from '@mattermost/types/users';

import {getGroups} from 'mattermost-redux/actions/groups';
import {createSelector} from 'mattermost-redux/selectors/create_selector';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {getCurrentTeamId, getTeam} from 'mattermost-redux/selectors/entities/teams';

import type {ActionFuncAsync, GlobalState} from 'types/store';

// Antimatter teams, as in the mockup: custom user groups of one community (a Mattermost team), marked as teams, with a
// short tag shown next to their members' names, a lead and a team channel. In each community, each person picks the
// team whose tag they show, in their profile; it's kept in their user props, so everyone sees it.
export const TEAM_BADGE_PROP = 'team_badge';

const NO_TEAMS: Group[] = [];
const NO_TEAMS_BY_USER: Record<string, Group[]> = {};

// isTeam tells a team from a role (the other groups of one community) and from Mattermost's own custom groups.
export function isTeam(group: Group): boolean {
    return group.source === 'custom' && Boolean(group.is_team) && Boolean(group.team_id);
}

// getTeamsByCommunity gives each community's teams (not archived), by name.
export const getTeamsByCommunity = createSelector(
    'fusionGetTeamsByCommunity',
    (state: GlobalState) => state.entities.groups.groups,
    (groups): Record<string, Group[]> => {
        const byCommunity: Record<string, Group[]> = {};
        for (const group of Object.values(groups)) {
            if (isTeam(group) && group.delete_at === 0) {
                (byCommunity[group.team_id!] ||= []).push(group);
            }
        }
        for (const teams of Object.values(byCommunity)) {
            teams.sort((a, b) => a.display_name.localeCompare(b.display_name));
        }
        return byCommunity;
    },
);

export function getCommunityTeams(state: GlobalState, communityId: string): Group[] {
    return getTeamsByCommunity(state)[communityId] || NO_TEAMS;
}

// getTeams lists the teams of the current community, by name.
export function getTeams(state: GlobalState): Group[] {
    return getCommunityTeams(state, getCurrentTeamId(state));
}

// getTeamsByUserByCommunity gives, in each community, the teams of each person, by name.
const getTeamsByUserByCommunity = createSelector(
    'fusionGetTeamsByUserByCommunity',
    getTeamsByCommunity,
    (byCommunity): Record<string, Record<string, Group[]>> => {
        const result: Record<string, Record<string, Group[]>> = {};
        for (const [communityId, teams] of Object.entries(byCommunity)) {
            const byUser: Record<string, Group[]> = {};
            for (const team of teams) {
                for (const userId of team.member_ids || []) {
                    (byUser[userId] ||= []).push(team);
                }
            }
            result[communityId] = byUser;
        }
        return result;
    },
);

// getTeamsByUserIn gives the teams of each person in a community, by name.
export function getTeamsByUserIn(state: GlobalState, communityId: string): Record<string, Group[]> {
    return getTeamsByUserByCommunity(state)[communityId] || NO_TEAMS_BY_USER;
}

// getTeamsByUser gives the teams of each person in the current community, by name.
export function getTeamsByUser(state: GlobalState): Record<string, Group[]> {
    return getTeamsByUserIn(state, getCurrentTeamId(state));
}

// The badge prop keeps one choice per community, as a JSON object of community id to team id. It used to keep a single
// team id, from when teams belonged to the whole server: that choice still counts in that team's community.
function parseBadgeChoices(value: string): Record<string, string> | undefined {
    if (!value.startsWith('{')) {
        return undefined;
    }
    try {
        const parsed = JSON.parse(value);
        return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {};
    } catch {
        return {};
    }
}

// badgeChoice is the team someone chose to show in a community, if any. An old single choice is returned as is: it only
// matches a team of its own community.
export function badgeChoice(user: UserProfile | undefined, communityId: string): string | undefined {
    const value = user?.props?.[TEAM_BADGE_PROP];
    if (!value) {
        return undefined;
    }
    const choices = parseBadgeChoices(value);
    if (!choices) {
        return value;
    }
    const chosen = choices[communityId];
    return typeof chosen === 'string' && chosen ? chosen : undefined;
}

// withBadgeChoice is the badge prop once someone picks a team ('' for none) in a community. An old single choice moves
// to the community of its team, which communityOf gives (it's dropped if that team is gone).
export function withBadgeChoice(value: string | undefined, communityId: string, teamId: string, communityOf: (teamId: string) => string | undefined): string {
    let choices = value ? parseBadgeChoices(value) : {};
    if (!choices) {
        const legacyCommunity = communityOf(value!);
        choices = legacyCommunity ? {[legacyCommunity]: value!} : {};
    }
    const next = {...choices};
    if (teamId) {
        next[communityId] = teamId;
    } else {
        delete next[communityId];
    }
    return Object.keys(next).length ? JSON.stringify(next) : '';
}

// badgeTeam is the team whose tag someone shows in a community, if they chose one, still belong to it, and it has a
// tag. teamsByUser are the community's teams. Where badges are required (a community setting, for example in a
// company), someone who chose none shows their first team's tag.
export function badgeTeam(user: UserProfile | undefined, teamsByUser: Record<string, Group[]>, communityId: string, required = false): Group | undefined {
    if (!user) {
        return undefined;
    }
    const tagged = (teamsByUser[user.id] || []).filter((t) => t.tag);
    const chosen = badgeChoice(user, communityId);
    return tagged.find((t) => t.id === chosen) || (required ? tagged[0] : undefined);
}

export function badgesRequiredIn(state: GlobalState, communityId: string): boolean {
    return Boolean(getTeam(state, communityId)?.require_team_badges);
}

export function badgesRequired(state: GlobalState): boolean {
    return badgesRequiredIn(state, getCurrentTeamId(state));
}

// mainTeam is the team someone is listed under in a community's member list: the one whose tag they show, else their
// first.
export function mainTeam(user: UserProfile, teamsByUser: Record<string, Group[]>, communityId: string): Group | undefined {
    const teams = teamsByUser[user.id] || [];
    const chosen = badgeChoice(user, communityId);
    return teams.find((t) => t.id === chosen) || teams[0];
}

// useBadgeTeam is the team whose tag someone shows in a community: the current one by default (for a message, its
// conversation's community).
export function useBadgeTeam(user: UserProfile | undefined, communityId?: string): Group | undefined {
    return useSelector((state: GlobalState) => {
        const community = communityId || getCurrentTeamId(state);
        return badgeTeam(user, getTeamsByUserIn(state, community), community, badgesRequiredIn(state, community));
    });
}

const TEAMS_PAGE = 200;

// loadTeams loads the teams and roles (custom groups) of every community and their members, page after page.
export function loadTeams(): ActionFuncAsync<boolean> {
    return async (dispatch) => {
        for (let page = 0; ; page++) {
            // eslint-disable-next-line no-await-in-loop
            const result = await dispatch(getGroups({group_source: 'custom', include_member_ids: true, page, per_page: TEAMS_PAGE} as Parameters<typeof getGroups>[0]));
            if ('error' in result && result.error) {
                return {error: result.error};
            }
            if (!('data' in result) || !Array.isArray(result.data) || result.data.length < TEAMS_PAGE) {
                return {data: true};
            }
        }
    };
}

export function useLoadTeams() {
    const dispatch = useDispatch();
    const enabled = useSelector((state: GlobalState) => getConfig(state).EnableCustomGroups !== 'false');
    useEffect(() => {
        if (enabled) {
            dispatch(loadTeams());
        }
    }, [enabled, dispatch]);
}

// Whether the admins let the member list group people by role (TeamSettings.MemberListRoleGroups).
export function useRoleGroupsAllowed(): boolean {
    return useSelector((state: GlobalState) => getConfig(state).MemberListRoleGroups !== 'false');
}
