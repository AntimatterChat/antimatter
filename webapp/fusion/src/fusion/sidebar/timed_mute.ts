// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useEffect} from 'react';
import {useDispatch, useSelector, useStore} from 'react-redux';

import {deletePreferences, savePreferences} from 'mattermost-redux/actions/preferences';
import {getMyChannelMembership} from 'mattermost-redux/selectors/entities/channels';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';
import {isChannelMuted} from 'mattermost-redux/utils/channel_utils';

import {muteChannel, unmuteChannel} from 'actions/channel_actions';

import type {ActionFuncAsync, GlobalState} from 'types/store';

// Channel mutes have no end in Mattermost: a timed mute keeps its end in a preference, and the client unmutes the
// channel once it's past (on the next start if it was closed then).
export const CATEGORY_MUTE_UNTIL = 'fusion_mute_until';

// The mute durations offered, in milliseconds; 0 is until unmuted.
export const MUTE_DURATIONS: Array<[number, {id: string; defaultMessage: string}]> = [
    [36e5, {id: 'fusion.mute.1h', defaultMessage: 'For 1 hour'}],
    [8 * 36e5, {id: 'fusion.mute.8h', defaultMessage: 'For 8 hours'}],
    [864e5, {id: 'fusion.mute.1d', defaultMessage: 'For 24 hours'}],
    [7 * 864e5, {id: 'fusion.mute.1w', defaultMessage: 'For a week'}],
    [0, {id: 'fusion.mute.forever', defaultMessage: 'Until I unmute it'}],
];

function forgetMuteEnd(userId: string, channelId: string, state: GlobalState) {
    const pref = state.entities.preferences.myPreferences[`${CATEGORY_MUTE_UNTIL}--${channelId}`];
    return pref ? deletePreferences(userId, [{user_id: userId, category: CATEGORY_MUTE_UNTIL, name: channelId, value: pref.value}]) : null;
}

// muteFor mutes a channel, until a time to come when given a duration.
export function muteFor(channelId: string, duration: number): ActionFuncAsync {
    return async (dispatch, getState) => {
        const userId = getCurrentUserId(getState());
        await dispatch(muteChannel(userId, channelId));
        if (duration > 0) {
            dispatch(savePreferences(userId, [{user_id: userId, category: CATEGORY_MUTE_UNTIL, name: channelId, value: String(Date.now() + duration)}]));
        } else {
            const forget = forgetMuteEnd(userId, channelId, getState());
            if (forget) {
                dispatch(forget);
            }
        }
        return {data: true};
    };
}

// unmute unmutes a channel and forgets when its mute was to end.
export function unmute(channelId: string): ActionFuncAsync {
    return async (dispatch, getState) => {
        const userId = getCurrentUserId(getState());
        await dispatch(unmuteChannel(userId, channelId));
        const forget = forgetMuteEnd(userId, channelId, getState());
        if (forget) {
            dispatch(forget);
        }
        return {data: true};
    };
}

// getMuteEnd says when a channel's timed mute ends, or 0.
export function getMuteEnd(state: GlobalState, channelId: string): number {
    return Number(state.entities.preferences.myPreferences[`${CATEGORY_MUTE_UNTIL}--${channelId}`]?.value) || 0;
}

// useMuteExpiry unmutes the channels whose timed mute is over, at start and then every minute.
export function useMuteExpiry() {
    const dispatch = useDispatch();
    const store = useStore<GlobalState>();
    const userId = useSelector(getCurrentUserId);
    useEffect(() => {
        const check = () => {
            const state = store.getState();
            const prefix = `${CATEGORY_MUTE_UNTIL}--`;
            for (const [key, pref] of Object.entries(state.entities.preferences.myPreferences)) {
                if (!key.startsWith(prefix) || Number(pref.value) > Date.now()) {
                    continue;
                }
                const channelId = key.slice(prefix.length);
                if (isChannelMuted(getMyChannelMembership(state, channelId))) {
                    dispatch(unmuteChannel(userId, channelId));
                }
                dispatch(deletePreferences(userId, [{user_id: userId, category: CATEGORY_MUTE_UNTIL, name: channelId, value: pref.value}]));
            }
        };
        check();
        const timer = setInterval(check, 60000);
        return () => clearInterval(timer);
    }, [dispatch, store, userId]);
}
