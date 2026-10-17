// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {MAX_PANEL_STATE_LENGTH, PANEL_STATE_PARAM, normalizePanelState, readPanelState, withPanelState} from './panel_state';

describe('normalizePanelState', () => {
    it('keeps a short string the app defines', () => {
        expect(normalizePanelState('note/abc123')).toBe('note/abc123');
        expect(normalizePanelState('board/b1/card/c2')).toBe('board/b1/card/c2');
    });

    it('is the home (undefined) for nothing, an empty string, or what is not a string', () => {
        expect(normalizePanelState(undefined)).toBeUndefined();
        expect(normalizePanelState(null)).toBeUndefined();
        expect(normalizePanelState('')).toBeUndefined();
        expect(normalizePanelState(42)).toBeUndefined();
        expect(normalizePanelState({note: 'abc'})).toBeUndefined();
    });

    it('drops a state that is too long, rather than cutting it', () => {
        expect(normalizePanelState('a'.repeat(MAX_PANEL_STATE_LENGTH))).toHaveLength(MAX_PANEL_STATE_LENGTH);
        expect(normalizePanelState('a'.repeat(MAX_PANEL_STATE_LENGTH + 1))).toBeUndefined();
    });

    it('drops a state with control characters', () => {
        expect(normalizePanelState('note/a\nb')).toBeUndefined();
        expect(normalizePanelState('note/a\u0000')).toBeUndefined();
        expect(normalizePanelState('note/a\u007f')).toBeUndefined();
    });
});

describe('withPanelState and readPanelState', () => {
    it('round-trips the state through the query string, escaped', () => {
        const state = 'note/a b&c=d?e#f/ü';
        const search = withPanelState('?channel=general', state);

        // Escaped: nothing in it can end the parameter, start another, or a fragment
        expect(search.startsWith('?channel=general&state=')).toBe(true);
        expect(search).not.toContain('#');
        expect(search.split('&')).toHaveLength(2);
        expect(readPanelState(search)).toBe(state);
        expect(new URLSearchParams(search).get('channel')).toBe('general');
    });

    it('removes the state when the panel is back on its home, keeping the other parameters', () => {
        expect(withPanelState('?channel=general&state=note%2Fabc', null)).toBe('?channel=general');
        expect(withPanelState('?channel=general&state=note%2Fabc', '')).toBe('?channel=general');
        expect(withPanelState('?state=note%2Fabc', undefined)).toBe('');
    });

    it('replaces an older state', () => {
        expect(readPanelState(withPanelState('?state=note%2Fold', 'note/new'))).toBe('note/new');
    });

    it('reads no state from a URL without one, or with one it would not keep', () => {
        expect(readPanelState('')).toBeUndefined();
        expect(readPanelState('?channel=general')).toBeUndefined();
        expect(readPanelState(`?${PANEL_STATE_PARAM}=`)).toBeUndefined();
        expect(readPanelState(`?${PANEL_STATE_PARAM}=${'a'.repeat(MAX_PANEL_STATE_LENGTH + 1)}`)).toBeUndefined();
        expect(readPanelState(`?${PANEL_STATE_PARAM}=note%2Fa%0Ab`)).toBeUndefined();
    });
});
