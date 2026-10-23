// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import IntegrationsNav from './integrations_nav';

const team = TestHelper.getTeamMock({id: 'team1', name: 'lab', display_name: 'Lumen Labs', delete_at: 0});

const render = (props: {enableCustomEmoji: boolean; canCreateOrDeleteCustomEmoji: boolean}) => renderWithContext(
    <IntegrationsNav
        team={team}
        enableIncomingWebhooks={false}
        enableOutgoingWebhooks={false}
        enableCommands={false}
        enableOAuthServiceProvider={false}
        enableOutgoingOAuthConnections={false}
        canManageIntegrations={false}
        {...props}
    />,
    {plugins: {components: {IntegrationsOption: []}}} as never,
);

describe('fusion/backstage/IntegrationsNav', () => {
    test('lists Custom emoji for those who may add or delete some', () => {
        render({enableCustomEmoji: true, canCreateOrDeleteCustomEmoji: true});
        expect(screen.getByRole('link', {name: 'Custom emoji'})).toHaveAttribute('href', '/lab/emoji');
    });

    test('has no Custom emoji without the permission, or when custom emoji are off', () => {
        const {unmount} = render({enableCustomEmoji: true, canCreateOrDeleteCustomEmoji: false});
        expect(screen.queryByRole('link', {name: 'Custom emoji'})).not.toBeInTheDocument();
        unmount();

        render({enableCustomEmoji: false, canCreateOrDeleteCustomEmoji: true});
        expect(screen.queryByRole('link', {name: 'Custom emoji'})).not.toBeInTheDocument();
    });

    // Antimatter: agents are made in the AI settings (the System Console's, and the community's settings), not here.
    const renderWithRoles = (roles: string, teamRoles: string, permissions: Record<string, string[]>) => renderWithContext(
        <IntegrationsNav
            team={team}
            enableCustomEmoji={false}
            enableIncomingWebhooks={false}
            enableOutgoingWebhooks={false}
            enableCommands={false}
            enableOAuthServiceProvider={false}
            enableOutgoingOAuthConnections={false}
            canCreateOrDeleteCustomEmoji={false}
            canManageIntegrations={true}
        />,
        {
            plugins: {components: {IntegrationsOption: []}},
            entities: {
                users: {currentUserId: 'me', profiles: {me: TestHelper.getUserMock({id: 'me', roles})}},
                teams: {currentTeamId: 'team1', teams: {team1: team}, myMembers: {team1: {team_id: 'team1', user_id: 'me', roles: teamRoles}}},
                roles: {roles: Object.fromEntries(Object.entries(permissions).map(([name, perms]) => [name, {name, permissions: perms}]))},
            },
        } as never,
    );

    test('has no Agents entry, not even for the admins who make agents', () => {
        const {unmount} = renderWithRoles('system_user system_admin', 'team_user', {system_admin: ['manage_system'], system_user: [], team_user: []});
        expect(screen.queryByRole('link', {name: 'Agents'})).not.toBeInTheDocument();
        unmount();

        renderWithRoles('system_user', 'team_user team_admin', {system_user: [], team_user: [], team_admin: ['manage_team']});
        expect(screen.queryByRole('link', {name: 'Agents'})).not.toBeInTheDocument();
    });
});
