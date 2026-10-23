// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import {useIntl} from 'react-intl';

import type {AIAgentToolPolicy} from '@mattermost/types/ai_agents';
import type {AIToolCatalogueSource} from '@mattermost/types/ai_tools';

import {Client4} from 'mattermost-redux/client';

import {Choice, Toggle} from 'fusion/modals/settings_controls';
import {am} from 'fusion/utils/class_names';

import {autoRunsReads, sourceMode, unknownNames, withAutoRunReads, withSourceMode, withTool} from './tool_policy';
import type {SourceMode} from './tool_policy';

type Props = {

    // The agent's scope: what it may be given ("server", or a community's ID).
    scope: string;
    policy: AIAgentToolPolicy;
    onChange: (policy: AIAgentToolPolicy) => void;
};

// ToolPolicyField edits which tools an agent may use (Antimatter's own, the running apps', the MCP servers available
// to its scope): per source, none, all, or chosen tools; and whether a source's reads run without asking. Anything
// else always waits for the person's confirmation, and every tool acts with the rights of the person asking.
export default function ToolPolicyField({scope, policy, onChange}: Props) {
    const {formatMessage} = useIntl();
    const [sources, setSources] = useState<AIToolCatalogueSource[] | null>(null);
    const [failed, setFailed] = useState(false);

    // Sources switched to "chosen" before any of their tools is.
    const [picking, setPicking] = useState<string[]>([]);

    useEffect(() => {
        let cancelled = false;
        Client4.getAIToolCatalogue(scope).then((catalogue) => {
            if (!cancelled) {
                setSources(catalogue.sources);
            }
        }).catch(() => {
            if (!cancelled) {
                setFailed(true);
            }
        });
        return () => {
            cancelled = true;
        };
    }, [scope]);

    const authNote = (source: AIToolCatalogueSource) => {
        switch (source.auth) {
        case 'oauth':
            return formatMessage({id: 'fusion.agents.toolsAuthOAuth', defaultMessage: 'each person signs in'});
        case 'service_account':
            return formatMessage({id: 'fusion.agents.toolsAuthService', defaultMessage: 'a shared account: reads only'});
        case 'none':
            return formatMessage({id: 'fusion.agents.toolsAuthNone', defaultMessage: 'no sign-in'});
        default:
            return '';
        }
    };

    const unknown = sources ? unknownNames(policy, sources) : [];

    return (
        <div
            className={am('field', 'agent-tools')}
            data-testid='agent-tools'
        >
            <Toggle
                title={formatMessage({id: 'fusion.agents.tools', defaultMessage: 'Tools'})}
                desc={formatMessage({id: 'fusion.agents.toolsDesc', defaultMessage: 'What the agent may do besides answering, always with the rights of the person asking. A tool that changes something asks them first.'})}
                on={policy.enabled}
                onChange={(enabled) => onChange({...policy, enabled})}
            />
            {policy.enabled && failed && <small className={am('field-note', 'bad')}>{formatMessage({id: 'fusion.agents.toolsLoadFailed', defaultMessage: 'The tools couldn\'t be loaded.'})}</small>}
            {policy.enabled && sources?.map((source) => {
                const saved = sourceMode(policy, source);
                const mode: SourceMode = saved === 'off' && picking.includes(source.namespace) ? 'some' : saved;
                const setMode = (next: SourceMode) => {
                    setPicking((list) => (next === 'some' ? [...list, source.namespace] : list.filter((x) => x !== source.namespace)));
                    onChange(withSourceMode(policy, sources, source, next));
                };
                const note = authNote(source);
                return (
                    <div
                        key={source.key}
                        className={am('agent-tool-source')}
                    >
                        <Choice
                            title={source.name}
                            desc={note ? `${source.namespace}.* · ${note}` : `${source.namespace}.*`}
                            value={mode}
                            options={[
                                ['off', formatMessage({id: 'fusion.agents.toolsOff', defaultMessage: 'Off'})],
                                ['all', formatMessage({id: 'fusion.agents.toolsAll', defaultMessage: 'All'})],
                                ['some', formatMessage({id: 'fusion.agents.toolsSome', defaultMessage: 'Chosen'})],
                            ]}
                            onChange={setMode}
                        />
                        {mode === 'some' && source.tools.length > 0 && (
                            <div
                                className={am('chip-picks')}
                                role='group'
                                aria-label={formatMessage({id: 'fusion.agents.toolsChosen', defaultMessage: '{name}: tools chosen'}, {name: source.name})}
                            >
                                {source.tools.map((tool) => {
                                    const on = policy.allowed.includes(tool.name);
                                    return (
                                        <button
                                            key={tool.name}
                                            type='button'
                                            className={am('chip-pick', {on})}
                                            aria-pressed={on}
                                            title={tool.description || tool.name}
                                            onClick={() => onChange(withTool(policy, tool.name, !on))}
                                        >
                                            {tool.title || tool.name}
                                        </button>
                                    );
                                })}
                            </div>
                        )}
                        {mode === 'some' && source.tools.length === 0 && <small className={am('field-note')}>{formatMessage({id: 'fusion.agents.toolsUnlisted', defaultMessage: 'Its tools are listed here once you sign in to it yourself, from the AI chat\'s Tools menu. Meanwhile, choose All.'})}</small>}
                        {mode !== 'off' && (
                            <Toggle
                                title={formatMessage({id: 'fusion.agents.toolsAutoRun', defaultMessage: 'Its reads run without asking'})}
                                desc={formatMessage({id: 'fusion.agents.toolsAutoRunDesc', defaultMessage: 'Tools that only look something up run on their own. Any other tool always waits for the person to confirm.'})}
                                on={autoRunsReads(policy, source)}
                                onChange={(on) => onChange(withAutoRunReads(policy, sources, source, on))}
                            />
                        )}
                    </div>
                );
            })}
            {policy.enabled && unknown.length > 0 && (
                <small className={am('field-note')}>
                    {formatMessage({id: 'fusion.agents.toolsUnknown', defaultMessage: 'Also allowed, not available now: {names}'}, {names: unknown.join(', ')})}
                </small>
            )}
        </div>
    );
}
