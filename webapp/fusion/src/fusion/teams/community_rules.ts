// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {CommunityRules, CommunityRulesRequest} from '@mattermost/types/community_rules';

import {TeamTypes} from 'mattermost-redux/action_types';
import {fetchCommunityRules} from 'mattermost-redux/actions/teams';
import {Client4} from 'mattermost-redux/client';

import type {ActionFuncAsync, GlobalState} from 'types/store';

// A community's rules (Antimatter), as Discord's rules screening: its admins write them and turn them on, and its
// members agree to them before they post in its channels. The server refuses their messages until then; the Fusion UI
// shows the rules when they open the community, and a notice in place of the message box.

export const COMMUNITY_RULES_NOT_ACCEPTED = 'api.post.community_rules_not_accepted';

export function getCommunityRules(state: GlobalState, teamId: string): CommunityRules | undefined {
    return state.entities.teams.communityRules?.[teamId];
}

// mustAcceptCommunityRules tells whether the current user must agree to a community's rules before posting in it, as
// the server told: the rules are on, they haven't agreed to this version, and they aren't exempt (admins, bots).
export function mustAcceptCommunityRules(state: GlobalState, teamId: string): boolean {
    return Boolean(teamId && getCommunityRules(state, teamId)?.must_accept);
}

export class RulesChangedError extends Error {}

// acceptCommunityRules agrees, for the current user, to the version of a community's rules they read. When an admin
// asked everyone to agree again meanwhile, the server refuses: the new rules are loaded, to be read first.
export function acceptCommunityRules(teamId: string, version: number): ActionFuncAsync<CommunityRules> {
    return async (dispatch) => {
        try {
            const rules = await Client4.acceptCommunityRules(teamId, version);
            dispatch({type: TeamTypes.RECEIVED_COMMUNITY_RULES, data: rules});
            return {data: rules};
        } catch (error) {
            if ((error as {status_code?: number}).status_code === 409) {
                await dispatch(fetchCommunityRules(teamId));
                return {error: new RulesChangedError()};
            }
            return {error};
        }
    };
}

// saveCommunityRules saves a community's rules (for its admins), then loads them as the admin sees them.
export function saveCommunityRules(teamId: string, request: CommunityRulesRequest): ActionFuncAsync<CommunityRules> {
    return async (dispatch) => {
        let saved: CommunityRules;
        try {
            saved = await Client4.updateCommunityRules(teamId, request);
        } catch (error) {
            return {error};
        }
        dispatch(fetchCommunityRules(teamId));
        return {data: saved};
    };
}
