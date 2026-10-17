// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {act, renderHook} from '@testing-library/react';

import {computeDockRows, useDockFit} from './dock_fit';

const mockSetDockRows = jest.fn();
jest.mock('fusion/shell/layout_context', () => ({useLayout: () => ({setDockRows: mockSetDockRows})}));

describe('fusion/sidebar/computeDockRows', () => {
    const measures = {sidebar: 600, others: 50, channels: 800, chrome: 50, row: 34, gap: 2};

    test('fits the rows in what the channel list leaves, the list keeping up to 60% of the sidebar', () => {
        // 600 - 50 - 360 - 50 = 140px: 4 rows of 32px with their gaps.
        expect(computeDockRows(measures)).toBe(4);
    });

    test('gives a short channel list\'s spare room to the dock', () => {
        expect(computeDockRows({...measures, channels: 100})).toBe(11);
    });

    test('fits none when not even one row has room', () => {
        expect(computeDockRows({...measures, sidebar: 300})).toBe(0);
        expect(computeDockRows({...measures, sidebar: 100})).toBe(0);
    });
});

describe('fusion/sidebar/useDockFit', () => {
    let resized: () => void = () => {};
    class MockResizeObserver {
        constructor(callback: () => void) {
            resized = callback;
        }
        observe() {}
        disconnect() {}
    }

    const size = {aside: 600, list: 400, content: 800};
    const sized = (el: HTMLElement, props: Record<string, () => number>) => {
        for (const [name, get] of Object.entries(props)) {
            Object.defineProperty(el, name, {configurable: true, get});
        }
        return el;
    };

    beforeEach(() => {
        jest.clearAllMocks();
        (window as unknown as {ResizeObserver: unknown}).ResizeObserver = MockResizeObserver;
        jest.spyOn(window, 'requestAnimationFrame').mockImplementation((cb) => {
            cb(0);
            return 1;
        });
    });

    const setup = () => {
        const aside = sized(document.createElement('aside'), {clientHeight: () => size.aside});
        const list = sized(document.createElement('div'), {offsetHeight: () => size.list, scrollHeight: () => size.content});
        const dock = sized(document.createElement('div'), {offsetHeight: () => 150});
        const rows = sized(document.createElement('div'), {offsetHeight: () => 100});
        rows.className = 'am-dock-list';
        rows.style.rowGap = '2px';
        rows.appendChild(sized(document.createElement('div'), {offsetHeight: () => 32}));
        dock.appendChild(rows);
        aside.append(list, dock);
        return renderHook(() => useDockFit({sidebar: {current: aside}, scroll: {current: list}, dock: {current: dock}}));
    };

    test('measures how many conversations fit, and measures again when the sidebar is resized', () => {
        size.aside = 600;
        size.list = 400;
        setup();
        expect(mockSetDockRows).toHaveBeenLastCalledWith(4);

        // A taller window: the channel list keeps 60% of it, the dock gets the rest.
        size.aside = 800;
        size.list = 600;
        act(() => resized());
        expect(mockSetDockRows).toHaveBeenLastCalledWith(6);
    });

    test('keeps the last measure while the sidebar is hidden', () => {
        size.aside = 600;
        size.list = 400;
        setup();
        mockSetDockRows.mockClear();

        size.aside = 0;
        act(() => resized());
        expect(mockSetDockRows).not.toHaveBeenCalled();
    });
});
