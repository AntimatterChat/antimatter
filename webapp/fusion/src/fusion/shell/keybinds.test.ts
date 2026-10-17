// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {GlobalState} from 'types/store';

import {KEY_ACTIONS, comboKeys, comboLabel, comboOf, getKeybinds, isKeybind} from './keybinds';

const key = (key: string, code: string, mods: Partial<KeyboardEvent> = {}) => ({key, code, ctrlKey: false, metaKey: false, altKey: false, shiftKey: false, ...mods}) as KeyboardEvent;
const stateWith = (binds: Record<string, string> = {}) => ({entities: {preferences: {myPreferences: {'fusion_keys--keybinds': {value: JSON.stringify(binds)}}}}}) as unknown as GlobalState;

describe('keybinds', () => {
    test('a key press is described by its place on the keyboard', () => {
        expect(comboOf(key('K', 'KeyK', {ctrlKey: true, shiftKey: true}))).toBe('mod+shift+k');
        expect(comboOf(key('?', 'Slash', {metaKey: true, shiftKey: true}))).toBe('mod+shift+/');
        expect(comboOf(key(' ', 'Space', {altKey: true}))).toBe('alt+space');
        expect(comboOf(key('Shift', 'ShiftLeft', {shiftKey: true}))).toBe('');
    });

    test('saved shortcuts replace the defaults, and an empty one is off', () => {
        const state = stateWith({search: 'mod+p', unreadFilter: ''});
        expect(getKeybinds(state).search).toBe('mod+p');
        expect(getKeybinds(state).settings).toBe('mod+shift+a');
        expect(isKeybind(state, key('p', 'KeyP', {ctrlKey: true}), 'search')).toBe(true);
        expect(isKeybind(state, key('k', 'KeyK', {ctrlKey: true}), 'search')).toBe(false);
        expect(isKeybind(state, key('u', 'KeyU', {ctrlKey: true, shiftKey: true}), 'unreadFilter')).toBe(false);
    });

    test('shortcuts are written as on the keyboard', () => {
        expect(comboLabel('mod+shift+k', false)).toEqual(['Ctrl', 'Shift', 'K']);
        expect(comboLabel('mod+shift+space', true)).toEqual(['⌘', '⇧', 'Space']);
    });

    // The same words as the community menu's checkbox it toggles.
    test('the unread filter is called Hide read channels', () => {
        expect(KEY_ACTIONS.find((a) => a.id === 'unreadFilter')?.label.defaultMessage).toBe('Hide read channels');
    });

    test('a combo is written for aria-keyshortcuts', () => {
        expect(comboKeys('mod+shift+u', false)).toBe('Control+Shift+U');
        expect(comboKeys('mod+shift+u', true)).toBe('Meta+Shift+U');
        expect(comboKeys('alt+space', false)).toBe('Alt+Space');
    });
});
