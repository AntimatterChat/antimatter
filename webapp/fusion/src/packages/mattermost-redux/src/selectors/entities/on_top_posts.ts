// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {OnTopPost} from '@mattermost/types/posts';
import type {GlobalState} from '@mattermost/types/store';

const NONE: OnTopPost[] = [];

// Antimatter: messages put on top of a channel.

// isOnTopActive tells whether a message is still on top at a time: it stays until removed, or until it expires.
export function isOnTopActive(entry: OnTopPost, now: number): boolean {
    return entry.expires_at === 0 || entry.expires_at > now;
}

// getOnTopPosts gives the messages loaded as on top of a channel, the latest put there first, expired ones included:
// filter them with isOnTopActive at the time shown.
export function getOnTopPosts(state: GlobalState, channelId: string): OnTopPost[] {
    return state.entities.channels.onTop?.[channelId] || NONE;
}

// getOnTopEntry is a message's place on top of its channel while it's there.
export function getOnTopEntry(state: GlobalState, channelId: string, postId: string, now = Date.now()): OnTopPost | undefined {
    return getOnTopPosts(state, channelId).find((o) => o.post_id === postId && isOnTopActive(o, now));
}
