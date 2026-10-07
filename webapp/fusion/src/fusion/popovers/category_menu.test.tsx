// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {CategorySorting} from '@mattermost/types/channel_categories';
import type {ChannelCategory} from '@mattermost/types/channel_categories';

import {setCategoryMuted, setCategorySorting} from 'mattermost-redux/actions/channel_categories';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import {openDeleteCategory, openRenameCategory} from 'fusion/utils/modals';

import CategoryMenu from './category_menu';

jest.mock('mattermost-redux/actions/channel_categories', () => ({
    setCategoryMuted: jest.fn(() => ({type: 'MUTED'})),
    setCategorySorting: jest.fn(() => ({type: 'SORTED'})),
}));
jest.mock('fusion/utils/modals', () => ({
    openCreateCategory: jest.fn(() => ({type: 'CREATE'})),
    openDeleteCategory: jest.fn(() => ({type: 'DELETE'})),
    openRenameCategory: jest.fn(() => ({type: 'RENAME'})),
}));
jest.mock('fusion/shell/toast_context', () => ({useToast: () => jest.fn()}));

describe('fusion/popovers/CategoryMenu', () => {
    beforeEach(() => {
        jest.clearAllMocks();
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
    });

    const category = (type: ChannelCategory['type'], extra: Partial<ChannelCategory> = {}) => ({id: 'cat', team_id: 'team', type, display_name: 'Work', channel_ids: [], sorting: CategorySorting.Default, muted: false, collapsed: false, ...extra} as ChannelCategory);
    const render = (c: ChannelCategory, onClose = jest.fn()) => renderWithContext(
        <CategoryMenu
            category={c}
            point={{x: 10, y: 10}}
            onClose={onClose}
        />,
        {entities: {channels: {channels: {}, myMembers: {}, messageCounts: {}}, channelCategories: {byId: {cat: c}}}} as never,
    );

    test('renames, deletes and mutes a custom category', async () => {
        const onClose = jest.fn();
        render(category('custom'), onClose);

        await userEvent.click(screen.getByRole('menuitem', {name: 'Rename category'}));
        expect(openRenameCategory).toHaveBeenCalledWith(expect.objectContaining({id: 'cat'}));
        expect(onClose).toHaveBeenCalled();

        await userEvent.click(screen.getByRole('menuitem', {name: 'Delete category'}));
        expect(openDeleteCategory).toHaveBeenCalled();

        await userEvent.click(screen.getByRole('menuitem', {name: 'Mute category'}));
        expect(setCategoryMuted).toHaveBeenCalledWith('cat', true);
    });

    test('only offers renaming and deleting for custom categories', () => {
        render(category('channels', {muted: true}));

        expect(screen.queryByRole('menuitem', {name: 'Rename category'})).not.toBeInTheDocument();
        expect(screen.queryByRole('menuitem', {name: 'Delete category'})).not.toBeInTheDocument();
        expect(screen.getByRole('menuitem', {name: 'Unmute category'})).toBeInTheDocument();
    });

    test('sorts the category', async () => {
        render(category('channels'));

        await userEvent.click(screen.getByRole('menuitemradio', {name: /By recent activity/}));
        expect(setCategorySorting).toHaveBeenCalledWith('cat', CategorySorting.Recency);
    });
});
