// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useMemo, useRef} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';
import type {ForumTopicStatus} from '@mattermost/types/posts';

import {getPostThread} from 'mattermost-redux/actions/posts';
import {getThread as fetchThread, updateThreadRead} from 'mattermost-redux/actions/threads';
import {getChannelTypeInfo} from 'mattermost-redux/selectors/entities/channel_types';
import {getPost, getReactionsForPosts, makeGetPostsForThread} from 'mattermost-redux/selectors/entities/posts';
import {isCollapsedThreadsEnabled} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {getThread} from 'mattermost-redux/selectors/entities/threads';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import {markThreadAsRead, updateThreadLastOpened} from 'actions/views/threads';

import Composer from 'fusion/composer/composer';
import Icon from 'fusion/components/icon';
import Message from 'fusion/messages/message';
import {getThreadTitle} from 'fusion/messages/thread_title';
import {am} from 'fusion/utils/class_names';
import {channelPath} from 'fusion/utils/paths';
import {getHistory} from 'utils/browser_history';

import type {GlobalState} from 'types/store';

import {ACCEPTED_ANSWER_PROP, TOPIC_STATUS_PROP} from './props';
import {orderReplies} from './reply_order';
import {topicStatusLabel} from './status';
import VoteButton from './vote_button';

type Props = {
    channel: Channel;
    rootId: string;
};

// TopicPage is a forum topic's own page (design §4.4), at /<team>/channels/<channel>/topics/<root id>: the first
// post, then the replies through the thread view's own components (Message, so every reply keeps its menu — Accept
// in Q&A, Set status in Ideas, both in fusion/messages/message_menu — and its reactions, the 👍 votes), with a
// reply composer at the bottom. "All topics" goes back to the topics view.
export default function TopicPage({channel, rootId}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const getPostsForThread = useMemo(() => makeGetPostsForThread(), []);
    const root = useSelector((state: GlobalState) => getPost(state, rootId));
    const posts = useSelector((state: GlobalState) => getPostsForThread(state, rootId));
    const reactionsByPost = useSelector(getReactionsForPosts);
    const me = useSelector(getCurrentUserId);
    const team = useSelector(getCurrentTeam);
    const mode = useSelector((state: GlobalState) => getChannelTypeInfo(state, channel.id).mode);

    useEffect(() => {
        dispatch(getPostThread(rootId));
    }, [rootId, dispatch]);

    // As the thread view: opening the topic reads it (collapsed reply threads), its unread badge clears.
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

    const replies = useMemo(
        () => orderReplies(posts.filter((p) => p.id !== rootId), mode, root?.props?.[ACCEPTED_ANSWER_PROP] as string | undefined, reactionsByPost),
        [posts, rootId, mode, root, reactionsByPost],
    );

    const goBack = () => {
        if (team) {
            getHistory().push(channelPath(team.name, channel));
        }
    };

    if (!root) {
        return <div className={am('empty')}>{formatMessage({id: 'fusion.thread.loading', defaultMessage: 'Loading…'})}</div>;
    }

    const title = getThreadTitle(root) || formatMessage({id: 'fusion.thread.title', defaultMessage: 'Thread'});
    const status = (root.props?.[TOPIC_STATUS_PROP] as ForumTopicStatus) || 'open';

    return (
        <>
            <div className={am('topic-head')}>
                <button
                    type='button'
                    className={am('back-link')}
                    onClick={goBack}
                >
                    <Icon
                        name='chev'
                        size='sm'
                    />
                    {formatMessage({id: 'fusion.forum.allTopics', defaultMessage: 'All topics', description: 'Link back from a topic page to the forum\'s topics view.'})}
                </button>
                <h2>{title}</h2>
                {mode === 'qa' && <VoteButton post={root}/>}
                {mode === 'ideas' && (
                    <>
                        <VoteButton post={root}/>
                        <span className={am('topic-status', status)}>
                            {formatMessage(topicStatusLabel(status))}
                        </span>
                    </>
                )}
            </div>
            <div className={am('rhs-body', 'topic-body')}>
                <Message
                    postId={rootId}
                    inThread={true}
                />
                <div className={am('replies-sep')}>
                    {formatMessage({id: 'fusion.thread.replies', defaultMessage: '{count, plural, one {# reply} other {# replies}}', description: 'Separator above a thread\'s or topic\'s replies: how many there are.'}, {count: replies.length})}
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
            {channel.delete_at === 0 && (
                <Composer
                    key={rootId}
                    channelId={channel.id}
                    rootId={rootId}
                    placeholder={formatMessage({id: 'fusion.thread.reply', defaultMessage: 'Reply to thread'})}
                />
            )}
        </>
    );
}
