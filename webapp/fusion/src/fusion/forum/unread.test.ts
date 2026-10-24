// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {TestHelper} from 'utils/test_helper';

import {isTopicUnread} from './unread';

function topicOf(createAt: number) {
    return {
        post: TestHelper.getPostMock({create_at: createAt}),
        reply_count: 0,
        last_reply_at: 0,
        participant_ids: [],
        votes: 0,
        on_top: false,
    };
}

describe('fusion/forum/unread', () => {
    test('newer than the channel\'s last view is unread', () => {
        expect(isTopicUnread(topicOf(200), 100)).toBe(true);
    });

    test('not newer, not followed, is read', () => {
        expect(isTopicUnread(topicOf(100), 200)).toBe(false);
    });

    test('not newer, followed with unread replies, is unread', () => {
        expect(isTopicUnread(topicOf(100), 200, {is_following: true, unread_replies: 2, unread_mentions: 0})).toBe(true);
    });

    test('followed but nothing unread in the thread is read', () => {
        expect(isTopicUnread(topicOf(100), 200, {is_following: true, unread_replies: 0, unread_mentions: 0})).toBe(false);
    });

    test('unread replies in a thread not followed don\'t count', () => {
        expect(isTopicUnread(topicOf(100), 200, {is_following: false, unread_replies: 3, unread_mentions: 0})).toBe(false);
    });
});
