// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useLayoutEffect, useRef, useState} from 'react';

import {PostTypes} from 'mattermost-redux/action_types';
import {markChannelAsRead} from 'mattermost-redux/actions/channels';

import {loadLatestPosts, loadPostsAround, syncPostsInChannel} from 'actions/views/channel';

import {act, fireEvent, renderWithContext, screen, waitFor} from 'tests/react_testing_utils';
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

type Props = {focusedPostId?: string; highlight?: boolean; onJumpToPresent?: () => void};

// The conversation's scrolling view is there before its list, as in ChannelView when a channel opens.
function List(props: Props) {
    const ref = useRef<HTMLDivElement>(null);
    const [ready, setReady] = useState(false);
    useLayoutEffect(() => setReady(true), []);
    return (
        <div ref={ref}>
            {ready && (
                <MessageList
                    channelId='oncall'
                    scrollRef={ref}
                    {...props}
                />
            )}
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

describe('fusion/messages/MessageList: You\'re viewing older messages · Jump to present', () => {
    beforeAll(() => {
        Element.prototype.scrollIntoView = jest.fn();
    });
    beforeEach(() => {
        jest.clearAllMocks();
    });

    // The channel's newest messages are loaded too, in the recent chunk.
    const tom = TestHelper.getUserMock({id: 'tom', username: 'tom'});
    const latest = TestHelper.getPostMock({id: 'latest', type: '', channel_id: 'oncall', user_id: 'tom', create_at: day + 3600000});
    const withPresent = {
        entities: {
            general: {config: {}},
            users: {currentUserId: 'me', profiles: {me, tom}},
            posts: {
                posts: {...Object.fromEntries(posts.map((p) => [p.id, p])), latest},
                postsInChannel: {oncall: [
                    {order: ['reply', 'root', 'older'], recent: false, oldest: false},
                    {order: ['latest'], recent: true, oldest: false},
                ]},
                postsInThread: {},
                reactions: {},
            },
            preferences: {myPreferences: {}},
        },
    } as never;
    const bar = () => screen.queryByText('You\'re viewing older messages');
    const jump = () => screen.getByRole('button', {name: /Jump to present/});

    test('shows on a list opened around an older message, highlighted or not, with a button reached by keyboard', () => {
        const onJumpToPresent = jest.fn();
        const {rerender} = renderWithContext(
            <List
                focusedPostId='root'
                highlight={true}
                onJumpToPresent={onJumpToPresent}
            />,
            withPresent,
        );
        expect(bar()).toBeInTheDocument();
        expect(screen.getByRole('status')).toContainElement(jump());
        expect(jump().tagName).toBe('BUTTON');

        // The highlight has ended; the list is still around the message (the anchor in the address's state).
        rerender(
            <List
                focusedPostId='root'
                highlight={false}
                onJumpToPresent={onJumpToPresent}
            />,
        );
        expect(bar()).toBeInTheDocument();
    });

    test('doesn\'t show in a channel that opened normally', () => {
        renderWithContext(<List onJumpToPresent={jest.fn()}/>, withPresent);
        expect(screen.getAllByTestId('message')).toHaveLength(1);
        expect(bar()).not.toBeInTheDocument();
    });

    test('doesn\'t show around a message among the newest ones', () => {
        renderWithContext(
            <List
                focusedPostId='latest'
                onJumpToPresent={jest.fn()}
            />,
            withPresent,
        );
        expect(bar()).not.toBeInTheDocument();
    });

    test('hides once the list reaches the newest messages (scrolling down joins them)', () => {
        const {replaceStoreState} = renderWithContext(
            <List
                focusedPostId='root'
                onJumpToPresent={jest.fn()}
            />,
            withPresent,
        );
        expect(bar()).toBeInTheDocument();
        const joined = JSON.parse(JSON.stringify(withPresent));
        joined.entities.posts.postsInChannel.oncall = [{order: ['latest', 'reply', 'root', 'older'], recent: true, oldest: false}];
        replaceStoreState(joined);
        expect(bar()).not.toBeInTheDocument();
    });

    test('Jump to present leaves the message, loads the newest messages afresh and reads the channel', async () => {
        const onJumpToPresent = jest.fn();
        const {rerender} = renderWithContext(
            <List
                focusedPostId='root'
                highlight={false}
                onJumpToPresent={onJumpToPresent}
            />,
            withPresent,
        );
        // (The ended highlight has read the channel once already.)
        await waitFor(() => expect(markChannelAsRead).toHaveBeenCalled());
        (markChannelAsRead as jest.Mock).mockClear();

        fireEvent.click(jump());
        expect(onJumpToPresent).toHaveBeenCalledTimes(1);

        // ChannelView's leaveAnchor drops the message from the address.
        rerender(<List onJumpToPresent={onJumpToPresent}/>);
        expect(loadLatestPosts).toHaveBeenCalledWith('oncall');
        expect(syncPostsInChannel).not.toHaveBeenCalled();
        expect(bar()).not.toBeInTheDocument();
        expect(screen.getAllByTestId('message')).toHaveLength(1);
        await waitFor(() => expect(markChannelAsRead).toHaveBeenCalledWith('oncall'));
    });

    test('counts the messages others send meanwhile', () => {
        const {store} = renderWithContext(
            <List
                focusedPostId='root'
                onJumpToPresent={jest.fn()}
            />,
            withPresent,
        );
        expect(screen.getByRole('status')).not.toHaveTextContent('new message');

        const receive = (id: string, userId: string, at: number) => act(() => {
            store.dispatch({type: PostTypes.RECEIVED_NEW_POST, data: TestHelper.getPostMock({id, type: '', channel_id: 'oncall', user_id: userId, create_at: at}), features: {crtEnabled: false}});
        });
        receive('new1', 'tom', day + 3700000);
        expect(screen.getByRole('status')).toHaveTextContent('1 new message');
        receive('new2', 'tom', day + 3800000);
        expect(screen.getByRole('status')).toHaveTextContent('2 new messages');

        // The reader's own messages aren't news to them.
        receive('mine', 'me', day + 3900000);
        expect(screen.getByRole('status')).toHaveTextContent('2 new messages');
    });

    test('without a way to leave the message (the call\'s chat), there\'s no bar', () => {
        renderWithContext(<List focusedPostId='root'/>, withPresent);
        expect(bar()).not.toBeInTheDocument();
    });
});
