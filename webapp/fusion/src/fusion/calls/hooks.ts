// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useEffect, useMemo, useState} from 'react';
import {shallowEqual, useSelector} from 'react-redux';

import type {GlobalState} from 'types/store';

import {
    getCall,
    getCallChannelIds,
    getCallsAPI,
    getLocalCall,
    getParticipants,
    getVoiceAPI,
    isVoiceChannel,
    participantsEqual,
    useCallsAPIs,
} from './calls_api';
import type {CallsCall, LocalCall, VoiceParticipant} from './calls_api';

// useCallsAvailable tells whether the Calls plugin drives calls here.
export function useCallsAvailable(): boolean {
    return Boolean(useCallsAPIs().calls);
}

export function useIsVoiceChannel(channelId?: string): boolean {
    useCallsAPIs();
    return useSelector((state: GlobalState) => (channelId ? isVoiceChannel(state, channelId) : false));
}

// useCall is the call in progress in a channel, if any.
export function useCall(channelId?: string): CallsCall | null {
    useCallsAPIs();
    return useSelector((state: GlobalState) => (channelId ? getCall(state, channelId) : null), shallowEqual);
}

// useParticipants lists the people in a channel's call (or voice channel), with deafen state in voice channels.
export function useParticipants(channelId?: string): VoiceParticipant[] {
    useCallsAPIs();
    return useSelector((state: GlobalState) => getParticipants(state, channelId || ''), participantsEqual);
}

// useLocalCall is the call this client is in, from any window of it.
export function useLocalCall(): LocalCall | null {
    useCallsAPIs();
    return useSelector(getLocalCall, shallowEqual);
}

export type MyCallState = {
    call: LocalCall;
    me?: VoiceParticipant;

    // The connection is to a voice channel (deafen exists) rather than an ad-hoc call.
    voice: boolean;
    muted: boolean;
    deafened: boolean;
    video: boolean;
    screen: boolean;
    handRaised: boolean;
    isHost: boolean;

    // Someone else shares their screen: there's one screen share per call.
    otherSharing: boolean;
    videoAllowed: boolean;
    screenAllowed: boolean;
};

// useMyCall is the local user's call and their state in it, or null when not in a call.
export function useMyCall(): MyCallState | null {
    const call = useLocalCall();
    const channelId = call?.channelId || '';
    const participants = useParticipants(channelId);
    const voice = useIsVoiceChannel(channelId);
    const flags = useSelector((state: GlobalState) => {
        const calls = getCallsAPI();
        if (!calls || !channelId) {
            return null;
        }
        return {
            muted: calls.selectors.getMyMuted(state),
            deafened: voice ? Boolean(getVoiceAPI()?.selectors.isDeafened(state)) : false,
            sharingSession: calls.selectors.getScreenSharingSessionId(state, channelId),
            videoAllowed: calls.selectors.isVideoAllowed(state, channelId),
            screenAllowed: calls.selectors.isScreenSharingAllowed(state),
        };
    }, shallowEqual);

    return useMemo(() => {
        if (!call || !flags) {
            return null;
        }
        const me = participants.find((p) => p.isMe) || participants.find((p) => p.sessionId === call.sessionId);
        const screen = Boolean(flags.sharingSession) && flags.sharingSession === call.sessionId;
        return {
            call,
            me,
            voice,
            muted: flags.muted,
            deafened: flags.deafened,
            video: Boolean(me?.video),
            screen,
            handRaised: Boolean(me?.raisedHand),
            isHost: Boolean(me?.isHost),
            otherSharing: Boolean(flags.sharingSession) && !screen,
            videoAllowed: flags.videoAllowed,
            screenAllowed: flags.screenAllowed,
        };
    }, [call, flags, participants, voice]);
}

export type CallMedia = {
    localVideo: MediaStream | null;
    remoteVideos: Record<string, MediaStream>;
    screen: MediaStream | null;
};

const NO_MEDIA: CallMedia = {localVideo: null, remoteVideos: {}, screen: null};

// useCallMedia gives the camera and screen streams of a channel's call, when this window runs it; it re-renders
// whenever Calls reports a change of streams.
export function useCallMedia(channelId?: string): CallMedia {
    const {calls} = useCallsAPIs();
    const call = useLocalCall();
    const active = Boolean(calls && call && call.mediaInThisWindow && call.channelId === channelId);
    const [version, setVersion] = useState(0);

    useEffect(() => {
        if (!calls || !active) {
            return undefined;
        }
        return calls.on('change', () => setVersion((v) => v + 1));
    }, [calls, active]);

    return useMemo(() => {
        if (!calls || !active) {
            return NO_MEDIA;
        }
        return {
            localVideo: calls.getLocalVideoStream(),
            remoteVideos: calls.getRemoteVideoStreams() || {},
            screen: calls.getLocalScreenStream() || calls.getRemoteScreenStream(),
        };

        // version: the getters' results change without the arguments changing.
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [calls, active, version]);
}

export type Talk = {kind: 'voice' | 'call'; channelId: string};

// The talking map is computed as a string (compared by value, so that nothing re-renders while it is unchanged),
// then parsed once per change. It's shared by every row that shows it, so it's computed once per state.
let lastState: GlobalState | undefined;
let lastKey = '';
function talkingKey(state: GlobalState): string {
    if (state !== lastState) {
        lastState = state;
        lastKey = computeTalkingKey(state);
    }
    return lastKey;
}

function computeTalkingKey(state: GlobalState): string {
    const channelIds = getCallChannelIds(state);
    if (!channelIds.length) {
        return '';
    }
    const voice: string[] = [];
    const adhoc: string[] = [];
    for (const channelId of channelIds) {
        const kind = isVoiceChannel(state, channelId) ? 'voice' : 'call';
        for (const p of getParticipants(state, channelId)) {
            (kind === 'voice' ? voice : adhoc).push(`${p.userId} ${kind} ${channelId}`);
        }
    }

    // Voice channels first, as the mockup's userTalk.
    return voice.concat(adhoc).join('\n');
}

// useTalkingMap tells, for everyone in a call or voice channel that this client knows of, where they are talking.
export function useTalkingMap(): Map<string, Talk> {
    useCallsAPIs();
    const key = useSelector(talkingKey);
    return useMemo(() => {
        const map = new Map<string, Talk>();
        for (const line of key ? key.split('\n') : []) {
            const [userId, kind, channelId] = line.split(' ');
            if (!map.has(userId)) {
                map.set(userId, {kind: kind as Talk['kind'], channelId});
            }
        }
        return map;
    }, [key]);
}

// useUserTalk tells where someone is talking right now, as the mockup's userTalk.
export function useUserTalk(userId?: string): Talk | null {
    useCallsAPIs();
    const line = useSelector((state: GlobalState) => {
        if (!userId) {
            return '';
        }
        return talkingKey(state).split('\n').find((l) => l.startsWith(userId + ' ')) || '';
    });
    return useMemo(() => {
        if (!line) {
            return null;
        }
        const [, kind, channelId] = line.split(' ');
        return {kind: kind as Talk['kind'], channelId};
    }, [line]);
}
