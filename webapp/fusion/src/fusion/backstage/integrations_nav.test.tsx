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
});
