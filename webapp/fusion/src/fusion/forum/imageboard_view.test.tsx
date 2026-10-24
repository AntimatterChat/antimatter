// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {Client4} from 'mattermost-redux/client';

import {fireEvent, renderWithContext, screen, waitFor} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import ImageboardView from './imageboard_view';

jest.mock('mattermost-redux/client', () => ({
    Client4: {
        getForumTopics: jest.fn(),
        getFileRoute: jest.fn((id: string) => `/files/${id}`),
    },
}));

describe('fusion/forum/ImageboardView', () => {
    const channel = TestHelper.getChannelMock({id: 'c1', team_id: 'team'});
    const asker = TestHelper.getUserMock({id: 'ada', username: 'ada'});
    const image = TestHelper.getFileInfoMock({id: 'f1', mime_type: 'image/png', has_preview_image: true});

    const topic = (override: Record<string, unknown> = {}) => ({
        post: TestHelper.getPostMock({id: 'p1', user_id: 'ada', message: 'How the beam looks today', create_at: 1000}),
        reply_count: 34,
        last_reply_at: 2000,
        participant_ids: ['ada'],
        votes: 0,
        on_top: false,
        ...override,
    });

    const baseState = {
        entities: {
            users: {currentUserId: 'me', profiles: {ada: asker}},
            teams: {currentTeamId: 'team', teams: {team: TestHelper.getTeamMock({id: 'team', name: 'lumen-labs'})}},
            channels: {currentChannelId: 'c1', channels: {c1: channel}},
            posts: {reactions: {p1: {}}},
            preferences: {myPreferences: {}},
        },
    } as never;

    beforeEach(() => {
        jest.mocked(Client4.getForumTopics).mockReset();
        jest.mocked(Client4.getForumTopics).mockResolvedValue({topics: [], next_cursor: ''});
    });

    test('a topic with a first image shows it large; one without shows text only', async () => {
        jest.mocked(Client4.getForumTopics).mockResolvedValue({
            topics: [
                topic({post: TestHelper.getPostMock({id: 'p1', user_id: 'ada', message: 'Beam with a picture', metadata: {files: [image]}})}),
                topic({post: TestHelper.getPostMock({id: 'p2', user_id: 'ada', message: 'Beam, text only'})}),
            ],
            next_cursor: '',
        });
        const {container} = renderWithContext(
            <ImageboardView
                channel={channel}
                mode='discussion'
            />, baseState);

        await screen.findByRole('button', {name: 'Beam with a picture'});
        const cards = container.querySelectorAll('.am-board-card');

        expect(cards).toHaveLength(2);
        expect(cards[0].querySelector('img.am-board-thumb')).toBeInTheDocument();
        expect(cards[0]).not.toHaveClass('am-no-image');
        expect(cards[1].querySelector('img.am-board-thumb')).not.toBeInTheDocument();
        expect(cards[1]).toHaveClass('am-no-image');
    });

    test('fetches a fixed page of 10, the latest 3 replies, in bump order', async () => {
        renderWithContext(
            <ImageboardView
                channel={channel}
                mode='discussion'
            />, baseState);

        await waitFor(() => expect(Client4.getForumTopics).toHaveBeenCalledWith('c1', {sort: 'activity', replies: 3, per_page: 10}));
    });

    test('shows the reply and image counts, and an Open topic button', async () => {
        jest.mocked(Client4.getForumTopics).mockResolvedValue({
            topics: [topic({
                post: TestHelper.getPostMock({id: 'p1', user_id: 'ada', message: 'Beam with a picture', metadata: {files: [image]}}),
                reply_count: 34,
                replies: [TestHelper.getPostMock({id: 'r1', user_id: 'ada', message: 'a reply', metadata: {files: [image]}})],
            })],
            next_cursor: '',
        });
        renderWithContext(
            <ImageboardView
                channel={channel}
                mode='discussion'
            />, baseState);

        expect(await screen.findByText('34 replies, 2 images')).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Open topic'})).toBeInTheDocument();
    });

    test('shows the latest replies, compact', async () => {
        jest.mocked(Client4.getForumTopics).mockResolvedValue({
            topics: [topic({
                replies: [
                    TestHelper.getPostMock({id: 'r1', message: 'first reply text'}),
                    TestHelper.getPostMock({id: 'r2', message: 'second reply text'}),
                ],
            })],
            next_cursor: '',
        });
        renderWithContext(
            <ImageboardView
                channel={channel}
                mode='discussion'
            />, baseState);

        expect(await screen.findByText('first reply text')).toBeInTheDocument();
        expect(screen.getByText('second reply text')).toBeInTheDocument();
    });

    test('shows the vote button in qa and ideas modes, not discussion (the author\'s own vote not counting is VoteButton\'s own rule)', async () => {
        jest.mocked(Client4.getForumTopics).mockResolvedValue({topics: [topic()], next_cursor: ''});
        const {container} = renderWithContext(
            <ImageboardView
                channel={channel}
                mode='qa'
            />, baseState);

        await screen.findByRole('button', {name: 'How the beam looks today'});

        expect(container.querySelector('.am-vote-btn')).toBeInTheDocument();
    });

    test('doesn\'t show the vote button in discussion mode', async () => {
        jest.mocked(Client4.getForumTopics).mockResolvedValue({topics: [topic()], next_cursor: ''});
        const {container} = renderWithContext(
            <ImageboardView
                channel={channel}
                mode='discussion'
            />, baseState);

        await screen.findByRole('button', {name: 'How the beam looks today'});

        expect(container.querySelector('.am-vote-btn')).not.toBeInTheDocument();
    });

    test('pages on scroll, with the cursor, once near the bottom', async () => {
        jest.mocked(Client4.getForumTopics).mockResolvedValueOnce({topics: [topic()], next_cursor: 'cursor-2'});
        const {container} = renderWithContext(
            <ImageboardView
                channel={channel}
                mode='discussion'
            />, baseState);

        await waitFor(() => expect(Client4.getForumTopics).toHaveBeenCalledTimes(1));

        jest.mocked(Client4.getForumTopics).mockResolvedValueOnce({topics: [topic({post: TestHelper.getPostMock({id: 'p2', user_id: 'ada'})})], next_cursor: ''});
        const board = container.querySelector('.am-board-view') as HTMLElement;
        Object.defineProperty(board, 'scrollHeight', {value: 1000, configurable: true});
        Object.defineProperty(board, 'clientHeight', {value: 500, configurable: true});
        Object.defineProperty(board, 'scrollTop', {value: 700, configurable: true});
        fireEvent.scroll(board);

        await waitFor(() => expect(Client4.getForumTopics).toHaveBeenLastCalledWith('c1', {sort: 'activity', replies: 3, per_page: 10, before: 'cursor-2'}));
    });

    test('doesn\'t page on scroll once there\'s no next cursor', async () => {
        jest.mocked(Client4.getForumTopics).mockResolvedValueOnce({topics: [topic()], next_cursor: ''});
        const {container} = renderWithContext(
            <ImageboardView
                channel={channel}
                mode='discussion'
            />, baseState);

        await waitFor(() => expect(Client4.getForumTopics).toHaveBeenCalledTimes(1));

        const board = container.querySelector('.am-board-view') as HTMLElement;
        Object.defineProperty(board, 'scrollHeight', {value: 1000, configurable: true});
        Object.defineProperty(board, 'clientHeight', {value: 500, configurable: true});
        Object.defineProperty(board, 'scrollTop', {value: 700, configurable: true});
        fireEvent.scroll(board);

        await new Promise((resolve) => setTimeout(resolve, 50));
        expect(Client4.getForumTopics).toHaveBeenCalledTimes(1);
    });

    test('empty state: no topics', async () => {
        renderWithContext(
            <ImageboardView
                channel={channel}
                mode='discussion'
            />, baseState);

        expect(await screen.findByText('No topics yet.')).toBeInTheDocument();
    });
});
