// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {act, renderHook} from '@testing-library/react';

import {COMPACT_QUERY, isCompact, isPhone, isTouch, PHONE_QUERY, TOUCH_QUERY, useIsCompact, useIsPhone, useMediaQuery} from './media';
import {fakeMatchMedia} from './media_test_utils';

describe('fusion/hooks/media', () => {
    const original = window.matchMedia;
    afterEach(() => {
        window.matchMedia = original;
    });

    test('tells a phone, a compact window and a touch screen by their media queries', () => {
        fakeMatchMedia((q) => q === PHONE_QUERY || q === TOUCH_QUERY);
        expect(isPhone()).toBe(true);
        expect(isTouch()).toBe(true);
        expect(isCompact()).toBe(false);
    });

    test('the breakpoints match the styles: phones up to 640px, compact up to 860px, both when held sideways', () => {
        expect(PHONE_QUERY).toContain('(max-width: 640px)');
        expect(COMPACT_QUERY).toContain('(max-width: 860px)');
        expect(PHONE_QUERY).toContain('(max-height: 500px)');
        expect(COMPACT_QUERY).toContain('(max-height: 500px)');
        expect(TOUCH_QUERY).toBe('(hover: none) and (pointer: coarse)');
    });

    test('without matchMedia, nothing matches', () => {
        // @ts-expect-error: an old browser, or a test environment
        window.matchMedia = undefined;
        expect(isPhone()).toBe(false);
        expect(isTouch()).toBe(false);
    });

    test('follows the query as it starts and stops matching', () => {
        const media = fakeMatchMedia(() => false);
        const phone = renderHook(() => useIsPhone());
        const compact = renderHook(() => useIsCompact());
        expect(phone.result.current).toBe(false);

        act(() => media.set((q) => q === PHONE_QUERY || q === COMPACT_QUERY));
        expect(phone.result.current).toBe(true);
        expect(compact.result.current).toBe(true);

        act(() => media.set(() => false));
        expect(phone.result.current).toBe(false);
    });

    test('stops listening when unmounted', () => {
        const media = fakeMatchMedia(() => false);
        const {unmount} = renderHook(() => useMediaQuery('(max-width: 1px)'));
        expect(media.listenerCount()).toBeGreaterThan(0);
        unmount();
        expect(media.listenerCount()).toBe(0);
    });
});
