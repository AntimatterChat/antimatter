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

    // When the hand was raised (0 when it isn't), to list raised hands in order.
    raisedHandAt: number;
    isHost: boolean;

    // The session of this window's call (or, with the desktop app, of its call window).
    isMe: boolean;

    // A moderator muted them for the call: they can't unmute until it's lifted.
    modMuted: boolean;

    // A moderator deafened them (in voice channels): they hear nothing and can't unmute until it's lifted.
    modDeafened: boolean;
};

// Someone muted (or deafened) by a moderator in the call of a channel, and by whom.
export type CallsModMute = {
    userId: string;
    byUserId: string;
};

// A moderator asks the user to unmute in their call.
export type CallsUnmuteRequest = {
    channelId: string;
    sessionId: string;
    byUserId: string;
};

// A moderator's mute or deafen of someone in a channel, as its moderators list them (in voice channels they last
// until lifted, also while the person is away). The by-user IDs are empty, and the times 0, when not muted or not
// deafened. canLift tells whether the user may lift it: not when given by someone ranking higher.
export type CallsLastingMute = {
    userId: string;
    mutedBy: string;
    mutedAt: number;
    deafenedBy: string;
    deafenedAt: number;
    canLiftMute: boolean;
    canLiftDeafen: boolean;
};

// What the user may do to a participant of a call from its menu, as the server works it out for them: mute them
// (also with a moderator mute), deafen and move them (in voice channels), and disconnect them. Nobody may act on
// someone at or above their rank, and those who may not moderate the call may do nothing. The controls still check
// for themselves: menus use it to hide what would be refused.
export type CallsParticipantPermissions = {
    sessionId: string;
    canMute: boolean;
    canDeafen: boolean;
    canMove: boolean;
    canKick: boolean;
};

// A moderator moves the user from the call of a voice channel to another voice channel.
export type CallsMoveRequest = {
    channelId: string;
    sessionId: string;
    toChannelId: string;
    byUserId: string;
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

    // The ring is playing for it.
    ringing: boolean;
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

    // The user can't unmute: a moderator muted them for the call, or deafened them.
    {kind: 'mod-muted' | 'mod-deafened'} |

    // Any other error of the calls client.
    {kind: 'other'; error: Error};

// A reaction sent in a call, shown for a few seconds.
export type CallsReaction = {
    sessionId: string;
    userId: string;

    // The emoji's short name (e.g. "+1") and unicode code points (e.g. "1f44d").
    emojiName: string;
    unified: string;
    timestamp: number;
};

// A screen sharing preset: the largest height sent (0 for the screen's own size) and the frame rate.
export type CallsScreenQuality = {
    height: number;
    frameRate: number;
};

// What to share: a whole screen, a window or a browser tab. The browser's picker opens on that kind of source (and
// leaves out whole screens for a window or a tab).
export type CallsScreenSurface = 'monitor' | 'window' | 'browser';

// In the desktop app, the screen or window to share, picked by the caller from the desktop app's sources
// (window.desktopAPI.getDesktopSources).
export type CallsScreenSource = {
    desktopSourceId: string;

    // Also share the system's audio (see canShareSystemAudio).
    withAudio?: boolean;
};

export type CallsBackgroundBlur = {
    on: boolean;

    // From 1 to 100.
    intensity: number;
};

// The transcription of a call, by a speech-to-text API from the participants' audio.
export type CallsTranscription = {
    running: boolean;

    // When it started (0 when it isn't running).
    startAt: number;

    // Whether its live captions are shown.
    liveCaptions: boolean;
};

// What a participant says, as live captions: shown for a few seconds, until they say more.
export type CallsCaption = {
    sessionId: string;
    userId: string;
    text: string;
    captionId: string;
};

export type CallsEvent = 'change' | 'error' | 'unmute-request' | 'move';

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

        // The recent reactions in the call of a channel, oldest first.
        getReactions(state: CallsGlobalState, channelId: string): CallsReaction[];

        // Whether the host can transcribe calls (by a speech-to-text API, from the participants' audio).
        isTranscriptionAvailable(state: CallsGlobalState): boolean;
        getTranscription(state: CallsGlobalState, channelId: string): CallsTranscription;

        // The captions shown now in the call of a channel, by session.
        getLiveCaptions(state: CallsGlobalState, channelId: string): CallsCaption[];

        // Who a moderator muted in the call of a channel.
        getModMutes(state: CallsGlobalState, channelId: string): CallsModMute[];

        // Who a moderator deafened in the call of a voice channel. Optional: older versions lack it.
        getModDeafens?(state: CallsGlobalState, channelId: string): CallsModMute[];
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

    // A quality preset asks for that size and frame rate, and sends the screen at a matching bitrate; a surface opens
    // the browser's picker on that kind of source. In the desktop app, shares the given source, or opens Calls' own
    // screen source picker without one.
    startScreenShare(quality?: CallsScreenQuality, surface?: CallsScreenSurface, source?: CallsScreenSource): void;

    // Whether a screen shared from the desktop app can bring the system's audio along (Linux and Windows, unless
    // turned off in the Calls settings).
    canShareSystemAudio(): boolean;
    stopScreenShare(): void;
    setHandRaised(raised: boolean): void;

    // Sends a reaction to the call.
    sendReaction(emojiName: string, unified: string): void;

    // How loud a participant's voice is for this user, from 0 to 1 (by session, for this call).
    setParticipantVolume(sessionId: string, volume: number): void;
    getParticipantVolume(sessionId: string): number;

    // The camera's background blur, kept for the next calls.
    getBackgroundBlur(): CallsBackgroundBlur;
    setBackgroundBlur(blur: CallsBackgroundBlur): Promise<void>;

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
    // re-read the getters then. 'error' gets a CallsErrorEvent. 'unmute-request' fires when a
    // moderator asks the user, muted in the call of this window, to unmute. 'move' fires when a
    // moderator moves the user to another voice channel: Calls switches to its call itself, the UI
    // follows. Returns the unsubscribe function.
    on(event: 'change', cb: () => void): () => void;
    on(event: 'error', cb: (event: CallsErrorEvent) => void): () => void;
    on(event: 'unmute-request', cb: (request: CallsUnmuteRequest) => void): () => void;
    on(event: 'move', cb: (request: CallsMoveRequest) => void): () => void;

    // Host controls on the call of this window (or of the desktop app's call window), or on the call of the given
    // channel, for the call's moderators.
    host: {
        mute(sessionId: string, channelId?: string): void;
        remove(sessionId: string, channelId?: string): void;
        lowerHand(sessionId: string, channelId?: string): void;
        stopScreen(sessionId: string, channelId?: string): void;
    };

    // Moderation of a channel's call (its host, channel and community admins, system admins, and in voice channels the
    // community's moderators). They reject with an Error carrying the HTTP status (e.g. 429 when asked again too soon,
    // 403 when not allowed, 400 when it can't be done, as moving someone to a channel they can't join).
    moderation: {

        // Mutes someone for the call, until lifted: they can't unmute, also after leaving and joining again.
        setModMuted(channelId: string, userId: string, muted: boolean): Promise<void>;

        // Asks a muted participant to unmute themselves.
        askToUnmute(channelId: string, sessionId: string): Promise<void>;

        // In voice channels: deafens someone until lifted (they hear nothing and can't unmute, also after leaving
        // and joining again), and moves someone to another voice channel of the community (their app switches
        // calls; they're disconnected otherwise). Optional: older versions lack them.
        setModDeafened?(channelId: string, userId: string, deafened: boolean): Promise<void>;
        move?(channelId: string, sessionId: string, toChannelId: string): Promise<void>;

        // The moderator mutes and deafens of a channel, the latest first, for those who may moderate its calls (403
        // otherwise); lift them with setModMuted and setModDeafened. Optional: older versions lack it.
        listModMutes?(channelId: string): Promise<CallsLastingMute[]>;

        // What the user may do to a participant (session ID) of the call of a channel, for its menu. Optional: older
        // versions lack it, and their menus offer every control.
        participantPermissions?(channelId: string, sessionId: string): Promise<CallsParticipantPermissions>;
    };

    // Starts or stops the transcription of the call of this window (host only). While it runs, each participant's
    // app sends what they say to be transcribed; the transcript is posted in the call's thread when it stops.
    startTranscription(): Promise<void>;
    stopTranscription(): Promise<void>;

    dismissIncomingCall(channelId: string, callId: string): void;

    // Stops the ring of an incoming call, keeping it.
    stopRinging(callId: string): void;

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
