// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {savePreferences} from 'mattermost-redux/actions/preferences';

import configureStore from 'tests/test_store';

import {CATEGORY_FORUM_VIEW, getForumView, setForumView} from './view_preference';

jest.mock('mattermost-redux/actions/preferences', () => ({savePreferences: jest.fn(() => ({type: 'MOCK_SAVE'}))}));

describe('fusion/forum/view_preference', () => {
    const baseState = {
        entities: {
            users: {currentUserId: 'me'},
            preferences: {myPreferences: {}},
        },
    } as never;

    test('getForumView falls back to the channel\'s own default', () => {
        expect(getForumView(baseState, 'c1', 'topics')).toBe('topics');
        expect(getForumView(baseState, 'c1', 'imageboard')).toBe('imageboard');
    });

    test('getForumView prefers the person\'s own saved choice', () => {
        const state = {
            entities: {
                users: {currentUserId: 'me'},
                preferences: {myPreferences: {[`${CATEGORY_FORUM_VIEW}--c1`]: {category: CATEGORY_FORUM_VIEW, name: 'c1', value: 'imageboard'}}},
            },
        } as never;
        expect(getForumView(state, 'c1', 'topics')).toBe('imageboard');
    });

    test('setForumView saves the choice under the channel id, for the current user', () => {
        const store = configureStore(baseState);
        store.dispatch(setForumView('c1', 'imageboard') as never);

        expect(savePreferences).toHaveBeenCalledWith('me', [{user_id: 'me', category: CATEGORY_FORUM_VIEW, name: 'c1', value: 'imageboard'}]);
    });
});
