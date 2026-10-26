// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Channel} from '@mattermost/types/channels';
import type {Post} from '@mattermost/types/posts';

import {act, fireEvent, renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import UnreadsView, {newPostsOf} from './unreads_view';

jest.mock('fusion/components/avatar', () => () => null);
jest.mock('fusion/hooks/users', () => ({
    ...jest.requireActual('fusion/hooks/users'),
    useDisplayName: (user?: {username?: string}) => user?.username || '',
}));

const general = TestHelper.getChannelMock({id: 'general', type: 'O', name: 'general', display_name: 'general', team_id: 't1', last_post_at: 5000});
const aliceDm = TestHelper.getChannelMock({id: 'alice-dm', type: 'D', name: 'alice-dm', display_name: 'Alice', teammate_id: 'alice', team_id: 't1', last_post_at: 9000});
const random = TestHelper.getChannelMock({id: 'random', type: 'O', name: 'random', display_name: 'random', team_id: 't1', last_post_at: 1000});

let mockUnreadChannels: Channel[] = [general, aliceDm];
jest.mock('mattermost-redux/selectors/entities/channels', () => ({
    ...jest.requireActual('mattermost-redux/selectors/entities/channels'),
    getUnreadChannels: () => mockUnreadChannels,
    getMyChannelMember: () => ({last_viewed_at: 10}),
}));

// Reading a conversation takes it out of the unread ones, as the real reducer does. The action it dispatches changes
// the store's state (its membership), or the selectors wouldn't run again (react-redux skips them for the same state).
const mockMarkChannelAsRead = jest.fn();
jest.mock('mattermost-redux/actions/channels', () => ({
    ...jest.requireActual('mattermost-redux/actions/channels'),
    markChannelAsRead: (channelId: string) => (dispatch: (a: unknown) => void) => {
        mockMarkChannelAsRead(channelId);
        mockUnreadChannels = mockUnreadChannels.filter((c) => c.id !== channelId);
        dispatch({type: 'RECEIVED_MY_CHANNEL_MEMBER', data: {channel_id: channelId, user_id: 'me', last_viewed_at: 99999, msg_count: 0, mention_count: 0, notify_props: {}}});
        return {data: true};
    },
}));

const post = (channelId: string, id: string, extra: Partial<Post> = {}) => ({id, user_id: 'bob', message: `${id} in ${channelId}`, create_at: 100, delete_at: 0, channel_id: channelId, ...extra} as Post);
let mockPostsFor: (channelId: string) => Post[] = (channelId) => [post(channelId, `p-${channelId}`, {message: `hi from ${channelId}`})];
const mockGetPostsSince = jest.fn((channelId: string) => {
    const posts = mockPostsFor(channelId);

    // Newest first, as the server orders them.
    return {data: {order: posts.map((p) => p.id).reverse(), posts: Object.fromEntries(posts.map((p) => [p.id, p]))}};
});
jest.mock('mattermost-redux/actions/posts', () => ({
    ...jest.requireActual('mattermost-redux/actions/posts'),
    getPostsSince: (channelId: string) => () => mockGetPostsSince(channelId),
}));

// A stand-in IntersectionObserver: the tests say when a conversation comes into view, and how much of it.
type Observed = {el: Element; callback: IntersectionObserverCallback};
let observed: Observed[] = [];
class MockIntersectionObserver {
    callback: IntersectionObserverCallback;
    constructor(callback: IntersectionObserverCallback) {
        this.callback = callback;
    }
    observe(el: Element) {
        observed.push({el, callback: this.callback});
    }
    unobserve() {
        // Not used.
    }
    disconnect() {
        observed = observed.filter((o) => o.callback !== this.callback);
    }
    takeRecords() {
        return [];
    }
}

function showGroup(name: RegExp | string, ratio: number) {
    const group = screen.getByRole('button', {name}).closest('.am-unread-group')!;
    const entry = observed.find((o) => o.el === group);
    expect(entry).toBeDefined();
    act(() => {
        entry!.callback([{isIntersecting: ratio > 0, intersectionRatio: ratio, target: group} as unknown as IntersectionObserverEntry], {} as IntersectionObserver);
    });
}

function scrollList() {
    fireEvent.wheel(document.querySelector('.am-unreads-list')!);
}

async function settle() {
    await act(async () => {
        await jest.runOnlyPendingTimersAsync();
    });
}

function state() {
    return {
        entities: {
            users: {currentUserId: 'me', profiles: {bob: {id: 'bob', username: 'bob'}, eve: {id: 'eve', username: 'eve'}}},
            teams: {currentTeamId: 't1', teams: {t1: {id: 't1', name: 'lumen', display_name: 'Lumen', delete_at: 0}}},
            preferences: {myPreferences: {'user_block--eve': {category: 'user_block', name: 'eve', value: 'ignore', user_id: 'me'}}},
            general: {config: {}},
        },
    } as never;
}

// F125: the Unreads view.
describe('fusion/unreads/UnreadsView', () => {
    const realObserver = window.IntersectionObserver;
    beforeEach(() => {
        mockUnreadChannels = [general, aliceDm];
        mockPostsFor = (channelId) => [post(channelId, `p-${channelId}`, {message: `hi from ${channelId}`})];
        mockMarkChannelAsRead.mockClear();
        mockGetPostsSince.mockClear();
        observed = [];
        window.IntersectionObserver = MockIntersectionObserver as unknown as typeof IntersectionObserver;
    });
    afterEach(() => {
        window.IntersectionObserver = realObserver;
        jest.useRealTimers();
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

        // The posts land a tick after the call count does (setPosts runs once the dispatched promise settles).
        await waitFor(() => expect(screen.getByText('hi from alice-dm')).toBeInTheDocument());
        expect(screen.getByText('hi from general')).toBeInTheDocument();
    });

    test('marks one conversation as read on its own, without touching the others', async () => {
        renderWithContext(<UnreadsView/>, state());
        await userEvent.click(screen.getByRole('button', {name: 'Mark Alice as read'}));
        expect(mockMarkChannelAsRead).toHaveBeenCalledWith('alice-dm');
        expect(mockMarkChannelAsRead).not.toHaveBeenCalledWith('general');
    });

    // sec-fusion #1: a conversation read left the list at once, the next one slid into view and was read 1.2 s later,
    // and so on until the whole inbox was read without anyone looking.
    test('a conversation read stays in the list, and the next one isn\'t read in its place', async () => {
        mockUnreadChannels = [aliceDm, general, random];
        jest.useFakeTimers();
        renderWithContext(<UnreadsView/>, state());
        await settle();
        scrollList();

        showGroup('Alice', 0.8);
        act(() => {
            jest.advanceTimersByTime(1200);
        });
        expect(mockMarkChannelAsRead).toHaveBeenCalledTimes(1);
        expect(mockMarkChannelAsRead).toHaveBeenCalledWith('alice-dm');

        // Still shown, marked read; the other two are still there and unread.
        expect(screen.getByRole('button', {name: 'Alice'})).toBeInTheDocument();
        expect(screen.getByText('hi from alice-dm')).toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Mark Alice as read'})).not.toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Mark general as read'})).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Mark random as read'})).toBeInTheDocument();

        act(() => {
            jest.advanceTimersByTime(5000);
        });
        expect(mockMarkChannelAsRead).toHaveBeenCalledTimes(1);
    });

    test('nothing is read on its own before you scroll the list', async () => {
        jest.useFakeTimers();
        renderWithContext(<UnreadsView/>, state());
        await settle();
        showGroup('Alice', 1);
        act(() => {
            jest.advanceTimersByTime(5000);
        });
        expect(mockMarkChannelAsRead).not.toHaveBeenCalled();
    });

    // sec-fusion #9: each callback at 0.6 or more started a timer and lost the previous one, which still fired.
    test('scrolling past quickly doesn\'t read, whatever thresholds it crosses', async () => {
        jest.useFakeTimers();
        renderWithContext(<UnreadsView/>, state());
        await settle();
        scrollList();

        showGroup('Alice', 0.6);
        act(() => {
            jest.advanceTimersByTime(150);
        });
        showGroup('Alice', 1);
        act(() => {
            jest.advanceTimersByTime(300);
        });
        showGroup('Alice', 0);
        act(() => {
            jest.advanceTimersByTime(2000);
        });
        expect(mockMarkChannelAsRead).not.toHaveBeenCalled();
    });

    // sec-fusion #10: posts-since also gives older posts edited or deleted since you looked.
    test('shows only messages new since you looked, not old ones edited or deleted since', async () => {
        mockUnreadChannels = [aliceDm];
        mockPostsFor = (channelId) => [
            post(channelId, 'old-edited', {create_at: 5, edit_at: 50}),
            post(channelId, 'deleted', {create_at: 20, delete_at: 30}),
            post(channelId, 'new', {create_at: 40}),
        ];
        renderWithContext(<UnreadsView/>, state());
        await waitFor(() => expect(screen.getByText('new in alice-dm')).toBeInTheDocument());
        expect(screen.queryByText('old-edited in alice-dm')).not.toBeInTheDocument();
        expect(screen.queryByText('deleted in alice-dm')).not.toBeInTheDocument();
    });

    test('newPostsOf keeps the posts created since, oldest first', () => {
        const posts = [post('c', 'a', {create_at: 11}), post('c', 'b', {create_at: 12}), post('c', 'old', {create_at: 9})];
        expect(newPostsOf(['b', 'a', 'old'], Object.fromEntries(posts.map((p) => [p.id, p])), 10).map((p) => p.id)).toEqual(['a', 'b']);
    });

    test('counts the new messages it doesn\'t show, and doesn\'t read them on its own', async () => {
        mockUnreadChannels = [aliceDm];
        mockPostsFor = (channelId) => Array.from({length: 8}, (_, i) => post(channelId, `m${i}`, {create_at: 100 + i}));
        jest.useFakeTimers();
        renderWithContext(<UnreadsView/>, state());
        await settle();
        expect(screen.getByText('3 earlier messages')).toBeInTheDocument();
        expect(screen.queryByText('m2 in alice-dm')).not.toBeInTheDocument();
        expect(screen.getByText('m3 in alice-dm')).toBeInTheDocument();
        expect(screen.getByText('m7 in alice-dm')).toBeInTheDocument();

        scrollList();
        showGroup('Alice', 1);
        act(() => {
            jest.advanceTimersByTime(5000);
        });
        expect(mockMarkChannelAsRead).not.toHaveBeenCalled();
    });

    // sec-fusion #5: a spoiler's text, or the text of someone you ignore, stays hidden in the inbox too.
    test('keeps a spoiler\'s text and an ignored person\'s text hidden', async () => {
        mockUnreadChannels = [aliceDm];
        mockPostsFor = (channelId) => [
            post(channelId, 'spoiler', {message: 'the butler did it', props: {spoiler: true}, create_at: 100}),
            post(channelId, 'ignored', {message: 'hello from eve', user_id: 'eve', create_at: 101}),
        ];
        renderWithContext(<UnreadsView/>, state());
        await waitFor(() => expect(screen.getByText('Spoiler')).toBeInTheDocument());
        expect(screen.queryByText('the butler did it')).not.toBeInTheDocument();
        expect(screen.getByText('Message from someone you ignore')).toBeInTheDocument();
        expect(screen.queryByText('hello from eve')).not.toBeInTheDocument();
    });
});
