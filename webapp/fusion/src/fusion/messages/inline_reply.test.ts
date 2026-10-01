// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Post} from '@mattermost/types/posts';

import {Posts} from 'mattermost-redux/constants';

import type {GlobalState} from 'types/store';

import {canReplyInline, replyCandidates, replyToId} from './inline_reply';

function post(id: string, patch: Partial<Post> = {}): Post {
    return {id, channel_id: 'channel', root_id: '', create_at: 0, type: '', props: {}, message: id, user_id: 'user', ...patch} as Post;
}

describe('fusion/messages/inline_reply', () => {
    test('replyToId reads the reply_to prop', () => {
        expect(replyToId(post('a', {props: {reply_to: 'b'}}))).toBe('b');
        expect(replyToId(post('a'))).toBe('');
        expect(replyToId(post('a', {props: {reply_to: 42}}))).toBe('');
        expect(replyToId(undefined)).toBe('');
    });

    test.each([
        ['a message', post('a'), true],
        ['a system message', post('a', {type: Posts.POST_TYPES.JOIN_CHANNEL}), false],
        ['a burn-on-read message', post('a', {type: Posts.POST_TYPES.BURN_ON_READ}), false],
        ['a deleted message', post('a', {state: 'DELETED' as const}), false],
        ['a message being sent', post('a', {pending_post_id: 'a'}), false],
        ['a message that failed', post('a', {failed: true}), false],
    ])('canReplyInline: %s', (_, p, expected) => {
        expect(canReplyInline(p)).toBe(expected);
    });

    test('replyCandidates lists the messages that can be quoted, newest first', () => {
        const posts = {
            old: post('old', {create_at: 1}),
            join: post('join', {create_at: 2, type: Posts.POST_TYPES.JOIN_CHANNEL}),
            root: post('root', {create_at: 3}),
            reply1: post('reply1', {create_at: 4, root_id: 'root'}),
            reply2: post('reply2', {create_at: 5, root_id: 'root'}),
            last: post('last', {create_at: 6}),
        };
        const state = {
            entities: {
                posts: {
                    posts,
                    postsInChannel: {channel: [{order: ['last', 'root', 'join', 'old'], recent: true}]},
                    postsInThread: {root: ['reply2', 'reply1']},
                },
            },
        } as unknown as GlobalState;

        expect(replyCandidates(state, 'channel', '')).toEqual(['last', 'root', 'old']);
        expect(replyCandidates(state, 'channel', 'root')).toEqual(['reply2', 'reply1', 'root']);
        expect(replyCandidates(state, 'other', '')).toEqual([]);
    });
});
