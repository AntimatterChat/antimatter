// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {defineMessages, FormattedMessage, useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import {Permissions} from 'mattermost-redux/constants';
import {haveISystemPermission} from 'mattermost-redux/selectors/entities/roles';

import type {SearchableStrings} from 'components/admin_console/types';
import AdminHeader from 'components/widgets/admin_console/admin_header';

import {AI_AGENT_SCOPE_SERVER} from 'fusion/agents/agents';
import AgentsSettings from 'fusion/agents/agents_settings';
import ModelConnections from 'fusion/ai/model_connections';
import {IconSprite} from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

// The Fusion styles, should the console be opened before the app.
import 'fusion/styles/_module.scss';

import './ai.scss';

const messages = defineMessages({
    title: {id: 'admin.ai.connections.title', defaultMessage: 'Connections and agents'},
    connections: {id: 'admin.ai.connections.connectionsSection', defaultMessage: 'Model connections'},
    agents: {id: 'admin.ai.connections.agentsSection', defaultMessage: 'Server-wide agents'},
    search: {id: 'admin.ai.connections.search', defaultMessage: 'AI providers, API keys, models, limits and usage'},
    searchAgents: {id: 'admin.ai.connections.searchAgents', defaultMessage: 'Agents: their model, instructions and who may talk to them'},
});

export const searchableStrings: SearchableStrings = Object.values(messages);

// ModelConnectionsPage is the System Console's AI › Connections and agents: the model connections (the server-wide
// ones and every community's) and the server-wide agents built on them, in one place, drawn with the Fusion controls
// the community settings use for a community's own. Agents are for system admins (as the server checks).
export default function ModelConnectionsPage() {
    const {formatMessage} = useIntl();
    const manageAgents = useSelector((state: GlobalState) => haveISystemPermission(state, {permission: Permissions.MANAGE_SYSTEM}));

    return (
        <div className='wrapper--fixed ai-connections-page'>
            <AdminHeader>
                <FormattedMessage {...messages.title}/>
            </AdminHeader>
            <div className='admin-console__wrapper'>
                <div className='admin-console__content'>
                    <div className={am('layer', 'ai-console')}>
                        <IconSprite/>
                        <section aria-label={formatMessage(messages.connections)}>
                            <h3 className={am('set-section')}>{formatMessage(messages.connections)}</h3>
                            <ModelConnections
                                scope='server'
                                listScope=''
                            />
                        </section>
                        {manageAgents && (
                            <section
                                aria-label={formatMessage(messages.agents)}
                                className={am('ai-console-agents')}
                            >
                                <h3 className={am('set-section')}>{formatMessage(messages.agents)}</h3>
                                <AgentsSettings scope={AI_AGENT_SCOPE_SERVER}/>
                            </section>
                        )}
                    </div>
                </div>
            </div>
        </div>
    );
}
