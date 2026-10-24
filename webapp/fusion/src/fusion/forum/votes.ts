// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Reaction} from '@mattermost/types/reactions';

// Antimatter forums: votes are 👍 reactions, one per person (the Reactions primary key already gives that), counted
// together whichever of the two names was used (design §3.3). Fusion sends '+1'; 'thumbsup' is read the same.
const VOTE_EMOJI_NAMES = new Set(['+1', 'thumbsup']);

export function isVoteReaction(emojiName: string): boolean {
    return VOTE_EMOJI_NAMES.has(emojiName);
}

// countVotes counts a post's 👍 reactions, minus the author's own (the owner's decision: it doesn't count).
export function countVotes(reactions: Record<string, Reaction> | undefined, authorId: string): number {
    if (!reactions) {
        return 0;
    }
    let count = 0;
    for (const reaction of Object.values(reactions)) {
        if (isVoteReaction(reaction.emoji_name) && reaction.user_id !== authorId) {
            count++;
        }
    }
    return count;
}

// hasVoted tells whether someone (not the author) has already voted.
export function hasVoted(reactions: Record<string, Reaction> | undefined, userId: string, authorId: string): boolean {
    if (!reactions || userId === authorId) {
        return false;
    }
    return Object.values(reactions).some((reaction) => isVoteReaction(reaction.emoji_name) && reaction.user_id === userId);
}
