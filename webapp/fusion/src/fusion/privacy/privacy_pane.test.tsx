// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {deletePreferences, savePreferences} from 'mattermost-redux/actions/preferences';
import {updateMe} from 'mattermost-redux/actions/users';

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
    updateMe: jest.fn(() => ({type: 'MOCK_UPDATE_ME'})),
}));
jest.mock('fusion/components/avatar', () => () => null);

const mockDialogs = {confirm: jest.fn()};
jest.mock('fusion/shell/dialogs_context', () => ({useDialogs: () => mockDialogs}));
jest.mock('fusion/shell/toast_context', () => ({useToast: () => jest.fn()}));

const me = 'me';
const pref = (category: string, name: string, value: string) => ({[`${category}--${name}`]: {user_id: me, category, name, value}});

function state(preferences: Record<string, unknown> = {}, {config = {}, props = {}}: {config?: Record<string, string>; props?: Record<string, string>} = {}) {
    return {
        entities: {
            general: {config},
            users: {
                currentUserId: me,
                profiles: Object.fromEntries([me, 'sofia', 'leo'].map((id) => [id, TestHelper.getUserMock({id, username: id, first_name: '', last_name: '', nickname: '', props: id === me ? props : {}})])),
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
    beforeEach(() => {
        jest.clearAllMocks();
    });

    test('accepts direct messages from everyone by default, and asks first or refuses when chosen', async () => {
        renderWithContext(<PrivacyPane/>, state() as never);
        const choice = screen.getByRole('group', {name: 'From people you don\'t talk with yet'});
        expect(within(choice).getByRole('button', {name: 'Accept from everyone'})).toHaveAttribute('aria-pressed', 'true');

        await userEvent.click(within(choice).getByRole('button', {name: 'Ask me first'}));
        expect(savePreferences).toHaveBeenCalledWith(me, [{user_id: me, category: 'dm_privacy', name: 'default', value: 'ask'}]);

        await userEvent.click(within(choice).getByRole('button', {name: 'Refuse'}));
        expect(savePreferences).toHaveBeenCalledWith(me, [{user_id: me, category: 'dm_privacy', name: 'default', value: 'refuse'}]);
    });

    test('shows the defaults only: each community\'s own choices are in its menu', () => {
        renderWithContext(<PrivacyPane/>, state(pref('dm_privacy', 't1', 'ask')) as never);
        expect(screen.queryByRole('combobox', {name: 'Lumen'})).not.toBeInTheDocument();
        expect(screen.queryByText('Lumen')).not.toBeInTheDocument();
    });

    test('the profile details shown by default: hiding one saves it for all communities', async () => {
        renderWithContext(<PrivacyPane/>, state({}, {config: {ShowEmailAddress: 'true'}}) as never);
        const position = screen.getByRole('switch', {name: 'Position'});
        expect(position).toHaveAttribute('aria-checked', 'true');
        expect(screen.getByRole('switch', {name: 'Email address'})).toBeInTheDocument();

        await userEvent.click(position);
        expect(updateMe).toHaveBeenCalledWith({props: {profile_privacy: JSON.stringify({default: ['position']})}});
    });

    test('showing everything again drops the choice, and keeps the other communities\' ones', async () => {
        const props = {profile_privacy: JSON.stringify({default: ['position'], t1: ['email']}), other: 'kept'};
        renderWithContext(<PrivacyPane/>, state({}, {props}) as never);
        const position = screen.getByRole('switch', {name: 'Position'});
        expect(position).toHaveAttribute('aria-checked', 'false');

        await userEvent.click(position);
        expect(updateMe).toHaveBeenCalledWith({props: {profile_privacy: JSON.stringify({t1: ['email']}), other: 'kept'}});
    });

    test('the email is offered only when the server shows emails', () => {
        renderWithContext(<PrivacyPane/>, state({}, {config: {ShowEmailAddress: 'false'}}) as never);
        expect(screen.queryByRole('switch', {name: 'Email address'})).not.toBeInTheDocument();
        expect(screen.getByRole('switch', {name: 'Full name'})).toBeInTheDocument();
    });

    test('while the admin turned blocking off, only the profile details', () => {
        renderWithContext(<PrivacyPane/>, state({}, {config: {EnableUserBlocking: 'false'}}) as never);
        expect(screen.queryByRole('group', {name: 'From people you don\'t talk with yet'})).not.toBeInTheDocument();
        expect(screen.queryByText('Blocked')).not.toBeInTheDocument();
        expect(screen.getByRole('switch', {name: 'Nickname'})).toBeInTheDocument();
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
