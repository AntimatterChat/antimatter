// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

// An app's side panel (registerRightHandSidebarComponent) says where it is with the setPanelState prop the host
// gives it: a short string the app defines, such as 'note/<id>' or 'board/<id>/card/<id>'. Opened in a new window,
// the panel gets it back as its initialPanelState prop, so it starts where it was. The string travels in the
// window's URL: it names what is shown, never a secret, and the app fetches that item again as the person signed in
// there, so it gives no access by itself.

// The query parameter of the popout URL (/_popout/rhs/<team>/plugin/<id>?state=...) that holds it.
export const PANEL_STATE_PARAM = 'state';

// The longest state kept: a few ids and words. Anything longer is dropped, not cut.
export const MAX_PANEL_STATE_LENGTH = 512;

// normalizePanelState returns the state an app gave, or undefined for its home: nothing, an empty string, something
// that isn't a string, longer than MAX_PANEL_STATE_LENGTH, or with control characters (line breaks included).
export function normalizePanelState(state: unknown): string | undefined {
    if (typeof state !== 'string' || !state || state.length > MAX_PANEL_STATE_LENGTH) {
        return undefined;
    }
    for (let i = 0; i < state.length; i++) {
        const code = state.charCodeAt(i);
        if (code < 0x20 || code === 0x7f) {
            return undefined;
        }
    }
    return state;
}

// readPanelState returns the state in a URL's query string (location.search), or undefined.
export function readPanelState(search: string): string | undefined {
    return normalizePanelState(new URLSearchParams(search).get(PANEL_STATE_PARAM));
}

// withPanelState returns a URL's query string (location.search) with the state set, or without it when the panel is
// on its home; the other parameters stay as they are. URLSearchParams escapes the state.
export function withPanelState(search: string, state: unknown): string {
    const params = new URLSearchParams(search);
    const normalized = normalizePanelState(state);
    if (normalized) {
        params.set(PANEL_STATE_PARAM, normalized);
    } else {
        params.delete(PANEL_STATE_PARAM);
    }
    const query = params.toString();
    return query ? `?${query}` : '';
}
