// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Post} from '@mattermost/types/posts';

import {TestHelper} from 'utils/test_helper';

import type {GlobalState} from 'types/store';

import {concernsMe, mentions} from './mentions';

describe('fusion/messages/mentions', () => {
    const keys = [{key: '@marie'}, {key: '@here'}, {key: 'Marie', caseSensitive: true}, {key: 'deploy'}];

    test.each([
        ['hi @marie', true],
        ['@MARIE, look', true],
        ['thanks @marie.', true],
        ['@here the build is red', true],
        ['ask Marie', true],
        ['ask marie', false],
        ['@marie.dupont is away', false],
        ['@mariea', false],
        ['write to marie@example.com', false],
        ['the deploy failed', true],
        ['redeploy it', false],
        ['`@marie` in code', false],
        ['```\n@here\n```', false],
        ['', false],
    ])('%j mentions: %s', (message, expected) => {
        expect(mentions(message, keys)).toBe(expected);
    });

    describe('concernsMe, for inline replies', () => {
        const me = TestHelper.getUserMock({id: 'me', username: 'jason'});
        const mine = TestHelper.getPostMock({id: 'mine', type: '', user_id: 'me', message: 'question'});
        const state = {
            entities: {
                general: {config: {}},
                users: {currentUserId: 'me', profiles: {me}},
                posts: {posts: {mine}},
                preferences: {myPreferences: {}},
            },
        } as unknown as GlobalState;
        const reply = (props: Record<string, unknown>, patch: Partial<Post> = {}) => TestHelper.getPostMock({id: 'reply', type: '', user_id: 'other', message: 'answer', props, ...patch});

        test('highlights a reply to my message', () => {
            expect(concernsMe(state, reply({reply_to: 'mine'}), false)).toBe(true);
            expect(concernsMe(state, reply({reply_to: 'mine', reply_to_mention: true}), false)).toBe(true);
        });

        test('highlights a reply to my message that isn\'t loaded, from its description', () => {
            const described = reply({reply_to: 'gone'}, {metadata: {reply_to: {post_id: 'gone', user_id: 'me'}} as Post['metadata']});
            expect(concernsMe(state, described, false)).toBe(true);
        });

        test('doesn\'t highlight a reply sent without notifying me', () => {
            expect(concernsMe(state, reply({reply_to: 'mine', reply_to_mention: false}), false)).toBe(false);
        });

        test('doesn\'t highlight my own reply, or replies with inline replies turned off', () => {
            expect(concernsMe(state, reply({reply_to: 'mine'}, {user_id: 'me'}), false)).toBe(false);
            const off = {...state, entities: {...state.entities, general: {config: {EnableInlineReplies: 'false'}}}} as unknown as GlobalState;
            expect(concernsMe(off, reply({reply_to: 'mine'}), false)).toBe(false);
        });
    });
});
