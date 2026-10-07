// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import UnreadCategory from './unread_category';

const mockUnread = [
    TestHelper.getChannelMock({id: 'mentions', type: 'O'}),
    TestHelper.getChannelMock({id: 'private', type: 'P'}),
    TestHelper.getChannelMock({id: 'dm', type: 'D'}),
    TestHelper.getChannelMock({id: 'gm', type: 'G'}),
];
jest.mock('selectors/views/channel_sidebar', () => ({
    getUnreadChannels: () => mockUnread,
}));
jest.mock('./channel_row', () => ({
    __esModule: true,
    default: ({channelId}: {channelId: string}) => <div data-testid='row'>{channelId}</div>,
}));

describe('fusion/sidebar/UnreadCategory', () => {
    const render = (showUnreadSection?: string) => renderWithContext(
        <UnreadCategory/>,
        {
            entities: {
                general: {config: {ExperimentalGroupUnreadChannels: 'disabled'}},
                preferences: {myPreferences: showUnreadSection === undefined ? {} : {
                    'sidebar_settings--show_unread_section': {category: 'sidebar_settings', name: 'show_unread_section', user_id: 'me', value: showUnreadSection},
                }},
            },
        } as never,
    );

    test('stays away by default', () => {
        render();

        expect(screen.queryByText('Unreads')).not.toBeInTheDocument();
        expect(screen.queryAllByTestId('row')).toHaveLength(0);
    });

    test('gathers the unread channels, leaving direct messages to the dock', () => {
        render('true');

        expect(screen.getByText('Unreads')).toBeInTheDocument();
        expect(screen.getAllByTestId('row').map((row) => row.textContent)).toEqual(['mentions', 'private']);
    });

    test('stays away when turned off', () => {
        render('false');

        expect(screen.queryByText('Unreads')).not.toBeInTheDocument();
    });
});
