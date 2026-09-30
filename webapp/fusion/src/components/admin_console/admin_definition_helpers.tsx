// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {MessageDescriptor} from 'react-intl';

import type {AdminConfig, ClientConfig} from '@mattermost/types/config';

import {getConfig as getClientConfig} from 'mattermost-redux/selectors/entities/general';

import store from 'stores/redux_store';

import type {Check, ConsoleAccess} from './types';
import ValidationResult from './validation';

export const it = {
    not: (func: Check) => (config: Partial<AdminConfig>, state: any, consoleAccess?: ConsoleAccess, isSystemAdmin?: boolean) => {
        return typeof func === 'function' ? !func(config, state, consoleAccess, isSystemAdmin) : !func;
    },
    all: (...funcs: Check[]) => (config: Partial<AdminConfig>, state: any, consoleAccess?: ConsoleAccess, isSystemAdmin?: boolean) => {
        for (const func of funcs) {
            if (typeof func === 'function' ? !func(config, state, consoleAccess, isSystemAdmin) : !func) {
                return false;
            }
        }
        return true;
    },
    any: (...funcs: Check[]) => (config: Partial<AdminConfig>, state: any, consoleAccess?: ConsoleAccess, isSystemAdmin?: boolean) => {
        for (const func of funcs) {
            if (typeof func === 'function' ? func(config, state, consoleAccess, isSystemAdmin) : func) {
                return true;
            }
        }
        return false;
    },
    stateMatches: (key: string, regex: RegExp) => (config: Partial<AdminConfig>, state: any) => state[key].match(regex),
    stateEquals: (key: string, value: any) => (config: Partial<AdminConfig>, state: any) => state[key] === value,
    stateEqualsOrDefault: (key: string, value: any, defaultValue: any) => (config: Partial<AdminConfig>, state: any) => {
        const stateValue = state[key];
        return stateValue === value || (stateValue == null && value === defaultValue);
    },
    stateIsTrue: (key: string) => (config: Partial<AdminConfig>, state: any) => Boolean(state[key]),
    stateIsFalse: (key: string) => (config: Partial<AdminConfig>, state: any) => !state[key],
    configIsTrue: (group: keyof Partial<AdminConfig>, setting: string) => (config: Partial<AdminConfig>) => Boolean((config[group] as any)?.[setting]),
    configIsFalse: (group: keyof Partial<AdminConfig>, setting: string) => (config: Partial<AdminConfig>) => !(config[group] as any)?.[setting],
    clientConfigIsTrue: (setting: keyof ClientConfig) => () => getClientConfig(store.getState())[setting] === 'true',
    clientConfigIsFalse: (setting: keyof ClientConfig) => () => getClientConfig(store.getState())[setting] !== 'true',
    configContains: (group: keyof Partial<AdminConfig>, setting: string, word: string) => (config: Partial<AdminConfig>) => Boolean((config[group] as any)?.[setting]?.includes(word)),
    userHasReadPermissionOnResource: (key: string) => (config: Partial<AdminConfig>, state: any, consoleAccess?: ConsoleAccess) => (consoleAccess?.read as any)?.[key],
    userHasReadPermissionOnSomeResources: (key: {[key: string]: string}) => (config: Partial<AdminConfig>, state: any, consoleAccess?: ConsoleAccess) => Object.values(key).some((resource) => (consoleAccess?.read as any)?.[resource]),
    userHasWritePermissionOnResource: (key: string) => (config: Partial<AdminConfig>, state: any, consoleAccess?: ConsoleAccess) => (consoleAccess?.write as any)?.[key],
    isSystemAdmin: (config: Partial<AdminConfig>, state: any, consoleAccess?: ConsoleAccess, isSystemAdmin?: boolean) => Boolean(isSystemAdmin),
};

export const validators = {
    isRequired: (text: MessageDescriptor | string) => (value: string) => new ValidationResult(Boolean(value), text),
    minValue: (min: number, text: MessageDescriptor | string) => (value: number) => new ValidationResult((value >= min), text),
    maxValue: (max: number, text: MessageDescriptor | string) => (value: number) => new ValidationResult((value <= max), text),
    numberInRange: (min: number, max: number, text: MessageDescriptor | string) => (value: number) => new ValidationResult(Number.isNaN(value) || (value >= min && value <= max), text),
};

export const usesLegacyOauth = (config: Partial<AdminConfig>, state: any, consoleAccess?: ConsoleAccess) => {
    if (!config.GitLabSettings || !config.GoogleSettings || !config.Office365Settings) {
        return false;
    }

    return it.any(
        it.all(
            it.not(it.configContains('GitLabSettings', 'Scope', 'openid')),
            it.any(
                it.configIsTrue('GitLabSettings', 'Id'),
                it.configIsTrue('GitLabSettings', 'Secret'),
            ),
        ),
        it.all(
            it.not(it.configContains('GoogleSettings', 'Scope', 'openid')),
            it.any(
                it.configIsTrue('GoogleSettings', 'Id'),
                it.configIsTrue('GoogleSettings', 'Secret'),
            ),
        ),
        it.all(
            it.not(it.configContains('Office365Settings', 'Scope', 'openid')),
            it.any(
                it.configIsTrue('Office365Settings', 'Id'),
                it.configIsTrue('Office365Settings', 'Secret'),
            ),
        ),
    )(config, state, consoleAccess);
};
