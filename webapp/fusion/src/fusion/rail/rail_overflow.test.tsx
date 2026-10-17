// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import RailDirectMessages from './rail_dms';

// The dock and the rail together: the dock keeps the latest active conversations, up to the Open conversations
// setting; the unread ones past it wait on the rail.
const mockIds = Array.from({length: 12}, (_, i) => `dm${i}`);
const mockChannels = mockIds.map((id, i) => TestHelper.getChannelMock({id, name: id, type: 'G', display_name: `group ${i}`, last_post_at: 1000 - i}));
const mockCategory = {id: 'dms', type: 'direct_messages', channel_ids: mockIds};
type Unread = {channel: typeof mockChannels[number]; mentions: number; messages: number};
let mockUnread: Unread[] = [];
const unreadIn = (ids: string[]) => mockChannels.filter((c) => ids.includes(c.id)).map((channel) => ({channel, mentions: 1, messages: 1}));

let mockDockRows: number | null = null;
jest.mock('fusion/shell/layout_context', () => ({useLayout: () => ({home: false, setHome: jest.fn(), setNavOpen: jest.fn(), dockRows: mockDockRows}), isPhoneLayout: () => false}));
jest.mock('fusion/sidebar/home_sidebar', () => ({DM_FIND_ID: 'am-dm-find'}));
jest.mock('fusion/sidebar/server_categories', () => ({getSidebarCategories: () => [mockCategory]}));
jest.mock('fusion/sidebar/hidden', () => ({getHiddenChannelIds: () => new Set()}));
jest.mock('fusion/privacy/blocking', () => ({getHiddenDirectIds: () => new Set()}));
jest.mock('mattermost-redux/selectors/entities/channel_categories', () => ({
    ...jest.requireActual('mattermost-redux/selectors/entities/channel_categories'),
    makeGetChannelIdsForCategory: () => (_: unknown, category: {channel_ids: string[]}) => category.channel_ids,
}));
jest.mock('fusion/selectors', () => ({
    getUnreadDirectChannels: () => mockUnread,
}));

describe('fusion/rail: the Open conversations overflow', () => {
    const team = TestHelper.getTeamMock({id: 'team', name: 'antimatter'});
    const render = (limit?: string) => renderWithContext(
        <RailDirectMessages/>,
        {
            entities: {
                teams: {currentTeamId: team.id, teams: {[team.id]: team}},
                channels: {currentChannelId: 'town-square', channels: Object.fromEntries(mockChannels.map((c) => [c.id, c]))},
                users: {currentUserId: 'me'},
                preferences: {myPreferences: limit ? {'sidebar_settings--limit_visible_dms_gms': {user_id: 'me', category: 'sidebar_settings', name: 'limit_visible_dms_gms', value: limit}} : {}},
            },
        } as never,
    );
    const bubbles = () => screen.queryAllByRole('button', {name: /^group \d+, 1 unread message$/}).map((b) => b.getAttribute('data-am-tip'));

    test('the unread conversations past the 10 the dock keeps by default wait on the rail', () => {
        mockUnread = unreadIn(['dm0', 'dm10', 'dm11']);
        render();

        expect(bubbles()).toEqual(['group 10', 'group 11']);
    });

    test('a smaller Open conversations setting sends more to the rail', () => {
        mockUnread = unreadIn(['dm0', 'dm6', 'dm10']);
        render('5');

        expect(bubbles()).toEqual(['group 6', 'group 10']);
    });

    test('none on the rail when every unread conversation fits in the dock', () => {
        mockUnread = unreadIn(['dm0', 'dm3', 'dm9']);
        render();

        expect(bubbles()).toEqual([]);
        expect(screen.getByRole('button', {name: /^All direct messages/})).not.toHaveClass('am-mini');
    });

    test('the unread conversations the dock has no room for wait on the rail too', () => {
        mockUnread = unreadIn(['dm0', 'dm3', 'dm9']);
        mockDockRows = 3;
        render();
        mockDockRows = null;

        expect(bubbles()).toEqual(['group 3', 'group 9']);
    });
});
