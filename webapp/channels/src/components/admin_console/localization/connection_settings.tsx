// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {FormattedMessage, useIntl} from 'react-intl';

import type {AIConnection} from '@mattermost/types/ai_connections';
import type {AIFeatureSettings} from '@mattermost/types/config';

import FeatureConnectionPicker, {useServerConnections} from 'components/admin_console/ai/feature_connection_picker';

import './localization.scss';

type Props = {
    value: AIFeatureSettings;
    disabled: boolean;
    onChange: (value: AIFeatureSettings) => void;
};

// ConnectionTranslationSettings picks the model connection (and model) auto-translation uses, the same setting as
// AI › Built-in features' (AISettings.AutoTranslation).
export default function ConnectionTranslationSettings({value, disabled, onChange}: Props) {
    const {formatMessage} = useIntl();
    const {connections, error} = useServerConnections();
    const chatDefault = (c: AIConnection | undefined) => (c?.models.chat ? formatMessage({id: 'admin.ai.features.defaultChat', defaultMessage: 'The connection\'s chat model ({model})'}, {model: c.models.chat}) : formatMessage({id: 'admin.ai.features.defaultChatNone', defaultMessage: 'The connection\'s chat model (none set: pick one)'}));

    return (
        <>
            <FeatureConnectionPicker
                id='AutoTranslation'
                label={
                    <FormattedMessage
                        id='admin.site.localization.autoTranslationConnectionTitle'
                        defaultMessage='Model connection'
                    />
                }
                helpText={(
                    <>
                        <FormattedMessage
                            id='admin.site.localization.autoTranslationConnectionHelp'
                            defaultMessage='A server-wide model connection, set up in the System Console of the Fusion web app (AI › Model connections). A dedicated translation model (such as HY-MT) is fast and accurate; a general LLM works too.'
                        />
                        {error && <div className='error-message'>{error}</div>}
                    </>
                )}
                value={value}
                connections={connections}
                defaultModelLabel={chatDefault}
                disabled={disabled}
                onChange={onChange}
            />
        </>
    );
}

type OlderProps = {

    // The AI chat app's service the older setting named; connections migrated from the app keep its ID.
    serviceId: string;
    disabled: boolean;
    onSwitch: (connectionId: string) => void;
};

// OlderAgentsProvider explains the older "agents" provider, which now translates with a model connection too (the
// one migrated from the AI chat app's service), and offers to switch to the setting that names it.
export function OlderAgentsProvider({serviceId, disabled, onSwitch}: OlderProps) {
    return (
        <div className='form-group'>
            <div className='col-sm-offset-4 col-sm-8'>
                <div className='alert alert-info older-agents-provider'>
                    <FormattedMessage
                        id='admin.site.localization.autoTranslationAgentsOlder'
                        defaultMessage='This older setting no longer needs the AI chat app: it translates with the model connection picked for auto-translation, else with the one migrated from the app’s service. Switch to “Model connection” to pick it here.'
                    />
                    <div>
                        <button
                            type='button'
                            className='btn btn-tertiary btn-sm'
                            disabled={disabled}
                            onClick={() => onSwitch(serviceId)}
                        >
                            <FormattedMessage
                                id='admin.site.localization.autoTranslationAgentsSwitch'
                                defaultMessage='Switch to Model connection'
                            />
                        </button>
                    </div>
                </div>
            </div>
        </div>
    );
}
