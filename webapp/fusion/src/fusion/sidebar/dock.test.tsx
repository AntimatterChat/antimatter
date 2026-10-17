// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import Dock from './dock';

// Eight open conversations, dm0 the latest active.
const mockIds = Array.from({length: 8}, (_, i) => `dm${i}`);
const mockChannels = mockIds.map((id, i) => TestHelper.getChannelMock({id, name: id, type: 'G', display_name: `group ${i}`, last_post_at: 1000 - i}));
const mockCategory = {id: 'dms', type: 'direct_messages', channel_ids: mockIds};
let mockDockRows: number | null = null;

jest.mock('fusion/shell/layout_context', () => ({useLayout: () => ({setNavOpen: jest.fn(), setHome: jest.fn(), setDockRows: jest.fn(), dockRows: mockDockRows})}));
jest.mock('./dock_fit', () => ({useDockFit: jest.fn()}));
jest.mock('./server_categories', () => ({getSidebarCategories: () => [mockCategory]}));
jest.mock('./hidden', () => ({getHiddenChannelIds: () => new Set()}));
jest.mock('fusion/privacy/blocking', () => ({getHiddenDirectIds: () => new Set()}));
jest.mock('fusion/calls/calls_on_top', () => ({useCallsOnTop: (channels: unknown[]) => channels}));
jest.mock('fusion/calls/markers', () => ({DirectCallMarker: () => null}));
jest.mock('mattermost-redux/selectors/entities/channel_categories', () => ({
    ...jest.requireActual('mattermost-redux/selectors/entities/channel_categories'),
    makeGetChannelIdsForCategory: () => (_: unknown, category: {channel_ids: string[]}) => category.channel_ids,
}));

describe('fusion/sidebar/Dock', () => {
    const team = TestHelper.getTeamMock({id: 'team', name: 'antimatter'});
    const render = (current = 'town-square') => renderWithContext(
        <Dock
            sidebarRef={{current: null}}
            scrollRef={{current: null}}
        />,
        {
            entities: {
                teams: {currentTeamId: team.id, teams: {[team.id]: team}},
                channels: {currentChannelId: current, channels: Object.fromEntries(mockChannels.map((c) => [c.id, c]))},
                users: {currentUserId: 'me'},
            },
        } as never,
    );
    const rows = () => screen.queryAllByRole('button', {name: /^Close conversation with /}).map((close) => close.getAttribute('aria-label')!.replace('Close conversation with ', ''));

    afterEach(() => {
        mockDockRows = null;
    });

    test('shows as many conversations as fit, the latest active, and never scrolls', () => {
        mockDockRows = 3;
        const {container} = render();

        expect(rows()).toEqual(['group 0', 'group 1', 'group 2']);
        expect(container.querySelector('.am-dock-list')).toHaveStyle({overflowY: 'hidden'});
    });

    test('shows them all, up to the setting, until the room is measured', () => {
        render();

        expect(rows()).toHaveLength(8);
    });

    test('keeps the conversation you are in', () => {
        mockDockRows = 2;
        render('dm7');

        expect(rows()).toEqual(['group 0', 'group 7']);
    });

    test('keeps the conversation you are in even when no row fits, and nothing else', () => {
        mockDockRows = 0;
        render('dm5');
        expect(rows()).toEqual(['group 5']);
    });
});
