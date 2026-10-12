// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useLayoutEffect, useRef, useState} from 'react';
import type {RefObject} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {setRhsExpanded} from 'actions/views/rhs';
import {getIsRhsExpanded} from 'selectors/rhs';

import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';

// An app's panel (Notes, Whiteboard, Playbooks…) is as wide as its default (styles/_09_apps.scss) until its user drags
// its left edge: it then keeps that width while it stays open, switching apps included, and goes back to its default
// when it closes, or with its header's "Restore default width" button. The member list in the same column keeps its
// width.
//
// The stylesheet gives the panel its bounds: min-width is its default width, max-width leaves the conversation its
// room (--am-main-resize-min). Narrow windows, where panels slide over the conversation, have none: no resizing there.

// The width the arrow keys move the panel's edge by.
export const WIDTH_STEP = 16;

export type WidthBounds = {min: number; max: number};

// clampWidth keeps a width within the panel's bounds.
export function clampWidth(width: number, {min, max}: WidthBounds) {
    return Math.round(Math.min(Math.max(width, min), Math.max(min, max)));
}

// fitWidth is the width the panel keeps for a width asked for: null (its default) for its minimum or less.
export function fitWidth(width: number | null, bounds: WidthBounds): number | null {
    if (width === null) {
        return null;
    }
    const fitted = clampWidth(width, bounds);
    return fitted <= bounds.min ? null : fitted;
}

// keyWidth is the width a key gives the panel, the edge being its left one: Left widens, Right narrows, Home restores
// the default and End takes the most room. Undefined for the other keys.
export function keyWidth(key: string, width: number | null, bounds: WidthBounds): number | null | undefined {
    const current = width ?? bounds.min;
    switch (key) {
    case 'ArrowLeft':
        return fitWidth(current + WIDTH_STEP, bounds);
    case 'ArrowRight':
        return fitWidth(current - WIDTH_STEP, bounds);
    case 'Home':
        return null;
    case 'End':
        return fitWidth(bounds.max, bounds);
    default:
        return undefined;
    }
}

// readBounds reads the panel's bounds from its style; null where it can't be resized.
export function readBounds(panel: HTMLElement): WidthBounds | null {
    const style = window.getComputedStyle(panel);
    const min = parseFloat(style.minWidth);
    const max = parseFloat(style.maxWidth);
    if (!Number.isFinite(min) || !Number.isFinite(max) || min <= 0) {
        return null;
    }
    return {min, max: Math.max(min, max)};
}

type Drag = {x: number; width: number; bounds: WidthBounds};

export type PanelWidth = {

    // The width the user gave the panel; null for its default.
    width: number | null;
    bounds: WidthBounds | null;
    dragging: boolean;
    reset: () => void;
    handleProps: Pick<React.HTMLAttributes<HTMLElement>, 'onPointerDown' | 'onPointerMove' | 'onPointerUp' | 'onPointerCancel' | 'onKeyDown' | 'onDoubleClick'>;
};

/**
 * usePanelWidth is the width of the right-hand panel while it shows an app (active): what the user made it by dragging
 * its edge or with the keyboard, forgotten when the app panel closes. kind changes with what sizes the panel by
 * default (the app's), to read its bounds again.
 *
 * Plugins and the "expand the panel" shortcut still see and set the classic expanded state
 * (state.views.rhs.isSidebarExpanded): for an app's panel it means "wider than its default". Expanding it gives the
 * panel all the room it can take; collapsing it restores its default width.
 */
export function usePanelWidth(panelRef: RefObject<HTMLElement | null>, active: boolean, kind?: string): PanelWidth {
    const dispatch = useDispatch();
    const expanded = useSelector(getIsRhsExpanded);
    const [width, setWidth] = useState<number | null>(null);
    const [bounds, setBounds] = useState<WidthBounds | null>(null);
    const [dragging, setDragging] = useState(false);
    const drag = useRef<Drag | null>(null);
    const wasActive = useRef(active);
    const sized = width !== null;

    // Opening an app's panel (or leaving it for a thread, results…) starts from the default width, not expanded.
    useEffect(() => {
        if (wasActive.current === active) {
            return;
        }
        wasActive.current = active;
        setWidth(null);
        drag.current = null;
        setDragging(false);
        if (expanded) {
            dispatch(setRhsExpanded(false));
        }

        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [active]);

    // The bounds follow the window's size; a width out of them is brought back in.
    useLayoutEffect(() => {
        if (!active) {
            setBounds(null);
            return undefined;
        }
        const measure = () => {
            const next = panelRef.current ? readBounds(panelRef.current) : null;
            setBounds(next);
            if (next) {
                setWidth((w) => fitWidth(w, next));
            }
        };
        measure();
        window.addEventListener('resize', measure);
        return () => window.removeEventListener('resize', measure);
    }, [active, kind, panelRef]);

    // The panel's width tells the classic expanded state…
    useEffect(() => {
        if (active && expanded !== sized) {
            dispatch(setRhsExpanded(sized));
        }

        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [sized]);

    // …which a plugin or the shortcut may change.
    useEffect(() => {
        if (!active || expanded === sized) {
            return;
        }
        const current = panelRef.current ? readBounds(panelRef.current) : null;
        setWidth(expanded && current ? fitWidth(current.max, current) : null);

        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [expanded]);

    const endDrag = (e: React.PointerEvent<HTMLElement>) => {
        if (!drag.current) {
            return;
        }
        drag.current = null;
        setDragging(false);
        if (e.currentTarget.hasPointerCapture?.(e.pointerId)) {
            e.currentTarget.releasePointerCapture(e.pointerId);
        }
    };

    return {
        width,
        bounds,
        dragging,
        reset: () => setWidth(null),
        handleProps: {
            onPointerDown: (e) => {
                const panel = panelRef.current;
                const current = panel ? readBounds(panel) : null;
                if (e.button !== 0 || !panel || !current) {
                    return;
                }
                e.preventDefault();
                e.currentTarget.setPointerCapture?.(e.pointerId);
                drag.current = {x: e.clientX, width: panel.getBoundingClientRect().width, bounds: current};
                setDragging(true);
            },
            onPointerMove: (e) => {
                const d = drag.current;
                if (d) {
                    setWidth(fitWidth(d.width + (d.x - e.clientX), d.bounds));
                }
            },
            onPointerUp: endDrag,
            onPointerCancel: endDrag,
            onDoubleClick: () => setWidth(null),
            onKeyDown: (e) => {
                const current = panelRef.current ? readBounds(panelRef.current) : null;
                const next = current ? keyWidth(e.key, width, current) : undefined;
                if (next !== undefined) {
                    e.preventDefault();
                    setWidth(next);
                }
            },
        },
    };
}

// PanelResizeHandle is the panel's left edge, to drag (or move with the arrow keys) to resize it.
export function PanelResizeHandle({panel}: {panel: PanelWidth}) {
    const {formatMessage} = useIntl();
    const {bounds} = panel;
    if (!bounds || bounds.max <= bounds.min) {
        return null;
    }
    const label = formatMessage({id: 'fusion.rhs.resize', defaultMessage: 'Resize the panel'});
    return (
        <div
            className={am('rhs-resize', {dragging: panel.dragging})}
            role='separator'
            aria-orientation='vertical'
            aria-label={label}
            aria-valuemin={bounds.min}
            aria-valuemax={bounds.max}
            aria-valuenow={panel.width ?? bounds.min}

            // A focusable separator is the ARIA window splitter, which the arrow keys move.
            // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex
            tabIndex={0}
            {...panel.handleProps}
        />
    );
}

// RestoreWidthButton, in the panel's header once its user resized it, gives it back its default width.
export function RestoreWidthButton({sized, onRestore}: {sized: boolean; onRestore: () => void}) {
    const {formatMessage} = useIntl();
    if (!sized) {
        return null;
    }
    const label = formatMessage({id: 'fusion.rhs.restoreWidth', defaultMessage: 'Restore default width'});
    return (
        <button
            type='button'
            className={am('icon-btn', 'rhs-restore-width')}
            title={label}
            aria-label={label}
            onClick={onRestore}
        >
            <Icon
                name='shrink-width'
                size='sm'
            />
        </button>
    );
}
