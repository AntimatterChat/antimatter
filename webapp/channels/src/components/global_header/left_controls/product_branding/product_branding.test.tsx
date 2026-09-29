// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TopLevelProducts} from 'utils/constants';
import * as productUtils from 'utils/products';
import {TestHelper} from 'utils/test_helper';

import type {ProductComponent} from 'types/store/plugins';

import {ProductBranding} from './product_branding';

// Compass icons render a bare <svg>, so the fallback icon needs a test id to be identifiable.
jest.mock('@mattermost/compass-icons/components', () => {
    const actual = jest.requireActual('@mattermost/compass-icons/components');
    return {
        ...actual,
        ProductChannelsIcon: (props: any) => (
            <svg
                data-testid='ProductChannelsIcon'
                {...props}
            />
        ),
    };
});

describe('ProductBranding', () => {
    afterEach(() => {
        jest.restoreAllMocks();
    });

    test('should show Channels when on Channels product', () => {
        jest.spyOn(productUtils, 'useCurrentProduct').mockReturnValue(null);

        renderWithContext(<ProductBranding/>);

        expect(screen.getAllByText('Channels').length).toBeGreaterThan(0);
    });

    test('should show Playbooks when on Playbooks product', () => {
        jest.spyOn(productUtils, 'useCurrentProduct').mockReturnValue(
            TestHelper.makeProduct(TopLevelProducts.PLAYBOOKS),
        );
        renderWithContext(<ProductBranding/>);

        expect(screen.getAllByText('Playbooks').length).toBeGreaterThan(0);
    });

    test('should show Boards when on Boards product', () => {
        jest.spyOn(productUtils, 'useCurrentProduct').mockReturnValue(
            TestHelper.makeProduct(TopLevelProducts.BOARDS),
        );
        renderWithContext(<ProductBranding/>);

        expect(screen.getAllByText('Boards').length).toBeGreaterThan(0);
    });

    test('should render a React element icon when switcherIcon is a React node', () => {
        const CustomIcon = (
            <svg data-testid='custom-icon'>
                <circle
                    cx='12'
                    cy='12'
                    r='10'
                />
            </svg>
        );

        jest.spyOn(productUtils, 'useCurrentProduct').mockReturnValue({
            ...TestHelper.makeProduct('CustomProduct'),
            switcherIcon: CustomIcon as unknown as ProductComponent['switcherIcon'],
        });

        renderWithContext(<ProductBranding/>);

        expect(screen.getByTestId('custom-icon')).toBeInTheDocument();
    });

    test('should fall back to the Channels icon when the icon name is not in the glyph map', () => {
        jest.spyOn(productUtils, 'useCurrentProduct').mockReturnValue({
            ...TestHelper.makeProduct('InvalidProduct'),
            switcherIcon: 'non-existent-icon-name' as ProductComponent['switcherIcon'],
        });

        renderWithContext(<ProductBranding/>);

        expect(screen.getByTestId('ProductChannelsIcon')).toBeInTheDocument();
    });
});
