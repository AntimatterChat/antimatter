// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {renderHook} from '@testing-library/react';

import {TOUCH_QUERY} from 'fusion/hooks/media';
import {fakeMatchMedia} from 'fusion/hooks/media_test_utils';

import {useLongPress} from './long_press';

function touch(target: Element, type: 'touchstart' | 'touchmove' | 'touchend', points: Array<{x: number; y: number}>) {
    const event = new Event(type, {bubbles: true, cancelable: true});
    Object.defineProperty(event, 'touches', {value: points.map(({x, y}) => ({clientX: x, clientY: y}))});
    target.dispatchEvent(event);
}

describe('fusion/shell/useLongPress', () => {
    const originalMatchMedia = window.matchMedia;
    let contextMenus: MouseEvent[];
    const onContextMenu = (e: MouseEvent) => contextMenus.push(e);

    beforeEach(() => {
        jest.useFakeTimers();
        fakeMatchMedia((q) => q === TOUCH_QUERY);
        contextMenus = [];
        document.body.innerHTML = `
            <div class="am-msg" id="msg">
                <p id="text">Hello</p>
                <span id="name">Nina</span>
                <div class="am-msg-tools"><button aria-haspopup="menu" id="more">More</button></div>
            </div>
            <input id="field"/>`;
        document.addEventListener('contextmenu', onContextMenu);
    });

    afterEach(() => {
        document.removeEventListener('contextmenu', onContextMenu);
        window.matchMedia = originalMatchMedia;
        jest.useRealTimers();
    });

    const el = (id: string) => document.getElementById(id)!;

    test('a finger held still is a right click where it rests', () => {
        renderHook(() => useLongPress());
        touch(el('name'), 'touchstart', [{x: 30, y: 40}]);
        jest.advanceTimersByTime(549);
        expect(contextMenus).toHaveLength(0);
        jest.advanceTimersByTime(1);
        expect(contextMenus).toHaveLength(1);
        expect(contextMenus[0].target).toBe(el('name'));
        expect(contextMenus[0]).toMatchObject({clientX: 30, clientY: 40});
    });

    test('lifting or moving the finger is not a long press', () => {
        renderHook(() => useLongPress());
        touch(el('name'), 'touchstart', [{x: 30, y: 40}]);
        touch(el('name'), 'touchend', []);
        jest.advanceTimersByTime(1000);

        touch(el('name'), 'touchstart', [{x: 30, y: 40}]);
        touch(el('name'), 'touchmove', [{x: 30, y: 52}]);
        jest.advanceTimersByTime(1000);
        expect(contextMenus).toHaveLength(0);

        // A little wobble is fine.
        touch(el('name'), 'touchstart', [{x: 30, y: 40}]);
        touch(el('name'), 'touchmove', [{x: 33, y: 44}]);
        jest.advanceTimersByTime(1000);
        expect(contextMenus).toHaveLength(1);
    });

    test('fields, two fingers and mice are left to the browser', () => {
        renderHook(() => useLongPress());
        touch(el('field'), 'touchstart', [{x: 1, y: 1}]);
        jest.advanceTimersByTime(1000);
        touch(el('name'), 'touchstart', [{x: 1, y: 1}, {x: 50, y: 50}]);
        jest.advanceTimersByTime(1000);

        fakeMatchMedia(() => false);
        touch(el('name'), 'touchstart', [{x: 1, y: 1}]);
        jest.advanceTimersByTime(1000);
        expect(contextMenus).toHaveLength(0);
    });

    test('a long press on a message opens its menu, and the lifting finger doesn\'t also tap', () => {
        const more = jest.fn();
        el('more').addEventListener('click', more);
        const tapped = jest.fn();
        el('text').addEventListener('click', tapped);
        Object.defineProperty(navigator, 'vibrate', {value: jest.fn(), configurable: true});
        renderHook(() => useLongPress());

        touch(el('text'), 'touchstart', [{x: 5, y: 5}]);
        jest.advanceTimersByTime(600);
        expect(more).toHaveBeenCalledTimes(1);
        expect(navigator.vibrate).toHaveBeenCalled();

        el('text').click();
        expect(tapped).not.toHaveBeenCalled();

        // The next tap is a tap.
        el('text').click();
        expect(tapped).toHaveBeenCalledTimes(1);
    });

    test('what has a menu of its own keeps it', () => {
        const more = jest.fn();
        el('more').addEventListener('click', more);
        el('name').addEventListener('contextmenu', (e) => e.preventDefault());
        renderHook(() => useLongPress());
        touch(el('name'), 'touchstart', [{x: 5, y: 5}]);
        jest.advanceTimersByTime(600);
        expect(more).not.toHaveBeenCalled();
    });

    test('stops when unmounted', () => {
        const {unmount} = renderHook(() => useLongPress());
        touch(el('name'), 'touchstart', [{x: 5, y: 5}]);
        unmount();
        jest.advanceTimersByTime(1000);
        expect(contextMenus).toHaveLength(0);
    });
});
