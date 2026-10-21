// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {setFocusSummary} from 'fusion/focus/focus';
import {act, renderWithContext, screen, userEvent} from 'tests/react_testing_utils';
import {getHistory} from 'utils/browser_history';
import {TestHelper} from 'utils/test_helper';

import RailDirectMessages from './rail_dms';

const mockLayout = {home: false, setHome: jest.fn(), setNavOpen: jest.fn()};
let mockStill = false;
let mockDock: string[] = [];
const alice = TestHelper.getChannelMock({id: 'alice-dm', type: 'G', name: 'alice-dm', display_name: 'alice, bob'});
const carol = TestHelper.getChannelMock({id: 'carol-dm', type: 'G', name: 'carol-dm', display_name: 'carol, dan'});
let mockUnread = [{channel: alice, mentions: 0, messages: 3}, {channel: carol, mentions: 2, messages: 2}];

jest.mock('fusion/shell/layout_context', () => ({useLayout: () => mockLayout, isPhoneLayout: () => false}));
jest.mock('fusion/sidebar/home_sidebar', () => ({DM_FIND_ID: 'am-dm-find'}));
jest.mock('fusion/hooks/media', () => ({useReducedMotion: () => mockStill}));
jest.mock('fusion/sidebar/dock_channels', () => ({makeGetDockChannelIds: () => () => mockDock}));
jest.mock('fusion/selectors', () => ({getUnreadDirectChannels: () => mockUnread}));

describe('fusion/rail/RailDirectMessages', () => {
    const team = TestHelper.getTeamMock({id: 'team', name: 'antimatter'});
    const render = (currentChannelId = 'general') => renderWithContext(
        <RailDirectMessages/>,
        {
            entities: {
                teams: {currentTeamId: team.id, teams: {[team.id]: team}},
                channels: {currentChannelId, channels: {[alice.id]: alice, [carol.id]: carol}},
                users: {currentUserId: 'me'},
            },
        } as never,
    );

    beforeEach(() => {
        jest.clearAllMocks();
        mockLayout.home = false;
        mockStill = false;
        mockDock = [];
        mockUnread = [{channel: alice, mentions: 0, messages: 3}, {channel: carol, mentions: 2, messages: 2}];
    });

    test('shows the unread conversations as bubbles with their counts, under a small chat bubble', () => {
        const {container} = render();

        const group = screen.getByRole('group', {name: 'Unread direct messages'});
        expect(group).toHaveClass('am-stacked');
        expect(screen.getByRole('button', {name: 'alice, bob, 3 unread messages'})).toHaveTextContent('3');
        expect(screen.getByRole('button', {name: 'carol, dan, 2 unread messages'})).toHaveTextContent('2');

        const all = screen.getByRole('button', {name: 'All direct messages, 5 unread messages'});
        expect(all).toHaveClass('am-mini');
        expect(all).toHaveAttribute('data-am-tip', 'All direct messages');
        expect(container.querySelectorAll('.am-dm-srv')[0].previousElementSibling).toBe(all);
    });

    test('shows the unread conversations the dock has no room for, and leaves out those it shows', () => {
        mockDock = ['alice-dm'];
        render();

        expect(screen.queryByRole('button', {name: /^alice, bob/})).not.toBeInTheDocument();
        expect(screen.getByRole('button', {name: /^carol, dan/})).toBeInTheDocument();
    });

    test('shows none while the list of all the direct messages takes the dock\'s place: they are all in it', () => {
        mockLayout.home = true;
        const {container} = render();

        expect(container.querySelector('.am-dm-srv')).not.toBeInTheDocument();
        expect(screen.getByRole('button', {name: /^All direct messages/})).not.toHaveClass('am-mini');
    });

    test('lets a conversation go from the rail once read', () => {
        mockDock = ['carol-dm'];
        const {unmount} = render();
        expect(screen.getByRole('button', {name: /^alice, bob/})).toBeInTheDocument();
        unmount();

        // Read, and still too old for the dock: no bubble any more, and the chat bubble grows back.
        mockUnread = [{channel: carol, mentions: 2, messages: 2}];
        render();
        expect(screen.queryByRole('button', {name: /^alice, bob/})).not.toBeInTheDocument();
        expect(screen.getByRole('button', {name: /^All direct messages/})).not.toHaveClass('am-mini');
    });

    test('is a full rail bubble when nothing waits outside the dock', () => {
        mockDock = ['alice-dm', 'carol-dm'];
        const {container} = render();

        expect(screen.queryByRole('group', {name: 'Unread direct messages'})).not.toBeInTheDocument();
        expect(container.querySelector('.am-dm-srv')).not.toBeInTheDocument();
        const all = screen.getByRole('button', {name: 'All direct messages, 5 unread messages'});
        expect(all).not.toHaveClass('am-mini');
        expect(all.querySelector('use')).toHaveAttribute('href', '#am-i-chat');
        expect(all.querySelector('.am-badge')).not.toBeInTheDocument();
    });

    test('is named without a count when no direct message is unread', () => {
        mockUnread = [];
        render();

        expect(screen.getByRole('button', {name: 'All direct messages'})).not.toHaveClass('am-mini');
    });

    test('opens a waiting conversation from its bubble', async () => {
        render();

        await userEvent.click(screen.getByRole('button', {name: /^carol, dan/}));
        expect(mockLayout.setNavOpen).toHaveBeenCalledWith(false);
        expect(getHistory().push).toHaveBeenCalledWith('/antimatter/channels/carol-dm');
    });

    test('leaves out the conversation you have open', () => {
        render('alice-dm');

        expect(screen.queryByRole('button', {name: /^alice, bob/})).not.toBeInTheDocument();
    });

    test('opens the list of the direct messages, in the navigation drawer on a phone, and marks it while open', async () => {
        render();

        await userEvent.click(screen.getByRole('button', {name: /^All direct messages/}));
        expect(mockLayout.setHome).toHaveBeenCalledWith(true);
        expect(mockLayout.setNavOpen).toHaveBeenCalledWith(true);

        mockLayout.home = true;
        render();
        expect(screen.getAllByRole('button', {name: /^All direct messages/})[1]).toHaveAttribute('aria-current', 'page');
    });

    test('morphs between bubble and icon, except with reduced motion', () => {
        const {container} = render();
        expect(container.querySelector('.am-dm-dock')).not.toHaveClass('am-still');

        mockStill = true;
        const {container: still} = render();
        expect(still.querySelector('.am-dm-dock')).toHaveClass('am-still');
    });

    test('keeps no conversation waiting while you focus (your unread badges hide), and they come back after', () => {
        const focusing = (until: number) => renderWithContext(
            <RailDirectMessages/>,
            {
                entities: {
                    teams: {currentTeamId: team.id, teams: {[team.id]: team}},
                    channels: {currentChannelId: 'general', channels: {[alice.id]: alice, [carol.id]: carol}},
                    users: {currentUserId: 'me', profiles: {me: {id: 'me', username: 'me', props: until ? {focus_until: String(until)} : {}}}},
                },
            } as never,
        );

        const {container, unmount} = focusing(Date.now() + 36e5);
        expect(container.querySelector('.am-dm-srv')).not.toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'All direct messages'})).not.toHaveClass('am-mini');
        unmount();

        // Ended, the summary still open: still hidden; once it's left, back.
        act(() => setFocusSummary({start_at: Date.now() - 36e5, end_at: Date.now()}));
        const after = focusing(0);
        expect(after.container.querySelector('.am-dm-srv')).not.toBeInTheDocument();
        act(() => setFocusSummary(null));
        expect(after.container.querySelectorAll('.am-dm-srv')).toHaveLength(2);
        expect(screen.getByRole('button', {name: 'All direct messages, 5 unread messages'})).toBeInTheDocument();
    });
});
