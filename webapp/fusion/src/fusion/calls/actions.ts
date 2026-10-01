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
import type {MyCallState} from './hooks';

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

// callLabel names a call as the mockup's callLabel: “Title”, your call, or Kenji's call.
export function callLabel(state: GlobalState, call: CallsCall, intl: IntlShape): string {
    if (call.title) {
        return intl.formatMessage({id: 'fusion.calls.label.title', defaultMessage: '“{title}”'}, {title: call.title});
    }
    if (call.ownerId === getCurrentUserId(state)) {
        return intl.formatMessage({id: 'fusion.calls.label.yours', defaultMessage: 'your call'});
    }
    return intl.formatMessage({id: 'fusion.calls.label.someones', defaultMessage: '{name}\'s call'}, {name: firstName(state, call.ownerId)});
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
        } catch {
            return;
        }
        const name = channelLabel(getState(), channelId);
        if (moved) {
            showToast(formatMessage({id: 'fusion.calls.toast.moved', defaultMessage: 'Moved to {name}'}, {name}));
        } else if (!quiet) {
            showToast(formatMessage({id: 'fusion.calls.toast.connected', defaultMessage: 'Connected to {name}'}, {name}));
        }
    };

    // Opening a voice channel from the sidebar joins it, when the voice channels plugin is set to.
    const autoJoinVoice = (channelId: string) => {
        const voice = getVoiceAPI();
        const state = getState();
        const local = getLocalCall(state);
        if (!voice || local?.channelId === channelId || !voice.selectors.isVoiceChannel(state, channelId)) {
            return;
        }
        voice.autoJoin(channelId);
        if (voice.selectors.getAutoJoin(state) && local && isVoiceChannel(state, local.channelId)) {
            showToast(formatMessage({id: 'fusion.calls.toast.moved', defaultMessage: 'Moved to {name}'}, {name: channelLabel(state, channelId)}));
        }
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
            showToast(formatMessage({id: 'fusion.calls.toast.joined', defaultMessage: 'Joined {call} in {where}'}, {call: callLabel(getState(), existing, intl), where}));
        } else {
            showToast(formatMessage({id: 'fusion.calls.toast.started', defaultMessage: 'Call started in {where}'}, {where}));
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
    const toggleScreen = (my: MyCallState) => {
        if (my.screen) {
            getCallsAPI()?.stopScreenShare();
        } else {
            getCallsAPI()?.startScreenShare();
        }
    };
    const toggleHand = (my: MyCallState) => getCallsAPI()?.setHandRaised(!my.handRaised);

    // Host controls, for the call's host and admins.
    const hostMute = (p: VoiceParticipant, channelId: string) => {
        const state = getState();
        getCallsAPI()?.host.mute(p.sessionId);
        showToast(formatMessage({id: 'fusion.calls.toast.hostMuted', defaultMessage: 'Muted {name} in {where}'}, {name: firstName(state, p.userId), where: channelLabel(state, channelId)}));
    };
    const hostRemove = (p: VoiceParticipant, channelId: string) => {
        const state = getState();
        getCallsAPI()?.host.remove(p.sessionId);
        const name = firstName(state, p.userId);
        if (isVoiceChannel(state, channelId)) {
            showToast(formatMessage({id: 'fusion.calls.toast.kicked', defaultMessage: 'Disconnected {name} from {where}'}, {name, where: channelLabel(state, channelId)}));
        } else {
            showToast(formatMessage({id: 'fusion.calls.toast.removed', defaultMessage: 'Removed {name} from the call'}, {name}));
        }
    };
    const hostLowerHand = (p: VoiceParticipant) => getCallsAPI()?.host.lowerHand(p.sessionId);
    const hostStopScreen = (p: VoiceParticipant) => getCallsAPI()?.host.stopScreen(p.sessionId);

    return {
        openDirect,
        joinVoice,
        autoJoinVoice,
        startOrJoinCall,
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
