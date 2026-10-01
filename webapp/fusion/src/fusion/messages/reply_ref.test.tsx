// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Post} from '@mattermost/types/posts';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import ReplyRef from './reply_ref';

describe('fusion/messages/ReplyRef', () => {
    const me = TestHelper.getUserMock({id: 'me', username: 'jason'});
    const author = TestHelper.getUserMock({id: 'author', username: 'marie'});
    const quoted = TestHelper.getPostMock({id: 'quoted', type: '', channel_id: 'channel', user_id: author.id, message: 'Is the **build** green?'});
    const reply = TestHelper.getPostMock({id: 'reply', type: '', channel_id: 'channel', user_id: me.id, message: 'Yes', props: {reply_to: quoted.id}});

    const state = (posts: Record<string, Post>) => ({
        entities: {
            users: {currentUserId: me.id, profiles: {me, author}},
            posts: {posts},
            teams: {currentTeamId: 'team', teams: {team: TestHelper.getTeamMock({id: 'team', name: 'team'})}},
        },
    });

    const renderRef = (post: Post, posts: Record<string, Post>) => renderWithContext(
        <ReplyRef
            post={post}
            quotedId={quoted.id}
            threadReply={false}
        />,
        state(posts),
    );

    test('shows the quoted author\'s picture and name, and the start of their message', () => {
        const {container} = renderRef(reply, {quoted, reply});

        const line = screen.getByRole('button', {name: /Replying to marie:/});
        expect(line).toHaveAttribute('title', 'Jump to the original message');
        expect(line).toBeEnabled();
        expect(container.querySelector('.am-ref-av.am-xs')).not.toBeNull();
        expect(line.querySelector('b')).toHaveTextContent('marie');
        expect(line.querySelector('.am-snip')).toHaveTextContent('Is the build green?');
    });

    test('falls back to the server\'s description of a quoted message that isn\'t loaded', () => {
        const described = {...reply, metadata: {...reply.metadata, reply_to: {post_id: quoted.id, user_id: author.id, message: 'From the server'}}} as Post;
        renderRef(described, {reply: described});

        expect(screen.getByRole('button', {name: /Replying to marie:/}).querySelector('.am-snip')).toHaveTextContent('From the server');
    });

    test('offers to see the attachments of a message without text', () => {
        const files = {...quoted, message: '', file_ids: ['file']};
        renderRef(reply, {quoted: files, reply});

        const snippet = screen.getByRole('button', {name: /Replying to marie:/}).querySelector('.am-snip');
        expect(snippet).toHaveTextContent('Click to see attachment');
        expect(snippet).toHaveClass('am-files');
        expect(snippet?.querySelector('svg')).not.toBeNull();
    });

    test('says when the quoted message was deleted, and can\'t jump to it', () => {
        const {container} = renderRef(reply, {quoted: {...quoted, delete_at: 1}, reply});

        const line = screen.getByRole('button', {name: /Replying to:/});
        expect(line).toBeDisabled();
        expect(line.querySelector('b')).toBeNull();
        expect(container.querySelector('.am-ref-av.am-none')).not.toBeNull();
        expect(line.querySelector('.am-snip')).toHaveClass('am-gone');
        expect(line.querySelector('.am-snip')).toHaveTextContent('Original message was deleted');
    });
});
