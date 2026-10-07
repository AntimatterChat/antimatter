// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import {MentionSwitch} from './inline_reply';

describe('fusion/messages/MentionSwitch', () => {
    const me = TestHelper.getUserMock({id: 'me', username: 'jason'});
    const author = TestHelper.getUserMock({id: 'author', username: 'marie'});
    const state = {entities: {users: {currentUserId: me.id, profiles: {me, author}}}};
    const quoted = {deleted: false, userId: author.id, message: 'hello', fileCount: 0, imageCount: 0};

    test('turns the mention of the quoted author off and on', () => {
        const onChange = jest.fn();
        const {rerender} = renderWithContext(
            <MentionSwitch
                quoted={quoted}
                on={true}
                onChange={onChange}
            />,
            state,
        );

        const button = screen.getByRole('button', {name: 'Mention marie'});
        expect(button).toHaveTextContent('@ On');
        expect(button).toHaveAttribute('title', 'marie will be notified of your reply. Click to turn off.');
        button.click();
        expect(onChange).toHaveBeenLastCalledWith(false);

        rerender(
            <MentionSwitch
                quoted={quoted}
                on={false}
                onChange={onChange}
            />,
        );
        screen.getByRole('button', {name: "Don't mention marie"}).click();
        expect(screen.getByRole('button', {name: "Don't mention marie"})).toHaveTextContent('@ Off');
        expect(onChange).toHaveBeenLastCalledWith(true);
    });

    test.each([
        ['your own message', {...quoted, userId: me.id}],
        ['a deleted message', {deleted: true, message: '', fileCount: 0, imageCount: 0}],
        ['a webhook message', {...quoted, fromWebhook: true}],
    ])('is not shown when replying to %s', (_, q) => {
        renderWithContext(
            <MentionSwitch
                quoted={q}
                on={true}
                onChange={jest.fn()}
            />,
            state,
        );
        expect(screen.queryByRole('button')).toBeNull();
    });
});
