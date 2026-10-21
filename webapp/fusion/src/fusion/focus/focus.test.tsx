// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import Avatar from 'fusion/components/avatar';
import {useUnreadMentions} from 'fusion/hooks/mentions';
import {act, renderHookWithContext, renderWithContext} from 'tests/react_testing_utils';

import {focusUntil, setFocusSummary, useFocusUntil, useHideUnreads, useShownUnread} from './focus';

// The same results every time, as selectors give.
const mockDMs = [{channel: {id: 'dm'}, mentions: 2, messages: 2}];
const mockTeamUnreads = [new Set(['t1']), new Map([['t1', 1]])];
jest.mock('fusion/selectors', () => ({getUnreadDirectChannels: () => mockDMs}));
jest.mock('mattermost-redux/selectors/entities/channels', () => ({
    ...jest.requireActual('mattermost-redux/selectors/entities/channels'),
    getTeamsUnreadStatuses: () => mockTeamUnreads,
}));

const NOW = new Date('2026-10-21T13:00:00Z').getTime();
const HOUR = 36e5;

// A state with you (me) and maya, their statuses, and focus_until on whoever focuses.
function state({me = 0, maya = 0, mayaStatus = 'dnd'}: {me?: number; maya?: number; mayaStatus?: string} = {}) {
    const props = (until: number) => (until ? {focus_until: String(until)} : {});
    return {
        entities: {
            users: {
                currentUserId: 'me',
                profiles: {
                    me: {id: 'me', username: 'me', props: props(me), notify_props: {}},
                    maya: {id: 'maya', username: 'maya', first_name: 'Maya', props: props(maya)},
                },
                statuses: {me: me ? 'dnd' : 'online', maya: mayaStatus},
            },
            teams: {currentTeamId: 't1', teams: {}, myMembers: {}},
            channels: {myMembers: {}, messageCounts: {}, channels: {}},
            preferences: {myPreferences: {}},
            general: {config: {}},
        },
    } as never;
}

describe('fusion/focus', () => {
    beforeEach(() => {
        jest.useFakeTimers({advanceTimers: true}).setSystemTime(NOW);
        setFocusSummary(null);
    });
    afterEach(() => {
        jest.useRealTimers();
    });

    test('focusUntil reads the marker the server keeps on the user', () => {
        expect(focusUntil({props: {focus_until: '1761054600000'}})).toBe(1761054600000);
        expect(focusUntil({props: {}})).toBe(0);
        expect(focusUntil({props: {focus_until: 'soon'}})).toBe(0);
        expect(focusUntil(undefined)).toBe(0);
    });

    test('someone focuses while in Do not disturb with the marker, until its end', () => {
        const {result} = renderHookWithContext(() => useFocusUntil('maya'), state({maya: NOW + HOUR}));
        expect(result.current).toBe(NOW + HOUR);

        act(() => {
            jest.advanceTimersByTime(HOUR + 1000);
        });
        expect(result.current).toBe(0);
    });

    test('the marker alone, without Do not disturb, is no focus', () => {
        const {result} = renderHookWithContext(() => useFocusUntil('maya'), state({maya: NOW + HOUR, mayaStatus: 'online'}));
        expect(result.current).toBe(0);
    });

    describe('the purple dot', () => {
        beforeEach(() => {
            if (!document.getElementById('am-layer')) {
                const layer = document.createElement('div');
                layer.id = 'am-layer';
                document.body.appendChild(layer);
            }
        });

        test('is purple while someone focuses, with the end in its tooltip', () => {
            const {container} = renderWithContext(
                <Avatar
                    userId='maya'
                    status={true}
                />,
                state({maya: NOW + (2.5 * HOUR)}),
            );
            const dot = container.querySelector('.am-st');
            expect(dot).toHaveClass('am-focus');
            expect(dot).not.toHaveClass('am-dnd');
            expect(dot?.getAttribute('data-am-tip')).toMatch(/^Focusing · until /);
        });

        test('is the status dot otherwise, without a tooltip', () => {
            const {container} = renderWithContext(
                <Avatar
                    userId='maya'
                    status={true}
                />,
                state(),
            );
            const dot = container.querySelector('.am-st');
            expect(dot).toHaveClass('am-dnd');
            expect(dot).not.toHaveClass('am-focus');
            expect(dot).not.toHaveAttribute('data-am-tip');
        });
    });

    describe('your unread badges', () => {
        const unread = {showUnread: true, mentions: 3, messages: 5, hasUrgent: true};

        test('hide while you focus, and the unread state stays', () => {
            const {result} = renderHookWithContext(() => ({hide: useHideUnreads(), shown: useShownUnread(unread), mentions: useUnreadMentions()}), state({me: NOW + HOUR}));
            expect(result.current.hide).toBe(true);
            expect(result.current.shown).toEqual({showUnread: false, mentions: 0, messages: 0, hasUrgent: false});
            expect(result.current.mentions).toBe(0);
            expect(unread.mentions).toBe(3);
        });

        test('stay hidden while the summary waits, and come back after', () => {
            const {result} = renderHookWithContext(() => ({hide: useHideUnreads(), shown: useShownUnread(unread), mentions: useUnreadMentions()}), state());
            expect(result.current.hide).toBe(false);
            expect(result.current.shown).toBe(unread);
            expect(result.current.mentions).toBe(3);

            act(() => setFocusSummary({start_at: NOW - HOUR, end_at: NOW}));
            expect(result.current.hide).toBe(true);
            expect(result.current.mentions).toBe(0);

            act(() => setFocusSummary(null));
            expect(result.current.hide).toBe(false);
            expect(result.current.shown).toBe(unread);
            expect(result.current.mentions).toBe(3);
        });

        test('stay hidden a little after the end, until the server ended the focus', () => {
            const {result} = renderHookWithContext(() => useHideUnreads(), state({me: NOW + 1000}));
            act(() => {
                jest.advanceTimersByTime(30000);
            });
            expect(result.current).toBe(true);
            act(() => {
                jest.advanceTimersByTime(3 * 60000);
            });
            expect(result.current).toBe(false);
        });
    });
});
