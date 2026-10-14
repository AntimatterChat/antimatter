// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useCallback, useSyncExternalStore} from 'react';
import {useSelector} from 'react-redux';

import type {Channel, ChannelMembership} from '@mattermost/types/channels';

import {ChannelTypes} from 'mattermost-redux/action_types';
import {deletePreferences, savePreferences} from 'mattermost-redux/actions/preferences';
import {Client4} from 'mattermost-redux/client';
import {createSelector} from 'mattermost-redux/selectors/create_selector';
import {getAllChannels, getChannelByName, getChannelMessageCounts, getDirectAndGroupChannels, getMyChannelMemberships} from 'mattermost-redux/selectors/entities/channels';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {getMyPreferences, get as getPreference} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';
import {getDirectChannelName, getUserIdFromChannelName, isMessageRequest} from 'mattermost-redux/utils/channel_utils';
import type {MessageRequestProps} from 'mattermost-redux/utils/channel_utils';
import {getPreferenceKey} from 'mattermost-redux/utils/preference_utils';

import type {ActionFuncAsync, GlobalState} from 'types/store';

// Ignoring, blocking and message requests, as in Discord. The server enforces them (no notification by any means,
// direct messages refused to someone blocked); the Fusion UI folds the messages of those you ignore or block, lists
// message requests apart from your conversations, and keeps them out of the unread counts.
//
// Whom you ignore or block is one of your preferences (user_block, named after them, worth "ignore" or "block"); how
// you take direct messages from people you don't know yet too (dm_privacy, "default" for all communities or a
// community's id, worth "everyone" or "ask"). Whether a direct message waits as a request is on your channel member
// (notify_props.dm_request), which only you see.
export const USER_BLOCK_CATEGORY = 'user_block';
export const DM_PRIVACY_CATEGORY = 'dm_privacy';
export const DM_PRIVACY_DEFAULT = 'default';

// "Don't ask again for this person", after the offer to block someone whose requests you keep ignoring.
export const BLOCK_PROMPT_CATEGORY = 'block_prompt';

export type BlockLevel = '' | 'ignore' | 'block';
export type DMPrivacy = 'everyone' | 'ask';

// The admin can turn it all off for workplaces where colleagues must always be reachable.
export function isUserBlockingEnabled(state: GlobalState): boolean {
    return getConfig(state)?.EnableUserBlocking !== 'false';
}

export function getBlockLevel(state: GlobalState, userId: string): BlockLevel {
    if (!userId || !isUserBlockingEnabled(state)) {
        return '';
    }
    const level = getPreference(state, USER_BLOCK_CATEGORY, userId, '');
    return level === 'ignore' || level === 'block' ? level : '';
}

export function useBlockLevel(userId?: string): BlockLevel {
    return useSelector((state: GlobalState) => getBlockLevel(state, userId || ''));
}

// getBlockLists lists whom you block and whom you ignore, in the order you did it, as their ids.
export const getBlockLists: (state: GlobalState) => {blocked: string[]; ignored: string[]} = createSelector(
    'fusionGetBlockLists',
    getMyPreferences,
    (preferences) => {
        const blocked: string[] = [];
        const ignored: string[] = [];
        Object.values(preferences).forEach((pref) => {
            if (pref.category === USER_BLOCK_CATEGORY) {
                if (pref.value === 'block') {
                    blocked.push(pref.name);
                } else if (pref.value === 'ignore') {
                    ignored.push(pref.name);
                }
            }
        });
        return {blocked, ignored};
    },
);

export function getDMPrivacy(state: GlobalState, communityId = DM_PRIVACY_DEFAULT): DMPrivacy | '' {
    const value = getPreference(state, DM_PRIVACY_CATEGORY, communityId, '');
    return value === 'everyone' || value === 'ask' ? value : '';
}

function teammateOf(channel: Channel, me: string): string {
    return channel.teammate_id || getUserIdFromChannelName(me, channel.name);
}

export type MessageRequest = {channel: Channel; member: ChannelMembership; userId: string};

// getMessageRequests lists the direct messages waiting as message requests, the latest first: pending ones, shown
// among the message requests, and those you ignored, hidden until their sender writes again. Conversations with
// people you block are in neither: they're hidden until you unblock them.
export const getMessageRequests: (state: GlobalState) => {pending: MessageRequest[]; dismissed: MessageRequest[]} = createSelector(
    'fusionGetMessageRequests',

    // The direct messages completed with their other person's name and id.
    (state: GlobalState) => getDirectAndGroupChannels(state),
    getMyChannelMemberships,
    getChannelMessageCounts,
    getCurrentUserId,
    getMyPreferences,
    isUserBlockingEnabled,
    (directChannels, members, counts, me, preferences, enabled) => {
        const pending: MessageRequest[] = [];
        const dismissed: MessageRequest[] = [];
        if (!enabled) {
            return {pending, dismissed};
        }
        const channels = new Map(directChannels.map((channel) => [channel.id, channel]));
        Object.values(members).forEach((member) => {
            const channel = channels.get(member.channel_id);
            if (!channel || channel.type !== 'D' || channel.delete_at || !isMessageRequest(member)) {
                return;
            }
            const userId = teammateOf(channel, me);
            if (preferences[getPreferenceKey(USER_BLOCK_CATEGORY, userId)]?.value === 'block') {
                return;
            }
            const request = {channel, member, userId};
            if ((member.notify_props as MessageRequestProps).dm_request === 'dismissed') {
                dismissed.push(request);
            } else if ((counts[channel.id]?.total ?? 0) > 0 || channel.last_post_at > 0) {
                pending.push(request);
            }
        });
        pending.sort((a, b) => b.channel.last_post_at - a.channel.last_post_at);
        return {pending, dismissed};
    },
);

// getHiddenDirectIds lists the direct messages left out of your conversations: message requests (also those without
// messages yet), those you ignored, and conversations with people you block.
export const getHiddenDirectIds: (state: GlobalState) => Set<string> = createSelector(
    'fusionGetHiddenDirectIds',
    getMyChannelMemberships,
    getAllChannels,
    getCurrentUserId,
    getBlockLists,
    isUserBlockingEnabled,
    (members, channels, me, {blocked}, enabled) => {
        const ids = new Set<string>();
        if (!enabled) {
            return ids;
        }
        Object.values(members).forEach((member) => {
            if (isMessageRequest(member)) {
                ids.add(member.channel_id);
            }
        });
        if (blocked.length) {
            const names = new Set(blocked.map((userId) => getDirectChannelName(me, userId)));
            Object.values(channels).forEach((channel) => {
                if (channel.type === 'D' && names.has(channel.name)) {
                    ids.add(channel.id);
                }
            });
        }
        return ids;
    },
);

export function isChannelMessageRequest(state: GlobalState, channelId: string): boolean {
    return isUserBlockingEnabled(state) && isMessageRequest(getMyChannelMemberships(state)[channelId]);
}

function receivedMember(member: ChannelMembership) {
    return {type: ChannelTypes.RECEIVED_MY_CHANNEL_MEMBER, data: member};
}

// updateMessageRequest acts on one of your direct messages: accept it, ignore it, or put it among the requests.
export function updateMessageRequest(channelId: string, action: 'accept' | 'dismiss' | 'request'): ActionFuncAsync<ChannelMembership> {
    return async (dispatch) => {
        try {
            const member = await Client4.updateMessageRequest(channelId, action);
            dispatch(receivedMember(member));
            return {data: member};
        } catch (error) {
            return {error};
        }
    };
}

// setBlockLevel ignores someone, blocks them, or forgets both (''). Your direct message with them follows: it goes
// among the message requests as you ignore or block them, out of the unread counts, and back to your conversations
// as you stop.
export function setBlockLevel(userId: string, level: BlockLevel): ActionFuncAsync {
    return async (dispatch, getState) => {
        const state = getState();
        const me = getCurrentUserId(state);
        const pref = {user_id: me, category: USER_BLOCK_CATEGORY, name: userId, value: level};
        const result = level ? await dispatch(savePreferences(me, [pref])) : await dispatch(deletePreferences(me, [pref]));
        if ('error' in result && result.error) {
            return result;
        }

        const direct = getChannelByName(state, getDirectChannelName(me, userId));
        const member = direct ? getMyChannelMemberships(state)[direct.id] : undefined;
        if (direct && member) {
            if (level && !isMessageRequest(member)) {
                await dispatch(updateMessageRequest(direct.id, 'request'));
            } else if (!level && isMessageRequest(member)) {
                await dispatch(updateMessageRequest(direct.id, 'accept'));
            }
        }
        return {data: true};
    };
}

export function setDMPrivacy(communityId: string, value: DMPrivacy | ''): ActionFuncAsync {
    return async (dispatch, getState) => {
        const me = getCurrentUserId(getState());
        const pref = {user_id: me, category: DM_PRIVACY_CATEGORY, name: communityId, value};
        return value ? dispatch(savePreferences(me, [pref])) : dispatch(deletePreferences(me, [pref]));
    };
}

// The folded messages of those you ignore or block that you chose to see, for the session, also when they're drawn
// again (scrolled back to, or opened in a thread).
const revealed = new Set<string>();
const listeners = new Set<() => void>();

function subscribe(listener: () => void) {
    listeners.add(listener);
    return () => {
        listeners.delete(listener);
    };
}

export function useRevealed(postId: string): [boolean, () => void] {
    const shown = useSyncExternalStore(subscribe, () => revealed.has(postId));
    const reveal = useCallback(() => {
        revealed.add(postId);
        listeners.forEach((listener) => listener());
    }, [postId]);
    return [shown, reveal];
}
