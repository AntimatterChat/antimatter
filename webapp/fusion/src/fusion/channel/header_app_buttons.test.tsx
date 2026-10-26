// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import HeaderAppButtons from './header_app_buttons';

// compat-plugins #4: Fusion drew apps' header buttons only in the app rail, and only with the app bar on. With
// DisableAppBar (an upstream setting, kept on migration) every app's button vanished, leaving it no way in.
describe('fusion/channel/HeaderAppButtons', () => {
    const channel = TestHelper.getChannelMock({id: 'c1', team_id: 't1'});
    const member = TestHelper.getChannelMembershipMock({channel_id: 'c1', user_id: 'me'}, {});
    const action = jest.fn();
    const render = (disableAppBar: string) => renderWithContext(
        <HeaderAppButtons/>,
        {
            entities: {
                general: {config: {DisableAppBar: disableAppBar}},
                users: {currentUserId: 'me'},
                channels: {currentChannelId: 'c1', channels: {c1: channel}, myMembers: {c1: member}},
            },
            plugins: {
                plugins: {zoom: {id: 'zoom', name: 'Zoom'}},
                components: {
                    ChannelHeaderButton: [{id: 'b1', pluginId: 'zoom', icon: <span>{'Z'}</span>, tooltipText: 'Start a meeting', dropdownText: 'Start a meeting', action}],
                    AppBar: [],
                },
            },
        } as never,
    );

    beforeEach(() => {
        action.mockClear();
    });

    test('shows apps\' header buttons when the app bar is off, and they work', async () => {
        render('true');
        await userEvent.click(screen.getByRole('button', {name: 'Start a meeting'}));
        expect(action).toHaveBeenCalledWith(expect.objectContaining({id: 'c1'}), expect.objectContaining({channel_id: 'c1'}));
    });

    test('shows nothing when the app bar is on: the buttons are in the app rail', () => {
        const {container} = render('false');
        expect(screen.queryByRole('button', {name: 'Start a meeting'})).not.toBeInTheDocument();
        expect(container).toBeEmptyDOMElement();
    });
});
