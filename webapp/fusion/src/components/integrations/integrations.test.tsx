// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen, userEvent, within} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import Integrations from './integrations';

describe('components/integrations/Integrations', () => {
    const team = TestHelper.getTeamMock({id: 'team', name: 'antimatter', display_name: 'Antimatter'});
    const admin = TestHelper.getUserMock({id: 'admin', roles: 'system_admin system_user'});
    const state = {
        entities: {
            users: {currentUserId: admin.id, profiles: {[admin.id]: admin}},
            teams: {currentTeamId: team.id, teams: {[team.id]: team}, myMembers: {[team.id]: {team_id: team.id, user_id: admin.id, roles: 'team_user team_admin'}}},
            roles: {roles: {system_admin: {permissions: ['manage_oauth', 'manage_bots', 'manage_incoming_webhooks', 'manage_outgoing_webhooks', 'manage_slash_commands', 'manage_outgoing_oauth_connections']}}},
        },
    };

    const baseProps = {
        siteName: 'Antimatter',
        enableIncomingWebhooks: true,
        enableOutgoingWebhooks: true,
        enableCommands: true,
        enableOAuthServiceProvider: true,
        enableOutgoingOAuthConnections: true,
        enableCustomEmoji: true,
        canCreateOrDeleteCustomEmoji: true,
        pluginOptions: [],
        team,
    };

    test('groups the integrations in sections, without the App Directory', () => {
        renderWithContext(<Integrations {...baseProps}/>, state as never);

        const section = (name: string) => screen.getByRole('heading', {name}).closest('section')!;
        expect(within(section('Webhooks and commands')).getAllByRole('link')).toHaveLength(3);
        expect(within(section('Apps and bots')).getAllByRole('link')).toHaveLength(3);
        expect(within(section('Customization')).getByRole('link', {name: /Custom Emoji/})).toHaveAttribute('href', '/antimatter/emoji');
        expect(screen.queryByText(/App Directory/)).not.toBeInTheDocument();
    });

    test('leaves out custom emoji for people who may not add any', () => {
        renderWithContext(
            <Integrations
                {...baseProps}
                canCreateOrDeleteCustomEmoji={false}
            />,
            state as never,
        );

        expect(screen.queryByRole('heading', {name: 'Customization'})).not.toBeInTheDocument();
    });

    test('shows the entries of plugins with custom emoji', async () => {
        const action = jest.fn();
        renderWithContext(
            <Integrations
                {...baseProps}
                pluginOptions={[{id: 'gifs', pluginId: 'com.antimatterchat.gifs', title: 'Custom GIFs', description: 'Add GIFs', fusionIcon: 'image', action, shouldRender: () => true}]}
            />,
            state as never,
        );

        await userEvent.click(within(screen.getByRole('heading', {name: 'Customization'}).closest('section')!).getByRole('button', {name: /Custom GIFs/}));
        expect(action).toHaveBeenCalled();
    });
});
