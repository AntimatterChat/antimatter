// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {GlobalState} from 'types/store';

import {CATEGORY_MUTE_UNTIL, getMuteEnd, muteFor, unmute} from './timed_mute';

jest.mock('actions/channel_actions', () => ({
    muteChannel: (userId: string, channelId: string) => ({type: 'MUTE', userId, channelId}),
    unmuteChannel: (userId: string, channelId: string) => ({type: 'UNMUTE', userId, channelId}),
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
