// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {Permissions} from 'mattermost-redux/constants';

import type {GlobalState} from 'types/store';

// accessState is a store with one community, t1, where you (me) have the given permissions through its member role,
// and the given ones on the server; channelAdmin makes you a channel's admin in it. For the tests of who gets the
// community settings and its pages.
export function accessState(teamPermissions: string[], {systemPermissions = [] as string[], customGroups = true, channelAdmin = false} = {}) {
    return {
        entities: {
            general: {config: {EnableCustomGroups: customGroups ? 'true' : 'false'}},
            admin: {config: {}},
            teams: {
                currentTeamId: 't1',
                teams: {t1: {id: 't1', name: 'one', display_name: 'One', delete_at: 0}},
                myMembers: {t1: {team_id: 't1', user_id: 'me', roles: 'team_user'}},
            },
            channels: {
                myMembers: channelAdmin ? {c1: {channel_id: 'c1', user_id: 'me', roles: 'channel_user channel_admin'}} : {},
                channels: channelAdmin ? {c1: {id: 'c1', team_id: 't1', type: 'O', delete_at: 0}} : {},
            },
            roles: {
                roles: {
                    team_user: {name: 'team_user', permissions: teamPermissions},
                    system_user: {name: 'system_user', permissions: systemPermissions},
                    channel_user: {name: 'channel_user', permissions: [Permissions.CREATE_POST]},
                    channel_admin: {name: 'channel_admin', permissions: [Permissions.MANAGE_CHANNEL_ROLES, Permissions.MANAGE_PUBLIC_CHANNEL_MEMBERS]},
                },
            },
            users: {currentUserId: 'me', profiles: {me: {id: 'me', roles: 'system_user'}}},
        },
    } as unknown as GlobalState;
}

