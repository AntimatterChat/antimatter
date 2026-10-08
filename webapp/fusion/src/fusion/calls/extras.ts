// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useEffect} from 'react';
import {shallowEqual, useSelector} from 'react-redux';

import type {GlobalState} from 'types/store';

import {getCallsAPI} from './calls_api';
import type {VoiceParticipant} from './calls_api';
import type {CallsReaction, CallsScreenQuality} from './plugin_api/calls_public_api';

// These settings are personal to this browser, as a headset's volume is.
function readStored<T>(key: string, fallback: T): T {
    try {
        const value = localStorage.getItem(key);
        return value ? JSON.parse(value) as T : fallback;
    } catch {
        return fallback;
    }
}

function writeStored(key: string, value: unknown) {
    try {
        localStorage.setItem(key, JSON.stringify(value));
    } catch {
        // Private browsing: it lasts until the page closes.
    }
}

// Per-person volume: how loud someone is heard in calls, from 0 to 1, by user, kept across calls.
const VOLUMES_KEY = 'am-call-volumes';

export function getUserVolume(userId: string): number {
    return readStored<Record<string, number>>(VOLUMES_KEY, {})[userId] ?? 1;
}

export function setUserVolume(participant: VoiceParticipant, volume: number) {
    const volumes = readStored<Record<string, number>>(VOLUMES_KEY, {});
    if (volume >= 1) {
        delete volumes[participant.userId];
    } else {
        volumes[participant.userId] = volume;
    }
    writeStored(VOLUMES_KEY, volumes);
    getCallsAPI()?.setParticipantVolume?.(participant.sessionId, volume);
}

// useApplyVolumes gives the people of the call their volumes as they join.
export function useApplyVolumes(participants: VoiceParticipant[]) {
    const sessions = participants.filter((p) => !p.isMe).map((p) => `${p.sessionId}:${p.userId}`).join(',');
    useEffect(() => {
        const api = getCallsAPI();
        if (!api?.setParticipantVolume) {
            return;
        }
        for (const p of participants) {
            if (!p.isMe) {
                api.setParticipantVolume(p.sessionId, getUserVolume(p.userId));
            }
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [sessions]);
}

export function canSetVolume(): boolean {
    return Boolean(getCallsAPI()?.setParticipantVolume);
}

// Screen sharing quality presets, as Vencord's: a size and a frame rate, the last ones used kept.
export const SCREEN_HEIGHTS = [480, 720, 1080, 1440, 0];
export const SCREEN_FRAME_RATES = [15, 30, 60];
const SCREEN_KEY = 'am-screen-quality';
const DEFAULT_QUALITY: CallsScreenQuality = {height: 1080, frameRate: 30};

export function getScreenQuality(): CallsScreenQuality {
    const q = readStored<CallsScreenQuality>(SCREEN_KEY, DEFAULT_QUALITY);
    return {height: SCREEN_HEIGHTS.includes(q.height) ? q.height : DEFAULT_QUALITY.height, frameRate: SCREEN_FRAME_RATES.includes(q.frameRate) ? q.frameRate : DEFAULT_QUALITY.frameRate};
}

export function setScreenQuality(quality: CallsScreenQuality) {
    writeStored(SCREEN_KEY, quality);
}

// Reactions: the quick ones of Calls, with their code points.
export const QUICK_REACTIONS: Array<[string, string]> = [['+1', '1f44d'], ['clap', '1f44f'], ['joy', '1f602'], ['heart', '2764-fe0f'], ['tada', '1f389'], ['open_mouth', '1f62e']];

const NO_REACTIONS: CallsReaction[] = [];

// useRecentReactions gives the reactions of the last seconds in a channel's call, for the stage's overlay.
export function useRecentReactions(channelId: string, seconds = 6): CallsReaction[] {
    return useSelector((state: GlobalState) => {
        const all = getCallsAPI()?.selectors.getReactions?.(state, channelId) || NO_REACTIONS;
        const since = Date.now() - (seconds * 1000);
        const recent = all.filter((r) => r.timestamp >= since);
        return recent.length ? recent : NO_REACTIONS;
    }, shallowEqual);
}

// raisedHandOrder numbers the raised hands of a call, the first raised first (1-based; 0 when the hand isn't up).
export function raisedHandOrder(participants: VoiceParticipant[]): Record<string, number> {
    const raised = participants.filter((p) => p.raisedHand).sort((a, b) => ((a as {raisedHandAt?: number}).raisedHandAt || 0) - ((b as {raisedHandAt?: number}).raisedHandAt || 0));
    const order: Record<string, number> = {};
    raised.forEach((p, i) => {
        order[p.sessionId] = i + 1;
    });
    return order;
}
