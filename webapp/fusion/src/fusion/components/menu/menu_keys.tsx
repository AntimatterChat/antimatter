// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useRef} from 'react';

import {am} from 'fusion/utils/class_names';

// The items a menu's arrow keys move between: its entries, but not the disabled ones nor those of a flyout the focus
// isn't in (hidden by CSS until then).
export const MENU_ITEMS = '[role="menuitem"], [role="menuitemcheckbox"], [role="menuitemradio"]';

function focusable(container: HTMLElement, selector: string): HTMLElement[] {
    return Array.from(container.querySelectorAll<HTMLElement>(selector)).filter((el) => {
        if ((el as HTMLButtonElement).disabled || el.getAttribute('aria-disabled') === 'true') {
            return false;
        }
        const fly = el.closest('.am-fly-menu');
        return !fly || !container.contains(fly) || Boolean(fly.parentElement?.contains(el.ownerDocument.activeElement));
    });
}

// rovingKeyDown moves the focus between a menu's or a toolbar's items with the arrow keys, Home and End, as the
// WAI-ARIA menu and toolbar patterns: down (or right, across) to the next one, up to the previous one, round at
// the ends. It returns whether it handled the key.
export function rovingKeyDown(e: React.KeyboardEvent<HTMLElement> | KeyboardEvent, container: HTMLElement | null, selector = MENU_ITEMS, orientation: 'vertical' | 'horizontal' = 'vertical'): boolean {
    if (!container || e.altKey || e.ctrlKey || e.metaKey) {
        return false;
    }
    const next = orientation === 'vertical' ? 'ArrowDown' : 'ArrowRight';
    const previous = orientation === 'vertical' ? 'ArrowUp' : 'ArrowLeft';
    if (![next, previous, 'Home', 'End'].includes(e.key)) {
        return false;
    }

    // A field in the menu (a search) keeps its own keys, but for leaving it downwards.
    const target = e.target as HTMLElement;
    if (target.matches?.('input, textarea, select') && e.key !== next) {
        return false;
    }
    const items = focusable(container, selector);
    if (!items.length) {
        return false;
    }
    const at = items.indexOf(target.closest?.(selector) as HTMLElement);
    let to: number;
    if (e.key === 'Home') {
        to = 0;
    } else if (e.key === 'End') {
        to = items.length - 1;
    } else if (e.key === next) {
        to = at < 0 ? 0 : (at + 1) % items.length;
    } else {
        to = at < 0 ? items.length - 1 : ((at + items.length) - 1) % items.length;
    }
    e.preventDefault();
    e.stopPropagation();
    items[to].focus();
    return true;
}

type Props = {
    children: React.ReactNode;

    // Whether the first item takes the focus as the menu opens, as a menu opened from its button does.
    autoFocus?: boolean;
};

// MenuKeys makes the menu it's in keyboard-navigable: its first item takes the focus as it opens, the arrow keys,
// Home and End move between its items (Escape is the popover's), and the focus goes back to where it was (the
// menu's button) when it closes, unless something else took it (the dialog an item opened).
export default function MenuKeys({children, autoFocus = true}: Props) {
    const ref = useRef<HTMLDivElement>(null);
    useEffect(() => {
        const doc = ref.current?.ownerDocument || document;
        const before = doc.activeElement as HTMLElement | null;
        if (autoFocus && ref.current) {
            focusable(ref.current, MENU_ITEMS)[0]?.focus({preventScroll: true});
        }
        return () => {
            const now = doc.activeElement;
            if (before && before !== doc.body && before.isConnected && (!now || now === doc.body)) {
                before.focus({preventScroll: true});
            }
        };
    }, []); // eslint-disable-line react-hooks/exhaustive-deps

    return (
        <div
            ref={ref}
            role='none'
            className={am('menu-keys')}
            onKeyDown={(e) => rovingKeyDown(e, ref.current)}
        >
            {children}
        </div>
    );
}
