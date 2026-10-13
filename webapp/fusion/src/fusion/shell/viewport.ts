// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useEffect} from 'react';

import {TOUCH_QUERY} from 'fusion/hooks/media';

// useVisualViewport keeps the Fusion UI above the on-screen keyboard. Chrome resizes the page for it
// (interactive-widget=resizes-content in root.html), but iOS only shrinks and pans the visual viewport: there the frame
// takes the visual viewport's height (--am-vvh) and follows it when it pans (--am-vv-shift), or the composer and the
// sheets would end up under the keyboard. A zoomed-in page is left alone, and so is anything with a mouse.
export function useVisualViewport() {
    useEffect(() => {
        const vv = window.visualViewport;
        const touch = window.matchMedia?.(TOUCH_QUERY);
        if (!vv || !touch) {
            return undefined;
        }
        const style = document.documentElement.style;
        const clear = () => {
            style.removeProperty('--am-vvh');
            style.removeProperty('--am-vv-shift');
        };
        const update = () => {
            if (!touch.matches || vv.scale > 1.01) {
                clear();
                return;
            }
            style.setProperty('--am-vvh', `${Math.round(vv.height)}px`);
            if (vv.offsetTop > 0) {
                style.setProperty('--am-vv-shift', `translateY(${Math.round(vv.offsetTop)}px)`);
            } else {
                style.removeProperty('--am-vv-shift');
            }
        };
        update();
        vv.addEventListener('resize', update);
        vv.addEventListener('scroll', update);
        touch.addEventListener('change', update);
        return () => {
            vv.removeEventListener('resize', update);
            vv.removeEventListener('scroll', update);
            touch.removeEventListener('change', update);
            clear();
        };
    }, []);
}
