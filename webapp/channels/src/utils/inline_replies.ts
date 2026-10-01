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
};

// getQuotedMessage is what to show of a quoted message: the message itself when it's loaded, as it follows edits and
// deletions as they happen, else the server's description of it.
export function getQuotedMessage(state: GlobalState, postId: string, described?: PostReplyTo): QuotedMessage {
    const post = postId ? getPost(state, postId) : undefined;
    if (post) {
        if (post.state === Posts.POST_DELETED || post.delete_at || post.type === Posts.POST_TYPES.BURN_ON_READ) {
            return {deleted: true, message: '', fileCount: 0};
        }
        return {
            deleted: false,
            userId: post.user_id,
            message: post.message,
            fileCount: post.file_ids?.length || post.metadata?.files?.length || 0,
            overrideUsername: post.props?.from_webhook === 'true' && typeof post.props?.override_username === 'string' ? post.props.override_username : undefined,
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
