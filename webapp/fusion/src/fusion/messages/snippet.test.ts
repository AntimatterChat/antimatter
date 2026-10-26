// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {Posts} from 'mattermost-redux/constants';

import {coverOf} from './snippet';

describe('fusion/messages/snippet: coverOf', () => {
    const lists = {blocked: ['bad'], ignored: ['meh']};

    test('hides the text of someone you block or ignore, before a spoiler', () => {
        expect(coverOf({user_id: 'bad', props: {spoiler: true}}, 'me', lists, true)).toBe('block');
        expect(coverOf({user_id: 'meh'}, 'me', lists, true)).toBe('ignore');
    });

    test('hides a spoiler\'s text', () => {
        expect(coverOf({user_id: 'bob', props: {spoiler: true}}, 'me', lists, true)).toBe('spoiler');
        expect(coverOf({user_id: 'bob', props: {spoiler: 'true'}}, 'me', lists, true)).toBe('spoiler');
    });

    test('leaves other messages, your own and deleted ones alone', () => {
        expect(coverOf({user_id: 'bob'}, 'me', lists, true)).toBe('');
        expect(coverOf({user_id: 'meh'}, 'meh', lists, true)).toBe('');
        expect(coverOf({user_id: 'bob', props: {spoiler: true}, state: Posts.POST_DELETED}, 'me', lists, true)).toBe('');
    });

    test('blocking turned off by the admin hides nothing for it', () => {
        expect(coverOf({user_id: 'bad'}, 'me', lists, false)).toBe('');
    });
});
