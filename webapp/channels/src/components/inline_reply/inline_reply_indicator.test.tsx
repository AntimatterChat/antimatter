// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import InlineReplyIndicator from './inline_reply_indicator';

describe('components/inline_reply/InlineReplyIndicator', () => {
    const author = TestHelper.getUserMock({id: 'author', username: 'marie'});
    const quoted = TestHelper.getPostMock({id: 'quoted', user_id: author.id, message: 'the **original** message'});
    const users = {profiles: {author}};

    test('shows the message being replied to above the message box', () => {
        const onCancel = jest.fn();
        renderWithContext(
            <InlineReplyIndicator
                postId={quoted.id}
                onCancel={onCancel}
            />,
            {entities: {posts: {posts: {quoted}}, users}},
        );

        expect(screen.getByTestId('inline-reply-indicator')).toHaveTextContent('Replying to marie: the original message');
        screen.getByRole('button', {name: 'Cancel reply'}).click();
        expect(onCancel).toHaveBeenCalled();
    });
});
