// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import fs from 'fs';
import path from 'path';

import React from 'react';
import {createIntl} from 'react-intl';
import type {IntlShape} from 'react-intl';

import {CategorySorting} from '@mattermost/types/channel_categories';
import type {ChannelCategory} from '@mattermost/types/channel_categories';

import {setCategoryMuted, setCategorySorting} from 'mattermost-redux/actions/channel_categories';

import {openDeleteCategory, openRenameCategory} from 'fusion/utils/modals';
import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import CategoryMenu from './category_menu';

// The tests mock every i18n/*.json import: read the real French catalog.
const frMessages: Record<string, string> = JSON.parse(fs.readFileSync(path.join(__dirname, '..', '..', 'i18n', 'fr.json'), 'utf8'));

// The tests' useIntl is English (tests/react-intl_mock.ts): speak French for these.
const intlModule: {useIntl: () => IntlShape} = jest.requireMock('react-intl');
const speakFrench = () => jest.spyOn(intlModule, 'useIntl').mockReturnValue(createIntl({locale: 'fr', messages: frMessages, defaultLocale: 'en', timeZone: 'Etc/UTC', textComponent: 'span'}));

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

    test('in French, names the Favourites category in French and a custom one by its own name', () => {
        const renderFr = (c: ChannelCategory) => renderWithContext(
            <CategoryMenu
                category={c}
                point={{x: 10, y: 10}}
                onClose={jest.fn()}
            />,
            {entities: {channels: {channels: {}, myMembers: {}, messageCounts: {}}, channelCategories: {byId: {cat: c}}}} as never,
            {locale: 'fr', intlMessages: frMessages},
        );

        const spy = speakFrench();
        try {
            const {unmount} = renderFr(category('favorites', {display_name: 'Favourites'}));
            expect(screen.getByRole('menu', {name: 'Options pour Favoris'})).toBeInTheDocument();
            unmount();

            renderFr(category('custom', {display_name: 'Work'}));
            expect(screen.getByRole('menu', {name: 'Options pour Work'})).toBeInTheDocument();
        } finally {
            spy.mockRestore();
        }
    });
});
