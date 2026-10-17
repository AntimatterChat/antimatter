// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import {useIntl} from 'react-intl';

import type {AIConnection, AIModelInfo} from '@mattermost/types/ai_connections';
import type {AIFeatureSettings} from '@mattermost/types/config';

import {Client4} from 'mattermost-redux/client';

import Setting from 'components/admin_console/setting';

const AI_CONNECTION_SCOPE_SERVER = 'server';

// The default models a connection names, each once.
function defaultModels(models: AIConnection['models'] | undefined): string[] {
    const out: string[] = [];
    for (const m of [models?.chat, models?.embeddings, models?.transcription, models?.decisions]) {
        if (m && !out.includes(m)) {
            out.push(m);
        }
    }
    return out;
}

// The message of an API error (its AppError's, translated by the server), else the fallback.
function errorMessage(error: unknown, fallback: string): string {
    const message = (error as {message?: string} | undefined)?.message;
    return typeof message === 'string' && message.trim() ? message : fallback;
}

// useServerConnections loads the server-wide model connections, those the built-in features may use.
export function useServerConnections(): {connections: AIConnection[] | null; error: string} {
    const {formatMessage} = useIntl();
    const [connections, setConnections] = useState<AIConnection[] | null>(null);
    const [error, setError] = useState('');
    useEffect(() => {
        let live = true;
        Client4.getAIConnections(AI_CONNECTION_SCOPE_SERVER).then((list) => {
            if (live) {
                setConnections(list.filter((c) => c.scope === AI_CONNECTION_SCOPE_SERVER).sort((a, b) => a.name.localeCompare(b.name)));
            }
        }).catch((err) => {
            if (live) {
                setConnections([]);
                setError(errorMessage(err, formatMessage({id: 'admin.ai.features.loadFailed', defaultMessage: 'The model connections couldn\'t be loaded.'})));
            }
        });
        return () => {
            live = false;
        };
    }, [formatMessage]);
    return {connections, error};
}

type Props = {

    // The setting's id, for its fields: Rewrite, Recaps, AutoTranslation, TextToSpeech.
    id: string;
    label: React.ReactNode;
    helpText?: React.ReactNode;
    value: AIFeatureSettings | undefined;
    connections: AIConnection[] | null;

    // What an empty model means for this feature, given the connection picked.
    defaultModelLabel: (connection: AIConnection | undefined) => string;
    disabled: boolean;
    setByEnv?: boolean;
    onChange: (value: AIFeatureSettings) => void;
};

// The classic System Console's copy of the Fusion one (components/admin_console/ai), for its auto-translation panel.

// FeatureConnectionPicker picks the model connection and the model of a built-in AI feature: a server-wide
// connection, or Off; then its default model, one of the defaults it names, or one of the models its endpoint lists.
export default function FeatureConnectionPicker({id, label, helpText, value, connections, defaultModelLabel, disabled, setByEnv = false, onChange}: Props) {
    const {formatMessage} = useIntl();
    const connectionId = value?.ConnectionId || '';
    const model = value?.Model || '';
    const [models, setModels] = useState<Record<string, AIModelInfo[]>>({});
    const [loading, setLoading] = useState(false);
    const [modelsError, setModelsError] = useState('');
    const connection = connections?.find((c) => c.id === connectionId);

    useEffect(() => {
        setModelsError('');
    }, [connectionId]);

    const loadModels = async () => {
        setLoading(true);
        setModelsError('');
        try {
            const list = await Client4.getAIConnectionModels(connectionId);
            setModels((all) => ({...all, [connectionId]: list}));
        } catch (err) {
            setModelsError(errorMessage(err, formatMessage({id: 'admin.ai.features.modelsFailed', defaultMessage: 'The models couldn\'t be loaded.'})));
        } finally {
            setLoading(false);
        }
    };

    // The models offered: the connection's defaults, the ones its endpoint lists, and the one saved even if neither
    // names it any more.
    const choices: string[] = [];
    const add = (m: string) => {
        if (m && !choices.includes(m)) {
            choices.push(m);
        }
    };
    defaultModels(connection?.models).forEach(add);
    (models[connectionId] || []).forEach((m) => add(m.id));
    add(model);

    const connectionHelp = () => {
        if (!connectionId) {
            return formatMessage({id: 'admin.ai.features.off', defaultMessage: 'Off: this feature uses no AI.'});
        }
        if (connections && !connection) {
            return formatMessage({id: 'admin.ai.features.missing', defaultMessage: 'This connection no longer exists or isn\'t server-wide: the feature fails until another one is picked.'});
        }
        if (connection?.needs_attention) {
            return formatMessage({id: 'admin.ai.features.attention', defaultMessage: 'This connection needs attention: {reason}'}, {reason: connection.needs_attention});
        }
        return '';
    };

    return (
        <>
            <Setting
                label={label}
                inputId={`${id}ConnectionId`}
                helpText={(
                    <>
                        {helpText}
                        {connectionHelp() && <div className={connection?.needs_attention || (connections && connectionId && !connection) ? 'error-message' : undefined}>{connectionHelp()}</div>}
                    </>
                )}
                setByEnv={setByEnv}
            >
                <select
                    id={`${id}ConnectionId`}
                    className='form-control'
                    value={connectionId}
                    disabled={disabled || setByEnv || !connections}
                    onChange={(e) => onChange({ConnectionId: e.target.value, Model: ''})}
                >
                    <option value=''>{formatMessage({id: 'admin.ai.features.offOption', defaultMessage: 'Off'})}</option>
                    {connections?.map((c) => (
                        <option
                            key={c.id}
                            value={c.id}
                        >
                            {c.needs_attention ? formatMessage({id: 'admin.ai.features.attentionOption', defaultMessage: '{name} (needs attention)'}, {name: c.name}) : c.name}
                        </option>
                    ))}
                    {connections && connectionId && !connection && (
                        <option value={connectionId}>{formatMessage({id: 'admin.ai.features.unknownOption', defaultMessage: 'Unknown connection ({id})'}, {id: connectionId})}</option>
                    )}
                </select>
            </Setting>
            {connectionId && (
                <Setting
                    label={formatMessage({id: 'admin.ai.features.model', defaultMessage: 'Model'})}
                    inputId={`${id}Model`}
                    helpText={modelsError ? <span className='error-message'>{modelsError}</span> : formatMessage({id: 'admin.ai.features.modelHelp', defaultMessage: 'The connection\'s defaults are listed; load the models to see all those its endpoint offers.'})}
                    setByEnv={setByEnv}
                >
                    <div className='ai-feature-model'>
                        <select
                            id={`${id}Model`}
                            className='form-control'
                            value={model}
                            disabled={disabled || setByEnv}
                            onChange={(e) => onChange({ConnectionId: connectionId, Model: e.target.value})}
                        >
                            <option value=''>{defaultModelLabel(connection)}</option>
                            {choices.map((m) => (
                                <option
                                    key={m}
                                    value={m}
                                >
                                    {m}
                                </option>
                            ))}
                        </select>
                        <button
                            type='button'
                            className='btn btn-tertiary btn-sm'
                            disabled={disabled || loading || !connection}
                            onClick={loadModels}
                        >
                            {formatMessage({id: 'admin.ai.features.loadModels', defaultMessage: 'Load models'})}
                        </button>
                    </div>
                </Setting>
            )}
        </>
    );
}
