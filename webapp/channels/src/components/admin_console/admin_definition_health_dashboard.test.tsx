// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {AdminConfig} from '@mattermost/types/config';

import {RESOURCE_KEYS} from 'mattermost-redux/constants/permissions_sysconsole';

import AdminDefinition from './admin_definition';
import type {Check, ConsoleAccess} from './types';

const reportingAccess = {
    read: {[RESOURCE_KEYS.REPORTING.SITE_STATISTICS]: true},
    write: {[RESOURCE_KEYS.REPORTING.SITE_STATISTICS]: true},
} as ConsoleAccess;
const noAccess = {read: {}, write: {}} as ConsoleAccess;

const reporting = AdminDefinition.reporting.subsections;
type ReportingKey = keyof typeof reporting;

function isHidden(key: ReportingKey, isSystemAdmin = true, flag = true, access = reportingAccess) {
    const check: Check | undefined = reporting[key].isHidden;
    if (typeof check !== 'function') {
        return Boolean(check);
    }
    const config: Partial<AdminConfig> = {FeatureFlags: {HealthDashboard: flag}};
    return check(config, {}, access, isSystemAdmin);
}

describe('AdminDefinition - Site health', () => {
    test('is registered under Reporting at reporting/site_health', () => {
        expect(reporting.site_health.url).toBe('reporting/site_health');
    });

    test('is shown to a system admin with the feature flag on', () => {
        expect(isHidden('site_health')).toBe(false);
    });

    test('needs no console access beyond being a system admin', () => {
        expect(isHidden('site_health', true, true, noAccess)).toBe(false);
    });

    test('is hidden when the feature flag is off', () => {
        expect(isHidden('site_health', true, false)).toBe(true);
    });

    test('is hidden from anyone who is not a system admin', () => {
        expect(isHidden('site_health', false)).toBe(true);
    });
});

describe('AdminDefinition - Site health or Workspace Optimization, never both', () => {
    test('flag on shows Site health and hides Workspace Optimization', () => {
        expect(isHidden('site_health')).toBe(false);
        expect(isHidden('workspace_optimization')).toBe(true);
    });

    test('flag off shows Workspace Optimization and hides Site health', () => {
        expect(isHidden('workspace_optimization', true, false)).toBe(false);
        expect(isHidden('site_health', true, false)).toBe(true);
    });

    test('flag on keeps Workspace Optimization for delegated admins who cannot see Site health', () => {
        expect(isHidden('site_health', false)).toBe(true);
        expect(isHidden('workspace_optimization', false)).toBe(false);
    });

    test('Workspace Optimization still needs read access to site statistics', () => {
        expect(isHidden('workspace_optimization', false, false, noAccess)).toBe(true);
    });

    test('never shows both dashboards', () => {
        for (const flag of [true, false]) {
            for (const isSystemAdmin of [true, false]) {
                for (const access of [reportingAccess, noAccess]) {
                    const visible = (Object.keys(reporting) as ReportingKey[]).filter((key) => !isHidden(key, isSystemAdmin, flag, access));
                    expect(visible.includes('site_health') && visible.includes('workspace_optimization')).toBe(false);
                }
            }
        }
    });
});
