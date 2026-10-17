// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Post} from '@mattermost/types/posts';

import {act, renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import Files from './files';
import {clearImagesShown, setImagesShown} from './image_fold';

jest.mock('components/file_preview_modal', () => () => null);

const imageFile = {id: 'f1', name: 'cat.png', extension: 'png', mime_type: 'image/png', has_preview_image: true, width: 100, height: 80, size: 1000, post_id: 'p1', user_id: 'u1', create_at: 0, update_at: 0, delete_at: 0};

// A message's images: no button to hide them over the picture (the message's ⋯ menu does it), and a line to show them
// again when hidden.
describe('fusion/messages/content/Files: hiding images', () => {
    const post = {id: 'p1', channel_id: 'c1', user_id: 'u1', type: '', message: '', create_at: 0, props: {}, metadata: {}} as unknown as Post;
    const render = () => renderWithContext(<Files post={post}/>, {entities: {files: {files: {f1: imageFile}, fileIdsByPostId: {p1: ['f1']}}}} as never);

    beforeEach(() => {
        clearImagesShown();
    });

    test('an image has no hide button over it', () => {
        render();
        const tile = screen.getByRole('button', {name: 'Open image cat.png'});
        expect(tile.querySelector('[role="button"]')).toBeNull();
        expect(document.querySelector('.am-att-fold')).toBeNull();
        expect(screen.queryByRole('button', {name: 'Hide image'})).not.toBeInTheDocument();
        expect(screen.queryByTitle('Hide image')).not.toBeInTheDocument();
    });

    test('hidden from the menu, the images leave a line that shows them again', async () => {
        render();
        act(() => setImagesShown('p1', false));
        expect(screen.queryByRole('button', {name: 'Open image cat.png'})).not.toBeInTheDocument();

        await userEvent.click(screen.getByRole('button', {name: 'Show image'}));
        expect(screen.getByRole('button', {name: 'Open image cat.png'})).toBeInTheDocument();
    });

    test('follow the "previews collapsed" setting until the reader chooses', () => {
        renderWithContext(<Files post={post}/>, {
            entities: {
                files: {files: {f1: imageFile}, fileIdsByPostId: {p1: ['f1']}},
                preferences: {myPreferences: {'display_settings--collapse_previews': {category: 'display_settings', name: 'collapse_previews', value: 'true', user_id: 'u1'}}},
            },
        } as never);
        expect(screen.queryByRole('button', {name: 'Open image cat.png'})).not.toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Show image'})).toBeInTheDocument();
    });
});
