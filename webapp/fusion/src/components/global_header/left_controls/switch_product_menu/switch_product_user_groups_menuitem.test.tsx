// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {DeepPartial} from '@mattermost/types/utilities';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import type {GlobalState} from 'types/store';

import ProductSwitcherUserGroupsMenuItem from './switch_product_user_groups_menuitem';

jest.mock('react-redux', () => ({
    ...jest.requireActual('react-redux'),
    useDispatch: jest.fn().mockReturnValue(jest.fn()),
}));

describe('ProductSwitcherUserGroupsMenuItem', () => {
    const makeState = (enableCustomGroups: string): DeepPartial<GlobalState> => ({
        entities: {
            users: {
                currentUserId: 'user_id',
                profiles: {
                    user_id: TestHelper.getUserMock({id: 'user_id'}),
                },
            },
            general: {
                config: {
                    EnableCustomGroups: enableCustomGroups,
                },
            },
            preferences: {
                myPreferences: {},
            },
        },
    });

    test('should not show when custom user groups are not enabled', () => {
        renderWithContext(<ProductSwitcherUserGroupsMenuItem/>, makeState('false'));

        expect(screen.queryByText('User Groups')).not.toBeInTheDocument();
    });

    test('should show when custom user groups are enabled', () => {
        renderWithContext(<ProductSwitcherUserGroupsMenuItem/>, makeState('true'));

        expect(screen.getByText('User Groups')).toBeInTheDocument();
    });
});
