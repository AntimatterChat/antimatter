// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {ErrorPageTypes} from 'utils/constants';

import ErrorPage from './error_page';

describe('ErrorPage', () => {
    it('displays permalink not found page', () => {
        renderWithContext(
            (
                <ErrorPage
                    location={{
                        search: `?type=${ErrorPageTypes.PERMALINK_NOT_FOUND}&returnTo=/team/channels/town-square`,
                    }}
                />
            ),
        );

        screen.getByText('Message Not Found');
    });
});
