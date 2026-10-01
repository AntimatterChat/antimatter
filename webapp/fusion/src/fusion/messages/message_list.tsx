// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {markChannelAsRead} from 'mattermost-redux/actions/channels';
import {getPost, getRecentPostsChunkInChannel, makeGetPostsChunkAroundPost} from 'mattermost-redux/selectors/entities/posts';
import {getDateForDateLine, isCombinedUserActivityPost, isDateLine, isStartOfNewMessages, makeGenerateCombinedPost, makePreparePostIdsForPostList} from 'mattermost-redux/utils/post_list';

import {loadLatestPosts, loadPosts, loadPostsAround, syncPostsInChannel} from 'actions/views/channel';

import PostMessageView from 'components/post_view/post_message_view';

import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';
import {PostRequestTypes} from 'utils/constants';

import type {GlobalState} from 'types/store';

import Message from './message';
import {useDayLabel} from './time';

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
        </div>
    );
}

type Props = {
    channelId: string;
    focusedPostId?: string;
    scrollRef: React.RefObject<HTMLDivElement | null>;

    // What an empty conversation says.
    emptyText?: string;
};

// MessageList renders a channel's messages top to bottom, loading older ones while scrolling up, like the classic
// post list but without virtualization: the mockup's plain, selectable list.
export default function MessageList({channelId, focusedPostId, scrollRef, emptyText}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const dayLabel = useDayLabel();
    const getChunkAround = useMemo(() => makeGetPostsChunkAroundPost(), []);
    const prepare = useMemo(() => makePreparePostIdsForPostList(), []);
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

    // Load the channel's messages when it opens, around the linked message for permalinks.
    useEffect(() => {
        let cancelled = false;
        (async () => {
            if (focusedPostId) {
                await dispatch(loadPostsAround(channelId, focusedPostId));
            } else if (firstLoad) {
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
            if (target) {
                target.scrollIntoView({block: 'center'});
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
        prevHeight.current = view.scrollHeight;
        prevFirst.current = realPostIds[0];
    }, [rows, realPostIds, channelId, focusedPostId, scrollRef]);

    const loadOlder = useCallback(async () => {
        if (loadingOlder || chunk?.oldest || !realPostIds.length) {
            return;
        }
        setLoadingOlder(true);
        await dispatch(loadPosts({channelId, postId: realPostIds[0], type: PostRequestTypes.BEFORE_ID, perPage: PAGE}));
        setLoadingOlder(false);
    }, [loadingOlder, chunk, realPostIds, channelId, dispatch]);

    const loadNewer = useCallback(async () => {
        if (loadingNewer || chunk?.recent || !realPostIds.length) {
            return;
        }
        setLoadingNewer(true);
        await dispatch(loadPosts({channelId, postId: realPostIds[realPostIds.length - 1], type: PostRequestTypes.AFTER_ID, perPage: PAGE}));
        setLoadingNewer(false);
    }, [loadingNewer, chunk, realPostIds, channelId, dispatch]);

    useEffect(() => {
        const view = scrollRef.current;
        if (!view) {
            return undefined;
        }
        const onScroll = () => {
            const fromBottom = view.scrollHeight - view.scrollTop - view.clientHeight;
            stick.current = fromBottom < 60 && Boolean(chunk?.recent);
            if (view.scrollTop < LOAD_THRESHOLD) {
                loadOlder();
            }
            if (fromBottom < LOAD_THRESHOLD) {
                loadNewer();
            }
        };
        view.addEventListener('scroll', onScroll, {passive: true});
        return () => view.removeEventListener('scroll', onScroll);
    }, [scrollRef, chunk, loadOlder, loadNewer]);

    if (!items) {
        return <div className={am('empty')}>{formatMessage({id: 'fusion.messages.loading', defaultMessage: 'Loading messages…'})}</div>;
    }
    if (!realPostIds.length && chunk?.oldest) {
        return <div className={am('empty')}>{emptyText || formatMessage({id: 'fusion.messages.empty', defaultMessage: 'No messages yet — say hello.'})}</div>;
    }

    let previousPostId: string | undefined;
    return (
        <div className={am('msgs')}>
            {loadingOlder && <div className={am('empty')}>{formatMessage({id: 'fusion.messages.older', defaultMessage: 'Loading older messages…'})}</div>}
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
                        >
                            {formatMessage({id: 'fusion.messages.new', defaultMessage: 'New messages'})}
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
                        highlighted={id === focusedPostId}
                    />
                );
            })}
            {loadingNewer && <div className={am('empty')}>{formatMessage({id: 'fusion.messages.newer', defaultMessage: 'Loading newer messages…'})}</div>}
        </div>
    );
}
