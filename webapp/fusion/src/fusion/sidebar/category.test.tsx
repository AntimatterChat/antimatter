// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import fs from 'fs';
import path from 'path';

import React from 'react';
import {createIntl} from 'react-intl';
import type {IntlShape} from 'react-intl';

import type {ChannelCategory} from '@mattermost/types/channel_categories';

import {setCategoryCollapsed} from 'mattermost-redux/actions/channel_categories';

import {fireEvent, renderWithContext, screen} from 'tests/react_testing_utils';

import Category from './category';

// The tests mock every i18n/*.json import: read the real French catalog.
const frMessages: Record<string, string> = JSON.parse(fs.readFileSync(path.join(__dirname, '..', '..', 'i18n', 'fr.json'), 'utf8'));

// The tests' useIntl is English (tests/react-intl_mock.ts): speak French for these.
const intlModule: {useIntl: () => IntlShape} = jest.requireMock('react-intl');
const speakFrench = () => jest.spyOn(intlModule, 'useIntl').mockReturnValue(createIntl({locale: 'fr', messages: frMessages, defaultLocale: 'en', timeZone: 'Etc/UTC', textComponent: 'span'}));

// The category's channels, the same list every time, as the memoised selector gives it.
const mockChannelIds = ['ops'];
jest.mock('selectors/views/channel_sidebar', () => ({makeGetFilteredChannelIdsForCategory: () => () => mockChannelIds}));
jest.mock('mattermost-redux/actions/channel_categories', () => ({
    ...jest.requireActual('mattermost-redux/actions/channel_categories'),
    setCategoryCollapsed: jest.fn(() => ({type: 'MOCK_SET_CATEGORY_COLLAPSED'})),
}));
jest.mock('./channel_row', () => ({__esModule: true, default: ({channelId}: {channelId: string}) => <div>{channelId}</div>}));

const staff = {id: 'managed_staff', team_id: 'team', user_id: 'me', type: 'managed', display_name: 'Staff', sorting: 'manual', channel_ids: ['ops'], muted: false, collapsed: false} as unknown as ChannelCategory;

describe('fusion/sidebar/Category', () => {
    const render = (locked: boolean) => renderWithContext(
        <Category category={staff}/>,
        {
            entities: {
                users: {currentUserId: 'me'},
                teams: {currentTeamId: 'team'},
                channelCategories: {serverCategories: {team: [{name: 'Staff', locked}]}},
            },
        } as never,
    );

    test('explains the lock of a category locked by the admins in a Fusion tooltip, not the browser\'s', () => {
        const {container} = render(true);

        const mark = screen.getByRole('img', {name: 'Set up for everyone by the community\'s admins. Its channels stay here.'});
        expect(mark).toHaveAttribute('data-am-tip', 'Set up for everyone by the community\'s admins. Its channels stay here.');
        expect(mark.querySelector('use')).toHaveAttribute('href', '#am-i-lock');
        expect(screen.getByRole('button', {name: 'Category options for Staff'})).toHaveAttribute('data-am-tip', 'Category options');
        expect(container.querySelector('[title]')).not.toBeInTheDocument();
    });

    test('says what an open category set up by the admins allows', () => {
        render(false);

        expect(screen.getByRole('img', {name: /You can move its channels to your own categories/})).toHaveAttribute('data-am-tip', expect.stringContaining('You can move its channels'));
    });
});

describe('fusion/sidebar/Category: names in the user\'s language', () => {
    const fr = frMessages;
    const category = (id: string, type: ChannelCategory['type'], displayName: string) => ({id, team_id: 'team', user_id: 'me', type, display_name: displayName, sorting: 'default', channel_ids: ['ops'], muted: false, collapsed: false}) as unknown as ChannelCategory;
    const renderIn = (c: ChannelCategory, locale = 'fr') => renderWithContext(
        <Category category={c}/>,
        {
            entities: {
                users: {currentUserId: 'me'},
                teams: {currentTeamId: 'team'},
                channelCategories: {serverCategories: {}},
            },
        } as never,
        {locale, intlMessages: locale === 'fr' ? fr : undefined},
    );

    test('in English, the Favourites category says Favourites', () => {
        renderIn(category('favorites_me_team', 'favorites', 'Favorites'), 'en');
        expect(screen.getByText('Favourites')).toBeInTheDocument();
    });

    describe('in French', () => {
        let spy: jest.SpyInstance;
        beforeEach(() => {
            spy = speakFrench();
        });
        afterEach(() => spy.mockRestore());

        test('the Favourites and Channels categories have their French names, whatever name the server stored', () => {
            const {unmount} = renderIn(category('favorites_me_team', 'favorites', 'Favorites'));
            expect(screen.getByText('Favoris')).toBeInTheDocument();
            expect(screen.queryByText('Favorites')).not.toBeInTheDocument();
            expect(screen.getByRole('button', {name: 'Options de la catégorie Favoris'})).toBeInTheDocument();
            unmount();

            renderIn(category('channels_me_team', 'channels', 'Channels'));
            expect(screen.getByText('Canaux')).toBeInTheDocument();
        });

        test('a custom category keeps the name its owner gave it', () => {
            renderIn(category('custom1', 'custom', 'Work'));
            expect(screen.getByText('Work')).toBeInTheDocument();
            expect(screen.getByRole('button', {name: 'Options de la catégorie Work'})).toBeInTheDocument();
        });
    });
});

describe('fusion/sidebar/Category: keys', () => {
    const mine = {id: 'custom_ops', team_id: 'team', user_id: 'me', type: 'custom', display_name: 'Ops', sorting: 'manual', channel_ids: ['ops'], muted: false, collapsed: false} as unknown as ChannelCategory;
    const render = () => renderWithContext(
        <Category category={mine}/>,
        {
            entities: {
                users: {currentUserId: 'me'},
                teams: {currentTeamId: 'team'},
                channelCategories: {serverCategories: {}},
            },
        } as never,
    );

    beforeEach(() => {
        (setCategoryCollapsed as jest.Mock).mockClear();
    });

    test('Enter or Space on the header folds the category', () => {
        const {container} = render();
        const header = container.querySelector('.am-cat') as HTMLElement;

        fireEvent.keyDown(header, {key: 'Enter'});
        fireEvent.keyDown(header, {key: ' '});
        expect(setCategoryCollapsed).toHaveBeenCalledTimes(2);
        expect(setCategoryCollapsed).toHaveBeenCalledWith('custom_ops', true);
    });

    test('Enter on the options button inside the header is the button\'s, not the header\'s', () => {
        render();

        fireEvent.keyDown(screen.getByRole('button', {name: 'Category options for Ops'}), {key: 'Enter'});
        fireEvent.keyDown(screen.getByRole('button', {name: 'Category options for Ops'}), {key: ' '});
        expect(setCategoryCollapsed).not.toHaveBeenCalled();
    });
});
