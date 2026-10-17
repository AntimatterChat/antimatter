// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useEffect, useState} from 'react';
import {useIntl} from 'react-intl';

import type {AIAgentDetails} from '@mattermost/types/ai_agents';

import {Client4} from 'mattermost-redux/client';

import {errorMessage} from 'fusion/ai/connections';
import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';

import AgentForm from './agent_form';
import {IntegrationTag} from './agent_status';
import {AI_AGENT_SCOPE_SERVER, agentName} from './agents';
import ExternalAgentHelp from './external_agent_help';

type Props = {

    // The agents of a scope: "server" on the Integrations page, the community's ID in its settings.
    scope: string;

    // The community of the page, for links to its integrations.
    teamName?: string;
};

type View = {kind: 'list'} | {kind: 'new'} | {kind: 'edit'; id: string} | {kind: 'connect'; id: string};

const byName = (a: AIAgentDetails, b: AIAgentDetails) => agentName(a).localeCompare(agentName(b));

// AgentsSettings lists a scope's agents (their connection and model, or "external", whether they're on, and what needs
// attention), and adds, edits, turns on and off, and deletes them (deleting asks first, in the page). Once an external
// agent is made, it shows how its program signs in.
export default function AgentsSettings({scope, teamName}: Props) {
    const {formatMessage} = useIntl();
    const [agents, setAgents] = useState<AIAgentDetails[] | null>(null);
    const [loadError, setLoadError] = useState('');
    const [view, setView] = useState<View>({kind: 'list'});
    const [confirming, setConfirming] = useState('');
    const [busyId, setBusyId] = useState('');
    const [status, setStatus] = useState('');
    const [rowError, setRowError] = useState<{id: string; message: string} | null>(null);

    const load = useCallback(async () => {
        try {
            const list = await Client4.getAIAgents(scope);
            setAgents([...list].sort(byName));
            setLoadError('');
        } catch (err) {
            setLoadError(errorMessage(err, formatMessage({id: 'fusion.agents.loadFailed', defaultMessage: 'The agents couldn\'t be loaded.'})));
            setAgents([]);
        }
    }, [scope, formatMessage]);

    useEffect(() => {
        load();
    }, [load]);

    const replace = (agent: AIAgentDetails) => setAgents((list) => [...(list || []).filter((x) => x.user_id !== agent.user_id), agent].sort(byName));

    const setEnabled = async (agent: AIAgentDetails, enabled: boolean) => {
        setBusyId(agent.user_id);
        setRowError(null);
        try {
            const updated = enabled ? await Client4.enableAIAgent(agent.user_id) : await Client4.disableAIAgent(agent.user_id);
            replace(updated);
            setStatus(enabled ? formatMessage({id: 'fusion.agents.enabled', defaultMessage: '{name} is on.'}, {name: agentName(agent)}) : formatMessage({id: 'fusion.agents.disabled', defaultMessage: '{name} is off: nobody can talk to it until it\'s turned on again.'}, {name: agentName(agent)}));
        } catch (err) {
            setRowError({id: agent.user_id, message: errorMessage(err, formatMessage({id: 'fusion.agents.toggleFailed', defaultMessage: 'The agent couldn\'t be changed.'}))});
        } finally {
            setBusyId('');
        }
    };

    const remove = async (agent: AIAgentDetails) => {
        setBusyId(agent.user_id);
        setRowError(null);
        try {
            await Client4.deleteAIAgent(agent.user_id);
            setAgents((list) => (list || []).filter((x) => x.user_id !== agent.user_id));
            setConfirming('');
            setStatus(formatMessage({id: 'fusion.agents.deleted', defaultMessage: '{name} deleted. Its messages stay.'}, {name: agentName(agent)}));
        } catch (err) {
            setRowError({id: agent.user_id, message: errorMessage(err, formatMessage({id: 'fusion.agents.deleteFailed', defaultMessage: 'The agent couldn\'t be deleted.'}))});
        } finally {
            setBusyId('');
        }
    };

    const saved = (agent: AIAgentDetails, created: boolean) => {
        replace(agent);
        if (created && agent.external) {
            setStatus('');
            setView({kind: 'connect', id: agent.user_id});
            return;
        }
        setView({kind: 'list'});
        setStatus(created ? formatMessage({id: 'fusion.agents.created', defaultMessage: '{name} added. Mention it or message it to talk to it.'}, {name: agentName(agent)}) : formatMessage({id: 'fusion.agents.saved', defaultMessage: '{name} saved.'}, {name: agentName(agent)}));
    };

    if (!agents) {
        return <p className={am('lead')}>{formatMessage({id: 'fusion.aiConnections.loading', defaultMessage: 'Loading…'})}</p>;
    }

    const current = view.kind === 'edit' || view.kind === 'connect' ? agents.find((a) => a.user_id === view.id) : undefined;
    const back = () => {
        setView({kind: 'list'});
        setStatus('');
    };

    if (view.kind === 'new' || (view.kind === 'edit' && current)) {
        return (
            <div className={am('ai-connections', 'agents')}>
                <AgentForm
                    key={current?.user_id || 'new'}
                    scope={scope}
                    agent={current}
                    onSaved={saved}
                    onCancel={back}
                />
                {current?.external && (
                    <ExternalAgentHelp
                        agent={current}
                        teamName={teamName}
                    />
                )}
            </div>
        );
    }

    if (view.kind === 'connect' && current) {
        return (
            <div className={am('ai-connections', 'agents')}>
                <p
                    className={am('ai-status')}
                    role='status'
                >
                    {formatMessage({id: 'fusion.agents.createdExternal', defaultMessage: '{name} added. Now connect the program that answers for it.'}, {name: agentName(current)})}
                </p>
                <ExternalAgentHelp
                    agent={current}
                    teamName={teamName}
                />
                <div className={am('modal-actions')}>
                    <button
                        type='button'
                        className={am('btn', 'primary')}
                        onClick={back}
                    >
                        {formatMessage({id: 'fusion.agents.done', defaultMessage: 'Done'})}
                    </button>
                </div>
            </div>
        );
    }

    const runtimeLine = (agent: AIAgentDetails) => {
        if (agent.external) {
            return formatMessage({id: 'fusion.agents.externalLine', defaultMessage: 'External: a program answers for it'});
        }
        const connection = agent.connection_name || formatMessage({id: 'fusion.agents.noConnection', defaultMessage: 'No connection'});
        return agent.model ? `${connection} · ${agent.model}` : connection;
    };

    return (
        <div className={am('ai-connections', 'agents')}>
            <p className={am('lead')}>
                {scope === AI_AGENT_SCOPE_SERVER ? formatMessage({id: 'fusion.agents.lead', defaultMessage: 'Agents are accounts people mention or message like a coworker. A built-in agent answers with a model connection and its instructions, as the person asking, with their rights; an external one is a program elsewhere that signs in with a token.'}) : formatMessage({id: 'fusion.agents.leadCommunity', defaultMessage: 'This community\'s own agents, for its members: accounts they mention or message like a coworker.'})}
            </p>
            {status && (
                <p
                    className={am('ai-status')}
                    role='status'
                >
                    {status}
                </p>
            )}
            {loadError && (
                <p
                    className={am('field-error')}
                    role='alert'
                >
                    {loadError}
                </p>
            )}
            {!loadError && agents.length === 0 && <p className={am('lead')}>{formatMessage({id: 'fusion.agents.none', defaultMessage: 'No agents yet.'})}</p>}
            <ul
                className={am('ai-list')}
                aria-label={formatMessage({id: 'fusion.agents.listLabel', defaultMessage: 'Agents'})}
            >
                {agents.map((agent) => {
                    const name = agentName(agent);
                    return (
                        <li
                            key={agent.user_id}
                            className={am('ai-row', {attention: Boolean(agent.needs_attention), 'agent-off': !agent.enabled})}
                        >
                            <div className={am('ai-row-main')}>
                                <img
                                    className={am('av', 'sm')}
                                    src={Client4.getProfilePictureUrl(agent.user_id, agent.last_icon_update)}
                                    alt=''
                                />
                                <div className={am('grow')}>
                                    <b className={am('agent-row-name')}>
                                        {name}
                                        <IntegrationTag agent={true}/>
                                    </b>
                                    <span>{`@${agent.username} · ${runtimeLine(agent)}`}</span>
                                    {!agent.enabled && <span>{formatMessage({id: 'fusion.agents.off', defaultMessage: 'Off'})}</span>}
                                </div>
                                {agent.needs_attention && (
                                    <span
                                        className={am('ai-badge')}
                                        title={agent.needs_attention}
                                    >
                                        {formatMessage({id: 'fusion.aiConnections.needsAttention', defaultMessage: 'Needs attention'})}
                                    </span>
                                )}
                                {confirming !== agent.user_id && (
                                    <div className={am('ai-row-actions')}>
                                        <button
                                            type='button'
                                            className={am('btn', 'small')}
                                            disabled={busyId === agent.user_id}
                                            onClick={() => setEnabled(agent, !agent.enabled)}
                                            aria-label={agent.enabled ? formatMessage({id: 'fusion.agents.disableOne', defaultMessage: 'Turn off {name}'}, {name}) : formatMessage({id: 'fusion.agents.enableOne', defaultMessage: 'Turn on {name}'}, {name})}
                                        >
                                            {agent.enabled ? formatMessage({id: 'fusion.agents.disable', defaultMessage: 'Turn off'}) : formatMessage({id: 'fusion.agents.enable', defaultMessage: 'Turn on'})}
                                        </button>
                                        <button
                                            type='button'
                                            className={am('icon-btn')}
                                            title={formatMessage({id: 'fusion.aiConnections.edit', defaultMessage: 'Edit'})}
                                            aria-label={formatMessage({id: 'fusion.aiConnections.editOne', defaultMessage: 'Edit {name}'}, {name})}
                                            onClick={() => {
                                                setStatus('');
                                                setView({kind: 'edit', id: agent.user_id});
                                            }}
                                        >
                                            <Icon
                                                name='pen'
                                                size='sm'
                                            />
                                        </button>
                                        <button
                                            type='button'
                                            className={am('icon-btn', 'danger')}
                                            title={formatMessage({id: 'fusion.aiConnections.delete', defaultMessage: 'Delete'})}
                                            aria-label={formatMessage({id: 'fusion.aiConnections.deleteOne', defaultMessage: 'Delete {name}'}, {name})}
                                            onClick={() => {
                                                setConfirming(agent.user_id);
                                                setRowError(null);
                                            }}
                                        >
                                            <Icon
                                                name='trash'
                                                size='sm'
                                            />
                                        </button>
                                    </div>
                                )}
                            </div>
                            {agent.needs_attention && <p className={am('ai-attention')}>{agent.needs_attention}</p>}
                            {confirming === agent.user_id && (
                                <div
                                    className={am('ai-confirm')}
                                    role='group'
                                    aria-label={formatMessage({id: 'fusion.aiConnections.confirmLabel', defaultMessage: 'Delete {name}?'}, {name})}
                                >
                                    <span>{formatMessage({id: 'fusion.agents.confirm', defaultMessage: 'Delete {name}? Its account is deactivated and its settings forgotten; its messages stay. This can\'t be undone.'}, {name})}</span>
                                    <button
                                        type='button'
                                        className={am('btn', 'small')}
                                        onClick={() => setConfirming('')}
                                    >
                                        {formatMessage({id: 'fusion.aiConnections.cancel', defaultMessage: 'Cancel'})}
                                    </button>
                                    <button
                                        type='button'
                                        className={am('btn', 'small', 'danger')}
                                        disabled={busyId === agent.user_id}
                                        onClick={() => remove(agent)}
                                    >
                                        {formatMessage({id: 'fusion.agents.confirmDelete', defaultMessage: 'Delete agent'})}
                                    </button>
                                </div>
                            )}
                            {rowError?.id === agent.user_id && (
                                <p
                                    className={am('field-error')}
                                    role='alert'
                                >
                                    {rowError.message}
                                </p>
                            )}
                        </li>
                    );
                })}
            </ul>
            <div className={am('modal-actions', 'ai-add')}>
                <button
                    type='button'
                    className={am('btn', 'primary')}
                    onClick={() => {
                        setStatus('');
                        setView({kind: 'new'});
                    }}
                >
                    <Icon
                        name='plus'
                        size='sm'
                    />
                    {formatMessage({id: 'fusion.agents.add', defaultMessage: 'Add an agent'})}
                </button>
            </div>
        </div>
    );
}
