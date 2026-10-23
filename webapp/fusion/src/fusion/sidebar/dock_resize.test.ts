// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {GlobalState} from 'types/store';

import {clampRows, getDockRowsPreference, keyRows, MAX_DOCK_ROWS, MIN_DOCK_ROWS} from './dock_resize';

describe('fusion/sidebar/clampRows', () => {
    test('keeps a row count within its bounds', () => {
        expect(clampRows(0, {min: 1, max: 20})).toBe(1);
        expect(clampRows(12.4, {min: 1, max: 20})).toBe(12);
        expect(clampRows(30, {min: 1, max: 20})).toBe(20);

        // A dock too short to grow at all keeps its minimum.
        expect(clampRows(5, {min: 3, max: 1})).toBe(3);
    });
});

describe('fusion/sidebar/keyRows', () => {
    const bounds = {min: 1, max: 20};

    test('moves one row with Up and Down, within the bounds', () => {
        expect(keyRows('ArrowUp', 10, bounds)).toBe(11);
        expect(keyRows('ArrowUp', 20, bounds)).toBe(20);
        expect(keyRows('ArrowDown', 10, bounds)).toBe(9);
        expect(keyRows('ArrowDown', 1, bounds)).toBe(1);
    });

    test('Home and End take it to its fewest or most', () => {
        expect(keyRows('Home', 10, bounds)).toBe(1);
        expect(keyRows('End', 10, bounds)).toBe(20);
    });

    test('other keys change nothing', () => {
        expect(keyRows('Enter', 10, bounds)).toBeUndefined();
    });
});

describe('fusion/sidebar/getDockRowsPreference', () => {
    const stateWith = (preferences: Record<string, {category: string; name: string; value: string}>) => ({
        entities: {preferences: {myPreferences: preferences}},
    }) as unknown as GlobalState;

    test('defaults to 10, as the old "Open conversations" setting did', () => {
        expect(getDockRowsPreference(stateWith({}))).toBe(10);
    });

    test('starts at the old setting\'s value, before the dock is ever dragged', () => {
        const state = stateWith({'sidebar_settings--limit_visible_dms_gms': {category: 'sidebar_settings', name: 'limit_visible_dms_gms', value: '6'}});
        expect(getDockRowsPreference(state)).toBe(6);
    });

    test('once dragged, its own preference wins over the old setting', () => {
        const state = stateWith({
            'sidebar_settings--limit_visible_dms_gms': {category: 'sidebar_settings', name: 'limit_visible_dms_gms', value: '6'},
            'fusion_sidebar--dock_rows': {category: 'fusion_sidebar', name: 'dock_rows', value: '15'},
        });
        expect(getDockRowsPreference(state)).toBe(15);
    });

    test('keeps a saved value within bounds', () => {
        const low = stateWith({'fusion_sidebar--dock_rows': {category: 'fusion_sidebar', name: 'dock_rows', value: '0'}});
        expect(getDockRowsPreference(low)).toBe(MIN_DOCK_ROWS);
        const high = stateWith({'fusion_sidebar--dock_rows': {category: 'fusion_sidebar', name: 'dock_rows', value: '999'}});
        expect(getDockRowsPreference(high)).toBe(MAX_DOCK_ROWS);
        const junk = stateWith({'fusion_sidebar--dock_rows': {category: 'fusion_sidebar', name: 'dock_rows', value: 'nope'}});
        expect(getDockRowsPreference(junk)).toBe(10);
    });
});
