// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Copied from the voice channels plugin (antimatter-plugin-voice-channels, webapp/src/types/public_api.ts): keep them
// in sync.

// window.antimatterVoiceChannels: the API through which other UIs (the Fusion web UI) show and
// drive voice channels. This file has no dependencies, so it can be copied as is.
//
// The API is installed when the plugin initializes and removed when it stops. As plugins may load
// after the UI, wait for the 'antimatter-voice-channels:ready' window event when it's missing.

// The webapp's Redux state; the selectors take it as is.
type VoiceGlobalState = object;

export type VoiceParticipant = {
    sessionId: string;
    userId: string;
    muted: boolean;
    deafened: boolean;
    video: boolean;
    screenSharing: boolean;
    speaking: boolean;
    raisedHand: boolean;
    isHost: boolean;

    // The session of the user's call run by this window (or by the desktop app's call window).
    isMe: boolean;
};

export type AntimatterVoiceChannelsAPI = {
    version: 1;

    // Pure selectors on the webapp's Redux store, for useSelector. Their results are memoised and
    // keep the same reference while unchanged.
    selectors: {

        // Whether the channel is a voice channel that isn't archived.
        isVoiceChannel(state: VoiceGlobalState, channelId: string): boolean;

        // The participants of the call of a channel, in the order they joined.
        getParticipants(state: VoiceGlobalState, channelId: string): VoiceParticipant[];

        // Whether the user is deafened in their call.
        isDeafened(state: VoiceGlobalState): boolean;

        // Whether opening a voice channel joins its call (plugin configuration).
        getAutoJoin(state: VoiceGlobalState): boolean;
    };

    // Joins the call of a voice channel. When in the call of another voice channel, it's left
    // first; other calls (e.g. a DM call) are only left with leaveOtherCalls.
    join(channelId: string, opts?: {leaveOtherCalls?: boolean}): Promise<void>;

    // What opening a voice channel from the sidebar does: joins its call if the plugin is
    // configured to.
    autoJoin(channelId: string): void;

    // Leaves the user's call.
    leave(): void;

    // Deafens the user: disables the call's audio and mutes them; undeafening unmutes them if
    // deafening muted them. Other participants see it. Only for calls run by this window.
    setDeafened(deafened: boolean): void;
};

declare global {
    interface Window {
        antimatterVoiceChannels?: AntimatterVoiceChannelsAPI;
    }

    interface WindowEventMap {
        'antimatter-voice-channels:ready': Event;
    }
}
