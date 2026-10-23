// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {fireEvent, renderWithContext, screen, userEvent} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import Dock from './dock';

// Eight open conversations, dm0 the latest active.
const mockIds = Array.from({length: 8}, (_, i) => `dm${i}`);
const mockChannels = mockIds.map((id, i) => TestHelper.getChannelMock({id, name: id, type: 'G', display_name: `group ${i}`, last_post_at: 1000 - i}));
const mockCategory = {id: 'dms', type: 'direct_messages', channel_ids: mockIds};
let mockDockRows: number | null = null;

jest.mock('fusion/shell/layout_context', () => ({useLayout: () => ({setNavOpen: jest.fn(), setHome: jest.fn(), setDockRows: jest.fn(), dockRows: mockDockRows})}));
jest.mock('./dock_fit', () => ({useDockFit: jest.fn(), DEFAULT_ROW: 34}));

// jsdom has no pointer events: mouse events with a pointer id stand in for them.
if (!window.PointerEvent) {
    class PointerEvent extends MouseEvent {
        pointerId: number;
        constructor(type: string, init: PointerEventInit = {}) {
            super(type, init);
            this.pointerId = init.pointerId ?? 0;
        }
    }
    window.PointerEvent = PointerEvent as typeof window.PointerEvent;
}
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
    const render = (current = 'general') => renderWithContext(
        <Dock
            sidebarRef={{current: null}}
            scrollRef={{current: null}}
        />,
        {
            entities: {
                teams: {currentTeamId: team.id, teams: {[team.id]: team}},
                channels: {currentChannelId: current, channels: Object.fromEntries(mockChannels.map((c) => [c.id, c]))},
                users: {currentUserId: 'me', profiles: {me: TestHelper.getUserMock({id: 'me', username: 'me'})}},
                preferences: {myPreferences: {}},
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

    // Its size (how many conversations show) is a drag of its top edge, snapping to whole rows, remembered as a
    // preference of yours: there is no longer a setting for it (see AppearancePane in modals/settings_modal.tsx).
    describe('resizing its top edge', () => {
        test('drags it taller or shorter, in whole rows', () => {
            mockDockRows = 20;
            const {store} = render();
            const handle = screen.getByRole('separator', {name: 'Resize Open conversations'});
            expect(handle).toHaveAttribute('aria-valuemin', '1');
            expect(handle).toHaveAttribute('aria-valuemax', '20');
            expect(handle).toHaveAttribute('aria-valuenow', '10');

            // Dragging the top edge up (the pointer's y decreasing) grows the dock; two rows' worth snaps to +2.
            fireEvent.pointerDown(handle, {button: 0, clientY: 500, pointerId: 1});
            fireEvent.pointerMove(handle, {clientY: 500 - (2 * 34), pointerId: 1});
            expect(handle).toHaveAttribute('aria-valuenow', '12');
            expect(rows()).toHaveLength(8);

            fireEvent.pointerUp(handle, {pointerId: 1});
            expect(store.getState().entities.preferences.myPreferences['fusion_sidebar--dock_rows'].value).toBe('12');

            // Moving the pointer after the drag ends changes nothing.
            fireEvent.pointerMove(handle, {clientY: 0, pointerId: 1});
            expect(handle).toHaveAttribute('aria-valuenow', '12');
        });

        test('never past what currently fits', () => {
            mockDockRows = 3;
            render();
            const handle = screen.getByRole('separator', {name: 'Resize Open conversations'});
            expect(handle).toHaveAttribute('aria-valuemax', '3');

            // The preference (10 by default) is clamped down to what fits right now, even before any drag.
            expect(handle).toHaveAttribute('aria-valuenow', '3');

            fireEvent.pointerDown(handle, {button: 0, clientY: 500, pointerId: 1});
            fireEvent.pointerMove(handle, {clientY: 500 - (50 * 34), pointerId: 1});
            expect(handle).toHaveAttribute('aria-valuenow', '3');
        });

        test('resizes with the arrow keys, saving at once', async () => {
            mockDockRows = 20;
            const {store} = render();
            const handle = screen.getByRole('separator', {name: 'Resize Open conversations'});
            handle.focus();

            await userEvent.keyboard('{ArrowUp}{ArrowUp}');
            expect(handle).toHaveAttribute('aria-valuenow', '12');
            expect(store.getState().entities.preferences.myPreferences['fusion_sidebar--dock_rows'].value).toBe('12');

            await userEvent.keyboard('{ArrowDown}');
            expect(handle).toHaveAttribute('aria-valuenow', '11');
        });

        test('starts at the old Open conversations setting the first time, before it is ever dragged', () => {
            mockDockRows = 20;
            const {store} = renderWithContext(
                <Dock
                    sidebarRef={{current: null}}
                    scrollRef={{current: null}}
                />,
                {
                    entities: {
                        teams: {currentTeamId: 'team', teams: {team: TestHelper.getTeamMock({id: 'team'})}},
                        channels: {currentChannelId: 'general', channels: Object.fromEntries(mockChannels.map((c) => [c.id, c]))},
                        users: {currentUserId: 'me', profiles: {me: TestHelper.getUserMock({id: 'me', username: 'me'})}},
                        preferences: {myPreferences: {'sidebar_settings--limit_visible_dms_gms': {user_id: 'me', category: 'sidebar_settings', name: 'limit_visible_dms_gms', value: '5'}}},
                    },
                } as never,
            );
            expect(screen.getByRole('separator', {name: 'Resize Open conversations'})).toHaveAttribute('aria-valuenow', '5');
            expect(store.getState().entities.preferences.myPreferences['fusion_sidebar--dock_rows']).toBeUndefined();
        });
    });
});
