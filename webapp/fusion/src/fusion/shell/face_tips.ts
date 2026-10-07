// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useEffect} from 'react';

import {LAYER_ID} from 'fusion/components/layer';

// The stacked faces that name their person on hover (thread summaries, acknowledgements), and the elements that give
// their tooltip in a data-am-tip attribute (reactions).
const SCOPE = '.am-faces .am-av[data-name], [data-am-tip]';

// useFaceTips shows the mockup's .tip above a stacked face or a data-am-tip element under the pointer, in the layer.
export function useFaceTips() {
    useEffect(() => {
        let tip: HTMLDivElement | null = null;
        const hide = () => {
            tip?.remove();
            tip = null;
        };
        const onOver = (e: MouseEvent) => {
            const av = (e.target as Element).closest?.<HTMLElement>(SCOPE);
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
            tip.className = av.dataset.amTip ? 'am-tip am-tip-text' : 'am-tip';
            tip.setAttribute('role', 'tooltip');
            tip.dataset.for = name;
            tip.textContent = name;
            layer.appendChild(tip);
            const r = av.getBoundingClientRect();
            tip.style.left = Math.max(8, Math.min((r.left + (r.width / 2)) - (tip.offsetWidth / 2), window.innerWidth - tip.offsetWidth - 8)) + 'px';
            tip.style.top = (r.top - tip.offsetHeight - 8) + 'px';
        };
        document.addEventListener('mouseover', onOver);
        document.addEventListener('scroll', hide, true);
        document.addEventListener('pointerdown', hide, true);
        return () => {
            hide();
            document.removeEventListener('mouseover', onOver);
            document.removeEventListener('scroll', hide, true);
            document.removeEventListener('pointerdown', hide, true);
        };
    }, []);
}
