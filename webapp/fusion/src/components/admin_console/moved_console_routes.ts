// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// The AI chat app, whose own settings page is under AI rather than Plugins.
export const AI_CHAT_APP_ID = 'mattermost-ai';

// Antimatter: System Console pages that moved, from their old address to the new one, so that links in the docs,
// bookmarks and messages still open them. Every AI setting is under AI now.
export const MOVED_CONSOLE_ROUTES: Record<string, string> = {
    'site_config/recaps': 'ai/recaps',
    'integrations/text_to_speech': 'ai/text_to_speech',
    [`plugins/plugin_${AI_CHAT_APP_ID}`]: 'ai/ai_chat_app',
};

// Apps whose settings page has its own place in the console: the sidebar's Plugins list leaves them out.
const APPS_WITH_THEIR_OWN_PAGE = new Set([AI_CHAT_APP_ID]);

export function listedUnderPlugins(pluginId: string): boolean {
    return !APPS_WITH_THEIR_OWN_PAGE.has(pluginId);
}
