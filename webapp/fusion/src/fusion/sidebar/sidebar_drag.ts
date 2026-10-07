// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {CategorySorting} from '@mattermost/types/channel_categories';
import type {ChannelCategory} from '@mattermost/types/channel_categories';

import {moveChannelsToCategory, patchCategory} from 'mattermost-redux/actions/channel_categories';
import {getCategory, makeGetChannelIdsForCategory} from 'mattermost-redux/selectors/entities/channel_categories';

import type {ActionFuncAsync} from 'types/store';

// What is being dragged in the sidebar: a channel out of a category, or a category by its header. The browser only
// tells drop targets the data's types while dragging, so the rest is kept here.
export type SidebarDrag =
    | {kind: 'channel'; channelId: string; categoryId: string; directMessage: boolean} |
    {kind: 'category'; categoryId: string};

let current: SidebarDrag | null = null;

export function startSidebarDrag(drag: SidebarDrag, dataTransfer: DataTransfer) {
    current = drag;
    dataTransfer.effectAllowed = 'move';
    dataTransfer.setData('application/x-am-sidebar', drag.kind === 'channel' ? drag.channelId : drag.categoryId);
}

export function getSidebarDrag(): SidebarDrag | null {
    return current;
}

export function endSidebarDrag() {
    current = null;
}

// canDropChannel tells whether a channel can go in a category, as in the classic sidebar: direct and group messages
// only in Favorites and custom categories, other channels anywhere but the direct messages category.
export function canDropChannel(drag: SidebarDrag | null, category: ChannelCategory): drag is Extract<SidebarDrag, {kind: 'channel'}> {
    if (drag?.kind !== 'channel') {
        return false;
    }
    if (category.type === 'custom' || category.type === 'favorites') {
        return true;
    }
    return !drag.directMessage && category.type !== 'direct_messages';
}

// moveSidebarChannel puts a dropped channel in a category, before or after one of its channels (first without one).
// A category sorted alphabetically or by recency becomes manually sorted, in the order it showed: the drop then lands
// where it was made, rather than among the category's channels in the order they were added.
export function moveSidebarChannel(categoryId: string, channelId: string, targetId: string | null, after: boolean): ActionFuncAsync {
    return async (dispatch, getState) => {
        let category = getCategory(getState(), categoryId);
        if (!category) {
            return {data: false};
        }
        if (category.sorting !== CategorySorting.Manual && category.type !== 'direct_messages') {
            const shown = makeGetChannelIdsForCategory()(getState(), category);
            const hidden = category.channel_ids.filter((id) => !shown.includes(id));
            await dispatch(patchCategory(category.id, {sorting: CategorySorting.Manual, channel_ids: [...shown, ...hidden]}));
            category = getCategory(getState(), categoryId);
        }

        const index = channelDropIndex(category.channel_ids, channelId, targetId, after);
        if (index === -1) {
            return {data: false};
        }
        return dispatch(moveChannelsToCategory(categoryId, [channelId], index, true));
    };
}

// channelDropIndex is where a channel lands in a category's channels when dropped before or after one of them (first
// without one), taking it out of the list first when it was already in it; -1 when it stays where it is.
export function channelDropIndex(channelIds: string[], channelId: string, targetId: string | null, after: boolean): number {
    let index = targetId ? channelIds.indexOf(targetId) + (after ? 1 : 0) : 0;
    const sourceIndex = channelIds.indexOf(channelId);
    if (sourceIndex !== -1 && sourceIndex < index) {
        index -= 1;
    }
    return index === sourceIndex ? -1 : index;
}

// categoryDropIndex is the new place in the team's category order of a category dropped before or after another.
export function categoryDropIndex(order: string[], categoryId: string, targetId: string, after: boolean): number {
    let index = order.indexOf(targetId) + (after ? 1 : 0);
    const sourceIndex = order.indexOf(categoryId);
    if (sourceIndex !== -1 && sourceIndex < index) {
        index -= 1;
    }
    return index === sourceIndex ? -1 : index;
}

// isLowerHalf tells whether the pointer is over the lower half of an element, to drop after it rather than before.
export function isLowerHalf(e: {clientY: number; currentTarget: Element}): boolean {
    const rect = e.currentTarget.getBoundingClientRect();
    return e.clientY > rect.top + (rect.height / 2);
}
