// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {fireEvent, renderHook, screen} from '@testing-library/react';

import {LAYER_ID} from 'fusion/components/layer';

import {placeTip, TIP_MARGIN, useFaceTips} from './face_tips';

describe('fusion/shell/useFaceTips', () => {
    beforeEach(() => {
        document.body.innerHTML = `<div id="${LAYER_ID}"></div><span id="mark" data-am-tip="Private"></span><span id="plain"></span>`;
    });

    test('shows the Fusion tooltip of a data-am-tip element under the pointer, and hides it after', () => {
        const {unmount} = renderHook(() => useFaceTips());

        fireEvent.mouseOver(document.getElementById('mark')!);
        expect(screen.getByRole('tooltip')).toHaveTextContent('Private');
        expect(document.getElementById(LAYER_ID)).toContainElement(screen.getByRole('tooltip'));

        fireEvent.mouseOver(document.getElementById('plain')!);
        expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
        unmount();
    });

    test('names a data-am-tip element the keyboard moves to, until the focus leaves it', () => {
        const {unmount} = renderHook(() => useFaceTips());
        const mark = document.getElementById('mark')!;
        jest.spyOn(mark, 'matches').mockImplementation((selector: string) => selector === ':focus-visible');

        fireEvent.focusIn(mark);
        expect(screen.getByRole('tooltip')).toHaveTextContent('Private');

        fireEvent.focusOut(mark);
        expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
        unmount();
    });

    test('hides the tooltip on Escape, whether the pointer or the keyboard showed it', () => {
        const {unmount} = renderHook(() => useFaceTips());
        const mark = document.getElementById('mark')!;

        fireEvent.mouseOver(mark);
        expect(screen.getByRole('tooltip')).toHaveTextContent('Private');
        fireEvent.keyDown(document.body, {key: 'Escape'});
        expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();

        jest.spyOn(mark, 'matches').mockImplementation((selector: string) => selector === ':focus-visible');
        fireEvent.focusIn(mark);
        expect(screen.getByRole('tooltip')).toHaveTextContent('Private');
        fireEvent.keyDown(mark, {key: 'Escape'});
        expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
        unmount();
    });

    test('keeps the tooltip of the pointer when the focus moves elsewhere', () => {
        const {unmount} = renderHook(() => useFaceTips());

        fireEvent.mouseOver(document.getElementById('mark')!);
        fireEvent.focusOut(document.getElementById('plain')!);
        expect(screen.getByRole('tooltip')).toHaveTextContent('Private');
        unmount();
    });

    test('opens a tooltip below an element at the top of the window, its arrow pointing up', () => {
        const {unmount} = renderHook(() => useFaceTips());
        const mark = document.getElementById('mark')!;
        jest.spyOn(mark, 'getBoundingClientRect').mockReturnValue({left: 100, right: 132, top: 10, bottom: 42, width: 32, height: 32, x: 100, y: 10, toJSON: () => ({})});

        fireEvent.mouseOver(mark);
        const tip = screen.getByRole('tooltip');
        expect(tip).toHaveClass('am-tip-below');
        expect(tip.dataset.side).toBe('below');
        expect(tip.style.top).toBe('50px');
        unmount();
    });

    test('keeps a tooltip above an element with room above it', () => {
        const {unmount} = renderHook(() => useFaceTips());
        const mark = document.getElementById('mark')!;
        jest.spyOn(mark, 'getBoundingClientRect').mockReturnValue({left: 100, right: 132, top: 300, bottom: 332, width: 32, height: 32, x: 100, y: 300, toJSON: () => ({})});

        fireEvent.mouseOver(mark);
        const tip = screen.getByRole('tooltip');
        expect(tip).not.toHaveClass('am-tip-below');
        expect(tip.dataset.side).toBe('above');
        unmount();
    });
});

describe('fusion/shell/placeTip', () => {
    const view = {width: 1200, height: 800};
    const tip = {width: 80, height: 28};
    const at = (left: number, top: number, size = 32) => ({left, top, right: left + size, bottom: top + size, width: size, height: size});

    test('is above its element, centred, when there is room', () => {
        expect(placeTip(at(400, 300), tip, view)).toEqual({side: 'above', left: 376, top: 300 - 28 - 8, arrowX: 40});
    });

    test('flips below an element at the top edge (a panel header, a toolbar), where above would be cut off', () => {
        const place = placeTip(at(400, 10), tip, view);
        expect(place.side).toBe('below');
        expect(place.top).toBe(10 + 32 + 8);
    });

    test('flips below as soon as above would leave less than the margin', () => {
        expect(placeTip(at(400, 28 + 8 + TIP_MARGIN), tip, view).side).toBe('above');
        expect(placeTip(at(400, (28 + 8 + TIP_MARGIN) - 1), tip, view).side).toBe('below');
    });

    test('is below when asked, unless there is no room below', () => {
        expect(placeTip(at(400, 300), tip, view, 'below').side).toBe('below');
        expect(placeTip(at(400, 760), tip, view, 'below').side).toBe('above');
    });

    test('takes the side with more room when neither fits', () => {
        const short = {width: 1200, height: 60};
        expect(placeTip(at(400, 4, 20), tip, short).side).toBe('below');
        expect(placeTip(at(400, 36, 20), tip, short).side).toBe('above');
    });

    test('stays in the window at its right edge, its arrow still on its element', () => {
        const place = placeTip(at(1150, 10, 16), tip, view);
        expect(place.left).toBe(1200 - 80 - TIP_MARGIN);
        expect(place.arrowX).toBe(1158 - place.left);
    });

    test('keeps the arrow on the tooltip for an element past its end', () => {
        expect(placeTip(at(1195, 300, 4), tip, view).arrowX).toBe(80 - 10);
    });

    test('is on the right of its element when asked, whatever the room above', () => {
        expect(placeTip(at(10, 0), tip, view, 'right')).toEqual({side: 'right', left: 52, top: 16 - 14});
    });
});
