// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Channel} from '@mattermost/types/channels';
import type {TeamKind} from '@mattermost/types/teams';

import {TestHelper} from 'utils/test_helper';

import type {GlobalState} from 'types/store';

import {canArchiveChannel, isCorporateCommunity} from './community_channels';

// Who may archive a channel, as the server's SessionCanArchiveChannel: in a community, not its members, even of the
// channels they created; in a corporate community, the channels' admins (their creators) too.
describe('fusion/channel/community_channels canArchiveChannel', () => {
    const channel = (extra: Partial<Channel> = {}) => TestHelper.getChannelMock({id: 'ch', team_id: 'team', type: 'O', delete_at: 0, ...extra});
    const state = ({kind = 'community', channelRoles = ['channel_user'], teamRoles = ['team_user'], permissions = {}}: {kind?: TeamKind; channelRoles?: string[]; teamRoles?: string[]; permissions?: Record<string, string[]>} = {}) => ({
        entities: {
            general: {config: {}},
            users: {currentUserId: 'me', profiles: {me: TestHelper.getUserMock({id: 'me', roles: 'system_user'})}},
            teams: {
                teams: {team: TestHelper.getTeamMock({id: 'team', kind})},
                myMembers: {team: {team_id: 'team', user_id: 'me', roles: teamRoles.join(' ')}},
            },
            channels: {
                channels: {},
                myMembers: {ch: {channel_id: 'ch', user_id: 'me', roles: channelRoles.join(' ')}},
                roles: {ch: new Set(channelRoles)},
                overrides: {},
            },
            roles: {roles: {
                system_user: {name: 'system_user', permissions: []},
                team_user: {name: 'team_user', permissions: permissions.team_user || []},
                team_admin: {name: 'team_admin', permissions: permissions.team_admin || ['manage_team', 'manage_channel_roles', 'delete_public_channel', 'delete_private_channel']},
                channel_user: {name: 'channel_user', permissions: permissions.channel_user || ['create_post', 'manage_public_channel_properties']},
                channel_admin: {name: 'channel_admin', permissions: ['manage_channel_roles']},
            }},
        },
    }) as unknown as GlobalState;

    test('in a community, members don\'t archive, nor the channels\' admins', () => {
        expect(canArchiveChannel(state(), channel())).toBe(false);
        expect(canArchiveChannel(state({channelRoles: ['channel_user', 'channel_admin']}), channel())).toBe(false);
        expect(canArchiveChannel(state({channelRoles: ['channel_user', 'channel_admin']}), channel({type: 'P'}))).toBe(false);
    });

    test('in a corporate community, the channels\' admins archive, not the other members', () => {
        expect(canArchiveChannel(state({kind: 'corporate'}), channel())).toBe(false);
        expect(canArchiveChannel(state({kind: 'corporate', channelRoles: ['channel_user', 'channel_admin']}), channel())).toBe(true);
        expect(canArchiveChannel(state({kind: 'corporate', channelRoles: ['channel_user', 'channel_admin']}), channel({type: 'P'}))).toBe(true);
    });

    test('the community\'s admins archive in both kinds', () => {
        for (const kind of ['community', 'corporate'] as TeamKind[]) {
            expect(canArchiveChannel(state({kind, teamRoles: ['team_user', 'team_admin']}), channel())).toBe(true);
            expect(canArchiveChannel(state({kind, teamRoles: ['team_user', 'team_admin']}), channel({type: 'P'}))).toBe(true);
        }
    });

    test('a permission a system admin gave members still applies', () => {
        expect(canArchiveChannel(state({permissions: {channel_user: ['delete_public_channel']}}), channel())).toBe(true);
    });

    test('nothing archived, nor a direct or group message', () => {
        const admin = state({teamRoles: ['team_user', 'team_admin']});
        expect(canArchiveChannel(admin, channel({delete_at: 5}))).toBe(false);
        expect(canArchiveChannel(admin, channel({type: 'D', team_id: ''}))).toBe(false);
        expect(canArchiveChannel(admin, channel({type: 'G', team_id: ''}))).toBe(false);
        expect(canArchiveChannel(admin, undefined)).toBe(false);
    });

    test('a community is of the community kind unless it says it\'s corporate', () => {
        expect(isCorporateCommunity(state({kind: 'corporate'}), 'team')).toBe(true);
        expect(isCorporateCommunity(state(), 'team')).toBe(false);
        expect(isCorporateCommunity(state(), 'unknown')).toBe(false);
    });
});
