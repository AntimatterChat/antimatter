// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Channel} from '@mattermost/types/channels';
import type {Team} from '@mattermost/types/teams';

import {createSelector} from 'mattermost-redux/selectors/create_selector';
import {getChannelMessageCount, getDirectAndGroupChannels, getMyChannelMemberships} from 'mattermost-redux/selectors/entities/channels';
import {get as getPreference, isCollapsedThreadsEnabled} from 'mattermost-redux/selectors/entities/preferences';
import {getMyTeams} from 'mattermost-redux/selectors/entities/teams';
import {calculateUnreadCount} from 'mattermost-redux/utils/channel_utils';

import {getCurrentLocale} from 'selectors/i18n';

import {Preferences} from 'utils/constants';
import {filterAndSortTeamsByDisplayName} from 'utils/team_utils';

import type {GlobalState} from 'types/store';

export type UnreadDirectChannel = {
    channel: Channel;
    mentions: number;
    messages: number;
};

// getUnreadDirectChannels lists the direct and group messages with unread messages, most recent first: they hang
// off the home icon of the team rail.
export const getUnreadDirectChannels: (state: GlobalState) => UnreadDirectChannel[] = createSelector(
    'fusionGetUnreadDirectChannels',
    getDirectAndGroupChannels,
    getMyChannelMemberships,
    (state: GlobalState) => state.entities.channels.messageCounts,
    isCollapsedThreadsEnabled,
    (channels, memberships, messageCounts, crt) => {
        return channels.
            map((channel) => {
                const unread = calculateUnreadCount(messageCounts[channel.id], memberships[channel.id], crt);
                return {channel, mentions: unread.mentions, messages: unread.messages, show: unread.showUnread};
            }).
            filter((c) => c.show && c.channel.delete_at === 0).
            sort((a, b) => b.channel.last_post_at - a.channel.last_post_at).
            map(({channel, mentions, messages}) => ({channel, mentions, messages}));
    },
);

export const getSortedMyTeams: (state: GlobalState) => Team[] = createSelector(
    'fusionGetSortedMyTeams',
    getMyTeams,
    getCurrentLocale,
    (state: GlobalState) => getPreference(state, Preferences.TEAMS_ORDER, '', ''),
    (teams, locale, order) => filterAndSortTeamsByDisplayName(teams, locale, order),
);

// getLastTeamChannelName names the team's channel you viewed last, leaving out direct messages, which belong to no
// team: where to go back to from them. '' when you haven't joined any of the team's channels.
export function getLastTeamChannelName(state: GlobalState, teamId: string): string {
    const channels = state.entities.channels.channels;
    let last: {name: string; viewedAt: number} = {name: '', viewedAt: -1};
    Object.values(getMyChannelMemberships(state)).forEach((member) => {
        const channel = channels[member.channel_id];
        if (channel && channel.team_id === teamId && channel.delete_at === 0 && member.last_viewed_at > last.viewedAt) {
            last = {name: channel.name, viewedAt: member.last_viewed_at};
        }
    });
    return last.name;
}

export {getChannelMessageCount};
