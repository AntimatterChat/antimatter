// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect} from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import type {Team} from '@mattermost/types/teams';

import {Permissions} from 'mattermost-redux/constants';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {haveISystemPermission, haveITeamPermission} from 'mattermost-redux/selectors/entities/roles';

import {AI_AGENT_SCOPE_SERVER} from 'fusion/agents/agents';
import AgentsSettings from 'fusion/agents/agents_settings';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

// AgentsPage is Integrations › Agents: the server-wide agents for system admins, and this community's own for its
// admins (also in the community's settings).
export default function AgentsPage({team}: {team: Team}) {
    const {formatMessage} = useIntl();
    const siteName = useSelector(getConfig).SiteName || '';
    const serverAdmin = useSelector((state: GlobalState) => haveISystemPermission(state, {permission: Permissions.MANAGE_SYSTEM}));
    const communityAdmin = useSelector((state: GlobalState) => haveITeamPermission(state, team.id, Permissions.MANAGE_TEAM));
    const title = formatMessage({id: 'fusion.agents.title', defaultMessage: 'Agents'});

    useEffect(() => {
        document.title = `${title} - ${team.display_name} ${siteName}`.trim();
    }, [title, team.display_name, siteName]);

    return (
        <div className={am('bs-section', 'agents-page')}>
            <h2>{title}</h2>
            {!serverAdmin && !communityAdmin && (
                <p className={am('lead')}>{formatMessage({id: 'fusion.agents.noPermission', defaultMessage: 'Only the server\'s and this community\'s admins manage agents.'})}</p>
            )}
            {serverAdmin && (
                <section aria-label={formatMessage({id: 'fusion.agents.serverSection', defaultMessage: 'Server-wide agents'})}>
                    <h3 className={am('set-section')}>{formatMessage({id: 'fusion.agents.serverSection', defaultMessage: 'Server-wide agents'})}</h3>
                    <AgentsSettings
                        scope={AI_AGENT_SCOPE_SERVER}
                        teamName={team.name}
                    />
                </section>
            )}
            {communityAdmin && (
                <section aria-label={formatMessage({id: 'fusion.agents.communitySection', defaultMessage: 'Agents of {team}'}, {team: team.display_name})}>
                    <h3 className={am('set-section')}>{formatMessage({id: 'fusion.agents.communitySection', defaultMessage: 'Agents of {team}'}, {team: team.display_name})}</h3>
                    <AgentsSettings
                        scope={team.id}
                        teamName={team.name}
                    />
                </section>
            )}
        </div>
    );
}
