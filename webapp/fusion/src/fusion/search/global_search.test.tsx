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

    const searchState = (extra: Record<string, unknown> = {}) => ({
        entities: {
            users: {
                currentUserId: 'me',
                profiles: {
                    me: TestHelper.getUserMock({id: 'me'}),
                    jane: TestHelper.getUserMock({id: 'jane', username: 'jane', first_name: 'Jane', last_name: 'Smith', nickname: ''}),
                    eve: TestHelper.getUserMock({id: 'eve', username: 'eve'}),
                },
                statuses: {},
            },
            teams: {
                currentTeamId: 't1',
                teams: {t1: TestHelper.getTeamMock({id: 't1', name: 'lumen', name_display: 'nickname_full_name'} as never)},
                myMembers: {t1: {team_id: 't1', user_id: 'me'}},
                communityProfiles: {t1: {jane: {user_id: 'jane', team_id: 't1', nickname: 'nightowl'}}},
            },
            channels: {channels: {town: TestHelper.getChannelMock({id: 'town', team_id: 't1', type: 'O', display_name: 'Town'})}, myMembers: {}},
            preferences: {myPreferences: {'user_block--eve': {category: 'user_block', name: 'eve', value: 'ignore', user_id: 'me'}}},
            general: {config: {}},
            ...extra,
        },
    } as never);

    const search = (q: string) => {
        fireEvent.change(screen.getByRole('combobox', {name: 'Search everything'}), {target: {value: q}});
    };

    // sec-fusion #13: quick search built names from first and last names, past the community's way of showing people.
    test('shows a person as their community shows them', async () => {
        jest.spyOn(Client4, 'autocompleteUsers').mockResolvedValue({users: [TestHelper.getUserMock({id: 'jane', username: 'jane', first_name: 'Jane', last_name: 'Smith'})]} as never);
        renderWithContext(<GlobalSearch/>, searchState());
        search('jan');
        await act(async () => {
            await jest.advanceTimersByTimeAsync(300);
        });
        expect(screen.getAllByText((_, el) => el?.tagName === 'B' && el.textContent === 'nightowl').length).toBeGreaterThan(0);
        expect(screen.queryAllByText((_, el) => el?.tagName === 'B' && el.textContent === 'Jane Smith')).toHaveLength(0);
    });

    test('names a message\'s author as its community shows them', async () => {
        jest.spyOn(Client4, 'autocompleteUsers').mockResolvedValue({users: []} as never);
        jest.spyOn(Client4, 'searchPostsWithParams').mockResolvedValue({
            order: ['p1'],
            posts: {p1: TestHelper.getPostMock({id: 'p1', channel_id: 'town', user_id: 'jane', message: 'deploy at noon'})},
        } as never);
        renderWithContext(<GlobalSearch/>, searchState());
        search('deploy');
        await act(async () => {
            await jest.advanceTimersByTimeAsync(300);
        });
        expect(screen.getByText(/nightowl/)).toBeInTheDocument();
        expect(screen.queryByText(/Jane Smith/)).not.toBeInTheDocument();
    });

    // sec-fusion #5: quick search showed a spoiler's text, and the text of someone you ignore.
    test('keeps a spoiler\'s text and an ignored person\'s text out of message results', async () => {
        jest.spyOn(Client4, 'autocompleteUsers').mockResolvedValue({users: []} as never);
        jest.spyOn(Client4, 'searchPostsWithParams').mockResolvedValue({
            order: ['p1', 'p2'],
            posts: {
                p1: TestHelper.getPostMock({id: 'p1', channel_id: 'town', user_id: 'jane', message: 'the plot twist is here', props: {spoiler: true}}),
                p2: TestHelper.getPostMock({id: 'p2', channel_id: 'town', user_id: 'eve', message: 'the plot from eve'}),
            },
        } as never);
        renderWithContext(<GlobalSearch/>, searchState());
        search('plot');
        await act(async () => {
            await jest.advanceTimersByTimeAsync(300);
        });
        expect(screen.getByText('Spoiler')).toBeInTheDocument();
        expect(screen.getByText('Message from someone you ignore')).toBeInTheDocument();
        expect(document.body.textContent).not.toContain('twist is here');
        expect(document.body.textContent).not.toContain('from eve');
    });
});
