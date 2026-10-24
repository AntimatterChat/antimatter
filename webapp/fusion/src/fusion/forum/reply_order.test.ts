// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {TestHelper} from 'utils/test_helper';

import {orderReplies} from './reply_order';

describe('fusion/forum/orderReplies', () => {
    const r1 = TestHelper.getPostMock({id: 'r1', user_id: 'a', create_at: 100});
    const r2 = TestHelper.getPostMock({id: 'r2', user_id: 'b', create_at: 200});
    const r3 = TestHelper.getPostMock({id: 'r3', user_id: 'c', create_at: 300});
    const voter = (userId: string) => TestHelper.getReactionMock({user_id: userId, emoji_name: '+1'});

    test('discussion and ideas: oldest first, accepted answer ignored', () => {
        expect(orderReplies([r3, r1, r2], 'discussion', 'r3', {})).toEqual([r1, r2, r3]);
        expect(orderReplies([r3, r1, r2], 'ideas', undefined, {})).toEqual([r1, r2, r3]);
    });

    test('qa: the accepted answer first, then the rest', () => {
        expect(orderReplies([r1, r2, r3], 'qa', 'r2', {})).toEqual([r2, r1, r3]);
    });

    test('qa: the rest ordered by votes, most first', () => {
        const reactionsByPost = {
            r1: {x: voter('x')},
            r3: {y: voter('y'), z: voter('z')},
        };
        expect(orderReplies([r1, r2, r3], 'qa', undefined, reactionsByPost)).toEqual([r3, r1, r2]);
    });

    test('qa: equal votes keep the oldest first', () => {
        expect(orderReplies([r2, r1], 'qa', undefined, {})).toEqual([r1, r2]);
    });
});
