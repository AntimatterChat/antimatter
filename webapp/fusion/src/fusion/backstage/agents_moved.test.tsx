// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {createMemoryHistory} from 'history';
import React from 'react';

import {renderWithContext} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import AgentsMoved, {COMMUNITY_AGENTS_TAB, SERVER_AGENTS_PATH} from './agents_moved';

const mockOpenTeamSettings = jest.fn((tab?: string) => ({type: 'MOCK_OPEN_TEAM_SETTINGS', tab}));
jest.mock('fusion/utils/modals', () => ({
    openTeamSettings: (tab?: string) => mockOpenTeamSettings(tab),
}));

const team = TestHelper.getTeamMock({id: 'team1', name: 'lab', display_name: 'Lumen Labs', delete_at: 0});

function render(roles: string, teamRoles: string, permissions: Record<string, string[]>) {
    const history = createMemoryHistory({initialEntries: ['/lab/integrations/agents']});
    renderWithContext(
        <AgentsMoved team={team}/>,
        {
            entities: {
                users: {currentUserId: 'me', profiles: {me: TestHelper.getUserMock({id: 'me', roles})}},
                teams: {currentTeamId: 'team1', teams: {team1: team}, myMembers: {team1: {team_id: 'team1', user_id: 'me', roles: teamRoles}}},
                roles: {roles: Object.fromEntries(Object.entries(permissions).map(([name, perms]) => [name, {name, permissions: perms}]))},
            },
        } as never,
        {history},
    );
    return history;
}

// Integrations › Agents is gone: agents are made in the AI settings, and its address sends there.
describe('fusion/backstage/AgentsMoved', () => {
    beforeEach(() => {
        mockOpenTeamSettings.mockClear();
    });

    test('a system admin goes to the System Console\'s connections and agents', () => {
        const history = render('system_user system_admin', 'team_user', {system_admin: ['manage_system'], system_user: [], team_user: []});
        expect(history.location.pathname).toBe(SERVER_AGENTS_PATH);
        expect(SERVER_AGENTS_PATH).toBe('/admin_console/ai/model_connections');
        expect(mockOpenTeamSettings).not.toHaveBeenCalled();
    });

    test('a community\'s admin goes to the community, its settings open on its agents', () => {
        const history = render('system_user', 'team_user team_admin', {system_user: [], team_user: [], team_admin: ['manage_team']});
        expect(history.location.pathname).toBe('/lab');
        expect(mockOpenTeamSettings).toHaveBeenCalledWith(COMMUNITY_AGENTS_TAB);
    });

    test('anyone else goes back to the integrations', () => {
        const history = render('system_user', 'team_user', {system_user: [], team_user: []});
        expect(history.location.pathname).toBe('/lab/integrations');
        expect(mockOpenTeamSettings).not.toHaveBeenCalled();
    });
});
