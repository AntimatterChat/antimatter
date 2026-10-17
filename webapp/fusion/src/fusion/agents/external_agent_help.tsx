// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';

import type {AIAgentDetails} from '@mattermost/types/ai_agents';

import {am} from 'fusion/utils/class_names';
import {getSiteURL} from 'utils/url';

import {agentName, externalAgentExamples} from './agents';

type Props = {
    agent: AIAgentDetails;

    // The community the Integrations page belongs to, for the link to its bot accounts.
    teamName?: string;
};

// ExternalAgentHelp tells how the program behind an external agent signs in: a token for its account (made by a
// system admin on the bot accounts page, or with the tokens API), then the API and the websocket with that token, and
// its activity with the agent activity API.
export default function ExternalAgentHelp({agent, teamName}: Props) {
    const {formatMessage} = useIntl();
    const examples = externalAgentExamples(getSiteURL(), agent.user_id);
    return (
        <div
            className={am('ai-status', 'agent-external-help')}
            role='region'
            aria-label={formatMessage({id: 'fusion.agents.externalHelpLabel', defaultMessage: 'Connect {name}'}, {name: agentName(agent)})}
        >
            <b>{formatMessage({id: 'fusion.agents.externalHelpTitle', defaultMessage: 'Connect the program behind {name}'}, {name: agentName(agent)})}</b>
            <p>
                {formatMessage({id: 'fusion.agents.externalHelpToken', defaultMessage: 'It signs in with a token of the agent\'s account (@{username}). A system admin makes one in Integrations › Bot accounts: find @{username} and pick Create new token. Or with the API:'}, {username: agent.username})}
                {teamName && (
                    <>
                        {' '}
                        <a href={`/${teamName}/integrations/bots`}>{formatMessage({id: 'fusion.agents.externalHelpBots', defaultMessage: 'Open Bot accounts'})}</a>
                    </>
                )}
            </p>
            <pre className={am('agent-token')}>{examples.token}</pre>
            <p>{formatMessage({id: 'fusion.agents.externalHelpUse', defaultMessage: 'The program then calls the API and opens the websocket with the header {header}, reads its mentions and messages, and answers like a person would. It shows what it\'s doing with:'}, {header: 'Authorization: Bearer <token>'})}</p>
            <pre className={am('agent-token')}>{examples.activity}</pre>
            <p>{formatMessage({id: 'fusion.agents.externalHelpKeep', defaultMessage: 'Keep the token secret: anyone who has it can post as the agent.'})}</p>
        </div>
    );
}
