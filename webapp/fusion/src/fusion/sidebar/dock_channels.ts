// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {makeGetChannelIdsForCategory} from 'mattermost-redux/selectors/entities/channel_categories';

import {getHiddenDirectIds} from 'fusion/privacy/blocking';

import type {GlobalState} from 'types/store';

import {getHiddenChannelIds} from './hidden';
import {getSidebarCategories} from './server_categories';

const NONE: string[] = [];

// makeGetDockChannelIds lists the conversations of the sidebar's Open conversations dock: the direct messages
// category, unread or not, without the hidden conversations, the message requests and the people you block. The
// team rail leaves these out of its unread bubbles, as they show in the dock already.
export function makeGetDockChannelIds(): (state: GlobalState) => string[] {
    const getChannelIds = makeGetChannelIdsForCategory();
    let last: string[] = NONE;
    return (state) => {
        const category = getSidebarCategories(state).find((c) => c.type === 'direct_messages');
        if (!category) {
            last = NONE;
            return last;
        }
        const hidden = getHiddenChannelIds(state);
        const requestsAndBlocked = getHiddenDirectIds(state);
        const ids = getChannelIds(state, category).filter((id) => !hidden.has(id) && !requestsAndBlocked.has(id));
        if (ids.length !== last.length || ids.some((id, i) => id !== last[i])) {
            last = ids;
        }
        return last;
    };
}
