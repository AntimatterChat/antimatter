// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useEffect} from 'react';
import {useDispatch, useSelector} from 'react-redux';

import type {Group} from '@mattermost/types/groups';
import type {UserProfile} from '@mattermost/types/users';

import {getGroups} from 'mattermost-redux/actions/groups';
import {createSelector} from 'mattermost-redux/selectors/create_selector';
import {getConfig} from 'mattermost-redux/selectors/entities/general';

import type {GlobalState} from 'types/store';

// Antimatter teams, as in the mockup: Mattermost's custom user groups, with a short tag shown next to their members'
// names, a lead and a team channel. Each person picks the team whose tag they show, in their profile; it's kept in their
// user props, so everyone sees it.
export const TEAM_BADGE_PROP = 'team_badge';

const NO_TEAMS: Group[] = [];

// getTeams lists the teams (the custom groups not archived), by name.
export const getTeams = createSelector(
    'fusionGetTeams',
    (state: GlobalState) => state.entities.groups.groups,
    (groups): Group[] => {
        const teams = Object.values(groups).filter((g) => g.source === 'custom' && g.delete_at === 0);
        return teams.length ? teams.sort((a, b) => a.display_name.localeCompare(b.display_name)) : NO_TEAMS;
    },
);

// getTeamsByUser gives the teams of each person, by name.
export const getTeamsByUser = createSelector(
    'fusionGetTeamsByUser',
    getTeams,
    (teams): Record<string, Group[]> => {
        const byUser: Record<string, Group[]> = {};
        for (const team of teams) {
            for (const userId of team.member_ids || []) {
                (byUser[userId] ||= []).push(team);
            }
        }
        return byUser;
    },
);

// badgeTeam is the team whose tag someone shows, if they chose one, still belong to it, and it has a tag.
export function badgeTeam(user: UserProfile | undefined, teamsByUser: Record<string, Group[]>): Group | undefined {
    const chosen = user?.props?.[TEAM_BADGE_PROP];
    if (!user || !chosen) {
        return undefined;
    }
    return (teamsByUser[user.id] || []).find((t) => t.id === chosen && t.tag);
}

// mainTeam is the team someone is listed under in the member list: the one whose tag they show, else their first.
export function mainTeam(user: UserProfile, teamsByUser: Record<string, Group[]>): Group | undefined {
    const teams = teamsByUser[user.id] || [];
    return teams.find((t) => t.id === user.props?.[TEAM_BADGE_PROP]) || teams[0];
}

export function useBadgeTeam(user: UserProfile | undefined): Group | undefined {
    return useSelector((state: GlobalState) => badgeTeam(user, getTeamsByUser(state)));
}

// useLoadTeams loads the teams and their members, once for the app and again when asked (after an edit).
export function loadTeams() {
    return getGroups({group_source: 'custom', include_member_ids: true, per_page: 200} as Parameters<typeof getGroups>[0]);
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
