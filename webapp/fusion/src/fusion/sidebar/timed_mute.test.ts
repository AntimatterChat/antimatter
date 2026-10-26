// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {act, renderHook} from '@testing-library/react';
import React from 'react';
import {Provider} from 'react-redux';
import configureStore from 'redux-mock-store';

import type {GlobalState} from 'types/store';

import {CATEGORY_MUTE_UNTIL, getMuteEnd, muteFor, unmute, useMuteExpiry} from './timed_mute';

// An unmute that fails comes back with its error, as the real action's result does.
let mockUnmuteFails = false;
jest.mock('actions/channel_actions', () => ({
    muteChannel: (userId: string, channelId: string) => ({type: 'MUTE', userId, channelId}),
    unmuteChannel: (userId: string, channelId: string) => (mockUnmuteFails ? {type: 'UNMUTE_FAILED', userId, channelId, error: new Error('offline')} : {type: 'UNMUTE', userId, channelId}),
}));
jest.mock('mattermost-redux/actions/preferences', () => ({
    savePreferences: (userId: string, prefs: unknown[]) => ({type: 'SAVE_PREFS', prefs}),
    deletePreferences: (userId: string, prefs: unknown[]) => ({type: 'DELETE_PREFS', prefs}),
}));

// A dispatch that records the plain actions it gets.
const recorder = <T>(actions: T[]) => ((a: T) => {
    actions.push(a);
    return a;
}) as unknown as Parameters<ReturnType<typeof unmute>>[0];

const stateWith = (prefs: Record<string, {value: string}> = {}) => ({
    entities: {
        users: {currentUserId: 'me'},
        preferences: {myPreferences: prefs},
    },
} as unknown as GlobalState);

describe('timed mute', () => {
    test('remembers when the mute ends', async () => {
        const actions: Array<{type: string; prefs?: Array<{category: string; name: string; value: string}>}> = [];
        const before = Date.now();
        await muteFor('ch1', 36e5)(recorder(actions), () => stateWith(), undefined);
        expect(actions[0]).toMatchObject({type: 'MUTE', channelId: 'ch1'});
        expect(actions[1].type).toBe('SAVE_PREFS');
        const pref = actions[1].prefs![0];
        expect(pref).toMatchObject({category: CATEGORY_MUTE_UNTIL, name: 'ch1'});
        expect(Number(pref.value)).toBeGreaterThanOrEqual(before + 36e5);
    });

    test('a mute without end, or an unmute, forgets the old end', async () => {
        const state = stateWith({[`${CATEGORY_MUTE_UNTIL}--ch1`]: {value: '123'}});
        const actions: Array<{type: string}> = [];
        await muteFor('ch1', 0)(recorder(actions), () => state, undefined);
        expect(actions.map((a) => a.type)).toEqual(['MUTE', 'DELETE_PREFS']);

        actions.length = 0;
        await unmute('ch1')(recorder(actions), () => state, undefined);
        expect(actions.map((a) => a.type)).toEqual(['UNMUTE', 'DELETE_PREFS']);
    });

    test('reads the end of a mute', () => {
        expect(getMuteEnd(stateWith({[`${CATEGORY_MUTE_UNTIL}--ch1`]: {value: '1700000000000'}}), 'ch1')).toBe(1700000000000);
        expect(getMuteEnd(stateWith(), 'ch1')).toBe(0);
    });
});

// sec-fusion #3: at start the memberships aren't loaded yet. The check used to skip the unmute (no membership, so "not
// muted") but still forget the end, and a one-hour mute stayed on for good.
describe('useMuteExpiry', () => {
    let current: GlobalState;
    const mutedMember = {channel_id: 'c1', user_id: 'me', notify_props: {mark_unread: 'mention'}};

    const make = (myMembers: Record<string, unknown>) => ({
        entities: {
            users: {currentUserId: 'me'},
            preferences: {myPreferences: {[`${CATEGORY_MUTE_UNTIL}--c1`]: {category: CATEGORY_MUTE_UNTIL, name: 'c1', value: String(Date.now() - 864e5)}}},
            channels: {myMembers},
        },
    } as unknown as GlobalState);

    beforeEach(() => {
        jest.useFakeTimers();
        mockUnmuteFails = false;
    });
    afterEach(() => {
        jest.useRealTimers();
    });

    const start = () => {
        const store = configureStore<GlobalState>([])(() => current);
        const view = renderHook(() => useMuteExpiry(), {
            wrapper: ({children}: {children: React.ReactNode}) => React.createElement(Provider, {store}, children),
        });
        return {store, view};
    };
    const flush = async () => {
        for (let i = 0; i < 5; i++) {
            // eslint-disable-next-line no-await-in-loop
            await Promise.resolve();
        }
    };
    const types = (store: {getActions: () => Array<{type: string}>}) => store.getActions().map((a) => a.type);

    test('waits for a membership before unmuting, and forgets the end only after the unmute', async () => {
        current = make({});
        const {store, view} = start();
        expect(types(store)).toEqual([]);

        // The memberships arrive: the next minute's check unmutes, then forgets the end.
        current = make({c1: mutedMember});
        await act(async () => {
            jest.advanceTimersByTime(60000);
            await flush();
        });
        expect(types(store)).toEqual(['UNMUTE', 'DELETE_PREFS']);
        view.unmount();
    });

    test('forgets the end of a channel found unmuted already, without unmuting it', async () => {
        current = make({c1: {...mutedMember, notify_props: {mark_unread: 'all'}}});
        const {store, view} = start();
        await act(flush);
        expect(types(store)).toEqual(['DELETE_PREFS']);
        view.unmount();
    });

    test('keeps the end when the unmute fails, to try again', async () => {
        mockUnmuteFails = true;
        current = make({c1: mutedMember});
        const {store, view} = start();
        await act(flush);
        expect(types(store)).toEqual(['UNMUTE_FAILED']);
        view.unmount();
    });
});
