// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {Client4} from 'mattermost-redux/client';

import {renderHook, waitFor} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import {useForumTopics} from './topics_data';

jest.mock('mattermost-redux/client', () => ({Client4: {getForumTopics: jest.fn()}}));

describe('fusion/forum/useForumTopics', () => {
    const topic = {post: TestHelper.getPostMock({id: 'p1'}), reply_count: 0, last_reply_at: 0, participant_ids: [], votes: 0, on_top: false};

    beforeEach(() => {
        jest.mocked(Client4.getForumTopics).mockReset();
    });

    test('loads the first page for the channel and query given', async () => {
        jest.mocked(Client4.getForumTopics).mockResolvedValue({topics: [topic], next_cursor: 'c1'});

        const {result} = renderHook(() => useForumTopics('ch1', {sort: 'activity'}));

        await waitFor(() => expect(result.current.loading).toBe(false));
        expect(Client4.getForumTopics).toHaveBeenCalledWith('ch1', {sort: 'activity'});
        expect(result.current.topics).toEqual([topic]);
        expect(result.current.hasMore).toBe(true);
    });

    test('loadMore appends the next page, using the cursor', async () => {
        jest.mocked(Client4.getForumTopics).
            mockResolvedValueOnce({topics: [topic], next_cursor: 'c1'}).
            mockResolvedValueOnce({topics: [{...topic, post: {...topic.post, id: 'p2'}}], next_cursor: ''});

        const {result} = renderHook(() => useForumTopics('ch1', {sort: 'activity'}));
        await waitFor(() => expect(result.current.loading).toBe(false));

        result.current.loadMore();

        await waitFor(() => expect(result.current.hasMore).toBe(false));
        expect(Client4.getForumTopics).toHaveBeenLastCalledWith('ch1', {sort: 'activity', before: 'c1'});
        expect(result.current.topics).toHaveLength(2);
    });

    test('a failed fetch clears the list and reports the error', async () => {
        jest.mocked(Client4.getForumTopics).mockRejectedValue({message: 'network down'});

        const {result} = renderHook(() => useForumTopics('ch1', {sort: 'activity'}));

        await waitFor(() => expect(result.current.loading).toBe(false));
        expect(result.current.topics).toEqual([]);
        expect(result.current.error).toBe('network down');
    });
});
