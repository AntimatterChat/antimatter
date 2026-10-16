// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {act, renderHook} from '@testing-library/react';

import {TOUCH_QUERY} from 'fusion/hooks/media';
import {fakeMatchMedia} from 'fusion/hooks/media_test_utils';

import {useVisualViewport} from './viewport';

// A visual viewport (iOS shrinks and pans it for the on-screen keyboard) the tests move.
function fakeVisualViewport() {
    const listeners: Record<string, Set<() => void>> = {resize: new Set(), scroll: new Set()};
    const vv = {
        height: 800,
        offsetTop: 0,
        scale: 1,
        addEventListener: (type: string, cb: () => void) => listeners[type].add(cb),
        removeEventListener: (type: string, cb: () => void) => listeners[type].delete(cb),
    };
    Object.defineProperty(window, 'visualViewport', {value: vv, configurable: true});
    return {
        vv,
        fire: (type: 'resize' | 'scroll') => act(() => listeners[type].forEach((cb) => cb())),
    };
}

const prop = (name: string) => document.documentElement.style.getPropertyValue(name);

describe('fusion/shell/useVisualViewport', () => {
    const originalMatchMedia = window.matchMedia;
    afterEach(() => {
        window.matchMedia = originalMatchMedia;
        Object.defineProperty(window, 'visualViewport', {value: undefined, configurable: true});
        document.documentElement.removeAttribute('style');
    });

    test('on a touch screen, the frame takes the visual viewport\'s height and follows it when it pans', () => {
        fakeMatchMedia((q) => q === TOUCH_QUERY);
        const {vv, fire} = fakeVisualViewport();
        renderHook(() => useVisualViewport());
        expect(prop('--am-vvh')).toBe('800px');
        expect(prop('--am-vv-shift')).toBe('');

        // The keyboard comes up, and iOS pans the page.
        vv.height = 455.6;
        vv.offsetTop = 120.4;
        fire('resize');
        fire('scroll');
        expect(prop('--am-vvh')).toBe('456px');
        expect(prop('--am-vv-shift')).toBe('translateY(120px)');

        vv.offsetTop = 0;
        fire('scroll');
        expect(prop('--am-vv-shift')).toBe('');
    });

    test('a zoomed-in page is left alone', () => {
        fakeMatchMedia((q) => q === TOUCH_QUERY);
        const {vv, fire} = fakeVisualViewport();
        renderHook(() => useVisualViewport());
        vv.scale = 2;
        fire('resize');
        expect(prop('--am-vvh')).toBe('');
    });

    test('with a mouse, nothing is set', () => {
        fakeMatchMedia(() => false);
        fakeVisualViewport();
        renderHook(() => useVisualViewport());
        expect(prop('--am-vvh')).toBe('');
    });

    test('unmounting clears what it set', () => {
        fakeMatchMedia((q) => q === TOUCH_QUERY);
        const {vv} = fakeVisualViewport();
        vv.offsetTop = 50;
        const {unmount} = renderHook(() => useVisualViewport());
        expect(prop('--am-vv-shift')).toBe('translateY(50px)');
        unmount();
        expect(prop('--am-vvh')).toBe('');
        expect(prop('--am-vv-shift')).toBe('');
    });
});
