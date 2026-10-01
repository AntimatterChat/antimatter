// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Copied from the Calls plugin (antimatter-plugin-calls, webapp/src/types/public_api.d.ts): keep them in sync.

// window.antimatterCalls: the API through which other UIs (the Fusion web UI, other plugins) show
// and drive calls. This file only depends on the DOM types, so it can be copied as is.
//
// The API is installed when the Calls plugin initializes and removed when it stops. As plugins
// may load after the UI, wait for the 'antimatter-calls:ready' window event when it's missing.

// GlobalState is the webapp's Redux state; the selectors take it as is.
type CallsGlobalState = object;

export type CallsParticipant = {
    sessionId: string;
    userId: string;
    muted: boolean;
    video: boolean;
    screenSharing: boolean;

    // Needs the session state of the call to be sent to the whole channel (the
    // broadcast_session_state channel prop) for people outside the call.
    speaking: boolean;
    raisedHand: boolean;
    isHost: boolean;

    // The session of this window's call (or, with the desktop app, of its call window).
    isMe: boolean;
};

export type CallsCall = {
    channelId: string;
    id: string;
    startAt: number;
    ownerId: string;
    hostId: string;

    // Empty when the call has no thread (channels with the disable_call_post prop).
    threadId: string;
    title?: string;
    recording: boolean;
};

export type LocalCall = {
    channelId: string;

    // Empty while connecting.
    sessionId: string;
    state: 'connecting' | 'connected';

    // False when the desktop app runs the call in its own window: this window then has none of
    // the call's media and the controls do nothing.
    mediaInThisWindow: boolean;
};

export type CallsIncomingCall = {
    channelId: string;
    callId: string;
    callerId: string;
    startAt: number;
    type: 'dm' | 'gm';
};

export type CallsJoinOptions = {
    title?: string;

    // When in another call, leave it and join this one instead of asking with Calls' switch call
    // modal.
    switchCall?: boolean;

    // Whether to unmute on connect. Calls in DMs and GMs are unmuted by default, others muted.
    unmuted?: boolean;

    // Turn the camera on once connected (where video is allowed).
    video?: boolean;
};

// Passed to the 'error' listeners. Calls still shows its own modal for errors ending the call.
export type CallsErrorEvent =
    | {kind: 'mic-permissions' | 'mic-missing' | 'camera-permissions' | 'camera-missing'; error: Error} |

    // The permissions were given after all: clear the matching banner.
    {kind: 'mic-ready' | 'camera-ready'} |

    // The chosen device is gone, and this one is used instead.
    {kind: 'device-fallback'; device: MediaDeviceInfo} |

    // No screen was shared: the user cancelled or screen recording isn't allowed.
    {kind: 'screen-share-failed'} |

    // The quality of the call dropped below the threshold, or went back above it.
    {kind: 'degraded-quality' | 'quality-restored'; mos: number} |

    // The call ended because of an error (Calls shows the error modal).
    {kind: 'call-error'; error: Error} |

    // Any other error of the calls client.
    {kind: 'other'; error: Error};

export type CallsEvent = 'change' | 'error';

export type AntimatterCallsAPI = {
    version: 1;

    // Pure selectors on the webapp's Redux store, for useSelector. Their results are memoised and
    // keep the same reference while unchanged.
    selectors: {

        // Whether calls can be started or joined in the channel.
        isCallsEnabled(state: CallsGlobalState, channelId: string): boolean;
        getCall(state: CallsGlobalState, channelId: string): CallsCall | null;

        // In the order they joined.
        getParticipants(state: CallsGlobalState, channelId: string): CallsParticipant[];
        getLocalCall(state: CallsGlobalState): LocalCall | null;

        // '' when nobody shares their screen.
        getScreenSharingSessionId(state: CallsGlobalState, channelId: string): string;

        // Ringing DM and GM calls, newest first.
        getIncomingCalls(state: CallsGlobalState): CallsIncomingCall[];
        isVideoAllowed(state: CallsGlobalState, channelId: string): boolean;
        isScreenSharingAllowed(state: CallsGlobalState): boolean;

        // Whether the user's microphone is muted in their call (true when not in a call).
        getMyMuted(state: CallsGlobalState): boolean;
    };

    // Joins (or starts) the call of a channel, through the same checks as Calls' own call button.
    // Resolves once the join started: follow it with getLocalCall. Rejects when calls are
    // disabled in the channel.
    join(channelId: string, opts?: CallsJoinOptions): Promise<void>;
    leave(): void;

    // Controls of the call run by this window; they do nothing otherwise. Call them right from
    // click handlers: sharing the screen needs the click's user activation.
    setMuted(muted: boolean): void;
    setVideo(on: boolean): Promise<void>;

    // In the desktop app, opens Calls' screen source picker.
    startScreenShare(): void;
    stopScreenShare(): void;
    setHandRaised(raised: boolean): void;

    // Media of the call run by this window, for tiles. The streams keep the same reference as
    // long as their track, and getRemoteVideoStreams() returns the same object while unchanged.
    // Show a camera only while the session's video flag is on: a track can stay live after it's
    // turned off.
    getLocalVideoStream(): MediaStream | null;
    getLocalScreenStream(): MediaStream | null;

    // By session ID.
    getRemoteVideoStreams(): Record<string, MediaStream>;
    getRemoteScreenStream(): MediaStream | null;

    // 'change' fires when the call of this window starts connecting, connects or closes, when
    // its local or remote video or screen streams change, and on mute, hand and device changes:
    // re-read the getters then. 'error' gets a CallsErrorEvent. Returns the unsubscribe function.
    on(event: 'change', cb: () => void): () => void;
    on(event: 'error', cb: (event: CallsErrorEvent) => void): () => void;

    // Host controls on the call of this window (or of the desktop app's call window).
    host: {
        mute(sessionId: string): void;
        remove(sessionId: string): void;
        lowerHand(sessionId: string): void;
        stopScreen(sessionId: string): void;
    };

    dismissIncomingCall(channelId: string, callId: string): void;

    // Opens the Calls tab of the user settings (audio, video and screen sharing devices).
    openSettings(): void;
};

declare global {
    interface Window {
        antimatterCalls?: AntimatterCallsAPI;
    }

    interface WindowEventMap {
        'antimatter-calls:ready': Event;
    }
}
