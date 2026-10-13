// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useSyncExternalStore} from 'react';

// What the call UI shows on this screen, shared by the sidebar, the channel view and the right-hand panel.
type CallsUIState = {

    // The call's floating window (the mockup's callWin).
    callWindow: boolean;

    // The voice channel whose text chat is open in the right-hand panel.
    voiceChat: string | null;

    // The browser window the call is shown in, when popped out (see CallPopout).
    callPopout: Window | null;

    // The channel whose call takes the main view instead of its messages, as a voice channel's does (see CallView).
    callView: string | null;
};

let state: CallsUIState = {callWindow: false, voiceChat: null, callPopout: null, callView: null};
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

// openCallPopout shows the call in a window of its own: opened right from the click (browsers block windows opened
// later), or the one already open brought forward. Where windows are blocked, the floating window shows it instead.
export function openCallPopout() {
    if (state.callPopout && !state.callPopout.closed) {
        state.callPopout.focus();
        return;
    }
    const win = window.open('', 'antimatter-call', 'popup,width=960,height=640');
    if (!win) {
        setCallWindowOpen(true);
        return;
    }
    update({callPopout: win, callWindow: false});
}

export function closeCallPopout() {
    if (state.callPopout) {
        if (!state.callPopout.closed) {
            state.callPopout.close();
        }
        update({callPopout: null});
    }
}

export function setVoiceChat(channelId: string | null) {
    if (state.voiceChat !== channelId) {
        update({voiceChat: channelId});
    }
}

export function setCallView(channelId: string | null) {
    if (state.callView !== channelId) {
        update({callView: channelId});
    }
}
