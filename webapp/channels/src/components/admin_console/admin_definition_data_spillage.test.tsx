// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {AdminConfig} from '@mattermost/types/config';

import {RESOURCE_KEYS} from 'mattermost-redux/constants/permissions_sysconsole';

import AdminDefinition from './admin_definition';
import type {AdminDefinitionSubSection, Check, ConsoleAccess} from './types';

const contentFlaggingConfigEnabled = {
    FeatureFlags: {
        ContentFlagging: true,
    },
} as unknown as Partial<AdminConfig>;

const contentFlaggingConfigDisabled = {
    FeatureFlags: {
        ContentFlagging: false,
    },
} as unknown as Partial<AdminConfig>;

const consoleAccess = {
    read: {
        [RESOURCE_KEYS.SITE.POSTS]: true,
        [RESOURCE_KEYS.USER_MANAGEMENT.SYSTEM_ROLES]: true,
    },
    write: {
        [RESOURCE_KEYS.SITE.POSTS]: true,
        [RESOURCE_KEYS.USER_MANAGEMENT.SYSTEM_ROLES]: true,
    },
} as ConsoleAccess;

function isHidden(subsection: AdminDefinitionSubSection, config: Partial<AdminConfig>) {
    const check = subsection.isHidden as Extract<Check, (...args: any[]) => boolean>;
    return check(config, {}, consoleAccess);
}

describe('AdminDefinition - Data Spillage', () => {
    const settingsSubsection = AdminDefinition.site.subsections.content_flagging;

    test('shows the settings page when the Content Flagging feature flag is enabled', () => {
        const siteSectionHiddenCheck = AdminDefinition.site.isHidden as Extract<Check, (...args: any[]) => boolean>;

        expect(siteSectionHiddenCheck(contentFlaggingConfigEnabled, {}, consoleAccess)).toBe(false);
        expect(isHidden(settingsSubsection, contentFlaggingConfigEnabled)).toBe(false);
    });

    test('hides the settings page when the Content Flagging feature flag is disabled', () => {
        expect(isHidden(settingsSubsection, contentFlaggingConfigDisabled)).toBe(true);
    });
});
