// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Post, PostReplyTo} from '@mattermost/types/posts';

import {getMissingProfilesByIds} from 'mattermost-redux/actions/users';
import {Posts} from 'mattermost-redux/constants';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {getPost, getPostIdsInChannel} from 'mattermost-redux/selectors/entities/posts';
import {getUser} from 'mattermost-redux/selectors/entities/users';
import {isPostPendingOrFailed} from 'mattermost-redux/utils/post_utils';

import Icon from 'fusion/components/icon';
import {useDisplayName} from 'fusion/hooks/users';
import {am} from 'fusion/utils/class_names';
import {plainText} from 'fusion/utils/plain_text';
import {isSystemMessage} from 'utils/post_utils';

import type {GlobalState} from 'types/store';

// Inline replies answer one message in the conversation, quoting it above the reply, without opening a thread (the
// mockup's "Replying to" line). The reply's reply_to prop holds the quoted message's id; the server describes that
// message in the reply's metadata.reply_to.

// REPLY_EVENT asks a conversation's composer to reply to a message.
export const REPLY_EVENT = 'am-reply-to';
export type ReplyEventDetail = {channelId: string; rootId: string; postId: string};

export function useInlineRepliesEnabled(): boolean {
    return useSelector((state: GlobalState) => getConfig(state).EnableInlineReplies !== 'false');
}

// replyToId is the id of the message a post (or a draft) replies to inline, or ''.
export function replyToId(post?: {props?: Record<string, unknown>}): string {
    const id = post?.props?.reply_to;
    return typeof id === 'string' ? id : '';
}

// canReplyInline tells whether a message can be quoted: the server refuses system messages, burn-on-read messages
// and messages it doesn't have yet.
export function canReplyInline(post: Post): boolean {
    return !isSystemMessage(post) &&
        post.type !== Posts.POST_TYPES.BURN_ON_READ &&
        post.state !== Posts.POST_DELETED &&
        !post.delete_at &&
        !isPostPendingOrFailed(post);
}

// replyInline asks the composer of the conversation the message is shown in to quote it: the channel's composer, or
// the thread's when rootId is set.
export function replyInline(post: Post, rootId: string) {
    window.dispatchEvent(new CustomEvent<ReplyEventDetail>(REPLY_EVENT, {detail: {channelId: post.channel_id, rootId, postId: post.id}}));
}

// replyCandidates lists the messages Shift+Up and Shift+Down move the quote between, newest first.
export function replyCandidates(state: GlobalState, channelId: string, rootId: string): string[] {
    let posts: Post[];
    if (rootId) {
        const thread = state.entities.posts.postsInThread[rootId] || [];
        posts = [rootId, ...thread].map((id) => getPost(state, id)).filter((post): post is Post => Boolean(post));
        posts.sort((a, b) => b.create_at - a.create_at);
    } else {
        posts = (getPostIdsInChannel(state, channelId) || []).map((id) => getPost(state, id)).filter((post): post is Post => Boolean(post));
    }
    return posts.filter(canReplyInline).map((post) => post.id);
}

type Quoted = {
    deleted: boolean;
    userId?: string;
    message: string;
    fileCount: number;
    overrideUsername?: string;
};

// useQuoted is what to show of a quoted message: the message itself when it's loaded, as it follows edits and
// deletions as they happen, else the server's description of it.
export function useQuoted(id: string, described?: PostReplyTo): Quoted {
    const post = useSelector((state: GlobalState) => (id ? getPost(state, id) : undefined));
    if (post) {
        if (post.state === Posts.POST_DELETED || post.delete_at || post.type === Posts.POST_TYPES.BURN_ON_READ) {
            return {deleted: true, message: '', fileCount: 0};
        }
        const fromWebhook = post.props?.from_webhook === 'true';
        return {
            deleted: false,
            userId: post.user_id,
            message: post.message,
            fileCount: post.file_ids?.length || post.metadata?.files?.length || 0,
            overrideUsername: fromWebhook && typeof post.props?.override_username === 'string' ? post.props.override_username : undefined,
        };
    }
    if (described && described.post_id === id && !described.deleted) {
        return {
            deleted: false,
            userId: described.user_id,
            message: described.message || '',
            fileCount: described.file_count || 0,
            overrideUsername: described.override_username,
        };
    }

    // Not loaded and not described: the server dropped it as deleted, or doesn't describe it yet.
    return {deleted: Boolean(described?.deleted), message: '', fileCount: 0};
}

// QuotedMessage is the "↩ Replying to Name snippet" content of the quote line above a reply and of the composer's
// reply bar.
export function QuotedMessage({quoted}: {quoted: Quoted}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const user = useSelector((state: GlobalState) => (quoted.userId ? getUser(state, quoted.userId) : undefined));
    const name = useDisplayName(user);
    const overrideAllowed = useSelector((state: GlobalState) => getConfig(state).EnablePostUsernameOverride === 'true');

    // The quoted message's author may not be loaded when the quoted message isn't.
    useEffect(() => {
        if (quoted.userId && !user) {
            dispatch(getMissingProfilesByIds([quoted.userId]));
        }
    }, [quoted.userId, user, dispatch]);

    const who = (overrideAllowed && quoted.overrideUsername) || (user ? name : '');
    let snippet;
    if (quoted.deleted) {
        snippet = formatMessage({id: 'fusion.inlineReply.deleted', defaultMessage: 'Original message deleted'});
    } else if (quoted.message.trim()) {
        snippet = plainText(quoted.message, 160);
    } else if (quoted.fileCount) {
        snippet = formatMessage({id: 'fusion.inlineReply.files', defaultMessage: '{count, plural, one {# file} other {# files}}'}, {count: quoted.fileCount});
    } else {
        snippet = formatMessage({id: 'fusion.replyRef.message', defaultMessage: 'a message'});
    }

    return (
        <>
            <Icon
                name='reply'
                size='xs'
            />
            <span className={am('ref-label')}>{formatMessage({id: 'fusion.replyRef.replying', defaultMessage: 'Replying to'})}</span>
            {who && <b>{who}</b>}
            <span className={am('snip', {gone: quoted.deleted})}>{snippet}</span>
        </>
    );
}

// jumpToMessage scrolls to a message shown in the same conversation as from and makes it flash, as the mockup does;
// it returns false when the message isn't shown there.
export function jumpToMessage(from: HTMLElement, postId: string): boolean {
    const scope = from.closest(`.${am('rhs-body')}, .${am('msgs')}`);
    const target = scope?.querySelector<HTMLElement>(`[id="post_${postId}"]`);
    if (!target) {
        return false;
    }
    target.scrollIntoView({block: 'center', behavior: 'smooth'});
    const flash = am('flash');
    target.classList.remove(flash);

    // Restart the animation when jumping to the same message again: reading the layout applies the removal.
    target.getBoundingClientRect();
    target.classList.add(flash);
    return true;
}
