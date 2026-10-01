// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import InlineReplyQuote from './inline_reply_quote';

function textInChildren(matchedText: string) {
    return (content: string, element: Element | null) => {
        const hasText = element?.textContent === matchedText;
        const childHasText = element && Array.from(element?.children).some((child) => child?.textContent === matchedText);
        return hasText && !childHasText;
    };
}

describe('components/inline_reply/InlineReplyQuote', () => {
    const author = TestHelper.getUserMock({id: 'author', username: 'marie'});
    const quoted = TestHelper.getPostMock({id: 'quoted', user_id: author.id, message: 'the **original** message'});
    const reply = TestHelper.getPostMock({id: 'reply', props: {reply_to: quoted.id}});
    const users = {profiles: {author}};

    test('quotes the loaded message', () => {
        renderWithContext(
            <InlineReplyQuote post={reply}/>,
            {entities: {posts: {posts: {quoted}}, users}},
        );

        expect(screen.getByText(textInChildren('Replying to marie: the original message'))).toBeInTheDocument();
        expect(screen.getByText('the original message').closest('a')).toBeInTheDocument();
    });

    test("quotes the server's description of a message that isn't loaded", () => {
        const described = {...reply, metadata: {...reply.metadata, reply_to: {post_id: quoted.id, user_id: author.id, message: 'as described'}}};
        renderWithContext(
            <InlineReplyQuote post={described}/>,
            {entities: {users}},
        );

        expect(screen.getByText(textInChildren('Replying to marie: as described'))).toBeInTheDocument();
    });

    test('says when the quoted message was deleted', () => {
        renderWithContext(
            <InlineReplyQuote post={reply}/>,
            {entities: {posts: {posts: {quoted: {...quoted, state: 'DELETED' as const}}}, users}},
        );

        expect(screen.getByText('Original message deleted')).toBeInTheDocument();
        expect(screen.getByText('Original message deleted').closest('a')).toBeNull();
    });

    test('shows nothing when inline replies are off', () => {
        const {container} = renderWithContext(
            <InlineReplyQuote post={reply}/>,
            {entities: {posts: {posts: {quoted}}, users, general: {config: {EnableInlineReplies: 'false'}}}},
        );

        expect(container).toBeEmptyDOMElement();
    });
});
