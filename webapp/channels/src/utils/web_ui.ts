// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// This module is part of the page's first script (see root.tsx), so it keeps its imports light.
import {navigateTo, reloadPage} from 'utils/browser_utils';

// The server can serve two web UIs. For each page load, it serves the one in the ?webui= query
// parameter, then the one the signed-in user picked (their web UI preference), then the one picked
// in this browser (the cookie below, which also covers the pages shown before signing in), and
// otherwise the default one.
export const WebUIs = {
    CLASSIC: 'classic',
    FUSION: 'fusion',
} as const;

export type WebUI = typeof WebUIs[keyof typeof WebUIs];

// The web UI this bundle implements.
export const CURRENT_WEB_UI: WebUI = WebUIs.CLASSIC;

export const WEB_UI_COOKIE = 'AMWEBUI';
export const WEB_UI_QUERY_PARAM = 'webui';

// The web UI picked by the user, which follows them to every browser they sign in from.
export const WEB_UI_PREFERENCE = {
    CATEGORY: 'display_settings',
    NAME: 'web_ui',
};

// Set once the user picked a web UI or dismissed the announcement of the Fusion UI.
export const FUSION_ANNOUNCEMENT_PREFERENCE = {
    CATEGORY: 'antimatter_announcements',
    NAME: 'fusion_web_ui',
    DISMISSED: 'dismissed',
};

const ONE_YEAR_IN_SECONDS = 365 * 24 * 60 * 60;

// Reloading to follow the user's preference is skipped when this page was already reloaded for it
// that recently, which would mean the server keeps serving another web UI.
const RELOAD_GUARD_STORAGE_KEY = 'amWebUIReload';
const RELOAD_GUARD_MS = 60 * 1000;

export function isWebUI(value: unknown): value is WebUI {
    return value === WebUIs.CLASSIC || value === WebUIs.FUSION;
}

// setWebUICookie remembers the web UI for this browser, for the pages shown before signing in.
export function setWebUICookie(webUI: WebUI) {
    const path = window.basename || '/';
    const secure = window.location.protocol === 'https:' ? ';secure' : '';
    document.cookie = `${WEB_UI_COOKIE}=${webUI};path=${path};max-age=${ONE_YEAR_IN_SECONDS};samesite=lax${secure}`;
}

// loadWebUI loads the current page in the given web UI. The ?webui= query parameter makes the server
// serve it and remember it for this browser, and that web UI then saves it as the user's preference.
export function loadWebUI(webUI: WebUI) {
    const url = new URL(window.location.href);
    url.searchParams.set(WEB_UI_QUERY_PARAM, webUI);
    navigateTo(url.toString());
}

let requestedWebUI: WebUI | null = null;

// captureRequestedWebUI keeps the web UI requested with ?webui= when this page was loaded, and removes
// the parameter from the address. It runs before the app starts.
export function captureRequestedWebUI() {
    const url = new URL(window.location.href);
    const requested = url.searchParams.get(WEB_UI_QUERY_PARAM);
    if (requested === null) {
        return;
    }

    // The server serves the requested web UI when users may pick their own.
    if (requested === CURRENT_WEB_UI) {
        requestedWebUI = requested;
    }
    url.searchParams.delete(WEB_UI_QUERY_PARAM);
    window.history.replaceState(window.history.state, '', url.toString());
}

// takeRequestedWebUI returns the web UI requested with ?webui= when this page was loaded, once.
export function takeRequestedWebUI(): WebUI | null {
    const requested = requestedWebUI;
    requestedWebUI = null;
    return requested;
}

function reloadedRecentlyFor(webUI: WebUI): boolean {
    try {
        const last = JSON.parse(window.sessionStorage.getItem(RELOAD_GUARD_STORAGE_KEY) || 'null');
        return last?.webUI === webUI && Date.now() - last.at < RELOAD_GUARD_MS;
    } catch {
        return false;
    }
}

function rememberReloadFor(webUI: WebUI): boolean {
    try {
        window.sessionStorage.setItem(RELOAD_GUARD_STORAGE_KEY, JSON.stringify({webUI, at: Date.now()}));
        return true;
    } catch {
        return false;
    }
}

// followPreferredWebUI makes this browser use the web UI preferred by the signed-in user: it saves it
// in the cookie, and reloads the page when it was served another web UI, e.g. when the user signed in
// on a page shown in the web UI picked in this browser. It returns whether the page reloads.
//
// To never loop, it doesn't reload again for the same web UI within a minute, nor when it can't
// remember that it reloaded.
export function followPreferredWebUI(preferred: WebUI): boolean {
    setWebUICookie(preferred);
    if (preferred === CURRENT_WEB_UI || reloadedRecentlyFor(preferred) || !rememberReloadFor(preferred)) {
        return false;
    }
    reloadPage();
    return true;
}
