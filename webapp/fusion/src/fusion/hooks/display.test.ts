// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {GlobalState} from 'types/store';

import {getDensity, getDisplayPrefs, isSimplifiedInterface} from './display';

const stateWith = (prefs: Record<string, string>) => ({
    entities: {
        general: {config: {}},
        preferences: {myPreferences: Object.fromEntries(Object.entries(prefs).map(([key, value]) => {
            const [category, name] = key.split('--');
            return [key, {user_id: 'me', category, name, value}];
        }))},
    },
}) as unknown as GlobalState;

describe('fusion/hooks/display', () => {
    test('messages are standard until chosen otherwise', () => {
        expect(getDensity(stateWith({}))).toBe('standard');
    });

    test('compact is Mattermost\'s message display, and wins over a spacious left over', () => {
        expect(getDensity(stateWith({'display_settings--message_display': 'compact'}))).toBe('compact');
        expect(getDensity(stateWith({'display_settings--message_display': 'compact', 'fusion_display--spacious': 'true'}))).toBe('compact');
    });

    test('spacious is Antimatter\'s, beside a standard message display', () => {
        expect(getDensity(stateWith({'display_settings--message_display': 'clean', 'fusion_display--spacious': 'true'}))).toBe('spacious');
        expect(getDisplayPrefs(stateWith({'fusion_display--spacious': 'true'})).compact).toBe(false);
    });

    test('names start in the colour of their role, or none: coloured names are off by default', () => {
        expect(getDisplayPrefs(stateWith({})).colorize).toBe(false);
        expect(getDisplayPrefs(stateWith({'display_settings--colorize_usernames': 'true'})).colorize).toBe(true);
    });

    test('the simplified interface is off by default', () => {
        expect(isSimplifiedInterface(stateWith({}))).toBe(false);
        expect(isSimplifiedInterface(stateWith({'fusion_display--simplified_interface': 'true'}))).toBe(true);
        expect(getDisplayPrefs(stateWith({'fusion_display--simplified_interface': 'true'})).simplified).toBe(true);
    });
});
