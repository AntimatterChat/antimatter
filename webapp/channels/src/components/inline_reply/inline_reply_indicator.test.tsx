// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import InlineReplyIndicator from './inline_reply_indicator';

describe('components/inline_reply/InlineReplyIndicator', () => {
    const me = TestHelper.getUserMock({id: 'me', username: 'jason'});
    const author = TestHelper.getUserMock({id: 'author', username: 'marie'});
    const quoted = TestHelper.getPostMock({id: 'quoted', user_id: author.id, message: 'the **original** message'});
    const mine = TestHelper.getPostMock({id: 'mine', user_id: me.id, message: 'my message'});
    const hook = TestHelper.getPostMock({id: 'hook', user_id: author.id, message: 'build passed', props: {from_webhook: 'true'}});
    const state = {entities: {posts: {posts: {quoted, mine, hook}}, users: {currentUserId: me.id, profiles: {me, author}}}};

    test('shows the message being replied to above the message box', () => {
        const onCancel = jest.fn();
        renderWithContext(
            <InlineReplyIndicator
                postId={quoted.id}
                onCancel={onCancel}
                mention={true}
                onMentionChange={jest.fn()}
            />,
            state,
        );

        const indicator = screen.getByTestId('inline-reply-indicator');
        expect(indicator.querySelector('.InlineReplyQuote__label')).toHaveTextContent('Replying to marie');
        expect(indicator.querySelector('.InlineReplyQuote__name')).toHaveTextContent('marie');
        expect(indicator.querySelector('.InlineReplyQuote__snippet')).toHaveTextContent('the original message');
        screen.getByRole('button', {name: 'Cancel reply'}).click();
        expect(onCancel).toHaveBeenCalled();
    });

    test('turns the mention of the quoted author off and on', () => {
        const onMentionChange = jest.fn();
        const {rerender} = renderWithContext(
            <InlineReplyIndicator
                postId={quoted.id}
                onCancel={jest.fn()}
                mention={true}
                onMentionChange={onMentionChange}
            />,
            state,
        );

        const on = screen.getByRole('button', {name: 'Mention marie'});
        expect(on).toHaveTextContent('@ On');
        on.click();
        expect(onMentionChange).toHaveBeenLastCalledWith(false);

        rerender(
            <InlineReplyIndicator
                postId={quoted.id}
                onCancel={jest.fn()}
                mention={false}
                onMentionChange={onMentionChange}
            />,
        );
        const off = screen.getByRole('button', {name: "Don't mention marie"});
        expect(off).toHaveTextContent('@ Off');
        off.click();
        expect(onMentionChange).toHaveBeenLastCalledWith(true);
    });

    test.each([
        ['your own message', mine.id],
        ['a webhook message', hook.id],
    ])('has no mention switch when replying to %s', (_, postId) => {
        renderWithContext(
            <InlineReplyIndicator
                postId={postId}
                onCancel={jest.fn()}
                mention={true}
                onMentionChange={jest.fn()}
            />,
            state,
        );

        expect(screen.queryByText('@ On')).toBeNull();
        expect(screen.getByRole('button', {name: 'Cancel reply'})).toBeInTheDocument();
    });
});
