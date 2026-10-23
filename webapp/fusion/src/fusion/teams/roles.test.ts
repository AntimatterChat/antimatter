// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Group} from '@mattermost/types/groups';

import type {GlobalState} from 'types/store';

import {byPosition, getGivenRolesOf, getGivenTeamRoles, getMemberRolesForCurrentTeam, getRoleColor, getTeamRoles, hoistedRole, isDefaultRole, isRole, roleColorsShown} from './roles';

const group = (id: string, fields: Partial<Group>) => ({id, display_name: id, source: 'custom', delete_at: 0, team_id: 'team', position: 0, member_ids: [], ...fields}) as Group;

const stateWith = (groups: Group[], hideRoleColors = false) => ({
    entities: {
        groups: {groups: Object.fromEntries(groups.map((g) => [g.id, g]))},
        teams: {currentTeamId: 'team', teams: {team: {id: 'team', hide_role_colors: hideRoleColors}}},
    },
}) as unknown as GlobalState;

describe('roles', () => {
    const mods = group('mods', {position: 0, color: '#e67e22', hoist: true, member_ids: ['ada', 'bob']});
    const gamers = group('gamers', {position: 2, color: '#2ecc71', member_ids: ['bob', 'cy']});
    const plain = group('plain', {position: 1, member_ids: ['cy']});
    const groups = [
        gamers, plain, mods,
        group('elsewhere', {team_id: 'other', member_ids: ['ada']}),
        group('deleted', {delete_at: 1, member_ids: ['ada']}),
        group('ldap', {source: 'ldap', member_ids: ['ada']}),
        group('det', {team_id: '', member_ids: ['ada']}),
        group('crew', {is_team: true, member_ids: ['ada']}),
    ];
    const state = stateWith(groups);

    test('a role is a group of one team', () => {
        expect(isRole(mods)).toBe(true);
        expect(isRole(group('det', {team_id: ''}))).toBe(false);
        expect(isRole(group('crew', {is_team: true}))).toBe(false);
    });

    test('a team has its live custom roles, highest first', () => {
        expect(getTeamRoles(state, 'team').map((r) => r.id)).toEqual(['mods', 'plain', 'gamers']);
        expect(getTeamRoles(state, 'other').map((r) => r.id)).toEqual(['elsewhere']);
        expect(getTeamRoles(state, 'none')).toEqual([]);
    });

    test('the same list comes back while the groups are unchanged', () => {
        expect(getTeamRoles(state, 'none')).toBe(getTeamRoles(state, 'none'));
        expect(getTeamRoles(state, 'team')).toBe(getTeamRoles(state, 'team'));
    });

    test('roles in the same place sort by name', () => {
        expect([group('b', {}), group('a', {})].sort(byPosition).map((r) => r.id)).toEqual(['a', 'b']);
    });

    test('each member has their roles in the current team, highest first', () => {
        const byUser = getMemberRolesForCurrentTeam(state);
        expect(byUser.ada.map((r) => r.id)).toEqual(['mods']);
        expect(byUser.bob.map((r) => r.id)).toEqual(['mods', 'gamers']);
        expect(byUser.cy.map((r) => r.id)).toEqual(['plain', 'gamers']);
    });

    test('a name takes the colour of its highest coloured role', () => {
        expect(getRoleColor(state, 'bob')).toBe('#e67e22');
        expect(getRoleColor(state, 'cy')).toBe('#2ecc71');
        expect(getRoleColor(state, 'nobody')).toBeUndefined();
    });

    test('a team can hide role colours', () => {
        const hidden = stateWith(groups, true);
        expect(roleColorsShown(hidden)).toBe(false);
        expect(getRoleColor(hidden, 'bob')).toBeUndefined();
    });

    test('the hoisted role is the highest shown apart', () => {
        expect(hoistedRole([gamers, mods])).toBe(mods);
        expect(hoistedRole([gamers, plain])).toBeUndefined();
        expect(hoistedRole(undefined)).toBeUndefined();
    });
});

// Antimatter: the default role everyone has colours those with no other coloured role, but isn't listed among anyone's
// roles nor given to anyone.
describe('the default role', () => {
    const mods = group('mods', {position: 0, color: '#e67e22', member_ids: ['ada']});
    const everyone = group('everyone', {position: 10000, color: '#95a5a6', member_ids: ['ada', 'bob']});
    const state = {
        entities: {
            groups: {groups: {mods, everyone}},
            teams: {currentTeamId: 'team', teams: {team: {id: 'team', default_role_id: 'everyone'}}},
        },
    } as unknown as GlobalState;

    test('is told from the others', () => {
        expect(isDefaultRole(state, everyone)).toBe(true);
        expect(isDefaultRole(state, mods)).toBe(false);
    });

    test('isn\'t among the roles given to some, nor among someone\'s roles', () => {
        expect(getTeamRoles(state, 'team').map((r) => r.id)).toEqual(['mods', 'everyone']);
        expect(getGivenTeamRoles(state, 'team').map((r) => r.id)).toEqual(['mods']);
        expect(getGivenRolesOf(state, 'ada').map((r) => r.id)).toEqual(['mods']);
        expect(getGivenRolesOf(state, 'bob')).toEqual([]);
    });

    test('colours those with no other coloured role', () => {
        expect(getRoleColor(state, 'ada')).toBe('#e67e22');
        expect(getRoleColor(state, 'bob')).toBe('#95a5a6');
    });
});
