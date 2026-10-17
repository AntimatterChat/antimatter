// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen} from 'tests/react_testing_utils';

import Header from './header';

describe('components/header_footer_route/Header', () => {
    const state = {entities: {general: {config: {SiteName: 'Antimatter'}}}};

    test('takes its back link where the page says, not to the root', () => {
        renderWithContext(
            <Header
                backButtonURL='/login?redirect_to=%2Fantimatter'
                onBackButtonClick={jest.fn()}
            />,
            state as never,
        );

        expect(screen.getByTestId('back_button')).toHaveAttribute('href', '/login?redirect_to=%2Fantimatter');
        expect(screen.getByTestId('back_button')).toHaveTextContent('Back');
    });

    test('shows no back link unless the page has one', () => {
        renderWithContext(<Header/>, state as never);

        expect(screen.queryByTestId('back_button')).not.toBeInTheDocument();
    });
});
