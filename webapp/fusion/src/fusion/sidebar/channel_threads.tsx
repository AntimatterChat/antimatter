// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect} from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useDispatch, useSelector} from 'react-redux';

import type {UserThread} from '@mattermost/types/threads';

import {getThreadsForCurrentTeam} from 'mattermost-redux/actions/threads';
import {getPost} from 'mattermost-redux/selectors/entities/posts';
import {isCollapsedThreadsEnabled} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentTeamId} from 'mattermost-redux/selectors/entities/teams';
import {getThreads} from 'mattermost-redux/selectors/entities/threads';

import {selectPostById} from 'actions/views/rhs';
import {getSelectedPostId} from 'selectors/rhs';

import {useHideUnreads} from 'fusion/focus/focus';
import {getThreadTitle} from 'fusion/messages/thread_title';
import {am} from 'fusion/utils/class_names';
import {plainText} from 'fusion/utils/plain_text';

import type {GlobalState} from 'types/store';

import {topicIdsForChannel} from './topics';

// A thread shows under its channel while it's a topic (a named thread, active in your topic period, whether you
// follow it or not: a topic is meant for the whole channel), while you follow it and it has unread replies (not
// while your unread badges hide, in focus mode), or while it's open in the right panel. Topics first, then by their
// last activity.
function sidebarThreadIds(state: GlobalState, channelId: string, hideUnreads: boolean, now: number): string[] {
    const selected = getSelectedPostId(state);
    const topics = topicIdsForChannel(state, channelId, now).
        sort((a, b) => (getPost(state, b)?.last_reply_at || getPost(state, b)?.create_at || 0) - (getPost(state, a)?.last_reply_at || getPost(state, a)?.create_at || 0));
    const ordinary = Object.values(getThreads(state)).
        filter((thread: UserThread) => thread.post?.channel_id === channelId && thread.is_following && !getThreadTitle(getPost(state, thread.id)) && ((!hideUnreads && thread.unread_replies > 0) || thread.id === selected)).
        sort((a, b) => b.last_reply_at - a.last_reply_at).
        slice(0, 8).
        map((thread) => thread.id);
    return [...topics, ...ordinary];
}

function ThreadRow({threadId}: {threadId: string}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();

    // A topic shows for everyone active in it, whether or not they follow it (then there's no UserThread for it):
    // it shows with no unread count of its own.
    const thread = useSelector((state: GlobalState) => getThreads(state)[threadId]);
    const named = useSelector((state: GlobalState) => getThreadTitle(getPost(state, threadId)));
    const firstWords = useSelector((state: GlobalState) => plainText(getPost(state, threadId)?.message || '').slice(0, 80));
    const title = named || firstWords;
    const open = useSelector(getSelectedPostId) === threadId;
    const hideUnreads = useHideUnreads();
    if (!thread && !named) {
        return null;
    }
    const unreadReplies = thread?.unread_replies || 0;
    const unreadMentions = thread?.unread_mentions || 0;
    const name = title || formatMessage({id: 'fusion.sidebar.threadUntitled', defaultMessage: 'Thread'});
    return (
        <button
            type='button'
            className={am('ch', 'ch-thread', {active: open, unread: unreadReplies > 0 && !hideUnreads, named: Boolean(named)})}
            title={name}
            aria-label={named ? formatMessage({id: 'fusion.sidebar.topicLabel', defaultMessage: 'Topic: {name}, {count, plural, one {# new reply} other {# new replies}}'}, {name, count: unreadReplies}) : formatMessage({id: 'fusion.sidebar.threadLabel', defaultMessage: 'Thread: {name}, {count, plural, one {# new reply} other {# new replies}}'}, {name, count: unreadReplies})}
            onClick={() => dispatch(selectPostById(threadId))}
        >
            <span
                className={am('thread-branch')}
                aria-hidden='true'
            />
            <span className={am('name')}>{name}</span>
            {unreadMentions > 0 && !hideUnreads && <span className={am('badge')}>{unreadMentions}</span>}
        </button>
    );
}

// ChannelThreads are a channel's threads in the sidebar, under it, as Discord shows threads: every topic active in
// your topic period, then the followed threads with new replies and the one open in the right panel. With collapsed
// reply threads only.
export default function ChannelThreads({channelId}: {channelId: string}) {
    const crt = useSelector(isCollapsedThreadsEnabled);
    const hideUnreads = useHideUnreads();
    const ids = useSelector((state: GlobalState) => (crt ? sidebarThreadIds(state, channelId, hideUnreads, Date.now()) : []), shallowEqual);
    if (!ids.length) {
        return null;
    }
    return (
        <div className={am('ch-threads')}>
            {ids.map((id) => (
                <ThreadRow
                    key={id}
                    threadId={id}
                />
            ))}
        </div>
    );
}

// useSidebarThreads loads the team's unread followed threads, once per team, for the sidebar to show them; replies
// arriving later come through the websocket.
export function useSidebarThreads() {
    const dispatch = useDispatch();
    const crt = useSelector(isCollapsedThreadsEnabled);
    const teamId = useSelector(getCurrentTeamId);
    useEffect(() => {
        if (crt && teamId) {
            dispatch(getThreadsForCurrentTeam({unread: true}));
            dispatch(getThreadsForCurrentTeam());
        }
    }, [crt, teamId, dispatch]);
}
