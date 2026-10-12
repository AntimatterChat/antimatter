// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {UserProfile} from '@mattermost/types/users';

import {Client4} from 'mattermost-redux/client';

import {act, fireEvent, renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import GlobalSearch from './global_search';

jest.mock('fusion/shell/global_search_context', () => ({useGlobalSearch: () => ({close: jest.fn()})}));

// Answers to the people search that the test gives when it wants, in the order it wants.
function pendingPeople() {
    let answer: (users: UserProfile[]) => void = () => {};
    const promise = new Promise<{users: UserProfile[]}>((resolve) => {
        answer = (users) => resolve({users});
    });
    return {promise, answer};
}

describe('fusion/search/GlobalSearch', () => {
    beforeEach(() => {
        jest.useFakeTimers();
        document.body.innerHTML = '<div id="am-layer"></div>';
        jest.spyOn(Client4, 'searchPostsWithParams').mockResolvedValue({order: [], posts: {}} as never);
    });

    afterEach(() => {
        jest.useRealTimers();
        jest.restoreAllMocks();
    });

    test('an answer to an earlier query, slower than the latest one, is dropped', async () => {
        const first = pendingPeople();
        const latest = pendingPeople();
        jest.spyOn(Client4, 'autocompleteUsers').mockReturnValueOnce(first.promise as never).mockReturnValueOnce(latest.promise as never);

        renderWithContext(<GlobalSearch/>, {
            entities: {
                users: {currentUserId: 'me', profiles: {me: TestHelper.getUserMock({id: 'me'})}, statuses: {}},
                teams: {currentTeamId: 't1', teams: {t1: TestHelper.getTeamMock({id: 't1', name: 'lumen'})}, myMembers: {t1: {team_id: 't1', user_id: 'me'}}},
                channels: {channels: {}, myMembers: {}},
            },
        } as never);
        const input = screen.getByRole('combobox', {name: 'Search everything'});

        fireEvent.change(input, {target: {value: 'al'}});
        act(() => {
            jest.advanceTimersByTime(250);
        });
        fireEvent.change(input, {target: {value: 'ali'}});
        act(() => {
            jest.advanceTimersByTime(250);
        });
        expect(Client4.autocompleteUsers).toHaveBeenCalledTimes(2);

        // The latest answer comes first; the earlier query's, slower, after it.
        await act(async () => {
            latest.answer([TestHelper.getUserMock({id: 'u2', username: 'alice'})]);
        });
        await act(async () => {
            first.answer([TestHelper.getUserMock({id: 'u1', username: 'albert'})]);
        });

        expect(screen.getAllByText((_, el) => el?.textContent === 'alice').length).toBeGreaterThan(0);
        expect(screen.queryAllByText((_, el) => el?.textContent === 'albert')).toHaveLength(0);
    });
});
