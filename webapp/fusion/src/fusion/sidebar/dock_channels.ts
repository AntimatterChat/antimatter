// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {makeGetChannelIdsForCategory} from 'mattermost-redux/selectors/entities/channel_categories';
import {getCurrentChannelId} from 'mattermost-redux/selectors/entities/channels';
import {getVisibleDmGmLimit} from 'mattermost-redux/selectors/entities/preferences';

import {getHiddenDirectIds} from 'fusion/privacy/blocking';

import type {GlobalState} from 'types/store';

import {getHiddenChannelIds} from './hidden';
import {getSidebarCategories} from './server_categories';

const NONE: string[] = [];

// makeGetDockChannelIds lists the conversations of the sidebar's Open conversations dock: the direct messages
// category, without the hidden conversations, the message requests and the people you block, and no more than the
// "Open conversations" setting allows (sidebar_settings/limit_visible_dms_gms, 10 by default) nor than the rows that
// fit in the sidebar (rows, measured by useDockFit; null until measured). Which ones: the conversation you're in, even
// when no row fits, then the latest active (their last message), in the category's own order. Unread ones left out
// wait at the bottom of the team rail instead (see rail/rail_dms.tsx).
export function makeGetDockChannelIds(): (state: GlobalState, rows?: number | null) => string[] {
    const getChannelIds = makeGetChannelIdsForCategory();
    let last: string[] = NONE;
    return (state, rows = null) => {
        const category = getSidebarCategories(state).find((c) => c.type === 'direct_messages');
        if (!category) {
            last = NONE;
            return last;
        }
        const hidden = getHiddenChannelIds(state);
        const requestsAndBlocked = getHiddenDirectIds(state);
        let ids = getChannelIds(state, category).filter((id) => !hidden.has(id) && !requestsAndBlocked.has(id));

        const current = getCurrentChannelId(state);
        const limit = Math.max(ids.includes(current) ? 1 : 0, Math.min(Math.max(1, getVisibleDmGmLimit(state)), rows ?? Infinity));
        if (ids.length > limit) {
            const channels = state.entities.channels.channels;
            const activity = (id: string) => channels[id]?.last_post_at || channels[id]?.create_at || 0;
            const kept = new Set([...ids].sort((a, b) => (Number(b === current) - Number(a === current)) || (activity(b) - activity(a))).slice(0, limit));
            ids = ids.filter((id) => kept.has(id));
        }

        if (ids.length !== last.length || ids.some((id, i) => id !== last[i])) {
            last = ids;
        }
        return last;
    };
}
