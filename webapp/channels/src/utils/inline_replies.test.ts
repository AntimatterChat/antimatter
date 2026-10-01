// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {Posts} from 'mattermost-redux/constants';

import {TestHelper} from 'utils/test_helper';

import type {GlobalState} from 'types/store';

import {canReplyInline, getQuotedMessage, getReplyProps, getReplyToId, mentionsQuotedAuthor, replyInline, INLINE_REPLY_EVENT} from './inline_replies';

describe('utils/inline_replies', () => {
    test('getReplyToId reads the reply_to prop', () => {
        expect(getReplyToId(TestHelper.getPostMock({props: {reply_to: 'quoted'}}))).toBe('quoted');
        expect(getReplyToId(TestHelper.getPostMock({props: {}}))).toBe('');
        expect(getReplyToId(TestHelper.getPostMock({props: {reply_to: 42}}))).toBe('');
        expect(getReplyToId(undefined)).toBe('');
    });

    test('mentionsQuotedAuthor is true unless reply_to_mention is false', () => {
        expect(mentionsQuotedAuthor(TestHelper.getPostMock({props: {reply_to: 'quoted'}}))).toBe(true);
        expect(mentionsQuotedAuthor(TestHelper.getPostMock({props: {reply_to: 'quoted', reply_to_mention: true}}))).toBe(true);
        expect(mentionsQuotedAuthor(TestHelper.getPostMock({props: {reply_to: 'quoted', reply_to_mention: false}}))).toBe(false);
        expect(mentionsQuotedAuthor(undefined)).toBe(true);
    });

    test('getReplyProps sets the quoted message and whether its author is mentioned', () => {
        expect(getReplyProps({other: 1}, 'quoted', true)).toEqual({other: 1, reply_to: 'quoted'});
        expect(getReplyProps({other: 1}, 'quoted', false)).toEqual({other: 1, reply_to: 'quoted', reply_to_mention: false});
        expect(getReplyProps({reply_to: 'quoted', reply_to_mention: false}, 'other', true)).toEqual({reply_to: 'other'});
        expect(getReplyProps({other: 1, reply_to: 'quoted', reply_to_mention: false}, '', false)).toEqual({other: 1});
        expect(getReplyProps(undefined, 'quoted', false)).toEqual({reply_to: 'quoted', reply_to_mention: false});
    });

    test.each([
        ['a message', {}, true],
        ['a system message', {type: Posts.POST_TYPES.JOIN_CHANNEL}, false],
        ['a burn-on-read message', {type: Posts.POST_TYPES.BURN_ON_READ}, false],
        ['a deleted message', {state: 'DELETED'}, false],
        ['a message being sent', {id: 'pending', pending_post_id: 'pending'}, false],
        ['a message that failed', {failed: true}, false],
    ] as const)('canReplyInline: %s', (_, patch, expected) => {
        expect(canReplyInline(TestHelper.getPostMock({id: 'post', type: '', ...patch}))).toBe(expected);
    });

    test('replyInline asks the conversation composer to quote the message', () => {
        const listener = jest.fn();
        window.addEventListener(INLINE_REPLY_EVENT, listener);
        replyInline(TestHelper.getPostMock({id: 'post', channel_id: 'channel'}), 'root');
        window.removeEventListener(INLINE_REPLY_EVENT, listener);

        expect(listener.mock.calls[0][0].detail).toEqual({channelId: 'channel', rootId: 'root', postId: 'post'});
    });

    describe('getQuotedMessage', () => {
        const quoted = TestHelper.getPostMock({id: 'quoted', user_id: 'author', message: 'hello', file_ids: ['f1']});
        const state = (posts: Record<string, unknown>) => ({entities: {posts: {posts}}}) as unknown as GlobalState;

        test('prefers the loaded message', () => {
            expect(getQuotedMessage(state({quoted}), 'quoted', {post_id: 'quoted', message: 'stale'})).toEqual({
                deleted: false,
                userId: 'author',
                message: 'hello',
                fileCount: 1,
                overrideUsername: undefined,
                fromWebhook: false,
            });
        });

        test("falls back to the server's description", () => {
            expect(getQuotedMessage(state({}), 'quoted', {post_id: 'quoted', user_id: 'author', message: 'described', file_count: 2})).toEqual({
                deleted: false,
                userId: 'author',
                message: 'described',
                fileCount: 2,
                overrideUsername: undefined,
            });
        });

        test('reports deleted messages', () => {
            expect(getQuotedMessage(state({quoted: {...quoted, state: Posts.POST_DELETED}}), 'quoted').deleted).toBe(true);
            expect(getQuotedMessage(state({}), 'quoted', {post_id: 'quoted', deleted: true}).deleted).toBe(true);
            expect(getQuotedMessage(state({}), 'quoted').deleted).toBe(false);
        });

        test('names the webhook a message was posted under', () => {
            const hook = {...quoted, props: {from_webhook: 'true', override_username: 'ci'}};
            expect(getQuotedMessage(state({quoted: hook}), 'quoted')).toMatchObject({overrideUsername: 'ci', fromWebhook: true});
        });
    });
});
