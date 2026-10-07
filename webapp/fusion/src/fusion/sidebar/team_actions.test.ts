// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {setCategoryMuted} from 'mattermost-redux/actions/channel_categories';
import {readMultipleChannels} from 'mattermost-redux/actions/channels';

import type {GlobalState} from 'types/store';

import {getUnreadChannelIdsInTeam, isTeamMuted, markTeamAsRead, setTeamMuted} from './team_actions';

jest.mock('mattermost-redux/actions/channel_categories', () => ({
    setCategoryMuted: jest.fn((id: string, muted: boolean) => ({type: 'MUTED', id, muted})),
}));
jest.mock('mattermost-redux/actions/channels', () => ({
    readMultipleChannels: jest.fn((ids: string[]) => ({type: 'READ', ids})),
}));

describe('fusion/sidebar/team_actions', () => {
    const channel = (id: string, teamId: string) => ({id, team_id: teamId, delete_at: 0, type: 'O'});
    const category = (id: string, type: string, muted: boolean, channelIds = ['x']) => ({id, team_id: 'team', type, muted, channel_ids: channelIds});
    const state = (categoriesMuted: boolean[]) => ({
        entities: {
            channels: {
                channels: {read: channel('read', 'team'), unread: channel('unread', 'team'), other: channel('other', 'other-team')},
                myMembers: {read: {channel_id: 'read', msg_count: 3, mention_count: 0, notify_props: {}}, unread: {channel_id: 'unread', msg_count: 1, mention_count: 0, notify_props: {}}, other: {channel_id: 'other', msg_count: 0, mention_count: 0, notify_props: {}}},
                messageCounts: {read: {root: 3, total: 3}, unread: {root: 4, total: 4}, other: {root: 2, total: 2}},
            },
            channelCategories: {
                byId: {
                    a: category('a', 'channels', categoriesMuted[0]),
                    b: category('b', 'custom', categoriesMuted[1]),
                    dms: category('dms', 'direct_messages', false),
                    empty: category('empty', 'favorites', false, []),
                },
                orderByTeam: {team: ['empty', 'a', 'b', 'dms']},
            },
            preferences: {myPreferences: {}},
            general: {config: {}},
            users: {currentUserId: 'me'},
        },
    } as unknown as GlobalState);

    test('finds the unread channels of any team', () => {
        expect(getUnreadChannelIdsInTeam(state([false, false]), 'team')).toEqual(['unread']);
        expect(getUnreadChannelIdsInTeam(state([false, false]), 'other-team')).toEqual(['other']);
    });

    test('marks a team as read', async () => {
        const dispatch: jest.Mock = jest.fn((a: unknown): unknown => (typeof a === 'function' ? a(dispatch, () => state([false, false])) : a));
        await markTeamAsRead('team')(dispatch as never, () => state([false, false]), undefined);
        expect(readMultipleChannels).toHaveBeenCalledWith(['unread']);
    });

    test('mutes a team through its categories, leaving direct messages and empty categories', async () => {
        expect(isTeamMuted(state([true, false]), 'team')).toBe(false);
        expect(isTeamMuted(state([true, true]), 'team')).toBe(true);

        const dispatch = jest.fn((a) => a);
        await setTeamMuted('team', true)(dispatch as never, () => state([true, false]), undefined);
        expect(setCategoryMuted).toHaveBeenCalledTimes(1);
        expect(setCategoryMuted).toHaveBeenCalledWith('b', true);
    });
});
