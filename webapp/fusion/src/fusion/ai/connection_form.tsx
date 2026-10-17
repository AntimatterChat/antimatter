// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import React, {useEffect, useId, useMemo, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {AIConnection, AIConnectionLimits, AIConnectionTestResult, AIModelCapability, AIModelInfo} from '@mattermost/types/ai_connections';
import type {Group} from '@mattermost/types/groups';

import {getTeams as fetchTeams} from 'mattermost-redux/actions/teams';
import {Client4} from 'mattermost-redux/client';
import {getTeams} from 'mattermost-redux/selectors/entities/teams';

import Icon from 'fusion/components/icon';
import {Choice, Toggle} from 'fusion/modals/settings_controls';
import {getRolesByTeam} from 'fusion/teams/roles';
import {getTeamsByCommunity, loadTeams} from 'fusion/teams/teams';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import {
    AI_CONNECTION_SCOPE_SERVER,
    DEFAULT_BASE_URLS,
    capabilitiesOf,
    draftFromConnection,
    endpointChanged,
    errorMessage,
    newDraft,
    newHeaderRow,
    patchOf,
    presetsFor,
    problemsOf,
    requestOf,
    testRequestNeedsSavedSecrets,
} from './connections';
import type {Draft, HeaderRow} from './connections';

type Props = {

    // The scope of a new connection ("server" or a team ID); an edited one keeps its own.
    scope: string;
    connection?: AIConnection;
    onSaved: (connection: AIConnection, created: boolean) => void;
    onCancel: () => void;
};

// The built-in roles a connection's audience can name, besides teams and roles: the server's and communities' admins.
const SERVER_ROLES = ['system_admin', 'team_admin'];
const COMMUNITY_ROLES = ['team_admin'];

const LIMIT_KEYS: Array<keyof AIConnectionLimits> = ['requests_per_minute', 'monthly_token_budget', 'per_user_requests_per_minute', 'per_user_monthly_token_budget'];

// ConnectionForm adds or edits a model connection: its provider and address, its key (write-only: a saved one shows as
// "set, ends in ••wxyz", to replace or remove), extra headers, the default model of each capability, limits, and who
// may use it. It tests the connection before it's saved, or as saved.
export default function ConnectionForm({scope: newScope, connection, onSaved, onCancel}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const scope = connection?.scope || newScope;
    const serverWide = scope === AI_CONNECTION_SCOPE_SERVER;
    const [draft, setDraft] = useState<Draft>(() => (connection ? draftFromConnection(connection) : newDraft(scope)));
    const [busy, setBusy] = useState<'' | 'save' | 'test' | 'models'>('');
    const [error, setError] = useState('');
    const [test, setTest] = useState<AIConnectionTestResult | null>(null);
    const [testNote, setTestNote] = useState('');
    const [models, setModels] = useState<AIModelInfo[] | null>(null);
    const [modelsNote, setModelsNote] = useState('');
    const modelListId = useId();

    const teams = useSelector(getTeams);
    const teamsByCommunity = useSelector(getTeamsByCommunity);
    const rolesByTeam = useSelector(getRolesByTeam);

    // The communities to share with, and the teams and roles an audience can name.
    useEffect(() => {
        dispatch(loadTeams());
        if (serverWide) {
            dispatch(fetchTeams(0, 200));
        }
    }, [dispatch, serverWide]);

    const groups = useMemo(() => {
        const out: Array<{group: Group; community: string}> = [];
        const communities = serverWide ? Object.keys({...teamsByCommunity, ...rolesByTeam}) : [scope];
        for (const id of communities) {
            const community = teams[id]?.display_name || '';
            for (const group of [...(teamsByCommunity[id] || []), ...(rolesByTeam[id] || [])]) {
                out.push({group, community});
            }
        }
        return out.sort((a, b) => a.community.localeCompare(b.community) || a.group.display_name.localeCompare(b.group.display_name));
    }, [serverWide, scope, teams, teamsByCommunity, rolesByTeam]);

    const communities = useMemo(() => Object.values(teams).filter((t) => t.delete_at === 0).sort((a, b) => a.display_name.localeCompare(b.display_name)), [teams]);

    const set = (patch: Partial<Draft>) => {
        setDraft((d) => ({...d, ...patch}));
        setError('');
    };
    const setHeader = (row: HeaderRow, patch: Partial<HeaderRow>) => set({headers: draft.headers.map((h) => (h.key === row.key ? {...h, ...patch} : h))});
    const toggleIn = (list: string[], id: string) => (list.includes(id) ? list.filter((x) => x !== id) : [...list, id]);

    const problems = problemsOf(draft);
    const capabilities = capabilitiesOf(draft.provider);

    const capabilityLabel = (cap: AIModelCapability) => ({
        chat: formatMessage({id: 'fusion.aiConnections.model.chat', defaultMessage: 'Chat'}),
        embeddings: formatMessage({id: 'fusion.aiConnections.model.embeddings', defaultMessage: 'Embeddings'}),
        transcription: formatMessage({id: 'fusion.aiConnections.model.transcription', defaultMessage: 'Transcription'}),
        decisions: formatMessage({id: 'fusion.aiConnections.model.decisions', defaultMessage: 'Decisions'}),
    })[cap];

    const roleLabel = (role: string) => ({
        system_admin: formatMessage({id: 'fusion.aiConnections.role.systemAdmin', defaultMessage: 'System admins'}),
        team_admin: formatMessage({id: 'fusion.aiConnections.role.teamAdmin', defaultMessage: 'Community admins'}),
    })[role] || role;

    const save = async () => {
        if (problems.length) {
            return;
        }
        setBusy('save');
        setError('');
        try {
            const saved = connection ? await Client4.patchAIConnection(connection.id, patchOf(draft, scope)) : await Client4.createAIConnection(requestOf(draft, scope));
            onSaved(saved, !connection);
        } catch (err) {
            setError(errorMessage(err, formatMessage({id: 'fusion.aiConnections.saveFailed', defaultMessage: 'The connection couldn\'t be saved.'})));
        } finally {
            setBusy('');
        }
    };

    // A saved connection is tested as saved, with its own key, while its address and key are unchanged; a new or
    // changed one with what the form holds.
    const runTest = async () => {
        setTest(null);
        setTestNote('');
        setError('');
        if (connection && endpointChanged(draft, connection) && testRequestNeedsSavedSecrets(draft, connection)) {
            setTestNote(formatMessage({id: 'fusion.aiConnections.test.saveFirst', defaultMessage: 'The saved key and headers stay on the server: to test the new address with them, save first, or type the key again.'}));
            return;
        }
        setBusy('test');
        try {
            let result;
            if (connection && !endpointChanged(draft, connection)) {
                result = await Client4.testAIConnection(connection.id);
            } else {
                // The server checks the request as it would a new connection, name included: a draft without one
                // yet is tested under a placeholder.
                const request = requestOf(draft, scope);
                result = await Client4.testAIConnectionRequest({...request, name: request.name || 'test'});
            }
            setTest(result);
        } catch (err) {
            setError(errorMessage(err, formatMessage({id: 'fusion.aiConnections.test.failedToRun', defaultMessage: 'The test couldn\'t run.'})));
        } finally {
            setBusy('');
        }
    };

    // A saved connection's models come from the server with its own key while its address and key are unchanged; a
    // new or changed one's with what the form holds, as its test does.
    const loadModels = async () => {
        setModelsNote('');
        setError('');
        if (connection && endpointChanged(draft, connection) && testRequestNeedsSavedSecrets(draft, connection)) {
            setModelsNote(formatMessage({id: 'fusion.aiConnections.models.saveFirst', defaultMessage: 'The saved key and headers stay on the server: to load the models of the new address with them, save first, or type the key again.'}));
            return;
        }
        setBusy('models');
        try {
            if (connection && !endpointChanged(draft, connection)) {
                setModels(await Client4.getAIConnectionModels(connection.id));
            } else {
                const request = requestOf(draft, scope);
                setModels(await Client4.getAIConnectionRequestModels({...request, name: request.name || 'models'}));
            }
        } catch (err) {
            setError(errorMessage(err, formatMessage({id: 'fusion.aiConnections.models.failed', defaultMessage: 'The models couldn\'t be loaded.'})));
        } finally {
            setBusy('');
        }
    };

    const keySet = Boolean(connection?.api_key_set);
    const last4 = connection?.api_key_last4;

    return (
        <form
            className={am('ai-form')}
            aria-label={connection ? formatMessage({id: 'fusion.aiConnections.editLabel', defaultMessage: 'Edit {name}'}, {name: connection.name}) : formatMessage({id: 'fusion.aiConnections.addLabel', defaultMessage: 'Add a model connection'})}
            onSubmit={(e) => {
                e.preventDefault();
                save();
            }}
        >
            {connection?.needs_attention && (
                <p
                    className={am('ai-attention')}
                    role='note'
                >
                    {formatMessage({id: 'fusion.aiConnections.attentionForm', defaultMessage: 'Needs attention: {reason} Saving clears this.'}, {reason: connection.needs_attention})}
                </p>
            )}
            <label className={am('field')}>
                <span>{formatMessage({id: 'fusion.aiConnections.name', defaultMessage: 'Name'})}</span>
                <input
                    type='text'
                    value={draft.name}
                    maxLength={64}
                    placeholder={formatMessage({id: 'fusion.aiConnections.namePlaceholder', defaultMessage: 'e.g. Company OpenAI'})}
                    onChange={(e) => set({name: e.target.value})}
                />
            </label>

            <Choice
                title={formatMessage({id: 'fusion.aiConnections.provider', defaultMessage: 'Provider'})}
                desc={formatMessage({id: 'fusion.aiConnections.providerDesc', defaultMessage: 'Most providers and gateways speak the OpenAI API (vLLM, Ollama, LiteLLM, OpenRouter, Azure OpenAI…).'})}
                value={draft.provider}
                options={[
                    ['openai_compatible', formatMessage({id: 'fusion.aiConnections.provider.openai', defaultMessage: 'OpenAI-compatible'})],
                    ['anthropic', formatMessage({id: 'fusion.aiConnections.provider.anthropic', defaultMessage: 'Anthropic'})],
                ]}
                onChange={(provider) => set({provider})}
            />
            <div className={am('field')}>
                <span>{formatMessage({id: 'fusion.aiConnections.presets', defaultMessage: 'Presets'})}</span>
                <div className={am('chip-picks')}>
                    {presetsFor(scope).map((p) => (
                        <button
                            key={p.id}
                            type='button'
                            className={am('chip-pick', {on: draft.provider === p.provider && draft.baseURL === p.baseURL})}
                            onClick={() => set({provider: p.provider, baseURL: p.baseURL, name: draft.name || p.name})}
                        >
                            {p.local ? formatMessage({id: 'fusion.aiConnections.presetLocal', defaultMessage: '{name} on this server'}, {name: p.name}) : p.name}
                        </button>
                    ))}
                </div>
            </div>
            <label className={am('field')}>
                <span>{formatMessage({id: 'fusion.aiConnections.baseURL', defaultMessage: 'Base URL'})}</span>
                <input
                    type='text'
                    inputMode='url'
                    className={am('mono')}
                    value={draft.baseURL}
                    maxLength={1024}
                    spellCheck={false}
                    autoComplete='off'
                    placeholder={DEFAULT_BASE_URLS[draft.provider]}
                    aria-invalid={problems.includes('baseURL')}
                    onChange={(e) => set({baseURL: e.target.value})}
                />
                <small className={am('field-note', {bad: problems.includes('baseURL')})}>
                    {problems.includes('baseURL') ? formatMessage({id: 'fusion.aiConnections.baseURLBad', defaultMessage: 'An http or https address, without a query or a password in it.'}) : formatMessage({id: 'fusion.aiConnections.baseURLDesc', defaultMessage: 'Up to and including /v1. Left empty: {url}.'}, {url: DEFAULT_BASE_URLS[draft.provider]})}
                </small>
            </label>

            <div className={am('field')}>
                <span>{formatMessage({id: 'fusion.aiConnections.apiKey', defaultMessage: 'API key'})}</span>
                {connection && draft.keyAction === 'keep' && (
                    <div className={am('ai-secret')}>
                        <span>
                            {keySet ? (last4 ? formatMessage({id: 'fusion.aiConnections.keySetLast4', defaultMessage: 'Set, ends in ••{last4}'}, {last4}) : formatMessage({id: 'fusion.aiConnections.keySet', defaultMessage: 'Set'})) : formatMessage({id: 'fusion.aiConnections.keyNone', defaultMessage: 'No key'})}
                        </span>
                        <button
                            type='button'
                            className={am('btn', 'small')}
                            onClick={() => set({keyAction: 'replace', apiKey: ''})}
                        >
                            {keySet ? formatMessage({id: 'fusion.aiConnections.keyReplace', defaultMessage: 'Replace'}) : formatMessage({id: 'fusion.aiConnections.keyAdd', defaultMessage: 'Add a key'})}
                        </button>
                        {keySet && (
                            <button
                                type='button'
                                className={am('btn', 'small', 'danger-ghost')}
                                onClick={() => set({keyAction: 'remove', apiKey: ''})}
                            >
                                {formatMessage({id: 'fusion.aiConnections.keyRemove', defaultMessage: 'Remove'})}
                            </button>
                        )}
                    </div>
                )}
                {draft.keyAction === 'replace' && (
                    <div className={am('ai-secret')}>
                        <input
                            type='password'
                            className={am('grow')}
                            value={draft.apiKey}
                            maxLength={8192}
                            autoComplete='new-password'
                            spellCheck={false}
                            aria-label={connection ? formatMessage({id: 'fusion.aiConnections.newKey', defaultMessage: 'New API key'}) : formatMessage({id: 'fusion.aiConnections.apiKey', defaultMessage: 'API key'})}
                            placeholder={formatMessage({id: 'fusion.aiConnections.keyPlaceholder', defaultMessage: 'Paste the key; leave it empty if the endpoint needs none'})}
                            onChange={(e) => set({apiKey: e.target.value})}
                        />
                        {connection && (
                            <button
                                type='button'
                                className={am('btn', 'small')}
                                onClick={() => set({keyAction: 'keep', apiKey: ''})}
                            >
                                {formatMessage({id: 'fusion.aiConnections.keyKeep', defaultMessage: 'Keep the saved key'})}
                            </button>
                        )}
                    </div>
                )}
                {draft.keyAction === 'remove' && (
                    <div className={am('ai-secret')}>
                        <span>{formatMessage({id: 'fusion.aiConnections.keyRemoving', defaultMessage: 'The key is removed when you save.'})}</span>
                        <button
                            type='button'
                            className={am('btn', 'small')}
                            onClick={() => set({keyAction: 'keep'})}
                        >
                            {formatMessage({id: 'fusion.aiConnections.undo', defaultMessage: 'Undo'})}
                        </button>
                    </div>
                )}
                <small className={am('field-note')}>{formatMessage({id: 'fusion.aiConnections.keyNote', defaultMessage: 'Stored encrypted. It\'s never shown again, only its last four characters.'})}</small>
            </div>

            <div className={am('field')}>
                <span>{formatMessage({id: 'fusion.aiConnections.headers', defaultMessage: 'Extra headers'})}</span>
                {!draft.headersChanged && (
                    <div className={am('ai-secret')}>
                        <span>
                            {draft.headers.length ? draft.headers.map((h) => h.name).join(', ') : formatMessage({id: 'fusion.aiConnections.headersNone', defaultMessage: 'None'})}
                        </span>
                        <button
                            type='button'
                            className={am('btn', 'small')}
                            onClick={() => set({headersChanged: true, headers: draft.headers.length ? draft.headers.map((h) => ({...h, value: ''})) : [newHeaderRow()]})}
                        >
                            {draft.headers.length ? formatMessage({id: 'fusion.aiConnections.headersChange', defaultMessage: 'Change'}) : formatMessage({id: 'fusion.aiConnections.headerAdd', defaultMessage: 'Add a header'})}
                        </button>
                        {draft.headers.length > 0 && (
                            <button
                                type='button'
                                className={am('btn', 'small', 'danger-ghost')}
                                onClick={() => set({headersChanged: true, headers: []})}
                            >
                                {formatMessage({id: 'fusion.aiConnections.headersRemove', defaultMessage: 'Remove all'})}
                            </button>
                        )}
                    </div>
                )}
                {draft.headersChanged && (
                    <>
                        {draft.headers.map((row, i) => (
                            <div
                                key={row.key}
                                className={am('ai-header-row')}
                            >
                                <input
                                    type='text'
                                    className={am('mono')}
                                    value={row.name}
                                    maxLength={128}
                                    spellCheck={false}
                                    placeholder='X-Org-Id'
                                    aria-label={formatMessage({id: 'fusion.aiConnections.headerName', defaultMessage: 'Name of header {n}'}, {n: i + 1})}
                                    onChange={(e) => setHeader(row, {name: e.target.value})}
                                />
                                <input
                                    type='password'
                                    className={am('mono')}
                                    value={row.value}
                                    maxLength={4096}
                                    autoComplete='new-password'
                                    spellCheck={false}
                                    aria-label={formatMessage({id: 'fusion.aiConnections.headerValue', defaultMessage: 'Value of header {n}'}, {n: i + 1})}
                                    onChange={(e) => setHeader(row, {value: e.target.value})}
                                />
                                <button
                                    type='button'
                                    className={am('icon-btn')}
                                    aria-label={formatMessage({id: 'fusion.aiConnections.headerRemove', defaultMessage: 'Remove header {n}'}, {n: i + 1})}
                                    onClick={() => set({headers: draft.headers.filter((h) => h.key !== row.key)})}
                                >
                                    <Icon
                                        name='trash'
                                        size='sm'
                                    />
                                </button>
                            </div>
                        ))}
                        <div className={am('ai-secret')}>
                            <button
                                type='button'
                                className={am('btn', 'small')}
                                onClick={() => set({headers: [...draft.headers, newHeaderRow()]})}
                            >
                                {formatMessage({id: 'fusion.aiConnections.headerAdd', defaultMessage: 'Add a header'})}
                            </button>
                            {connection && (
                                <button
                                    type='button'
                                    className={am('btn', 'small')}
                                    onClick={() => set({headersChanged: false, headers: (connection.extra_header_names || []).map((name) => newHeaderRow(name))})}
                                >
                                    {formatMessage({id: 'fusion.aiConnections.headersKeep', defaultMessage: 'Keep the saved headers'})}
                                </button>
                            )}
                        </div>
                        {problems.includes('headers') && <small className={am('field-note', 'bad')}>{formatMessage({id: 'fusion.aiConnections.headersBad', defaultMessage: 'A header name is letters, digits and - only, and can\'t be Host, Cookie or Content-Type.'})}</small>}
                        {connection && (connection.extra_header_names || []).length > 0 && <small className={am('field-note')}>{formatMessage({id: 'fusion.aiConnections.headersRetype', defaultMessage: 'Saved values are never shown: type each value again. Saving replaces all the headers.'})}</small>}
                    </>
                )}
            </div>

            <h3 className={am('set-section')}>{formatMessage({id: 'fusion.aiConnections.models', defaultMessage: 'Models'})}</h3>
            <p className={am('lead')}>{formatMessage({id: 'fusion.aiConnections.modelsDesc', defaultMessage: 'The model used for each capability when a feature or an app doesn\'t name one. Leave a capability empty if this endpoint doesn\'t offer it.'})}</p>
            <div className={am('field-row', 'ai-models')}>
                {capabilities.map((cap) => (
                    <label
                        key={cap}
                        className={am('field')}
                    >
                        <span>{capabilityLabel(cap)}</span>
                        <input
                            type='text'
                            className={am('mono')}
                            value={draft.models[cap]}
                            maxLength={256}
                            spellCheck={false}
                            autoComplete='off'
                            list={models ? modelListId : undefined}
                            onChange={(e) => set({models: {...draft.models, [cap]: e.target.value}})}
                        />
                    </label>
                ))}
            </div>
            {models && (
                <datalist id={modelListId}>
                    {models.map((m) => (
                        <option
                            key={m.id}
                            value={m.id}
                        >
                            {m.owned_by || m.id}
                        </option>
                    ))}
                </datalist>
            )}
            <div className={am('ai-secret')}>
                <button
                    type='button'
                    className={am('btn', 'small')}
                    disabled={busy !== '' || problems.includes('baseURL') || problems.includes('headers')}
                    onClick={loadModels}
                >
                    {formatMessage({id: 'fusion.aiConnections.loadModels', defaultMessage: 'Load models'})}
                </button>
                <small className={am('field-note')}>
                    {modelsNote}
                    {!modelsNote && models && formatMessage({id: 'fusion.aiConnections.modelsLoaded', defaultMessage: '{count, plural, =0 {The endpoint lists no models.} one {# model: pick it as you type.} other {# models: pick them as you type.}}'}, {count: models.length})}
                </small>
            </div>
            {draft.provider === 'openai_compatible' && (
                <Choice
                    title={formatMessage({id: 'fusion.aiConnections.dialect', defaultMessage: 'Decisions API'})}
                    desc={formatMessage({id: 'fusion.aiConnections.dialectDesc', defaultMessage: 'How the decisions model is called: OpenAI\'s /v1/decisions, or the native /v1/systemone of Jev and Laya.'})}
                    value={draft.decisionsDialect}
                    options={[
                        ['openai', formatMessage({id: 'fusion.aiConnections.dialect.openai', defaultMessage: 'OpenAI decisions'})],
                        ['systemone', formatMessage({id: 'fusion.aiConnections.dialect.systemone', defaultMessage: 'SystemOne'})],
                    ]}
                    onChange={(decisionsDialect) => set({decisionsDialect})}
                />
            )}

            <h3 className={am('set-section')}>{formatMessage({id: 'fusion.aiConnections.limits', defaultMessage: 'Limits'})}</h3>
            <p className={am('lead')}>{formatMessage({id: 'fusion.aiConnections.limitsDesc', defaultMessage: 'Empty or 0: no limit. Requests per minute are counted on each server; token budgets per calendar month (UTC).'})}</p>
            <div className={am('field-row', 'ai-limits')}>
                {LIMIT_KEYS.map((key) => (
                    <label
                        key={key}
                        className={am('field')}
                    >
                        <span>
                            {({
                                requests_per_minute: formatMessage({id: 'fusion.aiConnections.limit.rpm', defaultMessage: 'Requests per minute'}),
                                monthly_token_budget: formatMessage({id: 'fusion.aiConnections.limit.budget', defaultMessage: 'Tokens per month'}),
                                per_user_requests_per_minute: formatMessage({id: 'fusion.aiConnections.limit.userRpm', defaultMessage: 'Per person, requests per minute'}),
                                per_user_monthly_token_budget: formatMessage({id: 'fusion.aiConnections.limit.userBudget', defaultMessage: 'Per person, tokens per month'}),
                            })[key]}
                        </span>
                        <input
                            type='text'
                            inputMode='numeric'
                            value={draft.limits[key]}
                            maxLength={19}
                            placeholder='0'
                            onChange={(e) => set({limits: {...draft.limits, [key]: e.target.value}})}
                        />
                    </label>
                ))}
            </div>
            {problems.includes('limits') && <p className={am('field-error')}>{formatMessage({id: 'fusion.aiConnections.limitsBad', defaultMessage: 'Limits are whole numbers, 0 or more.'})}</p>}

            {serverWide && (
                <>
                    <h3 className={am('set-section')}>{formatMessage({id: 'fusion.aiConnections.sharing', defaultMessage: 'Communities'})}</h3>
                    <Toggle
                        title={formatMessage({id: 'fusion.aiConnections.allTeams', defaultMessage: 'All communities'})}
                        desc={formatMessage({id: 'fusion.aiConnections.allTeamsDesc', defaultMessage: 'Usable in every community, and in direct and group messages. Otherwise only in the communities chosen.'})}
                        on={draft.allTeams}
                        onChange={(allTeams) => set({allTeams})}
                    />
                    {!draft.allTeams && (
                        <div
                            className={am('chip-picks', 'ai-picks')}
                            role='group'
                            aria-label={formatMessage({id: 'fusion.aiConnections.teamsChosen', defaultMessage: 'Communities chosen'})}
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
                            {!communities.length && <span className={am('field-note')}>{formatMessage({id: 'fusion.aiConnections.noTeams', defaultMessage: 'No communities yet.'})}</span>}
                        </div>
                    )}
                </>
            )}

            <h3 className={am('set-section')}>{formatMessage({id: 'fusion.aiConnections.audience', defaultMessage: 'Who may use it'})}</h3>
            <Toggle
                title={formatMessage({id: 'fusion.aiConnections.everyone', defaultMessage: 'Everyone'})}
                desc={serverWide ? formatMessage({id: 'fusion.aiConnections.everyoneDesc', defaultMessage: 'Everyone where it\'s usable. Otherwise only the teams, roles and admins chosen.'}) : formatMessage({id: 'fusion.aiConnections.everyoneCommunityDesc', defaultMessage: 'Everyone in the community. Otherwise only its teams, roles and admins chosen.'})}
                on={draft.audience.everyone}
                onChange={(everyone) => set({audience: {...draft.audience, everyone}})}
            />
            {!draft.audience.everyone && (
                <div
                    className={am('chip-picks', 'ai-picks')}
                    role='group'
                    aria-label={formatMessage({id: 'fusion.aiConnections.audienceChosen', defaultMessage: 'Teams, roles and admins chosen'})}
                >
                    {(serverWide ? SERVER_ROLES : COMMUNITY_ROLES).map((role) => (
                        <button
                            key={role}
                            type='button'
                            className={am('chip-pick', {on: draft.audience.roles.includes(role)})}
                            aria-pressed={draft.audience.roles.includes(role)}
                            onClick={() => set({audience: {...draft.audience, roles: toggleIn(draft.audience.roles, role)}})}
                        >
                            {roleLabel(role)}
                        </button>
                    ))}
                    {groups.map(({group, community}) => (
                        <button
                            key={group.id}
                            type='button'
                            className={am('chip-pick', {on: draft.audience.group_ids.includes(group.id)})}
                            aria-pressed={draft.audience.group_ids.includes(group.id)}
                            onClick={() => set({audience: {...draft.audience, group_ids: toggleIn(draft.audience.group_ids, group.id)}})}
                        >
                            {serverWide && community ? `${community} · ${group.display_name}` : group.display_name}
                        </button>
                    ))}
                </div>
            )}
            <Toggle
                title={formatMessage({id: 'fusion.aiConnections.guests', defaultMessage: 'Guests too'})}
                desc={formatMessage({id: 'fusion.aiConnections.guestsDesc', defaultMessage: 'Guests may use it only when this is on, even with Everyone.'})}
                on={draft.audience.allow_guests}
                onChange={(allowGuests) => set({audience: {...draft.audience, allow_guests: allowGuests}})}
            />

            <div
                className={am('ai-status')}
                aria-live='polite'
            >
                {busy === 'test' && <span>{formatMessage({id: 'fusion.aiConnections.testing', defaultMessage: 'Testing…'})}</span>}
                {test?.ok && (
                    <span className={am('ai-ok')}>
                        <Icon
                            name='check'
                            size='sm'
                        />
                        {formatMessage({id: 'fusion.aiConnections.testOk', defaultMessage: 'It works: {count, plural, =0 {answered} one {# model} other {# models}} in {ms, number} ms.'}, {count: test.model_count, ms: test.latency_ms})}
                    </span>
                )}
                {test && !test.ok && (
                    <span
                        className={am('ai-bad')}
                        role='alert'
                    >
                        {test.status_code ? formatMessage({id: 'fusion.aiConnections.testFailedStatus', defaultMessage: 'It doesn\'t work (HTTP {status}): {error}'}, {status: test.status_code, error: test.error || ''}) : formatMessage({id: 'fusion.aiConnections.testFailed', defaultMessage: 'It doesn\'t work: {error}'}, {error: test.error || ''})}
                    </span>
                )}
                {testNote && <span>{testNote}</span>}
            </div>
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
                    type='button'
                    className={am('btn')}
                    disabled={busy !== '' || problems.includes('baseURL') || problems.includes('headers')}
                    onClick={runTest}
                >
                    {formatMessage({id: 'fusion.aiConnections.test', defaultMessage: 'Test connection'})}
                </button>
                <button
                    type='submit'
                    className={am('btn', 'primary')}
                    disabled={busy !== '' || problems.length > 0}
                >
                    {connection ? formatMessage({id: 'fusion.aiConnections.save', defaultMessage: 'Save'}) : formatMessage({id: 'fusion.aiConnections.create', defaultMessage: 'Add connection'})}
                </button>
            </div>
        </form>
    );
}
