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
