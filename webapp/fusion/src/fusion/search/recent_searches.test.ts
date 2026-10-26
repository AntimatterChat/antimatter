// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {addRecentSearch, clearRecentSearches, getRecentSearches} from './recent_searches';

let mockUserId = 'alice';
jest.mock('stores/redux_store', () => ({
    __esModule: true,
    default: {getState: () => ({entities: {users: {currentUserId: mockUserId}}})},
}));

// sec-fusion #8: the last searches were kept under one key for everyone using the browser, and signing out kept them:
// the next person signed in saw what the previous one had searched for.
describe('fusion/search/recent_searches', () => {
    beforeEach(() => {
        localStorage.clear();
        mockUserId = 'alice';
    });

    test('are the signed-in user\'s own', () => {
        addRecentSearch('from:hr salary review');
        expect(getRecentSearches()).toEqual(['from:hr salary review']);

        mockUserId = 'bob';
        expect(getRecentSearches()).toEqual([]);
        addRecentSearch('lunch');
        expect(getRecentSearches()).toEqual(['lunch']);

        mockUserId = 'alice';
        expect(getRecentSearches()).toEqual(['from:hr salary review']);
    });

    test('signing out forgets them', () => {
        addRecentSearch('from:hr salary review');
        clearRecentSearches();
        expect(getRecentSearches()).toEqual([]);
        expect(Object.keys(localStorage).filter((k) => k.startsWith('am-recent-searches'))).toEqual([]);
    });

    test('the list once shared by everyone is dropped, whose it was isn\'t known', () => {
        localStorage.setItem('am-recent-searches', JSON.stringify(['someone else\'s']));
        expect(getRecentSearches()).toEqual([]);
        expect(localStorage.getItem('am-recent-searches')).toBeNull();
    });

    test('nothing is kept while nobody is signed in', () => {
        mockUserId = '';
        addRecentSearch('anything');
        expect(getRecentSearches()).toEqual([]);
        expect(localStorage.length).toBe(0);
    });

    test('keeps the five latest, the latest first, without repeats', () => {
        ['a', 'b', 'c', 'd', 'e', 'f', 'c'].forEach(addRecentSearch);
        expect(getRecentSearches()).toEqual(['c', 'f', 'e', 'd', 'b']);
    });
});
