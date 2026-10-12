// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {isDesktopApp} from '@mattermost/shared/utils/user_agent';

// What getDisplayMedia's displaySurface asks the browser's picker to open on.
export type ScreenSurface = 'monitor' | 'window' | 'browser';

// How a screen share is picked, so the share menu offers only choices that do something:
// - 'desktop': the desktop app shows its own source picker (screens and windows);
// - 'surfaces': the browser opens its picker on the kind of source asked for (a whole screen, a window or a tab), as
//   Chromium-based browsers do (Chrome, Edge, Brave, Opera, Electron);
// - 'browser': the browser shows its own picker whatever is asked for (Firefox and Safari list screens and windows,
//   without tabs), as unknown browsers are assumed to.
export type ScreenPicker = 'desktop' | 'surfaces' | 'browser';

export type ScreenPickerEnv = {
    userAgent: string;

    // The desktop app told the page it runs in it (window.desktop, set by the desktop API handshake).
    desktop: boolean;

    // getDisplayMedia exists and the displaySurface constraint is reported as supported.
    displaySurface: boolean;
};

function currentEnv(): ScreenPickerEnv {
    const media = typeof navigator === 'undefined' ? undefined : navigator.mediaDevices;
    let displaySurface = false;
    try {
        displaySurface = typeof media?.getDisplayMedia === 'function' &&
            Boolean((media.getSupportedConstraints?.() as {displaySurface?: boolean} | undefined)?.displaySurface);
    } catch {
        displaySurface = false;
    }
    return {
        userAgent: typeof navigator === 'undefined' ? '' : navigator.userAgent,
        desktop: Boolean((window as {desktop?: unknown}).desktop) || isDesktopApp(),
        displaySurface,
    };
}

// The desktop app adds its name (and Mattermost's, for compatibility) to Electron's user agent.
function isDesktopAppAgent(ua: string): boolean {
    return (/\bElectron\//).test(ua) && (/\b(Antimatter|Mattermost)\//).test(ua);
}

// Chromium's engine: Chrome (headless too) and the browsers built on it, which keep "Chrome/" in their user agent.
// Firefox, iOS browsers (WebKit under another name) and the old EdgeHTML Edge are left out.
function isChromiumAgent(ua: string): boolean {
    return (/\b(Headless)?(Chrome|Chromium)\/\d/).test(ua) && !(/\b(Firefox|FxiOS|CriOS|EdgiOS|Edge)\//).test(ua);
}

// screenPicker says how the browser (or the desktop app) lets the user pick what to share. Firefox reports
// displaySurface as supported but ignores it, so the user agent decides on top of the capability.
export function screenPicker(env: ScreenPickerEnv = currentEnv()): ScreenPicker {
    if (env.desktop || isDesktopAppAgent(env.userAgent)) {
        return 'desktop';
    }
    if (env.displaySurface && isChromiumAgent(env.userAgent)) {
        return 'surfaces';
    }
    return 'browser';
}
