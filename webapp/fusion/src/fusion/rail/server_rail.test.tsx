// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {savePreferences} from 'mattermost-redux/actions/preferences';

import {updateTeamsOrderForUser} from 'actions/team_actions';

import {fireEvent, renderWithContext, screen, userEvent} from 'tests/react_testing_utils';
import {getHistory} from 'utils/browser_history';
import {TestHelper} from 'utils/test_helper';

import ServerRail from './server_rail';

const mockLayout = {home: false, setHome: jest.fn(), setNavOpen: jest.fn()};
jest.mock('fusion/shell/layout_context', () => ({useLayout: () => mockLayout}));
jest.mock('actions/team_actions', () => ({switchTeam: jest.fn(() => ({type: 'SWITCH'})), updateTeamsOrderForUser: jest.fn(() => ({type: 'ORDER'}))}));
jest.mock('fusion/shell/global_search_context', () => ({useGlobalSearch: () => ({open: jest.fn()})}));
jest.mock('mattermost-redux/actions/preferences', () => ({savePreferences: jest.fn(() => ({type: 'PREFS'}))}));

const mockTeam = TestHelper.getTeamMock({id: 'team', name: 'antimatter', display_name: 'Antimatter'});
const alice = TestHelper.getChannelMock({id: 'alice-dm', type: 'G', display_name: 'alice, bob'});
const bob = TestHelper.getChannelMock({id: 'bob-dm', type: 'G', display_name: 'bob, carol'});
const otherTeam = TestHelper.getTeamMock({id: 'other', name: 'other', display_name: 'Other'});
const thirdTeam = TestHelper.getTeamMock({id: 'third', name: 'third', display_name: 'Third'});
let mockTeams = [mockTeam];
let mockUnread = [{channel: alice, mentions: 0, messages: 3}, {channel: bob, mentions: 2, messages: 5}];
jest.mock('fusion/selectors', () => ({
    getSortedMyTeams: () => mockTeams,
    getUnreadDirectChannels: () => mockUnread,
    getLastTeamChannelName: () => 'off-topic',
}));

describe('fusion/rail/ServerRail', () => {
    const render = (currentChannelId = 'town-square', folders?: string) => renderWithContext(
        <ServerRail/>,
        {
            entities: {
                teams: {currentTeamId: mockTeam.id, teams: Object.fromEntries(mockTeams.map((t) => [t.id, t])), myMembers: Object.fromEntries(mockTeams.map((t) => [t.id, {team_id: t.id}]))},
                channels: {currentChannelId, channels: {[alice.id]: alice, [bob.id]: bob, 'town-square': TestHelper.getChannelMock({id: 'town-square', type: 'O'})}},
                users: {currentUserId: 'me'},
                preferences: {myPreferences: folders ? {'fusion_rail--folders': {user_id: 'me', category: 'fusion_rail', name: 'folders', value: folders}} : {}},
            },
        } as never,
    );

    beforeEach(() => {
        jest.clearAllMocks();
        mockTeams = [mockTeam];
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

    test('reorders the teams by dragging one onto another', () => {
        render();
        const team = screen.getByRole('button', {name: 'Antimatter'});
        const data = {types: ['application/x-am-team'], setData: jest.fn(), effectAllowed: ''};
        fireEvent.dragStart(team, {dataTransfer: data});
        fireEvent.drop(team, {dataTransfer: data, clientY: 0});

        // Dropping a team on itself changes nothing.
        expect(updateTeamsOrderForUser).not.toHaveBeenCalled();
    });

    test('dropping a team on the middle of another makes a folder of the two', () => {
        mockTeams = [mockTeam, otherTeam];
        render();
        jest.spyOn(Element.prototype, 'getBoundingClientRect').mockReturnValue({top: 0, height: 40, bottom: 40, left: 0, right: 46, width: 46, x: 0, y: 0, toJSON: () => ({})});
        const data = {types: ['application/x-am-team'], setData: jest.fn(), effectAllowed: ''};
        fireEvent.dragStart(screen.getByRole('button', {name: 'Other'}), {dataTransfer: data});
        fireEvent.drop(screen.getByRole('button', {name: 'Antimatter'}), {dataTransfer: data, clientY: 20});

        expect(updateTeamsOrderForUser).toHaveBeenCalledWith(['team', 'other']);
        const saved = (savePreferences as jest.Mock).mock.calls[0][1][0];
        expect(saved).toMatchObject({category: 'fusion_rail', name: 'folders'});
        expect(JSON.parse(saved.value)).toEqual([expect.objectContaining({teamIds: ['team', 'other']})]);
        jest.restoreAllMocks();
    });

    test('a folder shows its name on hover, and opens in place', async () => {
        mockTeams = [mockTeam, otherTeam, thirdTeam];
        render('town-square', JSON.stringify([{id: 'f1', name: 'Work', color: '#3ba55c', teamIds: ['team', 'third']}]));

        const folder = screen.getByRole('button', {name: 'Folder Work, 2 communities'});
        expect(folder).toHaveAttribute('data-am-tip', 'Work');
        expect(folder).toHaveAttribute('aria-expanded', 'false');
        expect(screen.queryByRole('button', {name: 'Third'})).not.toBeInTheDocument();

        await userEvent.click(folder);
        expect(folder).toHaveAttribute('aria-expanded', 'true');
        expect(screen.getByRole('group', {name: 'Work'})).toContainElement(screen.getByRole('button', {name: 'Third'}));
    });

    test('an unnamed folder is called after its communities', () => {
        mockTeams = [mockTeam, otherTeam];
        render('town-square', JSON.stringify([{id: 'f1', name: '', teamIds: ['team', 'other']}]));
        expect(screen.getByRole('button', {name: /^Folder Antimatter, Other/})).toHaveAttribute('data-am-tip', 'Antimatter, Other');
    });

    test('shows no group when nothing waits', () => {
        mockUnread = [];
        render();

        expect(screen.queryByRole('group')).not.toBeInTheDocument();
    });
});
