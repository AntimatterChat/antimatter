// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {AdminConfig} from '@mattermost/types/config';

import AdminDefinition from './admin_definition';
import type {Check, ConsoleAccess} from './types';

const fullAccess = {read: {}, write: {}} as ConsoleAccess;

function isHidden(isSystemAdmin: boolean, flag = true) {
    const check = AdminDefinition.reporting.subsections.site_health.isHidden as Exclude<Check, boolean>;
    const config: Partial<AdminConfig> = {FeatureFlags: {HealthDashboard: flag}};
    return check(config, {}, fullAccess, isSystemAdmin);
}

describe('AdminDefinition - Site health', () => {
    test('is registered under Reporting at reporting/site_health', () => {
        expect(AdminDefinition.reporting.subsections.site_health.url).toBe('reporting/site_health');
    });

    test('is shown to a system admin with the feature flag on', () => {
        expect(isHidden(true)).toBe(false);
    });

    test('is hidden when the feature flag is off', () => {
        expect(isHidden(true, false)).toBe(true);
    });

    test('is hidden from anyone who is not a system admin', () => {
        expect(isHidden(false)).toBe(true);
    });
});
