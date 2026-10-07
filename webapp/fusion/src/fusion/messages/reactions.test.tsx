// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Reaction} from '@mattermost/types/reactions';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import Reactions from './reactions';

describe('fusion/messages/Reactions', () => {
    const me = TestHelper.getUserMock({id: 'me', username: 'jason'});
    const alice = TestHelper.getUserMock({id: 'alice', username: 'alice'});
    const bob = TestHelper.getUserMock({id: 'bob', username: 'bob'});

    const reaction = (userId: string, emoji: string, at: number): Reaction => ({user_id: userId, post_id: 'post', emoji_name: emoji, create_at: at} as Reaction);

    const render = (reactions: Reaction[], profiles = {me, alice, bob}) => renderWithContext(
        <Reactions postId='post'/>,
        {
            entities: {
                users: {currentUserId: me.id, profiles},
                posts: {
                    posts: {post: TestHelper.getPostMock({id: 'post'})},
                    reactions: {post: Object.fromEntries(reactions.map((r) => [`${r.user_id}-${r.emoji_name}`, r]))},
                },
            },
        },
    );

    test('counts each emoji and tells who reacted with it, you first', () => {
        render([reaction('alice', 'thumbsup', 1), reaction('me', 'thumbsup', 2), reaction('bob', 'thumbsup', 3), reaction('bob', 'heart', 4)]);

        const thumbs = screen.getByRole('button', {name: /:thumbsup: 3/});
        expect(thumbs).toHaveAttribute('aria-pressed', 'true');
        expect(thumbs).toHaveAttribute('data-am-tip', 'You, alice, and bob reacted with :thumbsup:');
        expect(screen.getByRole('button', {name: /:heart: 1/})).toHaveAttribute('data-am-tip', 'bob reacted with :heart:');
    });

    test('counts the people who aren\'t loaded yet as others', () => {
        render([reaction('alice', 'tada', 1), reaction('stranger', 'tada', 2)], {me, alice} as never);

        expect(screen.getByRole('button', {name: /:tada: 2/})).toHaveAttribute('data-am-tip', 'alice and 1 other reacted with :tada:');
    });
});
