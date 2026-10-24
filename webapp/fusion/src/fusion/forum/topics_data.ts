// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useEffect, useRef, useState} from 'react';

import type {ForumTopic, ForumTopicsQuery} from '@mattermost/types/posts';

import {Client4} from 'mattermost-redux/client';

export type ForumTopicsState = {
    topics: ForumTopic[];
    loading: boolean;
    error: string;
    hasMore: boolean;
    loadMore: () => void;
};

// How long after the last change (a sort, a filter, typing in the search box) the list is refetched.
const FETCH_DELAY = 250;

// useForumTopics loads a forum channel's topics for the topics view (design §4.4): the query (sort, filters, q)
// starts the list over, from the first page; loadMore fetches the next with the query's own cursor (before).
export function useForumTopics(channelId: string, query: ForumTopicsQuery): ForumTopicsState {
    const [topics, setTopics] = useState<ForumTopic[]>([]);
    const [cursor, setCursor] = useState('');
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState('');
    const requestId = useRef(0);
    const queryKey = JSON.stringify(query);

    useEffect(() => {
        const id = ++requestId.current;
        setLoading(true);
        setError('');
        const timer = setTimeout(async () => {
            try {
                const list = await Client4.getForumTopics(channelId, query);
                if (id !== requestId.current) {
                    return;
                }
                setTopics(list.topics);
                setCursor(list.next_cursor);
                setLoading(false);
            } catch (e) {
                if (id !== requestId.current) {
                    return;
                }
                setTopics([]);
                setCursor('');
                setError((e as {message?: string})?.message || '');
                setLoading(false);
            }
        }, FETCH_DELAY);
        return () => clearTimeout(timer);

        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [channelId, queryKey]);

    const loadMore = () => {
        if (!cursor || loading) {
            return;
        }
        const id = ++requestId.current;
        setLoading(true);
        Client4.getForumTopics(channelId, {...query, before: cursor}).then((list) => {
            if (id !== requestId.current) {
                return;
            }
            setTopics((prev) => [...prev, ...list.topics]);
            setCursor(list.next_cursor);
            setLoading(false);
        }).catch((e) => {
            if (id !== requestId.current) {
                return;
            }
            setError((e as {message?: string})?.message || '');
            setLoading(false);
        });
    };

    return {topics, loading, error, hasMore: Boolean(cursor), loadMore};
}
