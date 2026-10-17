// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {render} from '@testing-library/react';
import type {History} from 'history';
import React from 'react';

import {getHistory} from 'utils/browser_history';

import ChannelIdentifierRouter from './channel_identifier_router';

// The Fusion UI renders its own conversation view: here, a stand-in that counts its mounts.
const mockViewMounted = jest.fn();
const mockViewRendered = jest.fn();
jest.mock('fusion/channel/channel_view', () => {
    const {useEffect} = jest.requireActual('react');
    return {
        __esModule: true,
        default: (props: {resolvingPermalink?: boolean}) => {
            mockViewRendered(props);
            useEffect(() => mockViewMounted(), []);
            return null;
        },
    };
});

const mockReplace = jest.fn();
jest.mock('utils/browser_history', () => ({
    getHistory: () => ({replace: mockReplace}),
}));

jest.useFakeTimers({legacyFakeTimers: true});

describe('components/channel_layout/ChannelIdentifierRouter', () => {
    const baseProps = {
        match: {
            isExact: false,
            params: {
                identifier: 'identifier',
                team: 'team',
                path: '/path',
            },
            path: '/team/channel/identifier',
            url: '/team/channel/identifier',
        },
        actions: {
            onChannelByIdentifierEnter: jest.fn(),
            focusPost: jest.fn(),
        },
        history: [] as unknown as History,
    };

    beforeEach(() => {
        jest.clearAllMocks();
    });

    test('should call onChannelByIdentifierEnter on props change', () => {
        const {rerender} = render(<ChannelIdentifierRouter {...baseProps}/>);
        expect(baseProps.actions.onChannelByIdentifierEnter).toHaveBeenCalledTimes(1);
        expect(baseProps.actions.onChannelByIdentifierEnter).toHaveBeenLastCalledWith(baseProps);

        const props2 = {
            ...baseProps,
            match: {
                isExact: false,
                params: {
                    identifier: 'identifier2',
                    team: 'team2',
                    path: '/path2',
                },
                path: '/team2/channel/identifier2',
                url: '/team2/channel/identifier2',
            },
        };
        rerender(<ChannelIdentifierRouter {...props2}/>);

        expect(clearTimeout).toHaveBeenCalled();
        expect(baseProps.actions.onChannelByIdentifierEnter).toHaveBeenCalledTimes(2);
        expect(baseProps.actions.onChannelByIdentifierEnter).toHaveBeenLastCalledWith(props2);
    });

    test('should call browserHistory.replace if it is permalink after timer', () => {
        const props = {
            ...baseProps,
            match: {
                isExact: false,
                params: {
                    identifier: 'identifier',
                    team: 'team',
                    path: '/path',
                    postid: 'abcd',
                },
                path: '/team/channel/identifier/abcd',
                url: '/team/channel/identifier/abcd',
            },
        };
        render(<ChannelIdentifierRouter {...props}/>);
        jest.runOnlyPendingTimers();
        expect(getHistory().replace).toHaveBeenLastCalledWith('/team/channel/identifier', {permalinkAnchor: 'abcd'});
    });

    test('should call browserHistory.replace on props change to permalink', () => {
        const props = {
            ...baseProps,
            match: {
                isExact: false,
                params: {
                    identifier: 'identifier1',
                    team: 'team1',
                    path: '/path1',
                    postid: 'abcd',
                },
                path: '/team1/channel/identifier1/abcd',
                url: '/team1/channel/identifier1/abcd',
            },
        };

        const {rerender} = render(<ChannelIdentifierRouter {...baseProps}/>);
        rerender(<ChannelIdentifierRouter {...props}/>);

        jest.runOnlyPendingTimers();
        expect(getHistory().replace).toHaveBeenLastCalledWith('/team1/channel/identifier1', {permalinkAnchor: 'abcd'});
    });

    // /<team>/pl/<post id>: the same route and component as the conversations (see CenterChannel).
    const permalink = (postid: string) => ({
        ...baseProps,
        match: {isExact: true, params: {postid} as never, path: '/team/pl/:postid', url: `/team/pl/${postid}`},
        returnTo: '/team/channels/town',
        currentUserId: 'me',
    });
    const conversation = (identifier: string, postid?: string) => ({
        ...baseProps,
        match: {
            isExact: true,
            params: {identifier, team: 'team', path: 'channels', postid},
            path: '/:team/:path/:identifier/:postid?',
            url: `/team/channels/${identifier}${postid ? '/' + postid : ''}`,
        },
    });

    test('a permalink resolves in place: its post is focused, without looking up a channel or changing the address', () => {
        render(<ChannelIdentifierRouter {...permalink('newroot')}/>);
        expect(baseProps.actions.focusPost).toHaveBeenCalledWith('newroot', '/team/channels/town', 'me');
        expect(baseProps.actions.onChannelByIdentifierEnter).not.toHaveBeenCalled();
        expect(mockViewRendered).toHaveBeenLastCalledWith({resolvingPermalink: true});

        jest.runOnlyPendingTimers();
        expect(mockReplace).not.toHaveBeenCalled();
    });

    test('the conversation view stays mounted from a conversation, through a permalink, to the linked conversation', () => {
        const {rerender} = render(<ChannelIdentifierRouter {...conversation('town')}/>);
        expect(mockViewRendered).toHaveBeenLastCalledWith({resolvingPermalink: false});

        rerender(<ChannelIdentifierRouter {...permalink('newroot')}/>);
        expect(baseProps.actions.focusPost).toHaveBeenCalledTimes(1);
        expect(mockViewRendered).toHaveBeenLastCalledWith({resolvingPermalink: true});

        // focusPost replaces the address with the post's conversation, the post in it.
        rerender(<ChannelIdentifierRouter {...conversation('ops-oncall', 'newroot')}/>);
        expect(baseProps.actions.onChannelByIdentifierEnter).toHaveBeenLastCalledWith(conversation('ops-oncall', 'newroot'));
        expect(mockViewRendered).toHaveBeenLastCalledWith({resolvingPermalink: false});
        expect(mockViewMounted).toHaveBeenCalledTimes(1);

        // The highlight ends; the list stays around the post.
        jest.runOnlyPendingTimers();
        expect(mockReplace).toHaveBeenLastCalledWith('/team/channels/ops-oncall', {permalinkAnchor: 'newroot'});
    });

    test('a highlight fading in the conversation shown doesn\'t send the reader back there while a permalink opens', () => {
        const {rerender} = render(<ChannelIdentifierRouter {...conversation('town', 'oldpost')}/>);
        rerender(<ChannelIdentifierRouter {...permalink('newroot')}/>);
        jest.runOnlyPendingTimers();
        expect(mockReplace).not.toHaveBeenCalled();
    });

    test('another permalink while one resolves is focused too', () => {
        const {rerender} = render(<ChannelIdentifierRouter {...permalink('first')}/>);
        rerender(<ChannelIdentifierRouter {...permalink('second')}/>);
        expect(baseProps.actions.focusPost).toHaveBeenCalledTimes(2);
        expect(baseProps.actions.focusPost).toHaveBeenLastCalledWith('second', '/team/channels/town', 'me');
    });
});
