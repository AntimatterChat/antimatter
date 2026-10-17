// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen} from 'tests/react_testing_utils';

import StatusPopover from './status_popover';

jest.mock('fusion/shell/settings_context', () => ({useSettings: () => ({open: jest.fn()})}));
jest.mock('fusion/shell/toast_context', () => ({useToast: () => jest.fn()}));
jest.mock('components/plugin_marketplace/marketplace_modal', () => () => null);
jest.mock('fusion/popovers/emoji_picker', () => () => null);

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
