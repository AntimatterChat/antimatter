// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import InlineReplyQuote, {InlineReplySpine} from './inline_reply_quote';

describe('components/inline_reply/InlineReplyQuote', () => {
    const author = TestHelper.getUserMock({id: 'author', username: 'marie'});
    const quoted = TestHelper.getPostMock({id: 'quoted', user_id: author.id, message: 'the **original** message'});
    const reply = TestHelper.getPostMock({id: 'reply', props: {reply_to: quoted.id}});
    const users = {profiles: {author}};

    test('quotes the loaded message with its author\'s picture and name', () => {
        renderWithContext(
            <InlineReplyQuote post={reply}/>,
            {entities: {posts: {posts: {quoted}}, users}},
        );

        const line = screen.getByRole('button', {name: 'Replying to marie the original message'});
        expect(line).toHaveAttribute('title', 'Jump to the original message');
        expect(line.querySelector('img.Avatar-xxs')).toBeInTheDocument();
        expect(line.querySelector('.InlineReplyQuote__name')).toHaveTextContent('marie');
        expect(line.querySelector('.InlineReplyQuote__snippet')).toHaveTextContent('the original message');
        expect(line.querySelector('.InlineReplyQuote__icon')).toBeNull();
    });

    test("quotes the server's description of a message that isn't loaded", () => {
        const described = {...reply, metadata: {...reply.metadata, reply_to: {post_id: quoted.id, user_id: author.id, message: 'as described'}}};
        renderWithContext(
            <InlineReplyQuote post={described}/>,
            {entities: {users}},
        );

        expect(screen.getByRole('button', {name: 'Replying to marie as described'})).toBeInTheDocument();
    });

    test('offers to see the attachments of a message without text', () => {
        renderWithContext(
            <InlineReplyQuote post={reply}/>,
            {entities: {posts: {posts: {quoted: {...quoted, message: '', file_ids: ['file']}}}, users}},
        );

        const snippet = screen.getByText('Click to see attachment');
        expect(snippet).toHaveClass('InlineReplyQuote__snippet--files');
        expect(snippet.querySelector('svg')).toBeInTheDocument();
    });

    test('says when the quoted message was deleted', () => {
        renderWithContext(
            <InlineReplyQuote post={reply}/>,
            {entities: {posts: {posts: {quoted: {...quoted, state: 'DELETED' as const}}}, users}},
        );

        expect(screen.getByText('Original message was deleted')).toHaveClass('InlineReplyQuote__snippet--deleted');
        expect(screen.queryByRole('button')).toBeNull();
        expect(screen.getByTestId('post-inline-reply').querySelector('.InlineReplyQuote__avatar--none')).toBeInTheDocument();
    });

    test('starts with a reply arrow in the compact display, which has no spine', () => {
        renderWithContext(
            <InlineReplyQuote
                post={reply}
                compact={true}
            />,
            {entities: {posts: {posts: {quoted}}, users}},
        );

        expect(screen.getByTestId('post-inline-reply')).toHaveClass('InlineReplyQuote--compact');
        expect(screen.getByTestId('post-inline-reply').querySelector('.InlineReplyQuote__icon')).toBeInTheDocument();
    });

    test('shows nothing when inline replies are off', () => {
        const {container} = renderWithContext(
            <InlineReplyQuote post={reply}/>,
            {entities: {posts: {posts: {quoted}}, users, general: {config: {EnableInlineReplies: 'false'}}}},
        );

        expect(container).toBeEmptyDOMElement();
    });

    test('the spine is hidden from screen readers', () => {
        const {container} = renderWithContext(<InlineReplySpine/>);

        expect(container.firstChild).toHaveClass('InlineReplyQuote__spine');
        expect(container.firstChild).toHaveAttribute('aria-hidden', 'true');
    });
});
