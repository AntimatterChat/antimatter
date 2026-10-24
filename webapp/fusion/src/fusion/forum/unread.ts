// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {ForumTopic} from '@mattermost/types/posts';
import type {UserThread} from '@mattermost/types/threads';

// isTopicUnread (design §4.4): "a topic is unread when it is newer than the channel's last view, or when it's
// followed and has unread replies".
export function isTopicUnread(topic: ForumTopic, lastViewedAt: number, thread?: Pick<UserThread, 'is_following' | 'unread_replies' | 'unread_mentions'>): boolean {
    if (topic.post.create_at > lastViewedAt) {
        return true;
    }
    return Boolean(thread?.is_following && (thread.unread_replies > 0 || thread.unread_mentions > 0));
}
