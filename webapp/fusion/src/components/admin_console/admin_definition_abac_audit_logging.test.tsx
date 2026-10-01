// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {AdminConfig} from '@mattermost/types/config';

import {RESOURCE_KEYS} from 'mattermost-redux/constants/permissions_sysconsole';
import {getConfig} from 'mattermost-redux/selectors/entities/general';

import AdminDefinition from './admin_definition';
import type {AdminDefinitionSettingInput, Check, ConsoleAccess} from './types';

jest.mock('mattermost-redux/selectors/entities/general', () => ({
    ...jest.requireActual('mattermost-redux/selectors/entities/general'),
    getConfig: jest.fn(),
}));

const mockedGetConfig = getConfig as jest.Mock;

type BoolAdminDefinitionSetting = AdminDefinitionSettingInput;

const consoleAccess = {
    read: {},
    write: {},
} as ConsoleAccess;

const abacFeatureFlagEnabled = {
    FeatureFlags: {
        AttributeBasedAccessControl: true,
    },
} as unknown as Partial<AdminConfig>;

function getAuditLoggingSetting(): BoolAdminDefinitionSetting {
    const subsection = AdminDefinition.system_attributes.subsections.attribute_based_access_control;
    const schema = subsection.schema;
    const sections = 'sections' in schema ? schema.sections ?? [] : [];
    const settings = sections[0]?.settings ?? [];
    const setting = settings.find((s) => s.key === 'AccessControlSettings.EnableAccessControlAuditLogging');
    return setting as BoolAdminDefinitionSetting;
}

function callIsDisabled(check: Check | undefined, state: Record<string, unknown>) {
    const disabledCheck = check as Extract<Check, (...args: any[]) => boolean>;
    return disabledCheck({}, state, consoleAccess);
}

describe('AdminDefinition - ABAC audit logging toggle', () => {
    afterEach(() => {
        mockedGetConfig.mockReset();
    });

    test('defines the EnableAccessControlAuditLogging bool setting with the expected copy', () => {
        const setting = getAuditLoggingSetting();

        expect(setting).toBeDefined();
        expect(setting.type).toBe('bool');
        expect(setting.key).toBe('AccessControlSettings.EnableAccessControlAuditLogging');
        expect(setting.label).toBeDefined();
        expect((setting.label as {id: string}).id).toBe('admin.accesscontrol.enableAuditLogging.title');
        expect(setting.help_text).toBeDefined();
        expect((setting.help_text as {id: string}).id).toBe('admin.accesscontrol.enableAuditLogging.desc');
        expect(setting.disabled_help_text).toBeDefined();
        expect((setting.disabled_help_text as {id: string}).id).toBe('admin.accesscontrol.enableAuditLogging.disabled');
    });

    test('is enabled when ABAC is on and audit logging is active', () => {
        mockedGetConfig.mockReturnValue({AuditLoggingActive: 'true'});

        const setting = getAuditLoggingSetting();
        const disabled = callIsDisabled(setting.isDisabled, {'AccessControlSettings.EnableAttributeBasedAccessControl': true});

        expect(disabled).toBe(false);
    });

    test('is disabled when ABAC master toggle is off', () => {
        mockedGetConfig.mockReturnValue({AuditLoggingActive: 'true'});

        const setting = getAuditLoggingSetting();
        const disabled = callIsDisabled(setting.isDisabled, {'AccessControlSettings.EnableAttributeBasedAccessControl': false});

        expect(disabled).toBe(true);
    });

    test('is disabled when server audit logging is not active', () => {
        mockedGetConfig.mockReturnValue({AuditLoggingActive: 'false'});

        const setting = getAuditLoggingSetting();
        const disabled = callIsDisabled(setting.isDisabled, {'AccessControlSettings.EnableAttributeBasedAccessControl': true});

        expect(disabled).toBe(true);
    });

    test('is disabled when AuditLoggingActive is undefined (fail-safe default)', () => {
        mockedGetConfig.mockReturnValue({});

        const setting = getAuditLoggingSetting();
        const disabled = callIsDisabled(setting.isDisabled, {'AccessControlSettings.EnableAttributeBasedAccessControl': true});

        expect(disabled).toBe(true);
    });

    test('subsection is visible with the feature flag and read access', () => {
        const subsection = AdminDefinition.system_attributes.subsections.attribute_based_access_control;
        const hiddenCheck = subsection.isHidden as Extract<Check, (...args: any[]) => boolean>;

        const readAccess = {
            read: {[RESOURCE_KEYS.USER_MANAGEMENT.SYSTEM_ROLES]: true},
        } as unknown as ConsoleAccess;

        expect(hiddenCheck(abacFeatureFlagEnabled, {}, readAccess)).toBe(false);
    });
});
