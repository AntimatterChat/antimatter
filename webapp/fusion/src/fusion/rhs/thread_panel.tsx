// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useLayoutEffect, useMemo, useRef} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {getPostThread} from 'mattermost-redux/actions/posts';
import {setThreadFollow} from 'mattermost-redux/actions/threads';
import {getChannel} from 'mattermost-redux/selectors/entities/channels';
import {getPost, makeGetPostsForThread} from 'mattermost-redux/selectors/entities/posts';
import {isCollapsedThreadsEnabled} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentTeamId} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import {closeRightHandSide} from 'actions/views/rhs';

import Icon from 'fusion/components/icon';
import Composer from 'fusion/composer/composer';
import Message from 'fusion/messages/message';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

// ThreadPanel shows a thread in the right-hand panel: the first message, its replies, and a composer.
export default function ThreadPanel({rootId}: {rootId: string}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const getPostsForThread = useMemo(() => makeGetPostsForThread(), []);
    const root = useSelector((state: GlobalState) => getPost(state, rootId));
    const posts = useSelector((state: GlobalState) => getPostsForThread(state, rootId));
    const channel = useSelector((state: GlobalState) => (root ? getChannel(state, root.channel_id) : undefined));
    const crt = useSelector(isCollapsedThreadsEnabled);
    const me = useSelector(getCurrentUserId);
    const teamId = useSelector(getCurrentTeamId);
    const bodyRef = useRef<HTMLDivElement>(null);

    useEffect(() => {
        dispatch(getPostThread(rootId));
    }, [rootId, dispatch]);

    const replies = useMemo(() => posts.filter((p) => p.id !== rootId).sort((a, b) => a.create_at - b.create_at), [posts, rootId]);
    useLayoutEffect(() => {
        if (bodyRef.current) {
            bodyRef.current.scrollTop = bodyRef.current.scrollHeight;
        }
    }, [replies.length]);

    if (!root) {
        return null;
    }
    let where = '';
    if (channel) {
        where = (channel.type === 'D' || channel.type === 'G') ? formatMessage({id: 'fusion.thread.inDirect', defaultMessage: 'Direct message · {name}'}, {name: channel.display_name}) : `#${channel.display_name}`;
    }

    return (
        <>
            <div className={am('rhs-head')}>
                <div className={am('grow')}>
                    <h3>
                        <span className={am('tname')}>{formatMessage({id: 'fusion.thread.title', defaultMessage: 'Thread'})}</span>
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
                {crt && (
                    <button
                        className={am('icon-btn')}
                        title={root.is_following ? formatMessage({id: 'fusion.thread.unfollow', defaultMessage: 'Unfollow thread'}) : formatMessage({id: 'fusion.thread.follow', defaultMessage: 'Follow thread'})}
                        aria-label={root.is_following ? formatMessage({id: 'fusion.thread.unfollow', defaultMessage: 'Unfollow thread'}) : formatMessage({id: 'fusion.thread.follow', defaultMessage: 'Follow thread'})}
                        onClick={() => dispatch(setThreadFollow(me, teamId, rootId, !root.is_following))}
                    >
                        <Icon name='follow'/>
                    </button>
                )}
                <button
                    className={am('icon-btn')}
                    title={formatMessage({id: 'fusion.rhs.close', defaultMessage: 'Close'})}
                    aria-label={formatMessage({id: 'fusion.thread.close', defaultMessage: 'Close thread'})}
                    onClick={() => dispatch(closeRightHandSide())}
                >
                    <Icon name='x'/>
                </button>
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
                    compact={true}
                    placeholder={formatMessage({id: 'fusion.thread.reply', defaultMessage: 'Reply to thread'})}
                />
            )}
        </>
    );
}
