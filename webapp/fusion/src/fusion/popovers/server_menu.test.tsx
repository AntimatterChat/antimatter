// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {Permissions} from 'mattermost-redux/constants';

import {CATEGORY_HIDE_MUTED, setHidingMutedChannels} from 'fusion/sidebar/team_actions';
import {accessState} from 'fusion/teams/settings_test_utils';
import {openTeamMembers, openTeamSettings, openTeams} from 'fusion/utils/modals';
import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import ServerMenu from './server_menu';

jest.mock('fusion/utils/modals', () => ({
    openInvitePeople: jest.fn(() => ({type: 'MOCK_INVITE'})),
    openLeaveTeam: jest.fn(() => ({type: 'MOCK_LEAVE'})),
    openTeamMembers: jest.fn(() => ({type: 'MOCK_MEMBERS'})),
    openTeamSettings: jest.fn(() => ({type: 'MOCK_SETTINGS'})),
    openTeams: jest.fn(() => ({type: 'MOCK_TEAMS'})),
}));
jest.mock('fusion/sidebar/team_actions', () => ({
    ...jest.requireActual('fusion/sidebar/team_actions'),
    setHidingMutedChannels: jest.fn(() => ({type: 'MOCK_HIDE_MUTED'})),
}));
const mockDialogs = {browseChannels: jest.fn(), createChannel: jest.fn(), createCategory: jest.fn(), selfRoles: jest.fn(), communityNotifications: jest.fn(), communityPrivacy: jest.fn(), communityProfile: jest.fn()};
jest.mock('fusion/shell/dialogs_context', () => ({useDialogs: () => mockDialogs}));
jest.mock('fusion/shell/settings_context', () => ({useSettings: () => ({open: jest.fn()})}));
jest.mock('fusion/shell/toast_context', () => ({useToast: () => jest.fn()}));

// The community menu: its order, who sees Community settings (and the entries it took over), the hide options' checks.
describe('fusion/popovers/ServerMenu', () => {
    beforeEach(() => {
        jest.clearAllMocks();
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
    });

    type Options = Parameters<typeof accessState>[1] & {unreadOnly?: boolean; hideMuted?: boolean};
    const render = (permissions: string[], {unreadOnly = false, hideMuted = false, ...options}: Options = {}) => {
        const state = accessState(permissions, options) as unknown as {entities: Record<string, unknown>; views?: unknown};
        state.entities.preferences = {
            myPreferences: hideMuted ? {[`${CATEGORY_HIDE_MUTED}--t1`]: {user_id: 'me', category: CATEGORY_HIDE_MUTED, name: 't1', value: 'true'}} : {},
        };
        state.views = {channelSidebar: {unreadFilterEnabled: unreadOnly}};
        return renderWithContext(
            <ServerMenu
                point={{x: 10, y: 10}}
                onClose={jest.fn()}
            />,
            state as never,
        );
    };

    // The menu as it reads: each entry's label (without its second line), '|' for a separator.
    const layout = () => Array.from(screen.getByRole('menu', {name: 'Community menu'}).children).flatMap((el) => {
        if (el.tagName === 'HR') {
            return ['|'];
        }
        const item = el.matches('[role^="menuitem"]') ? el : el.querySelector(':scope > [role^="menuitem"]');
        if (!item) {
            return [];
        }
        const copy = item.cloneNode(true) as HTMLElement;
        copy.querySelectorAll('small').forEach((s) => s.remove());
        return [copy.textContent?.trim() || ''];
    });

    const ADMIN = [Permissions.MANAGE_TEAM, Permissions.MANAGE_TEAM_ROLES, Permissions.INVITE_USER, Permissions.ADD_USER_TO_TEAM, Permissions.MANAGE_OWN_INCOMING_WEBHOOKS];
    const MEMBER = [Permissions.INVITE_USER, Permissions.ADD_USER_TO_TEAM, Permissions.CREATE_PUBLIC_CHANNEL];

    test('an admin\'s menu: Invite people, Community settings, the channels, the hide options, then Leave, in groups', () => {
        render(ADMIN);
        expect(layout()).toEqual([
            'Invite people', 'Community settings', 'Your profile in One…', '|',
            'Create channel', 'Create category', 'Browse channels', '|',
            'Hide read channels', 'Hide muted channels', 'Mute community', 'Notification settings', 'Privacy settings', '|',
            'Leave community',
        ]);
    });

    test('no "Organise the community", nor the entries Community settings now holds', () => {
        render(ADMIN);
        for (const gone of ['Organise the community', 'Roles…', 'Onboarding…', 'Community layout…', 'Member attributes…', 'Model connections…', 'Teams…', 'Manage members', 'View members', 'Integrations']) {
            expect(screen.queryByRole('menuitem', {name: gone})).not.toBeInTheDocument();
            expect(screen.queryByText(gone)).not.toBeInTheDocument();
        }
    });

    test('Notification settings and Privacy settings open the community\'s own, not the global settings', async () => {
        render(MEMBER);
        await userEvent.click(screen.getByRole('menuitem', {name: 'Notification settings'}));
        expect(mockDialogs.communityNotifications).toHaveBeenCalledWith('t1');
        await userEvent.click(screen.getByRole('menuitem', {name: 'Privacy settings'}));
        expect(mockDialogs.communityPrivacy).toHaveBeenCalledWith('t1');
    });

    // Antimatter: one's profile in the community, for everyone (My account has the global profile).
    test('Your profile in the community opens its dialog, for everyone', async () => {
        render(MEMBER);
        await userEvent.click(screen.getByRole('menuitem', {name: 'Your profile in One…'}));
        expect(mockDialogs.communityProfile).toHaveBeenCalled();
    });

    test('Community settings opens the settings of the menu\'s community', async () => {
        render(ADMIN);
        await userEvent.click(screen.getByRole('menuitem', {name: 'Community settings'}));
        expect(openTeamSettings).toHaveBeenCalledWith();
    });

    describe('who sees Community settings', () => {
        test('a member without rights doesn\'t, and keeps the members and teams in the menu', async () => {
            render(MEMBER);
            expect(screen.queryByRole('menuitem', {name: 'Community settings'})).not.toBeInTheDocument();
            expect(screen.queryByRole('menuitem', {name: 'Integrations'})).not.toBeInTheDocument();
            expect(layout().slice(0, 5)).toEqual(['Invite people', 'View members', 'Teams…', 'Your profile in One…', '|']);

            await userEvent.click(screen.getByRole('menuitem', {name: 'View members'}));
            expect(openTeamMembers).toHaveBeenCalled();
            await userEvent.click(screen.getByRole('menuitem', {name: 'Teams…'}));
            expect(openTeams).toHaveBeenCalled();
        });

        test('without teams on the server, a member has no Teams entry', () => {
            render(MEMBER, {customGroups: false});
            expect(screen.queryByRole('menuitem', {name: 'Teams…'})).not.toBeInTheDocument();
            expect(screen.getByRole('menuitem', {name: 'View members'})).toBeInTheDocument();
        });

        test('a channel admin, who manages nothing of the community, doesn\'t', () => {
            render(MEMBER, {channelAdmin: true});
            expect(screen.queryByRole('menuitem', {name: 'Community settings'})).not.toBeInTheDocument();
        });

        test.each([
            ['the community\'s roles', [Permissions.MANAGE_TEAM_ROLES]],
            ['its model connections', [Permissions.MANAGE_TEAM_AI_CONNECTIONS]],
            ['its webhooks', [Permissions.MANAGE_OWN_INCOMING_WEBHOOKS]],
            ['its slash commands', [Permissions.MANAGE_OWN_SLASH_COMMANDS]],
        ])('someone who manages %s does, right after Invite people, and finds the members and teams there', (_, permissions) => {
            render([...MEMBER, ...permissions]);
            expect(layout().slice(0, 4)).toEqual(['Invite people', 'Community settings', 'Your profile in One…', '|']);
            expect(screen.queryByRole('menuitem', {name: 'View members'})).not.toBeInTheDocument();
            expect(screen.queryByRole('menuitem', {name: 'Teams…'})).not.toBeInTheDocument();
        });

        test('Community settings comes first for those who can\'t invite', () => {
            render([Permissions.MANAGE_TEAM_ROLES]);
            expect(layout()[0]).toBe('Community settings');
        });
    });

    describe('the hide options', () => {
        test('are named short, and are checkboxes that are off by default', () => {
            render(MEMBER);
            const read = screen.getByRole('menuitemcheckbox', {name: 'Hide read channels'});
            const muted = screen.getByRole('menuitemcheckbox', {name: 'Hide muted channels'});
            expect(screen.queryByText('Show unread channels only')).not.toBeInTheDocument();
            for (const item of [read, muted]) {
                expect(item).toHaveAttribute('aria-checked', 'false');
                expect(item.querySelector('.am-end')).toBeNull();
            }
        });

        test('show a check when on', () => {
            render(MEMBER, {unreadOnly: true, hideMuted: true});
            for (const name of ['Hide read channels', 'Hide muted channels']) {
                const item = screen.getByRole('menuitemcheckbox', {name});
                expect(item).toHaveAttribute('aria-checked', 'true');
                expect(item.querySelector('.am-end use')).toHaveAttribute('href', '#am-i-check');
            }
        });

        test('Hide read channels keeps its shortcut in a tooltip, not on a second line', () => {
            render(MEMBER);
            const read = screen.getByRole('menuitemcheckbox', {name: 'Hide read channels'});
            expect(read.querySelector('small')).toBeNull();
            expect(read).toHaveAttribute('title', 'Ctrl+Shift+U');
            expect(read).toHaveAttribute('aria-keyshortcuts', 'Control+Shift+U');
        });

        test('Hide muted channels turns on, and off again', async () => {
            const {unmount} = render(MEMBER);
            await userEvent.click(screen.getByRole('menuitemcheckbox', {name: 'Hide muted channels'}));
            expect(setHidingMutedChannels).toHaveBeenLastCalledWith('t1', true);
            unmount();

            render(MEMBER, {hideMuted: true});
            await userEvent.click(screen.getByRole('menuitemcheckbox', {name: 'Hide muted channels'}));
            expect(setHidingMutedChannels).toHaveBeenLastCalledWith('t1', false);
        });
    });
});
