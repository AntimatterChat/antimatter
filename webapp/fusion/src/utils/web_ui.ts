// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// The server can serve two web UIs, and picks the one to serve for each page load from the cookie below.
export const WebUIs = {
    CLASSIC: 'classic',
    FUSION: 'fusion',
} as const;

export type WebUI = typeof WebUIs[keyof typeof WebUIs];

// The web UI this bundle implements.
export const CURRENT_WEB_UI: WebUI = WebUIs.FUSION;

export const WEB_UI_COOKIE = 'AMWEBUI';

const ONE_YEAR_IN_SECONDS = 365 * 24 * 60 * 60;

// switchWebUI remembers the web UI picked for this browser, then reloads the page to load it.
export function switchWebUI(webUI: WebUI) {
    const path = window.basename || '/';
    const secure = window.location.protocol === 'https:' ? ';secure' : '';
    document.cookie = `${WEB_UI_COOKIE}=${webUI};path=${path};max-age=${ONE_YEAR_IN_SECONDS};samesite=lax${secure}`;
    window.location.reload();
}
