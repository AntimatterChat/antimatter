// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useMemo, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useDispatch, useSelector} from 'react-redux';

import type {Channel, ChannelType} from '@mattermost/types/channels';
import type {Post} from '@mattermost/types/posts';

import {addChannelMembers, createChannel} from 'mattermost-redux/actions/channels';
import {createPost} from 'mattermost-redux/actions/posts';
import {Permissions, Posts} from 'mattermost-redux/constants';
import {getChannel} from 'mattermost-redux/selectors/entities/channels';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {makeGetPostsForThread} from 'mattermost-redux/selectors/entities/posts';
import {isCollapsedThreadsEnabled} from 'mattermost-redux/selectors/entities/preferences';
import {haveITeamPermission} from 'mattermost-redux/selectors/entities/roles';
import {getTeam} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import {switchToChannel} from 'actions/views/channel';

import Icon from 'fusion/components/icon';
import {Dialog} from 'fusion/components/layer';
import {getThreadTitle} from 'fusion/messages/thread_title';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';
import {permalinkPath} from 'fusion/utils/paths';
import {plainText} from 'fusion/utils/plain_text';
import Constants from 'utils/constants';
import {canEditPost, isSystemMessage} from 'utils/post_utils';
import {cleanUpUrlable, getSiteURL} from 'utils/url';
import {generateId} from 'utils/utils';

import type {GlobalState} from 'types/store';

// A thread (the stream of replies to a message) can become a topic (it gets a name, and stays in its channel) or
// the start of a channel of its own. What the thread view offers, for its first message.
export type ThreadConversion = 'topic' | 'channel';

// useThreadConversions says which of the two the current user may do with a thread:
// - a topic, with collapsed reply threads (the sidebar lists topics under their channel), when they may edit its first
//   message (where the name is kept), and it isn't a topic yet (renameTopic once it is);
// - a channel, in a community's channel (not a direct or group message) that isn't archived, when they may create a
//   public or a private channel there.
export function useThreadConversions(root: Post): {topic: boolean; renameTopic: boolean; channel: boolean} {
    return useSelector((state: GlobalState) => {
        const channel = getChannel(state, root.channel_id);
        const plain = !isSystemMessage(root) && root.type !== Posts.POST_TYPES.BURN_ON_READ && !root.root_id;
        if (!channel || !plain) {
            return {topic: false, renameTopic: false, channel: false};
        }
        const named = Boolean(getThreadTitle(root));
        const editable = isCollapsedThreadsEnabled(state) && canEditPost(state, root, getConfig(state), channel, getCurrentUserId(state));
        const inCommunity = Boolean(channel.team_id) && channel.type !== Constants.DM_CHANNEL && channel.type !== Constants.GM_CHANNEL && channel.delete_at === 0;
        const canCreate = inCommunity && (haveITeamPermission(state, channel.team_id, Permissions.CREATE_PUBLIC_CHANNEL) || haveITeamPermission(state, channel.team_id, Permissions.CREATE_PRIVATE_CHANNEL));
        return {topic: editable && !named, renameTopic: editable && named, channel: canCreate};
    }, shallowEqual);
}

// The people who wrote in the thread, the current user aside.
function useParticipants(root: Post): string[] {
    const getPostsForThread = useMemo(() => makeGetPostsForThread(), []);
    const me = useSelector(getCurrentUserId);
    return useSelector((state: GlobalState) => {
        const ids = new Set<string>();
        for (const post of [root, ...getPostsForThread(state, root.id)]) {
            if (post.user_id && post.user_id !== me && !isSystemMessage(post)) {
                ids.add(post.user_id);
            }
        }
        return [...ids].sort();
    }, shallowEqual);
}

// ThreadToChannelDialog starts a channel from a thread: it creates the channel (named after the topic, or the first
// words of the thread), invites the people who wrote in the thread, starts it with a link back to the thread, and
// leaves a pointer to the new channel in the thread. The thread's messages stay where they are: the server can't move
// them (its move-thread API is behind a retired feature flag).
export function ThreadToChannelDialog({root, onClose}: {root: Post; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const me = useSelector(getCurrentUserId);
    const source = useSelector((state: GlobalState) => getChannel(state, root.channel_id));
    const team = useSelector((state: GlobalState) => (source ? getTeam(state, source.team_id) : undefined));
    const canPublic = useSelector((state: GlobalState) => Boolean(source) && haveITeamPermission(state, source!.team_id, Permissions.CREATE_PUBLIC_CHANNEL));
    const canPrivate = useSelector((state: GlobalState) => Boolean(source) && haveITeamPermission(state, source!.team_id, Permissions.CREATE_PRIVATE_CHANNEL));
    const participants = useParticipants(root);
    const [name, setName] = useState(() => (getThreadTitle(root) || plainText(root.message, 60)).slice(0, 64));
    const [isPrivate, setPrivate] = useState(() => (source?.type === Constants.PRIVATE_CHANNEL && canPrivate) || !canPublic);
    const [invite, setInvite] = useState(true);
    const [saving, setSaving] = useState(false);
    const [error, setError] = useState('');
    const nameRef = useRef<HTMLInputElement>(null);

    const create = async () => {
        if (!source || !team || saving) {
            return;
        }
        if (!name.trim()) {
            setError(formatMessage({id: 'fusion.createChannel.noName', defaultMessage: 'Give the channel a name'}));
            nameRef.current?.focus();
            return;
        }
        setSaving(true);
        setError('');
        const type = (isPrivate ? Constants.PRIVATE_CHANNEL : Constants.OPEN_CHANNEL) as ChannelType;
        const url = cleanUpUrlable(name) || generateId().slice(0, 26);
        const result = await dispatch(createChannel({team_id: team.id, name: url, display_name: name.trim(), purpose: '', header: '', type} as Channel, me));
        if (!result || !('data' in result) || !result.data) {
            setSaving(false);
            setError(((result as {error?: {message?: string}})?.error?.message) || formatMessage({id: 'fusion.createChannel.failed', defaultMessage: 'Something went wrong. Please try again.'}));
            return;
        }
        const created = result.data;
        if (invite && participants.length) {
            await dispatch(addChannelMembers(created.id, participants));
        }
        const link = getSiteURL() + permalinkPath(team.name, root.id);
        await dispatch(createPost({
            channel_id: created.id,
            user_id: me,
            message: formatMessage({id: 'fusion.threadToChannel.firstMessage', defaultMessage: 'Continued from a thread in #{channel}: {link}'}, {channel: source.name, link}),
        } as Post));
        await dispatch(createPost({
            channel_id: source.id,
            root_id: root.id,
            user_id: me,
            message: formatMessage({id: 'fusion.threadToChannel.pointer', defaultMessage: 'This conversation continues in #{channel}.'}, {channel: created.name}),
        } as Post));
        setSaving(false);
        dispatch(switchToChannel(created));
        toast(formatMessage({id: 'fusion.toast.created', defaultMessage: 'Created {name}'}, {name: '#' + created.display_name}));
        onClose();
    };

    const title = formatMessage({id: 'fusion.threadToChannel.title', defaultMessage: 'Make it a channel'});
    return (
        <Dialog
            label={title}
            onClose={onClose}
            onSubmit={create}
        >
            <div className={am('pane')}>
                <h2>{title}</h2>
                <p className={am('lead')}>
                    {formatMessage({id: 'fusion.threadToChannel.lead', defaultMessage: 'A new channel in {team} carries on this thread\'s conversation. It starts with a link to the thread, and the thread gets a link to it; the replies stay in the thread.'}, {team: team?.display_name || ''})}
                </p>
                <label className={am('field')}>
                    <span>{formatMessage({id: 'fusion.createChannel.name', defaultMessage: 'Name'})}</span>
                    <input
                        ref={nameRef}
                        autoFocus={true}
                        type='text'
                        value={name}
                        maxLength={64}
                        placeholder={formatMessage({id: 'fusion.createChannel.namePlaceholder', defaultMessage: 'e.g. beam-schedule'})}
                        onChange={(e) => setName(e.target.value)}
                    />
                </label>
                {(canPublic || canPrivate) && (
                    <div className={am('toggle-row')}>
                        <div className={am('txt')}>
                            <b>{formatMessage({id: 'fusion.createChannel.private', defaultMessage: 'Private channel'})}</b>
                            <span>{formatMessage({id: 'fusion.createChannel.privateDesc', defaultMessage: 'Only invited members can see and join it.'})}</span>
                        </div>
                        <button
                            type='button'
                            className={am('switch')}
                            role='switch'
                            aria-checked={isPrivate}
                            aria-label={formatMessage({id: 'fusion.createChannel.private', defaultMessage: 'Private channel'})}
                            disabled={!(canPublic && canPrivate)}
                            onClick={() => setPrivate(!isPrivate)}
                        />
                    </div>
                )}
                {participants.length > 0 && (
                    <div className={am('toggle-row')}>
                        <div className={am('txt')}>
                            <b>{formatMessage({id: 'fusion.threadToChannel.invite', defaultMessage: '{count, plural, one {Add the other person in the thread} other {Add the # other people in the thread}}'}, {count: participants.length})}</b>
                            <span>{formatMessage({id: 'fusion.threadToChannel.inviteDesc', defaultMessage: 'Everyone who wrote in it becomes a member of the new channel.'})}</span>
                        </div>
                        <button
                            type='button'
                            className={am('switch')}
                            role='switch'
                            aria-checked={invite}
                            aria-label={formatMessage({id: 'fusion.threadToChannel.invite', defaultMessage: '{count, plural, one {Add the other person in the thread} other {Add the # other people in the thread}}'}, {count: participants.length})}
                            onClick={() => setInvite(!invite)}
                        />
                    </div>
                )}
                {error && (
                    <p
                        className={am('lead')}
                        role='alert'
                        style={{color: 'var(--am-danger)', marginTop: 10}}
                    >
                        {error}
                    </p>
                )}
                <div className={am('modal-actions')}>
                    <span className={am('grow')}/>
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={onClose}
                    >
                        {formatMessage({id: 'fusion.createChannel.cancel', defaultMessage: 'Cancel'})}
                    </button>
                    <button
                        className={am('btn', 'primary')}
                        disabled={saving || (!canPublic && !canPrivate)}
                    >
                        {formatMessage({id: 'fusion.threadToChannel.create', defaultMessage: 'Create the channel'})}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}

// The threads whose suggestion was dismissed, until the page reloads.
const dismissed = new Set<string>();

// ThreadSuggestion proposes, at the top of a thread with replies that isn't a topic, to make it a topic or a channel.
export function ThreadSuggestion({root, replyCount, onConvert}: {root: Post; replyCount: number; onConvert: (to: ThreadConversion) => void}) {
    const {formatMessage} = useIntl();
    const can = useThreadConversions(root);
    const [, setDismissed] = useState(0);

    // A topic already has its own space.
    if (!replyCount || getThreadTitle(root) || (!can.topic && !can.channel) || dismissed.has(root.id)) {
        return null;
    }
    return (
        <div
            className={am('thread-to')}
            role='note'
        >
            <Icon
                name='thread'
                size='sm'
            />
            <span className={am('grow')}>{formatMessage({id: 'fusion.threadTo.lead', defaultMessage: 'Worth its own space?'})}</span>
            {can.topic && (
                <button
                    type='button'
                    className={am('thread-to-btn')}
                    onClick={() => onConvert('topic')}
                >
                    {formatMessage({id: 'fusion.threadTo.topic', defaultMessage: 'Make it a topic'})}
                </button>
            )}
            {can.channel && (
                <button
                    type='button'
                    className={am('thread-to-btn')}
                    onClick={() => onConvert('channel')}
                >
                    {formatMessage({id: 'fusion.threadTo.channel', defaultMessage: 'Make it a channel'})}
                </button>
            )}
            <button
                type='button'
                className={am('sleep-x')}
                aria-label={formatMessage({id: 'fusion.sleep.dismiss', defaultMessage: 'Dismiss'})}
                title={formatMessage({id: 'fusion.sleep.dismiss', defaultMessage: 'Dismiss'})}
                onClick={() => {
                    dismissed.add(root.id);
                    setDismissed((n) => n + 1);
                }}
            >
                <Icon
                    name='x'
                    size='xs'
                />
            </button>
        </div>
    );
}
