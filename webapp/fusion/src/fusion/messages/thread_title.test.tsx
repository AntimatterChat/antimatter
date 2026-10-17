// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {editPost} from 'actions/views/posts';

import {LAYER_ID} from 'fusion/components/layer';
import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import {getThreadTitle, THREAD_TITLE_PROP, TopicDialog} from './thread_title';

jest.mock('actions/views/posts', () => ({
    ...jest.requireActual('actions/views/posts'),
    editPost: jest.fn(() => ({type: 'EDITED'})),
}));

describe('fusion/messages/TopicDialog', () => {
    beforeEach(() => {
        const layer = document.createElement('div');
        layer.id = LAYER_ID;
        document.body.appendChild(layer);
        jest.clearAllMocks();
    });
    afterEach(() => {
        document.getElementById(LAYER_ID)?.remove();
    });

    test('names a thread to make it a topic, keeping the name where it always was', async () => {
        const post = TestHelper.getPostMock({id: 'root', props: {}});
        const onClose = jest.fn();
        renderWithContext(
            <TopicDialog
                post={post}
                onClose={onClose}
            />,
        );
        expect(screen.getByRole('heading', {name: 'Make it a topic'})).toBeInTheDocument();
        expect(screen.getByText(/A topic is a named conversation inside its channel/)).toBeInTheDocument();
        expect(screen.queryByText(/thread named|sub-chat/i)).not.toBeInTheDocument();

        await userEvent.type(screen.getByRole('textbox', {name: 'Name'}), ' Release 2.1 ');
        await userEvent.click(screen.getByRole('button', {name: 'Save'}));
        await waitFor(() => expect(onClose).toHaveBeenCalled());
        expect(THREAD_TITLE_PROP).toBe('am_thread_title');
        expect(editPost).toHaveBeenCalledWith(expect.objectContaining({id: 'root', props: {am_thread_title: 'Release 2.1'}}));
    });

    test('renames a topic, and says an empty name makes it a plain thread', () => {
        const post = TestHelper.getPostMock({id: 'root', props: {am_thread_title: 'Release 2.1'}});
        expect(getThreadTitle(post)).toBe('Release 2.1');
        renderWithContext(
            <TopicDialog
                post={post}
                onClose={jest.fn()}
            />,
        );
        expect(screen.getByRole('heading', {name: 'Rename the topic'})).toBeInTheDocument();
        expect(screen.getByRole('textbox', {name: 'Name'})).toHaveValue('Release 2.1');
        expect(screen.getByText(/Leave the name empty to make it a plain thread again/)).toBeInTheDocument();
    });
});
