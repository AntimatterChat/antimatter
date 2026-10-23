// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';

import type {Team} from '@mattermost/types/teams';

import AgentsSettings from 'fusion/agents/agents_settings';
import ModelConnections from 'fusion/ai/model_connections';
import {Section} from 'fusion/modals/settings_controls';
import {am} from 'fusion/utils/class_names';

type Props = {
    team: Team;

    // What one may manage here: the community's own model connections, and its agents.
    connections: boolean;
    agents: boolean;
};

// AISettings is the AI tab of the community settings: its own model connections (the AI providers and keys its agents
// and features use), then its agents.
export default function AISettings({team, connections, agents}: Props) {
    const {formatMessage} = useIntl();
    return (
        <div className={am('ai-pane')}>
            {connections && (
                <>
                    <Section title={formatMessage({id: 'fusion.teamSettings.aiConnections', defaultMessage: 'Model connections'})}/>
                    <ModelConnections
                        scope={team.id}
                        listScope={team.id}
                    />
                </>
            )}
            {agents && (
                <>
                    <Section title={formatMessage({id: 'fusion.teamSettings.agents', defaultMessage: 'Agents'})}/>
                    <AgentsSettings
                        scope={team.id}
                        teamName={team.name}
                    />
                </>
            )}
        </div>
    );
}
