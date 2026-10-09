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

jest.mock('mattermost-redux/actions/preferences', () => ({
    ...jest.requireActual('mattermost-redux/actions/preferences'),
    savePreferences: (userId: string, preferences: unknown[]) => ({type: 'SAVE', userId, preferences}),
}));

describe('hideConversation', () => {
    test('keeps the time of the last message by the server\'s clock, not this device\'s', async () => {
        const {hideConversation} = jest.requireActual('./hidden');
        const saved: unknown[] = [];
        const state = {entities: {users: {currentUserId: 'me'}, channels: {channels: {dm: {id: 'dm', last_post_at: 1000}}}}};
        const dispatch = jest.fn((action: unknown) => {
            saved.push(action);
            return Promise.resolve({data: true});
        });
        jest.spyOn(Date, 'now').mockReturnValue(999999);
        await hideConversation('dm')(dispatch, () => state, undefined);
        expect(JSON.stringify(saved)).toContain('"value":"1000"');
        jest.restoreAllMocks();
    });
});
