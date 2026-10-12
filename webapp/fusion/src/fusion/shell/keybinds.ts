// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {get as getPreference} from 'mattermost-redux/selectors/entities/preferences';

import type {GlobalState} from 'types/store';

// The keyboard shortcuts of the Fusion UI that can be changed in its settings, kept in a preference as
// {action: combo}. A combo is like "mod+shift+k": mod is Ctrl, or ⌘ on macOS.
export const CATEGORY_FUSION_KEYS = 'fusion_keys';
export const NAME_KEYBINDS = 'keybinds';

export type KeyAction = 'search' | 'shortcuts' | 'settings' | 'unreadFilter' | 'callMute' | 'pushToTalk' | 'callHand' | 'callScreen' | 'callLeave' | 'callWindow';

export const KEY_ACTIONS: Array<{id: KeyAction; label: {id: string; defaultMessage: string}; combo: string}> = [
    {id: 'search', label: {id: 'fusion.keys.search', defaultMessage: 'Search everything'}, combo: 'mod+k'},
    {id: 'shortcuts', label: {id: 'fusion.keys.shortcuts', defaultMessage: 'Show the keyboard shortcuts'}, combo: 'mod+/'},
    {id: 'settings', label: {id: 'fusion.keys.settings', defaultMessage: 'Open the settings'}, combo: 'mod+shift+a'},
    {id: 'unreadFilter', label: {id: 'fusion.keys.unreadFilter', defaultMessage: 'Hide read channels'}, combo: 'mod+shift+u'},
    {id: 'callMute', label: {id: 'fusion.keys.callMute', defaultMessage: 'Mute or unmute in a call'}, combo: 'mod+shift+space'},
    {id: 'pushToTalk', label: {id: 'fusion.keys.pushToTalk', defaultMessage: 'Push to talk in a call (hold)'}, combo: ''},
    {id: 'callHand', label: {id: 'fusion.keys.callHand', defaultMessage: 'Raise or lower your hand in a call'}, combo: 'mod+shift+y'},
    {id: 'callScreen', label: {id: 'fusion.keys.callScreen', defaultMessage: 'Share your screen in a call'}, combo: 'mod+shift+e'},
    {id: 'callLeave', label: {id: 'fusion.keys.callLeave', defaultMessage: 'Leave the call'}, combo: 'mod+shift+l'},
    {id: 'callWindow', label: {id: 'fusion.keys.callWindow', defaultMessage: 'Show the call'}, combo: 'mod+shift+p'},
];

const MODIFIER_KEYS = new Set(['Control', 'Meta', 'Shift', 'Alt', 'AltGraph', 'CapsLock', 'OS']);

// keyOf names the key pressed by its place on the keyboard for letters, digits and signs, so that Shift doesn't
// change it (Shift+/ is still "/").
function keyOf(e: Pick<KeyboardEvent, 'key' | 'code'>): string {
    const code = e.code || '';
    if ((/^Key[A-Z]$/).test(code)) {
        return code.slice(3).toLowerCase();
    }
    if ((/^Digit\d$/).test(code)) {
        return code.slice(5);
    }
    const signs: Record<string, string> = {Slash: '/', Backslash: '\\', Period: '.', Comma: ',', Semicolon: ';', Quote: '\'', BracketLeft: '[', BracketRight: ']', Minus: '-', Equal: '=', Backquote: '`', Space: 'space'};
    if (signs[code]) {
        return signs[code];
    }
    return e.key === ' ' ? 'space' : (e.key || '').toLowerCase();
}

// comboOf describes a key press as a combo, or '' for a modifier alone.
export function comboOf(e: Pick<KeyboardEvent, 'key' | 'code' | 'ctrlKey' | 'metaKey' | 'altKey' | 'shiftKey'>): string {
    if (MODIFIER_KEYS.has(e.key)) {
        return '';
    }
    return [(e.ctrlKey || e.metaKey) && 'mod', e.altKey && 'alt', e.shiftKey && 'shift', keyOf(e)].filter(Boolean).join('+');
}

// getSavedKeybinds is the preference that keeps the shortcuts changed in the settings.
export function getSavedKeybinds(state: GlobalState): string {
    return getPreference(state, CATEGORY_FUSION_KEYS, NAME_KEYBINDS, '{}');
}

// parseKeybinds reads the changed shortcuts from their preference.
export function parseKeybinds(saved: string): Partial<Record<KeyAction, string>> {
    try {
        return JSON.parse(saved) || {};
    } catch {
        return {};
    }
}

// withDefaults gives every action its shortcut: the changed one, else its default.
export function withDefaults(saved: Partial<Record<KeyAction, string>>): Record<KeyAction, string> {
    const binds = {} as Record<KeyAction, string>;
    for (const action of KEY_ACTIONS) {
        binds[action.id] = typeof saved[action.id] === 'string' ? saved[action.id]! : action.combo;
    }
    return binds;
}

export function getKeybinds(state: GlobalState): Record<KeyAction, string> {
    return withDefaults(parseKeybinds(getSavedKeybinds(state)));
}

// isKeybind tells whether a key press is the shortcut of an action ('' turns a shortcut off).
export function isKeybind(state: GlobalState, e: KeyboardEvent, action: KeyAction): boolean {
    const combo = getKeybinds(state)[action];
    return Boolean(combo) && comboOf(e) === combo;
}

// comboLabel writes a combo as on the keyboard: Ctrl+Shift+K, or ⌘⇧K on macOS.
export function comboLabel(combo: string, mac = navigator.platform.toUpperCase().includes('MAC')): string[] {
    return combo.split('+').map((part) => {
        switch (part) {
        case 'mod': return mac ? '⌘' : 'Ctrl';
        case 'shift': return mac ? '⇧' : 'Shift';
        case 'alt': return mac ? '⌥' : 'Alt';
        case 'space': return 'Space';
        default: return part.length === 1 ? part.toUpperCase() : part.charAt(0).toUpperCase() + part.slice(1);
        }
    });
}

const ARIA_KEYS: Record<string, string> = {shift: 'Shift', alt: 'Alt', space: 'Space'};

// comboKeys writes a combo as aria-keyshortcuts wants it: Control+Shift+K, or Meta+Shift+K on macOS.
export function comboKeys(combo: string, mac = navigator.platform.toUpperCase().includes('MAC')): string {
    return combo.split('+').map((part) => {
        if (part === 'mod') {
            return mac ? 'Meta' : 'Control';
        }
        return ARIA_KEYS[part] || (part.length === 1 ? part.toUpperCase() : part.charAt(0).toUpperCase() + part.slice(1));
    }).join('+');
}
