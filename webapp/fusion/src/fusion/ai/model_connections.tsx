// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useEffect, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {AIConnection, AIConnectionTestResult, AIConnectionUsage} from '@mattermost/types/ai_connections';

import {getTeams as fetchTeams} from 'mattermost-redux/actions/teams';
import {Client4} from 'mattermost-redux/client';
import {getTeams} from 'mattermost-redux/selectors/entities/teams';

import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';

import ConnectionForm from './connection_form';
import ConnectionUsage from './connection_usage';
import {AI_CONNECTION_SCOPE_SERVER, errorMessage, totalTokens} from './connections';

type Props = {

    // The scope of the connections added here: "server" in the System Console, the community's ID in its settings.
    scope: string;

    // The connections listed: those of a scope, or '' for every one the admin manages (the System Console's).
    listScope: string;
};

type View = {kind: 'list'} | {kind: 'new'} | {kind: 'edit'; id: string} | {kind: 'usage'; id: string};

// ModelConnections lists model connections, with this month's usage of each and what needs attention, and adds,
// edits, tests and deletes them (deleting asks first, in the page). The System Console shows the server's and every
// community's; a community's settings its own.
export default function ModelConnections({scope, listScope}: Props) {
    const {formatMessage, formatNumber} = useIntl();
    const dispatch = useDispatch();
    const teams = useSelector(getTeams);
    const [connections, setConnections] = useState<AIConnection[] | null>(null);
    const [usage, setUsage] = useState<Record<string, AIConnectionUsage>>({});
    const [loadError, setLoadError] = useState('');
    const [view, setView] = useState<View>({kind: 'list'});
    const [confirming, setConfirming] = useState('');
    const [busyId, setBusyId] = useState('');
    const [status, setStatus] = useState('');
    const [rowError, setRowError] = useState<{id: string; message: string} | null>(null);
    const [tests, setTests] = useState<Record<string, AIConnectionTestResult>>({});

    const loadUsage = useCallback((list: AIConnection[]) => {
        for (const c of list) {
            Client4.getAIConnectionUsage(c.id).then((u) => setUsage((all) => ({...all, [c.id]: u}))).catch(() => {});
        }
    }, []);

    const load = useCallback(async () => {
        try {
            const list = await Client4.getAIConnections(listScope || undefined);
            const sorted = [...list].sort((a, b) => a.name.localeCompare(b.name));
            setConnections(sorted);
            setLoadError('');
            loadUsage(sorted);
        } catch (err) {
            setLoadError(errorMessage(err, formatMessage({id: 'fusion.aiConnections.loadFailed', defaultMessage: 'The model connections couldn\'t be loaded.'})));
            setConnections([]);
        }
    }, [listScope, loadUsage, formatMessage]);

    useEffect(() => {
        load();
        if (!listScope) {
            dispatch(fetchTeams(0, 200));
        }
    }, [load, listScope, dispatch]);

    const providerLabel = (c: AIConnection) => (c.provider === 'anthropic' ? formatMessage({id: 'fusion.aiConnections.provider.anthropic', defaultMessage: 'Anthropic'}) : formatMessage({id: 'fusion.aiConnections.provider.openai', defaultMessage: 'OpenAI-compatible'}));
    const scopeLabel = (c: AIConnection) => {
        if (c.scope === AI_CONNECTION_SCOPE_SERVER) {
            if (c.sharing?.all_teams) {
                return formatMessage({id: 'fusion.aiConnections.scope.serverAll', defaultMessage: 'Server-wide, all communities'});
            }
            return formatMessage({id: 'fusion.aiConnections.scope.serverSome', defaultMessage: 'Server-wide, {count, plural, =0 {no community yet} one {# community} other {# communities}}'}, {count: c.sharing?.team_ids?.length || 0});
        }
        return formatMessage({id: 'fusion.aiConnections.scope.community', defaultMessage: 'Community: {name}'}, {name: teams[c.scope]?.display_name || c.scope});
    };
    const usageLine = (c: AIConnection) => {
        const u = usage[c.id];
        if (!u) {
            return '';
        }
        return formatMessage({id: 'fusion.aiConnections.usageLine', defaultMessage: 'This month: {tokens} tokens, {requests, plural, one {# request} other {# requests}}'}, {tokens: formatNumber(totalTokens(u.totals), {notation: 'compact', maximumFractionDigits: 1}), requests: u.totals.requests});
    };

    const remove = async (c: AIConnection) => {
        setBusyId(c.id);
        setRowError(null);
        try {
            await Client4.deleteAIConnection(c.id);
            setConnections((list) => (list || []).filter((x) => x.id !== c.id));
            setConfirming('');
            setStatus(formatMessage({id: 'fusion.aiConnections.deleted', defaultMessage: '{name} deleted. The features that used it no longer use AI until another connection is picked.'}, {name: c.name}));
        } catch (err) {
            setRowError({id: c.id, message: errorMessage(err, formatMessage({id: 'fusion.aiConnections.deleteFailed', defaultMessage: 'The connection couldn\'t be deleted.'}))});
        } finally {
            setBusyId('');
        }
    };

    const runTest = async (c: AIConnection) => {
        setBusyId(c.id);
        setRowError(null);
        try {
            const result = await Client4.testAIConnection(c.id);
            setTests((all) => ({...all, [c.id]: result}));
        } catch (err) {
            setRowError({id: c.id, message: errorMessage(err, formatMessage({id: 'fusion.aiConnections.test.failedToRun', defaultMessage: 'The test couldn\'t run.'}))});
        } finally {
            setBusyId('');
        }
    };

    const saved = (c: AIConnection, created: boolean) => {
        setConnections((list) => [...(list || []).filter((x) => x.id !== c.id), c].sort((a, b) => a.name.localeCompare(b.name)));
        setTests((all) => {
            const next = {...all};
            delete next[c.id];
            return next;
        });
        setView({kind: 'list'});
        setStatus(created ? formatMessage({id: 'fusion.aiConnections.created', defaultMessage: '{name} added.'}, {name: c.name}) : formatMessage({id: 'fusion.aiConnections.saved', defaultMessage: '{name} saved.'}, {name: c.name}));
    };

    if (!connections) {
        return <p className={am('lead')}>{formatMessage({id: 'fusion.aiConnections.loading', defaultMessage: 'Loading…'})}</p>;
    }

    const current = view.kind === 'edit' || view.kind === 'usage' ? connections.find((c) => c.id === view.id) : undefined;

    if (view.kind === 'new' || (view.kind === 'edit' && current)) {
        return (
            <div className={am('ai-connections')}>
                <ConnectionForm
                    key={current?.id || 'new'}
                    scope={scope}
                    connection={current}
                    onSaved={saved}
                    onCancel={() => {
                        setView({kind: 'list'});
                        setStatus('');
                    }}
                />
            </div>
        );
    }

    if (view.kind === 'usage' && current) {
        const u = usage[current.id];
        return (
            <div className={am('ai-connections')}>
                <h3 className={am('set-section')}>{formatMessage({id: 'fusion.aiConnections.usageOf', defaultMessage: 'Usage of {name}'}, {name: current.name})}</h3>
                {u ? <ConnectionUsage usage={u}/> : <p className={am('lead')}>{formatMessage({id: 'fusion.aiConnections.loading', defaultMessage: 'Loading…'})}</p>}
                <div className={am('modal-actions')}>
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={() => setView({kind: 'list'})}
                    >
                        {formatMessage({id: 'fusion.aiConnections.back', defaultMessage: 'Back to the connections'})}
                    </button>
                </div>
            </div>
        );
    }

    return (
        <div className={am('ai-connections')}>
            <p className={am('lead')}>
                {scope === AI_CONNECTION_SCOPE_SERVER ? formatMessage({id: 'fusion.aiConnections.lead', defaultMessage: 'The AI providers Antimatter may call, with their keys. The built-in features use the server-wide ones picked in Built-in features; apps use those their people may use. Keys are stored encrypted and never shown again.'}) : formatMessage({id: 'fusion.aiConnections.leadCommunity', defaultMessage: 'AI providers this community uses on its own, with their keys, for the apps of its people. Keys are stored encrypted and never shown again.'})}
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
            {!loadError && connections.length === 0 && <p className={am('lead')}>{formatMessage({id: 'fusion.aiConnections.none', defaultMessage: 'No model connections yet.'})}</p>}
            <ul
                className={am('ai-list')}
                aria-label={formatMessage({id: 'fusion.aiConnections.listLabel', defaultMessage: 'Model connections'})}
            >
                {connections.map((c) => {
                    const test = tests[c.id];
                    return (
                        <li
                            key={c.id}
                            className={am('ai-row', {attention: Boolean(c.needs_attention)})}
                        >
                            <div className={am('ai-row-main')}>
                                <div className={am('grow')}>
                                    <b>{c.name}</b>
                                    <span>{`${providerLabel(c)} · ${scopeLabel(c)}`}</span>
                                    {usageLine(c) && <span>{usageLine(c)}</span>}
                                </div>
                                {c.needs_attention && (
                                    <span
                                        className={am('ai-badge')}
                                        title={c.needs_attention}
                                    >
                                        {formatMessage({id: 'fusion.aiConnections.needsAttention', defaultMessage: 'Needs attention'})}
                                    </span>
                                )}
                                {confirming !== c.id && (
                                    <div className={am('ai-row-actions')}>
                                        <button
                                            type='button'
                                            className={am('btn', 'small')}
                                            disabled={busyId === c.id}
                                            onClick={() => runTest(c)}
                                            aria-label={formatMessage({id: 'fusion.aiConnections.testOne', defaultMessage: 'Test {name}'}, {name: c.name})}
                                        >
                                            {formatMessage({id: 'fusion.aiConnections.testShort', defaultMessage: 'Test'})}
                                        </button>
                                        <button
                                            type='button'
                                            className={am('icon-btn')}
                                            title={formatMessage({id: 'fusion.aiConnections.usageButton', defaultMessage: 'Usage'})}
                                            aria-label={formatMessage({id: 'fusion.aiConnections.usageOne', defaultMessage: 'Usage of {name}'}, {name: c.name})}
                                            onClick={() => {
                                                setStatus('');
                                                setView({kind: 'usage', id: c.id});
                                            }}
                                        >
                                            <Icon
                                                name='poll'
                                                size='sm'
                                            />
                                        </button>
                                        <button
                                            type='button'
                                            className={am('icon-btn')}
                                            title={formatMessage({id: 'fusion.aiConnections.edit', defaultMessage: 'Edit'})}
                                            aria-label={formatMessage({id: 'fusion.aiConnections.editOne', defaultMessage: 'Edit {name}'}, {name: c.name})}
                                            onClick={() => {
                                                setStatus('');
                                                setView({kind: 'edit', id: c.id});
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
                                            aria-label={formatMessage({id: 'fusion.aiConnections.deleteOne', defaultMessage: 'Delete {name}'}, {name: c.name})}
                                            onClick={() => {
                                                setConfirming(c.id);
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
                            {c.needs_attention && <p className={am('ai-attention')}>{c.needs_attention}</p>}
                            {confirming === c.id && (
                                <div
                                    className={am('ai-confirm')}
                                    role='group'
                                    aria-label={formatMessage({id: 'fusion.aiConnections.confirmLabel', defaultMessage: 'Delete {name}?'}, {name: c.name})}
                                >
                                    <span>{formatMessage({id: 'fusion.aiConnections.confirm', defaultMessage: 'Delete {name}? Its key is forgotten, and what uses it stops using AI. This can\'t be undone.'}, {name: c.name})}</span>
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
                                        disabled={busyId === c.id}
                                        onClick={() => remove(c)}
                                    >
                                        {formatMessage({id: 'fusion.aiConnections.confirmDelete', defaultMessage: 'Delete connection'})}
                                    </button>
                                </div>
                            )}
                            {test && (
                                <p
                                    className={am('ai-status', test.ok ? 'ai-ok' : 'ai-bad')}
                                    role='status'
                                >
                                    {test.ok ? formatMessage({id: 'fusion.aiConnections.testOk', defaultMessage: 'It works: {count, plural, =0 {answered} one {# model} other {# models}} in {ms, number} ms.'}, {count: test.model_count, ms: test.latency_ms}) : formatMessage({id: 'fusion.aiConnections.testFailed', defaultMessage: 'It doesn\'t work: {error}'}, {error: test.error || String(test.status_code || '')})}
                                </p>
                            )}
                            {rowError?.id === c.id && (
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
                    {formatMessage({id: 'fusion.aiConnections.add', defaultMessage: 'Add a connection'})}
                </button>
            </div>
        </div>
    );
}
