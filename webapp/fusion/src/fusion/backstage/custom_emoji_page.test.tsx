// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {createMemoryHistory} from 'history';
import React from 'react';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import CustomEmojiPage from './custom_emoji_page';

const mockHistory = {push: jest.fn()};
jest.mock('utils/browser_history', () => ({getHistory: () => mockHistory}));
jest.mock('mattermost-redux/actions/emojis', () => ({
    createCustomEmoji: jest.fn(),
    deleteCustomEmoji: jest.fn(() => ({type: 'MOCK_DELETE_EMOJI', data: true})),
    getCustomEmojiByName: jest.fn(() => ({type: 'MOCK_EMOJI_BY_NAME', error: {status_code: 404}})),
    getCustomEmojis: jest.fn(() => ({type: 'MOCK_GET_EMOJIS', data: []})),
    searchCustomEmojis: jest.fn(() => ({type: 'MOCK_SEARCH_EMOJIS', data: []})),
}));

const team = TestHelper.getTeamMock({id: 'team1', name: 'lab', display_name: 'Lumen Labs'});
const emoji = (id: string, name: string) => ({id, name, creator_id: 'me', create_at: 1, update_at: 1, delete_at: 0, category: 'custom'});

const stateWith = ({perms = ['create_emojis', 'delete_emojis'], enabled = true} = {}) => ({
    entities: {
        general: {config: {EnableCustomEmoji: enabled ? 'true' : 'false', SiteName: 'Antimatter'}},
        users: {currentUserId: 'me', profiles: {me: TestHelper.getUserMock({id: 'me', roles: 'system_user'})}},
        roles: {roles: {system_user: {id: 'r1', name: 'system_user', permissions: perms}}},
        teams: {currentTeamId: 'team1', teams: {team1: team}, myMembers: {}},
        emojis: {customEmoji: {e1: emoji('e1', 'party_parrot'), e2: emoji('e2', 'blobcat')}},
        preferences: {myPreferences: {}},
    },
});

describe('fusion/backstage/CustomEmojiPage', () => {
    beforeEach(() => {
        mockHistory.push.mockClear();
    });

    test('is the picker\'s manage view on a page, with a way to add one', async () => {
        renderWithContext(<CustomEmojiPage team={team}/>, stateWith() as never);
        expect(screen.getByRole('region', {name: 'Custom emoji'})).toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Back'})).not.toBeInTheDocument();
        expect(screen.getAllByRole('listitem')).toHaveLength(2);
        expect(document.title).toBe('Custom emoji - Lumen Labs Antimatter');

        await userEvent.click(screen.getByRole('button', {name: 'Add custom emoji'}));
        expect(mockHistory.push).toHaveBeenCalledWith('/lab/emoji/add');
    });

    test('/:team/emoji/add is the add view; Back goes to the list', async () => {
        renderWithContext(
            <CustomEmojiPage
                team={team}
                view='add'
            />,
            stateWith() as never,
        );
        expect(screen.getByRole('form', {name: 'Add a custom emoji'})).toBeInTheDocument();
        await userEvent.click(screen.getByRole('button', {name: 'Back'}));
        expect(mockHistory.push).toHaveBeenCalledWith('/lab/emoji');
    });

    test('Escape goes back from adding one', async () => {
        renderWithContext(
            <CustomEmojiPage
                team={team}
                view='add'
            />,
            stateWith() as never,
        );
        await userEvent.keyboard('{Escape}');
        expect(mockHistory.push).toHaveBeenCalledWith('/lab/emoji');
    });

    test('after adding one, the list is searched for it', () => {
        renderWithContext(
            <CustomEmojiPage team={team}/>,
            stateWith() as never,
            {history: createMemoryHistory({initialEntries: [{pathname: '/lab/emoji', state: {added: 'party_parrot'}}]})},
        );
        expect(screen.getByText(':party_parrot: is ready to use.')).toBeInTheDocument();
        expect(screen.getByRole('searchbox', {name: 'Search custom emoji'})).toHaveValue('party_parrot');
        expect(screen.getAllByRole('listitem')).toHaveLength(1);
    });

    test('without the permission to add, there\'s no add button and no add view', () => {
        renderWithContext(
            <CustomEmojiPage
                team={team}
                view='add'
            />,
            stateWith({perms: []}) as never,
        );
        expect(screen.queryByRole('form', {name: 'Add a custom emoji'})).not.toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Add custom emoji'})).not.toBeInTheDocument();
        expect(screen.queryByRole('button', {name: /^Delete/})).not.toBeInTheDocument();
    });

    test('says when custom emoji are off', () => {
        renderWithContext(<CustomEmojiPage team={team}/>, stateWith({enabled: false}) as never);
        expect(screen.getByText('Custom emoji are turned off on this server.')).toBeInTheDocument();
        expect(screen.queryByRole('region', {name: 'Custom emoji'})).not.toBeInTheDocument();
    });
});
