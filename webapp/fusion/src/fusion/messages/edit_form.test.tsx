// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Post} from '@mattermost/types/posts';

import {editPost} from 'actions/views/posts';

import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import EditForm from './edit_form';

jest.mock('actions/views/posts', () => ({
    editPost: jest.fn((post: Post) => () => Promise.resolve({data: post})),
}));
jest.mock('actions/post_actions', () => ({
    unsetEditingPost: () => ({type: 'MOCK_UNSET_EDITING_POST'}),
}));
jest.mock('fusion/shell/dialogs_context', () => ({useDialogs: () => ({deleteMessage: jest.fn()})}));

// compat-plugins #3: apps' registerMessageWillBeUpdatedHook (encryption, data loss prevention) was skipped on every
// edit in Fusion: the edited text went to the server as typed, and an app's refusal was ignored.
describe('fusion/messages/EditForm: apps\' update hooks', () => {
    const post = TestHelper.getPostMock({id: 'p1', channel_id: 'c1', user_id: 'me', message: 'old text'});
    const render = (hook?: (newPost: Partial<Post>, oldPost: Post) => unknown) => renderWithContext(
        <EditForm post={post}/>,
        {
            entities: {users: {currentUserId: 'me'}},
            plugins: {components: {MessageWillBeUpdated: hook ? [{id: 'h1', pluginId: 'crypto', hook}] : []}},
        } as never,
    );
    const edit = async (text: string) => {
        const box = screen.getByRole('textbox', {name: 'Edit message'});
        await userEvent.clear(box);
        await userEvent.type(box, `${text}{Enter}`);
    };

    beforeEach(() => {
        jest.mocked(editPost).mockClear();
    });

    test('an app\'s hook changes the edit before it\'s saved', async () => {
        const hook = jest.fn(async (newPost: Partial<Post>) => ({post: {...newPost, message: `ENC(${newPost.message})`}}));
        render(hook);
        await edit('new text');

        await waitFor(() => expect(editPost).toHaveBeenCalled());
        expect(hook).toHaveBeenCalledWith(expect.objectContaining({id: 'p1', channel_id: 'c1', message: 'new text'}), post);
        expect(jest.mocked(editPost).mock.calls[0][0]).toEqual(expect.objectContaining({id: 'p1', message: 'ENC(new text)'}));
    });

    test('an app\'s hook refusing the edit keeps it from being saved, and says why', async () => {
        render(async () => ({error: {message: 'Edits are locked in this channel'}}));
        await edit('new text');

        expect(await screen.findByRole('alert')).toHaveTextContent('Edits are locked in this channel');
        expect(editPost).not.toHaveBeenCalled();
        expect(screen.getByRole('textbox', {name: 'Edit message'})).toHaveValue('new text');
    });

    test('without hooks, the edit is saved as typed', async () => {
        render();
        await edit('new text');
        await waitFor(() => expect(editPost).toHaveBeenCalled());
        expect(jest.mocked(editPost).mock.calls[0][0]).toEqual(expect.objectContaining({id: 'p1', message: 'new text'}));
    });
});
