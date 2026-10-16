// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import FoldedMessage from './folded_message';

jest.mock('fusion/messages/time', () => () => null);

describe('fusion/privacy/FoldedMessage', () => {
    const post = TestHelper.getPostMock({id: 'p1', user_id: 'sofia', message: 'secret words', create_at: 1});

    test('folds a blocked person\'s message without naming them, and shows it on demand', async () => {
        const onShow = jest.fn();
        renderWithContext(
            <FoldedMessage
                post={post}
                level='block'
                onShow={onShow}
            />,
        );
        expect(screen.getByText(/Message from someone you blocked/)).toBeInTheDocument();
        expect(screen.queryByText('secret words')).not.toBeInTheDocument();
        expect(screen.queryByText(/sofia/)).not.toBeInTheDocument();

        await userEvent.click(screen.getByRole('button', {name: 'Show'}));
        expect(onShow).toHaveBeenCalled();
    });

    test('says when it is someone you ignore', () => {
        renderWithContext(
            <FoldedMessage
                post={post}
                level='ignore'
                onShow={jest.fn()}
            />,
        );
        expect(screen.getByText(/Message from someone you ignore/)).toBeInTheDocument();
    });
});
