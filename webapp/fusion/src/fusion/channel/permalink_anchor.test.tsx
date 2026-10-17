// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {createMemoryHistory} from 'history';
import React from 'react';
import {Route} from 'react-router-dom';

import {PostTypes} from 'mattermost-redux/action_types';

import {act, fireEvent, renderWithContext, screen} from 'tests/react_testing_utils';
import {getHistory} from 'utils/browser_history';
import {TestHelper} from 'utils/test_helper';

import {usePermalinkAnchor} from './permalink_anchor';

const me = TestHelper.getUserMock({id: 'me', username: 'maya'});
const tom = TestHelper.getUserMock({id: 'tom', username: 'tom'});
const latest = TestHelper.getPostMock({id: 'latest', type: '', channel_id: 'oncall', user_id: 'tom', create_at: 2000});
const state = {
    entities: {
        general: {config: {}},
        users: {currentUserId: 'me', profiles: {me, tom}},
        posts: {
            posts: {latest},
            postsInChannel: {oncall: [{order: ['latest'], recent: true}]},
            postsInThread: {},
            reactions: {},
        },
        preferences: {myPreferences: {}},
    },
} as never;

function Probe({channelId, resolving}: {channelId: string; resolving: boolean}) {
    const {linkedPostId, anchorPostId, leaveAnchor} = usePermalinkAnchor(channelId, resolving);
    return (
        <button
            type='button'
            data-testid='probe'
            data-linked={linkedPostId || ''}
            data-anchor={anchorPostId || ''}
            onClick={leaveAnchor}
        />
    );
}

// The routes of CenterChannel: a permalink and a conversation, one route.
function view(channelId: string, resolving = false) {
    return (
        <Route
            path={['/lumen/pl/:postid', '/:team/:path(channels|messages)/:identifier/:postid?']}
            render={() => (
                <Probe
                    channelId={channelId}
                    resolving={resolving}
                />
            )}
        />
    );
}

function setup(path: string, locationState?: unknown) {
    const history = createMemoryHistory({initialEntries: [{pathname: path, state: locationState}]});

    // The hook drops the anchor through the app's history.
    jest.spyOn(getHistory(), 'replace').mockImplementation((to: unknown) => history.replace(to as string));
    return {history, ...renderWithContext(view('oncall'), state, {history})};
}

const probe = () => screen.getByTestId('probe');

describe('fusion/channel/usePermalinkAnchor', () => {
    afterEach(() => {
        jest.restoreAllMocks();
    });

    test('the linked message is highlighted, then stays the list\'s anchor once the address drops it', () => {
        const {history} = setup('/lumen/channels/ops-oncall/newroot');
        expect(probe()).toHaveAttribute('data-linked', 'newroot');
        expect(probe()).toHaveAttribute('data-anchor', 'newroot');

        act(() => history.replace('/lumen/channels/ops-oncall', {permalinkAnchor: 'newroot'}));
        expect(probe()).toHaveAttribute('data-linked', '');
        expect(probe()).toHaveAttribute('data-anchor', 'newroot');
    });

    test('opening the conversation again (from the sidebar) brings its newest messages', () => {
        const {history} = setup('/lumen/channels/ops-oncall', {permalinkAnchor: 'newroot'});
        act(() => history.push('/lumen/channels/ops-oncall'));
        expect(probe()).toHaveAttribute('data-anchor', '');
    });

    test('while a permalink resolves, the conversation shown keeps its anchor; another conversation has none', () => {
        const {history, rerender} = setup('/lumen/channels/ops-oncall', {permalinkAnchor: 'newroot'});

        // ChannelIdentifierRouter tells the view from the address; here, the two in turn.
        rerender(view('oncall', true));
        act(() => history.push('/lumen/pl/pointer'));
        expect(probe()).toHaveAttribute('data-linked', '');
        expect(probe()).toHaveAttribute('data-anchor', 'newroot');

        // focusPost selected the post's conversation.
        rerender(view('general', true));
        expect(probe()).toHaveAttribute('data-anchor', '');
    });

    test('sending a message while the list is around an older one brings back the newest messages', () => {
        const {history, store} = setup('/lumen/channels/ops-oncall', {permalinkAnchor: 'newroot'});

        // Someone else's message doesn't move the reader.
        act(() => {
            store.dispatch({type: PostTypes.RECEIVED_NEW_POST, data: TestHelper.getPostMock({id: 'theirs', type: '', channel_id: 'oncall', user_id: 'tom', create_at: 3000}), features: {crtEnabled: false}});
        });
        expect(probe()).toHaveAttribute('data-anchor', 'newroot');

        act(() => {
            store.dispatch({type: PostTypes.RECEIVED_NEW_POST, data: TestHelper.getPostMock({id: 'me:now', pending_post_id: 'me:now', type: '', channel_id: 'oncall', user_id: 'me', create_at: Date.now() + 1000}), features: {crtEnabled: false}});
        });
        expect(probe()).toHaveAttribute('data-anchor', '');
        expect(history.location.pathname).toBe('/lumen/channels/ops-oncall');
        expect(history.location.state).toBeUndefined();
    });

    test('an older message of the reader\'s, loaded with the newest ones, doesn\'t move the reader', () => {
        const {store} = setup('/lumen/channels/ops-oncall', {permalinkAnchor: 'newroot'});
        act(() => {
            store.dispatch({type: PostTypes.RECEIVED_NEW_POST, data: TestHelper.getPostMock({id: 'mine', type: '', channel_id: 'oncall', user_id: 'me', create_at: 2500}), features: {crtEnabled: false}});
        });
        expect(probe()).toHaveAttribute('data-anchor', 'newroot');
    });

    test('sending a message while the linked message is still highlighted brings back the newest messages too', () => {
        const {history, store} = setup('/lumen/channels/ops-oncall/newroot');
        act(() => {
            store.dispatch({type: PostTypes.RECEIVED_NEW_POST, data: TestHelper.getPostMock({id: 'me:now', pending_post_id: 'me:now', type: '', channel_id: 'oncall', user_id: 'me', create_at: Date.now() + 1000}), features: {crtEnabled: false}});
        });
        expect(history.location.pathname).toBe('/lumen/channels/ops-oncall');
        expect(probe()).toHaveAttribute('data-anchor', '');
    });

    test('Jump to present leaves the message: while it\'s highlighted, the address drops it', () => {
        const {history} = setup('/lumen/channels/ops-oncall/newroot');
        fireEvent.click(probe());
        expect(history.location.pathname).toBe('/lumen/channels/ops-oncall');
        expect(history.location.state).toBeUndefined();
        expect(probe()).toHaveAttribute('data-linked', '');
        expect(probe()).toHaveAttribute('data-anchor', '');
    });

    test('Jump to present leaves the message: once its highlight has ended, the address\'s state drops it', () => {
        const {history} = setup('/lumen/channels/ops-oncall', {permalinkAnchor: 'newroot'});
        fireEvent.click(probe());
        expect(history.location.pathname).toBe('/lumen/channels/ops-oncall');
        expect(history.location.state).toBeUndefined();
        expect(probe()).toHaveAttribute('data-anchor', '');
    });
});
