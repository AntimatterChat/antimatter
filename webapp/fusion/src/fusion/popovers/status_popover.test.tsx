// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {Client4} from 'mattermost-redux/client';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import StatusPopover from './status_popover';

jest.mock('fusion/shell/settings_context', () => ({useSettings: () => ({open: jest.fn()})}));
jest.mock('fusion/shell/toast_context', () => ({useToast: () => jest.fn()}));
jest.mock('components/plugin_marketplace/marketplace_modal', () => () => null);
jest.mock('fusion/popovers/emoji_picker', () => () => null);
const mockDialogs = {focus: jest.fn()};
jest.mock('fusion/shell/dialogs_context', () => ({useDialogs: () => mockDialogs}));

// Your menu (from your avatar): a community's integrations aren't there any more (they're in its settings); the System
// Console still is, for those who may see part of it.
describe('fusion/popovers/StatusPopover links', () => {
    beforeEach(() => {
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
    });

    const render = (roles: string) => renderWithContext(
        <StatusPopover
            anchor={null}
            onClose={jest.fn()}
        />,
        {
            entities: {
                teams: {currentTeamId: 't1', teams: {t1: {id: 't1', name: 'one', display_name: 'One', delete_at: 0}}, myMembers: {t1: {team_id: 't1', user_id: 'me', roles: 'team_user'}}},
                users: {currentUserId: 'me', profiles: {me: {id: 'me', username: 'me', roles, notify_props: {}}}},
                roles: {roles: {system_user: {name: 'system_user', permissions: []}, system_admin: {name: 'system_admin', permissions: ['manage_system']}, team_user: {name: 'team_user', permissions: []}}},
            },
        } as never,
    );

    test('a system admin has the System Console, and no Integrations', () => {
        render('system_user system_admin');
        expect(screen.getByRole('button', {name: 'System Console'})).toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Integrations'})).not.toBeInTheDocument();
    });

    test('a member has neither', () => {
        render('system_user');
        expect(screen.getByRole('button', {name: 'Profile & settings'})).toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'System Console'})).not.toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Integrations'})).not.toBeInTheDocument();
    });
});

// Focus mode starts and ends from your menu, and nowhere else: "Focus mode…" while you don't focus, "End focus" with
// the time left while you do.
describe('fusion/popovers/StatusPopover focus mode', () => {
    const NOW = new Date('2026-10-21T12:00:00Z').getTime();

    beforeEach(() => {
        jest.useFakeTimers({advanceTimers: true}).setSystemTime(NOW);
        jest.clearAllMocks();
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
        Client4.endFocus = jest.fn().mockResolvedValue({active: false});
    });
    afterEach(() => {
        jest.useRealTimers();
    });

    const render = (focusUntil = 0) => {
        const onClose = jest.fn();
        renderWithContext(
            <StatusPopover
                anchor={null}
                onClose={onClose}
            />,
            {
                entities: {
                    teams: {currentTeamId: 't1', teams: {t1: {id: 't1', name: 'one', display_name: 'One', delete_at: 0}}, myMembers: {t1: {team_id: 't1', user_id: 'me', roles: 'team_user'}}},
                    users: {
                        currentUserId: 'me',
                        profiles: {me: {id: 'me', username: 'me', roles: 'system_user', notify_props: {}, props: focusUntil ? {focus_until: String(focusUntil)} : {}}},
                        statuses: {me: focusUntil ? 'dnd' : 'online'},
                    },
                    roles: {roles: {system_user: {name: 'system_user', permissions: []}, team_user: {name: 'team_user', permissions: []}}},
                },
            } as never,
        );
        return onClose;
    };

    test('"Focus mode…" opens the dialog to start it', async () => {
        const onClose = render();
        const item = screen.getByRole('button', {name: /^Focus mode…/});
        expect(item).toHaveAttribute('aria-pressed', 'false');
        expect(screen.queryByRole('button', {name: /^End focus/})).not.toBeInTheDocument();

        await userEvent.click(item);
        expect(mockDialogs.focus).toHaveBeenCalled();
        expect(onClose).toHaveBeenCalled();
    });

    test('while focusing: "End focus", the time left, and Do not disturb not checked', async () => {
        const onClose = render(NOW + (95 * 60000));
        const item = screen.getByRole('button', {name: /^End focus/});
        expect(item).toHaveAttribute('aria-pressed', 'true');
        expect(item).toHaveTextContent('1 h 35 min left · until');
        expect(screen.queryByRole('button', {name: /^Focus mode…/})).not.toBeInTheDocument();
        expect(screen.getByRole('button', {name: /^Do not disturb/})).toHaveAttribute('aria-pressed', 'false');
        expect(screen.getByText(/^@me · Focusing · until /)).toBeInTheDocument();

        await userEvent.click(item);
        expect(Client4.endFocus).toHaveBeenCalled();
        expect(mockDialogs.focus).not.toHaveBeenCalled();
        expect(onClose).toHaveBeenCalled();
    });

    test('under an hour, the minutes left', () => {
        render(NOW + (20 * 60000));
        expect(screen.getByRole('button', {name: /^End focus/})).toHaveTextContent('20 min left');
    });
});
