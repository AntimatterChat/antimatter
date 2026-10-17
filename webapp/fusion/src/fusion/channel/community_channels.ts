// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Channel} from '@mattermost/types/channels';

import {Permissions} from 'mattermost-redux/constants';
import {getChannelsNameMapInTeam} from 'mattermost-redux/selectors/entities/channels';
import {haveIChannelPermission} from 'mattermost-redux/selectors/entities/roles';
import {getTeam} from 'mattermost-redux/selectors/entities/teams';
import {getUser} from 'mattermost-redux/selectors/entities/users';
import {isGuest} from 'mattermost-redux/utils/user_utils';

import Constants from 'utils/constants';

import type {GlobalState} from 'types/store';

// A community's channels, as the server keeps them (server/channels/app/community_channels.go).
//
// A community can have everyone in all its channels, as in Discord: its members are in every public channel and every
// private channel their roles open, and mute channels rather than leave them. Guests and bots aren't concerned.
//
// Its default channel is where newcomers land and its join and leave messages go: the public channel its admins chose,
// else Town Square. Either can be archived, and the community then has none.

// isAllChannelsCommunity tells whether a community has everyone in all its channels.
export function isAllChannelsCommunity(state: GlobalState, teamId: string): boolean {
    return Boolean(getTeam(state, teamId)?.all_channels_membership);
}

// isCommunityChannelFor tells whether someone is in a channel because they're in its community: they can't leave it,
// and nobody can remove them from it.
export function isCommunityChannelFor(state: GlobalState, channel: Channel | undefined, userId: string): boolean {
    if (!channel || !channel.team_id || channel.delete_at !== 0 || channel.policy_enforced || !isAllChannelsCommunity(state, channel.team_id)) {
        return false;
    }
    const gated = channel.type === Constants.PRIVATE_CHANNEL && Boolean(channel.group_constrained);
    if (channel.type !== Constants.OPEN_CHANNEL && !gated) {
        return false;
    }
    const user = getUser(state, userId);
    return Boolean(user) && !user.is_bot && !isGuest(user.roles) && user.delete_at === 0;
}

// getDefaultChannel gives a community's default channel, when it's known here and open to all.
export function getDefaultChannel(state: GlobalState, teamId: string): Channel | undefined {
    const team = getTeam(state, teamId);
    if (!team) {
        return undefined;
    }
    const channel = team.default_channel_id ? state.entities.channels.channels[team.default_channel_id] : getChannelsNameMapInTeam(state, teamId)[Constants.DEFAULT_CHANNEL];
    return channel && channel.team_id === teamId && channel.type === Constants.OPEN_CHANNEL && channel.delete_at === 0 ? channel : undefined;
}

// isDefaultChannel tells whether a channel is its community's default channel.
export function isDefaultChannel(state: GlobalState, channel: Channel | undefined): boolean {
    return Boolean(channel?.team_id) && getDefaultChannel(state, channel!.team_id)?.id === channel!.id;
}

// isFirstDefaultChannel tells whether a channel is one of those every community starts with (Town Square and
// Off-Topic), or the one it chose: only the community's admins archive those.
export function isFirstDefaultChannel(state: GlobalState, channel: Channel): boolean {
    return channel.name === Constants.DEFAULT_CHANNEL || channel.name === Constants.OFFTOPIC_CHANNEL || isDefaultChannel(state, channel);
}

// isWorkplaceCommunity tells whether a community is of the workplace kind (a company's or an organisation's
// workspace) rather than a community (the default).
export function isWorkplaceCommunity(state: GlobalState, teamId: string): boolean {
    return getTeam(state, teamId)?.kind === 'workplace';
}

// canArchiveChannel tells whether one may archive a channel of a community, as the server does
// (SessionCanArchiveChannel): those the permissions let (the community's admins, roles that manage channels, what a
// system admin gave members), and in a workplace community the channel's admins too, that is whoever created it. In
// a community, members don't archive channels, not even those they created. The default channels' extra rule (the
// community's admins only) is the caller's.
export function canArchiveChannel(state: GlobalState, channel: Channel | undefined): boolean {
    if (!channel || !channel.team_id || channel.delete_at !== 0 || (channel.type !== Constants.OPEN_CHANNEL && channel.type !== Constants.PRIVATE_CHANNEL)) {
        return false;
    }
    const permission = channel.type === Constants.PRIVATE_CHANNEL ? Permissions.DELETE_PRIVATE_CHANNEL : Permissions.DELETE_PUBLIC_CHANNEL;
    if (haveIChannelPermission(state, channel.team_id, channel.id, permission)) {
        return true;
    }
    return isWorkplaceCommunity(state, channel.team_id) && haveIChannelPermission(state, channel.team_id, channel.id, Permissions.MANAGE_CHANNEL_ROLES);
}
