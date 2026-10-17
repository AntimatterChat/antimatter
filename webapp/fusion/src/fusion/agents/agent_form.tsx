// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useId, useMemo, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {AIAgentDetails, AIAgentReasoningEffort} from '@mattermost/types/ai_agents';
import type {AIConnectionInfo, AIModelInfo} from '@mattermost/types/ai_connections';

import {getTeams as fetchTeams} from 'mattermost-redux/actions/teams';
import {Client4} from 'mattermost-redux/client';
import {getAllChannels} from 'mattermost-redux/selectors/entities/channels';
import {getTeams} from 'mattermost-redux/selectors/entities/teams';

import {errorMessage} from 'fusion/ai/connections';
import {Choice, Toggle} from 'fusion/modals/settings_controls';
import {am} from 'fusion/utils/class_names';

import AgentAvatar from './agent_avatar';
import {
    AI_AGENT_SCOPE_SERVER,
    COMMUNITY_ROLES,
    REASONING_EFFORTS,
    SERVER_ROLES,
    draftFromAgent,
    newAgentDraft,
    patchOf,
    problemsOf,
    requestOf,
} from './agents';
import type {AgentDraft, AgentKind} from './agents';

type Props = {

    // The scope of a new agent ("server" or a team ID); an edited one keeps its own.
    scope: string;
    agent?: AIAgentDetails;
    onSaved: (agent: AIAgentDetails, created: boolean) => void;
    onCancel: () => void;
};

const CHANNELS_SHOWN = 40;

// AgentForm makes or edits an agent: its account (username, name, description, picture), for a built-in agent its
// model connection, model and system prompt, who may talk to it, and in More its reasoning and tool turns. An external
// agent has only its account and access: the program behind it signs in with a token.
export default function AgentForm({scope: newScope, agent, onSaved, onCancel}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const scope = agent?.scope || newScope;
    const serverWide = scope === AI_AGENT_SCOPE_SERVER;
    const [draft, setDraft] = useState<AgentDraft>(() => (agent ? draftFromAgent(agent) : newAgentDraft()));
    const [connections, setConnections] = useState<AIConnectionInfo[] | null>(null);
    const [models, setModels] = useState<AIModelInfo[]>([]);
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');
    const [more, setMore] = useState(false);
    const [channelFilter, setChannelFilter] = useState('');
    const modelListId = useId();
    const teams = useSelector(getTeams);
    const allChannels = useSelector(getAllChannels);

    const set = (patch: Partial<AgentDraft>) => {
        setDraft((d) => ({...d, ...patch}));
        setError('');
    };
    const toggleIn = (list: string[], id: string) => (list.includes(id) ? list.filter((x) => x !== id) : [...list, id]);

    useEffect(() => {
        if (serverWide) {
            dispatch(fetchTeams(0, 200));
        }
    }, [dispatch, serverWide]);

    // The connections an agent of this scope may use.
    useEffect(() => {
        let cancelled = false;
        Client4.getAIAgentConnections(scope).then((list) => {
            if (!cancelled) {
                setConnections([...list].sort((a, b) => a.name.localeCompare(b.name)));
            }
        }).catch((err) => {
            if (!cancelled) {
                setConnections([]);
                setError(errorMessage(err, formatMessage({id: 'fusion.agents.connectionsFailed', defaultMessage: 'The model connections couldn\'t be loaded.'})));
            }
        });
        return () => {
            cancelled = true;
        };
    }, [scope, formatMessage]);

    // The models of the connection picked, to choose from (any other can be typed).
    useEffect(() => {
        setModels([]);
        if (!draft.connectionId || draft.kind !== 'builtin') {
            return undefined;
        }
        let cancelled = false;
        Client4.getAIAgentConnectionModels(draft.connectionId, scope).then((list) => {
            if (!cancelled) {
                setModels(list);
            }
        }).catch(() => {});
        return () => {
            cancelled = true;
        };
    }, [draft.connectionId, draft.kind, scope]);

    const connection = connections?.find((c) => c.id === draft.connectionId);
    const communities = useMemo(() => Object.values(teams).filter((t) => t.delete_at === 0).sort((a, b) => a.display_name.localeCompare(b.display_name)), [teams]);
    const channels = useMemo(() => {
        const filter = channelFilter.trim().toLowerCase();
        return Object.values(allChannels).
            filter((c) => (c.type === 'O' || c.type === 'P') && c.delete_at === 0 && (serverWide || c.team_id === scope)).
            filter((c) => draft.channelIds.includes(c.id) || !filter || c.display_name.toLowerCase().includes(filter) || c.name.includes(filter)).
            sort((a, b) => Number(draft.channelIds.includes(b.id)) - Number(draft.channelIds.includes(a.id)) || a.display_name.localeCompare(b.display_name)).
            slice(0, CHANNELS_SHOWN);
    }, [allChannels, channelFilter, serverWide, scope, draft.channelIds]);

    const problems = problemsOf(draft, Boolean(agent));
    const external = draft.kind === 'external';

    const roleLabel = (role: string) => ({
        system_admin: formatMessage({id: 'fusion.aiConnections.role.systemAdmin', defaultMessage: 'System admins'}),
        team_admin: formatMessage({id: 'fusion.aiConnections.role.teamAdmin', defaultMessage: 'Community admins'}),
    })[role] || role;

    const effortLabel = (effort: AIAgentReasoningEffort) => ({
        '': formatMessage({id: 'fusion.agents.reasoning.off', defaultMessage: 'Off'}),
        minimal: formatMessage({id: 'fusion.agents.reasoning.minimal', defaultMessage: 'Minimal'}),
        low: formatMessage({id: 'fusion.agents.reasoning.low', defaultMessage: 'Low'}),
        medium: formatMessage({id: 'fusion.agents.reasoning.medium', defaultMessage: 'Medium'}),
        high: formatMessage({id: 'fusion.agents.reasoning.high', defaultMessage: 'High'}),
    })[effort];

    const save = async () => {
        if (problems.length) {
            return;
        }
        setBusy(true);
        setError('');
        try {
            const saved = agent ? await Client4.patchAIAgent(agent.user_id, patchOf(draft)) : await Client4.createAIAgent(requestOf(draft, scope));
            onSaved(saved, !agent);
        } catch (err) {
            setError(errorMessage(err, formatMessage({id: 'fusion.agents.saveFailed', defaultMessage: 'The agent couldn\'t be saved.'})));
        } finally {
            setBusy(false);
        }
    };

    return (
        <form
            className={am('ai-form', 'agent-form')}
            aria-label={agent ? formatMessage({id: 'fusion.agents.editLabel', defaultMessage: 'Edit {name}'}, {name: agent.display_name || agent.username}) : formatMessage({id: 'fusion.agents.addLabel', defaultMessage: 'Add an agent'})}
            onSubmit={(e) => {
                e.preventDefault();
                save();
            }}
        >
            {agent?.needs_attention && (
                <p
                    className={am('ai-attention')}
                    role='note'
                >
                    {formatMessage({id: 'fusion.agents.attentionForm', defaultMessage: 'Needs attention: {reason} Saving clears this.'}, {reason: agent.needs_attention})}
                </p>
            )}
            {agent ? (
                <p className={am('lead')}>
                    {external ? formatMessage({id: 'fusion.agents.kindExternalNote', defaultMessage: 'External agent: a program elsewhere answers for it.'}) : formatMessage({id: 'fusion.agents.kindBuiltinNote', defaultMessage: 'Built-in agent: Antimatter answers for it with the model below.'})}
                </p>
            ) : (
                <Choice<AgentKind>
                    title={formatMessage({id: 'fusion.agents.kind', defaultMessage: 'Kind'})}
                    desc={external ? formatMessage({id: 'fusion.agents.kindExternalDesc', defaultMessage: 'A program elsewhere (a coding agent on a project, for instance) signs in with the agent\'s token and answers like a person would. Antimatter doesn\'t answer for it.'}) : formatMessage({id: 'fusion.agents.kindBuiltinDesc', defaultMessage: 'Antimatter answers for the agent when someone mentions it, messages it or replies in its threads, with a model connection.'})}
                    value={draft.kind}
                    options={[
                        ['builtin', formatMessage({id: 'fusion.agents.kindBuiltin', defaultMessage: 'Built-in'})],
                        ['external', formatMessage({id: 'fusion.agents.kindExternal', defaultMessage: 'External agent'})],
                    ]}
                    onChange={(kind) => set({kind})}
                />
            )}

            {agent && <AgentAvatar agent={agent}/>}

            <div className={am('field-row')}>
                <label className={am('field')}>
                    <span>{formatMessage({id: 'fusion.agents.username', defaultMessage: 'Username'})}</span>
                    <input
                        type='text'
                        value={draft.username}
                        maxLength={22}
                        disabled={Boolean(agent)}
                        spellCheck={false}
                        autoComplete='off'
                        placeholder={formatMessage({id: 'fusion.agents.usernamePlaceholder', defaultMessage: 'e.g. researcher'})}
                        aria-invalid={problems.includes('username')}
                        onChange={(e) => set({username: e.target.value})}
                    />
                    {!agent && (
                        <small className={am('field-note', {bad: problems.includes('username') && draft.username !== ''})}>
                            {formatMessage({id: 'fusion.agents.usernameDesc', defaultMessage: 'How people mention it: 3 to 22 lower-case letters, digits, dots, dashes or underscores, starting with a letter.'})}
                        </small>
                    )}
                </label>
                <label className={am('field')}>
                    <span>{formatMessage({id: 'fusion.agents.displayName', defaultMessage: 'Display name'})}</span>
                    <input
                        type='text'
                        value={draft.displayName}
                        maxLength={64}
                        placeholder={formatMessage({id: 'fusion.agents.displayNamePlaceholder', defaultMessage: 'e.g. Researcher'})}
                        onChange={(e) => set({displayName: e.target.value})}
                    />
                </label>
            </div>
            <label className={am('field')}>
                <span>{formatMessage({id: 'fusion.agents.description', defaultMessage: 'Description'})}</span>
                <input
                    type='text'
                    value={draft.description}
                    maxLength={1024}
                    placeholder={formatMessage({id: 'fusion.agents.descriptionPlaceholder', defaultMessage: 'What it\'s for, shown on its profile'})}
                    onChange={(e) => set({description: e.target.value})}
                />
            </label>

            {!external && (
                <>
                    <h3 className={am('set-section')}>{formatMessage({id: 'fusion.agents.model', defaultMessage: 'Model'})}</h3>
                    <label className={am('field')}>
                        <span>{formatMessage({id: 'fusion.agents.connection', defaultMessage: 'Model connection'})}</span>
                        <select
                            value={draft.connectionId}
                            aria-invalid={problems.includes('connection')}
                            onChange={(e) => set({connectionId: e.target.value, model: ''})}
                        >
                            <option value=''>{connections ? formatMessage({id: 'fusion.agents.connectionPick', defaultMessage: 'Pick a connection'}) : formatMessage({id: 'fusion.aiConnections.loading', defaultMessage: 'Loading…'})}</option>
                            {(connections || []).map((c) => (
                                <option
                                    key={c.id}
                                    value={c.id}
                                >
                                    {c.name}
                                </option>
                            ))}
                        </select>
                        {connections?.length === 0 && (
                            <small className={am('field-note', 'bad')}>
                                {serverWide ? formatMessage({id: 'fusion.agents.noConnections', defaultMessage: 'No model connection yet: add one in the System Console, AI › Model connections.'}) : formatMessage({id: 'fusion.agents.noConnectionsCommunity', defaultMessage: 'No model connection yet: add one in Model connections, or ask a system admin to share one with this community.'})}
                            </small>
                        )}
                    </label>
                    <label className={am('field')}>
                        <span>{formatMessage({id: 'fusion.agents.modelName', defaultMessage: 'Model'})}</span>
                        <input
                            type='text'
                            className={am('mono')}
                            value={draft.model}
                            maxLength={256}
                            spellCheck={false}
                            autoComplete='off'
                            list={modelListId}
                            disabled={!draft.connectionId}
                            placeholder={connection?.models.chat || ''}
                            onChange={(e) => set({model: e.target.value})}
                        />
                        <datalist id={modelListId}>
                            {models.map((m) => (
                                <option
                                    key={m.id}
                                    value={m.id}
                                />
                            ))}
                        </datalist>
                        <small className={am('field-note')}>
                            {connection?.models.chat ? formatMessage({id: 'fusion.agents.modelDesc', defaultMessage: 'Left empty: the connection\'s chat model, {model}.'}, {model: connection.models.chat}) : formatMessage({id: 'fusion.agents.modelDescNone', defaultMessage: 'Pick one of the connection\'s models, or type its name.'})}
                        </small>
                    </label>
                    <label className={am('field')}>
                        <span>{formatMessage({id: 'fusion.agents.systemPrompt', defaultMessage: 'Instructions'})}</span>
                        <textarea
                            className={am('agent-prompt')}
                            value={draft.systemPrompt}
                            maxLength={32000}
                            placeholder={formatMessage({id: 'fusion.agents.systemPromptPlaceholder', defaultMessage: 'Who the agent is and how it works, e.g. "You research questions and answer with sources."'})}
                            onChange={(e) => set({systemPrompt: e.target.value})}
                        />
                        <small className={am('field-note')}>{formatMessage({id: 'fusion.agents.systemPromptDesc', defaultMessage: 'Its system prompt. People who talk to it don\'t see it.'})}</small>
                    </label>
                    <div className={am('field')}>
                        <span>{formatMessage({id: 'fusion.agents.tools', defaultMessage: 'Tools'})}</span>
                        <small className={am('field-note')}>{formatMessage({id: 'fusion.agents.toolsSoon', defaultMessage: 'Coming soon: the tools of Antimatter and its apps the agent may use, always with the rights of the person asking.'})}</small>
                    </div>
                </>
            )}

            <h3 className={am('set-section')}>{formatMessage({id: 'fusion.agents.access', defaultMessage: 'Who may talk to it'})}</h3>
            <Toggle
                title={formatMessage({id: 'fusion.agents.everyone', defaultMessage: 'Everyone'})}
                desc={serverWide ? formatMessage({id: 'fusion.agents.everyoneDesc', defaultMessage: 'Everyone on the server. Otherwise only the communities, channels and admins chosen.'}) : formatMessage({id: 'fusion.agents.everyoneCommunityDesc', defaultMessage: 'Everyone in the community. Otherwise only the channels and admins chosen.'})}
                on={draft.everyone}
                onChange={(everyone) => set({everyone})}
            />
            {!draft.everyone && (
                <>
                    <div
                        className={am('chip-picks', 'ai-picks')}
                        role='group'
                        aria-label={formatMessage({id: 'fusion.agents.rolesChosen', defaultMessage: 'Roles chosen'})}
                    >
                        {(serverWide ? SERVER_ROLES : COMMUNITY_ROLES).map((role) => (
                            <button
                                key={role}
                                type='button'
                                className={am('chip-pick', {on: draft.roles.includes(role)})}
                                aria-pressed={draft.roles.includes(role)}
                                onClick={() => set({roles: toggleIn(draft.roles, role)})}
                            >
                                {roleLabel(role)}
                            </button>
                        ))}
                    </div>
                    {serverWide && (
                        <div
                            className={am('chip-picks', 'ai-picks')}
                            role='group'
                            aria-label={formatMessage({id: 'fusion.agents.teamsChosen', defaultMessage: 'Communities chosen'})}
                        >
                            {communities.map((t) => (
                                <button
                                    key={t.id}
                                    type='button'
                                    className={am('chip-pick', {on: draft.teamIds.includes(t.id)})}
                                    aria-pressed={draft.teamIds.includes(t.id)}
                                    onClick={() => set({teamIds: toggleIn(draft.teamIds, t.id)})}
                                >
                                    {t.display_name}
                                </button>
                            ))}
                        </div>
                    )}
                    <label className={am('field')}>
                        <span>{formatMessage({id: 'fusion.agents.channels', defaultMessage: 'Channels'})}</span>
                        <input
                            type='search'
                            value={channelFilter}
                            placeholder={formatMessage({id: 'fusion.agents.channelsFilter', defaultMessage: 'Find a channel'})}
                            onChange={(e) => setChannelFilter(e.target.value)}
                        />
                    </label>
                    <div
                        className={am('chip-picks', 'ai-picks')}
                        role='group'
                        aria-label={formatMessage({id: 'fusion.agents.channelsChosen', defaultMessage: 'Channels chosen'})}
                    >
                        {channels.map((c) => (
                            <button
                                key={c.id}
                                type='button'
                                className={am('chip-pick', {on: draft.channelIds.includes(c.id)})}
                                aria-pressed={draft.channelIds.includes(c.id)}
                                onClick={() => set({channelIds: toggleIn(draft.channelIds, c.id)})}
                            >
                                {serverWide && teams[c.team_id] ? `${teams[c.team_id].display_name} · ${c.display_name}` : c.display_name}
                            </button>
                        ))}
                    </div>
                    {problems.includes('access') && <small className={am('field-note', 'bad')}>{formatMessage({id: 'fusion.agents.accessNone', defaultMessage: 'Choose who may talk to it, or turn on Everyone.'})}</small>}
                </>
            )}
            <Toggle
                title={formatMessage({id: 'fusion.agents.guests', defaultMessage: 'Guests too'})}
                desc={formatMessage({id: 'fusion.agents.guestsDesc', defaultMessage: 'Guests may talk to it only when this is on, even with Everyone.'})}
                on={draft.allowGuests}
                onChange={(allowGuests) => set({allowGuests})}
            />

            {!external && (
                <>
                    <button
                        type='button'
                        className={am('btn', 'small', 'ghost')}
                        aria-expanded={more}
                        onClick={() => setMore(!more)}
                    >
                        {formatMessage({id: 'fusion.agents.more', defaultMessage: 'More settings'})}
                    </button>
                    {more && (
                        <>
                            <Choice<AIAgentReasoningEffort>
                                title={formatMessage({id: 'fusion.agents.reasoning', defaultMessage: 'Reasoning'})}
                                desc={formatMessage({id: 'fusion.agents.reasoningDesc', defaultMessage: 'For models that can think before they answer: slower, often better. Others ignore it.'})}
                                value={draft.reasoningEffort}
                                options={REASONING_EFFORTS.map((effort) => [effort, effortLabel(effort)] as [AIAgentReasoningEffort, string])}
                                onChange={(reasoningEffort) => set({reasoningEffort})}
                            />
                            <label className={am('field')}>
                                <span>{formatMessage({id: 'fusion.agents.maxToolTurns', defaultMessage: 'Tool turns'})}</span>
                                <input
                                    type='text'
                                    inputMode='numeric'
                                    value={draft.maxToolTurns}
                                    placeholder='30'
                                    aria-invalid={problems.includes('maxToolTurns')}
                                    onChange={(e) => set({maxToolTurns: e.target.value})}
                                />
                                <small className={am('field-note', {bad: problems.includes('maxToolTurns')})}>{formatMessage({id: 'fusion.agents.maxToolTurnsDesc', defaultMessage: 'How many times in a row it may use tools before it answers: up to 250; empty for 30.'})}</small>
                            </label>
                        </>
                    )}
                </>
            )}

            {error && (
                <p
                    className={am('field-error')}
                    role='alert'
                >
                    {error}
                </p>
            )}
            <div className={am('modal-actions')}>
                <button
                    type='button'
                    className={am('btn')}
                    onClick={onCancel}
                >
                    {formatMessage({id: 'fusion.aiConnections.cancel', defaultMessage: 'Cancel'})}
                </button>
                <button
                    type='submit'
                    className={am('btn', 'primary')}
                    disabled={busy || problems.length > 0}
                >
                    {agent ? formatMessage({id: 'fusion.agents.save', defaultMessage: 'Save'}) : formatMessage({id: 'fusion.agents.create', defaultMessage: 'Add agent'})}
                </button>
            </div>
        </form>
    );
}
