// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Channel} from '@mattermost/types/channels';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import HeaderAppChips from './header_app_chips';

// The chat header's place for apps' chips (registerChannelHeaderChip): each registered chip gets the conversation,
// they show in order, an app's chip that throws doesn't take the header down, and with nothing to show the place is
// empty (so its :empty rule collapses it).

describe('fusion/channel/HeaderAppChips', () => {
    const channel = TestHelper.getChannelMock({id: 'ch', name: 'standup', display_name: 'Standup', type: 'O'});
    const state = (components: unknown[]) => ({plugins: {components: {ChannelHeaderChip: components}}} as any);

    test('renders nothing without a registered chip', () => {
        const {container} = renderWithContext(<HeaderAppChips channel={channel}/>, state([]));
        const place = container.querySelector('.am-head-chips');
        expect(place).toBeInTheDocument();
        expect(place).toBeEmptyDOMElement();
    });

    test('gives each app\'s chip the conversation, in order', () => {
        const Chip = ({channel: c, label}: {channel: Channel; label: string}) => <span className='am-head-chip'>{`${label} ${c.display_name}`}</span>;
        renderWithContext(
            <HeaderAppChips channel={channel}/>,
            state([
                {id: 'a', pluginId: 'calendar', component: (props: {channel: Channel}) => <Chip {...props} label='Next call in'/>},
                {id: 'b', pluginId: 'other', component: (props: {channel: Channel}) => <Chip {...props} label='Other in'/>},
            ]),
        );
        const chips = screen.getAllByText(/ Standup$/);
        expect(chips.map((c) => c.textContent)).toEqual(['Next call in Standup', 'Other in Standup']);
    });

    test('an app\'s chip that returns null leaves the place empty', () => {
        const {container} = renderWithContext(
            <HeaderAppChips channel={channel}/>,
            state([{id: 'a', pluginId: 'calendar', component: () => null}]),
        );
        expect(container.querySelector('.am-head-chips')).toBeEmptyDOMElement();
    });

    test('an app\'s chip that throws doesn\'t take the others down', () => {
        jest.spyOn(console, 'error').mockImplementation(() => {});
        const Broken = () => {
            throw new Error('broken');
        };
        renderWithContext(
            <HeaderAppChips channel={channel}/>,
            state([
                {id: 'a', pluginId: 'broken', component: Broken},
                {id: 'b', pluginId: 'calendar', component: () => <span className='am-head-chip'>{'Next call'}</span>},
            ]),
        );
        expect(screen.getByText('Next call')).toBeInTheDocument();
        jest.restoreAllMocks();
    });
});
