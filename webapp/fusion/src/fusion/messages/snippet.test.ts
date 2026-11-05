// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {Posts} from 'mattermost-redux/constants';

import {coverOf, snippetTitle} from './snippet';

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
        expect(coverOf({user_id: 'bob', props: {spoiler: true}, state: Posts.POST_DELETED as 'DELETED'}, 'me', lists, true)).toBe('');
    });

    test('blocking turned off by the admin hides nothing for it', () => {
        expect(coverOf({user_id: 'bad'}, 'me', lists, false)).toBe('');
    });
});

// audit-1104 fusion-core-4: the titles of threads and topics in lists.
describe('fusion/messages/snippet: snippetTitle', () => {
    test('a name, else the text, when nothing covers it', () => {
        expect(snippetTitle('Release 2.1', 'Notes', '', '')).toBe('Release 2.1');
        expect(snippetTitle('', 'Notes', '', '')).toBe('Notes');
    });

    test('a spoiler keeps its name but not its text', () => {
        expect(snippetTitle('Season finale', 'The ending', 'spoiler', 'Spoiler')).toBe('Season finale');
        expect(snippetTitle('', 'The ending', 'spoiler', 'Spoiler')).toBe('Spoiler');
    });

    test('someone you ignore or block shows neither', () => {
        expect(snippetTitle('Insults', 'You all are wrong', 'block', 'Message from someone you blocked')).toBe('Message from someone you blocked');
        expect(snippetTitle('', 'You all are wrong', 'ignore', 'Message from someone you ignore')).toBe('Message from someone you ignore');
    });
});
