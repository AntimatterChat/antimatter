// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useSyncExternalStore} from 'react';

// The Fusion UI's breakpoints, the same as styles/_breakpoints.scss.

// Compact: the navigation and the right-hand panels slide over the conversation as drawers. Phones held sideways
// count, however wide they are.
export const COMPACT_QUERY = '(max-width: 860px), (max-width: 960px) and (max-height: 500px)';

// Phone: one pane at a time. Panels, dialogs and floating windows take the whole screen, popovers and menus open as
// sheets from the bottom.
export const PHONE_QUERY = '(max-width: 640px), (max-width: 960px) and (max-height: 500px)';

// A finger rather than a mouse: no hover, long presses instead of right clicks.
export const TOUCH_QUERY = '(hover: none) and (pointer: coarse)';

// The system's reduced motion setting: no animations.
export const REDUCED_MOTION_QUERY = '(prefers-reduced-motion: reduce)';

const matches = (query: string) => typeof window !== 'undefined' && Boolean(window.matchMedia?.(query).matches);

export const isCompact = () => matches(COMPACT_QUERY);
export const isPhone = () => matches(PHONE_QUERY);
export const isTouch = () => matches(TOUCH_QUERY);

// useMediaQuery follows a media query, re-rendering when it starts or stops matching (a phone turned sideways, a
// window resized).
export function useMediaQuery(query: string): boolean {
    return useSyncExternalStore(
        (onChange) => {
            const media = window.matchMedia?.(query);
            media?.addEventListener('change', onChange);
            return () => media?.removeEventListener('change', onChange);
        },
        () => matches(query),
        () => false,
    );
}

export const useIsPhone = () => useMediaQuery(PHONE_QUERY);
export const useIsCompact = () => useMediaQuery(COMPACT_QUERY);
export const useReducedMotion = () => useMediaQuery(REDUCED_MOTION_QUERY);
