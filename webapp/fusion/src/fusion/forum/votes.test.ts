// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {TestHelper} from 'utils/test_helper';

import {countVotes, hasVoted, isVoteReaction} from './votes';

describe('fusion/forum/votes', () => {
    const author = 'ada';
    const voter1 = TestHelper.getReactionMock({user_id: 'maya', emoji_name: '+1'});
    const voter2 = TestHelper.getReactionMock({user_id: 'ivy', emoji_name: 'thumbsup'});
    const ownVote = TestHelper.getReactionMock({user_id: author, emoji_name: '+1'});
    const otherEmoji = TestHelper.getReactionMock({user_id: 'sam', emoji_name: 'heart'});
    const reactions = {
        [`${voter1.user_id}-${voter1.emoji_name}`]: voter1,
        [`${voter2.user_id}-${voter2.emoji_name}`]: voter2,
        [`${ownVote.user_id}-${ownVote.emoji_name}`]: ownVote,
        [`${otherEmoji.user_id}-${otherEmoji.emoji_name}`]: otherEmoji,
    };

    test('isVoteReaction counts both +1 and thumbsup, nothing else', () => {
        expect(isVoteReaction('+1')).toBe(true);
        expect(isVoteReaction('thumbsup')).toBe(true);
        expect(isVoteReaction('heart')).toBe(false);
    });

    test('countVotes counts +1 and thumbsup together, minus the author\'s own', () => {
        expect(countVotes(reactions, author)).toBe(2);
    });

    test('countVotes is 0 without reactions', () => {
        expect(countVotes(undefined, author)).toBe(0);
    });

    test('hasVoted is true only for someone else who voted', () => {
        expect(hasVoted(reactions, 'maya', author)).toBe(true);
        expect(hasVoted(reactions, 'sam', author)).toBe(false);
        expect(hasVoted(reactions, author, author)).toBe(false);
    });
});
