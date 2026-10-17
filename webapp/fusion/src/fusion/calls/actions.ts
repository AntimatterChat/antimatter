// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useMemo} from 'react';
import {useIntl} from 'react-intl';
import type {IntlShape} from 'react-intl';
import {useDispatch, useSelector, useStore} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';

import {makeGetChannel} from 'mattermost-redux/selectors/entities/channels';
import {getTeammateNameDisplaySetting} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentTeam, getTeam} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUserId, getUser} from 'mattermost-redux/selectors/entities/users';
import {displayUsername} from 'mattermost-redux/utils/user_utils';

import {openDirectChannelToUserId} from 'actions/channel_actions';

import {showToast} from 'fusion/components/toast';
import {channelPath} from 'fusion/utils/paths';
import {getHistory} from 'utils/browser_history';

import type {GlobalState} from 'types/store';

import {getCall, getCallsAPI, getLocalCall, getVoiceAPI, isVoiceChannel} from './calls_api';
import type {CallsCall, VoiceParticipant} from './calls_api';
import {getScreenQuality, getScreenSurface} from './extras';
import {endCallForEveryone, makeHost as requestMakeHost, startRecording, stopRecording} from './host_api';
import type {MyCallState} from './hooks';
import {screenPicker} from './screen_picker';
import {startScreenShare} from './screen_sources';
import {setCallView, setCallWindowOpen} from './ui_state';

const getChannelWithNames = makeGetChannel();

function isDirect(channel?: Channel) {
    return channel?.type === 'D' || channel?.type === 'G';
}

// channelLabel names a channel as the call UI does: #name for channels, the name alone for voice channels and
// direct messages.
export function channelLabel(state: GlobalState, channelId: string): string {
    const channel = getChannelWithNames(state, channelId);
    if (!channel) {
        return '';
    }
    if (isDirect(channel) || isVoiceChannel(state, channelId)) {
        return channel.display_name;
    }
    return '#' + channel.display_name;
}

// useChannelLabel is channelLabel for components.
export function useChannelLabel(channelId?: string): string {
    const getChannel = useMemo(() => makeGetChannel(), []);
    return useSelector((state: GlobalState) => {
        const channel = channelId ? getChannel(state, channelId) : undefined;
        if (!channel) {
            return '';
        }
        return isDirect(channel) || isVoiceChannel(state, channel.id) ? channel.display_name : '#' + channel.display_name;
    });
}

export function firstName(state: GlobalState, userId: string): string {
    const user = getUser(state, userId);
    return user?.first_name || displayUsername(user, getTeammateNameDisplaySetting(state));
}

// joinedMessage tells that the user joined a call, named as the mockup's callLabel: “Title”, your call, or Kenji's
// call (whole sentences, which languages with grammatical cases need).
function joinedMessage(state: GlobalState, call: CallsCall, where: string, intl: IntlShape): string {
    if (call.title) {
        return intl.formatMessage({id: 'fusion.calls.toast.joinedTitled', defaultMessage: 'Joined “{title}” in {where}'}, {title: call.title, where});
    }
    if (call.ownerId === getCurrentUserId(state)) {
        return intl.formatMessage({id: 'fusion.calls.toast.joinedYours', defaultMessage: 'Joined your call in {where}'}, {where});
    }
    return intl.formatMessage({id: 'fusion.calls.toast.joinedSomeones', defaultMessage: 'Joined {name}’s call in {where}'}, {name: firstName(state, call.ownerId), where});
}

// The team to show a channel in: its own, or the current one for direct messages.
export function teamNameFor(state: GlobalState, channelId: string): string {
    const channel = getChannelWithNames(state, channelId);
    const team = channel?.team_id ? getTeam(state, channel.team_id) : getCurrentTeam(state);
    return team?.name || getCurrentTeam(state)?.name || '';
}

export function goToChannel(state: GlobalState, channelId: string) {
    const channel = getChannelWithNames(state, channelId);
    const teamName = teamNameFor(state, channelId);
    if (channel && teamName) {
        getHistory().push(channelPath(teamName, channel));
    }
}

// showCall shows a call in the main view: a voice channel's own view, or a conversation's call in place of its
// messages, with the same stage and controls. Popping it out to a window of its own is done from there.
export function showCall(state: GlobalState, channelId: string) {
    setCallWindowOpen(false);
    if (!isVoiceChannel(state, channelId)) {
        setCallView(channelId);
    }
    goToChannel(state, channelId);
}

// The HTTP status a moderation request failed with, if any.
function statusOf(e: unknown): number {
    return (e as {status?: number})?.status || 0;
}

export type CallActions = ReturnType<typeof makeActions>;

function makeActions(getState: () => GlobalState, intl: IntlShape, openDirect: (userId: string) => Promise<Channel | undefined>) {
    const {formatMessage} = intl;

    // Joining a voice channel leaves any other call; you're in one call or room at a time.
    const joinVoice = async (channelId: string, quiet = false) => {
        const voice = getVoiceAPI();
        const state = getState();
        const local = getLocalCall(state);
        if (!voice || local?.channelId === channelId) {
            return;
        }
        const moved = Boolean(local && isVoiceChannel(state, local.channelId));
        try {
            await voice.join(channelId, {leaveOtherCalls: true});
        } catch (e) {
            showToast(formatMessage({id: 'fusion.calls.toast.joinFailed', defaultMessage: 'Couldn\'t join {name}: {error}'}, {name: channelLabel(getState(), channelId), error: (e as Error)?.message || String(e)}));
            return;
        }
        const name = channelLabel(getState(), channelId);
        if (moved) {
            showToast(formatMessage({id: 'fusion.calls.toast.moved', defaultMessage: 'Moved to {name}'}, {name}));
        } else if (!quiet) {
            showToast(formatMessage({id: 'fusion.calls.toast.connected', defaultMessage: 'Connected to {name}'}, {name}));
        }
    };

    // Clicking a voice channel in the sidebar joins it and leaves you where you are, as in Discord, when the voice
    // channels plugin is set to join on open; clicking it again (once in it), or without auto-join, shows its view.
    // Tells whether it joined.
    const openVoiceFromSidebar = (channelId: string): boolean => {
        const voice = getVoiceAPI();
        const state = getState();
        if (!voice || getLocalCall(state)?.channelId === channelId || !voice.selectors.isVoiceChannel(state, channelId) ||
            !voice.selectors.getAutoJoin(state)) {
            return false;
        }
        joinVoice(channelId);
        return true;
    };

    // startOrJoinCall joins the call in progress in a channel, or starts one (Mattermost has one call per channel).
    const startOrJoinCall = async (channelId: string) => {
        const calls = getCallsAPI();
        const state = getState();
        if (isVoiceChannel(state, channelId)) {
            await joinVoice(channelId);
            return;
        }
        const local = getLocalCall(state);
        if (!calls || local?.channelId === channelId) {
            return;
        }
        const existing = getCall(state, channelId);
        try {
            await calls.join(channelId, {switchCall: Boolean(local)});
        } catch {
            // Calls refuses to join where calls are turned off.
            showToast(formatMessage({id: 'fusion.calls.toast.disabled', defaultMessage: 'Calls are turned off in {where}'}, {where: channelLabel(getState(), channelId)}));
            return;
        }
        const where = channelLabel(getState(), channelId);
        if (existing) {
            showToast(joinedMessage(getState(), existing, where, intl));
        } else {
            showToast(formatMessage({id: 'fusion.calls.toast.started', defaultMessage: 'Call started in {where}'}, {where}));
        }
    };

    // joinAndShowCall joins a conversation's call and shows it in the main view, from its card or its banner.
    const joinAndShowCall = async (channelId: string) => {
        await startOrJoinCall(channelId);
        if (getLocalCall(getState())?.channelId === channelId) {
            showCall(getState(), channelId);
        }
    };

    const leaveCall = () => {
        const state = getState();
        const local = getLocalCall(state);
        if (!local) {
            return;
        }
        const name = channelLabel(state, local.channelId);
        const voice = getVoiceAPI();
        if (voice && isVoiceChannel(state, local.channelId)) {
            voice.leave();
            showToast(formatMessage({id: 'fusion.calls.toast.disconnected', defaultMessage: 'Disconnected from {name}'}, {name}));
        } else {
            getCallsAPI()?.leave();
            showToast(formatMessage({id: 'fusion.calls.toast.left', defaultMessage: 'Left the call in {where}'}, {where: name}));
        }
    };

    // Call someone: open your direct message with them, and start or join its call.
    const callUser = async (userId: string) => {
        const channel = await openDirect(userId);
        if (channel) {
            goToChannel(getState(), channel.id);
            await startOrJoinCall(channel.id);
        }
    };

    // The toggles of the voice panel, the controls and the call window. Unmuting also undeafens; deafening mutes.
    // They're called straight from click handlers: screen sharing needs the user's gesture.
    const toggleMute = (my: MyCallState) => {
        const calls = getCallsAPI();
        if (my.muted && my.modMuted) {
            showToast(formatMessage({id: 'fusion.calls.toast.modMutedCantUnmute', defaultMessage: 'A moderator muted you: you can unmute once they allow it'}));
            return;
        }
        if (my.muted) {
            if (my.deafened) {
                getVoiceAPI()?.setDeafened(false);
            }
            calls?.setMuted(false);
        } else {
            calls?.setMuted(true);
        }
    };
    const toggleDeafen = (my: MyCallState) => getVoiceAPI()?.setDeafened(!my.deafened);
    const toggleVideo = (my: MyCallState) => {
        getCallsAPI()?.setVideo(!my.video).catch(() => {
            // Calls reports camera errors itself.
        });
    };

    // Sharing again asks for the size, the frame rate and the kind of source chosen last in the share menu.
    const toggleScreen = (my: MyCallState) => {
        if (my.screen) {
            getCallsAPI()?.stopScreenShare();
        } else {
            startScreenShare(getScreenQuality(), screenPicker() === 'surfaces' ? getScreenSurface() : undefined);
        }
    };
    const toggleHand = (my: MyCallState) => getCallsAPI()?.setHandRaised(!my.handRaised);

    // Host controls, for the call's host and moderators (see useCanModerateCall); they act on the call of the
    // channel, which moderators needn't be in.
    const hostMute = (p: VoiceParticipant, channelId: string) => {
        const state = getState();
        getCallsAPI()?.host.mute(p.sessionId, channelId);
        showToast(formatMessage({id: 'fusion.calls.toast.hostMuted', defaultMessage: 'Muted {name} in {where}'}, {name: firstName(state, p.userId), where: channelLabel(state, channelId)}));
    };
    const hostRemove = (p: VoiceParticipant, channelId: string) => {
        const state = getState();
        getCallsAPI()?.host.remove(p.sessionId, channelId);
        const name = firstName(state, p.userId);
        if (isVoiceChannel(state, channelId)) {
            showToast(formatMessage({id: 'fusion.calls.toast.kicked', defaultMessage: 'Disconnected {name} from {where}'}, {name, where: channelLabel(state, channelId)}));
        } else {
            showToast(formatMessage({id: 'fusion.calls.toast.removed', defaultMessage: 'Removed {name} from the call'}, {name}));
        }
    };
    const hostLowerHand = (p: VoiceParticipant, channelId: string) => getCallsAPI()?.host.lowerHand(p.sessionId, channelId);
    const hostStopScreen = (p: VoiceParticipant, channelId: string) => getCallsAPI()?.host.stopScreen(p.sessionId, channelId);

    // A moderator mute keeps someone muted for the call, also when they leave and come back, until lifted.
    const modMute = async (p: VoiceParticipant, channelId: string, muted: boolean) => {
        const moderation = getCallsAPI()?.moderation;
        if (!moderation) {
            return;
        }
        const name = firstName(getState(), p.userId);
        try {
            await moderation.setModMuted(channelId, p.userId, muted);
        } catch (e) {
            showToast(statusOf(e) === 403 ? formatMessage({id: 'fusion.calls.toast.modMuteForbidden', defaultMessage: 'You can\'t mute {name}'}, {name}) : formatMessage({id: 'fusion.calls.toast.modMuteFailed', defaultMessage: 'Couldn\'t change {name}\'s mute'}, {name}));
            return;
        }
        if (muted) {
            showToast(formatMessage({id: 'fusion.calls.toast.modMuted', defaultMessage: 'Muted {name} for this call: they can\'t unmute until you allow it'}, {name}));
        } else {
            showToast(formatMessage({id: 'fusion.calls.toast.modUnmuted', defaultMessage: '{name} can unmute again'}, {name}));
        }
    };

    // Asking to unmute prompts someone muted to unmute themselves; it can't be asked again right away.
    const askToUnmute = async (p: VoiceParticipant, channelId: string) => {
        const moderation = getCallsAPI()?.moderation;
        if (!moderation) {
            return;
        }
        const name = firstName(getState(), p.userId);
        try {
            await moderation.askToUnmute(channelId, p.sessionId);
        } catch (e) {
            showToast(statusOf(e) === 429 ? formatMessage({id: 'fusion.calls.toast.askedRecently', defaultMessage: '{name} was asked a moment ago: wait a little'}, {name}) : formatMessage({id: 'fusion.calls.toast.askFailed', defaultMessage: 'Couldn\'t ask {name} to unmute'}, {name}));
            return;
        }
        showToast(formatMessage({id: 'fusion.calls.toast.asked', defaultMessage: 'Asked {name} to unmute'}, {name}));
    };

    // Makes someone in the call its host (for the host and system admins): they keep it until they leave.
    const makeHost = async (p: VoiceParticipant, channelId: string) => {
        const name = firstName(getState(), p.userId);
        try {
            await requestMakeHost(channelId, p.userId);
        } catch {
            showToast(formatMessage({id: 'fusion.calls.toast.makeHostFailed', defaultMessage: 'Couldn\'t make {name} the host'}, {name}));
            return;
        }
        showToast(formatMessage({id: 'fusion.calls.toast.madeHost', defaultMessage: '{name} is now the host'}, {name}));
    };

    // Ends a channel's call for everyone in it, from its card once out of it (see canEndCall).
    const endForEveryone = async (channelId: string) => {
        const where = channelLabel(getState(), channelId);
        try {
            await endCallForEveryone(channelId);
        } catch {
            showToast(formatMessage({id: 'fusion.calls.toast.endFailed', defaultMessage: 'Couldn\'t end the call in {where}'}, {where}));
            return;
        }
        showToast(formatMessage({id: 'fusion.calls.toast.ended', defaultMessage: 'Ended the call in {where} for everyone'}, {where}));
    };

    // The host records the call; the recording is posted in the call's thread once it stops.
    const toggleRecording = async (channelId: string, recording: boolean) => {
        try {
            await (recording ? stopRecording(channelId) : startRecording(channelId));
        } catch {
            showToast(recording ?
                formatMessage({id: 'fusion.calls.toast.recordingStopFailed', defaultMessage: 'The recording couldn\'t be stopped'}) :
                formatMessage({id: 'fusion.calls.toast.recordingStartFailed', defaultMessage: 'The recording couldn\'t be started'}));
            return;
        }
        showToast(recording ?
            formatMessage({id: 'fusion.calls.toast.recordingStopped', defaultMessage: 'Recording stopped: it will be posted in the call\'s thread'}) :
            formatMessage({id: 'fusion.calls.toast.recordingStarted', defaultMessage: 'Recording started'}));
    };

    return {
        openDirect,
        joinVoice,
        openVoiceFromSidebar,
        startOrJoinCall,
        joinAndShowCall,
        leaveCall,
        callUser,
        toggleMute,
        toggleDeafen,
        toggleVideo,
        toggleScreen,
        toggleHand,
        hostMute,
        hostRemove,
        hostLowerHand,
        hostStopScreen,
        modMute,
        askToUnmute,
        makeHost,
        endForEveryone,
        toggleRecording,
    };
}

// useCallActions gives the call UI's actions: joining, leaving, the toggles and host controls, with their toasts.
export function useCallActions(): CallActions {
    const store = useStore<GlobalState>();
    const dispatch = useDispatch();
    const intl = useIntl();
    return useMemo(() => {
        const openDirect = async (userId: string) => {
            const result = await dispatch(openDirectChannelToUserId(userId));
            return 'data' in result ? result.data as Channel | undefined : undefined;
        };
        return makeActions(store.getState, intl, openDirect);
    }, [store, dispatch, intl]);
}
