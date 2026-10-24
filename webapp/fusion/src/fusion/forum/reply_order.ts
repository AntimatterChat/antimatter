// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {ChannelKindMode} from '@mattermost/types/channels';
import type {Post} from '@mattermost/types/posts';
import type {Reaction} from '@mattermost/types/reactions';

import {countVotes} from './votes';

// orderReplies is the topic page's reply order (design §4.4): "the question, then the accepted answer pinned under
// it, then answers by votes (oldest first among equal votes)" for Q&A; oldest first, as any thread, otherwise.
export function orderReplies(replies: Post[], mode: ChannelKindMode, acceptedId: string | undefined, reactionsByPost: Record<string, Record<string, Reaction> | undefined>): Post[] {
    const byAge = [...replies].sort((a, b) => a.create_at - b.create_at);
    if (mode !== 'qa') {
        return byAge;
    }
    const accepted = byAge.filter((r) => r.id === acceptedId);
    const rest = byAge.filter((r) => r.id !== acceptedId).sort((a, b) => {
        const votes = countVotes(reactionsByPost[b.id], b.user_id) - countVotes(reactionsByPost[a.id], a.user_id);
        return votes !== 0 ? votes : a.create_at - b.create_at;
    });
    return [...accepted, ...rest];
}
