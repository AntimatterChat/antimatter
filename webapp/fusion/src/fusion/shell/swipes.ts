// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useEffect, useRef} from 'react';

import {isPhoneLayout} from './layout_context';
import type {useLayout} from './layout_context';

// How far the closed navigation drawer sits off screen (see .am-app:not(.am-nav-open) in _25_responsive.scss).
const NAV_OFFSET = 340;

// A horizontal drag shorter than this doesn't open or close anything.
const MIN_SWIPE = 50;

// Touches that start on these scroll or edit sideways themselves.
const IGNORE = 'input, textarea, [contenteditable="true"], pre, code, .am-tags, .am-coll-nav, .am-gs-filters, .am-emoji-scroll, .am-fmt-bar';

type Swipe = {x: number; y: number; dx: number; locked: 'x' | 'y' | null};

// useSwipes gives the phone layout the mockup's swipes: right opens the navigation (or closes the right-hand drawer),
// left closes the navigation or opens the right-hand drawer (member list, panels and the app rail). The navigation
// follows the finger while it's dragged.
export function useSwipes(appRef: React.RefObject<HTMLElement | null>, layout: ReturnType<typeof useLayout>) {
    // The listeners are added once; they read the layout's latest state from here.
    const current = useRef(layout);
    current.current = layout;

    useEffect(() => {
        const app = appRef.current;
        if (!app) {
            return undefined;
        }
        let swipe: Swipe | null = null;
        const drawers = () => Array.from(app.querySelectorAll<HTMLElement>('.am-servers, .am-sidebar'));

        const onStart = (e: TouchEvent) => {
            const target = e.target as Element;
            if (!isPhoneLayout() || e.touches.length !== 1 || target.closest?.(IGNORE)) {
                swipe = null;
                return;
            }
            const t = e.touches[0];
            swipe = {x: t.clientX, y: t.clientY, dx: 0, locked: null};
        };

        const onMove = (e: TouchEvent) => {
            if (!swipe) {
                return;
            }
            const t = e.touches[0];
            swipe.dx = t.clientX - swipe.x;
            const dy = t.clientY - swipe.y;
            if (swipe.locked === null && (Math.abs(swipe.dx) > 10 || Math.abs(dy) > 10)) {
                swipe.locked = Math.abs(swipe.dx) > Math.abs(dy) * 1.3 ? 'x' : 'y';
            }
            if (swipe.locked !== 'x' || current.current.rightOpen) {
                return;
            }
            const base = current.current.navOpen ? 0 : -NAV_OFFSET;
            const offset = Math.max(-NAV_OFFSET, Math.min(0, base + swipe.dx));
            for (const el of drawers()) {
                el.classList.add('am-dragging');
                el.style.transform = `translateX(${offset}px)`;
            }
        };

        const onEnd = () => {
            if (!swipe) {
                return;
            }
            const {dx, locked} = swipe;
            swipe = null;
            for (const el of drawers()) {
                el.classList.remove('am-dragging');
                el.style.transform = '';
            }
            if (locked !== 'x' || Math.abs(dx) < MIN_SWIPE) {
                return;
            }
            const l = current.current;
            if (dx > 0) {
                if (l.rightOpen) {
                    l.setRightOpen(false);
                } else if (!l.navOpen) {
                    l.setNavOpen(true);
                }
            } else if (l.navOpen) {
                l.setNavOpen(false);
            } else if (!l.rightOpen) {
                l.openRightDrawer();
            }
        };

        app.addEventListener('touchstart', onStart, {passive: true});
        app.addEventListener('touchmove', onMove, {passive: true});
        app.addEventListener('touchend', onEnd, {passive: true});
        app.addEventListener('touchcancel', onEnd, {passive: true});
        return () => {
            app.removeEventListener('touchstart', onStart);
            app.removeEventListener('touchmove', onMove);
            app.removeEventListener('touchend', onEnd);
            app.removeEventListener('touchcancel', onEnd);
        };
    }, [appRef]);
}
