// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useLayoutEffect, useMemo, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {getPostThread} from 'mattermost-redux/actions/posts';
import {getThread as fetchThread, updateThreadRead} from 'mattermost-redux/actions/threads';
import {getChannel} from 'mattermost-redux/selectors/entities/channels';
import {getPost, makeGetPostsForThread} from 'mattermost-redux/selectors/entities/posts';
import {isCollapsedThreadsEnabled} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {getThread} from 'mattermost-redux/selectors/entities/threads';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import {closeRightHandSide, toggleRhsExpanded} from 'actions/views/rhs';
import {markThreadAsRead, updateThreadLastOpened} from 'actions/views/threads';
import {getIsRhsExpanded} from 'selectors/rhs';

import {focusPost} from 'components/permalink_view/actions';
import {getThreadPopoutTitle} from 'components/thread_popout/thread_popout';

import Icon from 'fusion/components/icon';
import Composer from 'fusion/composer/composer';
import Message from 'fusion/messages/message';
import {getThreadTitle} from 'fusion/messages/thread_title';
import {am} from 'fusion/utils/class_names';
import {canPopout, popoutThread} from 'utils/popouts/popout_windows';

import type {GlobalState} from 'types/store';

import ThreadMenu from './thread_menu';

// ThreadPanel shows a thread in the right-hand panel: the first message, its replies, and a composer.
// inWindow draws it as a window of its own (a popout): without the buttons that open, size or close the panel.
export default function ThreadPanel({rootId, inWindow = false}: {rootId: string; inWindow?: boolean}) {
    const intl = useIntl();
    const {formatMessage} = intl;
    const dispatch = useDispatch();
    const getPostsForThread = useMemo(() => makeGetPostsForThread(), []);
    const root = useSelector((state: GlobalState) => getPost(state, rootId));
    const posts = useSelector((state: GlobalState) => getPostsForThread(state, rootId));
    const channel = useSelector((state: GlobalState) => (root ? getChannel(state, root.channel_id) : undefined));
    const me = useSelector(getCurrentUserId);
    const team = useSelector(getCurrentTeam);
    const expanded = useSelector(getIsRhsExpanded);
    const bodyRef = useRef<HTMLDivElement>(null);
    const menuRef = useRef<HTMLButtonElement>(null);
    const [menu, setMenu] = useState(false);

    useEffect(() => {
        dispatch(getPostThread(rootId));
    }, [rootId, dispatch]);

    // With collapsed reply threads, opening a thread reads it, as in the classic thread viewer: its unread badge
    // clears. Replies arriving while it's open are read too, unless it was marked unread by hand.
    const threadsOn = useSelector(isCollapsedThreadsEnabled);
    const thread = useSelector((state: GlobalState) => getThread(state, rootId));
    const following = Boolean(root?.is_following);
    const readFor = useRef('');
    useEffect(() => {
        if (!threadsOn || !team) {
            return;
        }
        if (!thread) {
            if (following) {
                dispatch(fetchThread(me, team.id, rootId, true));
            }
            return;
        }
        if (readFor.current !== rootId) {
            readFor.current = rootId;
            dispatch(updateThreadLastOpened(thread.id, thread.last_viewed_at));
            if (thread.last_viewed_at < thread.last_reply_at || thread.unread_replies > 0 || thread.unread_mentions > 0) {
                dispatch(updateThreadRead(me, team.id, rootId, Date.now()));
            }
        } else if (thread.unread_replies > 0 || thread.unread_mentions > 0) {
            dispatch(markThreadAsRead(rootId));
        }
    }, [threadsOn, team, thread, following, rootId, me, dispatch]);

    const replies = useMemo(() => posts.filter((p) => p.id !== rootId).sort((a, b) => a.create_at - b.create_at), [posts, rootId]);
    useLayoutEffect(() => {
        if (bodyRef.current) {
            bodyRef.current.scrollTop = bodyRef.current.scrollHeight;
        }
    }, [replies.length]);

    if (!root) {
        return <div className={am('empty')}>{formatMessage({id: 'fusion.thread.loading', defaultMessage: 'Loading…'})}</div>;
    }
    const lastReply = replies[replies.length - 1];

    // A real window with the thread (the mockup's floating window is a design choice).
    const popout = () => {
        if (!team) {
            return;
        }
        popoutThread(intl.formatMessage(getThreadPopoutTitle(channel)), rootId, team.name, (postId, returnTo) => {
            dispatch(focusPost(postId, returnTo, me, {skipRedirectReplyPermalink: true}));
        });
        dispatch(closeRightHandSide());
    };
    let where = '';
    if (channel) {
        where = (channel.type === 'D' || channel.type === 'G') ? formatMessage({id: 'fusion.thread.inDirect', defaultMessage: 'Direct message · {name}'}, {name: channel.display_name}) : `#${channel.display_name}`;
    }

    return (
        <>
            <div className={am('rhs-head')}>
                <div className={am('grow')}>
                    <h3>
                        <span className={am('tname')}>{getThreadTitle(root) || formatMessage({id: 'fusion.thread.title', defaultMessage: 'Thread'})}</span>
                        {root.is_following && (
                            <span
                                className={am('meta-tag')}
                                style={{color: 'var(--am-anti)'}}
                            >
                                <Icon name='follow'/>
                                {formatMessage({id: 'fusion.thread.following', defaultMessage: 'Following'})}
                            </span>
                        )}
                    </h3>
                    <span className={am('where')}>{where}</span>
                </div>
                <button
                    ref={menuRef}
                    className={am('icon-btn')}
                    title={formatMessage({id: 'fusion.threadMenu.label', defaultMessage: 'Thread actions'})}
                    aria-label={formatMessage({id: 'fusion.threadMenu.label', defaultMessage: 'Thread actions'})}
                    aria-haspopup='menu'
                    aria-expanded={menu}
                    onClick={() => setMenu(!menu)}
                >
                    <Icon name='dots'/>
                </button>
                {!inWindow && canPopout() && (
                    <button
                        className={am('icon-btn')}
                        title={formatMessage({id: 'fusion.thread.popout', defaultMessage: 'Open in a new window'})}
                        aria-label={formatMessage({id: 'fusion.thread.popout', defaultMessage: 'Open in a new window'})}
                        onClick={popout}
                    >
                        <Icon
                            name='popout'
                            size='sm'
                        />
                    </button>
                )}
                {!inWindow && (
                    <button
                        className={am('icon-btn')}
                        aria-pressed={expanded}
                        title={expanded ? formatMessage({id: 'fusion.thread.collapse', defaultMessage: 'Collapse the panel'}) : formatMessage({id: 'fusion.thread.expand', defaultMessage: 'Expand the panel'})}
                        aria-label={expanded ? formatMessage({id: 'fusion.thread.collapse', defaultMessage: 'Collapse the panel'}) : formatMessage({id: 'fusion.thread.expand', defaultMessage: 'Expand the panel'})}
                        onClick={() => dispatch(toggleRhsExpanded())}
                    >
                        <Icon
                            name='expand'
                            size='sm'
                        />
                    </button>
                )}
                {!inWindow && (
                    <button
                        className={am('icon-btn')}
                        title={formatMessage({id: 'fusion.rhs.close', defaultMessage: 'Close'})}
                        aria-label={formatMessage({id: 'fusion.thread.close', defaultMessage: 'Close thread'})}
                        onClick={() => dispatch(closeRightHandSide())}
                    >
                        <Icon name='x'/>
                    </button>
                )}
                {menu && (
                    <ThreadMenu
                        root={root}
                        lastReplyAt={lastReply ? lastReply.edit_at || lastReply.create_at : root.create_at}
                        anchor={menuRef.current}
                        onClose={() => setMenu(false)}
                    />
                )}
            </div>
            <div
                ref={bodyRef}
                className={am('rhs-body')}
            >
                <Message
                    postId={rootId}
                    inThread={true}
                />
                <div className={am('replies-sep')}>
                    {formatMessage({id: 'fusion.thread.replies', defaultMessage: '{count, plural, one {# reply} other {# replies}}'}, {count: replies.length})}
                </div>
                {replies.map((reply, i) => (
                    <Message
                        key={reply.id}
                        postId={reply.id}
                        previousPostId={i > 0 ? replies[i - 1].id : undefined}
                        inThread={true}
                    />
                ))}
            </div>
            {channel && channel.delete_at === 0 && (
                <Composer
                    key={rootId}
                    channelId={root.channel_id}
                    rootId={rootId}
                    placeholder={formatMessage({id: 'fusion.thread.reply', defaultMessage: 'Reply to thread'})}
                />
            )}
        </>
    );
}
