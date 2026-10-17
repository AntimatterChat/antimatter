// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useRef} from 'react';

import {markChannelAsRead} from 'mattermost-redux/actions/channels';

import {loadLatestPosts, loadPostsAround} from 'actions/views/channel';

import {renderWithContext} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import MessageList from './message_list';

jest.mock('actions/views/channel', () => ({
    loadPostsAround: jest.fn(() => ({type: 'MOCK_LOAD_POSTS_AROUND'})),
    loadLatestPosts: jest.fn(() => ({type: 'MOCK_LOAD_LATEST_POSTS'})),
    loadPosts: jest.fn(() => ({type: 'MOCK_LOAD_POSTS'})),
    syncPostsInChannel: jest.fn(() => ({type: 'MOCK_SYNC_POSTS'})),
}));

jest.mock('mattermost-redux/actions/channels', () => ({
    ...jest.requireActual('mattermost-redux/actions/channels'),
    markChannelAsRead: jest.fn(() => ({type: 'MOCK_MARK_CHANNEL_AS_READ'})),
}));

jest.mock('plugins/pluggable', () => () => null);

// The messages themselves: what the list asks of them.
jest.mock('./message', () => ({
    __esModule: true,
    default: ({postId, highlighted}: {postId: string; highlighted?: boolean}) => (
        <div
            id={`post_${postId}`}
            data-testid='message'
            data-highlighted={String(Boolean(highlighted))}
        />
    ),
}));

const me = TestHelper.getUserMock({id: 'me', username: 'maya'});
const day = Date.UTC(2026, 8, 1, 10);
const posts = ['older', 'root', 'reply'].map((id, i) => TestHelper.getPostMock({id, type: '', channel_id: 'oncall', user_id: 'me', create_at: day + (i * 60000)}));

// A moved thread keeps its times: its messages are older than the channel's latest ones, in a chunk of their own.
const state = {
    entities: {
        general: {config: {}},
        users: {currentUserId: 'me', profiles: {me}},
        posts: {
            posts: Object.fromEntries(posts.map((p) => [p.id, p])),
            postsInChannel: {oncall: [{order: ['reply', 'root', 'older'], recent: false, oldest: false}]},
            postsInThread: {},
            reactions: {},
        },
        preferences: {myPreferences: {}},
    },
} as never;

type Props = {focusedPostId?: string; highlight?: boolean};

function List(props: Props) {
    const ref = useRef<HTMLDivElement>(null);
    return (
        <div ref={ref}>
            <MessageList
                channelId='oncall'
                scrollRef={ref}
                {...props}
            />
        </div>
    );
}

describe('fusion/messages/MessageList around a linked message', () => {
    const scrollIntoView = jest.fn();
    beforeAll(() => {
        Element.prototype.scrollIntoView = scrollIntoView;
    });
    beforeEach(() => {
        jest.clearAllMocks();
    });

    test('opens around the linked message, highlighted', () => {
        const {container} = renderWithContext(
            <List
                focusedPostId='root'
                highlight={true}
            />,
            state,
        );
        expect(loadPostsAround).toHaveBeenCalledWith('oncall', 'root');
        expect(container.querySelector('#post_root')).toHaveAttribute('data-highlighted', 'true');
        expect(container.querySelector('#post_older')).toHaveAttribute('data-highlighted', 'false');
        expect(scrollIntoView).toHaveBeenCalledTimes(1);
        expect(markChannelAsRead).not.toHaveBeenCalled();
    });

    test('once the highlight ends, the list stays where it is: same container, no reload, no scroll, the channel read', () => {
        const {container, rerender} = renderWithContext(
            <List
                focusedPostId='root'
                highlight={true}
            />,
            state,
        );
        const list = container.querySelector('.am-msgs');
        expect(list).toBeInTheDocument();

        // ChannelIdentifierRouter drops the message from the address; ChannelView keeps it as the list's anchor.
        rerender(
            <List
                focusedPostId='root'
                highlight={false}
            />,
        );
        expect(container.querySelector('.am-msgs')).toBe(list);
        expect(container.querySelectorAll('[data-testid="message"]')).toHaveLength(3);
        expect(container.querySelector('#post_root')).toHaveAttribute('data-highlighted', 'false');
        expect(loadPostsAround).toHaveBeenCalledTimes(1);
        expect(loadLatestPosts).not.toHaveBeenCalled();
        expect(scrollIntoView).toHaveBeenCalledTimes(1);
        expect(markChannelAsRead).toHaveBeenCalledWith('oncall');
    });

    test('while the messages around a linked message load, the list shows the loading state', () => {
        const {container} = renderWithContext(
            <List
                focusedPostId='unknown'
                highlight={true}
            />,
            state,
        );
        expect(container).toHaveTextContent('Loading messages…');
        expect(container.querySelector('.am-empty')).toBeInTheDocument();
        expect(loadPostsAround).toHaveBeenCalledWith('oncall', 'unknown');
    });
});
