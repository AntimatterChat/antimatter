// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Team} from '@mattermost/types/teams';

import {setCategoryMuted} from 'mattermost-redux/actions/channel_categories';
import {readMultipleChannels} from 'mattermost-redux/actions/channels';
import {savePreferences} from 'mattermost-redux/actions/preferences';
import {createSelector} from 'mattermost-redux/selectors/create_selector';
import {makeGetCategoriesForTeam} from 'mattermost-redux/selectors/entities/channel_categories';
import {getAllChannels, getMyChannelMemberships} from 'mattermost-redux/selectors/entities/channels';
import {get as getPreference, isCollapsedThreadsEnabled} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentTeamId} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';
import {calculateUnreadCount} from 'mattermost-redux/utils/channel_utils';

import {getHistory} from 'utils/browser_history';

import type {ActionFuncAsync, GlobalState} from 'types/store';

// Antimatter's own sidebar preferences: hiding a team's muted channels (Discord's "Hide muted channels").
const CATEGORY_FUSION_SIDEBAR = 'fusion_sidebar';
const hideMutedName = (teamId: string) => `hide_muted_channels--${teamId}`;

export function isHidingMutedChannels(state: GlobalState, teamId: string): boolean {
    return getPreference(state, CATEGORY_FUSION_SIDEBAR, hideMutedName(teamId), 'false') === 'true';
}

export function setHidingMutedChannels(teamId: string, hide: boolean): ActionFuncAsync {
    return (dispatch, getState) => {
        const userId = getCurrentUserId(getState());
        return dispatch(savePreferences(userId, [{user_id: userId, category: CATEGORY_FUSION_SIDEBAR, name: hideMutedName(teamId), value: String(hide)}]));
    };
}

// getUnreadChannelIdsInTeam lists the team's channels with unread messages, whichever team is current.
export const getUnreadChannelIdsInTeam = createSelector(
    'fusionGetUnreadChannelIdsInTeam',
    getAllChannels,
    getMyChannelMemberships,
    (state: GlobalState) => state.entities.channels.messageCounts,
    isCollapsedThreadsEnabled,
    (state: GlobalState, teamId: string) => teamId,
    (channels, members, messageCounts, crt, teamId) => Object.values(channels).
        filter((channel) => channel.team_id === teamId && channel.delete_at === 0 && members[channel.id]).
        filter((channel) => calculateUnreadCount(messageCounts[channel.id], members[channel.id], crt).showUnread).
        map((channel) => channel.id),
);

const getCategoriesForTeam = makeGetCategoriesForTeam();

// A team is muted when all its categories are: Mattermost has no team-wide mute, but a muted category mutes its
// channels, including the ones added to it later.
function mutableCategories(state: GlobalState, teamId: string) {
    return getCategoriesForTeam(state, teamId).filter((category) => category.type !== 'direct_messages' && category.channel_ids.length > 0);
}

export function isTeamMuted(state: GlobalState, teamId: string): boolean {
    const categories = mutableCategories(state, teamId);
    return categories.length > 0 && categories.every((category) => category.muted);
}

export function setTeamMuted(teamId: string, muted: boolean): ActionFuncAsync {
    return async (dispatch, getState) => {
        const categories = mutableCategories(getState(), teamId).filter((category) => category.muted !== muted);
        await Promise.all(categories.map((category) => dispatch(setCategoryMuted(category.id, muted))));
        return {data: true};
    };
}

export function markTeamAsRead(teamId: string): ActionFuncAsync {
    return (dispatch, getState) => dispatch(readMultipleChannels(getUnreadChannelIdsInTeam(getState(), teamId)));
}

// withTeam runs an action of the current team on the given one: it switches to it first, and runs the action once
// it's current, for the dialogs that act on the current team (settings, invitations, channel creation…).
export function withTeam(team: Team, action: () => void): ActionFuncAsync {
    return async (dispatch, getState) => {
        if (getCurrentTeamId(getState()) === team.id) {
            action();
            return {data: true};
        }
        getHistory().push(`/${team.name}`);
        const deadline = Date.now() + 5000;
        while (getCurrentTeamId(getState()) !== team.id && Date.now() < deadline) {
            // eslint-disable-next-line no-await-in-loop
            await new Promise((resolve) => setTimeout(resolve, 50));
        }
        if (getCurrentTeamId(getState()) === team.id) {
            action();
        }
        return {data: true};
    };
}
