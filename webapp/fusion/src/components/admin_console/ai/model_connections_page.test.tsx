// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import ModelConnectionsPage from './model_connections_page';

jest.mock('fusion/ai/model_connections', () => ({
    __esModule: true,
    default: ({scope, listScope}: {scope: string; listScope: string}) => <div data-testid='connections'>{`${scope}|${listScope}`}</div>,
}));
jest.mock('fusion/agents/agents_settings', () => ({
    __esModule: true,
    default: ({scope}: {scope: string}) => <div data-testid='agents'>{scope}</div>,
}));

function render(roles: string, permissions: Record<string, string[]>) {
    return renderWithContext(
        <ModelConnectionsPage/>,
        {
            entities: {
                users: {currentUserId: 'me', profiles: {me: TestHelper.getUserMock({id: 'me', roles})}},
                roles: {roles: Object.fromEntries(Object.entries(permissions).map(([name, perms]) => [name, {name, permissions: perms}]))},
            },
        } as never,
    );
}

// The System Console's AI › Connections and agents: the model connections and the server-wide agents, on one page.
describe('components/admin_console/ai/ModelConnectionsPage', () => {
    test('a system admin has the connections (every one) and the server-wide agents', () => {
        render('system_user system_admin', {system_admin: ['manage_system', 'manage_ai_connections'], system_user: []});

        expect(screen.getByText('Connections and agents')).toBeInTheDocument();
        expect(screen.getByRole('region', {name: 'Model connections'})).toBeInTheDocument();
        expect(screen.getByTestId('connections')).toHaveTextContent('server|');
        expect(screen.getByRole('region', {name: 'Server-wide agents'})).toBeInTheDocument();
        expect(screen.getByTestId('agents')).toHaveTextContent('server');
    });

    test('who only manages the connections has no agents (the server refuses them)', () => {
        render('system_user system_manager', {system_manager: ['manage_ai_connections'], system_user: []});

        expect(screen.getByTestId('connections')).toBeInTheDocument();
        expect(screen.queryByRole('region', {name: 'Server-wide agents'})).not.toBeInTheDocument();
        expect(screen.queryByTestId('agents')).not.toBeInTheDocument();
    });
});
