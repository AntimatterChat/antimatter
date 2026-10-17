// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useEffect} from 'react';

import {LAYER_ID} from 'fusion/components/layer';

// The stacked faces that name their person on hover (thread summaries, acknowledgements), and the elements that give
// their tooltip in a data-am-tip attribute (reactions, the left rail), above them or, with data-am-tip-side="right",
// on their right, centred.
const SCOPE = '.am-faces .am-av[data-name], [data-am-tip]';

// useFaceTips shows the mockup's .tip above a stacked face or a data-am-tip element under the pointer, in the layer; and
// above a data-am-tip element the keyboard moves to, as browsers show no title tooltip there.
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
            const r = av.getBoundingClientRect();
            if (right) {
                tip.style.left = (r.right + 10) + 'px';
                tip.style.top = ((r.top + (r.height / 2)) - (tip.offsetHeight / 2)) + 'px';
                return;
            }
            tip.style.left = Math.max(8, Math.min((r.left + (r.width / 2)) - (tip.offsetWidth / 2), window.innerWidth - tip.offsetWidth - 8)) + 'px';
            tip.style.top = (r.top - tip.offsetHeight - 8) + 'px';
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
        document.addEventListener('mouseover', onOver);
        document.addEventListener('focusin', onFocus);
        document.addEventListener('focusout', onBlur);
        document.addEventListener('scroll', hide, true);
        document.addEventListener('pointerdown', hide, true);
        return () => {
            hide();
            document.removeEventListener('mouseover', onOver);
            document.removeEventListener('focusin', onFocus);
            document.removeEventListener('focusout', onBlur);
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
