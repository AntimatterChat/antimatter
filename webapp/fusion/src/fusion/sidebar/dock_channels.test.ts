// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {GlobalState} from 'types/store';

import {makeGetDockChannelIds} from './dock_channels';

const mockCategory = {id: 'dms', type: 'direct_messages', channel_ids: [] as string[]};
jest.mock('./server_categories', () => ({getSidebarCategories: () => [mockCategory]}));
jest.mock('./hidden', () => ({getHiddenChannelIds: () => new Set()}));
jest.mock('fusion/privacy/blocking', () => ({getHiddenDirectIds: () => new Set()}));

// The category's conversations as Mattermost lists them (filtered and sorted), here as given.
jest.mock('mattermost-redux/selectors/entities/channel_categories', () => ({
    makeGetChannelIdsForCategory: () => (_: unknown, category: {channel_ids: string[]}) => category.channel_ids,
}));

describe('fusion/sidebar/makeGetDockChannelIds', () => {
    // dm0 is the latest active, dm11 the oldest; the category lists them in its own order.
    const ids = ['dm3', 'dm0', 'dm11', 'dm7', 'dm1', 'dm10', 'dm5', 'dm2', 'dm9', 'dm6', 'dm4', 'dm8'];
    const stateWith = ({limit, dockRows, current = 'general'}: {limit?: string; dockRows?: string; current?: string} = {}) => ({
        entities: {
            general: {config: {}},
            channels: {
                currentChannelId: current,
                channels: Object.fromEntries(ids.map((id) => [id, {id, last_post_at: 1000 - Number(id.slice(2)), create_at: 1}])),
            },
            preferences: {
                myPreferences: {
                    ...(limit ? {'sidebar_settings--limit_visible_dms_gms': {user_id: 'me', category: 'sidebar_settings', name: 'limit_visible_dms_gms', value: limit}} : {}),
                    ...(dockRows ? {'fusion_sidebar--dock_rows': {user_id: 'me', category: 'fusion_sidebar', name: 'dock_rows', value: dockRows}} : {}),
                },
            },
        },
    }) as unknown as GlobalState;

    beforeEach(() => {
        mockCategory.channel_ids = ids;
    });

    test('keeps the 10 latest active conversations by default, in the category\'s order', () => {
        expect(makeGetDockChannelIds()(stateWith())).toEqual(['dm3', 'dm0', 'dm7', 'dm1', 'dm5', 'dm2', 'dm9', 'dm6', 'dm4', 'dm8']);
    });

    test('starts at the old Open conversations setting, before the dock is ever dragged', () => {
        expect(makeGetDockChannelIds()(stateWith({limit: '5'}))).toEqual(['dm3', 'dm0', 'dm1', 'dm2', 'dm4']);
        expect(makeGetDockChannelIds()(stateWith({limit: '20'}))).toEqual(ids);
    });

    test('once dragged, follows its own row count instead of the old setting', () => {
        expect(makeGetDockChannelIds()(stateWith({limit: '20', dockRows: '5'}))).toEqual(['dm3', 'dm0', 'dm1', 'dm2', 'dm4']);
        expect(makeGetDockChannelIds()(stateWith({dockRows: '3'}))).toEqual(['dm3', 'dm0', 'dm1']);
    });

    test('always keeps the conversation you are in', () => {
        expect(makeGetDockChannelIds()(stateWith({limit: '5', current: 'dm11'}))).toEqual(['dm3', 'dm0', 'dm11', 'dm1', 'dm2']);
    });

    test('lists all of them when they fit', () => {
        mockCategory.channel_ids = ['dm2', 'dm1'];
        expect(makeGetDockChannelIds()(stateWith())).toEqual(['dm2', 'dm1']);
    });

    test('gives the same list while nothing changes', () => {
        const get = makeGetDockChannelIds();
        expect(get(stateWith())).toBe(get(stateWith()));
    });

    test('keeps no more than the rows that fit, the latest active', () => {
        expect(makeGetDockChannelIds()(stateWith(), 3)).toEqual(['dm0', 'dm1', 'dm2']);
        expect(makeGetDockChannelIds()(stateWith({limit: '5'}), 8)).toHaveLength(5);
        expect(makeGetDockChannelIds()(stateWith(), null)).toHaveLength(10);
    });

    test('keeps the conversation you are in even when no row fits', () => {
        expect(makeGetDockChannelIds()(stateWith({current: 'dm11'}), 0)).toEqual(['dm11']);
        expect(makeGetDockChannelIds()(stateWith(), 0)).toEqual([]);
        expect(makeGetDockChannelIds()(stateWith({current: 'dm11'}), 2)).toEqual(['dm0', 'dm11']);
    });
});
