// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Post} from '@mattermost/types/posts';

import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';

import {selectPost} from 'actions/views/rhs';

import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';
import {permalinkPath} from 'fusion/utils/paths';
import {getHistory} from 'utils/browser_history';

import {QuotedRef, jumpToMessage, useQuoted} from './inline_reply';

function ThreadButton({post}: {post: Post}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    return (
        <button
            className={am('ref-thread')}
            title={formatMessage({id: 'fusion.replyRef.threadTitle', defaultMessage: 'Open the conversation as a thread'})}
            aria-label={formatMessage({id: 'fusion.replyRef.threadTitle', defaultMessage: 'Open the conversation as a thread'})}
            onClick={() => dispatch(selectPost(post))}
        >
            <Icon
                name='thread'
                size='xs'
            />
            <span>{formatMessage({id: 'fusion.replyRef.thread', defaultMessage: 'Thread'})}</span>
        </button>
    );
}

// The quote of an inline reply: clicking it jumps to the quoted message and makes it flash, loading it around its
// place in the channel when it isn't shown.
function InlineReplyRef({post, quotedId, thread}: {post: Post; quotedId: string; thread: boolean}) {
    const {formatMessage} = useIntl();
    const team = useSelector(getCurrentTeam);
    const quoted = useQuoted(quotedId, post.metadata?.reply_to);

    return (
        <div className={am('reply-ref')}>
            <button
                className={am('ref-jump')}
                title={quoted.deleted ? undefined : formatMessage({id: 'fusion.replyRef.jump', defaultMessage: 'Jump to the original message'})}
                disabled={quoted.deleted}
                onClick={(e) => {
                    if (!jumpToMessage(e.currentTarget, quotedId) && team) {
                        getHistory().push(permalinkPath(team.name, quotedId));
                    }
                }}
            >
                <QuotedRef quoted={quoted}/>
            </button>
            {thread && <ThreadButton post={post}/>}
        </div>
    );
}

// The first of a run of thread replies shown in the channel (collapsed reply threads off) quotes the thread's first
// message, where the classic web app said "Commented on".
function ThreadReplyRef({post}: {post: Post}) {
    const {formatMessage} = useIntl();
    const team = useSelector(getCurrentTeam);
    const quoted = useQuoted(post.root_id);

    return (
        <div className={am('reply-ref')}>
            <button
                className={am('ref-jump')}
                title={formatMessage({id: 'fusion.replyRef.jump', defaultMessage: 'Jump to the original message'})}
                onClick={(e) => {
                    if (!jumpToMessage(e.currentTarget, post.root_id) && team) {
                        getHistory().push(permalinkPath(team.name, post.root_id));
                    }
                }}
            >
                <QuotedRef quoted={quoted}/>
            </button>
            <ThreadButton post={post}/>
        </div>
    );
}

type Props = {
    post: Post;

    // The message the post replies to inline, if any.
    quotedId?: string;

    // Whether the post is a thread reply shown in the channel, which offers to open its thread.
    threadReply: boolean;
};

// ReplyRef is the "(picture) Name snippet · Thread" line above a reply, which the message's spine joins to its
// author's picture: the mockup's .reply-ref, for inline replies and for thread replies shown in the channel.
export default function ReplyRef({post, quotedId, threadReply}: Props) {
    if (quotedId) {
        return (
            <InlineReplyRef
                post={post}
                quotedId={quotedId}
                thread={threadReply}
            />
        );
    }
    return <ThreadReplyRef post={post}/>;
}
