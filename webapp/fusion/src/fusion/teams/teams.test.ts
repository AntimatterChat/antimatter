// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Group} from '@mattermost/types/groups';
import type {UserProfile} from '@mattermost/types/users';

import type {GlobalState} from 'types/store';

import {
    badgeChoice,
    badgeTeam,
    badgesRequiredIn,
    getCommunityTeams,
    getTeams,
    getTeamsByUser,
    getTeamsByUserIn,
    isTeam,
    mainTeam,
    TEAM_BADGE_PROP,
    withBadgeChoice,
} from './teams';

const team = (id: string, tag = '') => ({id, tag, display_name: id}) as Group;
const user = (id: string, badge?: string) => ({id, props: badge === undefined ? {} : {[TEAM_BADGE_PROP]: badge}}) as unknown as UserProfile;
const badges = (choices: Record<string, string>) => JSON.stringify(choices);

describe('teams', () => {
    const det = team('det', 'DET');
    const ops = team('ops', 'OPS');
    const untagged = team('beam');
    const teamsByUser = {ada: [det, ops], ivo: [untagged], sam: [det]};

    test('the badge is the team someone chose in the community, if they belong to it and it has a tag', () => {
        expect(badgeTeam(user('ada', badges({labs: 'ops'})), teamsByUser, 'labs')).toBe(ops);
        expect(badgeTeam(user('sam', badges({labs: 'ops'})), teamsByUser, 'labs')).toBeUndefined();
        expect(badgeTeam(user('ivo', badges({labs: 'beam'})), teamsByUser, 'labs')).toBeUndefined();
    });

    test('each community keeps its own choice', () => {
        const ada = user('ada', badges({labs: 'ops', community: 'det'}));
        expect(badgeTeam(ada, teamsByUser, 'labs')).toBe(ops);
        expect(badgeTeam(ada, teamsByUser, 'community')).toBe(det);
        expect(badgeTeam(ada, teamsByUser, 'elsewhere')).toBeUndefined();
    });

    test('an old single choice still counts, for its own team only', () => {
        expect(badgeChoice(user('ada', 'ops'), 'labs')).toBe('ops');
        expect(badgeTeam(user('ada', 'ops'), teamsByUser, 'labs')).toBe(ops);
        expect(badgeTeam(user('ada', 'ops'), {ada: [det]}, 'community')).toBeUndefined();
    });

    test('no choice, no badge', () => {
        expect(badgeTeam(user('ada'), teamsByUser, 'labs')).toBeUndefined();
        expect(badgeTeam(user('ada', ''), teamsByUser, 'labs')).toBeUndefined();
        expect(badgeTeam(user('ada', '{broken'), teamsByUser, 'labs')).toBeUndefined();
        expect(badgeTeam(user('ada', badges({})), teamsByUser, 'labs')).toBeUndefined();
    });

    test('where badges are required, someone who chose none shows their first tagged team', () => {
        expect(badgeTeam(user('ada'), teamsByUser, 'labs', true)).toBe(det);
        expect(badgeTeam(user('ada', badges({labs: 'ops'})), teamsByUser, 'labs', true)).toBe(ops);
        expect(badgeTeam(user('ada', badges({community: 'ops'})), teamsByUser, 'labs', true)).toBe(det);
        expect(badgeTeam(user('ivo'), teamsByUser, 'labs', true)).toBeUndefined();
    });

    test('people are listed under their badge team, else their first team', () => {
        expect(mainTeam(user('ada', badges({labs: 'ops'})), teamsByUser, 'labs')).toBe(ops);
        expect(mainTeam(user('ada', 'ops'), teamsByUser, 'labs')).toBe(ops);
        expect(mainTeam(user('ada'), teamsByUser, 'labs')).toBe(det);
        expect(mainTeam(user('ivo'), teamsByUser, 'labs')).toBe(untagged);
        expect(mainTeam(user('lena'), teamsByUser, 'labs')).toBeUndefined();
    });

    describe('withBadgeChoice', () => {
        const communityOf = (id: string) => ({ops: 'labs', mod: 'community'} as Record<string, string>)[id];

        test('sets and clears the choice of one community, leaving the others', () => {
            expect(JSON.parse(withBadgeChoice(undefined, 'labs', 'ops', communityOf))).toEqual({labs: 'ops'});
            expect(JSON.parse(withBadgeChoice(badges({labs: 'ops'}), 'community', 'mod', communityOf))).toEqual({labs: 'ops', community: 'mod'});
            expect(JSON.parse(withBadgeChoice(badges({labs: 'ops', community: 'mod'}), 'labs', '', communityOf))).toEqual({community: 'mod'});
            expect(withBadgeChoice(badges({labs: 'ops'}), 'labs', '', communityOf)).toBe('');
        });

        test('moves an old single choice to its team\'s community', () => {
            expect(JSON.parse(withBadgeChoice('ops', 'community', 'mod', communityOf))).toEqual({labs: 'ops', community: 'mod'});
            expect(JSON.parse(withBadgeChoice('ops', 'labs', 'det', communityOf))).toEqual({labs: 'det'});
            expect(JSON.parse(withBadgeChoice('gone', 'labs', 'det', communityOf))).toEqual({labs: 'det'});
        });
    });
});

describe('teams of a community', () => {
    const group = (id: string, fields: Partial<Group>) => ({id, display_name: id, source: 'custom', delete_at: 0, team_id: 'labs', is_team: true, member_ids: [], ...fields}) as Group;
    const groups = [
        group('ops', {tag: 'OPS', member_ids: ['ada', 'bob']}),
        group('det', {tag: 'DET', member_ids: ['ada']}),
        group('mod', {team_id: 'community', member_ids: ['ada']}),
        group('mods-role', {is_team: false, member_ids: ['ada']}),
        group('archived', {delete_at: 1, member_ids: ['ada']}),
        group('plain', {team_id: '', is_team: false, member_ids: ['ada']}),
        group('ldap', {source: 'ldap', member_ids: ['ada']}),
    ];
    const state = {
        entities: {
            groups: {groups: Object.fromEntries(groups.map((g) => [g.id, g]))},
            teams: {currentTeamId: 'labs', teams: {labs: {id: 'labs', require_team_badges: true}, community: {id: 'community'}}},
        },
    } as unknown as GlobalState;

    test('a team is a custom group of one community marked as a team', () => {
        expect(isTeam(groups[0])).toBe(true);
        expect(isTeam(groups[3])).toBe(false);
        expect(isTeam(groups[5])).toBe(false);
        expect(isTeam(groups[6])).toBe(false);
    });

    test('a community has its live teams, by name', () => {
        expect(getTeams(state).map((t) => t.id)).toEqual(['det', 'ops']);
        expect(getCommunityTeams(state, 'community').map((t) => t.id)).toEqual(['mod']);
        expect(getCommunityTeams(state, 'none')).toEqual([]);
    });

    test('each person\'s teams are those of the community', () => {
        expect(getTeamsByUser(state).ada.map((t) => t.id)).toEqual(['det', 'ops']);
        expect(getTeamsByUser(state).bob.map((t) => t.id)).toEqual(['ops']);
        expect(getTeamsByUserIn(state, 'community').ada.map((t) => t.id)).toEqual(['mod']);
        expect(getTeamsByUserIn(state, 'none')).toEqual({});
        expect(getTeamsByUser(state)).toBe(getTeamsByUser(state));
    });

    test('badges are required per community', () => {
        expect(badgesRequiredIn(state, 'labs')).toBe(true);
        expect(badgesRequiredIn(state, 'community')).toBe(false);
    });
});
