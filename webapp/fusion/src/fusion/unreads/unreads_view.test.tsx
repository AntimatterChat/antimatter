// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Channel} from '@mattermost/types/channels';
import type {Post} from '@mattermost/types/posts';

import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import UnreadsView from './unreads_view';

jest.mock('fusion/components/avatar', () => () => null);
jest.mock('fusion/hooks/users', () => ({
    ...jest.requireActual('fusion/hooks/users'),
    useDisplayName: (user?: {username?: string}) => user?.username || '',
}));

const general = TestHelper.getChannelMock({id: 'general', type: 'O', name: 'general', display_name: 'general', team_id: 't1', last_post_at: 5000});
const aliceDm = TestHelper.getChannelMock({id: 'alice-dm', type: 'D', name: 'alice-dm', display_name: 'Alice', teammate_id: 'alice', team_id: 't1', last_post_at: 9000});

let mockUnreadChannels: Channel[] = [general, aliceDm];
jest.mock('mattermost-redux/selectors/entities/channels', () => ({
    ...jest.requireActual('mattermost-redux/selectors/entities/channels'),
    getUnreadChannels: () => mockUnreadChannels,
    getMyChannelMember: () => ({last_viewed_at: 0}),
}));

const mockMarkChannelAsRead = jest.fn();
jest.mock('mattermost-redux/actions/channels', () => ({
    ...jest.requireActual('mattermost-redux/actions/channels'),
    markChannelAsRead: (channelId: string) => () => {
        mockMarkChannelAsRead(channelId);
        return {data: true};
    },
}));

const mockGetPostsSince = jest.fn((channelId: string) => ({
    data: {
        order: [`p-${channelId}`],
        posts: {[`p-${channelId}`]: {id: `p-${channelId}`, user_id: 'bob', message: `hi from ${channelId}`, create_at: 100, channel_id: channelId} as Post},
    },
}));
jest.mock('mattermost-redux/actions/posts', () => ({
    ...jest.requireActual('mattermost-redux/actions/posts'),
    getPostsSince: (channelId: string) => () => mockGetPostsSince(channelId),
}));

function state() {
    return {
        entities: {
            users: {currentUserId: 'me', profiles: {bob: {id: 'bob', username: 'bob'}}},
            teams: {currentTeamId: 't1', teams: {t1: {id: 't1', name: 'lumen', display_name: 'Lumen', delete_at: 0}}},
        },
    } as never;
}

// F125: the Unreads view.
describe('fusion/unreads/UnreadsView', () => {
    beforeEach(() => {
        mockUnreadChannels = [general, aliceDm];
        mockMarkChannelAsRead.mockClear();
        mockGetPostsSince.mockClear();
    });

    test('says so when nothing is unread', () => {
        mockUnreadChannels = [];
        renderWithContext(<UnreadsView/>, state());
        expect(screen.getByText("You're all caught up.")).toBeInTheDocument();
    });

    test('lists every unread conversation, its most recently active first', async () => {
        renderWithContext(<UnreadsView/>, state());
        await waitFor(() => expect(mockGetPostsSince).toHaveBeenCalledTimes(2));
        expect(mockGetPostsSince).toHaveBeenCalledWith('general');
        expect(mockGetPostsSince).toHaveBeenCalledWith('alice-dm');
        const names = screen.getAllByRole('button', {name: /^(Alice|#general)$/}).map((el) => el.textContent);
        expect(names).toEqual(['Alice', '#general']);
        expect(screen.getByText('hi from alice-dm')).toBeInTheDocument();
        expect(screen.getByText('hi from general')).toBeInTheDocument();
    });

    test('marks one conversation as read on its own, without touching the others', async () => {
        renderWithContext(<UnreadsView/>, state());
        await userEvent.click(screen.getByRole('button', {name: 'Mark Alice as read'}));
        expect(mockMarkChannelAsRead).toHaveBeenCalledWith('alice-dm');
        expect(mockMarkChannelAsRead).not.toHaveBeenCalledWith('general');
    });
});
