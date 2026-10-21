// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {DeepPartial} from '@mattermost/types/utilities';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import type {GlobalState} from 'types/store';

import LeftControls from './left_controls';

jest.mock('@mattermost/shared/utils/user_agent', () => ({
    ...jest.requireActual('@mattermost/shared/utils/user_agent'),
    isDesktopApp: jest.fn(() => false),
}));

// Antimatter: Fusion has no product menu any more, but the classic web app keeps its own, even with no app that has a
// page of its own: it's the classic web app's way to the System Console, Integrations and the rest.
describe('classic components/global_header/left_controls', () => {
    const state: DeepPartial<GlobalState> = {
        entities: {
            general: {config: {}},
            users: {
                currentUserId: 'user_id',
                profiles: {user_id: TestHelper.getUserMock({id: 'user_id', roles: 'system_admin system_user'})},
            },
            teams: {
                currentTeamId: 'team_id',
                teams: {team_id: TestHelper.getTeamMock({id: 'team_id'})},
            },
        },
        plugins: {
            components: {
                Product: [],
            },
        },
    };

    test('keeps the product menu with no app product', async () => {
        renderWithContext(<LeftControls productId={null}/>, state);

        const button = screen.getByRole('button', {name: 'Open product menu'});
        expect(button).toBeInTheDocument();

        await userEvent.click(button);
        expect(screen.getByRole('menu', {name: 'Product menu'})).toBeInTheDocument();
    });
});
