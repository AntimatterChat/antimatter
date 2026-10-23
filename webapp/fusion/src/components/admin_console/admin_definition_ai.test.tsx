// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {AdminConfig} from '@mattermost/types/config';

import {RESOURCE_KEYS} from 'mattermost-redux/constants/permissions_sysconsole';

import AdminDefinition from './admin_definition';
import {MOVED_CONSOLE_ROUTES} from './moved_console_routes';
import type {AdminDefinitionSection, Check, ConsoleAccess} from './types';

let mockManagesAIConnections = false;
jest.mock('mattermost-redux/selectors/entities/roles', () => ({
    ...jest.requireActual('mattermost-redux/selectors/entities/roles'),
    haveISystemPermission: (_state: unknown, {permission}: {permission: string}) => permission === 'manage_ai_connections' && mockManagesAIConnections,
}));

const access = (...read: string[]) => ({read: Object.fromEntries(read.map((key) => [key, true])), write: {}} as ConsoleAccess);

function hidden(check: Check | undefined, consoleAccess: ConsoleAccess, config: Partial<AdminConfig> = {}) {
    if (typeof check !== 'function') {
        return Boolean(check);
    }
    return check(config, {}, consoleAccess);
}

const sections = Object.values(AdminDefinition) as AdminDefinitionSection[];
const urls = sections.flatMap((section) => Object.values(section.subsections).map((sub) => sub.url));

// Every AI setting is in the System Console's AI section: model connections and agents on one page, the built-in
// features, recaps and text to speech.
describe('AdminDefinition - AI', () => {
    beforeEach(() => {
        mockManagesAIConnections = false;
    });

    test('has every AI page, and only there', () => {
        const ai = AdminDefinition.ai.subsections;
        expect(Object.values(ai).map((sub) => sub.url)).toEqual(['ai/model_connections', 'ai/builtin_features', 'ai/recaps', 'ai/text_to_speech']);
        expect(urls.filter((url) => url.endsWith('/recaps') || url.endsWith('/text_to_speech'))).toEqual(['ai/recaps', 'ai/text_to_speech']);
        expect((AdminDefinition.site.subsections as Record<string, unknown>).recaps).toBeUndefined();
        expect((AdminDefinition.integrations.subsections as Record<string, unknown>).text_to_speech).toBeUndefined();
    });

    test('the old addresses of the moved pages open the new ones', () => {
        for (const [from, to] of Object.entries(MOVED_CONSOLE_ROUTES)) {
            expect(urls).toContain(to);
            expect(urls).not.toContain(from);
        }
        expect(MOVED_CONSOLE_ROUTES).toEqual({
            'site_config/recaps': 'ai/recaps',
            'integrations/text_to_speech': 'ai/text_to_speech',
        });
    });

    test('the section shows for whoever may see one of its pages, and each page for whoever may see it', () => {
        const section = AdminDefinition.ai.isHidden;
        const {model_connections: connections, builtin_features: features, recaps, text_to_speech: speech} = AdminDefinition.ai.subsections;
        const recapsOn = {FeatureFlags: {EnableAIRecaps: true}} as Partial<AdminConfig>;

        expect(hidden(section, access())).toBe(true);

        // Recaps' limits alone.
        const recapsOnly = access(RESOURCE_KEYS.SITE.AI_RECAPS);
        expect(hidden(section, recapsOnly)).toBe(false);
        expect(hidden(recaps.isHidden, recapsOnly, recapsOn)).toBe(false);
        expect(hidden(connections.isHidden, recapsOnly)).toBe(true);
        expect(hidden(features.isHidden, recapsOnly)).toBe(true);
        expect(hidden(speech.isHidden, recapsOnly)).toBe(true);

        // Integrations: text to speech.
        const integrations = access(RESOURCE_KEYS.INTEGRATIONS.INTEGRATION_MANAGEMENT);
        expect(hidden(section, integrations)).toBe(false);
        expect(hidden(speech.isHidden, integrations)).toBe(false);
        expect(hidden(connections.isHidden, integrations)).toBe(true);

        // Who manages the server's connections: the connections and agents, and the built-in features.
        mockManagesAIConnections = true;
        expect(hidden(section, access())).toBe(false);
        expect(hidden(connections.isHidden, access())).toBe(false);
        expect(hidden(features.isHidden, access())).toBe(false);
    });
});
