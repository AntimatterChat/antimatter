// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Channel} from '@mattermost/types/channels';

import {getParticipants, getStage} from './calls_api';
import {callsOnTop} from './calls_on_top';
import type {AntimatterCallsAPI, CallsModMute, CallsParticipant} from './plugin_api/calls_public_api';
import type {AntimatterVoiceChannelsAPI, VoiceParticipant} from './plugin_api/voice_channels_public_api';

const ch = (id: string) => ({id, type: 'D'}) as Channel;
const ids = (channels: Channel[]) => channels.map((c) => c.id);

describe('callsOnTop', () => {
    test('conversations with a call go first, in their order, the others as they were', () => {
        expect(ids(callsOnTop([ch('a'), ch('b'), ch('c'), ch('d')], [ch('d'), ch('b')]))).toEqual(['b', 'd', 'a', 'c']);
    });

    test('a conversation with a call that isn\'t listed comes back on top', () => {
        expect(ids(callsOnTop([ch('a'), ch('b')], [ch('x'), ch('b')]))).toEqual(['b', 'x', 'a']);
    });

    test('without calls, the list is kept as is', () => {
        const list = [ch('a'), ch('b')];
        expect(callsOnTop(list, [])).toBe(list);
    });
});

describe('getParticipants', () => {
    const state = {} as never;
    afterEach(() => {
        delete window.antimatterCalls;
        delete window.antimatterVoiceChannels;
    });

    const callsAPI = (participants: CallsParticipant[], mutes: CallsModMute[], deafens: CallsModMute[] = []) => {
        window.antimatterCalls = {
            version: 1,
            selectors: {getParticipants: () => participants, getModMutes: () => mutes, getModDeafens: () => deafens},
        } as unknown as AntimatterCallsAPI;
    };

    test('tells who a moderator muted, and stays the same while nothing changes', () => {
        const list = [{sessionId: 's1', userId: 'u1'}, {sessionId: 's2', userId: 'u2'}] as CallsParticipant[];
        const mutes = [{userId: 'u2', byUserId: 'mod'}];
        callsAPI(list, mutes);

        const result = getParticipants(state, 'channel');
        expect(result.map((p) => [p.userId, p.modMuted, p.deafened])).toEqual([['u1', false, false], ['u2', true, false]]);
        expect(getParticipants(state, 'channel')).toBe(result);
    });

    test('in voice channels too, which the voice channels plugin lists', () => {
        const voice = [{sessionId: 's1', userId: 'u1', deafened: true}] as VoiceParticipant[];
        callsAPI([], [{userId: 'u1', byUserId: 'mod'}]);
        window.antimatterVoiceChannels = {version: 1, selectors: {isVoiceChannel: () => true, getParticipants: () => voice}} as unknown as AntimatterVoiceChannelsAPI;

        expect(getParticipants(state, 'room')).toEqual([{sessionId: 's1', userId: 'u1', deafened: true, modMuted: true, modDeafened: false, audience: false}]);
    });

    test('tells who a moderator deafened: they show deafened', () => {
        const voice = [{sessionId: 's1', userId: 'u1', deafened: false}, {sessionId: 's2', userId: 'u2', deafened: false}] as VoiceParticipant[];
        callsAPI([], [], [{userId: 'u2', byUserId: 'mod'}]);
        window.antimatterVoiceChannels = {version: 1, selectors: {isVoiceChannel: () => true, getParticipants: () => voice}} as unknown as AntimatterVoiceChannelsAPI;

        const result = getParticipants(state, 'room');
        expect(result.map((p) => [p.userId, p.deafened, p.modDeafened, p.modMuted])).toEqual([['u1', false, false, false], ['u2', true, true, false]]);
        expect(getParticipants(state, 'room')).toBe(result);
    });

    test('in a stage\'s call, those who aren\'t its speakers are its audience', () => {
        const voice = [{sessionId: 's1', userId: 'u1'}, {sessionId: 's2', userId: 'u2'}] as VoiceParticipant[];
        const stage = {speakers: ['u1']};
        window.antimatterCalls = {
            version: 1,
            selectors: {getParticipants: () => [], getModMutes: () => [], getModDeafens: () => [], getStage: () => stage},
        } as unknown as AntimatterCallsAPI;
        window.antimatterVoiceChannels = {version: 1, selectors: {isVoiceChannel: () => true, getParticipants: () => voice}} as unknown as AntimatterVoiceChannelsAPI;

        const result = getParticipants(state, 'room');
        expect(result.map((p) => [p.userId, p.audience])).toEqual([['u1', false], ['u2', true]]);
        expect(getParticipants(state, 'room')).toBe(result);
        expect(getStage(state, 'room')).toBe(stage);
    });

    test('an older Calls has no stages: everyone in its calls may speak', () => {
        const voice = [{sessionId: 's1', userId: 'u1'}] as VoiceParticipant[];
        callsAPI([], []);
        window.antimatterVoiceChannels = {version: 1, selectors: {isVoiceChannel: () => true, getParticipants: () => voice}} as unknown as AntimatterVoiceChannelsAPI;

        expect(getStage(state, 'room')).toBeNull();
        expect(getParticipants(state, 'room')[0].audience).toBe(false);
    });

    test('older Calls versions have no moderator mutes', () => {
        const list = [{sessionId: 's1', userId: 'u1'}] as CallsParticipant[];
        window.antimatterCalls = {version: 1, selectors: {getParticipants: () => list}} as unknown as AntimatterCallsAPI;

        expect(getParticipants(state, 'channel')[0].modMuted).toBe(false);
        expect(getParticipants(state, 'channel')[0].modDeafened).toBe(false);
    });
});
