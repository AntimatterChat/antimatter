// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {Posts} from 'mattermost-redux/constants';
import {getPost, getPostIdsInChannel} from 'mattermost-redux/selectors/entities/posts';
import {get as getPreference} from 'mattermost-redux/selectors/entities/preferences';

import {getThreadTitle} from 'fusion/messages/thread_title';

import type {GlobalState} from 'types/store';

// How long a topic (a named thread) keeps showing under its channel once nothing new is said in it: a preference of
// yours, in Fusion's sidebar settings. Preference names are capped at 32 characters.
export const CATEGORY_SIDEBAR = 'fusion_sidebar';
export const NAME_TOPIC_PERIOD = 'topic_period';
export const DEFAULT_TOPIC_PERIOD = '1w';

const DAY = 864e5;

export const TOPIC_PERIODS: Array<[string, number]> = [
    ['1d', DAY],
    ['3d', 3 * DAY],
    ['1w', 7 * DAY],
    ['2w', 14 * DAY],
    ['1m', 30 * DAY],
];

export function topicPeriodMs(period: string): number {
    return TOPIC_PERIODS.find(([key]) => key === period)?.[1] ?? TOPIC_PERIODS.find(([key]) => key === DEFAULT_TOPIC_PERIOD)![1];
}

export function getTopicPeriodMs(state: GlobalState): number {
    return topicPeriodMs(getPreference(state, CATEGORY_SIDEBAR, NAME_TOPIC_PERIOD, DEFAULT_TOPIC_PERIOD));
}

// activeTopicAt is when a topic was last active: its last reply, else when it started. Undefined for a post that
// isn't a topic (unnamed, a reply itself, or gone).
export function activeTopicAt(post?: {root_id: string; delete_at: number; state?: string; last_reply_at?: number; create_at: number; props: Record<string, unknown>}): number | undefined {
    if (!post || post.root_id || post.delete_at || post.state === Posts.POST_DELETED || !getThreadTitle(post)) {
        return undefined;
    }
    return post.last_reply_at || post.create_at;
}

// topicIdsForChannel are the channel's topics (its named threads) active in the period, every one of them and not
// just the ones you follow: unlike an ordinary thread, a topic is meant for the whole channel to see. Only among the
// posts loaded at the bottom of the channel (as the message list itself shows); an older topic nobody's loaded isn't
// known to be active anyway.
export function topicIdsForChannel(state: GlobalState, channelId: string, now: number): string[] {
    const periodMs = getTopicPeriodMs(state);
    const ids = getPostIdsInChannel(state, channelId) || [];
    return ids.filter((id) => {
        const at = activeTopicAt(getPost(state, id));
        return at !== undefined && now - at < periodMs;
    });
}
