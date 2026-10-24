// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';
import {getHistory} from 'utils/browser_history';
import {TestHelper} from 'utils/test_helper';

import TopicPage from './topic_page';

jest.mock('fusion/messages/message', () => ({__esModule: true, default: ({postId}: {postId: string}) => <div>{`Message:${postId}`}</div>}));
jest.mock('fusion/composer/composer', () => ({__esModule: true, default: () => <div>{'composer'}</div>}));
jest.mock('mattermost-redux/actions/posts', () => ({getPostThread: jest.fn(() => ({type: 'MOCK_GET_POST_THREAD'}))}));
jest.mock('mattermost-redux/actions/threads', () => ({getThread: jest.fn(() => ({type: 'MOCK_FETCH_THREAD'})), updateThreadRead: jest.fn(() => ({type: 'MOCK_UPDATE_THREAD_READ'}))}));
jest.mock('actions/views/threads', () => ({markThreadAsRead: jest.fn(() => ({type: 'MOCK_MARK_READ'})), updateThreadLastOpened: jest.fn(() => ({type: 'MOCK_LAST_OPENED'}))}));

describe('fusion/forum/TopicPage', () => {
    const channel = TestHelper.getChannelMock({id: 'c1', team_id: 'team', name: 'beam-questions'});
    const root = TestHelper.getPostMock({id: 'r0', channel_id: 'c1', user_id: 'ada', message: 'How do I reset the beam?', props: {am_thread_title: 'How do I reset the beam?'}});

    const baseState = (channelTypes: Record<string, unknown>, posts: Record<string, unknown> = {}, postsInThread: Record<string, unknown> = {}) => ({
        entities: {
            general: {config: {}},
            users: {currentUserId: 'me'},
            teams: {currentTeamId: 'team', teams: {team: TestHelper.getTeamMock({id: 'team', name: 'lumen-labs'})}},
            channels: {currentChannelId: 'c1', channels: {c1: channel}, channelTypes},
            posts: {posts: {r0: root, ...posts}, postsInThread: {r0: [], ...postsInThread}, reactions: {}},
            threads: {threads: {}},
            preferences: {myPreferences: {}},
        },
    } as never);

    test('shows the title and the back link to the topics view', () => {
        renderWithContext(
            <TopicPage
                channel={channel}
                rootId='r0'
            />,
            baseState({c1: {type: 'forum', mode: 'discussion', root_posts: 'members', view: 'topics', preset: ''}}),
        );

        expect(screen.getByRole('heading', {name: 'How do I reset the beam?'})).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'All topics'})).toBeInTheDocument();
        expect(screen.getByText('Message:r0')).toBeInTheDocument();
    });

    test('Q&A: the question and its replies, accepted answer first', () => {
        const r1 = TestHelper.getPostMock({id: 'r1', channel_id: 'c1', root_id: 'r0', user_id: 'maya', create_at: 100, type: ''});
        const r2 = TestHelper.getPostMock({id: 'r2', channel_id: 'c1', root_id: 'r0', user_id: 'ivy', create_at: 200, type: ''});
        renderWithContext(
            <TopicPage
                channel={channel}
                rootId='r0'
            />,
            baseState(
                {c1: {type: 'forum', mode: 'qa', root_posts: 'members', view: 'topics', preset: ''}},
                {r1, r2},
                {r0: ['r1', 'r2']},
            ),
        );

        // No accepted answer and no votes: oldest first (reply_order.test.ts covers the ordering itself).
        const messages = screen.getAllByText(/^Message:/).map((el) => el.textContent);
        expect(messages).toEqual(['Message:r0', 'Message:r1', 'Message:r2']);
    });

    test('shows "N replies" above the replies', () => {
        const r1 = TestHelper.getPostMock({id: 'r1', channel_id: 'c1', root_id: 'r0', create_at: 100, type: ''});
        renderWithContext(
            <TopicPage
                channel={channel}
                rootId='r0'
            />,
            baseState({c1: {type: 'forum', mode: 'discussion', root_posts: 'members', view: 'topics', preset: ''}}, {r1}, {r0: ['r1']}),
        );

        expect(screen.getByText('1 reply')).toBeInTheDocument();
    });

    test('"All topics" goes back to the channel', async () => {
        const push = jest.spyOn(getHistory(), 'push').mockImplementation(() => undefined);
        renderWithContext(
            <TopicPage
                channel={channel}
                rootId='r0'
            />,
            baseState({c1: {type: 'forum', mode: 'discussion', root_posts: 'members', view: 'topics', preset: ''}}),
        );

        await userEvent.click(screen.getByRole('button', {name: 'All topics'}));

        expect(push).toHaveBeenCalledWith('/lumen-labs/channels/beam-questions');
    });

    test('Ideas: the status chip shows under the title', () => {
        const idea = {...root, props: {...root.props, am_topic_status: 'planned'}};
        renderWithContext(
            <TopicPage
                channel={channel}
                rootId='r0'
            />,
            baseState({c1: {type: 'forum', mode: 'ideas', root_posts: 'members', view: 'topics', preset: ''}}, {r0: idea}),
        );

        expect(screen.getByText('Planned')).toBeInTheDocument();
    });
});
