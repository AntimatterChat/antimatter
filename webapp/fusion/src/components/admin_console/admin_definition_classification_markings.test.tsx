// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {AdminConfig} from '@mattermost/types/config';

import AdminDefinition from './admin_definition';
import type {AdminDefinitionSubSection, Check, ConsoleAccess} from './types';

const classificationConfigEnabled = {
    FeatureFlags: {
        ClassificationMarkings: true,
    },
} as unknown as Partial<AdminConfig>;

const classificationConfigDisabled = {
    FeatureFlags: {
        ClassificationMarkings: false,
    },
} as unknown as Partial<AdminConfig>;

const consoleAccess = {
    read: {},
    write: {},
} as ConsoleAccess;

function isHidden(subsection: AdminDefinitionSubSection, config: Partial<AdminConfig>) {
    const check = subsection.isHidden as Extract<Check, (...args: any[]) => boolean>;
    return check(config, {}, consoleAccess);
}

describe('AdminDefinition - Classification Markings', () => {
    const settingsSubsection = AdminDefinition.site.subsections.classification_markings;

    test('shows the settings page when the Classification Markings feature flag is enabled', () => {
        expect(isHidden(settingsSubsection, classificationConfigEnabled)).toBe(false);
    });

    test('hides the settings page when the Classification Markings feature flag is disabled', () => {
        expect(isHidden(settingsSubsection, classificationConfigDisabled)).toBe(true);
    });

    test('disables the settings page for non system admins', () => {
        const settingsDisabledCheck = settingsSubsection.isDisabled as Extract<Check, (...args: any[]) => boolean>;

        const asSystemAdmin = settingsDisabledCheck(classificationConfigEnabled, {}, consoleAccess, true);
        const asNonSystemAdmin = settingsDisabledCheck(classificationConfigEnabled, {}, consoleAccess, false);

        expect(asSystemAdmin).toBe(false);
        expect(asNonSystemAdmin).toBe(true);
    });
});
