// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {defineMessage} from 'react-intl';
import type {MessageDescriptor} from 'react-intl';

import {fetchTeamTimeouts} from 'mattermost-redux/actions/teams';
import {Client4} from 'mattermost-redux/client';
import {Permissions} from 'mattermost-redux/constants';
import {haveITeamPermission} from 'mattermost-redux/selectors/entities/roles';

import {ActionTypes, RHSStates} from 'utils/constants';

import type {ActionFunc, ActionFuncAsync, GlobalState} from 'types/store';

// Antimatter moderation in the Fusion UI, as Discord's: the mod view of someone in the right-hand side, and
// timeouts.

// canModerateTeam tells whether someone gets the team's mod view, as the server tells it: its admins, and those who
// may remove its members, manage its roles or manage its messages.
export function canModerateTeam(state: GlobalState, teamId: string): boolean {
    return [Permissions.MANAGE_TEAM, Permissions.REMOVE_USER_FROM_TEAM, Permissions.MANAGE_TEAM_ROLES, Permissions.DELETE_OTHERS_POSTS].
        some((p) => haveITeamPermission(state, teamId, p));
}

// canBanInTeam tells whether someone may ban in a team and see its bans, as the server tells it: its admins, and those
// who may remove its members (the server also checks that the person banned ranks below them).
export function canBanInTeam(state: GlobalState, teamId: string): boolean {
    return [Permissions.MANAGE_TEAM, Permissions.REMOVE_USER_FROM_TEAM].some((p) => haveITeamPermission(state, teamId, p));
}

// openModView shows someone's mod view in the right-hand side.
export function openModView(userId: string): ActionFunc {
    return (dispatch) => {
        dispatch({type: ActionTypes.UPDATE_RHS_STATE, state: RHSStates.MOD_VIEW, userId});
        return {data: true};
    };
}

const MINUTE = 60 * 1000;

// The timeouts offered, as Discord's.
export const TIMEOUT_DURATIONS: Array<{ms: number; label: MessageDescriptor}> = [
    {ms: MINUTE, label: defineMessage({id: 'fusion.timeout.1m', defaultMessage: '60 seconds'})},
    {ms: 5 * MINUTE, label: defineMessage({id: 'fusion.timeout.5m', defaultMessage: '5 minutes'})},
    {ms: 10 * MINUTE, label: defineMessage({id: 'fusion.timeout.10m', defaultMessage: '10 minutes'})},
    {ms: 60 * MINUTE, label: defineMessage({id: 'fusion.timeout.1h', defaultMessage: '1 hour'})},
    {ms: 24 * 60 * MINUTE, label: defineMessage({id: 'fusion.timeout.1d', defaultMessage: '1 day'})},
    {ms: 7 * 24 * 60 * MINUTE, label: defineMessage({id: 'fusion.timeout.1w', defaultMessage: '1 week'})},
];

// timeOutMember times someone out of a team until a time, or lifts their timeout (until 0).
export function timeOutMember(teamId: string, userId: string, until: number, reason = ''): ActionFuncAsync {
    return async (dispatch) => {
        try {
            await Client4.setMemberTimeout(teamId, userId, until, reason);
        } catch (error) {
            return {error};
        }
        dispatch(fetchTeamTimeouts(teamId));
        return {data: true};
    };
}

const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;

// How long a ban lasts: until it's lifted (0), or a while.
export const BAN_DURATIONS: Array<{ms: number; label: MessageDescriptor}> = [
    {ms: 0, label: defineMessage({id: 'fusion.ban.untilLifted', defaultMessage: 'Until it\'s lifted'})},
    {ms: DAY, label: defineMessage({id: 'fusion.ban.1d', defaultMessage: '1 day'})},
    {ms: 7 * DAY, label: defineMessage({id: 'fusion.ban.7d', defaultMessage: '7 days'})},
    {ms: 30 * DAY, label: defineMessage({id: 'fusion.ban.30d', defaultMessage: '30 days'})},
];

// How far back a ban deletes the person's messages in the community, as Discord's (the server allows up to 7 days).
export const BAN_PURGE_WINDOWS: Array<{seconds: number; label: MessageDescriptor}> = [
    {seconds: 0, label: defineMessage({id: 'fusion.ban.purge.none', defaultMessage: 'Don\'t delete any'})},
    {seconds: 3600, label: defineMessage({id: 'fusion.ban.purge.1h', defaultMessage: 'The last hour'})},
    {seconds: 6 * 3600, label: defineMessage({id: 'fusion.ban.purge.6h', defaultMessage: 'The last 6 hours'})},
    {seconds: 12 * 3600, label: defineMessage({id: 'fusion.ban.purge.12h', defaultMessage: 'The last 12 hours'})},
    {seconds: 24 * 3600, label: defineMessage({id: 'fusion.ban.purge.24h', defaultMessage: 'The last 24 hours'})},
    {seconds: 3 * 24 * 3600, label: defineMessage({id: 'fusion.ban.purge.3d', defaultMessage: 'The last 3 days'})},
    {seconds: 7 * 24 * 3600, label: defineMessage({id: 'fusion.ban.purge.7d', defaultMessage: 'The last 7 days'})},
];
