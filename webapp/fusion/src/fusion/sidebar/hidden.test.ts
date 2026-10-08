// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Channel} from '@mattermost/types/channels';
import type {PreferencesType} from '@mattermost/types/preferences';

import {CATEGORY_HIDDEN, hiddenSince} from './hidden';

const channel = (id: string, lastPostAt: number) => ({id, last_post_at: lastPostAt}) as Channel;
const pref = (name: string, value: number) => ({[`${CATEGORY_HIDDEN}--${name}`]: {user_id: 'me', category: CATEGORY_HIDDEN, name, value: String(value)}});

describe('hiddenSince', () => {
    const channels = {quiet: channel('quiet', 100), busy: channel('busy', 300)};

    test('keeps a conversation hidden while nothing new is said in it', () => {
        const preferences: PreferencesType = {...pref('quiet', 200)};
        expect([...hiddenSince(preferences, channels).keys()]).toEqual(['quiet']);
    });

    test('shows a conversation again once it has a newer message', () => {
        const preferences: PreferencesType = {...pref('busy', 200)};
        expect(hiddenSince(preferences, channels).size).toBe(0);
    });

    test('ignores other preferences and unknown channels', () => {
        const preferences: PreferencesType = {...pref('gone', 200), 'other--quiet': {user_id: 'me', category: 'other', name: 'quiet', value: '500'}};
        expect(hiddenSince(preferences, channels).size).toBe(0);
    });
});
