// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useMemo, useState} from 'react';
import {defineMessage, defineMessages, FormattedMessage, useIntl} from 'react-intl';

import type {AIFeatureSettings, AutoTranslationSettings} from '@mattermost/types/config';

import BooleanSetting from 'components/admin_console/boolean_setting';
import MultiSelectSetting from 'components/admin_console/multiselect_settings';
import Setting from 'components/admin_console/setting';
import {
    AdminSection,
    SectionContent,
    SectionHeader,
} from 'components/admin_console/system_properties/controls';
import TextSetting from 'components/admin_console/text_setting';
import Toggle from 'components/toggle';
import BetaTag from 'components/widgets/tag/beta_tag';

import * as I18n from 'i18n/i18n';

import AutoTranslationInfo from './auto_translation_info';
import ConnectionTranslationSettings, {OlderAgentsProvider} from './connection_settings';
import LibreTranslateSettings from './libreTranslate_settings';
import OpenAISettings from './openai_settings';

import type {SystemConsoleCustomSettingsComponentProps} from '../schema_admin_settings';
import type {SearchableStrings} from '../types';

import './localization.scss';

const locales = I18n.getAllLanguages();

const messages = defineMessages({
    providerConnection: {
        id: 'admin.site.localization.autoTranslationProviderConnection',
        defaultMessage: 'Model connection (recommended)',
    },
    providerOpenAI: {
        id: 'admin.site.localization.autoTranslationProviderOpenAI',
        defaultMessage: 'OpenAI-compatible API, set up here',
    },
    providerAgents: {
        id: 'admin.site.localization.autoTranslationProviderAgents',
        defaultMessage: 'AI chat app\'s service (older)',
    },
    enableAutoTranslationTitle: {
        id: 'admin.site.localization.enableAutoTranslationTitle',
        defaultMessage: 'Auto-translation',
    },
    enableAutoTranslationDescription: {
        id: 'admin.site.localization.enableAutoTranslationDescription',
        defaultMessage: 'Configure auto-translation for channels and direct messages',
    },
});

export const searchableStrings: SearchableStrings = Object.values(messages);

// The key of AISettings.AutoTranslation on the Localization page (a hidden setting there saves it).
export const AI_AUTO_TRANSLATION_KEY = 'AISettings.AutoTranslation';

export default function AutoTranslation(props: SystemConsoleCustomSettingsComponentProps) {
    const {formatMessage} = useIntl();
    // Antimatter: a model connection is the recommended provider, the default for a new setup.
    const [autoTranslationSettings, setAutoTranslationSettings] = useState<AutoTranslationSettings>(() => {
        const settings = props.value as AutoTranslationSettings;
        if (!settings.Provider) {
            return {...settings, Provider: 'connection'};
        }
        return settings;
    });

    // The connection and model when the provider is a model connection: AISettings.AutoTranslation.
    const [connectionSettings, setConnectionSettings] = useState<AIFeatureSettings>(() => ({
        ConnectionId: props.config?.AISettings?.AutoTranslation?.ConnectionId || '',
        Model: props.config?.AISettings?.AutoTranslation?.Model || '',
    }));
    const handleConnectionChange = useCallback((value: AIFeatureSettings) => {
        setConnectionSettings(value);
        props.onChange(AI_AUTO_TRANSLATION_KEY, value);
    }, [props]);

    // Track timeout input separately to allow intermediate invalid states while typing
    const [timeoutInputValue, setTimeoutInputValue] = useState<string>(
        String(autoTranslationSettings.TimeoutMs || 5000),
    );
    const [timeoutError, setTimeoutError] = useState<string>('');

    const handleChange = useCallback((id: string, value: AutoTranslationSettings[keyof AutoTranslationSettings] | string[]) => {
        const updatedSettings = {
            ...autoTranslationSettings,
            [id]: value,
        };
        setAutoTranslationSettings(updatedSettings);
        props.onChange(props.id, updatedSettings);
    }, [props, autoTranslationSettings]);

    const handleTimeoutChange = useCallback((id: string, value: string) => {
        setTimeoutInputValue(value);

        const numValue = parseInt(value, 10);
        if (value === '' || isNaN(numValue) || numValue <= 0) {
            setTimeoutError('Timeout must be a positive number');

            // Propagate 0 so backend validation will reject the save
            handleChange(id, 0);
            return;
        }

        setTimeoutError('');
        handleChange(id, numValue);
    }, [handleChange]);

    const handleToggle = useCallback(() => {
        const newValue = !autoTranslationSettings.Enable;
        const newSettings = {
            ...autoTranslationSettings,
            Enable: newValue,
        };

        // Ensure provider is set when enabling
        if (newValue && !newSettings.Provider) {
            newSettings.Provider = 'connection';
        }

        setAutoTranslationSettings(newSettings);
        props.onChange(props.id, newSettings);
    }, [autoTranslationSettings, props]);

    const availableLanguages = useMemo(() => {
        const values: Array<{value: string; text: string; order: number}> = [];
        for (const l of Object.values(locales)) {
            values.push({value: l.value, text: l.name, order: l.order});
        }
        values.sort((a, b) => a.order - b.order);
        return values;
    }, []);

    const providerDescription = useMemo(() => (
        <div className='auto-translation-provider-description'>
            <FormattedMessage
                id='admin.site.localization.autoTranslationProviderHint'
                defaultMessage="Choose the provider you'd like to use for translation."
            />
        </div>
    ), []);

    const providerNote = useMemo(() => (
        <FormattedMessage
            id='admin.site.localization.autoTranslationProviderDescription'
            defaultMessage='<strong>NOTE:</strong> If using external translation services (e.g., cloud-based LLMs), message data may be processed outside your environment.'
            values={{
                strong: (msg: React.ReactNode) => <strong>{msg}</strong>,
            }}
        />
    ), []);

    const on = (
        <FormattedMessage
            id='admin.site.localization.auto_translation.on'
            defaultMessage='On'
        />
    );
    const off = (
        <FormattedMessage
            id='admin.site.localization.auto_translation.off'
            defaultMessage='Off'
        />
    );

    const selectedLanguages = autoTranslationSettings.TargetLanguages || ['en'];

    return (
        <AdminSection>
            <SectionHeader>
                <div
                    data-testid='autotranslation-section-header'
                    className='autotranslation-section-header'
                >
                    <hgroup>
                        <h1 className='localization-section-title'>
                            <FormattedMessage {...messages.enableAutoTranslationTitle}/>
                            <BetaTag
                                variant='default'
                            />
                        </h1>
                        <h5 className='localization-section-description'>
                            <FormattedMessage {...messages.enableAutoTranslationDescription}/>
                        </h5>
                    </hgroup>
                    <div
                        data-testid='autotranslation-section-toggle'
                        className='autotranslation-section-toggle'
                    >
                        <span style={{marginRight: '12px'}}>
                            {autoTranslationSettings.Enable ? on : off}
                        </span>
                        <Toggle
                            size='btn-md'
                            disabled={props.disabled || props.setByEnv}
                            toggled={autoTranslationSettings.Enable}
                            id={'Enable'}
                            tabIndex={-1}
                            toggleClassName='btn-toggle-primary'
                            onToggle={handleToggle}
                        />
                    </div>
                </div>
            </SectionHeader>
            {autoTranslationSettings.Enable &&
            <SectionContent $compact={true}>
                <AutoTranslationInfo/>
                <div>
                    <Setting
                        label={
                            <FormattedMessage
                                id='admin.site.localization.autoTranslationProviderTitle'
                                defaultMessage='Translation provider'
                            />
                        }
                        inputId={'Provider'}
                        setByEnv={props.setByEnv}
                        helpText={providerNote}
                    >
                        <div className='auto-translation-provider-body'>
                            {providerDescription}
                            <select
                                data-testid='Providerdropdown'
                                className='form-control'
                                id='Provider'
                                value={autoTranslationSettings.Provider || 'connection'}
                                onChange={(e) => handleChange('Provider', e.target.value as AutoTranslationSettings['Provider'])}
                                disabled={props.disabled || props.setByEnv}
                            >
                                <option value='connection'>{formatMessage(messages.providerConnection)}</option>
                                <option value='libretranslate'>{'LibreTranslate'}</option>
                                <option value='openai'>{formatMessage(messages.providerOpenAI)}</option>
                                <option value='agents'>{formatMessage(messages.providerAgents)}</option>
                            </select>
                        </div>
                    </Setting>
                </div>
                {autoTranslationSettings.Provider === 'connection' &&
                <ConnectionTranslationSettings
                    value={connectionSettings}
                    disabled={props.disabled || props.setByEnv}
                    onChange={handleConnectionChange}
                />
                }
                {autoTranslationSettings.Provider === 'agents' &&
                <OlderAgentsProvider
                    serviceId={autoTranslationSettings.Agents?.LLMServiceID || ''}
                    disabled={props.disabled || props.setByEnv}
                    onSwitch={(connectionId) => {
                        handleConnectionChange({ConnectionId: connectionSettings.ConnectionId || connectionId, Model: connectionSettings.Model});
                        handleChange('Provider', 'connection');
                    }}
                />
                }
                {autoTranslationSettings.Provider === 'libretranslate' &&
                <LibreTranslateSettings
                    {...props}
                    onChange={handleChange}
                />
                }
                {autoTranslationSettings.Provider === 'openai' &&
                <OpenAISettings
                    {...props}
                    onChange={handleChange}
                />
                }
                <MultiSelectSetting
                    id={'TargetLanguages'}
                    label={
                        <FormattedMessage
                            id='admin.site.localization.targetLanguagesTitle'
                            defaultMessage='Languages allowed'
                        />
                    }
                    values={availableLanguages}
                    helpText={
                        <FormattedMessage
                            id='admin.site.localization.targetLanguagesDescription'
                            defaultMessage="Choose which languages you'd like to make available for auto-translation."
                        />
                    }
                    selected={selectedLanguages}
                    onChange={handleChange}
                    disabled={props.disabled || props.setByEnv}
                    setByEnv={props.setByEnv}
                />
                <TextSetting
                    id='TimeoutMs'
                    label={
                        <FormattedMessage
                            id='admin.site.localization.autoTranslationTimeoutTitle'
                            defaultMessage='Translation timeout (ms):'
                        />
                    }
                    placeholder={defineMessage({
                        id: 'admin.site.localization.autoTranslationTimeoutPlaceholder',
                        defaultMessage: 'e.g.: 5000',
                    })}
                    helpText={timeoutError ? (
                        <span className='autotranslation-error error-message'>{timeoutError}</span>
                    ) : (
                        <FormattedMessage
                            id='admin.site.localization.autoTranslationTimeoutDescription'
                            defaultMessage='Maximum time in milliseconds to wait for a translation response. Default is 5000ms (5 seconds).'
                        />
                    )}
                    type='number'
                    value={timeoutInputValue}
                    setByEnv={props.setByEnv}
                    onChange={handleTimeoutChange}
                    disabled={props.disabled}
                />
                <BooleanSetting
                    id='RestrictDMAndGM'
                    label={
                        <FormattedMessage
                            id='admin.site.localization.restrictDMAndGMTitle'
                            defaultMessage='Restrict auto-translation on direct messages and group messages'
                        />
                    }
                    helpText={
                        <FormattedMessage
                            id='admin.site.localization.restrictDMAndGMDescription'
                            defaultMessage='By default, any member of a direct message or group message can enable auto-translation in those channels. If restricted, auto-translation will not be available in direct messages and group messages.'
                        />
                    }
                    value={autoTranslationSettings.RestrictDMAndGM}
                    onChange={handleChange}
                    disabled={props.disabled || props.setByEnv}
                    setByEnv={props.setByEnv}
                />
            </SectionContent>
            }
        </AdminSection>
    );
}
