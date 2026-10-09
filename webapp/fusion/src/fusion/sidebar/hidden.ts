// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Channel} from '@mattermost/types/channels';
import type {PreferencesType} from '@mattermost/types/preferences';
import type {IDMappedObjects} from '@mattermost/types/utilities';

import {deletePreferences, savePreferences} from 'mattermost-redux/actions/preferences';
import {createSelector} from 'mattermost-redux/selectors/create_selector';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import type {ActionFuncAsync, GlobalState} from 'types/store';

// A conversation can be hidden from the sidebar until something new is said in it (Telegram's and WhatsApp's archive):
// a preference keeps when it was hidden, and the conversation shows again once it has a message newer than that.
export const CATEGORY_HIDDEN = 'fusion_hidden_conversation';

const NONE: ReadonlySet<string> = new Set();

// hiddenSince tells which conversations are hidden and since when, from the preferences.
export function hiddenSince(preferences: PreferencesType, channels: IDMappedObjects<Channel>): Map<string, number> {
    const prefix = `${CATEGORY_HIDDEN}--`;
    const hidden = new Map<string, number>();
    for (const [key, pref] of Object.entries(preferences)) {
        if (!key.startsWith(prefix)) {
            continue;
        }
        const channelId = key.slice(prefix.length);
        const since = Number(pref.value) || 0;
        const channel = channels[channelId];
        if (channel && channel.last_post_at <= since) {
            hidden.set(channelId, since);
        }
    }
    return hidden;
}

// getHiddenChannelIds is the set of the conversations hidden now, with nothing new since.
export const getHiddenChannelIds = createSelector(
    'fusionGetHiddenChannelIds',
    (state: GlobalState) => state.entities.preferences.myPreferences,
    (state: GlobalState) => state.entities.channels.channels,
    (preferences, channels): ReadonlySet<string> => {
        const hidden = hiddenSince(preferences, channels);
        return hidden.size ? new Set(hidden.keys()) : NONE;
    },
);

export function isHidden(state: GlobalState, channelId: string): boolean {
    return getHiddenChannelIds(state).has(channelId);
}

// hideConversation hides a conversation from the sidebar until its next message. It keeps the time of the last one by
// the server's clock, which the next is compared with: this device's clock may be off.
export function hideConversation(channelId: string): ActionFuncAsync {
    return async (dispatch, getState) => {
        const state = getState();
        const userId = getCurrentUserId(state);
        const lastPostAt = state.entities.channels.channels[channelId]?.last_post_at || 0;
        await dispatch(savePreferences(userId, [{user_id: userId, category: CATEGORY_HIDDEN, name: channelId, value: String(lastPostAt)}]));
        return {data: true};
    };
}

// showConversation puts a hidden conversation back in the sidebar.
export function showConversation(channelId: string): ActionFuncAsync {
    return async (dispatch, getState) => {
        const state = getState();
        const userId = getCurrentUserId(state);
        const pref = state.entities.preferences.myPreferences[`${CATEGORY_HIDDEN}--${channelId}`];
        if (pref) {
            await dispatch(deletePreferences(userId, [{user_id: userId, category: CATEGORY_HIDDEN, name: channelId, value: pref.value}]));
        }
        return {data: true};
    };
}
