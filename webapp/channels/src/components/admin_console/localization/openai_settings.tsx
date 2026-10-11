// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useState} from 'react';
import {defineMessage, FormattedMessage} from 'react-intl';

import type {AutoTranslationSettings} from '@mattermost/types/config';

import TextSetting from 'components/admin_console/text_setting';

import './localization.scss';

import {type SystemConsoleCustomSettingsComponentProps} from '../schema_admin_settings';

type OpenAISettings = NonNullable<AutoTranslationSettings['OpenAI']>;

// OpenAISettings are the settings of an OpenAI-compatible API translating directly: a translation model such as
// HY-MT or any general LLM served by vLLM, llama.cpp, Ollama, LiteLLM...
export default function OpenAISettings(props: SystemConsoleCustomSettingsComponentProps) {
    const values = props.value as AutoTranslationSettings;
    const [settings, setSettings] = useState<OpenAISettings>(values.OpenAI || {URL: '', APIKey: '', Model: ''});

    const handleChange = useCallback((id: string, value: string) => {
        const updatedSettings = {
            ...settings,
            [id]: value,
        };
        setSettings(updatedSettings);
        props.onChange('OpenAI', updatedSettings);
    }, [props, settings]);

    return (
        <>
            <TextSetting
                id='URL'
                label={
                    <FormattedMessage
                        id='admin.site.localization.autoTranslationProviderOpenAIURLTitle'
                        defaultMessage='API base URL:'
                    />
                }
                placeholder={defineMessage({
                    id: 'admin.site.localization.autoTranslationProviderOpenAIURLExample',
                    defaultMessage: 'e.g.: "https://llm.yourdomain.com/v1"',
                })}
                helpText={
                    <FormattedMessage
                        id='admin.site.localization.autoTranslationProviderOpenAIURLDescription'
                        defaultMessage='The base URL of an OpenAI-compatible API, up to and including /v1: messages are sent to its chat completions endpoint.'
                    />
                }
                type='url'
                value={settings.URL}
                setByEnv={false}
                onChange={handleChange}
                disabled={props.disabled}
            />
            <TextSetting
                id='Model'
                label={
                    <FormattedMessage
                        id='admin.site.localization.autoTranslationProviderOpenAIModelTitle'
                        defaultMessage='Model:'
                    />
                }
                placeholder={defineMessage({
                    id: 'admin.site.localization.autoTranslationProviderOpenAIModelExample',
                    defaultMessage: 'e.g.: "tencent/HY-MT1.5-1.8B"',
                })}
                helpText={
                    <FormattedMessage
                        id='admin.site.localization.autoTranslationProviderOpenAIModelDescription'
                        defaultMessage='The model as the API names it. A dedicated translation model (such as HY-MT) is fast and accurate; a general LLM works too.'
                    />
                }
                value={settings.Model}
                setByEnv={false}
                onChange={handleChange}
                disabled={props.disabled}
            />
            <TextSetting
                id='APIKey'
                label={
                    <FormattedMessage
                        id='admin.site.localization.autoTranslationProviderOpenAIAPIKeyTitle'
                        defaultMessage='API key:'
                    />
                }
                placeholder={defineMessage({
                    id: 'admin.site.localization.autoTranslationProviderOpenAIAPIKeyExample',
                    defaultMessage: 'Enter the API key',
                })}
                helpText={
                    <FormattedMessage
                        id='admin.site.localization.autoTranslationProviderOpenAIAPIKeyDescription'
                        defaultMessage='Sent as a bearer token. Leave it blank if the API needs none.'
                    />
                }
                type='password'
                value={settings.APIKey}
                setByEnv={false}
                onChange={handleChange}
                disabled={props.disabled}
            />
        </>
    );
}
