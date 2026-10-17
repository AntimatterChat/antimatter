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
jest.mock('fusion/shell/layout_context', () => ({useLayout: () => mockLayout, isPhoneLayout: () => false}));
jest.mock('fusion/sidebar/home_sidebar', () => ({DM_FIND_ID: 'am-dm-find'}));
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
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
        mockTeams = [mockTeam];
        mockUnread = [{channel: alice, mentions: 0, messages: 3}, {channel: bob, mentions: 2, messages: 5}];
    });

    test('starts with the chat bubble of all the direct messages, then the teams', () => {
        render();

        const buttons = screen.getAllByRole('button');
        expect(buttons[0]).toHaveAccessibleName('All direct messages, 8 unread messages');
        expect(buttons[1]).toHaveAccessibleName('Antimatter');
    });

    test('counts the messages waiting in the direct messages on the bubble', () => {
        render();

        const bubble = screen.getByRole('button', {name: /^All direct messages/});
        expect(bubble).toHaveAttribute('data-am-tip', 'All direct messages');
        expect(bubble.querySelector('.am-badge')).toHaveTextContent('8');
    });

    test('is a plain chat bubble without unread direct messages', () => {
        mockUnread = [];
        render();

        const bubble = screen.getByRole('button', {name: 'All direct messages'});
        expect(bubble.querySelector('use')).toHaveAttribute('href', '#am-i-chat');
        expect(bubble.querySelector('.am-badge')).not.toBeInTheDocument();
    });

    test('opens the list of the direct messages from the bubble, in the navigation drawer on a phone', async () => {
        render();

        await userEvent.click(screen.getByRole('button', {name: /^All direct messages/}));
        expect(mockLayout.setHome).toHaveBeenCalledWith(true);
        expect(mockLayout.setNavOpen).toHaveBeenCalledWith(true);
    });

    test('marks the bubble while the direct messages list is open', () => {
        mockLayout.home = true;
        render();
        mockLayout.home = false;

        expect(screen.getByRole('button', {name: /^All direct messages/})).toHaveAttribute('aria-current', 'page');
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

    const savedFolders = () => JSON.parse((savePreferences as jest.Mock).mock.calls.at(-1)[1][0].value);
    const workFolder = JSON.stringify([{id: 'f1', name: 'Work', teamIds: ['team', 'third']}]);

    test('a folder is renamed and coloured from its menu', async () => {
        mockTeams = [mockTeam, otherTeam, thirdTeam];
        render('town-square', workFolder);
        await userEvent.pointer({keys: '[MouseRight]', target: screen.getByRole('button', {name: 'Folder Work, 2 communities'})});
        await userEvent.click(screen.getByRole('menuitem', {name: /Folder settings/}));

        const dialog = screen.getByRole('dialog', {name: 'Folder settings'});
        const name = screen.getByPlaceholderText('Shown when you point at the folder');
        expect(name).toHaveValue('Work');
        expect(screen.getByRole('radio', {name: 'Theme colour'})).toHaveAttribute('aria-checked', 'true');
        await userEvent.clear(name);
        await userEvent.type(name, '  Clients ');
        await userEvent.click(screen.getByRole('radio', {name: '#ed4245'}));
        await userEvent.click(screen.getByRole('button', {name: 'Save'}));

        expect(savedFolders()).toEqual([{id: 'f1', name: 'Clients', color: '#ed4245', teamIds: ['team', 'third']}]);
        expect(dialog).not.toBeInTheDocument();
    });

    test('removing a folder keeps its communities in the rail', async () => {
        mockTeams = [mockTeam, otherTeam, thirdTeam];
        render('town-square', workFolder);
        await userEvent.pointer({keys: '[MouseRight]', target: screen.getByRole('button', {name: 'Folder Work, 2 communities'})});
        await userEvent.click(screen.getByRole('menuitem', {name: /Remove folder/}));
        expect(savedFolders()).toEqual([]);
    });

    test('a team dropped on a folder joins it', () => {
        mockTeams = [mockTeam, otherTeam, thirdTeam];
        render('town-square', workFolder);
        const data = {types: ['application/x-am-team'], setData: jest.fn(), effectAllowed: ''};
        fireEvent.dragStart(screen.getByRole('button', {name: 'Other'}), {dataTransfer: data});
        fireEvent.drop(screen.getByRole('button', {name: 'Folder Work, 2 communities'}), {dataTransfer: data});
        expect(savedFolders()).toEqual([{id: 'f1', name: 'Work', teamIds: ['team', 'third', 'other']}]);
    });

    test('an open folder stays open next time, in this browser', async () => {
        localStorage.removeItem('am-rail-open-folders');
        mockTeams = [mockTeam, otherTeam, thirdTeam];
        const {unmount} = render('town-square', workFolder);
        await userEvent.click(screen.getByRole('button', {name: 'Folder Work, 2 communities'}));
        unmount();

        render('town-square', workFolder);
        expect(screen.getByRole('button', {name: 'Folder Work, 2 communities'})).toHaveAttribute('aria-expanded', 'true');
        localStorage.removeItem('am-rail-open-folders');
    });

    test('shows no group when nothing waits', () => {
        mockUnread = [];
        render();

        expect(screen.queryByRole('group')).not.toBeInTheDocument();
    });
});
