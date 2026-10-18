// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {act, render} from '@testing-library/react';
import {createMemoryHistory} from 'history';
import React from 'react';
import {Router} from 'react-router-dom';

import CenterChannel from './center_channel';

import type {OwnProps} from './index';

// The conversation's router: a stand-in that counts its mounts and says what it was given.
const mockRouterMounted = jest.fn();
const mockRouterRendered = jest.fn();
jest.mock('components/channel_layout/channel_identifier_router', () => {
    const {useEffect} = jest.requireActual('react');
    const {useRouteMatch} = jest.requireActual('react-router-dom');
    function MockChannelIdentifierRouter(props: {returnTo?: string}) {
        const match = useRouteMatch();
        mockRouterRendered({returnTo: props.returnTo, params: match.params});
        useEffect(() => mockRouterMounted(), []);
        return null;
    }
    return {__esModule: true, default: MockChannelIdentifierRouter};
});

describe('components/channel_layout/CenterChannel', () => {
    const props = {
        location: {
            pathname: '/some',
        } as OwnProps['location'],
        match: {
            url: '/url',
        } as OwnProps['match'],
        history: {} as OwnProps['history'],
        lastChannelPath: '',
        lhsOpen: true,
        rhsOpen: true,
        rhsMenuOpen: true,
        isCollapsedThreadsEnabled: true,
        currentUserId: 'testUserId',
        isMobileView: false,
        actions: {
            getProfiles: jest.fn(),
        },
    };
    test('should call update returnTo on props change', () => {
        let state = {returnTo: '', lastReturnTo: ''};

        // Simulate initial render with pathname '/some'
        let derived = CenterChannel.getDerivedStateFromProps(props as any, state);
        state = {...state, ...derived};
        expect(state.returnTo).toBe('');

        // Simulate props change to '/pl/path'
        derived = CenterChannel.getDerivedStateFromProps(
            {...props, location: {pathname: '/pl/path'}} as any,
            state,
        );
        state = {...state, ...derived};
        expect(state.returnTo).toBe('/some');

        // Simulate props change to '/pl/path1'
        derived = CenterChannel.getDerivedStateFromProps(
            {...props, location: {pathname: '/pl/path1'}} as any,
            state,
        );
        state = {...state, ...derived};
        expect(state.returnTo).toBe('/pl/path');
    });

    test('a permalink and the conversation it opens are one route: the conversation view is never unmounted between them', () => {
        const postId = 'newrootnewrootnewrootnewro';
        const history = createMemoryHistory({initialEntries: ['/lumen/channels/town']});
        const view = (
            <Router history={history}>
                <CenterChannel
                    {...props}
                    location={history.location as OwnProps['location']}
                    match={{url: '/lumen', params: {team: 'lumen'}} as OwnProps['match']}
                    lastChannelPath='/lumen/channels/town'
                />
            </Router>
        );
        render(view);
        expect(mockRouterRendered).toHaveBeenLastCalledWith(expect.objectContaining({params: expect.objectContaining({identifier: 'town'})}));

        // The moved thread's link, or any link to a message.
        act(() => history.push(`/lumen/pl/${postId}`));
        expect(mockRouterRendered).toHaveBeenLastCalledWith(expect.objectContaining({params: {postid: postId}}));

        // focusPost found the post's conversation.
        act(() => history.replace(`/lumen/channels/ops-oncall/${postId}`));
        expect(mockRouterRendered).toHaveBeenLastCalledWith(expect.objectContaining({params: expect.objectContaining({identifier: 'ops-oncall', postid: postId})}));

        expect(mockRouterMounted).toHaveBeenCalledTimes(1);
    });
});
