// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Antimatter: System Console pages that moved, from their old address to the new one, so that links in the docs,
// bookmarks and messages still open them. Every AI setting is under AI now.
export const MOVED_CONSOLE_ROUTES: Record<string, string> = {
    'site_config/recaps': 'ai/recaps',
    'integrations/text_to_speech': 'ai/text_to_speech',
};
