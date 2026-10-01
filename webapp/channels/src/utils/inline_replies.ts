// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Post, PostReplyTo} from '@mattermost/types/posts';

import {Posts} from 'mattermost-redux/constants';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {getPost} from 'mattermost-redux/selectors/entities/posts';
import {isPostPendingOrFailed} from 'mattermost-redux/utils/post_utils';

import {isSystemMessage} from 'utils/post_utils';

import type {GlobalState} from 'types/store';

// Inline replies answer one message in the channel, quoting it above the reply, without opening a thread. The reply's
// reply_to prop holds the quoted message's id; the server describes that message in the reply's metadata.reply_to.

// INLINE_REPLY_EVENT asks the composer of a conversation (a channel, or a thread when rootId is set) to quote a message.
export const INLINE_REPLY_EVENT = 'inline-reply';
export type InlineReplyEventDetail = {channelId: string; rootId: string; postId: string};

export function isInlineRepliesEnabled(state: GlobalState): boolean {
    return getConfig(state).EnableInlineReplies !== 'false';
}

// getReplyToId is the id of the message a post (or a draft) replies to inline, or ''.
export function getReplyToId(post?: {props?: Record<string, unknown>}): string {
    const id = post?.props?.reply_to;
    return typeof id === 'string' ? id : '';
}

// A reply notifies the quoted message's author unless its reply_to_mention prop is false. The message box's "@" switch
// sets it, and remembers the choice in this preference as the default of the next replies.
export const MENTION_PREFERENCE_CATEGORY = 'inline_replies';
export const MENTION_PREFERENCE_NAME = 'mention_quoted_author';

export function mentionsQuotedAuthor(post?: {props?: Record<string, unknown>}): boolean {
    return post?.props?.reply_to_mention !== false;
}

// getReplyProps are a draft's props replying to postId (to nothing when ''), notifying its author or not.
export function getReplyProps(props: Record<string, unknown> | undefined, postId: string, mention: boolean): Record<string, unknown> {
    const next = {...props};
    delete next.reply_to;
    delete next.reply_to_mention;
    if (postId) {
        next.reply_to = postId;
        if (!mention) {
            next.reply_to_mention = false;
        }
    }
    return next;
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

export function replyInline(post: Post, rootId: string) {
    window.dispatchEvent(new CustomEvent<InlineReplyEventDetail>(INLINE_REPLY_EVENT, {detail: {channelId: post.channel_id, rootId, postId: post.id}}));
}

export type QuotedMessage = {
    deleted: boolean;
    userId?: string;
    message: string;
    fileCount: number;
    overrideUsername?: string;

    // Set when the quoted message is loaded and was posted by an incoming webhook: replies don't notify anyone then.
    fromWebhook?: boolean;
};

// getQuotedMessage is what to show of a quoted message: the message itself when it's loaded, as it follows edits and
// deletions as they happen, else the server's description of it.
export function getQuotedMessage(state: GlobalState, postId: string, described?: PostReplyTo): QuotedMessage {
    const post = postId ? getPost(state, postId) : undefined;
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
            fromWebhook,
        };
    }
    if (described && described.post_id === postId && !described.deleted) {
        return {
            deleted: false,
            userId: described.user_id,
            message: described.message || '',
            fileCount: described.file_count || 0,
            overrideUsername: described.override_username,
        };
    }
    return {deleted: Boolean(described?.deleted), message: '', fileCount: 0};
}
