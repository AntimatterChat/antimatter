// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';
import type {Post} from '@mattermost/types/posts';

import {markChannelAsRead} from 'mattermost-redux/actions/channels';
import {getPostsSince} from 'mattermost-redux/actions/posts';
import {Posts} from 'mattermost-redux/constants';
import {getMyChannelMember, getUnreadChannels} from 'mattermost-redux/selectors/entities/channels';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';

import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import type {IconName} from 'fusion/components/icon';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import {useSnippetCover} from 'fusion/messages/snippet';
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

// groupGlyph is the simple glyph for a conversation that isn't a direct message (which gets the other person's
// picture instead): a lock for private channels, people for group messages, a hash for everything else.
function groupGlyph(channel: Channel): IconName {
    if (channel.type === 'P') {
        return 'lock';
    }
    if (channel.type === 'G') {
        return 'users';
    }
    return 'hash';
}

// How many of a conversation's new messages it shows; the others are counted ("3 more messages").
const SHOWN = 5;

type GroupProps = {
    channel: Channel;
    onRead: (channelId: string) => void;

    // Read in this visit: it stays in the list, marked so, until you leave the page.
    read: boolean;

    // You've scrolled the list: until then nothing is read on its own (the page opening, or a layout shift, isn't you
    // reading).
    armed: boolean;
};

// newPostsOf keeps a posts-since answer's posts that are new since you last looked, oldest first: posts-since also
// gives older ones edited or deleted since then.
export function newPostsOf(order: string[], byId: Record<string, Post>, since: number): Post[] {
    return order.map((id) => byId[id]).filter((post) => post && post.create_at > since && !post.delete_at && post.state !== Posts.POST_DELETED).reverse();
}

// UnreadGroup is one unread conversation: its new messages since you last saw it (the server's posts-since-last-
// viewed), read as you scroll past it or by its own "Mark read".
function UnreadGroup({channel, onRead, read, armed}: GroupProps) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const when = useWhen();
    const cover = useSnippetCover();
    const team = useSelector(getCurrentTeam);
    const lastViewedAt = useSelector((state: GlobalState) => getMyChannelMember(state, channel.id)?.last_viewed_at || 0);

    // What was new when the page showed it: reading it moves last_viewed_at, and its messages stay shown.
    const [since] = useState(lastViewedAt);
    const [posts, setPosts] = useState<Post[] | null>(null);
    const [inView, setInView] = useState(false);
    const ref = useRef<HTMLDivElement>(null);

    useEffect(() => {
        let cancelled = false;
        (async () => {
            const result = await dispatch(getPostsSince(channel.id, since));
            if (cancelled) {
                return;
            }
            if ('data' in result && result.data) {
                setPosts(newPostsOf(result.data.order, result.data.posts, since));
            } else {
                setPosts([]);
            }
        })();
        return () => {
            cancelled = true;
        };
    }, [channel.id, since, dispatch]);

    // Mostly in view or not, as it changes (the observer reports every threshold it crosses).
    useEffect(() => {
        const el = ref.current;
        if (!el || typeof IntersectionObserver === 'undefined') {
            return undefined;
        }
        const observer = new IntersectionObserver(([entry]) => {
            setInView(entry.isIntersecting && entry.intersectionRatio >= 0.6);
        }, {threshold: [0, 0.6, 1]});
        observer.observe(el);
        return () => observer.disconnect();
    }, []);

    // Kept in view for a moment after you scrolled: read, as if you'd opened it. Not when it has more new messages
    // than it shows: those you haven't seen stay unread (its "Mark read" still reads them all).
    const hidden = posts ? Math.max(0, posts.length - SHOWN) : 0;
    const autoRead = armed && inView && !read && posts !== null && !hidden;
    useEffect(() => {
        if (!autoRead) {
            return undefined;
        }
        const timer = setTimeout(() => onRead(channel.id), READ_DWELL_MS);
        return () => clearTimeout(timer);
    }, [autoRead, channel.id, onRead]);

    const direct = channel.type === 'D' || channel.type === 'G';
    const name = direct ? channel.display_name : `#${channel.display_name}`;

    return (
        <div
            ref={ref}
            className={am('unread-group', {read})}
        >
            <div className={am('unread-head')}>
                <button
                    type='button'
                    className={am('unread-open')}
                    onClick={() => team && getHistory().push(channelPath(team.name, channel))}
                >
                    {channel.type === 'D' && channel.teammate_id ? (
                        <Avatar
                            userId={channel.teammate_id}
                            size='sm'
                        />
                    ) : (
                        <Icon
                            name={groupGlyph(channel)}
                            size='sm'
                        />
                    )}
                    <span className={am('name')}>{name}</span>
                </button>
                {read ? (
                    <span className={am('unread-done')}>
                        <Icon
                            name='check'
                            size='sm'
                        />
                        {formatMessage({id: 'fusion.unreads.read', defaultMessage: 'Read'})}
                    </span>
                ) : (
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
                )}
            </div>
            <div className={am('unread-messages')}>
                {posts === null && <div className={am('loading')}>{formatMessage({id: 'fusion.unreads.loading', defaultMessage: 'Loading…'})}</div>}
                {hidden > 0 && (
                    <div className={am('unread-more')}>
                        {formatMessage({id: 'fusion.unreads.more', defaultMessage: '{count, plural, one {# earlier message} other {# earlier messages}}'}, {count: hidden})}
                    </div>
                )}
                {posts?.slice(-SHOWN).map((post) => {
                    const covered = cover(post);
                    return (
                        <div
                            key={post.id}
                            className={am('unread-msg')}
                        >
                            <span className={am('who')}><Sender userId={post.user_id}/></span>
                            <span className={am('snip', {'snip-cover': Boolean(covered)})}>{covered || plainText(post.message).slice(0, 160)}</span>
                            <span className={am('when')}>{when(post.create_at)}</span>
                        </div>
                    );
                })}
            </div>
        </div>
    );
}

// UnreadsView is one screen of every unread conversation of the community (public and private channels, direct and
// group messages), newest first: Discord's inbox, Slack's "Catch up". Scrolling a conversation into view, or its own
// "Mark read", reads it (the server's markChannelAsRead, as opening it would). What you read stays in the list, marked
// read, until you leave the page: otherwise the next one would slide into its place and be read in turn without you
// having seen it.
export default function UnreadsView() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const unread = useSelector(getUnreadChannels);
    const [readIds, setReadIds] = useState<string[]>([]);
    const [armed, setArmed] = useState(false);

    // The conversations as last listed unread (direct messages completed with their person's name), to keep showing
    // those read since.
    const lastListed = useRef(new Map<string, Channel>());
    const unreadIds = useMemo(() => new Set(unread.map((channel) => channel.id)), [unread]);
    const sorted = useMemo(() => {
        unread.forEach((channel) => lastListed.current.set(channel.id, channel));
        const kept = readIds.filter((id) => !unreadIds.has(id)).map((id) => lastListed.current.get(id)).filter((channel): channel is Channel => Boolean(channel));
        return [...unread, ...kept].sort((a, b) => b.last_post_at - a.last_post_at);
    }, [unread, unreadIds, readIds]);

    const onRead = useCallback((channelId: string) => {
        setReadIds((ids) => (ids.includes(channelId) ? ids : [...ids, channelId]));
        dispatch(markChannelAsRead(channelId));
    }, [dispatch]);

    // Read on its own only once you scroll the list yourself (wheel, touch, keys, the scroll bar).
    const arm = useCallback(() => setArmed(true), []);

    return (
        <div className={am('unreads-page')}>
            <div className={am('unreads-head')}>
                <h1>{formatMessage({id: 'fusion.unreads.title', defaultMessage: 'Unreads'})}</h1>
            </div>
            <div
                className={am('unreads-list')}
                onWheel={armed ? undefined : arm}
                onTouchMove={armed ? undefined : arm}
                onKeyDown={armed ? undefined : arm}
                onPointerDown={armed ? undefined : arm}
            >
                {!sorted.length && (
                    <div className={am('empty')}>{formatMessage({id: 'fusion.unreads.empty', defaultMessage: "You're all caught up."})}</div>
                )}
                {sorted.map((channel) => (
                    <UnreadGroup
                        key={channel.id}
                        channel={channel}
                        onRead={onRead}
                        read={!unreadIds.has(channel.id)}
                        armed={armed}
                    />
                ))}
            </div>
        </div>
    );
}
