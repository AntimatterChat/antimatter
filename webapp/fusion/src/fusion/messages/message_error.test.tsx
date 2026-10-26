// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Post} from '@mattermost/types/posts';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import Message from './message';

// A message whose content throws while drawing (sec-fusion #2: a malformed image link) shows one line in its place;
// the rest of the conversation keeps working instead of the whole app going blank.
jest.mock('./content/message_content', () => ({
    __esModule: true,
    default: ({post}: {post: Post}) => {
        if (post.message === 'boom') {
            throw new URIError('URI malformed');
        }
        return <p>{post.message}</p>;
    },
}));

describe('fusion/messages: a message that can\'t be drawn', () => {
    let consoleError: jest.SpyInstance;
    beforeEach(() => {
        consoleError = jest.spyOn(console, 'error').mockImplementation(() => {});
    });
    afterEach(() => {
        consoleError.mockRestore();
    });

    const me = TestHelper.getUserMock({id: 'me', username: 'jason'});
    const bob = TestHelper.getUserMock({id: 'bob', username: 'bob'});
    const state = (posts: Post[]) => ({
        entities: {
            users: {currentUserId: me.id, profiles: {me, bob}},
            posts: {posts: Object.fromEntries(posts.map((p) => [p.id, p]))},
            channels: {channels: {channel: TestHelper.getChannelMock({id: 'channel', team_id: 'team'})}},
            teams: {currentTeamId: 'team', teams: {team: TestHelper.getTeamMock({id: 'team', name: 'team'})}},
        },
    } as never);

    test('shows a line in its place, and the next message still shows', () => {
        const bad = TestHelper.getPostMock({id: 'bad', type: '', channel_id: 'channel', user_id: bob.id, message: 'boom', create_at: 1, update_at: 1});
        const good = TestHelper.getPostMock({id: 'good', type: '', channel_id: 'channel', user_id: bob.id, message: 'hello there', create_at: 2, update_at: 2});
        renderWithContext(
            <>
                <Message postId='bad'/>
                <Message
                    postId='good'
                    previousPostId='bad'
                />
            </>,
            state([bad, good]),
        );
        expect(screen.getByRole('note')).toHaveTextContent('This message can\'t be shown.');
        expect(screen.getByText('hello there')).toBeInTheDocument();
    });

    test('is drawn again once it\'s edited', () => {
        const bad = TestHelper.getPostMock({id: 'bad', type: '', channel_id: 'channel', user_id: bob.id, message: 'boom', update_at: 1});
        const {replaceStoreState} = renderWithContext(<Message postId='bad'/>, state([bad]));
        expect(screen.getByRole('note')).toBeInTheDocument();

        replaceStoreState(state([{...bad, message: 'fixed it', update_at: 2}]));
        expect(screen.getByText('fixed it')).toBeInTheDocument();
        expect(screen.queryByRole('note')).not.toBeInTheDocument();
    });
});
