// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useSyncExternalStore} from 'react';

// What the call UI shows on this screen, shared by the sidebar, the channel view and the right-hand panel.
type CallsUIState = {

    // The call's floating window (the mockup's callWin).
    callWindow: boolean;

    // The voice channel whose text chat is open in the right-hand panel.
    voiceChat: string | null;
};

let state: CallsUIState = {callWindow: false, voiceChat: null};
const listeners = new Set<() => void>();

function update(changes: Partial<CallsUIState>) {
    state = {...state, ...changes};
    listeners.forEach((listener) => listener());
}

function subscribe(listener: () => void) {
    listeners.add(listener);
    return () => {
        listeners.delete(listener);
    };
}

export function useCallsUI(): CallsUIState {
    return useSyncExternalStore(subscribe, () => state);
}

export function setCallWindowOpen(open: boolean) {
    if (state.callWindow !== open) {
        update({callWindow: open});
    }
}

export function setVoiceChat(channelId: string | null) {
    if (state.voiceChat !== channelId) {
        update({voiceChat: channelId});
    }
}
