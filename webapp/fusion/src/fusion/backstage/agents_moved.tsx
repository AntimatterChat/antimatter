// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect} from 'react';
import {useDispatch, useSelector} from 'react-redux';
import {Redirect} from 'react-router-dom';

import type {Team} from '@mattermost/types/teams';

import {Permissions} from 'mattermost-redux/constants';
import {haveISystemPermission, haveITeamPermission} from 'mattermost-redux/selectors/entities/roles';

import type {FusionTeamSettingsTab} from 'components/team_settings_modal/team_settings_modal';

import {openTeamSettings} from 'fusion/utils/modals';

import type {GlobalState} from 'types/store';

// The System Console's page of the server-wide agents (with the model connections).
export const SERVER_AGENTS_PATH = '/admin_console/ai/model_connections';

// The community settings' AI tab: the community's model connections and agents.
export const COMMUNITY_AI_TAB: FusionTeamSettingsTab = 'ai';

// AgentsMoved is what's left of Integrations › Agents (/<team>/integrations/agents, where the AI chat's old agents
// page and its "Set up an agent" also lead): agents are made in the AI settings now. A system admin is sent to the
// System Console's AI › Connections and agents; a community's admin to the community's settings, on its AI tab; anyone
// else back to the integrations.
export default function AgentsMoved({team}: {team: Team}) {
    const dispatch = useDispatch();
    const serverAdmin = useSelector((state: GlobalState) => haveISystemPermission(state, {permission: Permissions.MANAGE_SYSTEM}));
    const communityAdmin = useSelector((state: GlobalState) => haveITeamPermission(state, team.id, Permissions.MANAGE_TEAM));
    const openCommunitySettings = !serverAdmin && communityAdmin;

    useEffect(() => {
        if (openCommunitySettings) {
            dispatch(openTeamSettings(COMMUNITY_AI_TAB));
        }
    }, [openCommunitySettings, dispatch]);

    if (serverAdmin) {
        return <Redirect to={SERVER_AGENTS_PATH}/>;
    }
    if (communityAdmin) {
        return <Redirect to={`/${team.name}`}/>;
    }
    return <Redirect to={`/${team.name}/integrations`}/>;
}
