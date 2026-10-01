// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Post} from '@mattermost/types/posts';

import {replyToId} from './inline_reply';

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
});
