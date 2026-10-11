// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {GlobalState} from 'types/store';

import {getOneClickEmoji} from './index';

describe('fusion/selectors/getOneClickEmoji', () => {
    const state = (recent: Array<{name: string; usageCount: number}>) => ({
        entities: {
            general: {config: {}},
            users: {currentUserId: 'me'},
            emojis: {customEmoji: {}},
            preferences: {myPreferences: recent.length ? {'recent_emojis--me': {category: 'recent_emojis', name: 'me', user_id: 'me', value: JSON.stringify(recent)}} : {}},
        },
    }) as unknown as GlobalState;

    test('gives the defaults to someone who never reacted', () => {
        expect(getOneClickEmoji(state([]))).toEqual(['+1', 'heart', 'joy']);
    });

    test('gives the emoji used most first, the defaults filling in', () => {
        expect(getOneClickEmoji(state([{name: 'tada', usageCount: 2}, {name: 'eyes', usageCount: 5}]))).toEqual(['eyes', 'tada', '+1']);
    });

    test('doesn\'t repeat a default already used', () => {
        expect(getOneClickEmoji(state([{name: 'heart', usageCount: 3}]))).toEqual(['heart', '+1', 'joy']);
    });
});
