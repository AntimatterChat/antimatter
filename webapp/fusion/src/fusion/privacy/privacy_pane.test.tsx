// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {deletePreferences, savePreferences} from 'mattermost-redux/actions/preferences';

import {renderWithContext, screen, userEvent, within} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import PrivacyPane from './privacy_pane';

jest.mock('mattermost-redux/actions/preferences', () => ({
    savePreferences: jest.fn(() => ({type: 'MOCK_SAVE_PREFERENCES'})),
    deletePreferences: jest.fn(() => ({type: 'MOCK_DELETE_PREFERENCES'})),
}));
jest.mock('mattermost-redux/actions/users', () => ({
    ...jest.requireActual('mattermost-redux/actions/users'),
    getMissingProfilesByIds: jest.fn(() => ({type: 'MOCK_MISSING_PROFILES'})),
}));
jest.mock('fusion/components/avatar', () => () => null);

const mockDialogs = {confirm: jest.fn()};
jest.mock('fusion/shell/dialogs_context', () => ({useDialogs: () => mockDialogs}));
jest.mock('fusion/shell/toast_context', () => ({useToast: () => jest.fn()}));

const me = 'me';
const pref = (category: string, name: string, value: string) => ({[`${category}--${name}`]: {user_id: me, category, name, value}});

function state(preferences: Record<string, unknown> = {}) {
    return {
        entities: {
            general: {config: {}},
            users: {
                currentUserId: me,
                profiles: Object.fromEntries([me, 'sofia', 'leo'].map((id) => [id, TestHelper.getUserMock({id, username: id, first_name: '', last_name: '', nickname: ''})])),
            },
            teams: {
                teams: {t1: TestHelper.getTeamMock({id: 't1', display_name: 'Lumen', delete_at: 0})},
                myMembers: {t1: {team_id: 't1', user_id: me, roles: 'team_user'}},
            },
            channels: {channels: {}, myMembers: {}},
            preferences: {myPreferences: preferences},
        },
    };
}

describe('fusion/privacy/PrivacyPane', () => {
    test('accepts direct messages from everyone by default, and asks first when chosen', async () => {
        renderWithContext(<PrivacyPane/>, state() as never);
        const choice = screen.getByRole('group', {name: 'From people you don\'t talk with yet'});
        expect(within(choice).getByRole('button', {name: 'Accept from everyone'})).toHaveAttribute('aria-pressed', 'true');

        await userEvent.click(within(choice).getByRole('button', {name: 'Ask me first'}));
        expect(savePreferences).toHaveBeenCalledWith(me, [{user_id: me, category: 'dm_privacy', name: 'default', value: 'ask'}]);
    });

    test('each community follows the choice above until it has its own', async () => {
        renderWithContext(<PrivacyPane/>, state(pref('dm_privacy', 'default', 'ask')) as never);
        const select = screen.getByRole('combobox', {name: 'Lumen'});
        expect(select).toHaveValue('');
        expect(within(select).getByRole('option', {name: 'As above (Ask me first)'})).toBeInTheDocument();

        await userEvent.selectOptions(select, 'everyone');
        expect(savePreferences).toHaveBeenCalledWith(me, [{user_id: me, category: 'dm_privacy', name: 't1', value: 'everyone'}]);
    });

    test('going back to "as above" forgets the community\'s choice', async () => {
        renderWithContext(<PrivacyPane/>, state(pref('dm_privacy', 't1', 'ask')) as never);
        await userEvent.selectOptions(screen.getByRole('combobox', {name: 'Lumen'}), '');
        expect(deletePreferences).toHaveBeenCalledWith(me, [{user_id: me, category: 'dm_privacy', name: 't1', value: ''}]);
    });

    test('says when nobody is blocked or ignored', () => {
        renderWithContext(<PrivacyPane/>, state() as never);
        expect(screen.getByText('You haven\'t blocked anyone.')).toBeInTheDocument();
        expect(screen.getByText('You aren\'t ignoring anyone.')).toBeInTheDocument();
    });

    test('lists the people blocked and ignored, each with the button that stops it', async () => {
        renderWithContext(<PrivacyPane/>, state({...pref('user_block', 'sofia', 'block'), ...pref('user_block', 'leo', 'ignore')}) as never);
        expect(screen.getByText('@sofia')).toBeInTheDocument();
        expect(screen.getByText('@leo')).toBeInTheDocument();

        await userEvent.click(screen.getByRole('button', {name: 'Stop ignoring'}));
        expect(deletePreferences).toHaveBeenCalledWith(me, [expect.objectContaining({category: 'user_block', name: 'leo'})]);

        // Unblocking doesn't ask; only blocking does.
        await userEvent.click(screen.getByRole('button', {name: 'Unblock'}));
        expect(deletePreferences).toHaveBeenCalledWith(me, [expect.objectContaining({category: 'user_block', name: 'sofia'})]);
        expect(mockDialogs.confirm).not.toHaveBeenCalled();
    });
});
