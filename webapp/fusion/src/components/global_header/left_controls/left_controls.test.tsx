// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {DeepPartial} from '@mattermost/types/utilities';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import type {GlobalState} from 'types/store';
import type {ProductComponent} from 'types/store/plugins';

import LeftControls from './left_controls';

jest.mock('@mattermost/shared/utils/user_agent', () => ({
    ...jest.requireActual('@mattermost/shared/utils/user_agent'),
    isDesktopApp: jest.fn(() => false),
}));

// Antimatter: Fusion has no product menu (the app switcher), whatever the apps register; the classic web app keeps its
// own (see webapp/channels).
describe('components/global_header/left_controls', () => {
    const state = (products: ProductComponent[] = []): DeepPartial<GlobalState> => ({
        entities: {
            general: {config: {}},
            users: {
                currentUserId: 'user_id',
                profiles: {user_id: TestHelper.getUserMock({id: 'user_id'})},
            },
            teams: {
                currentTeamId: 'team_id',
                teams: {team_id: TestHelper.getTeamMock({id: 'team_id'})},
            },
        },
        plugins: {
            components: {
                Product: products,
            },
        },
    });

    test('has no product menu', () => {
        renderWithContext(<LeftControls/>, state());

        expect(screen.queryByRole('button', {name: 'Open product menu'})).not.toBeInTheDocument();

        // The page still names the product it shows, for screen readers.
        expect(screen.getByRole('heading', {name: 'Channels'})).toBeInTheDocument();
    });

    test('has none either while an app still registers a product', () => {
        renderWithContext(<LeftControls/>, state([TestHelper.makeProduct('Agents')]));

        expect(screen.queryByRole('button', {name: 'Open product menu'})).not.toBeInTheDocument();
        expect(document.getElementById('switchProductMenuButton')).toBeNull();
    });
});
