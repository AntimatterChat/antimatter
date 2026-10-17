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

import {getThreadTitle} from 'fusion/messages/thread_title';
import {am} from 'fusion/utils/class_names';
import {plainText} from 'fusion/utils/plain_text';

import type {GlobalState} from 'types/store';

// A followed thread shows under its channel while it has unread replies, while it's open in the right panel, and
// always when it's a topic (a named thread): topics first.
function sidebarThreadIds(state: GlobalState, channelId: string): string[] {
    const selected = getSelectedPostId(state);
    const named = (thread: UserThread) => Boolean(getThreadTitle(getPost(state, thread.id)));
    return Object.values(getThreads(state)).
        filter((thread: UserThread) => thread.post?.channel_id === channelId && thread.is_following && (thread.unread_replies > 0 || thread.id === selected || named(thread))).
        sort((a, b) => Number(named(b)) - Number(named(a)) || b.last_reply_at - a.last_reply_at).
        slice(0, 8).
        map((thread) => thread.id);
}

function ThreadRow({threadId}: {threadId: string}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const thread = useSelector((state: GlobalState) => getThreads(state)[threadId]);
    const named = useSelector((state: GlobalState) => getThreadTitle(getPost(state, threadId)));
    const firstWords = useSelector((state: GlobalState) => plainText(getPost(state, threadId)?.message || '').slice(0, 80));
    const title = named || firstWords;
    const open = useSelector(getSelectedPostId) === threadId;
    if (!thread) {
        return null;
    }
    const name = title || formatMessage({id: 'fusion.sidebar.threadUntitled', defaultMessage: 'Thread'});
    return (
        <button
            type='button'
            className={am('ch', 'ch-thread', {active: open, unread: thread.unread_replies > 0, named: Boolean(named)})}
            title={name}
            aria-label={named ? formatMessage({id: 'fusion.sidebar.topicLabel', defaultMessage: 'Topic: {name}, {count, plural, one {# new reply} other {# new replies}}'}, {name, count: thread.unread_replies}) : formatMessage({id: 'fusion.sidebar.threadLabel', defaultMessage: 'Thread: {name}, {count, plural, one {# new reply} other {# new replies}}'}, {name, count: thread.unread_replies})}
            onClick={() => dispatch(selectPostById(threadId))}
        >
            <span
                className={am('thread-branch')}
                aria-hidden='true'
            />
            <span className={am('name')}>{name}</span>
            {thread.unread_mentions > 0 && <span className={am('badge')}>{thread.unread_mentions}</span>}
        </button>
    );
}

// ChannelThreads are a channel's threads in the sidebar, under it, as Discord shows threads: the followed threads
// with new replies, and the one open in the right panel. With collapsed reply threads only.
export default function ChannelThreads({channelId}: {channelId: string}) {
    const crt = useSelector(isCollapsedThreadsEnabled);
    const ids = useSelector((state: GlobalState) => (crt ? sidebarThreadIds(state, channelId) : []), shallowEqual);
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
