// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {getSessions, revokeSession, updateMe, updateUserPassword} from 'mattermost-redux/actions/users';

import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import SettingsModal from './settings_modal';
import type {SettingsTab} from './settings_modal';

jest.mock('mattermost-redux/actions/users', () => ({
    ...jest.requireActual('mattermost-redux/actions/users'),
    getMe: jest.fn(() => () => Promise.resolve({data: {}})),
    getSessions: jest.fn(() => () => Promise.resolve({data: []})),
    revokeSession: jest.fn(() => () => Promise.resolve({data: true})),
    updateMe: jest.fn(() => () => Promise.resolve({data: {}})),
    updateUserPassword: jest.fn(() => () => Promise.resolve({data: true})),
}));

describe('fusion/modals/SettingsModal', () => {
    beforeEach(() => {
        jest.clearAllMocks();

        // The Fusion dialog is drawn in the Fusion layer.
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
    });

    const me = TestHelper.getUserMock({id: 'me', username: 'jason', first_name: 'Jason', nickname: '', auth_service: '', roles: 'system_user'});

    const render = (tab: SettingsTab, {user = me, config = {}, sessions = []}: {user?: typeof me; config?: Record<string, string>; sessions?: unknown[]} = {}) => renderWithContext(
        <SettingsModal
            tab={tab}
            onTab={jest.fn()}
            onClose={jest.fn()}
        />,
        {
            entities: {
                general: {config: {PasswordMinimumLength: '8', ...config}},
                users: {currentUserId: user.id, profiles: {[user.id]: user}, mySessions: sessions},
            },
        } as never,
    );

    test('saves the profile fields that changed', async () => {
        render('account');

        await userEvent.type(screen.getByLabelText(/^Nickname/), 'proto');
        await userEvent.click(screen.getByRole('button', {name: 'Save'}));

        expect(updateMe).toHaveBeenCalledWith(expect.objectContaining({nickname: 'proto', username: 'jason'}));
        expect(await screen.findByRole('button', {name: 'Saved'})).toBeInTheDocument();
    });

    test('keeps the name and username the admin locked for email accounts', () => {
        render('account', {config: {LockProfileFieldsForEmailUsers: 'name_and_username'}});

        expect(screen.getByLabelText(/^First name/)).toBeDisabled();
        expect(screen.getByLabelText(/^Username/)).toBeDisabled();
        expect(screen.getByLabelText(/^Nickname/)).toBeEnabled();
        expect(screen.getAllByText('Managed by your System Admin.')).toHaveLength(3);
    });

    test('leaves the username to the login provider', () => {
        render('account', {user: {...me, auth_service: 'ldap'}});

        expect(screen.getByLabelText(/^Username/)).toBeDisabled();
        expect(screen.getByText('Set by your login provider.')).toBeInTheDocument();
    });

    test('refuses a username that breaks the rules', async () => {
        render('account');

        const username = screen.getByLabelText(/^Username/);
        await userEvent.clear(username);
        await userEvent.type(username, '1bad name');

        expect(screen.getByRole('button', {name: 'Save'})).toBeDisabled();
    });

    test('changes the password once the new one is valid and confirmed', async () => {
        render('security');

        await userEvent.click(screen.getByRole('button', {name: 'Change…'}));
        await userEvent.type(screen.getByLabelText('Current password'), 'old-password');
        await userEvent.type(screen.getByLabelText(/^New password/), 'short');
        expect(screen.getByText(/must be 8-/)).toBeInTheDocument();

        await userEvent.type(screen.getByLabelText(/^New password/), '-but-longer');
        await userEvent.type(screen.getByLabelText(/^Confirm the new password/), 'short-but-long');
        expect(screen.getByText('The passwords don\'t match.')).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Change password'})).toBeDisabled();

        await userEvent.type(screen.getByLabelText(/^Confirm the new password/), 'er');
        await userEvent.click(screen.getByRole('button', {name: 'Change password'}));

        expect(updateUserPassword).toHaveBeenCalledWith('me', 'old-password', 'short-but-longer');
        expect(await screen.findByText('Your password was changed.')).toBeInTheDocument();
    });

    test('tells people who sign in elsewhere where their password is', () => {
        render('security', {user: {...me, auth_service: 'gitlab'}});

        expect(screen.getByText('You sign in through Gitlab: your password is managed there.')).toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Change…'})).not.toBeInTheDocument();
    });

    test('lists where you are signed in, without the access tokens, to sign out of them', async () => {
        render('security', {sessions: [
            {id: 'web', props: {browser: 'Firefox/140.0', os: 'Linux'}, last_activity_at: 2},
            {id: 'phone', device_id: 'android_rn-v2:abc', props: {}, last_activity_at: 1},
            {id: 'token', props: {type: 'UserAccessToken'}, last_activity_at: 3},
        ]});

        expect(getSessions).toHaveBeenCalledWith('me');
        expect(screen.getByText('Firefox/140.0 · Linux')).toBeInTheDocument();
        expect(screen.getByText('Mobile app')).toBeInTheDocument();
        expect(screen.getAllByRole('button', {name: 'Sign out'})).toHaveLength(2);

        await userEvent.click(screen.getAllByRole('button', {name: 'Sign out'})[0]);
        expect(revokeSession).toHaveBeenCalledWith('me', 'web');
        await waitFor(() => expect(getSessions).toHaveBeenCalledTimes(2));
    });
});
