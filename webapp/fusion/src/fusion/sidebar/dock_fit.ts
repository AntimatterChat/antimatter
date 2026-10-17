// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useEffect} from 'react';
import type {RefObject} from 'react';

import {useLayout} from 'fusion/shell/layout_context';

// The channel list keeps the room its channels need, up to this share of the sidebar, and never less than its own
// minimum height (.am-chan-scroll); the Open conversations dock gets the rest.
export const CHANNELS_SHARE = 0.6;
export const CHANNELS_MIN = 60;

// A dock row and the gap under it, until one is there to measure.
export const DEFAULT_ROW = 34;

export type DockMeasures = {

    /** The sidebar's height. */
    sidebar: number;

    /** What else the sidebar holds: its header, the unread filter's bar, the call panel... */
    others: number;

    /** The height the channel list's content needs. */
    channels: number;

    /** The dock without its rows: its title and padding. */
    chrome: number;

    /** A row of the dock with the gap after it. */
    row: number;

    /** The gap between two rows (none after the last). */
    gap: number;
};

// computeDockRows tells how many conversations fit in the dock, with no scrolling: 0 when not even one does.
export function computeDockRows({sidebar, others, channels, chrome, row, gap}: DockMeasures): number {
    const channelRoom = Math.max(CHANNELS_MIN, Math.min(channels, sidebar * CHANNELS_SHARE));
    const space = sidebar - others - channelRoom - chrome;
    if (row <= 0 || space <= 0) {
        return 0;
    }
    return Math.max(0, Math.floor((space + gap) / row));
}

type Refs = {
    sidebar: RefObject<HTMLElement>;
    scroll: RefObject<HTMLElement>;
    dock: RefObject<HTMLElement>;
};

// useDockFit measures how many conversations fit in the Open conversations dock, so that it never scrolls, and keeps
// it in the layout (dockRows) for the dock and the team rail, which shows the unread ones left out. It measures again
// when the sidebar is resized (the window, the drawer on a phone) and when the channel list grows or shrinks (a
// category folded, a channel joined). A hidden sidebar (no height) keeps the last measure.
export function useDockFit({sidebar, scroll, dock}: Refs) {
    const {setDockRows} = useLayout();
    useEffect(() => {
        const aside = sidebar.current;
        const list = scroll.current;
        if (!aside || !list) {
            return undefined;
        }
        const measure = () => {
            const box = dock.current;
            if (!box || !aside.clientHeight) {
                return;
            }
            const rows = box.querySelector<HTMLElement>('.am-dock-list');
            const first = rows?.firstElementChild as HTMLElement | null;
            const gap = rows ? parseFloat(getComputedStyle(rows).rowGap) || 0 : 2;
            setDockRows(computeDockRows({
                sidebar: aside.clientHeight,
                others: aside.clientHeight - list.offsetHeight - box.offsetHeight,
                channels: list.scrollHeight,
                chrome: box.offsetHeight - (rows?.offsetHeight ?? 0),
                row: first?.offsetHeight ? first.offsetHeight + gap : DEFAULT_ROW,
                gap,
            }));
        };
        let frame = 0;
        const schedule = () => {
            cancelAnimationFrame(frame);
            frame = requestAnimationFrame(measure);
        };
        const resize = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(schedule);
        resize?.observe(aside);
        resize?.observe(list);
        const content = typeof MutationObserver === 'undefined' ? null : new MutationObserver(schedule);
        content?.observe(list, {childList: true, subtree: true});
        schedule();
        return () => {
            cancelAnimationFrame(frame);
            resize?.disconnect();
            content?.disconnect();
        };
    }, [sidebar, scroll, dock, setDockRows]);
}
