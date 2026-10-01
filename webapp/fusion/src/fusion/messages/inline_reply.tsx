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
import {getCurrentUserId, getUser} from 'mattermost-redux/selectors/entities/users';
import {isPostPendingOrFailed} from 'mattermost-redux/utils/post_utils';

import Avatar from 'fusion/components/avatar';
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

// A reply notifies the quoted message's author unless its reply_to_mention prop is false. The reply bar's "@" switch
// sets it, and remembers the choice in this preference as the default of the next replies.
export const MENTION_PREFERENCE_CATEGORY = 'inline_replies';
export const MENTION_PREFERENCE_NAME = 'mention_quoted_author';

export function mentionsQuotedAuthor(post?: {props?: Record<string, unknown>}): boolean {
    return post?.props?.reply_to_mention !== false;
}

// replyProps are a draft's props replying to postId (to nothing when ''), notifying its author or not.
export function replyProps(props: Record<string, unknown> | undefined, postId: string, mention: boolean): Record<string, unknown> {
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

export type Quoted = {
    deleted: boolean;
    userId?: string;
    message: string;
    fileCount: number;
    overrideUsername?: string;

    // Set when the quoted message is loaded and was posted by an incoming webhook: replies don't notify anyone then.
    fromWebhook?: boolean;
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
            fromWebhook,
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

// useQuotedAuthor is the name to show for a quoted message's author: the webhook's name when it overrides it, else
// the author's display name, or '' while the author isn't loaded.
function useQuotedAuthor(quoted: Quoted): string {
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

    return (overrideAllowed && quoted.overrideUsername) || (user ? name : '');
}

// QuotedMessage is the "↩ Replying to Name snippet" content of the composer's reply bar.
export function QuotedMessage({quoted}: {quoted: Quoted}) {
    const {formatMessage} = useIntl();
    const who = useQuotedAuthor(quoted);

    let snippet;
    if (quoted.deleted) {
        snippet = formatMessage({id: 'fusion.inlineReply.deleted', defaultMessage: 'Original message was deleted'});
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

// QuotedRef is the content of the reference line above a reply, as in Discord: the quoted author's small picture and
// name, then the start of their message. Its label tells screen readers that the message is a reply.
export function QuotedRef({quoted}: {quoted: Quoted}) {
    const {formatMessage} = useIntl();
    const who = useQuotedAuthor(quoted);

    let snippet;
    let icon;
    if (quoted.deleted) {
        snippet = formatMessage({id: 'fusion.inlineReply.deleted', defaultMessage: 'Original message was deleted'});
    } else if (quoted.message.trim()) {
        snippet = plainText(quoted.message, 160);
    } else if (quoted.fileCount) {
        snippet = formatMessage({id: 'fusion.inlineReply.seeFiles', defaultMessage: 'Click to see attachment'});
        icon = (
            <Icon
                name='attach'
                size='xs'
            />
        );
    } else {
        snippet = formatMessage({id: 'fusion.inlineReply.seeMessage', defaultMessage: 'Click to see message'});
    }

    return (
        <>
            <span className={am('ref-sr')}>
                {who ? formatMessage({id: 'fusion.inlineReply.replyingTo', defaultMessage: 'Replying to {name}:'}, {name: who}) : formatMessage({id: 'fusion.inlineReply.replying', defaultMessage: 'Replying to:'})}
            </span>
            {quoted.userId && !quoted.deleted ? (
                <Avatar
                    userId={quoted.userId}
                    size='xs'
                    className={am('ref-av')}
                />
            ) : (
                <span
                    className={am('ref-av', 'none')}
                    aria-hidden='true'
                >
                    <Icon name='reply'/>
                </span>
            )}
            {who && <b aria-hidden='true'>{who}</b>}
            <span className={am('snip', {gone: quoted.deleted, files: Boolean(icon)})}>
                {icon}
                {snippet}
            </span>
        </>
    );
}

type MentionSwitchProps = {
    quoted: Quoted;
    on: boolean;
    onChange: (on: boolean) => void;
};

// MentionSwitch is the reply bar's "@ On" / "@ Off" switch, which tells whether the reply notifies the quoted
// message's author. It isn't shown when no one would be notified: replying to yourself, to a deleted message or to a
// webhook's message.
export function MentionSwitch({quoted, on, onChange}: MentionSwitchProps) {
    const {formatMessage} = useIntl();
    const currentUserId = useSelector(getCurrentUserId);
    const user = useSelector((state: GlobalState) => (quoted.userId ? getUser(state, quoted.userId) : undefined));
    const name = useDisplayName(user);

    if (!quoted.userId || quoted.userId === currentUserId || quoted.deleted || quoted.fromWebhook) {
        return null;
    }

    const label = on ? formatMessage({id: 'fusion.inlineReply.mention', defaultMessage: 'Mention {name}'}, {name}) : formatMessage({id: 'fusion.inlineReply.noMention', defaultMessage: "Don't mention {name}"}, {name});
    const tooltip = on ? formatMessage({id: 'fusion.inlineReply.mentionTip', defaultMessage: '{name} will be notified of your reply. Click to turn off.'}, {name}) : formatMessage({id: 'fusion.inlineReply.noMentionTip', defaultMessage: "{name} won't be notified of your reply. Click to turn on."}, {name});
    const text = on ? formatMessage({id: 'fusion.inlineReply.mentionOn', defaultMessage: '@ On'}) : formatMessage({id: 'fusion.inlineReply.mentionOff', defaultMessage: '@ Off'});

    return (
        <button
            type='button'
            className={am('mention-switch', {off: !on})}
            aria-label={label}
            title={tooltip}
            onClick={() => onChange(!on)}
        >
            {text}
        </button>
    );
}

// jumpToMessage scrolls to a message shown in the same conversation as from and makes it flash, as the mockup does
// but longer and brighter, so that it can't be missed once the smooth scroll ends; it returns false when the message
// isn't shown there.
export function jumpToMessage(from: HTMLElement, postId: string): boolean {
    const scope = from.closest(`.${am('rhs-body')}, .${am('msgs')}`);
    const target = scope?.querySelector<HTMLElement>(`[id="post_${postId}"]`);
    if (!target) {
        return false;
    }
    target.scrollIntoView({block: 'center', behavior: 'smooth'});
    const flash = am('jump-flash');
    target.classList.remove(flash);

    // Restart the animation when jumping to the same message again: reading the layout applies the removal.
    target.getBoundingClientRect();
    target.classList.add(flash);
    const done = (e: AnimationEvent) => {
        if (e.target === target) {
            target.classList.remove(flash);
            target.removeEventListener('animationend', done);
        }
    };
    target.addEventListener('animationend', done);
    return true;
}
