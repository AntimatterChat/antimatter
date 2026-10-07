// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {removePost} from 'mattermost-redux/actions/posts';

import {createPost} from 'actions/post_actions';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import FailedPost from './failed_post';

jest.mock('actions/post_actions', () => ({createPost: jest.fn(() => ({type: 'CREATE'}))}));
jest.mock('mattermost-redux/actions/posts', () => ({removePost: jest.fn(() => ({type: 'REMOVE'}))}));

describe('fusion/messages/FailedPost', () => {
    const post = TestHelper.getPostMock({id: 'pending', pending_post_id: 'pending', message: 'hello', failed: true} as never);

    test('sends the message again, without its id', async () => {
        renderWithContext(<FailedPost post={post}/>);

        expect(screen.getByRole('alert')).toHaveTextContent('Not sent');
        await userEvent.click(screen.getByRole('button', {name: 'Retry'}));
        expect(createPost).toHaveBeenCalledWith(expect.not.objectContaining({id: expect.anything()}), []);
        expect((createPost as jest.Mock).mock.calls[0][0]).toMatchObject({message: 'hello', pending_post_id: 'pending'});
    });

    test('drops the message', async () => {
        renderWithContext(<FailedPost post={post}/>);

        await userEvent.click(screen.getByRole('button', {name: 'Cancel'}));
        expect(removePost).toHaveBeenCalledWith(post);
    });
});
