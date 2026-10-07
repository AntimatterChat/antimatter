// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {TestHelper} from 'utils/test_helper';

import type {GlobalState} from 'types/store';

import {getLastTeamChannelName} from './index';

describe('fusion/selectors/getLastTeamChannelName', () => {
    const channel = (id: string, teamId: string, deleteAt = 0) => TestHelper.getChannelMock({id, name: id, team_id: teamId, delete_at: deleteAt});
    const member = (id: string, viewedAt: number) => TestHelper.getChannelMembershipMock({channel_id: id, last_viewed_at: viewedAt});
    const state = {
        entities: {
            channels: {
                channels: {
                    'town-square': channel('town-square', 'team'),
                    'off-topic': channel('off-topic', 'team'),
                    archived: channel('archived', 'team', 1),
                    elsewhere: channel('elsewhere', 'other'),
                    dm: channel('dm', ''),
                },
                myMembers: {
                    'town-square': member('town-square', 1),
                    'off-topic': member('off-topic', 2),
                    archived: member('archived', 5),
                    elsewhere: member('elsewhere', 6),
                    dm: member('dm', 7),
                },
            },
        },
    } as unknown as GlobalState;

    test('names the team channel viewed last, leaving out direct messages, other teams and archived channels', () => {
        expect(getLastTeamChannelName(state, 'team')).toBe('off-topic');
    });

    test('is empty for a team with no channel joined', () => {
        expect(getLastTeamChannelName(state, 'nowhere')).toBe('');
    });
});
