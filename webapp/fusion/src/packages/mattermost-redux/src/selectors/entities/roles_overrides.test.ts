// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {ChannelOverride} from '@mattermost/types/channel_overrides';
import type {GlobalState} from '@mattermost/types/store';

import {Permissions} from 'mattermost-redux/constants';
import {applyChannelOverrides, haveIChannelPermission} from 'mattermost-redux/selectors/entities/roles';

const role = (id: string, allow: string[], deny: string[]): ChannelOverride => ({channel_id: 'dev', target_type: 'role', target_id: id, allow, deny, update_at: 0});
const person = (id: string, allow: string[], deny: string[]): ChannelOverride => ({channel_id: 'dev', target_type: 'user', target_id: id, allow, deny, update_at: 0});

describe('applyChannelOverrides', () => {
    const post = Permissions.CREATE_POST;

    test('a role\'s deny takes a permission away, another role\'s allow gives it back', () => {
        expect(applyChannelOverrides(true, post, ['muted'], 'me', [role('muted', [], [post])])).toBe(false);
        expect(applyChannelOverrides(true, post, ['muted', 'speakers'], 'me', [role('muted', [], [post]), role('speakers', [post], [])])).toBe(true);
    });

    test('someone\'s own override wins over their roles\'', () => {
        expect(applyChannelOverrides(true, post, ['speakers'], 'me', [role('speakers', [post], []), person('me', [], [post])])).toBe(false);
        expect(applyChannelOverrides(false, post, ['muted'], 'me', [role('muted', [], [post]), person('me', [post], [])])).toBe(true);
    });

    test('overrides of roles someone lacks, or of other people, change nothing', () => {
        expect(applyChannelOverrides(true, post, [], 'me', [role('muted', [], [post]), person('you', [], [post])])).toBe(true);
    });
});

describe('haveIChannelPermission with overrides', () => {
    const stateWith = (overrides: ChannelOverride[] | undefined, teamRoles = 'team_user team_role_muted', admin = false) => ({
        entities: {
            general: {serverVersion: ''},
            users: {currentUserId: 'me', profiles: {me: {id: 'me', roles: 'system_user'}}},
            teams: {currentTeamId: 'team', myMembers: {team: {team_id: 'team', user_id: 'me', roles: admin ? `${teamRoles} team_admin` : teamRoles}}},
            channels: {currentChannelId: 'dev', myMembers: {dev: {channel_id: 'dev', user_id: 'me', roles: 'channel_user'}}, roles: {dev: new Set(['channel_user'])}, overrides: overrides ? {dev: overrides} : {}},
            roles: {
                roles: {
                    system_user: {name: 'system_user', permissions: []},
                    team_user: {name: 'team_user', permissions: []},
                    team_admin: {name: 'team_admin', permissions: [Permissions.MANAGE_TEAM]},
                    team_role_muted: {name: 'team_role_muted', permissions: []},
                    channel_user: {name: 'channel_user', permissions: [Permissions.CREATE_POST]},
                },
            },
        },
    }) as unknown as GlobalState;

    test('without overrides, roles answer', () => {
        expect(haveIChannelPermission(stateWith(undefined), 'team', 'dev', Permissions.CREATE_POST)).toBe(true);
    });

    test('a deny of one of my roles takes the permission away here', () => {
        expect(haveIChannelPermission(stateWith([role('muted', [], [Permissions.CREATE_POST])]), 'team', 'dev', Permissions.CREATE_POST)).toBe(false);
    });

    test('a person\'s allow gives what roles don\'t', () => {
        expect(haveIChannelPermission(stateWith([person('me', [Permissions.DELETE_OTHERS_POSTS], [])]), 'team', 'dev', Permissions.DELETE_OTHERS_POSTS)).toBe(true);
    });

    test('team admins pass overrides by', () => {
        expect(haveIChannelPermission(stateWith([role('muted', [], [Permissions.CREATE_POST])], 'team_user team_role_muted', true), 'team', 'dev', Permissions.CREATE_POST)).toBe(true);
    });

    test('permissions overrides don\'t touch stay as roles say', () => {
        expect(haveIChannelPermission(stateWith([person('me', ['read_channel'], [])]), 'team', 'dev', 'read_channel')).toBe(false);
    });
});
