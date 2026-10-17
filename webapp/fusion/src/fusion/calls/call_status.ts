// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useSelector} from 'react-redux';

import {getChannel} from 'mattermost-redux/selectors/entities/channels';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import type {GlobalState} from 'types/store';

import {useCall, useParticipants} from './hooks';
import type {MyCallState} from './hooks';

// What the call dock says of the user's call: connecting, calling (a call in a direct or group message that nobody
// else has joined yet, while it rings), connected to a voice channel, or connected to a call.
export type CallStatus = 'connecting' | 'calling' | 'voice' | 'call';

// The calls of direct and group messages that someone else joined. Once answered, a call stays connected when the
// others leave it (a group message's call goes on; a direct message's ends).
const answered = new Set<string>();

type StatusInput = {
    connecting: boolean;
    voice: boolean;

    // A direct or group message.
    direct: boolean;
    callId: string;

    // Someone other than the user is in the call (from another device of theirs doesn't count).
    othersIn: boolean;
};

export function callStatus({connecting, voice, direct, callId, othersIn}: StatusInput): CallStatus {
    if (connecting) {
        return 'connecting';
    }
    if (voice) {
        return 'voice';
    }
    if (!direct) {
        return 'call';
    }
    if (othersIn && callId) {
        answered.add(callId);
    }
    return othersIn || (Boolean(callId) && answered.has(callId)) ? 'call' : 'calling';
}

// useCallStatus is callStatus for the user's call (null when not in one).
export function useCallStatus(my: MyCallState | null): CallStatus | null {
    const channelId = my?.call.channelId || '';
    const participants = useParticipants(channelId);
    const call = useCall(channelId);
    const me = useSelector(getCurrentUserId);
    const direct = useSelector((state: GlobalState) => {
        const type = channelId ? getChannel(state, channelId)?.type : undefined;
        return type === 'D' || type === 'G';
    });
    if (!my) {
        return null;
    }
    return callStatus({
        connecting: my.call.state === 'connecting',
        voice: my.voice,
        direct,
        callId: call?.id || '',
        othersIn: participants.some((p) => p.userId !== me),
    });
}
