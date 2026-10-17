// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback} from 'react';
import {defineMessages, FormattedMessage, useIntl} from 'react-intl';
import {Link} from 'react-router-dom';

import type {AIConnection} from '@mattermost/types/ai_connections';
import type {AIFeatureSettings, AISettings} from '@mattermost/types/config';

import type {SystemConsoleCustomSettingsComponentProps} from 'components/admin_console/schema_admin_settings';
import type {SearchableStrings} from 'components/admin_console/types';

import FeatureConnectionPicker, {useServerConnections} from './feature_connection_picker';

import './ai.scss';

const messages = defineMessages({
    rewrite: {id: 'admin.ai.features.rewrite', defaultMessage: 'Rewrite in the composer'},
    rewriteHelp: {id: 'admin.ai.features.rewriteHelp', defaultMessage: 'The AI menu of the message box: improve, shorten, fix, translate. It shows once a connection is picked.'},
    recaps: {id: 'admin.ai.features.recaps', defaultMessage: 'Recaps'},
    recapsHelp: {id: 'admin.ai.features.recapsHelp', defaultMessage: 'Summaries of the channels people choose, on demand or on a schedule. Recaps also need to be on in Site Configuration › Recaps.'},
    autoTranslation: {id: 'admin.ai.features.autoTranslation', defaultMessage: 'Auto-translation'},
    autoTranslationHelp: {id: 'admin.ai.features.autoTranslationHelp', defaultMessage: 'Used when the translation provider is "Model connection", in <link>Site Configuration › Localization</link>.'},
    textToSpeech: {id: 'admin.ai.features.textToSpeech', defaultMessage: 'Text to speech'},
    textToSpeechHelp: {id: 'admin.ai.features.textToSpeechHelp', defaultMessage: 'Reads messages aloud with the connection\'s /audio/speech endpoint, in place of the address and key of <link>Integrations › Text to speech</link>, which also sets the voice.'},
    lead: {id: 'admin.ai.features.lead', defaultMessage: 'Pick the server-wide model connection each built-in feature uses, and its model. A feature without one uses no AI. The built-in features don\'t need the AI chat app.'},
});

export const searchableStrings: SearchableStrings = Object.values(messages);

const FEATURES: Array<keyof AISettings> = ['Rewrite', 'Recaps', 'AutoTranslation', 'TextToSpeech'];

const NO_FEATURE: AIFeatureSettings = {ConnectionId: '', Model: ''};

// BuiltInFeatures is the System Console's AI › Built-in features: for the composer's rewrite, recaps,
// auto-translation and text to speech, a server-wide model connection (or Off) and a model. It edits AISettings.
export default function BuiltInFeatures(props: SystemConsoleCustomSettingsComponentProps) {
    const {formatMessage} = useIntl();
    const value = (props.value || {}) as Partial<AISettings>;
    const {connections, error} = useServerConnections();
    const speechModel = props.config?.TextToSpeechSettings?.Model || '';

    const change = useCallback((feature: keyof AISettings, next: AIFeatureSettings) => {
        const all: AISettings = {
            Rewrite: value.Rewrite || NO_FEATURE,
            Recaps: value.Recaps || NO_FEATURE,
            AutoTranslation: value.AutoTranslation || NO_FEATURE,
            TextToSpeech: value.TextToSpeech || NO_FEATURE,
        };
        props.onChange(props.id, {...all, [feature]: next});
    }, [props, value]);

    const chatDefault = (c: AIConnection | undefined) => (c?.models.chat ? formatMessage({id: 'admin.ai.features.defaultChat', defaultMessage: 'The connection\'s chat model ({model})'}, {model: c.models.chat}) : formatMessage({id: 'admin.ai.features.defaultChatNone', defaultMessage: 'The connection\'s chat model (none set: pick one)'}));
    const speechDefault = () => (speechModel ? formatMessage({id: 'admin.ai.features.defaultSpeech', defaultMessage: 'The text to speech model ({model})'}, {model: speechModel}) : formatMessage({id: 'admin.ai.features.defaultSpeechNone', defaultMessage: 'The text to speech model (none set: pick one)'}));

    const link = (to: string) => (chunks: React.ReactNode) => <Link to={to}>{chunks}</Link>;

    const labels: Record<keyof AISettings, [React.ReactNode, React.ReactNode]> = {
        Rewrite: [
            <FormattedMessage
                {...messages.rewrite}
                key='l'
            />,
            <FormattedMessage
                {...messages.rewriteHelp}
                key='h'
            />,
        ],
        Recaps: [
            <FormattedMessage
                {...messages.recaps}
                key='l'
            />,
            <FormattedMessage
                {...messages.recapsHelp}
                key='h'
            />,
        ],
        AutoTranslation: [
            <FormattedMessage
                {...messages.autoTranslation}
                key='l'
            />,
            <FormattedMessage
                {...messages.autoTranslationHelp}
                key='h'
                values={{link: link('/admin_console/site_config/localization')}}
            />,
        ],
        TextToSpeech: [
            <FormattedMessage
                {...messages.textToSpeech}
                key='l'
            />,
            <FormattedMessage
                {...messages.textToSpeechHelp}
                key='h'
                values={{link: link('/admin_console/integrations/text_to_speech')}}
            />,
        ],
    };

    return (
        <div className='ai-builtin-features'>
            <p className='ai-lead'>
                <FormattedMessage {...messages.lead}/>
            </p>
            {error && (
                <div
                    className='error-message'
                    role='alert'
                >
                    {error}
                </div>
            )}
            {connections?.length === 0 && !error && (
                <p className='ai-lead'>
                    <FormattedMessage
                        id='admin.ai.features.noConnections'
                        defaultMessage='There are no server-wide model connections yet: add one in <link>AI › Model connections</link>.'
                        values={{link: link('/admin_console/ai/model_connections')}}
                    />
                </p>
            )}
            {FEATURES.map((feature) => (
                <FeatureConnectionPicker
                    key={feature}
                    id={feature}
                    label={labels[feature][0]}
                    helpText={labels[feature][1]}
                    value={value[feature]}
                    connections={connections}
                    defaultModelLabel={feature === 'TextToSpeech' ? speechDefault : chatDefault}
                    disabled={props.disabled}
                    setByEnv={props.setByEnv}
                    onChange={(next) => change(feature, next)}
                />
            ))}
        </div>
    );
}
