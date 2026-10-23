// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Channel} from '@mattermost/types/channels';

import {Permissions} from 'mattermost-redux/constants';
import {haveIChannelPermission} from 'mattermost-redux/selectors/entities/roles';
import {getTeam} from 'mattermost-redux/selectors/entities/teams';

import Constants from 'utils/constants';

import type {GlobalState} from 'types/store';

// canArchiveChannel tells whether one may archive a channel of a community, as the server does
// (SessionCanArchiveChannel) and as Fusion's canArchiveChannel: those the permissions let (the community's admins,
// roles that manage channels, what a system admin gave members), and in a workspace the channel's admins too, that
// is whoever created it. In a community, members don't archive channels, not even those they created.
export function canArchiveChannel(state: GlobalState, channel: Channel | undefined): boolean {
    if (!channel || !channel.team_id || channel.delete_at || (channel.type !== Constants.OPEN_CHANNEL && channel.type !== Constants.PRIVATE_CHANNEL)) {
        return false;
    }
    const permission = channel.type === Constants.PRIVATE_CHANNEL ? Permissions.DELETE_PRIVATE_CHANNEL : Permissions.DELETE_PUBLIC_CHANNEL;
    if (haveIChannelPermission(state, channel.team_id, channel.id, permission)) {
        return true;
    }
    return getTeam(state, channel.team_id)?.kind === 'workspace' && haveIChannelPermission(state, channel.team_id, channel.id, Permissions.MANAGE_CHANNEL_ROLES);
}
