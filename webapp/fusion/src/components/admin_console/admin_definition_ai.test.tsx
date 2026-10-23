// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {AdminConfig} from '@mattermost/types/config';

import {RESOURCE_KEYS} from 'mattermost-redux/constants/permissions_sysconsole';

import store from 'stores/redux_store';

import AdminDefinition from './admin_definition';
import {MOVED_CONSOLE_ROUTES, listedUnderPlugins} from './moved_console_routes';
import type {AdminDefinitionSection, Check, ConsoleAccess} from './types';

let mockManagesAIConnections = false;
jest.mock('mattermost-redux/selectors/entities/roles', () => ({
    ...jest.requireActual('mattermost-redux/selectors/entities/roles'),
    haveISystemPermission: (_state: unknown, {permission}: {permission: string}) => permission === 'manage_ai_connections' && mockManagesAIConnections,
}));

// The apps the server has (System Console › Plugins), for the AI chat app's page.
function withPlugins(plugins: Record<string, unknown>) {
    const real = store.getState();
    jest.spyOn(store, 'getState').mockReturnValue({...real, entities: {...real.entities, admin: {...real.entities.admin, plugins}}} as never);
}

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
// features, recaps, text to speech and the AI chat app's own settings.
describe('AdminDefinition - AI', () => {
    beforeEach(() => {
        mockManagesAIConnections = false;
    });

    afterEach(() => {
        jest.restoreAllMocks();
    });

    test('has every AI page, and only there', () => {
        const ai = AdminDefinition.ai.subsections;
        expect(Object.values(ai).map((sub) => sub.url)).toEqual(['ai/model_connections', 'ai/builtin_features', 'ai/recaps', 'ai/text_to_speech', 'ai/ai_chat_app']);
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
            'plugins/plugin_mattermost-ai': 'ai/ai_chat_app',
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

    // The AI chat app's own settings page (its services, embeddings, web search, MCP) is under AI, not Plugins.
    test('the AI chat app\'s page: under AI for who may see apps\' settings, when the app is there', () => {
        const page = AdminDefinition.ai.subsections.ai_chat_app;
        const pluginsOn = {PluginSettings: {Enable: true}} as Partial<AdminConfig>;
        const pluginsRead = access('plugins');

        expect(hidden(AdminDefinition.ai.isHidden, pluginsRead)).toBe(false);
        expect(hidden(page.isHidden, access(), pluginsOn)).toBe(true);
        expect(hidden(page.isHidden, pluginsRead, {PluginSettings: {Enable: false}} as Partial<AdminConfig>)).toBe(true);

        // Before the apps are loaded, and with the app: shown.
        withPlugins({});
        expect(hidden(page.isHidden, pluginsRead, pluginsOn)).toBe(false);
        withPlugins({'mattermost-ai': {id: 'mattermost-ai'}, playbooks: {id: 'playbooks'}});
        expect(hidden(page.isHidden, pluginsRead, pluginsOn)).toBe(false);

        // The apps are loaded, without the AI chat app: hidden.
        withPlugins({playbooks: {id: 'playbooks'}});
        expect(hidden(page.isHidden, pluginsRead, pluginsOn)).toBe(true);
    });

    test('the sidebar\'s Plugins list leaves the AI chat app out, and only it', () => {
        expect(listedUnderPlugins('mattermost-ai')).toBe(false);
        expect(listedUnderPlugins('playbooks')).toBe(true);
        expect(listedUnderPlugins('focalboard')).toBe(true);
    });
});
