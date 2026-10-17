// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useLayoutEffect, useMemo, useRef} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Post} from '@mattermost/types/posts';

import {getPostThread} from 'mattermost-redux/actions/posts';
import {updateThreadRead} from 'mattermost-redux/actions/threads';
import {getChannel} from 'mattermost-redux/selectors/entities/channels';
import {getPost, makeGetPostsForThread} from 'mattermost-redux/selectors/entities/posts';
import {isCollapsedThreadsEnabled} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentTeamId} from 'mattermost-redux/selectors/entities/teams';
import {getThread} from 'mattermost-redux/selectors/entities/threads';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import Composer from 'fusion/composer/composer';
import Message from 'fusion/messages/message';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

// Conversations for apps' side panels (Antimatter): the AI chat shows a person's conversation with an agent with
// Fusion's own messages and composer, rather than the classic thread viewer and editor (whose composer, next to the
// app's own text box, made two text entries). An app gets them as window.Components.FusionConversation and
// FusionComposer, draws its own header around them, and has exactly one composer on screen.

type ConversationProps = {

    // The thread's first message.
    rootId: string;
    placeholder?: string;
};

// AppConversation is a thread in an app's panel: its messages as in the thread panel (an agent's answer streams in
// as it's written), then the one composer, replying in the thread. Opening it reads the thread.
export function AppConversation({rootId, placeholder}: ConversationProps) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const getPostsForThread = useMemo(() => makeGetPostsForThread(), []);
    const root = useSelector((state: GlobalState) => getPost(state, rootId));
    const posts = useSelector((state: GlobalState) => getPostsForThread(state, rootId));
    const channel = useSelector((state: GlobalState) => (root ? getChannel(state, root.channel_id) : undefined));
    const me = useSelector(getCurrentUserId);
    const teamId = useSelector(getCurrentTeamId);
    const threadsOn = useSelector(isCollapsedThreadsEnabled);
    const thread = useSelector((state: GlobalState) => getThread(state, rootId));
    const bodyRef = useRef<HTMLDivElement>(null);

    useEffect(() => {
        dispatch(getPostThread(rootId));
    }, [rootId, dispatch]);

    const replies = useMemo(() => posts.filter((p) => p.id !== rootId).sort((a, b) => a.create_at - b.create_at), [posts, rootId]);
    const lastReply = replies[replies.length - 1];

    // What's shown is read: the unread badge of the conversation clears, as in the thread panel.
    const unread = Boolean(thread && (thread.unread_replies > 0 || thread.unread_mentions > 0));
    useEffect(() => {
        if (threadsOn && teamId && unread) {
            dispatch(updateThreadRead(me, teamId, rootId, Date.now()));
        }
    }, [threadsOn, teamId, unread, me, rootId, dispatch]);

    // The newest at the bottom, in view, also while an answer grows.
    useLayoutEffect(() => {
        if (bodyRef.current) {
            bodyRef.current.scrollTop = bodyRef.current.scrollHeight;
        }
    }, [replies.length, lastReply?.message]);

    if (!root) {
        return <div className={am('empty')}>{formatMessage({id: 'fusion.thread.loading', defaultMessage: 'Loading…'})}</div>;
    }

    return (
        <div
            className={am('app-conv')}
            data-testid='app-conversation'
        >
            <div
                ref={bodyRef}
                className={am('app-conv-body')}
                role='log'
                aria-live='polite'
            >
                <Message
                    postId={rootId}
                    inThread={true}
                />
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
                    placeholder={placeholder || formatMessage({id: 'fusion.thread.reply', defaultMessage: 'Reply to thread'})}
                />
            )}
        </div>
    );
}

type ComposerProps = {
    channelId: string;

    // A reply in a thread; a new message (a new conversation) without.
    rootId?: string;
    placeholder: string;

    // Called with the message once it's posted (e.g. to open the conversation it starts).
    onPosted?: (post: Post) => void;
};

// AppComposer is Fusion's composer for an app's panel: a new message in a channel (the AI chat's direct message with
// an agent), with what it posted handed back.
export function AppComposer({channelId, rootId = '', placeholder, onPosted}: ComposerProps) {
    return (
        <Composer
            channelId={channelId}
            rootId={rootId}
            placeholder={placeholder}
            afterSubmit={onPosted ? (response) => {
                const created = (response as {created?: unknown}).created;
                if (created && typeof created === 'object' && 'id' in created) {
                    onPosted(created as Post);
                }
            } : undefined}
        />
    );
}
