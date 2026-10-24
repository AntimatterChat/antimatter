// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {defineMessage} from 'react-intl';
import type {MessageDescriptor} from 'react-intl';

import type {ChannelKindMode} from '@mattermost/types/channels';
import type {ForumTopicsQuery, ForumTopicSort, ForumTopicStatus} from '@mattermost/types/posts';

import {FORUM_TOPIC_STATUS_MENU_ORDER, topicStatusLabel} from './status';

// A forum topics view filter chip (design §4.4): "All, Unread, Mine, Following, plus the mode's own." Unread has no
// server parameter (GET /channels/{id}/topics takes answered, status, mine, following): it filters the fetched page
// on the client (see unread.ts), the one exception to this being a server query.
export type TopicFilter = 'all' | 'unread' | 'mine' | 'following' | 'unanswered' | `status:${ForumTopicStatus}`;

export function sortsForMode(mode: ChannelKindMode): Array<{id: ForumTopicSort; label: MessageDescriptor}> {
    if (mode === 'qa') {
        return [
            {id: 'unanswered', label: defineMessage({id: 'fusion.forum.sortUnanswered', defaultMessage: 'Unanswered first', description: 'Topic list sort, Q&A forums: questions without an accepted answer first.'})},
            {id: 'newest', label: defineMessage({id: 'fusion.forum.sortNewest', defaultMessage: 'Newest', description: 'Topic list sort: newest topics first.'})},
            {id: 'votes', label: defineMessage({id: 'fusion.forum.sortVotes', defaultMessage: 'Most voted', description: 'Topic list sort: most 👍 votes first.'})},
        ];
    }
    if (mode === 'ideas') {
        return [
            {id: 'votes', label: defineMessage({id: 'fusion.forum.sortVotes', defaultMessage: 'Most voted', description: 'Topic list sort: most 👍 votes first.'})},
            {id: 'newest', label: defineMessage({id: 'fusion.forum.sortNewest', defaultMessage: 'Newest', description: 'Topic list sort: newest topics first.'})},
            {id: 'status', label: defineMessage({id: 'fusion.forum.sortStatus', defaultMessage: 'Status', description: 'Topic list sort, Ideas forums: grouped by status (in progress, planned, open, done, declined).'})},
        ];
    }
    return [
        {id: 'activity', label: defineMessage({id: 'fusion.forum.sortActivity', defaultMessage: 'Latest activity', description: 'Topic list sort: topics with the newest reply, or newest if no reply, first.'})},
        {id: 'newest', label: defineMessage({id: 'fusion.forum.sortNewest', defaultMessage: 'Newest', description: 'Topic list sort: newest topics first.'})},
    ];
}

export function defaultSortForMode(mode: ChannelKindMode): ForumTopicSort {
    return sortsForMode(mode)[0].id;
}

const BASE_FILTERS: Array<{id: TopicFilter; label: MessageDescriptor}> = [
    {id: 'all', label: defineMessage({id: 'fusion.forum.filterAll', defaultMessage: 'All', description: 'Topic list filter: every topic.'})},
    {id: 'unread', label: defineMessage({id: 'fusion.forum.filterUnread', defaultMessage: 'Unread', description: 'Topic list filter: topics with something new.'})},
    {id: 'mine', label: defineMessage({id: 'fusion.forum.filterMine', defaultMessage: 'Mine', description: 'Topic list filter: topics I opened.'})},
    {id: 'following', label: defineMessage({id: 'fusion.forum.filterFollowing', defaultMessage: 'Following', description: 'Topic list filter: topics I follow.'})},
];

export function filtersForMode(mode: ChannelKindMode): Array<{id: TopicFilter; label: MessageDescriptor}> {
    if (mode === 'qa') {
        return [...BASE_FILTERS, {id: 'unanswered', label: defineMessage({id: 'fusion.forum.filterUnanswered', defaultMessage: 'Unanswered', description: 'Topic list filter, Q&A forums: questions without an accepted answer.'})}];
    }
    if (mode === 'ideas') {
        return [...BASE_FILTERS, ...FORUM_TOPIC_STATUS_MENU_ORDER.map((status) => ({id: `status:${status}` as TopicFilter, label: topicStatusLabel(status)}))];
    }
    return BASE_FILTERS;
}

// buildTopicsQuery turns a sort, a filter chip and a search term into GET /channels/{id}/topics's query (design
// §3.3, §4.4). per_page keeps the server's default; before (the previous page's next_cursor) is given by the caller.
export function buildTopicsQuery(sort: ForumTopicSort, filter: TopicFilter, search: string, before = ''): ForumTopicsQuery {
    const query: ForumTopicsQuery = {sort};
    if (filter === 'mine') {
        query.mine = true;
    } else if (filter === 'following') {
        query.following = true;
    } else if (filter === 'unanswered') {
        query.answered = false;
    } else if (filter.startsWith('status:')) {
        query.status = filter.slice('status:'.length) as ForumTopicStatus;
    }
    const q = search.trim();
    if (q) {
        query.q = q;
    }
    if (before) {
        query.before = before;
    }
    return query;
}
