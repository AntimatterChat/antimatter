// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {DeepPartial} from '@mattermost/types/utilities';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import type {GlobalState} from 'types/store';
import type {ProductComponent, ProductSwitcherMenuItemRegistration} from 'types/store/plugins';

import LeftControls, {hasProductSwitcherEntries} from './left_controls';

jest.mock('@mattermost/shared/utils/user_agent', () => ({
    ...jest.requireActual('@mattermost/shared/utils/user_agent'),
    isDesktopApp: jest.fn(() => false),
}));

// Antimatter: no Antimatter app has a page of its own any more (Kanban and Playbooks are apps of the side panel), so
// the product menu only shows when an app still registers one, or adds an entry to the menu.
describe('components/global_header/left_controls', () => {
    const state = (plugins: {Product?: ProductComponent[]; ProductSwitcherMenuItem?: ProductSwitcherMenuItemRegistration[]} = {}): DeepPartial<GlobalState> => ({
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
                Product: plugins.Product ?? [],
                ProductSwitcherMenuItem: plugins.ProductSwitcherMenuItem ?? [],
            },
        },
    });

    const product = TestHelper.makeProduct('Agents');

    test('has no product menu when no app registers a product', () => {
        renderWithContext(<LeftControls productId={null}/>, state());

        expect(screen.queryByRole('button', {name: 'Open product menu'})).not.toBeInTheDocument();

        // The page still names the product it shows, for screen readers.
        expect(screen.getByRole('heading', {name: 'Channels'})).toBeInTheDocument();
    });

    test('has the product menu while an app registers a product', () => {
        renderWithContext(<LeftControls productId={null}/>, state({Product: [product]}));

        expect(screen.getByRole('button', {name: 'Open product menu'})).toBeInTheDocument();
    });

    test('has the product menu while an app adds an entry to it', () => {
        const item = {id: 'item', pluginId: 'plugin', text: 'Item', action: jest.fn()} as unknown as ProductSwitcherMenuItemRegistration;
        renderWithContext(<LeftControls productId={null}/>, state({ProductSwitcherMenuItem: [item]}));

        expect(screen.getByRole('button', {name: 'Open product menu'})).toBeInTheDocument();
    });

    test.each([
        [{}, false],
        [{Product: [product]}, true],
        [{ProductSwitcherMenuItem: [{id: 'item'} as ProductSwitcherMenuItemRegistration]}, true],
    ])('hasProductSwitcherEntries(%o) is %s', (plugins, expected) => {
        expect(hasProductSwitcherEntries({plugins: {components: plugins}} as unknown as GlobalState)).toBe(expected);
    });
});
