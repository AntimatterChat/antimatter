// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useCallback, useEffect, useRef} from 'react';
import {defineMessage} from 'react-intl';
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

// The mute durations offered, in milliseconds; 0 is until unmuted. defineMessage puts them in the catalog to
// translate.
export const MUTE_DURATIONS: Array<[number, {id: string; defaultMessage: string}]> = [
    [36e5, defineMessage({id: 'fusion.mute.1h', defaultMessage: 'For 1 hour'})],
    [8 * 36e5, defineMessage({id: 'fusion.mute.8h', defaultMessage: 'For 8 hours'})],
    [864e5, defineMessage({id: 'fusion.mute.1d', defaultMessage: 'For 24 hours'})],
    [7 * 864e5, defineMessage({id: 'fusion.mute.1w', defaultMessage: 'For a week'})],
    [0, defineMessage({id: 'fusion.mute.forever', defaultMessage: 'Until I unmute it'})],
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

// useMuteExpiry unmutes the channels whose timed mute is over, at start and then every minute. A channel whose
// membership isn't loaded yet (right after start) waits for it: its end is forgotten only once it's unmuted, or found
// unmuted already, or the mute would stay on for good with no end left to undo it.
export function useMuteExpiry() {
    const dispatch = useDispatch();
    const store = useStore<GlobalState>();
    const userId = useSelector(getCurrentUserId);

    // Checked again as soon as the memberships arrive, not a minute later.
    const myMembers = useSelector((state: GlobalState) => state.entities.channels.myMembers);
    const pending = useRef(new Set<string>());

    const check = useCallback(() => {
        const state = store.getState();
        const prefix = `${CATEGORY_MUTE_UNTIL}--`;
        for (const [key, pref] of Object.entries(state.entities.preferences.myPreferences)) {
            if (!key.startsWith(prefix) || Number(pref.value) > Date.now()) {
                continue;
            }
            const channelId = key.slice(prefix.length);
            const member = getMyChannelMembership(state, channelId);
            if (!member || pending.current.has(channelId)) {
                continue;
            }
            const forget = () => dispatch(deletePreferences(userId, [{user_id: userId, category: CATEGORY_MUTE_UNTIL, name: channelId, value: pref.value}]));
            if (!isChannelMuted(member)) {
                forget();
                continue;
            }
            pending.current.add(channelId);
            Promise.resolve(dispatch(unmuteChannel(userId, channelId))).then((result) => {
                if (!(result && typeof result === 'object' && 'error' in result && result.error)) {
                    forget();
                }
            }).finally(() => {
                pending.current.delete(channelId);
            });
        }
    }, [dispatch, store, userId]);

    useEffect(() => {
        check();
    }, [check, myMembers]);

    useEffect(() => {
        const timer = setInterval(check, 60000);
        return () => clearInterval(timer);
    }, [check]);
}
