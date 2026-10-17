// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Channel} from '@mattermost/types/channels';

import {renderHookWithContext} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import {useCanAddPeople} from './member_add_popover';

// An archived channel still takes members, to read it (an auditor going through old information), as the server
// lets them.
describe('fusion/members/useCanAddPeople', () => {
    const channel = (extra: Partial<Channel> = {}) => TestHelper.getChannelMock({id: 'ch', team_id: 'team', type: 'O', delete_at: 0, ...extra});
    const add = (c: Channel, permissions: string[]) => renderHookWithContext(() => useCanAddPeople(c), {
        entities: {
            general: {config: {}},
            users: {currentUserId: 'me', profiles: {me: TestHelper.getUserMock({id: 'me', roles: 'system_user'})}},
            teams: {
                currentTeamId: 'team',
                teams: {team: TestHelper.getTeamMock({id: 'team'})},
                myMembers: {team: {team_id: 'team', user_id: 'me', roles: 'team_user'}},
            },
            channels: {
                channels: {ch: c},
                myMembers: {ch: {channel_id: 'ch', user_id: 'me', roles: 'channel_user'}},
                roles: {ch: new Set(['channel_user'])},
                overrides: {},
            },
            roles: {roles: {
                system_user: {name: 'system_user', permissions: []},
                team_user: {name: 'team_user', permissions: []},
                channel_user: {name: 'channel_user', permissions},
            }},
        },
    } as never).result.current.add;

    test('adds people to an archived channel, for those who may', () => {
        expect(add(channel({delete_at: 5}), ['manage_public_channel_members'])).toBe(true);
        expect(add(channel({delete_at: 5, type: 'P'}), ['manage_private_channel_members'])).toBe(true);
        expect(add(channel({delete_at: 5}), [])).toBe(false);
    });

    test('adds people to a channel as before', () => {
        expect(add(channel(), ['manage_public_channel_members'])).toBe(true);
        expect(add(channel(), [])).toBe(false);
    });
});
