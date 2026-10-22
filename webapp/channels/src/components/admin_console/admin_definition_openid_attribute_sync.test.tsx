// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {AdminConfig} from '@mattermost/types/config';

import {RESOURCE_KEYS} from 'mattermost-redux/constants/permissions_sysconsole';

import Constants from 'utils/constants';

import AdminDefinition from './admin_definition';
import CustomProfileAttributes from './custom_profile_attributes/custom_profile_attributes';
import type {AdminDefinitionSetting, Check, ConsoleAccess} from './types';

const consoleAccess = {
    read: {[RESOURCE_KEYS.AUTHENTICATION.OPENID]: true},
    write: {[RESOURCE_KEYS.AUTHENTICATION.OPENID]: true},
} as ConsoleAccess;

const flagOn = {FeatureFlags: {OpenIdAttributeSync: true}} as unknown as Partial<AdminConfig>;
const flagOff = {FeatureFlags: {OpenIdAttributeSync: false}} as unknown as Partial<AdminConfig>;

const openIdState = {openidType: Constants.OPENID_SERVICE};

function getSetting(key: string): AdminDefinitionSetting {
    const settings = (AdminDefinition.authentication.subsections.openid.schema as {settings: AdminDefinitionSetting[]}).settings;
    const setting = settings.find((s) => s.key === key);
    if (!setting) {
        throw new Error(`setting ${key} not found`);
    }
    return setting;
}

function check(setting: AdminDefinitionSetting, prop: 'isHidden' | 'isDisabled', config: Partial<AdminConfig>, state: Record<string, unknown>, access = consoleAccess) {
    const fn = setting[prop] as Extract<Check, (...args: any[]) => boolean>;
    return fn(config, state, access, true);
}

describe('AdminDefinition - OpenID Connect attribute sync', () => {
    describe('User attributes sync section', () => {
        const setting = getSetting('OpenIdSettings.CustomProfileAttributes');

        test('mounts the shared CustomProfileAttributes component', () => {
            expect(setting.type).toBe('custom');
            expect((setting as Extract<AdminDefinitionSetting, {type: 'custom'}>).component).toBe(CustomProfileAttributes);
        });

        test('shows for a generic OpenID provider with the flag on', () => {
            expect(check(setting, 'isHidden', flagOn, openIdState)).toBe(false);
        });

        test.each([
            ['another provider is selected', flagOn, {openidType: Constants.GITLAB_SERVICE}],
            ['the flag is off', flagOff, openIdState],
        ])('hides when %s', (_, config, state) => {
            expect(check(setting, 'isHidden', config, state)).toBe(true);
        });

        test('is read-only without write access to OpenID Connect settings', () => {
            expect(check(setting, 'isDisabled', flagOn, openIdState)).toBe(false);
            expect(check(setting, 'isDisabled', flagOn, openIdState, {...consoleAccess, write: {}} as ConsoleAccess)).toBe(true);
        });
    });

    describe('Additional Scopes', () => {
        const setting = getSetting('OpenIdSettings.AdditionalScopes');

        test('is a text setting shown for the generic OpenID provider only', () => {
            expect(setting.type).toBe('text');
            expect(check(setting, 'isHidden', flagOn, openIdState)).toBe(false);
            expect(check(setting, 'isHidden', flagOn, {openidType: Constants.GOOGLE_SERVICE})).toBe(true);
        });
    });
});
