// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {createIntl} from 'react-intl';

import {Client4} from 'mattermost-redux/client';

import webSocketClient from 'client/web_websocket_client';
import {act, renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';

import SignInWarnings, {describeDevice} from './sign_in_warnings';
import type {SignIn} from './sign_in_warnings';

const mockToast = jest.fn();
jest.mock('fusion/shell/toast_context', () => ({useToast: () => mockToast}));

let mockListeners: Array<(message: unknown) => void> = [];
jest.mock('client/web_websocket_client', () => ({
    __esModule: true,
    default: {
        addMessageListener: jest.fn((listener) => mockListeners.push(listener)),
        removeMessageListener: jest.fn((listener) => {
            mockListeners = mockListeners.filter((l) => l !== listener);
        }),
    },
}));

const signIn = (id: string, extra: Partial<SignIn> = {}): SignIn => ({
    session_id: id,
    create_at: Date.now(),
    platform: 'Linux',
    os: 'Linux',
    browser: 'Firefox/131.0',
    mobile: false,
    ...extra,
});

const send = (event: string, data: unknown) => act(() => {
    mockListeners.forEach((listener) => listener({event, data}));
});

describe('fusion/shell/SignInWarnings', () => {
    const render = () => renderWithContext(
        <SignInWarnings/>,
        {
            entities: {
                users: {
                    currentUserId: 'me',
                    profiles: {me: {id: 'me', username: 'me', props: {}, timezone: {useAutomaticTimezone: 'false', manualTimezone: 'Europe/Paris', automaticTimezone: ''}}},
                },
                general: {config: {}},
            },
        } as never,
    );

    beforeEach(() => {
        jest.clearAllMocks();
        mockListeners = [];
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
        Client4.acknowledgeSession = jest.fn().mockResolvedValue({status: 'OK'});
        Client4.revokeSession = jest.fn().mockResolvedValue({status: 'OK'});
    });

    test('nothing until someone signs in; then the device, the browser and the time', async () => {
        render();
        await waitFor(() => expect(webSocketClient.addMessageListener).toHaveBeenCalled());
        expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();

        send('session_created', signIn('s1'));
        const warning = screen.getByRole('alertdialog', {name: 'New sign-in to your account: Firefox on Linux'});
        expect(warning).toHaveTextContent(/Firefox on Linux · \d{1,2}:\d{2}/);
        expect(warning).toHaveTextContent('Not you? Disconnect it, then change your password.');
        expect(screen.getByRole('button', {name: 'It\'s me'})).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Disconnect'})).toBeInTheDocument();
    });

    test('"It\'s me" dismisses it, and tells the server (for the other devices)', async () => {
        render();
        send('session_created', signIn('s1'));
        await userEvent.click(screen.getByRole('button', {name: 'It\'s me'}));

        expect(Client4.acknowledgeSession).toHaveBeenCalledWith('me', 's1');
        expect(Client4.revokeSession).not.toHaveBeenCalled();
        expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();
    });

    test('"Disconnect" revokes that session, then the warning goes', async () => {
        render();
        send('session_created', signIn('s1'));
        await userEvent.click(screen.getByRole('button', {name: 'Disconnect'}));

        expect(Client4.revokeSession).toHaveBeenCalledWith('me', 's1');
        expect(Client4.acknowledgeSession).not.toHaveBeenCalled();
        await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument());
        expect(mockToast).not.toHaveBeenCalled();
    });

    test('a failed disconnect keeps the warning, and says so', async () => {
        Client4.revokeSession = jest.fn().mockRejectedValue(new Error('offline'));
        render();
        send('session_created', signIn('s1'));
        await userEvent.click(screen.getByRole('button', {name: 'Disconnect'}));

        await waitFor(() => expect(mockToast).toHaveBeenCalledWith('Couldn\'t disconnect that session. Try again.'));
        expect(screen.getByRole('alertdialog')).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Disconnect'})).toBeEnabled();
    });

    test('several sign-ins stack, the newest on top, three at a time', () => {
        render();
        send('session_created', signIn('s1', {browser: 'Chrome/130.0', os: 'Windows 10'}));
        send('session_created', signIn('s2', {mobile: true, browser: 'Mobile App/2.30', os: 'iOS'}));
        send('session_created', signIn('s3', {browser: 'Desktop App/5.10', os: 'Mac OS'}));
        send('session_created', signIn('s3', {browser: 'Desktop App/5.10', os: 'Mac OS'}));

        expect(screen.getAllByRole('alertdialog').map((w) => w.getAttribute('aria-label'))).toEqual([
            'New sign-in to your account: Desktop app on Mac OS',
            'New sign-in to your account: Mobile app on iOS',
            'New sign-in to your account: Chrome on Windows 10',
        ]);
        expect(screen.queryByText(/more sign-in/)).not.toBeInTheDocument();

        send('session_created', signIn('s4'));
        expect(screen.getAllByRole('alertdialog')).toHaveLength(3);
        expect(screen.getAllByRole('alertdialog')[0]).toHaveAccessibleName('New sign-in to your account: Firefox on Linux');
        expect(screen.getByText('1 more sign-in')).toBeInTheDocument();
    });

    test('handled on another device or tab (revoked or confirmed): gone here too', () => {
        render();
        send('session_created', signIn('s1', {browser: 'Chrome/130.0', os: 'Windows 10'}));
        send('session_created', signIn('s2'));
        send('session_created', signIn('s3', {mobile: true, os: 'Android'}));

        send('session_revoked', {session_id: 's2'});
        expect(screen.getAllByRole('alertdialog').map((w) => w.getAttribute('aria-label'))).toEqual([
            'New sign-in to your account: Mobile app on Android',
            'New sign-in to your account: Chrome on Windows 10',
        ]);

        send('session_acknowledged', {session_id: 's3'});
        expect(screen.getAllByRole('alertdialog')).toHaveLength(1);

        // Another session revoked (a sign-out): nothing changes.
        send('session_revoked', {session_id: 'other'});
        expect(screen.getAllByRole('alertdialog')).toHaveLength(1);

        send('session_acknowledged', {session_id: 's1'});
        expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();
        expect(Client4.acknowledgeSession).not.toHaveBeenCalled();
        expect(Client4.revokeSession).not.toHaveBeenCalled();
    });

    test('an event without a session is ignored', () => {
        render();
        send('session_created', {});
        send('session_created', null);
        expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();
    });
});

describe('fusion/shell/describeDevice', () => {
    const intl = createIntl({locale: 'en', defaultLocale: 'en'});

    test.each([
        [{browser: 'Firefox/131.0', os: 'Linux'}, 'Firefox on Linux'],
        [{browser: 'Desktop App/5.10', os: 'Windows 10'}, 'Desktop app on Windows 10'],
        [{browser: 'Mobile App/2.30', os: 'iOS', mobile: true}, 'Mobile app on iOS'],
        [{browser: 'Unknown/0.0', os: 'Android', mobile: true}, 'Mobile app on Android'],
        [{browser: 'Safari/17.0', os: '', platform: 'Macintosh'}, 'Safari on Macintosh'],
        [{browser: 'Unknown/0.0', os: '', platform: 'Unknown'}, 'Unknown device'],
        [{browser: '', os: 'Linux'}, 'Linux'],
    ])('%j: %s', (extra, expected) => {
        expect(describeDevice(intl, signIn('s', extra as Partial<SignIn>))).toBe(expected);
    });
});
