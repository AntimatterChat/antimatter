// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useEffect} from 'react';

import {LAYER_ID} from 'fusion/components/layer';

// The stacked faces that name their person on hover (thread summaries, acknowledgements), and the elements that give
// their tooltip in a data-am-tip attribute (reactions, the left rail, the apps' panels), above them or, with
// data-am-tip-side="right", on their right, centred. A tooltip with no room above (a panel's header, a toolbar at the
// top of the window) opens below instead; data-am-tip-side="below" asks for below from the start.
const SCOPE = '.am-faces .am-av[data-name], [data-am-tip]';

// The room a tooltip keeps from the window's edges, and the gap between it and its element.
export const TIP_MARGIN = 8;
const TIP_GAP = 8;

// The arrow's distance from the tooltip's ends, at the least, so that it stays on its rounded box.
const ARROW_INSET = 10;

export type TipSide = 'above' | 'below' | 'right';

export type TipPlace = {
    left: number;
    top: number;
    side: TipSide;

    // Where the arrow points, from the tooltip's left (above or below): its element's centre, which the tooltip may
    // have left to stay in the window.
    arrowX?: number;
};

type Box = Pick<DOMRect, 'left' | 'top' | 'right' | 'bottom' | 'width' | 'height'>;

// placeTip is where a tooltip of size tip goes for an element at rect, in a window of size view: on its right when
// asked, else above it when there is room and below it when there is none (or when asked), centred on it and kept in
// the window.
export function placeTip(rect: Box, tip: {width: number; height: number}, view: {width: number; height: number}, asked?: string): TipPlace {
    if (asked === 'right') {
        return {side: 'right', left: rect.right + 10, top: (rect.top + (rect.height / 2)) - (tip.height / 2)};
    }
    const centre = rect.left + (rect.width / 2);
    const left = Math.max(TIP_MARGIN, Math.min(centre - (tip.width / 2), view.width - tip.width - TIP_MARGIN));
    const arrowX = Math.max(ARROW_INSET, Math.min(centre - left, tip.width - ARROW_INSET));
    const above = rect.top - tip.height - TIP_GAP;
    const below = rect.bottom + TIP_GAP;
    const roomAbove = above >= TIP_MARGIN;
    const roomBelow = below + tip.height <= view.height - TIP_MARGIN;

    // Below when asked or when there's no room above, unless there's no room below either: then the side with the
    // more room.
    let side: TipSide = asked === 'below' || !roomAbove ? 'below' : 'above';
    if (side === 'below' && !roomBelow && (asked !== 'below' || roomAbove)) {
        side = roomAbove || rect.top > view.height - rect.bottom ? 'above' : 'below';
    }
    return {side, left, top: side === 'above' ? above : below, arrowX};
}

// useFaceTips shows the mockup's .tip above (or, with no room there, below) a stacked face or a data-am-tip element under
// the pointer, in the layer; and by a data-am-tip element the keyboard moves to, as browsers show no title tooltip
// there. Escape hides it.
export function useFaceTips() {
    useEffect(() => {
        let tip: HTMLDivElement | null = null;

        // Whether the tip shown is the focused element's: leaving it hides it, not the pointer's.
        let fromFocus = false;
        const hide = () => {
            tip?.remove();
            tip = null;
        };
        const show = (av: HTMLElement | null | undefined) => {
            const name = av?.dataset.amTip || av?.dataset.name;
            if (!av || !name) {
                hide();
                return;
            }
            if (tip?.dataset.for === name) {
                return;
            }
            hide();
            const layer = document.getElementById(LAYER_ID);
            if (!layer) {
                return;
            }
            tip = document.createElement('div');
            const right = av.dataset.amTipSide === 'right';
            tip.className = (av.dataset.amTip ? 'am-tip am-tip-text' : 'am-tip') + (right ? ' am-tip-right' : '');
            tip.setAttribute('role', 'tooltip');
            tip.dataset.for = name;
            tip.textContent = name;

            // Laid out at the corner before it's measured: wherever it lands first (the layer sits at the window's
            // right edge), its text would wrap narrower than it ends up, and the centring be off by the difference.
            tip.style.left = '0px';
            tip.style.top = '0px';
            layer.appendChild(tip);
            const place = placeTip(
                av.getBoundingClientRect(),
                {width: tip.offsetWidth, height: tip.offsetHeight},
                {width: window.innerWidth, height: window.innerHeight},
                av.dataset.amTipSide,
            );
            tip.style.left = place.left + 'px';
            tip.style.top = place.top + 'px';
            tip.dataset.side = place.side;
            if (place.side === 'below') {
                tip.classList.add('am-tip-below');
            }
            if (place.arrowX !== undefined) {
                tip.style.setProperty('--am-tip-arrow-x', place.arrowX + 'px');
            }
        };
        const onOver = (e: MouseEvent) => {
            fromFocus = false;
            show((e.target as Element).closest?.<HTMLElement>(SCOPE));
        };
        const onFocus = (e: FocusEvent) => {
            const el = e.target as HTMLElement;
            if (el?.dataset?.amTip && isFocusVisible(el)) {
                show(el);
                fromFocus = true;
            }
        };
        const onBlur = () => {
            if (fromFocus) {
                hide();
                fromFocus = false;
            }
        };
        const onKeyDown = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                hide();
            }
        };
        document.addEventListener('mouseover', onOver);
        document.addEventListener('focusin', onFocus);
        document.addEventListener('focusout', onBlur);
        document.addEventListener('keydown', onKeyDown);
        document.addEventListener('scroll', hide, true);
        document.addEventListener('pointerdown', hide, true);
        return () => {
            hide();
            document.removeEventListener('mouseover', onOver);
            document.removeEventListener('focusin', onFocus);
            document.removeEventListener('focusout', onBlur);
            document.removeEventListener('keydown', onKeyDown);
            document.removeEventListener('scroll', hide, true);
            document.removeEventListener('pointerdown', hide, true);
        };
    }, []);
}

// Focus from the keyboard, not from a click (the pointer already showed the tip, and a click hides it).
function isFocusVisible(el: Element): boolean {
    try {
        return el.matches(':focus-visible');
    } catch {
        return true;
    }
}
