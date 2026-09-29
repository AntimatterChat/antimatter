// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {AdminConfig} from '@mattermost/types/config';

import AdminDefinition from './admin_definition';
import type {AdminDefinitionSubSection, Check, ConsoleAccess} from './types';

const boardsFlagEnabled = {
    FeatureFlags: {
        IntegratedBoards: true,
    },
} as unknown as Partial<AdminConfig>;

const boardsFlagDisabled = {
    FeatureFlags: {
        IntegratedBoards: false,
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

describe('AdminDefinition - Board Attributes', () => {
    const settingsSubsection = AdminDefinition.system_attributes.subsections.board_attributes;

    test('shows Board Attributes when the IntegratedBoards feature flag is enabled', () => {
        expect(isHidden(settingsSubsection, boardsFlagEnabled)).toBe(false);
    });

    test('hides Board Attributes when the IntegratedBoards feature flag is disabled', () => {
        expect(isHidden(settingsSubsection, boardsFlagDisabled)).toBe(true);
    });
});
