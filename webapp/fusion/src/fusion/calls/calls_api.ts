// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// The Fusion UI draws all call UI itself; the Calls plugin (window.antimatterCalls) stays the media and signalling
// backend, and the voice channels plugin (window.antimatterVoiceChannels) turns channels into always-open rooms.
// Both are optional: they may be missing, or appear after the Fusion UI has rendered.

import {useSyncExternalStore} from 'react';

import type {GlobalState} from 'types/store';

import type {
    AntimatterCallsAPI,
    CallsCall,
    CallsErrorEvent,
    CallsIncomingCall,
    CallsModMute,
    CallsMoveRequest,
    CallsParticipant,
    CallsStage,
    CallsStageInvite,
    CallsUnmuteRequest,
    LocalCall,
} from './plugin_api/calls_public_api';
import type {AntimatterVoiceChannelsAPI, VoiceParticipant as VoicePluginParticipant} from './plugin_api/voice_channels_public_api';

export type {AntimatterCallsAPI, AntimatterVoiceChannelsAPI, CallsCall, CallsErrorEvent, CallsIncomingCall, CallsModMute, CallsMoveRequest, CallsParticipant, CallsStage, CallsStageInvite, CallsUnmuteRequest, LocalCall};

// Someone in a call or a voice channel, as the call UI shows them: deafened in voice channels (by themselves or a
// moderator), whether a moderator muted or deafened them (which Calls knows, not the voice channels plugin), and in a
// stage's call, whether they're in its audience (not a speaker; false elsewhere).
export type VoiceParticipant = VoicePluginParticipant & {modMuted: boolean; modDeafened: boolean; audience?: boolean};

// Not in version 1 of the API: the channels that have a call. Calls' state is read when it's missing.
type CallsSelectorsNext = AntimatterCallsAPI['selectors'] & {
    getCallChannelIds?(state: GlobalState): string[];
};

const CALLS_READY = 'antimatter-calls:ready';
const VOICE_READY = 'antimatter-voice-channels:ready';
const CALLS_STATE = 'plugins-com.mattermost.calls';

// Only version 1 is understood; a plugin speaking another version is treated as absent.
export function getCallsAPI(): AntimatterCallsAPI | undefined {
    const api = window.antimatterCalls;
    return api?.version === 1 ? api : undefined;
}

export function getVoiceAPI(): AntimatterVoiceChannelsAPI | undefined {
    const api = window.antimatterVoiceChannels;
    return api?.version === 1 ? api : undefined;
}

type APIs = {calls?: AntimatterCallsAPI; voice?: AntimatterVoiceChannelsAPI};

let snapshot: APIs = {};
function readAPIs(): APIs {
    const calls = getCallsAPI();
    const voice = getVoiceAPI();
    if (calls !== snapshot.calls || voice !== snapshot.voice) {
        snapshot = {calls, voice};
    }
    return snapshot;
}

function subscribe(onChange: () => void) {
    window.addEventListener(CALLS_READY, onChange);
    window.addEventListener(VOICE_READY, onChange);
    return () => {
        window.removeEventListener(CALLS_READY, onChange);
        window.removeEventListener(VOICE_READY, onChange);
    };
}

// useCallsAPIs gives the plugins' APIs, re-rendering when one of them is installed.
export function useCallsAPIs(): APIs {
    return useSyncExternalStore(subscribe, readAPIs);
}

// Selectors that read nothing when a plugin is absent; the arrays are constant so that they compare equal.
const NO_PARTICIPANTS: VoiceParticipant[] = [];
const NO_IDS: string[] = [];
const NO_MOD_MUTES: CallsModMute[] = [];

export function isVoiceChannel(state: GlobalState, channelId: string): boolean {
    const voice = getVoiceAPI();
    return Boolean(voice && channelId && voice.selectors.isVoiceChannel(state, channelId));
}

export function getCall(state: GlobalState, channelId: string): CallsCall | null {
    const calls = getCallsAPI();
    return calls && channelId ? calls.selectors.getCall(state, channelId) : null;
}

export function getLocalCall(state: GlobalState): LocalCall | null {
    return getCallsAPI()?.selectors.getLocalCall(state) || null;
}

// getModMutes lists who a moderator muted in a channel's call (older Calls versions have no moderator mute).
export function getModMutes(state: GlobalState, channelId: string): CallsModMute[] {
    const calls = getCallsAPI();
    return (calls && channelId && calls.selectors.getModMutes?.(state, channelId)) || NO_MOD_MUTES;
}

// getModDeafens lists who a moderator deafened in a voice channel's call (older Calls versions have no moderator
// deafen).
export function getModDeafens(state: GlobalState, channelId: string): CallsModMute[] {
    const calls = getCallsAPI();
    return (calls && channelId && calls.selectors.getModDeafens?.(state, channelId)) || NO_MOD_MUTES;
}

// getStage tells whether the call of a channel is a stage's, and who its speakers are: null for other calls, and with
// a Calls version that has no stages (its calls are all talk).
export function getStage(state: GlobalState, channelId: string): CallsStage | null {
    const calls = getCallsAPI();
    return (calls && channelId && calls.selectors.getStage?.(state, channelId)) || null;
}

const converted = new WeakMap<object, {mutes: CallsModMute[]; deafens: CallsModMute[]; stage: CallsStage | null; result: VoiceParticipant[]}>();

// getParticipants lists the people in a channel's call, with whether they are deafened in voice channels, muted or
// deafened by a moderator, and in a stage's audience.
export function getParticipants(state: GlobalState, channelId: string): VoiceParticipant[] {
    if (!channelId) {
        return NO_PARTICIPANTS;
    }
    const voice = getVoiceAPI();
    const calls = getCallsAPI();
    let list: Array<VoicePluginParticipant | CallsParticipant> | undefined;
    if (voice && voice.selectors.isVoiceChannel(state, channelId)) {
        list = voice.selectors.getParticipants(state, channelId);
    } else if (calls) {
        list = calls.selectors.getParticipants(state, channelId);
    }
    if (!list?.length) {
        return NO_PARTICIPANTS;
    }
    const mutes = getModMutes(state, channelId);
    const deafens = getModDeafens(state, channelId);
    const stage = getStage(state, channelId);

    // The plugins keep their arrays stable while unchanged, so the converted one can be too.
    const cached = converted.get(list);
    if (cached && cached.mutes === mutes && cached.deafens === deafens && cached.stage === stage) {
        return cached.result;
    }
    const muted = new Set(mutes.map((m) => m.userId));
    const deafened = new Set(deafens.map((m) => m.userId));
    const speakers = stage ? new Set(stage.speakers) : null;
    const result = list.map((p) => {
        const modDeafened = deafened.has(p.userId);
        return {...p, deafened: ('deafened' in p && p.deafened) || modDeafened, modMuted: muted.has(p.userId), modDeafened, audience: Boolean(speakers && !speakers.has(p.userId))};
    });
    converted.set(list, {mutes, deafens, stage, result});
    return result;
}

// getCallChannelIds lists the channels with a call that this client knows about.
export function getCallChannelIds(state: GlobalState): string[] {
    const calls = getCallsAPI();
    if (!calls) {
        return NO_IDS;
    }
    const selectors: CallsSelectorsNext = calls.selectors;
    if (selectors.getCallChannelIds) {
        return selectors.getCallChannelIds(state);
    }
    const slice = (state as unknown as Record<string, {calls?: Record<string, unknown>} | undefined>)[CALLS_STATE];
    return slice?.calls ? Object.keys(slice.calls) : NO_IDS;
}

// participantsEqual compares two participant lists by value, for useSelector.
export function participantsEqual(a: VoiceParticipant[], b: VoiceParticipant[]): boolean {
    if (a === b) {
        return true;
    }
    if (a.length !== b.length) {
        return false;
    }
    return a.every((p, i) => {
        const q = b[i];
        return p.sessionId === q.sessionId && p.userId === q.userId && p.muted === q.muted && p.deafened === q.deafened &&
            p.video === q.video && p.screenSharing === q.screenSharing && p.speaking === q.speaking &&
            p.raisedHand === q.raisedHand && p.isHost === q.isHost && p.isMe === q.isMe && p.modMuted === q.modMuted &&
            p.modDeafened === q.modDeafened && Boolean(p.audience) === Boolean(q.audience);
    });
}

// Someone is shown speaking only while they can be heard.
export function isSpeaking(p: VoiceParticipant): boolean {
    return p.speaking && !p.muted && !p.deafened;
}
