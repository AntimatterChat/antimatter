// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {shallowEqual, useSelector} from 'react-redux';

import {get as getPreference} from 'mattermost-redux/selectors/entities/preferences';

import {Preferences} from 'utils/constants';

import type {GlobalState} from 'types/store';

// Antimatter's own display preferences, next to Mattermost's.
export const CATEGORY_FUSION_DISPLAY = 'fusion_display';
export const STREAMER_MODE = 'streamer_mode';
export const DEVELOPER_MODE = 'developer_mode';

export type DisplayPrefs = {

    /** Compact message display: no pictures, tighter rows (Mattermost's message display setting). */
    compact: boolean;

    /** Author names in their own colour. */
    colorize: boolean;

    /** Messages in a centred column instead of the full width. */
    centered: boolean;

    /** Image attachments start collapsed. */
    collapsePreviews: boolean;

    /** Recent emoji in the hover toolbar of messages. */
    oneClickReactions: boolean;

    /** Open unread channels at the first unread message rather than the newest. */
    startFromLeftOff: boolean;

    /** Hide personal information and notifications, e.g. while sharing the screen. */
    streamer: boolean;

    /** Copy ID items in menus. */
    developer: boolean;
};

export function getDisplayPrefs(state: GlobalState): DisplayPrefs {
    const get = (category: string, name: string, fallback: string) => getPreference(state, category, name, fallback);
    return {
        compact: get(Preferences.CATEGORY_DISPLAY_SETTINGS, Preferences.MESSAGE_DISPLAY, Preferences.MESSAGE_DISPLAY_DEFAULT) === Preferences.MESSAGE_DISPLAY_COMPACT,
        colorize: get(Preferences.CATEGORY_DISPLAY_SETTINGS, Preferences.COLORIZE_USERNAMES, Preferences.COLORIZE_USERNAMES_DEFAULT) === 'true',
        centered: get(Preferences.CATEGORY_DISPLAY_SETTINGS, Preferences.CHANNEL_DISPLAY_MODE, Preferences.CHANNEL_DISPLAY_MODE_DEFAULT) === Preferences.CHANNEL_DISPLAY_MODE_CENTERED,
        collapsePreviews: get(Preferences.CATEGORY_DISPLAY_SETTINGS, Preferences.COLLAPSE_DISPLAY, Preferences.COLLAPSE_DISPLAY_DEFAULT) === 'true',
        oneClickReactions: get(Preferences.CATEGORY_DISPLAY_SETTINGS, Preferences.ONE_CLICK_REACTIONS_ENABLED, Preferences.ONE_CLICK_REACTIONS_ENABLED_DEFAULT) === 'true',
        startFromLeftOff: get(Preferences.CATEGORY_ADVANCED_SETTINGS, Preferences.UNREAD_SCROLL_POSITION, Preferences.UNREAD_SCROLL_POSITION_START_FROM_LEFT) === Preferences.UNREAD_SCROLL_POSITION_START_FROM_LEFT,
        streamer: get(CATEGORY_FUSION_DISPLAY, STREAMER_MODE, 'false') === 'true',
        developer: get(CATEGORY_FUSION_DISPLAY, DEVELOPER_MODE, 'false') === 'true',
    };
}

export function useDisplayPrefs(): DisplayPrefs {
    return useSelector(getDisplayPrefs, shallowEqual);
}

// nameColor gives a person a colour of their own, stable across sessions, readable on dark and light themes.
export function nameColor(userId: string): string {
    let hash = 0;
    for (let i = 0; i < userId.length; i++) {
        hash = ((hash * 31) + userId.charCodeAt(i)) | 0;
    }
    return `hsl(${Math.abs(hash) % 360} 62% 58%)`;
}
