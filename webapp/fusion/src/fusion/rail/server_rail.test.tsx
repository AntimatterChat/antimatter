// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';
import {getHistory} from 'utils/browser_history';
import {TestHelper} from 'utils/test_helper';

import ServerRail from './server_rail';

const mockLayout = {home: false, setHome: jest.fn(), setNavOpen: jest.fn()};
jest.mock('fusion/shell/layout_context', () => ({useLayout: () => mockLayout}));
jest.mock('fusion/shell/global_search_context', () => ({useGlobalSearch: () => ({open: jest.fn()})}));

const mockTeam = TestHelper.getTeamMock({id: 'team', name: 'antimatter', display_name: 'Antimatter'});
const alice = TestHelper.getChannelMock({id: 'alice-dm', type: 'G', display_name: 'alice, bob'});
const bob = TestHelper.getChannelMock({id: 'bob-dm', type: 'G', display_name: 'bob, carol'});
const mockTeams = [mockTeam];
let mockUnread = [{channel: alice, mentions: 0, messages: 3}, {channel: bob, mentions: 2, messages: 5}];
jest.mock('fusion/selectors', () => ({
    getSortedMyTeams: () => mockTeams,
    getUnreadDirectChannels: () => mockUnread,
    getLastTeamChannelName: () => 'off-topic',
}));

describe('fusion/rail/ServerRail', () => {
    const render = (currentChannelId = 'town-square') => renderWithContext(
        <ServerRail/>,
        {
            entities: {
                teams: {currentTeamId: mockTeam.id, teams: {[mockTeam.id]: mockTeam}, myMembers: {[mockTeam.id]: {team_id: mockTeam.id}}},
                channels: {currentChannelId, channels: {[alice.id]: alice, [bob.id]: bob, 'town-square': TestHelper.getChannelMock({id: 'town-square', type: 'O'})}},
            },
        } as never,
    );

    beforeEach(() => {
        jest.clearAllMocks();
        mockUnread = [{channel: alice, mentions: 0, messages: 3}, {channel: bob, mentions: 2, messages: 5}];
    });

    test('starts with the teams, without the home logo', () => {
        render();

        const buttons = screen.getAllByRole('button');
        expect(buttons[0]).toHaveAccessibleName('Antimatter');
        expect(screen.queryByRole('button', {name: 'Direct messages'})).not.toBeInTheDocument();
    });

    test('lists the direct messages waiting for you at the bottom, above search', () => {
        render();

        const group = screen.getByRole('group', {name: 'Unread direct messages'});
        expect(screen.getByRole('button', {name: 'alice, bob, 3 unread messages'})).toHaveTextContent('3');
        expect(screen.getByRole('button', {name: 'bob, carol, 2 unread messages'})).toHaveTextContent('2');
        expect(group.nextElementSibling).toHaveAccessibleName('Search everything');
    });

    test('leaves out the conversation you have open', async () => {
        render('alice-dm');

        expect(screen.queryByRole('button', {name: /^alice, bob/})).not.toBeInTheDocument();
        await userEvent.click(screen.getByRole('button', {name: /^bob, carol/}));
        expect(mockLayout.setNavOpen).toHaveBeenCalledWith(false);
    });

    test('selects the current team, but not while you are in a direct message', async () => {
        render();
        expect(screen.getByRole('button', {name: 'Antimatter'})).toHaveAttribute('aria-current', 'page');

        render('alice-dm');
        const teams = screen.getAllByRole('button', {name: 'Antimatter'});
        expect(teams[1]).not.toHaveAttribute('aria-current');

        // The team takes you back to its channel you were last in.
        await userEvent.click(teams[1]);
        expect(getHistory().push).toHaveBeenCalledWith('/antimatter/channels/off-topic');
    });

    test('selects no team while the direct messages list is open', () => {
        mockLayout.home = true;
        render();
        mockLayout.home = false;

        expect(screen.getByRole('button', {name: 'Antimatter'})).not.toHaveAttribute('aria-current');
    });

    test('shows no group when nothing waits', () => {
        mockUnread = [];
        render();

        expect(screen.queryByRole('group')).not.toBeInTheDocument();
    });
});
