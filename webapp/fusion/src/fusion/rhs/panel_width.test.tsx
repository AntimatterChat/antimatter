// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useRef} from 'react';

import {setRhsExpanded} from 'actions/views/rhs';

import {act, fireEvent, renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import {clampWidth, fitWidth, keyWidth, PanelResizeHandle, readBounds, RestoreWidthButton, usePanelWidth, WIDTH_STEP} from './panel_width';

const bounds = {min: 400, max: 800};

// jsdom has no pointer events: mouse events with a pointer id stand in for them.
if (!window.PointerEvent) {
    class PointerEvent extends MouseEvent {
        pointerId: number;
        constructor(type: string, init: PointerEventInit = {}) {
            super(type, init);
            this.pointerId = init.pointerId ?? 0;
        }
    }
    window.PointerEvent = PointerEvent as typeof window.PointerEvent;
}

describe('fusion/rhs/panel_width', () => {
    test('keeps widths within the bounds, the minimum being the default', () => {
        expect(clampWidth(300, bounds)).toBe(400);
        expect(clampWidth(612.4, bounds)).toBe(612);
        expect(clampWidth(1200, bounds)).toBe(800);

        // A window too small to widen the panel leaves it its default width.
        expect(clampWidth(500, {min: 400, max: 300})).toBe(400);

        expect(fitWidth(null, bounds)).toBeNull();
        expect(fitWidth(400, bounds)).toBeNull();
        expect(fitWidth(390, bounds)).toBeNull();
        expect(fitWidth(401, bounds)).toBe(401);
        expect(fitWidth(5000, bounds)).toBe(800);
    });

    test('moves the left edge with the arrow keys, Home and End', () => {
        expect(keyWidth('ArrowLeft', null, bounds)).toBe(400 + WIDTH_STEP);
        expect(keyWidth('ArrowLeft', 790, bounds)).toBe(800);
        expect(keyWidth('ArrowRight', 500, bounds)).toBe(500 - WIDTH_STEP);
        expect(keyWidth('ArrowRight', 410, bounds)).toBeNull();
        expect(keyWidth('ArrowRight', null, bounds)).toBeNull();
        expect(keyWidth('Home', 600, bounds)).toBeNull();
        expect(keyWidth('End', null, bounds)).toBe(800);
        expect(keyWidth('Enter', 600, bounds)).toBeUndefined();
    });

    test('reads the bounds from the style, none where the panel has none', () => {
        const panel = document.body.appendChild(document.createElement('div'));
        expect(readBounds(panel)).toBeNull();
        panel.style.minWidth = '414px';
        panel.style.maxWidth = '900px';
        expect(readBounds(panel)).toEqual({min: 414, max: 900});
        panel.style.maxWidth = '300px';
        expect(readBounds(panel)).toEqual({min: 414, max: 414});
        panel.remove();
    });

    describe('usePanelWidth', () => {
        // The panel's width: what the user chose, else its minimum (jsdom lays nothing out).
        beforeEach(() => {
            jest.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function(this: HTMLElement) {
                const width = parseFloat(this.dataset.width || '') || parseFloat(this.style.minWidth) || 0;
                return {width, height: 0, top: 0, left: 0, right: width, bottom: 0, x: 0, y: 0, toJSON: () => ({})} as DOMRect;
            });
        });
        afterEach(() => jest.restoreAllMocks());

        function Panel({active, kind}: {active: boolean; kind?: string}) {
            const ref = useRef<HTMLDivElement>(null);
            const panel = usePanelWidth(ref, active, kind);
            return (
                <div
                    ref={ref}
                    data-testid='panel'
                    data-width={panel.width ?? ''}
                    style={{minWidth: kind === 'roomy' ? '700px' : '400px', maxWidth: '800px'}}
                >
                    {active && <PanelResizeHandle panel={panel}/>}
                    <RestoreWidthButton
                        sized={panel.width !== null}
                        onRestore={panel.reset}
                    />
                </div>
            );
        }

        const render = (active = true) => renderWithContext(<Panel active={active}/>, {views: {rhs: {isSidebarExpanded: false}}} as never);
        const width = () => screen.getByTestId('panel').dataset.width;

        test('drags the left edge within the bounds, the default width at the minimum', () => {
            const {store} = render();
            const handle = screen.getByRole('separator', {name: 'Resize the panel'});
            expect(handle).toHaveAttribute('aria-valuemin', '400');
            expect(handle).toHaveAttribute('aria-valuemax', '800');
            expect(handle).toHaveAttribute('aria-valuenow', '400');

            fireEvent.pointerDown(handle, {button: 0, clientX: 1000, pointerId: 1});
            fireEvent.pointerMove(handle, {clientX: 850, pointerId: 1});
            expect(width()).toBe('550');
            expect(handle).toHaveAttribute('aria-valuenow', '550');
            expect(store.getState().views.rhs.isSidebarExpanded).toBe(true);

            fireEvent.pointerMove(handle, {clientX: 0, pointerId: 1});
            expect(width()).toBe('800');
            fireEvent.pointerMove(handle, {clientX: 1100, pointerId: 1});
            expect(width()).toBe('');
            expect(store.getState().views.rhs.isSidebarExpanded).toBe(false);

            fireEvent.pointerMove(handle, {clientX: 900, pointerId: 1});
            fireEvent.pointerUp(handle, {pointerId: 1});
            expect(width()).toBe('500');

            // Moving the pointer after the drag changes nothing.
            fireEvent.pointerMove(handle, {clientX: 500, pointerId: 1});
            expect(width()).toBe('500');
        });

        test('resizes with the keyboard', async () => {
            render();
            const handle = screen.getByRole('separator');
            handle.focus();
            await userEvent.keyboard('{ArrowLeft}{ArrowLeft}');
            expect(width()).toBe(String(400 + (2 * WIDTH_STEP)));
            await userEvent.keyboard('{ArrowRight}');
            expect(width()).toBe(String(400 + WIDTH_STEP));
            await userEvent.keyboard('{End}');
            expect(width()).toBe('800');
            await userEvent.keyboard('{Home}');
            expect(width()).toBe('');
        });

        test('shows the restore button only once resized, and restores the default width', async () => {
            render();
            expect(screen.queryByRole('button', {name: "Restore the panel's size"})).toBeNull();
            const handle = screen.getByRole('separator');
            handle.focus();
            await userEvent.keyboard('{End}');
            const restore = screen.getByRole('button', {name: "Restore the panel's size"});
            expect(restore).toHaveAttribute('data-am-tip', "Restore the panel's size");
            await userEvent.click(restore);
            expect(width()).toBe('');
            expect(screen.queryByRole('button', {name: "Restore the panel's size"})).toBeNull();
        });

        test('forgets the width when the panel closes', async () => {
            const {rerender} = render();
            screen.getByRole('separator').focus();
            await userEvent.keyboard('{End}');
            expect(width()).toBe('800');

            rerender(<Panel active={false}/>);
            expect(width()).toBe('');
            rerender(<Panel active={true}/>);
            expect(width()).toBe('');
        });

        test('keeps the width while the panel stays open, within the next app\'s bounds', async () => {
            const {rerender} = render();
            screen.getByRole('separator').focus();
            await userEvent.keyboard('{ArrowLeft}');
            rerender(<Panel active={true}/>);
            expect(width()).toBe(String(400 + WIDTH_STEP));

            // An app with a wider default width starts from it.
            rerender(
                <Panel
                    active={true}
                    kind='roomy'
                />,
            );
            expect(width()).toBe('');
        });

        test('follows the window: a width out of the new bounds comes back in', async () => {
            render();
            screen.getByRole('separator').focus();
            await userEvent.keyboard('{End}');
            const panel = screen.getByTestId('panel');
            panel.style.maxWidth = '600px';
            act(() => {
                window.dispatchEvent(new Event('resize'));
            });
            expect(width()).toBe('600');
            panel.style.maxWidth = '300px';
            act(() => {
                window.dispatchEvent(new Event('resize'));
            });
            expect(width()).toBe('');
            expect(screen.queryByRole('separator')).toBeNull();
        });

        test('a plugin or the shortcut expanding the panel gives it all the room; collapsing restores it', () => {
            const {store} = render();
            act(() => {
                store.dispatch(setRhsExpanded(true));
            });
            expect(width()).toBe('800');
            act(() => {
                store.dispatch(setRhsExpanded(false));
            });
            expect(width()).toBe('');
        });

        test('opens at the default width, not expanded, even after an expanded thread', () => {
            const {store, rerender} = renderWithContext(<Panel active={false}/>, {views: {rhs: {isSidebarExpanded: true}}} as never);
            rerender(<Panel active={true}/>);
            expect(width()).toBe('');
            expect(store.getState().views.rhs.isSidebarExpanded).toBe(false);
        });
    });

    test('RestoreWidthButton', async () => {
        const onRestore = jest.fn();
        const {rerender} = renderWithContext(
            <RestoreWidthButton
                sized={false}
                onRestore={onRestore}
            />,
        );
        expect(screen.queryByRole('button')).toBeNull();
        rerender(
            <RestoreWidthButton
                sized={true}
                onRestore={onRestore}
            />,
        );
        await userEvent.click(screen.getByRole('button', {name: "Restore the panel's size"}));
        expect(onRestore).toHaveBeenCalledTimes(1);
    });
});
