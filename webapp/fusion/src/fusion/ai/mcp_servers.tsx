// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useEffect, useState} from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import type {AIMCPAuth, AIMCPServer} from '@mattermost/types/ai_tools';

import {Client4} from 'mattermost-redux/client';
import {getTeams} from 'mattermost-redux/selectors/entities/teams';

import Icon from 'fusion/components/icon';
import {Choice, Toggle} from 'fusion/modals/settings_controls';
import {am} from 'fusion/utils/class_names';

import {AI_CONNECTION_SCOPE_SERVER, errorMessage, newHeaderRow} from './connections';
import type {HeaderRow} from './connections';
import {draftFromMCPServer, mcpNamespace, mcpPatchOf, mcpProblemsOf, mcpRequestOf, newMCPDraft, slugFromName} from './mcp';
import type {MCPDraft} from './mcp';

type Props = {

    // The scope of the servers added here: "server" in the System Console, the community's ID in its settings.
    scope: string;

    // The servers listed: those of a scope, or '' for every one the admin manages (the System Console's).
    listScope: string;
};

type View = {kind: 'list'} | {kind: 'new'} | {kind: 'edit'; id: string};

// MCPServers lists the MCP servers agents may use (their tools: mcp.<short name>.<tool>), and adds, edits and deletes
// them, next to the model connections. A server's secrets are write-only; people's sign-ins are theirs and never
// shown, not even here.
export default function MCPServers({scope, listScope}: Props) {
    const {formatMessage} = useIntl();
    const teams = useSelector(getTeams);
    const [servers, setServers] = useState<AIMCPServer[] | null>(null);
    const [loadError, setLoadError] = useState('');
    const [view, setView] = useState<View>({kind: 'list'});
    const [confirming, setConfirming] = useState('');
    const [busyId, setBusyId] = useState('');
    const [status, setStatus] = useState('');
    const [rowError, setRowError] = useState<{id: string; message: string} | null>(null);

    const load = useCallback(async () => {
        try {
            const list = await Client4.getAIMCPServers(listScope || undefined);
            setServers([...list].sort((a, b) => a.name.localeCompare(b.name)));
            setLoadError('');
        } catch (err) {
            setLoadError(errorMessage(err, formatMessage({id: 'fusion.mcpServers.loadFailed', defaultMessage: 'The MCP servers couldn\'t be loaded.'})));
            setServers([]);
        }
    }, [listScope, formatMessage]);

    useEffect(() => {
        load();
    }, [load]);

    const authLabel = (auth: AIMCPAuth) => {
        switch (auth) {
        case 'none':
            return formatMessage({id: 'fusion.mcpServers.auth.none', defaultMessage: 'No sign-in'});
        case 'service_account':
            return formatMessage({id: 'fusion.mcpServers.auth.service', defaultMessage: 'Shared account (reads only)'});
        default:
            return formatMessage({id: 'fusion.mcpServers.auth.oauth', defaultMessage: 'Each person signs in'});
        }
    };
    const scopeLabel = (s: AIMCPServer) => {
        if (s.scope === AI_CONNECTION_SCOPE_SERVER) {
            if (s.sharing?.all_teams) {
                return formatMessage({id: 'fusion.aiConnections.scope.serverAll', defaultMessage: 'Server-wide, all communities'});
            }
            return formatMessage({id: 'fusion.aiConnections.scope.serverSome', defaultMessage: 'Server-wide, {count, plural, =0 {no community yet} one {# community} other {# communities}}'}, {count: s.sharing?.team_ids?.length || 0});
        }
        return formatMessage({id: 'fusion.aiConnections.scope.community', defaultMessage: 'Community: {name}'}, {name: teams[s.scope]?.display_name || s.scope});
    };

    const remove = async (s: AIMCPServer) => {
        setBusyId(s.id);
        setRowError(null);
        try {
            await Client4.deleteAIMCPServer(s.id);
            setServers((list) => (list || []).filter((x) => x.id !== s.id));
            setConfirming('');
            setStatus(formatMessage({id: 'fusion.mcpServers.deleted', defaultMessage: '{name} deleted, with everyone\'s sign-in to it.'}, {name: s.name}));
        } catch (err) {
            setRowError({id: s.id, message: errorMessage(err, formatMessage({id: 'fusion.mcpServers.deleteFailed', defaultMessage: 'The MCP server couldn\'t be deleted.'}))});
        } finally {
            setBusyId('');
        }
    };

    const saved = (s: AIMCPServer, created: boolean) => {
        setServers((list) => [...(list || []).filter((x) => x.id !== s.id), s].sort((a, b) => a.name.localeCompare(b.name)));
        setView({kind: 'list'});
        setStatus(created ? formatMessage({id: 'fusion.aiConnections.created', defaultMessage: '{name} added.'}, {name: s.name}) : formatMessage({id: 'fusion.aiConnections.saved', defaultMessage: '{name} saved.'}, {name: s.name}));
    };

    if (!servers) {
        return null;
    }

    const current = view.kind === 'edit' ? servers.find((s) => s.id === view.id) : undefined;
    if (view.kind === 'new' || (view.kind === 'edit' && current)) {
        return (
            <section className={am('ai-connections', 'ai-mcp')}>
                <h3 className={am('set-section')}>{current ? formatMessage({id: 'fusion.mcpServers.editTitle', defaultMessage: 'Edit {name}'}, {name: current.name}) : formatMessage({id: 'fusion.mcpServers.newTitle', defaultMessage: 'Add an MCP server'})}</h3>
                <MCPServerForm
                    key={current?.id || 'new'}
                    scope={current?.scope || scope}
                    server={current}
                    onSaved={saved}
                    onCancel={() => {
                        setView({kind: 'list'});
                        setStatus('');
                    }}
                />
            </section>
        );
    }

    return (
        <section
            className={am('ai-connections', 'ai-mcp')}
            aria-label={formatMessage({id: 'fusion.mcpServers.title', defaultMessage: 'MCP servers'})}
        >
            <h3 className={am('set-section')}>{formatMessage({id: 'fusion.mcpServers.title', defaultMessage: 'MCP servers'})}</h3>
            <p className={am('lead')}>
                {formatMessage({id: 'fusion.mcpServers.lead', defaultMessage: 'Services whose tools agents may use, as their settings allow. The tools always act as the person who asked: each person signs in with their own account, from the AI chat\'s Tools menu.'})}
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
            {!loadError && servers.length === 0 && <p className={am('lead')}>{formatMessage({id: 'fusion.mcpServers.none', defaultMessage: 'No MCP servers yet.'})}</p>}
            <ul
                className={am('ai-list')}
                aria-label={formatMessage({id: 'fusion.mcpServers.title', defaultMessage: 'MCP servers'})}
            >
                {servers.map((s) => (
                    <li
                        key={s.id}
                        className={am('ai-row', {attention: Boolean(s.needs_attention)})}
                    >
                        <div className={am('ai-row-main')}>
                            <div className={am('grow')}>
                                <b>{s.name}</b>
                                <span className={am('mono')}>{`${mcpNamespace(s.slug)}.*`}</span>
                                <span>{`${authLabel(s.auth)} · ${scopeLabel(s)}`}</span>
                            </div>
                            {confirming !== s.id && (
                                <div className={am('ai-row-actions')}>
                                    <button
                                        type='button'
                                        className={am('icon-btn')}
                                        title={formatMessage({id: 'fusion.aiConnections.edit', defaultMessage: 'Edit'})}
                                        aria-label={formatMessage({id: 'fusion.aiConnections.editOne', defaultMessage: 'Edit {name}'}, {name: s.name})}
                                        onClick={() => {
                                            setStatus('');
                                            setView({kind: 'edit', id: s.id});
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
                                        aria-label={formatMessage({id: 'fusion.aiConnections.deleteOne', defaultMessage: 'Delete {name}'}, {name: s.name})}
                                        onClick={() => {
                                            setConfirming(s.id);
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
                        {s.needs_attention && <p className={am('ai-attention')}>{s.needs_attention}</p>}
                        {confirming === s.id && (
                            <div
                                className={am('ai-confirm')}
                                role='group'
                                aria-label={formatMessage({id: 'fusion.aiConnections.confirmLabel', defaultMessage: 'Delete {name}?'}, {name: s.name})}
                            >
                                <span>{formatMessage({id: 'fusion.mcpServers.confirm', defaultMessage: 'Delete {name}? Its secrets and everyone\'s sign-in to it are forgotten, and agents lose its tools. This can\'t be undone.'}, {name: s.name})}</span>
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
                                    disabled={busyId === s.id}
                                    onClick={() => remove(s)}
                                >
                                    {formatMessage({id: 'fusion.mcpServers.confirmDelete', defaultMessage: 'Delete server'})}
                                </button>
                            </div>
                        )}
                        {rowError?.id === s.id && (
                            <p
                                className={am('field-error')}
                                role='alert'
                            >
                                {rowError.message}
                            </p>
                        )}
                    </li>
                ))}
            </ul>
            <div className={am('modal-actions', 'ai-add')}>
                <button
                    type='button'
                    className={am('btn')}
                    onClick={() => {
                        setStatus('');
                        setView({kind: 'new'});
                    }}
                >
                    <Icon
                        name='plus'
                        size='sm'
                    />
                    {formatMessage({id: 'fusion.mcpServers.add', defaultMessage: 'Add an MCP server'})}
                </button>
            </div>
        </section>
    );
}

type FormProps = {
    scope: string;
    server?: AIMCPServer;
    onSaved: (server: AIMCPServer, created: boolean) => void;
    onCancel: () => void;
};

// MCPServerForm adds or edits an MCP server: its name and short name, its address, how people use its account (each
// their own sign-in, none, or a shared service account whose tools only read), and, server-wide, the communities it's
// shared with.
export function MCPServerForm({scope, server, onSaved, onCancel}: FormProps) {
    const {formatMessage} = useIntl();
    const teams = useSelector(getTeams);
    const [draft, setDraft] = useState<MCPDraft>(() => (server ? draftFromMCPServer(server) : newMCPDraft(scope)));
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');
    const problems = mcpProblemsOf(draft, Boolean(server));
    const serverWide = scope === AI_CONNECTION_SCOPE_SERVER;
    const communities = Object.values(teams).filter((t) => t.delete_at === 0).sort((a, b) => a.display_name.localeCompare(b.display_name));

    const set = (patch: Partial<MCPDraft>) => {
        setDraft((d) => ({...d, ...patch}));
        setError('');
    };
    const setHeader = (row: HeaderRow, patch: Partial<HeaderRow>) => set({headers: draft.headers.map((h) => (h.key === row.key ? {...h, ...patch} : h))});
    const toggleIn = (list: string[], id: string) => (list.includes(id) ? list.filter((x) => x !== id) : [...list, id]);

    const save = async () => {
        setBusy(true);
        setError('');
        try {
            const result = server ? await Client4.patchAIMCPServer(server.id, mcpPatchOf(draft, server)) : await Client4.createAIMCPServer(mcpRequestOf(draft, scope));
            onSaved(result, !server);
        } catch (err) {
            setError(errorMessage(err, formatMessage({id: 'fusion.mcpServers.saveFailed', defaultMessage: 'The MCP server couldn\'t be saved.'})));
        } finally {
            setBusy(false);
        }
    };

    return (
        <form
            className={am('ai-form')}
            onSubmit={(e) => {
                e.preventDefault();
                save();
            }}
        >
            <label className={am('field')}>
                <span>{formatMessage({id: 'fusion.aiConnections.name', defaultMessage: 'Name'})}</span>
                <input
                    type='text'
                    value={draft.name}
                    maxLength={64}
                    placeholder={formatMessage({id: 'fusion.mcpServers.namePlaceholder', defaultMessage: 'e.g. Issue tracker'})}
                    onChange={(e) => set({name: e.target.value, ...(draft.slugTouched ? {} : {slug: slugFromName(e.target.value)})})}
                />
            </label>
            <label className={am('field')}>
                <span>{formatMessage({id: 'fusion.mcpServers.slug', defaultMessage: 'Short name'})}</span>
                <input
                    type='text'
                    className={am('mono')}
                    value={draft.slug}
                    maxLength={32}
                    spellCheck={false}
                    disabled={Boolean(server)}
                    onChange={(e) => set({slug: e.target.value.toLowerCase(), slugTouched: true})}
                />
                <small className={am('field-note', {bad: problems.includes('slug')})}>
                    {formatMessage({id: 'fusion.mcpServers.slugDesc', defaultMessage: 'Its tools are named {namespace}.tool_name in agents\' settings. Lower-case letters, digits, - and _; it can\'t change later.'}, {namespace: mcpNamespace(draft.slug || '…')})}
                </small>
            </label>
            <label className={am('field')}>
                <span>{formatMessage({id: 'fusion.mcpServers.url', defaultMessage: 'Address'})}</span>
                <input
                    type='url'
                    className={am('mono')}
                    value={draft.url}
                    maxLength={1024}
                    spellCheck={false}
                    placeholder='https://mcp.example.com/mcp'
                    onChange={(e) => set({url: e.target.value})}
                />
                <small className={am('field-note', {bad: problems.includes('url') && Boolean(draft.url)})}>
                    {serverWide ? formatMessage({id: 'fusion.mcpServers.urlDesc', defaultMessage: 'Its Streamable HTTP endpoint. Changing it signs everyone out.'}) : formatMessage({id: 'fusion.mcpServers.urlDescCommunity', defaultMessage: 'Its Streamable HTTP endpoint, on the internet (not the server\'s own network). Changing it signs everyone out.'})}
                </small>
            </label>

            <Choice
                title={formatMessage({id: 'fusion.mcpServers.auth', defaultMessage: 'Account'})}
                desc={formatMessage({id: 'fusion.mcpServers.authDesc', defaultMessage: 'Whose account its tools use. A shared account acts for everyone: only its tools that read are offered.'})}
                value={draft.auth}
                options={[
                    ['oauth', formatMessage({id: 'fusion.mcpServers.auth.oauthShort', defaultMessage: 'Each person'})],
                    ['none', formatMessage({id: 'fusion.mcpServers.auth.noneShort', defaultMessage: 'None'})],
                    ['service_account', formatMessage({id: 'fusion.mcpServers.auth.serviceShort', defaultMessage: 'Shared'})],
                ]}
                onChange={(auth) => set({auth, ...(auth === 'service_account' && draft.headers.length === 0 ? {headersChanged: true, headers: [newHeaderRow('Authorization')]} : {})})}
            />

            {draft.auth === 'oauth' && (
                <>
                    <label className={am('field')}>
                        <span>{formatMessage({id: 'fusion.mcpServers.clientId', defaultMessage: 'OAuth client ID'})}</span>
                        <input
                            type='text'
                            className={am('mono')}
                            value={draft.clientId}
                            maxLength={512}
                            spellCheck={false}
                            autoComplete='off'
                            onChange={(e) => set({clientId: e.target.value})}
                        />
                        <small className={am('field-note')}>{formatMessage({id: 'fusion.mcpServers.clientIdDesc', defaultMessage: 'Left empty, Antimatter registers itself with the server\'s sign-in the first time someone signs in. People are sent back to {path}.'}, {path: '/api/v4/ai/mcp/oauth/callback'})}</small>
                    </label>
                    <div className={am('field')}>
                        <span>{formatMessage({id: 'fusion.mcpServers.clientSecret', defaultMessage: 'OAuth client secret'})}</span>
                        {draft.secretAction === 'keep' && server?.oauth_client_secret_set ? (
                            <div className={am('ai-secret')}>
                                <span>{formatMessage({id: 'fusion.mcpServers.secretSet', defaultMessage: 'Set'})}</span>
                                <button
                                    type='button'
                                    className={am('btn', 'small')}
                                    onClick={() => set({secretAction: 'replace', clientSecret: ''})}
                                >
                                    {formatMessage({id: 'fusion.mcpServers.secretReplace', defaultMessage: 'Replace'})}
                                </button>
                                <button
                                    type='button'
                                    className={am('btn', 'small', 'danger-ghost')}
                                    onClick={() => set({secretAction: 'remove', clientSecret: ''})}
                                >
                                    {formatMessage({id: 'fusion.mcpServers.secretRemove', defaultMessage: 'Remove'})}
                                </button>
                            </div>
                        ) : (
                            <input
                                type='password'
                                className={am('mono')}
                                value={draft.clientSecret}
                                maxLength={4096}
                                autoComplete='new-password'
                                spellCheck={false}
                                aria-label={formatMessage({id: 'fusion.mcpServers.clientSecret', defaultMessage: 'OAuth client secret'})}
                                onChange={(e) => set({clientSecret: e.target.value, secretAction: 'replace'})}
                            />
                        )}
                        <small className={am('field-note')}>{formatMessage({id: 'fusion.mcpServers.secretDesc', defaultMessage: 'Stored encrypted and never shown again. People\'s own sign-ins are never shown either, to anyone.'})}</small>
                    </div>
                    <label className={am('field')}>
                        <span>{formatMessage({id: 'fusion.mcpServers.scopes', defaultMessage: 'Scopes'})}</span>
                        <input
                            type='text'
                            className={am('mono')}
                            value={draft.scopes}
                            maxLength={1024}
                            spellCheck={false}
                            placeholder='read write'
                            onChange={(e) => set({scopes: e.target.value})}
                        />
                    </label>
                </>
            )}

            {draft.auth === 'service_account' && (
                <div className={am('field')}>
                    <span>{formatMessage({id: 'fusion.mcpServers.headers', defaultMessage: 'The shared account\'s headers'})}</span>
                    {!draft.headersChanged && (
                        <div className={am('ai-secret')}>
                            <span>{draft.headers.map((h) => h.name).join(', ')}</span>
                            <button
                                type='button'
                                className={am('btn', 'small')}
                                onClick={() => set({headersChanged: true, headers: draft.headers.map((h) => ({...h, value: ''}))})}
                            >
                                {formatMessage({id: 'fusion.aiConnections.headersChange', defaultMessage: 'Change'})}
                            </button>
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
                                        placeholder='Authorization'
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
                            </div>
                        </>
                    )}
                    <small className={am('field-note', {bad: problems.includes('headers') && draft.headersChanged})}>{formatMessage({id: 'fusion.mcpServers.headersDesc', defaultMessage: 'Sent with every call, for everyone: give it an account that can only read. Stored encrypted; saved values are never shown, type them all again to change them.'})}</small>
                </div>
            )}

            {serverWide && (
                <>
                    <h3 className={am('set-section')}>{formatMessage({id: 'fusion.aiConnections.sharing', defaultMessage: 'Communities'})}</h3>
                    <Toggle
                        title={formatMessage({id: 'fusion.aiConnections.allTeams', defaultMessage: 'All communities'})}
                        desc={formatMessage({id: 'fusion.mcpServers.allTeamsDesc', defaultMessage: 'Server-wide agents may use it in every community, and in direct and group messages. Otherwise only in the communities chosen, where their own agents may use it too.'})}
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
                    {server ? formatMessage({id: 'fusion.aiConnections.save', defaultMessage: 'Save'}) : formatMessage({id: 'fusion.mcpServers.create', defaultMessage: 'Add server'})}
                </button>
            </div>
        </form>
    );
}
