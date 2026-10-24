// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {toggleReaction} from 'actions/post_actions';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import VoteButton from './vote_button';

jest.mock('actions/post_actions', () => ({toggleReaction: jest.fn(() => () => Promise.resolve({data: true}))}));

describe('fusion/forum/VoteButton', () => {
    const post = TestHelper.getPostMock({id: 'p1', user_id: 'ada'});
    const voter = TestHelper.getReactionMock({user_id: 'maya', emoji_name: '+1'});

    const render = (currentUserId: string, reactions: Record<string, unknown> = {}) => renderWithContext(
        <VoteButton post={post}/>,
        {
            entities: {
                users: {currentUserId},
                posts: {reactions: {p1: reactions}},
            },
        } as never,
    );

    test('shows the vote count, excluding the author\'s own', () => {
        render('maya', {[`${voter.user_id}-${voter.emoji_name}`]: voter});

        expect(screen.getByRole('button', {name: 'Remove your vote'})).toHaveTextContent('1');
    });

    test('hidden on your own post', () => {
        render('ada');

        expect(screen.queryByRole('button')).not.toBeInTheDocument();
    });

    test('toggles the +1 reaction when clicked', async () => {
        render('maya');

        await userEvent.click(screen.getByRole('button', {name: 'Vote for this'}));

        expect(toggleReaction).toHaveBeenCalledWith('p1', '+1');
    });
});
