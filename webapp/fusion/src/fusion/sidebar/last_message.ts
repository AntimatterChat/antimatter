// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useEffect, useState} from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';
import type {Post} from '@mattermost/types/posts';

import {Client4} from 'mattermost-redux/client';
import {Posts} from 'mattermost-redux/constants';
import {getPost, getPostIdsInChannel} from 'mattermost-redux/selectors/entities/posts';
import {isCollapsedThreadsEnabled} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import {plainText} from 'fusion/utils/plain_text';
import {isSystemMessage} from 'utils/post_utils';

import type {GlobalState} from 'types/store';

type Last = {at: number; text: string; mine: boolean};

// The last message of conversations whose messages aren't loaded, fetched once per new message. Mattermost has no
// bulk API for it, so the home list asks a few at a time, newest conversations first.
const cache = new Map<string, Last>();
const queue: Array<() => Promise<void>> = [];
let running = 0;
const MAX_RUNNING = 3;

function done() {
    running--;
    next();
}

function next() {
    while (running < MAX_RUNNING && queue.length) {
        const job = queue.shift()!;
        running++;
        job().finally(done);
    }
}

function describe(post: Post | undefined, me: string, at: number): Last | null {
    if (!post || post.state === Posts.POST_DELETED || isSystemMessage(post)) {
        return null;
    }
    const files = post.file_ids?.length || post.metadata?.files?.length || 0;
    const text = plainText(post.message, 120) || (files ? '📎' : '');
    return {at, text, mine: post.user_id === me};
}

// useLastMessage gives the second line of a conversation in the home list: its last message, as in the mockup.
export function useLastMessage(channel: Channel): string {
    const {formatMessage} = useIntl();
    const me = useSelector(getCurrentUserId);
    const crt = useSelector(isCollapsedThreadsEnabled);

    // From the store when the conversation's messages are loaded.
    const loaded = useSelector((state: GlobalState) => {
        const ids = getPostIdsInChannel(state, channel.id);
        if (!ids?.length) {
            return undefined;
        }
        for (const id of ids) {
            const post = getPost(state, id);
            if (post && !(crt && post.root_id) && !isSystemMessage(post)) {
                return post;
            }
        }
        return undefined;
    });
    const [fetched, setFetched] = useState<Last | null>(() => cache.get(channel.id) || null);

    useEffect(() => {
        if (loaded || !channel.last_post_at) {
            return undefined;
        }
        const cached = cache.get(channel.id);
        if (cached && cached.at >= channel.last_post_at) {
            setFetched(cached);
            return undefined;
        }
        let cancelled = false;
        queue.push(async () => {
            if (cancelled) {
                return;
            }
            try {
                const list = await Client4.getPosts(channel.id, 0, 5, false, crt);
                const post = list.order.map((id) => list.posts[id]).find((p) => p && !(crt && p.root_id) && !isSystemMessage(p));
                const last = describe(post, me, channel.last_post_at) || {at: channel.last_post_at, text: '', mine: false};
                cache.set(channel.id, last);
                if (!cancelled) {
                    setFetched(last);
                }
            } catch {
                // The second line stays empty.
            }
        });
        next();
        return () => {
            cancelled = true;
        };
    }, [loaded, channel.id, channel.last_post_at, crt, me]);

    const last = loaded ? describe(loaded, me, loaded.create_at) : fetched;
    if (!last?.text) {
        return '';
    }
    return last.mine ? formatMessage({id: 'fusion.home.you', defaultMessage: 'You: {text}'}, {text: last.text}) : last.text;
}
