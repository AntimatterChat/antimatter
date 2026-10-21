// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {DateTime} from 'luxon';
import {useEffect, useState, useSyncExternalStore} from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import type {UserProfile} from '@mattermost/types/users';

import {getCurrentTimezone} from 'mattermost-redux/selectors/entities/timezone';
import {getCurrentUser, getStatusForUserId, getUser} from 'mattermost-redux/selectors/entities/users';

import type {GlobalState} from 'types/store';

// Focus mode (Antimatter) is Do not disturb until a time, with a reason as the custom status (🎯). While someone
// focuses, the server keeps the end of their focus (milliseconds) in their focus_until user prop, which every client
// has with the user: their status dot turns purple, and, for themself, their unread badges hide until the end, and
// then until they've seen the summary of what they missed.
export const FOCUS_UNTIL_PROP = 'focus_until';

// The summary seen last, across your devices: the start of its focus.
export const FOCUS_PREF_CATEGORY = 'focus';
export const FOCUS_PREF_SEEN = 'summary_seen';

// The server ends a focus with its Do not disturb expiry, within a minute of its end: your own badges stay hidden
// until it did (and the summary came), rather than coming back for a moment first.
const OWN_GRACE = 2 * 60 * 1000;

// The longest a timer may wait (setTimeout's limit).
const MAX_TIMEOUT = (2 ** 31) - 1;

// focusUntil is the end of someone's focus from their user props, 0 when they don't focus.
export function focusUntil(user?: Pick<UserProfile, 'props'>): number {
    const value = Number(user?.props?.[FOCUS_UNTIL_PROP] || 0);
    return Number.isFinite(value) && value > 0 ? value : 0;
}

// useBefore tells whether the time given is still to come, rendering again when it comes.
function useBefore(at: number): boolean {
    const [, tick] = useState(0);
    useEffect(() => {
        const left = at - Date.now();
        if (!at || left <= 0) {
            return undefined;
        }
        const timer = setTimeout(() => tick((n) => n + 1), Math.min(left + 50, MAX_TIMEOUT));
        return () => clearTimeout(timer);
    }, [at]);
    return at > Date.now();
}

// useFocusUntil is when someone's focus ends, while they focus (in Do not disturb, with the marker), else 0.
export function useFocusUntil(userId?: string): number {
    const until = useSelector((state: GlobalState) => {
        if (!userId) {
            return 0;
        }
        const end = focusUntil(getUser(state, userId));
        return end && getStatusForUserId(state, userId) === 'dnd' ? end : 0;
    });
    return useBefore(until) ? until : 0;
}

// useMyFocusUntil is when your own focus ends, while you focus, else 0.
export function useMyFocusUntil(): number {
    const until = useSelector((state: GlobalState) => focusUntil(getCurrentUser(state)));
    return useBefore(until ? until + OWN_GRACE : 0) ? until : 0;
}

// useFocusTime is the end of a focus as a time, with the day when it isn't today.
export function useFocusTime(until: number): string {
    const intl = useIntl();

    // Your timezone, as the composer's schedule (fusion/composer/schedule) has it.
    const zone = useSelector(getCurrentTimezone) || DateTime.local().zoneName || 'UTC';
    if (!until) {
        return '';
    }
    const today = DateTime.fromMillis(until, {zone}).hasSame(DateTime.now().setZone(zone), 'day');
    return intl.formatDate(until, today ? {hour: 'numeric', minute: '2-digit', timeZone: zone} : {weekday: 'short', hour: 'numeric', minute: '2-digit', timeZone: zone});
}

// useFocusLabel names someone's focus, for their status dot's tooltip and their status line: "Focusing · until
// 15:30"; empty when they don't focus.
export function useFocusLabel(until: number): string {
    const {formatMessage} = useIntl();
    const time = useFocusTime(until);
    if (!until) {
        return '';
    }
    return formatMessage({id: 'fusion.focus.label', defaultMessage: 'Focusing · until {time}', description: 'Someone\'s status while in focus mode (Do not disturb with a reason, for a while): the tooltip of their purple status dot and their status line; {time} is when it ends.'}, {time});
}

// The focus whose summary waits to be seen in this window (its start and end), shared by the summary's host and
// everything that hides the unread badges until then.
export type FocusWindow = {start_at: number; end_at: number};

let pending: FocusWindow | null = null;
const listeners = new Set<() => void>();

export function setFocusSummary(next: FocusWindow | null) {
    if (next?.start_at === pending?.start_at && next?.end_at === pending?.end_at) {
        return;
    }
    pending = next;
    listeners.forEach((listener) => listener());
}

function subscribe(listener: () => void) {
    listeners.add(listener);
    return () => {
        listeners.delete(listener);
    };
}

export function useFocusSummaryWindow(): FocusWindow | null {
    return useSyncExternalStore(subscribe, () => pending);
}

type UnreadCount = {showUnread: boolean; mentions: number; messages: number; hasUrgent?: boolean};

// useShownUnread is a channel's unread count as the sidebar shows it: nothing while your unread badges hide.
export function useShownUnread<T extends UnreadCount>(unread: T): T {
    const hide = useHideUnreads();
    return hide ? {...unread, showUnread: false, mentions: 0, messages: 0, hasUrgent: false} as T : unread;
}

// useHideUnreads tells whether your unread badges and mention counts hide: while you focus, and after, until you
// leave the summary of what you missed. Unread state is still kept; only its marks hide.
export function useHideUnreads(): boolean {
    const focusing = useMyFocusUntil() > 0;
    return useFocusSummaryWindow() !== null || focusing;
}
