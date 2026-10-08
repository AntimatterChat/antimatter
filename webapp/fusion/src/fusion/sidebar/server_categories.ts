// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {CategorySorting} from '@mattermost/types/channel_categories';
import type {ChannelCategory, ChannelCategoryType, ServerCategory} from '@mattermost/types/channel_categories';
import type {Channel} from '@mattermost/types/channels';

import {addChannelToCategory, addChannelToManagedCategory, removeChannelFromManagedCategory, saveServerCategories} from 'mattermost-redux/actions/channel_categories';
import {patchChannel} from 'mattermost-redux/actions/channels';
import {savePreferences} from 'mattermost-redux/actions/preferences';
import {Permissions} from 'mattermost-redux/constants';
import {CategoryTypes} from 'mattermost-redux/constants/channel_categories';
import {createSelector} from 'mattermost-redux/selectors/create_selector';
import {getCategoryInTeamByType, getCategoryInTeamWithChannel, getManagedCategoryMappings, makeGetNonManagedCategoriesForTeam} from 'mattermost-redux/selectors/entities/channel_categories';
import {get as getPreference} from 'mattermost-redux/selectors/entities/preferences';
import {haveIChannelPermission, haveITeamPermission} from 'mattermost-redux/selectors/entities/roles';
import {getCurrentTeamId} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import type {ActionFunc, ActionFuncAsync, GlobalState} from 'types/store';

// Server categories (Antimatter) are the categories an admin sets up for everyone in a team, above everyone's own.
// Channels are filed in them by name (Mattermost's managed categories). Moving a channel of an open category into one
// of your own categories (or your favourites) wins for you; a locked category keeps its channels.
export const SERVER_CATEGORY_PREFIX = 'managed_';

// Which server categories each person folded, per team: a list of names.
export const CATEGORY_SERVER_COLLAPSED = 'fusion_server_collapsed';

const NO_SERVER_CATEGORIES: ServerCategory[] = [];
const NO_COLLAPSED: string[] = [];

export function isServerCategory(category: Pick<ChannelCategory, 'type'>): boolean {
    return category.type === CategoryTypes.MANAGED;
}

export function serverCategoryName(category: Pick<ChannelCategory, 'display_name'>): string {
    return category.display_name;
}

export function getServerCategories(state: GlobalState, teamId: string): ServerCategory[] {
    return state.entities.channelCategories.serverCategories?.[teamId] || NO_SERVER_CATEGORIES;
}

// findServerCategory finds a team's server category by name, ignoring case, as the server does.
export function findServerCategory(categories: ServerCategory[], name: string): ServerCategory | undefined {
    const key = name.trim().toLowerCase();
    return categories.find((c) => c.name.toLowerCase() === key);
}

function getCollapsedServerCategories(state: GlobalState, teamId: string): string[] {
    const value = getPreference(state, CATEGORY_SERVER_COLLAPSED, teamId, '');
    if (!value) {
        return NO_COLLAPSED;
    }
    try {
        const names = JSON.parse(value);
        return Array.isArray(names) ? names : NO_COLLAPSED;
    } catch {
        return NO_COLLAPSED;
    }
}

export function isTeamAdmin(state: GlobalState, teamId: string): boolean {
    return haveITeamPermission(state, teamId, Permissions.MANAGE_TEAM);
}

// layoutSidebar puts a team's channels in the sidebar: the server categories in their order (the open ones the list
// lacks after them), then each person's own categories without the channels shown above. A channel of an open server
// category that its person moved into one of their custom categories or their favourites stays there.
export function layoutSidebar(
    personal: ChannelCategory[],
    mappings: Record<string, string>,
    serverCategories: ServerCategory[],
    collapsed: string[],
    userId: string,
    teamId: string,
    showEmpty: boolean,
): ChannelCategory[] {
    const keptBy = new Map<string, ChannelCategoryType>();
    for (const category of personal) {
        if (category.type === CategoryTypes.CUSTOM || category.type === CategoryTypes.FAVORITES) {
            for (const id of category.channel_ids) {
                keptBy.set(id, category.type);
            }
        }
    }

    const names = serverCategories.map((c) => c.name);
    const extra = new Set<string>();
    const filed = new Map<string, string[]>();
    for (const [channelId, rawName] of Object.entries(mappings)) {
        const category = findServerCategory(serverCategories, rawName);
        const name = category?.name || rawName;
        if (!category?.locked && keptBy.has(channelId)) {
            continue;
        }
        if (!category && !names.includes(name)) {
            extra.add(name);
        }
        filed.set(name, [...(filed.get(name) || []), channelId]);
    }
    const order = [...names, ...[...extra].sort((a, b) => a.localeCompare(b, undefined, {numeric: true}))];

    const shownAbove = new Set([...filed.values()].flat());
    const server: ChannelCategory[] = order.
        filter((name) => showEmpty || filed.get(name)?.length).
        map((name) => ({
            id: SERVER_CATEGORY_PREFIX + name,
            user_id: userId,
            team_id: teamId,
            type: CategoryTypes.MANAGED as ChannelCategoryType,
            display_name: name,
            sorting: CategorySorting.Alphabetical,
            channel_ids: filed.get(name) || [],
            muted: false,
            collapsed: collapsed.includes(name),
        }));

    if (!shownAbove.size) {
        return [...server, ...personal];
    }
    return [
        ...server,
        ...personal.map((category) => (category.channel_ids.some((id) => shownAbove.has(id)) ? {...category, channel_ids: category.channel_ids.filter((id) => !shownAbove.has(id))} : category)),
    ];
}

const getPersonalCategoriesForCurrentTeam = (() => {
    const getForTeam = makeGetNonManagedCategoriesForTeam();
    return (state: GlobalState) => getForTeam(state, getCurrentTeamId(state));
})();

// getSidebarCategories is the current team's sidebar, server categories first. Team admins also see the empty ones,
// to fill them.
export const getSidebarCategories: (state: GlobalState) => ChannelCategory[] = createSelector(
    'fusionGetSidebarCategories',
    getPersonalCategoriesForCurrentTeam,
    (state: GlobalState) => getManagedCategoryMappings(state, getCurrentTeamId(state)),
    (state: GlobalState) => getServerCategories(state, getCurrentTeamId(state)),
    (state: GlobalState) => getCollapsedServerCategories(state, getCurrentTeamId(state)),
    getCurrentUserId,
    getCurrentTeamId,
    (state: GlobalState) => isTeamAdmin(state, getCurrentTeamId(state)),
    (personal, mappings, serverCategories, collapsed, userId, teamId, admin) => layoutSidebar(personal, mappings || {}, serverCategories, collapsed, userId, teamId, admin),
);

// setServerCategoryCollapsed folds or unfolds a server category for its person.
export function setServerCategoryCollapsed(teamId: string, name: string, collapsed: boolean): ActionFunc {
    return (dispatch, getState) => {
        const state = getState();
        const current = getCollapsedServerCategories(state, teamId);
        const next = collapsed ? [...new Set([...current, name])] : current.filter((n) => n !== name);
        const userId = getCurrentUserId(state);
        dispatch(savePreferences(userId, [{user_id: userId, category: CATEGORY_SERVER_COLLAPSED, name: teamId, value: JSON.stringify(next)}]));
        return {data: true};
    };
}

// canFileChannel tells whether the person may file a channel in a server category, or take it out of one (''), as the
// server checks it: a team admin anywhere; else someone who manages the channel's roles, between open categories.
export function canFileChannel(state: GlobalState, channel: Pick<Channel, 'id' | 'team_id'>, name: string): boolean {
    if (isTeamAdmin(state, channel.team_id)) {
        return true;
    }
    const categories = getServerCategories(state, channel.team_id);
    const current = getManagedCategoryMappings(state, channel.team_id)?.[channel.id] || '';
    for (const n of [current, name]) {
        if (!n.trim()) {
            continue;
        }
        const category = findServerCategory(categories, n);
        if (!category || category.locked) {
            return false;
        }
    }
    return haveIChannelPermission(state, channel.team_id, channel.id, Permissions.MANAGE_CHANNEL_ROLES);
}

// fileChannel files a channel in a server category for everyone, or takes it out of its category (''). Others hear of
// it from the server; this person's sidebar follows at once.
export function fileChannel(channelId: string, name: string): ActionFuncAsync {
    return async (dispatch) => {
        const result = await dispatch(patchChannel(channelId, {managed_category_name: name}));
        if ('data' in result && result.data) {
            const channel = result.data;
            dispatch(name ? addChannelToManagedCategory(channel.team_id, channel.id, channel.managed_category_name || name) : removeChannelFromManagedCategory(channel.team_id, channel.id));
        }
        return result;
    };
}

// fileChannelHere files a channel in a server category and shows it there for its person too: out of their own
// category or favourites, where it would otherwise stay for them.
export function fileChannelHere(channel: Pick<Channel, 'id' | 'team_id'>, name: string): ActionFuncAsync {
    return async (dispatch, getState) => {
        const result = await dispatch(fileChannel(channel.id, name));
        if ('error' in result && result.error) {
            return result;
        }
        const state = getState();
        const mine = getCategoryInTeamWithChannel(state, channel.team_id, channel.id);
        const channels = getCategoryInTeamByType(state, channel.team_id, CategoryTypes.CHANNELS);
        if (mine && channels && mine.id !== channels.id && (mine.type === CategoryTypes.CUSTOM || mine.type === CategoryTypes.FAVORITES)) {
            await dispatch(addChannelToCategory(channels.id, channel.id));
        }
        return {data: true};
    };
}

// The admins' edits of the list: each sends the whole list, in order.
function save(teamId: string, categories: Array<Pick<ServerCategory, 'name' | 'locked'>>, renames?: Record<string, string>): ActionFuncAsync<ServerCategory[]> {
    return saveServerCategories(teamId, {categories: categories.map(({name, locked}) => ({name, locked})), renames});
}

export function addServerCategory(teamId: string, name: string, locked: boolean): ActionFuncAsync<ServerCategory[]> {
    return (dispatch, getState) => dispatch(save(teamId, [...getServerCategories(getState(), teamId), {name: name.trim(), locked}]));
}

export function renameServerCategory(teamId: string, from: string, to: string): ActionFuncAsync<ServerCategory[]> {
    return (dispatch, getState) => {
        const categories = getServerCategories(getState(), teamId);
        const known = findServerCategory(categories, from);
        const list = known ? categories.map((c) => (c === known ? {...c, name: to.trim()} : c)) : [...categories, {name: to.trim(), locked: false}];
        return dispatch(save(teamId, list, {[known?.name || from]: to.trim()}));
    };
}

export function setServerCategoryLocked(teamId: string, name: string, locked: boolean): ActionFuncAsync<ServerCategory[]> {
    return (dispatch, getState) => {
        const categories = getServerCategories(getState(), teamId);
        const known = findServerCategory(categories, name);
        const list = known ? categories.map((c) => (c === known ? {...c, locked} : c)) : [...categories, {name, locked}];
        return dispatch(save(teamId, list));
    };
}

export function moveServerCategory(teamId: string, name: string, by: -1 | 1): ActionFuncAsync<ServerCategory[]> {
    return (dispatch, getState) => {
        const list = [...getServerCategories(getState(), teamId)];
        const at = list.findIndex((c) => c.name === name);
        const to = at + by;
        if (at < 0 || to < 0 || to >= list.length) {
            return Promise.resolve({data: list});
        }
        [list[at], list[to]] = [list[to], list[at]];
        return dispatch(save(teamId, list));
    };
}

// deleteServerCategory removes a category: its channels go back to everyone's own categories.
export function deleteServerCategory(teamId: string, name: string): ActionFuncAsync<ServerCategory[]> {
    return (dispatch, getState) => dispatch(save(teamId, getServerCategories(getState(), teamId).filter((c) => c.name !== name)));
}
