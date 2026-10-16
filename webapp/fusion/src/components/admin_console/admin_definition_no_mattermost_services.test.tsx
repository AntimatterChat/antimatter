// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {DocLinks} from 'utils/constants';

import AdminDefinition from './admin_definition';

// Antimatter never contacts Mattermost, Inc. (72e3ea7af5, 690a393607): the System Console offers no setting that
// would, and the push notification page only knows your own push proxy.

type Page = {url?: string; isHidden?: unknown; schema?: unknown};

// Every setting of the System Console with the page it's on.
function allSettings(): Array<{key: string; page: Page}> {
    const found: Array<{key: string; page: Page}> = [];
    const walk = (node: unknown, page: Page | undefined) => {
        if (!node || typeof node !== 'object') {
            return;
        }
        if (Array.isArray(node)) {
            node.forEach((n) => walk(n, page));
            return;
        }
        const obj = node as Record<string, unknown>;
        const here = typeof obj.url === 'string' && 'schema' in obj ? obj as Page : page;
        if (page && typeof obj.key === 'string' && typeof obj.type === 'string') {
            found.push({key: obj.key, page});
        }
        for (const [name, value] of Object.entries(obj)) {
            if (name !== 'help_text_values' && typeof value === 'object') {
                walk(value, here);
            }
        }
    };
    walk(AdminDefinition, undefined);
    return found;
}

const keys = () => allSettings().map((s) => s.key);

describe('AdminDefinition: nothing that calls Mattermost, Inc.', () => {
    test('finds the console\'s settings', () => {
        expect(keys()).toContain('EmailSettings.PushNotificationContents');
        expect(keys().length).toBeGreaterThan(200);
    });

    test.each([
        'LogSettings.EnableDiagnostics',
        'ServiceSettings.EnableSecurityFixAlert',
    ])('has no %s setting', (key) => {
        expect(keys()).not.toContain(key);
    });

    test('the notices settings are only on a hidden page', () => {
        const notices = allSettings().filter((s) => s.key === 'AnnouncementSettings.AdminNoticesEnabled' || s.key === 'AnnouncementSettings.UserNoticesEnabled');
        expect(notices).toHaveLength(2);
        for (const {page} of notices) {
            expect(page.isHidden).toBe(true);
        }
        expect(AdminDefinition.site.subsections.notices.isHidden).toBe(true);
    });

    test('no Mattermost push service or marketplace is named', () => {
        const text = JSON.stringify(AdminDefinition);
        expect(text).not.toMatch(/push(-test)?\.mattermost\.com/);
        expect(text).not.toMatch(/api\.integrations\.mattermost\.com/);
        expect(text).not.toContain('mtpns');
    });

    test('the push notification page documents your own push proxy', () => {
        expect(AdminDefinition.environment.subsections.push_notification_server.url).toBe('environment/push_notification_server');
        expect(DocLinks.SETUP_PUSH_NOTIFICATIONS).toBe('https://github.com/AntimatterChat/push-proxy#self-hosting');
    });
});
