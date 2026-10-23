// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useRef, useState} from 'react';
import type React from 'react';
import {useDispatch, useSelector} from 'react-redux';

import {savePreferences} from 'mattermost-redux/actions/preferences';
import {get as getPreference, getVisibleDmGmLimit} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentUser} from 'mattermost-redux/selectors/entities/users';

import type {GlobalState} from 'types/store';

// Where the dock's own row count lives: a Fusion-only preference (the dock's size isn't a classic setting), kept
// apart from the classic "Open conversations" one it replaces (sidebar_settings/limit_visible_dms_gms), which Fusion
// no longer shows a control for. Preference names are capped at 32 characters.
export const CATEGORY_DOCK = 'fusion_sidebar';
export const NAME_DOCK_ROWS = 'dock_rows';

export const MIN_DOCK_ROWS = 1;
export const MAX_DOCK_ROWS = 40;

export type DockRowBounds = {min: number; max: number};

export function clampRows(rows: number, {min, max}: DockRowBounds): number {
    return Math.round(Math.min(Math.max(rows, min), Math.max(min, max)));
}

// getDockRowsPreference is the dock's row count: what the user dragged it to, or, before they ever have, the old
// "Open conversations" setting's value (itself defaulting to 10) as the dock's starting size.
export function getDockRowsPreference(state: GlobalState): number {
    const legacy = getVisibleDmGmLimit(state);
    const value = Number(getPreference(state, CATEGORY_DOCK, NAME_DOCK_ROWS, String(legacy)));
    return clampRows(Number.isFinite(value) ? value : legacy, {min: MIN_DOCK_ROWS, max: MAX_DOCK_ROWS});
}

// useDockRows reads the dock's row count and saves it when the user drags its edge (or moves it with the keyboard).
export function useDockRows(): [number, (rows: number) => void] {
    const dispatch = useDispatch();
    const me = useSelector(getCurrentUser);
    const rows = useSelector(getDockRowsPreference);
    const save = (next: number) => {
        if (!me) {
            return;
        }
        const value = clampRows(next, {min: MIN_DOCK_ROWS, max: MAX_DOCK_ROWS});
        dispatch(savePreferences(me.id, [{user_id: me.id, category: CATEGORY_DOCK, name: NAME_DOCK_ROWS, value: String(value)}]));
    };
    return [rows, save];
}

// keyRows is the row count a key gives the dock, its handle being its top edge: Up asks for one more row, Down one
// less, Home its fewest, End the most it can take right now. Undefined for the other keys.
export function keyRows(key: string, rows: number, bounds: DockRowBounds): number | undefined {
    switch (key) {
    case 'ArrowUp':
        return clampRows(rows + 1, bounds);
    case 'ArrowDown':
        return clampRows(rows - 1, bounds);
    case 'Home':
        return bounds.min;
    case 'End':
        return bounds.max;
    default:
        return undefined;
    }
}

type Drag = {y: number; rows: number};

export type DockDrag = {

    // The row count while dragging (snapped to whole rows); the saved one otherwise.
    rows: number;
    dragging: boolean;
    handleProps: Pick<React.HTMLAttributes<HTMLElement>, 'onPointerDown' | 'onPointerMove' | 'onPointerUp' | 'onPointerCancel' | 'onKeyDown'>;
};

// useDockDrag is the dock's top edge, dragged to resize it in steps of one row (snapping as you drag, not after), or
// moved with the arrow keys. rowHeight is a row's height plus the gap after it, in pixels.
export function useDockDrag(rows: number, bounds: DockRowBounds, rowHeight: number, onChange: (rows: number) => void): DockDrag {
    const [dragRows, setDragRows] = useState<number | null>(null);
    const [dragging, setDragging] = useState(false);
    const drag = useRef<Drag | null>(null);
    const current = dragRows ?? rows;

    const end = (e: React.PointerEvent<HTMLElement>) => {
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
        rows: current,
        dragging,
        handleProps: {
            onPointerDown: (e) => {
                if (e.button !== 0) {
                    return;
                }
                e.preventDefault();
                e.currentTarget.setPointerCapture?.(e.pointerId);
                drag.current = {y: e.clientY, rows};
                setDragRows(rows);
                setDragging(true);
            },
            onPointerMove: (e) => {
                const d = drag.current;
                if (!d || rowHeight <= 0) {
                    return;
                }

                // The handle sits on the dock's top edge: dragging it up (the pointer's y going down the screen
                // less, i.e. decreasing) grows the dock by a row.
                setDragRows(clampRows(d.rows + Math.round((d.y - e.clientY) / rowHeight), bounds));
            },
            onPointerUp: (e) => {
                if (drag.current) {
                    onChange(dragRows ?? rows);
                    setDragRows(null);
                }
                end(e);
            },
            onPointerCancel: (e) => {
                setDragRows(null);
                end(e);
            },
            onKeyDown: (e) => {
                const next = keyRows(e.key, rows, bounds);
                if (next !== undefined) {
                    e.preventDefault();
                    onChange(next);
                }
            },
        },
    };
}
