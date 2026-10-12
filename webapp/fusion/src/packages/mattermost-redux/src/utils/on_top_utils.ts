// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {OnTopPost} from '@mattermost/types/posts';

import {ChannelTypes} from 'mattermost-redux/action_types';

// Antimatter: messages put on top of a channel.

// OnTopChange is a message put on top of its channel (again, or with a new time), or taken off it.
export type OnTopChange =
    {type: typeof ChannelTypes.POST_PUT_ON_TOP; data: OnTopPost} |
    {type: typeof ChannelTypes.POST_REMOVED_FROM_TOP; data: {channel_id: string; post_id: string}};

// applyOnTopChange gives a channel's messages on top after a change, the latest put there first. Changes are
// idempotent: applying one the list already has gives the same list.
export function applyOnTopChange(list: OnTopPost[], change: OnTopChange): OnTopPost[] {
    if (change.type === ChannelTypes.POST_PUT_ON_TOP) {
        const entry = change.data;
        const others = list.filter((o) => o.post_id !== entry.post_id);
        return [entry, ...others].sort((a, b) => b.create_at - a.create_at);
    }
    if (!list.some((o) => o.post_id === change.data.post_id)) {
        return list;
    }
    return list.filter((o) => o.post_id !== change.data.post_id);
}
