// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {markChannelAsRead} from 'mattermost-redux/actions/channels';
import {getPost, getRecentPostsChunkInChannel, makeGetPostsChunkAroundPost} from 'mattermost-redux/selectors/entities/posts';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';
import {getDateForDateLine, getTimestampForStartOfNewMessages, isCombinedUserActivityPost, isDateLine, isStartOfNewMessages, makeGenerateCombinedPost, makePreparePostIdsForPostList} from 'mattermost-redux/utils/post_list';

import {loadLatestPosts, loadPosts, loadPostsAround, syncPostsInChannel} from 'actions/views/channel';

import PostMessageView from 'components/post_view/post_message_view';

import Icon from 'fusion/components/icon';
import {useDisplayPrefs} from 'fusion/hooks/display';
import {a11yRegion} from 'fusion/utils/a11y_regions';
import {am} from 'fusion/utils/class_names';
import Pluggable from 'plugins/pluggable';
import PluggableErrorBoundary from 'plugins/pluggable/error_boundary';
import {PostRequestTypes} from 'utils/constants';

import type {GlobalState} from 'types/store';
import type {NewMessagesSeparatorActionComponent} from 'types/store/plugins';

const NO_SEPARATOR_ACTIONS: NewMessagesSeparatorActionComponent[] = [];

import Message from './message';
import {pageEdges} from './paging';
import MessageTime, {useDayLabel} from './time';

const PAGE = 60;
const LOAD_THRESHOLD = 600;

function CombinedActivity({id}: {id: string}) {
    const generate = useMemo(() => makeGenerateCombinedPost(), []);
    const post = useSelector((state: GlobalState) => generate(state, id));
    const {locale} = useIntl();
    return (
        <div className={am('msg', 'sysline')}>
            <span className={am('sys-ic')}><Icon name='user-plus'/></span>
            <span className={am('body')}>
                <PostMessageView
                    post={post}
                    isRHS={false}
                    isChannelAutotranslated={false}
                    userLanguage={locale}
                />
            </span>
            <MessageTime timestamp={post.create_at}/>
        </div>
    );
}

type Props = {
    channelId: string;

    // The message a link opened: the list loads and opens around it.
    focusedPostId?: string;

    // Whether that message is highlighted: while the address names it. Once it doesn't, the list stays where it
    // is, around the message, and the channel is read.
    highlight?: boolean;

    // Leaves that message for the newest ones: the "Jump to present" of the bar shown while the list is around older
    // messages. Without it, there's no bar.
    onJumpToPresent?: () => void;
    scrollRef: React.RefObject<HTMLDivElement | null>;

    // What an empty conversation says.
    emptyText?: string;

    // The start of the conversation, shown above its first message once there are no older ones.
    intro?: React.ReactNode;
};

// MessageList renders a channel's messages top to bottom, loading older ones while scrolling up, like the classic
// post list but without virtualization: the mockup's plain, selectable list.
export default function MessageList({channelId, focusedPostId, highlight = true, onJumpToPresent, scrollRef, emptyText, intro}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const dayLabel = useDayLabel();
    const getChunkAround = useMemo(() => makeGetPostsChunkAroundPost(), []);
    const prepare = useMemo(() => makePreparePostIdsForPostList(), []);
    const prefs = useDisplayPrefs();
    const separatorActions = useSelector((state: GlobalState) => state.plugins.components.NewMessagesSeparatorAction) || NO_SEPARATOR_ACTIONS;
    const [loadingOlder, setLoadingOlder] = useState(false);
    const [loadingNewer, setLoadingNewer] = useState(false);

    const chunk = useSelector((state: GlobalState) => (focusedPostId ? getChunkAround(state, focusedPostId, channelId) : getRecentPostsChunkInChannel(state, channelId)));
    const lastViewedAt = useSelector((state: GlobalState) => state.views.channel.lastChannelViewTime[channelId]);
    const firstLoad = useSelector((state: GlobalState) => !state.entities.posts.postsInChannel[channelId]);
    const postIds = chunk?.order;
    const latestTimestamp = useSelector((state: GlobalState) => (postIds?.length ? getPost(state, postIds[0])?.create_at || 0 : 0));
    const items = useSelector((state: GlobalState) => (postIds ? prepare(state, {postIds, lastViewedAt, indicateNewMessages: true}) : undefined));

    // The prepared list is newest first; the Fusion UI reads top to bottom.
    const rows = useMemo(() => (items ? [...items].reverse() : []), [items]);
    const realPostIds = useMemo(() => rows.filter((id) => !isDateLine(id) && !isStartOfNewMessages(id)), [rows]);

    // How many messages follow the Unread line, for the pill that jumps to it.
    const unreadCount = useMemo(() => {
        const at = rows.findIndex((id) => isStartOfNewMessages(id));
        return at === -1 ? 0 : rows.slice(at + 1).filter((id) => !isDateLine(id)).length;
    }, [rows]);

    // Discord's pills: "N new messages · Jump" while the Unread line is above the view, and "N new messages" at the
    // bottom for those that arrived while the reader was scrolled up.
    const [unreadAbove, setUnreadAbove] = useState(false);
    const [arrivedBelow, setArrivedBelow] = useState(0);
    const lastSeenNewest = useRef<string | undefined>(undefined);

    // Discord's "You're viewing older messages · Jump to present": the list is around a message (from a link) and
    // the newest messages aren't loaded with it (scrolling down to them, or loading them, joins them). The messages
    // that arrive meanwhile are counted from the newest one known when the list went there.
    const away = Boolean(onJumpToPresent && focusedPostId && chunk && !chunk.recent);
    const newestKnown = useSelector((state: GlobalState) => {
        const recent = getRecentPostsChunkInChannel(state, channelId);
        return recent?.order.length ? getPost(state, recent.order[0])?.create_at || 0 : 0;
    });
    const [awaySince, setAwaySince] = useState<number | null>(null);
    useEffect(() => {
        setAwaySince(away ? newestKnown : null);

        // When the list goes away from the newest messages, or comes back.
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [away]);
    const arrivedAway = useSelector((state: GlobalState) => {
        const recent = awaySince === null ? undefined : getRecentPostsChunkInChannel(state, channelId);
        if (!recent || awaySince === null) {
            return 0;
        }
        const me = getCurrentUserId(state);
        let count = 0;
        for (const id of recent.order) {
            const post = getPost(state, id);
            if (!post || post.create_at <= awaySince) {
                break;
            }
            if (post.user_id !== me) {
                count++;
            }
        }
        return count;
    });

    // Jump to present: the newest messages are loaded afresh, and the list opens at the bottom, whatever the reader's
    // choice of where unread conversations open.
    const loadLatestNext = useRef(false);
    const bottomNext = useRef(false);
    const jumpToPresent = () => {
        loadLatestNext.current = true;
        bottomNext.current = true;
        onJumpToPresent?.();
    };

    // Load the channel's messages when it opens, around the linked message for permalinks.
    useEffect(() => {
        let cancelled = false;
        const latest = loadLatestNext.current && !focusedPostId;
        if (latest) {
            loadLatestNext.current = false;
        }
        (async () => {
            if (focusedPostId) {
                await dispatch(loadPostsAround(channelId, focusedPostId));
            } else if (latest || firstLoad) {
                await dispatch(loadLatestPosts(channelId));
            } else if (latestTimestamp) {
                await dispatch(syncPostsInChannel(channelId, latestTimestamp, false));
            } else {
                await dispatch(loadLatestPosts(channelId));
            }
            if (!cancelled && !focusedPostId) {
                dispatch(markChannelAsRead(channelId));
            }
        })();
        return () => {
            cancelled = true;
        };

        // Only when the channel or the linked message changes.
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [channelId, focusedPostId]);

    // A linked message's highlight has ended: the reader has seen the conversation, as when it opens normally.
    useEffect(() => {
        if (focusedPostId && !highlight) {
            dispatch(markChannelAsRead(channelId));
        }
    }, [channelId, focusedPostId, highlight, dispatch]);

    // Scrolling: start at the bottom (or the linked message), stay at the bottom as messages arrive, and keep the
    // reader's place when older messages are added above.
    const stick = useRef(true);
    const prevHeight = useRef(0);
    const prevFirst = useRef<string | undefined>(undefined);
    const opened = useRef<string>('');
    useLayoutEffect(() => {
        const view = scrollRef.current;
        if (!view || !rows.length) {
            return;
        }
        const key = channelId + '|' + (focusedPostId || '');
        if (opened.current !== key) {
            opened.current = key;
            const target = focusedPostId ? document.getElementById(`post_${focusedPostId}`) : null;

            // An unread conversation opens at its Unread line, or at the newest message, as the reader chose.
            const unreadLine = !focusedPostId && prefs.startFromLeftOff && !bottomNext.current ? view.querySelector('.am-day.am-new') : null;
            bottomNext.current = false;
            if (target) {
                target.scrollIntoView({block: 'center'});
                stick.current = false;
            } else if (unreadLine && unreadLine.getBoundingClientRect().top - view.getBoundingClientRect().top > view.clientHeight / 3) {
                unreadLine.scrollIntoView({block: 'start'});
                view.scrollTop -= 8;
                stick.current = false;
            } else {
                view.scrollTop = view.scrollHeight;
                stick.current = true;
            }
        } else if (prevFirst.current && realPostIds[0] !== prevFirst.current && !stick.current) {
            view.scrollTop += view.scrollHeight - prevHeight.current;
        } else if (stick.current) {
            view.scrollTop = view.scrollHeight;
        }

        // Messages that arrive at the bottom while the reader is further up are counted for the bottom pill.
        const newest = realPostIds[realPostIds.length - 1];
        if (lastSeenNewest.current && newest !== lastSeenNewest.current && !stick.current && opened.current === key) {
            const added = realPostIds.length - realPostIds.indexOf(lastSeenNewest.current) - 1;
            if (added > 0 && realPostIds.includes(lastSeenNewest.current)) {
                setArrivedBelow((n) => n + added);
            }
        }
        lastSeenNewest.current = newest;
        prevHeight.current = view.scrollHeight;
        prevFirst.current = realPostIds[0];
    }, [rows, realPostIds, channelId, focusedPostId, scrollRef, prefs.startFromLeftOff]);

    const loadOlder = useCallback(async () => {
        if (loadingOlder || chunk?.oldest || !realPostIds.length) {
            return;
        }
        setLoadingOlder(true);
        await dispatch(loadPosts({channelId, postId: pageEdges(items || []).oldest, type: PostRequestTypes.BEFORE_ID, perPage: PAGE}));
        setLoadingOlder(false);
    }, [loadingOlder, chunk, realPostIds, items, channelId, dispatch]);

    const loadNewer = useCallback(async () => {
        if (loadingNewer || chunk?.recent || !realPostIds.length) {
            return;
        }
        setLoadingNewer(true);
        await dispatch(loadPosts({channelId, postId: pageEdges(items || []).newest, type: PostRequestTypes.AFTER_ID, perPage: PAGE}));
        setLoadingNewer(false);
    }, [loadingNewer, chunk, realPostIds, items, channelId, dispatch]);

    useEffect(() => {
        const view = scrollRef.current;
        if (!view) {
            return undefined;
        }
        const onScroll = () => {
            const fromBottom = view.scrollHeight - view.scrollTop - view.clientHeight;

            // At the bottom of the latest messages, the view follows them (a linked message's view only once it has
            // reached the latest ones).
            stick.current = fromBottom < 60 && (Boolean(chunk?.recent) || !focusedPostId);
            if (stick.current) {
                setArrivedBelow(0);
            }
            const line = view.querySelector('.am-day.am-new');
            setUnreadAbove(Boolean(line && line.getBoundingClientRect().bottom < view.getBoundingClientRect().top + 4));
            if (view.scrollTop < LOAD_THRESHOLD) {
                loadOlder();
            }
            if (fromBottom < LOAD_THRESHOLD) {
                loadNewer();
            }
        };
        view.addEventListener('scroll', onScroll, {passive: true});
        requestAnimationFrame(onScroll);
        return () => view.removeEventListener('scroll', onScroll);
    }, [scrollRef, chunk, loadOlder, loadNewer, focusedPostId]);

    // Images, link previews, embeds and folded long messages finish rendering after the list: while the view is at
    // the bottom, it stays there as they grow, so a channel opens on its latest messages.
    const listRef = useRef<HTMLDivElement>(null);
    const loaded = items !== undefined;
    useEffect(() => {
        const view = scrollRef.current;
        const list = listRef.current;
        if (!view || !list || typeof ResizeObserver === 'undefined') {
            return undefined;
        }
        const observer = new ResizeObserver(() => {
            if (stick.current) {
                view.scrollTop = view.scrollHeight;
            }
            prevHeight.current = view.scrollHeight;
        });
        observer.observe(list);
        return () => observer.disconnect();
    }, [scrollRef, loaded]);

    if (!items) {
        return <div className={am('empty')}>{formatMessage({id: 'fusion.messages.loading', defaultMessage: 'Loading messages…'})}</div>;
    }
    if (!realPostIds.length && chunk?.oldest) {
        return intro ? <div className={am('msgs')}>{intro}</div> : <div className={am('empty')}>{emptyText || formatMessage({id: 'fusion.messages.empty', defaultMessage: 'No messages yet — say hello.'})}</div>;
    }

    let previousPostId: string | undefined;
    const jumpToUnread = () => {
        scrollRef.current?.querySelector('.am-day.am-new')?.scrollIntoView({block: 'start', behavior: 'smooth'});
    };
    const jumpToNewest = () => {
        const view = scrollRef.current;
        if (view) {
            view.scrollTo({top: view.scrollHeight, behavior: 'smooth'});
        }
        setArrivedBelow(0);
    };

    return (
        <div
            ref={listRef}
            {...a11yRegion(am('msgs', {compact: prefs.compact, centered: prefs.centered, away}), 'messages')}
        >
            <div className={am('plugin-toasts')}>
                <Pluggable pluggableName='ChannelToast'/>
            </div>
            {unreadAbove && unreadCount > 0 && (
                <div className={am('jump', 'jump-top')}>
                    <button
                        type='button'
                        onClick={jumpToUnread}
                    >
                        <span>{formatMessage({id: 'fusion.messages.unreadSince', defaultMessage: '{count, plural, one {# new message} other {# new messages}} since you were last here'}, {count: unreadCount})}</span>
                        <b>{formatMessage({id: 'fusion.messages.jump', defaultMessage: 'Jump'})}</b>
                    </button>
                </div>
            )}
            {loadingOlder && <div className={am('empty')}>{formatMessage({id: 'fusion.messages.older', defaultMessage: 'Loading older messages…'})}</div>}
            {chunk?.oldest && intro}
            {rows.map((id) => {
                if (isDateLine(id)) {
                    previousPostId = undefined;
                    return (
                        <div
                            key={id}
                            className={am('day')}
                            role='separator'
                        >
                            {dayLabel(getDateForDateLine(id))}
                        </div>
                    );
                }
                if (isStartOfNewMessages(id)) {
                    previousPostId = undefined;
                    return (
                        <div
                            key={id}
                            className={am('day', 'new')}
                            role='separator'
                            aria-label={formatMessage({id: 'fusion.messages.unreadLabel', defaultMessage: 'Unread messages start here'})}
                        >
                            <span>{formatMessage({id: 'fusion.messages.unread', defaultMessage: 'Unread'})}</span>
                            {separatorActions.map((item) => {
                                const Component = item.component;
                                return Component ? (
                                    <PluggableErrorBoundary
                                        key={item.id}
                                        pluginId={item.pluginId}
                                    >
                                        <Component
                                            lastViewedAt={getTimestampForStartOfNewMessages(id)}
                                            channelId={channelId}
                                        />
                                    </PluggableErrorBoundary>
                                ) : null;
                            })}
                        </div>
                    );
                }
                if (isCombinedUserActivityPost(id)) {
                    previousPostId = undefined;
                    return (
                        <CombinedActivity
                            key={id}
                            id={id}
                        />
                    );
                }
                const prev = previousPostId;
                previousPostId = id;
                return (
                    <Message
                        key={id}
                        postId={id}
                        previousPostId={prev}
                        highlighted={highlight && id === focusedPostId}
                    />
                );
            })}
            {loadingNewer && <div className={am('empty')}>{formatMessage({id: 'fusion.messages.newer', defaultMessage: 'Loading newer messages…'})}</div>}
            {away && (
                <div
                    className={am('present')}
                    role='status'
                >
                    <span className={am('present-text')}>
                        {formatMessage({id: 'fusion.messages.olderMessages', defaultMessage: 'You\'re viewing older messages'})}
                        {arrivedAway > 0 && (
                            <b className={am('present-new')}>
                                {formatMessage({id: 'fusion.messages.arrived', defaultMessage: '{count, plural, one {# new message} other {# new messages}}'}, {count: arrivedAway})}
                            </b>
                        )}
                    </span>
                    <button
                        type='button'
                        className={am('present-jump')}
                        onClick={jumpToPresent}
                    >
                        {formatMessage({id: 'fusion.messages.jumpToPresent', defaultMessage: 'Jump to present'})}
                        <Icon
                            name='chev'
                            size='xs'
                        />
                    </button>
                </div>
            )}
            {arrivedBelow > 0 && (
                <div className={am('jump', 'jump-bottom')}>
                    <button
                        type='button'
                        onClick={jumpToNewest}
                    >
                        <span>{formatMessage({id: 'fusion.messages.arrived', defaultMessage: '{count, plural, one {# new message} other {# new messages}}'}, {count: arrivedBelow})}</span>
                        <Icon
                            name='chev'
                            size='xs'
                        />
                    </button>
                </div>
            )}
        </div>
    );
}
