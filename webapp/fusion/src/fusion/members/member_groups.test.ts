// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Group} from '@mattermost/types/groups';
import type {UserProfile} from '@mattermost/types/users';

import {renderHookWithContext} from 'tests/react_testing_utils';

import type {MemberRole} from './member_groups';
import {useMemberGroups} from './member_groups';

const user = (id: string) => ({id, username: id, roles: 'system_user', props: {}}) as unknown as UserProfile;
const group = (id: string, fields: Partial<Group>) => ({id, name: id, display_name: id, source: 'custom', delete_at: 0, team_id: '', member_ids: [], ...fields}) as Group;

describe('useMemberGroups', () => {
    const [ada, bob, cy, dee, eve] = ['ada', 'bob', 'cy', 'dee', 'eve'].map(user);
    const users = [ada, bob, cy, dee, eve];
    const statuses = {ada: 'online', bob: 'away', cy: 'online', dee: 'offline', eve: 'dnd'};

    const groupsOf = (groups: Group[], grouping: 'roles' | 'teams' = 'roles', roles: Record<string, MemberRole> = {}) => renderHookWithContext(
        () => useMemberGroups(users, roles, grouping),
        {
            entities: {
                users: {currentUserId: 'ada', profiles: Object.fromEntries(users.map((u) => [u.id, u])), statuses},
                teams: {currentTeamId: 'team', teams: {team: {id: 'team'}}},
                groups: {groups: Object.fromEntries(groups.map((g) => [g.id, g]))},
            },
        } as never,
    ).result.current.map((g) => [g.key, g.users.map((u) => u.id)]);

    test('without roles shown apart: admins, moderators, the others online, then offline', () => {
        expect(groupsOf([], 'roles', {bob: 'admin', cy: 'mod', dee: 'admin'})).toEqual([
            ['admin', ['bob']],
            ['mod', ['cy']],
            ['online', ['ada', 'eve']],
            ['offline', ['dee']],
        ]);
    });

    test('roles shown apart come first, highest first, each with the online people whose highest such role it is', () => {
        const mods = group('mods', {team_id: 'team', position: 0, hoist: true, member_ids: ['bob', 'dee']});
        const crew = group('crew', {team_id: 'team', position: 1, hoist: true, member_ids: ['bob', 'cy']});
        const quiet = group('quiet', {team_id: 'team', position: 2, member_ids: ['ada']});
        expect(groupsOf([crew, quiet, mods])).toEqual([
            ['role-mods', ['bob']],
            ['role-crew', ['cy']],
            ['online', ['ada', 'eve']],
            ['offline', ['dee']],
        ]);
    });

    test('empty role groups are left out', () => {
        const nobody = group('nobody', {team_id: 'team', hoist: true, member_ids: ['dee']});
        expect(groupsOf([nobody]).map(([key]) => key)).toEqual(['online', 'offline']);
    });

    test('by team: each team of the community in name order, online members first, then the others', () => {
        const ops = group('ops', {display_name: 'Ops', tag: 'OPS', team_id: 'team', is_team: true, member_ids: ['dee', 'cy']});
        const det = group('det', {display_name: 'Detector', tag: 'DET', team_id: 'team', is_team: true, member_ids: ['ada']});
        const elsewhere = group('elsewhere', {display_name: 'Elsewhere', team_id: 'other', is_team: true, member_ids: ['bob']});
        expect(groupsOf([ops, det, elsewhere], 'teams')).toEqual([
            ['team-det', ['ada']],
            ['team-ops', ['cy', 'dee']],
            ['online', ['bob', 'eve']],
        ]);
    });

    test('roles are not teams', () => {
        const mods = group('mods', {team_id: 'team', hoist: true, member_ids: ['ada']});
        expect(groupsOf([mods], 'teams').map(([key]) => key)).toEqual(['online', 'offline']);
    });
});

describe('useMemberGroups when the team hides role colours', () => {
    test('roles shown apart keep their group, without their colour', () => {
        const mods = {id: 'mods', name: 'mods', display_name: 'mods', source: 'custom', delete_at: 0, team_id: 'team', hoist: true, color: '#e67e22', member_ids: ['ada']} as unknown as Group;
        const ada = {id: 'ada', username: 'ada', roles: 'system_user', props: {}} as unknown as UserProfile;
        const groups = renderHookWithContext(
            () => useMemberGroups([ada], {}, 'roles'),
            {
                entities: {
                    users: {currentUserId: 'ada', profiles: {ada}, statuses: {ada: 'online'}},
                    teams: {currentTeamId: 'team', teams: {team: {id: 'team', hide_role_colors: true}}},
                    groups: {groups: {mods}},
                },
            } as never,
        ).result.current;
        expect(groups[0]).toMatchObject({key: 'role-mods', color: undefined});
    });
});
