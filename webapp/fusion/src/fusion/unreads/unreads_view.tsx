// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';
import type {Post} from '@mattermost/types/posts';

import {markChannelAsRead} from 'mattermost-redux/actions/channels';
import {getPostsSince} from 'mattermost-redux/actions/posts';
import {getMyChannelMember, getUnreadChannels} from 'mattermost-redux/selectors/entities/channels';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';

import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import {useWhen} from 'fusion/messages/time';
import {am} from 'fusion/utils/class_names';
import {channelPath} from 'fusion/utils/paths';
import {plainText} from 'fusion/utils/plain_text';
import {getHistory} from 'utils/browser_history';

import type {GlobalState} from 'types/store';

// How long an unread conversation has to stay mostly in view before it's marked read on its own: scrolling past it
// quickly doesn't count, but pausing on it does, as the mockup's inbox (Discord) and "Catch up" (Slack).
const READ_DWELL_MS = 1200;

function Sender({userId}: {userId: string}) {
    const user = useUser(userId);
    return <>{useDisplayName(user)}</>;
}

type GroupProps = {
    channel: Channel;
    onRead: (channelId: string) => void;
};

// UnreadGroup is one unread conversation: its new messages since you last saw it (the server's posts-since-last-
// viewed), read as you scroll past it or by its own "Mark read".
function UnreadGroup({channel, onRead}: GroupProps) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const when = useWhen();
    const team = useSelector(getCurrentTeam);
    const since = useSelector((state: GlobalState) => getMyChannelMember(state, channel.id)?.last_viewed_at || 0);
    const [posts, setPosts] = useState<Post[] | null>(null);
    const ref = useRef<HTMLDivElement>(null);
    const read = useRef(false);

    useEffect(() => {
        let cancelled = false;
        (async () => {
            const result = await dispatch(getPostsSince(channel.id, since));
            if (cancelled) {
                return;
            }
            if ('data' in result && result.data) {
                const {order, posts: byId} = result.data;
                setPosts(order.map((id) => byId[id]).filter(Boolean).reverse().slice(-5));
            } else {
                setPosts([]);
            }
        })();
        return () => {
            cancelled = true;
        };
    }, [channel.id, since, dispatch]);

    // Scrolled into view for a moment: read, as if you'd opened it.
    useEffect(() => {
        const el = ref.current;
        if (!el || typeof IntersectionObserver === 'undefined') {
            return undefined;
        }
        let timer: ReturnType<typeof setTimeout> | undefined;
        const observer = new IntersectionObserver(([entry]) => {
            if (entry.isIntersecting && entry.intersectionRatio >= 0.6) {
                timer = setTimeout(() => {
                    if (!read.current) {
                        read.current = true;
                        onRead(channel.id);
                    }
                }, READ_DWELL_MS);
            } else if (timer) {
                clearTimeout(timer);
            }
        }, {threshold: [0, 0.6, 1]});
        observer.observe(el);
        return () => {
            observer.disconnect();
            if (timer) {
                clearTimeout(timer);
            }
        };
    }, [channel.id, onRead]);

    const direct = channel.type === 'D' || channel.type === 'G';
    const name = direct ? channel.display_name : `#${channel.display_name}`;

    return (
        <div
            ref={ref}
            className={am('unread-group')}
        >
            <div className={am('unread-head')}>
                <button
                    type='button'
                    className={am('unread-open')}
                    onClick={() => team && getHistory().push(channelPath(team.name, channel.id))}
                >
                    {channel.type === 'D' && channel.teammate_id ? (
                        <Avatar
                            userId={channel.teammate_id}
                            size='sm'
                        />
                    ) : (
                        <Icon
                            name={channel.type === 'P' ? 'lock' : channel.type === 'G' ? 'users' : 'hash'}
                            size='sm'
                        />
                    )}
                    <span className={am('name')}>{name}</span>
                </button>
                <button
                    type='button'
                    className={am('unread-mark')}
                    aria-label={formatMessage({id: 'fusion.unreads.markReadLabel', defaultMessage: 'Mark {name} as read'}, {name: channel.display_name})}
                    onClick={() => onRead(channel.id)}
                >
                    <Icon
                        name='check'
                        size='sm'
                    />
                    {formatMessage({id: 'fusion.unreads.mark', defaultMessage: 'Mark read'})}
                </button>
            </div>
            <div className={am('unread-messages')}>
                {posts === null && <div className={am('loading')}>{formatMessage({id: 'fusion.unreads.loading', defaultMessage: 'Loading…'})}</div>}
                {posts?.map((post) => (
                    <div
                        key={post.id}
                        className={am('unread-msg')}
                    >
                        <span className={am('who')}><Sender userId={post.user_id}/></span>
                        <span className={am('snip')}>{plainText(post.message).slice(0, 160)}</span>
                        <span className={am('when')}>{when(post.create_at)}</span>
                    </div>
                ))}
            </div>
        </div>
    );
}

// UnreadsView is one screen of every unread conversation of the community (public and private channels, direct and
// group messages), newest first: Discord's inbox, Slack's "Catch up". Scrolling a conversation into view, or its own
// "Mark read", reads it (the server's markChannelAsRead, as opening it would).
export default function UnreadsView() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const channels = useSelector(getUnreadChannels);
    const sorted = useMemo(() => [...channels].sort((a, b) => b.last_post_at - a.last_post_at), [channels]);
    const onRead = useCallback((channelId: string) => {
        dispatch(markChannelAsRead(channelId));
    }, [dispatch]);

    return (
        <div className={am('unreads-page')}>
            <div className={am('unreads-head')}>
                <h1>{formatMessage({id: 'fusion.unreads.title', defaultMessage: 'Unreads'})}</h1>
            </div>
            <div className={am('unreads-list')}>
                {!sorted.length && (
                    <div className={am('empty')}>{formatMessage({id: 'fusion.unreads.empty', defaultMessage: "You're all caught up."})}</div>
                )}
                {sorted.map((channel) => (
                    <UnreadGroup
                        key={channel.id}
                        channel={channel}
                        onRead={onRead}
                    />
                ))}
            </div>
        </div>
    );
}
