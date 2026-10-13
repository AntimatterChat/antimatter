// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useEffect} from 'react';

import {isTouch} from 'fusion/hooks/media';

// How long a finger rests before it's a long press, and how far it may wander meanwhile.
const HOLD_MS = 550;
const SLOP = 10;

// Where a long press is left to the browser: selecting and editing text.
const EDITABLE = 'input, textarea, select, [contenteditable="true"]';

// The "More actions" button of a message's toolbar (hidden on touch screens): it opens the message's menu.
function messageMenuButton(msg: Element): HTMLButtonElement | null {
    const tools = Array.from(msg.querySelectorAll('.am-msg-tools')).find((el) => el.closest('.am-msg') === msg);
    return tools?.querySelector<HTMLButtonElement>(':scope > button[aria-haspopup="menu"]') ?? null;
}

// useLongPress gives touch screens what right clicks and hovers give a mouse: a long press is a context menu. Android
// fires one itself; iOS doesn't, so a press held still gets a synthetic one. A long press on a message opens its menu
// (its hover toolbar isn't there on touch screens), unless what's under the finger has a menu of its own: a name, a
// file, a channel in the sidebar.
export function useLongPress() {
    useEffect(() => {
        let timer = 0;
        let press: {x: number; y: number; target: Element} | null = null;
        let browserMenu = false;
        let handledAt = 0;

        const cancel = () => {
            window.clearTimeout(timer);
            press = null;
        };

        const onTouchStart = (e: TouchEvent) => {
            cancel();
            handledAt = 0;
            if (e.touches.length !== 1 || !isTouch() || (e.target as Element).closest?.(EDITABLE)) {
                return;
            }
            const t = e.touches[0];
            press = {x: t.clientX, y: t.clientY, target: e.target as Element};
            browserMenu = false;
            timer = window.setTimeout(() => {
                if (!press || browserMenu) {
                    return;
                }
                const {x, y, target} = press;
                press = null;
                target.dispatchEvent(new MouseEvent('contextmenu', {bubbles: true, cancelable: true, clientX: x, clientY: y, view: window}));
            }, HOLD_MS);
        };

        const onTouchMove = (e: TouchEvent) => {
            const t = e.touches[0];
            if (press && t && Math.hypot(t.clientX - press.x, t.clientY - press.y) > SLOP) {
                cancel();
            }
        };

        const onBrowserMenu = (e: MouseEvent) => {
            if (e.isTrusted) {
                browserMenu = true;
            }
        };

        // After the menus of what's under the finger had their chance (React's handlers run on the root, before
        // this one on the document).
        const onContextMenu = (e: MouseEvent) => {
            if (!isTouch()) {
                return;
            }
            const target = e.target as Element;
            if (!e.defaultPrevented && !target.closest?.(EDITABLE)) {
                const msg = target.closest?.('.am-msg');
                const more = msg && messageMenuButton(msg);
                if (more) {
                    e.preventDefault();
                    more.click();
                }
            }
            if (e.defaultPrevented) {
                handledAt = Date.now();
                navigator.vibrate?.(8);
            }
        };

        // The finger lifting after a long press that opened a menu isn't also a tap on what's under it (the next
        // touch is a new tap).
        const onClick = (e: MouseEvent) => {
            if (handledAt && Date.now() - handledAt < 1500) {
                handledAt = 0;
                e.preventDefault();
                e.stopPropagation();
            }
        };

        document.addEventListener('touchstart', onTouchStart, {capture: true, passive: true});
        document.addEventListener('touchmove', onTouchMove, {capture: true, passive: true});
        document.addEventListener('touchend', cancel, {capture: true, passive: true});
        document.addEventListener('touchcancel', cancel, {capture: true, passive: true});
        document.addEventListener('contextmenu', onBrowserMenu, true);
        document.addEventListener('contextmenu', onContextMenu);
        document.addEventListener('click', onClick, true);
        return () => {
            cancel();
            document.removeEventListener('touchstart', onTouchStart, true);
            document.removeEventListener('touchmove', onTouchMove, true);
            document.removeEventListener('touchend', cancel, true);
            document.removeEventListener('touchcancel', cancel, true);
            document.removeEventListener('contextmenu', onBrowserMenu, true);
            document.removeEventListener('contextmenu', onContextMenu);
            document.removeEventListener('click', onClick, true);
        };
    }, []);
}
