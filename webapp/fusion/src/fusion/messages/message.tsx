// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Post} from '@mattermost/types/posts';

import {removePost} from 'mattermost-redux/actions/posts';
import {Posts} from 'mattermost-redux/constants';
import {isMyChannelAutotranslated} from 'mattermost-redux/selectors/entities/channels';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {getPost, isPostFlagged, isPostPriorityEnabled} from 'mattermost-redux/selectors/entities/posts';
import {isCollapsedThreadsEnabled} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentUserId, getUser} from 'mattermost-redux/selectors/entities/users';

import {toggleReaction} from 'actions/post_actions';
import {openShowEditHistory, selectPost} from 'actions/views/rhs';
import {shouldDisplayConcealedPlaceholder} from 'selectors/burn_on_read_posts';
import {getIsPostBeingEdited, getIsPostBeingEditedInRHS} from 'selectors/posts';

import MessageWithAdditionalContent from 'components/message_with_additional_content';

import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import type {IconName} from 'fusion/components/icon';
import {useDisplayName} from 'fusion/hooks/users';
import EmojiPicker from 'fusion/popovers/emoji_picker';
import UserPopover from 'fusion/popovers/user_popover';
import {am} from 'fusion/utils/class_names';
import {areConsecutivePostsBySameUser, isFromBot, isFromWebhook, isSystemMessage} from 'utils/post_utils';

import type {GlobalState} from 'types/store';

import Acknowledge from './acknowledge';
import {BurnCover, BurnTag} from './burn_on_read';
import Files from './content/files';
import MessageContent from './content/message_content';
import EditForm from './edit_form';
import {useConcernsMe} from './mentions';
import MessageMenu from './message_menu';
import Reactions from './reactions';
import ThreadSummary from './thread_summary';
import MessageTime from './time';

type Props = {
    postId: string;
    previousPostId?: string;

    // In a thread panel, messages don't offer to open their thread.
    inThread?: boolean;
    highlighted?: boolean;
};

function SystemLine({post}: {post: Post}) {
    const pluginPostTypes = useSelector((state: GlobalState) => state.plugins.postTypes);
    let icon: IconName = 'bell';
    if (post.type === Posts.POST_TYPES.JOIN_CHANNEL || post.type === Posts.POST_TYPES.ADD_TO_CHANNEL || post.type === Posts.POST_TYPES.COMBINED_USER_ACTIVITY) {
        icon = 'user-plus';
    } else if (post.type === Posts.POST_TYPES.HEADER_CHANGE || post.type === Posts.POST_TYPES.DISPLAYNAME_CHANGE || post.type === Posts.POST_TYPES.PURPOSE_CHANGE) {
        icon = 'pen';
    }
    return (
        <div
            className={am('msg', 'sysline')}
            id={`post_${post.id}`}
        >
            <span className={am('sys-ic')}><Icon name={icon}/></span>
            <span className={am('body')}>
                <MessageWithAdditionalContent
                    post={post}
                    pluginPostTypes={pluginPostTypes}
                    isRHS={false}
                    isChannelAutotranslated={false}
                />
            </span>
            <MessageTime timestamp={post.create_at}/>
        </div>
    );
}

// Message is one message in the Fusion UI's conversation: the mockup's .msg row around the classic web app's
// message body renderers (markdown, attachments, embeds, plugin post types and files).
export default function Message({postId, previousPostId, inThread = false, highlighted = false}: Props) {
    const intl = useIntl();
    const {formatMessage} = intl;
    const dispatch = useDispatch();
    const post = useSelector((state: GlobalState) => getPost(state, postId));
    const previous = useSelector((state: GlobalState) => (previousPostId ? getPost(state, previousPostId) : undefined));
    const user = useSelector((state: GlobalState) => (post ? getUser(state, post.user_id) : undefined));
    const name = useDisplayName(user);
    const me = useSelector(getCurrentUserId);
    const config = useSelector(getConfig);
    const saved = useSelector((state: GlobalState) => isPostFlagged(state, postId));
    const priorityEnabled = useSelector(isPostPriorityEnabled);
    const crt = useSelector(isCollapsedThreadsEnabled);
    const autotranslated = useSelector((state: GlobalState) => (post ? isMyChannelAutotranslated(state, post.channel_id) : false));
    const editing = useSelector((state: GlobalState) => getIsPostBeingEdited(state, postId) && getIsPostBeingEditedInRHS(state, postId) === inThread);
    const concealed = useSelector((state: GlobalState) => shouldDisplayConcealedPlaceholder(state, postId));
    const concernsMe = useConcernsMe(post, inThread);
    const avatarRef = useRef<HTMLButtonElement>(null);
    const moreRef = useRef<HTMLButtonElement>(null);
    const reactRef = useRef<HTMLButtonElement>(null);
    const [popover, setPopover] = useState<'user' | 'menu' | 'react' | null>(null);

    if (!post) {
        return null;
    }
    if (isSystemMessage(post) && post.type !== Posts.POST_TYPES.EPHEMERAL) {
        return <SystemLine post={post}/>;
    }

    const ephemeral = post.type === Posts.POST_TYPES.EPHEMERAL;
    const webhook = isFromWebhook(post) && config.EnablePostUsernameOverride === 'true' && Boolean(post.props?.override_username);
    const bot = isFromBot(post) || Boolean(user?.is_bot);
    const priority = priorityEnabled ? post.metadata?.priority?.priority : undefined;
    const burn = post.type === Posts.POST_TYPES.BURN_ON_READ && post.state !== Posts.POST_DELETED;
    const consecutive = !inThread && Boolean(previous) && !priority && !burn && !ephemeral && areConsecutivePostsBySameUser(post, previous!) && !isSystemMessage(previous!);
    const deleted = post.state === Posts.POST_DELETED;
    const openThread = () => dispatch(selectPost(post));

    const authorName = webhook ? String(post.props.override_username) : name;
    const overrideIcon = isFromWebhook(post) && config.EnablePostIconOverride === 'true' ? (post.props?.override_icon_url as string | undefined) : undefined;

    const tools = !deleted && !ephemeral && (
        <div
            className={am('msg-tools')}
            role='toolbar'
            aria-label={formatMessage({id: 'fusion.message.tools', defaultMessage: 'Message actions'})}
        >
            <button
                ref={reactRef}
                title={formatMessage({id: 'fusion.message.react', defaultMessage: 'Add reaction'})}
                aria-label={formatMessage({id: 'fusion.message.react', defaultMessage: 'Add reaction'})}
                aria-haspopup='dialog'
                onClick={() => setPopover('react')}
            >
                <Icon
                    name='smile'
                    size='sm'
                />
            </button>
            {!inThread && !burn && (
                <button
                    title={formatMessage({id: 'fusion.message.reply', defaultMessage: 'Reply in thread'})}
                    aria-label={formatMessage({id: 'fusion.message.reply', defaultMessage: 'Reply in thread'})}
                    onClick={openThread}
                >
                    <Icon
                        name='reply'
                        size='sm'
                    />
                </button>
            )}
            <button
                ref={moreRef}
                title={formatMessage({id: 'fusion.message.more', defaultMessage: 'More actions'})}
                aria-label={formatMessage({id: 'fusion.message.more', defaultMessage: 'More actions'})}
                aria-haspopup='menu'
                onClick={() => setPopover('menu')}
            >
                <Icon
                    name='dots'
                    size='sm'
                />
            </button>
        </div>
    );

    const editedTitle = formatMessage({id: 'fusion.message.editedAt', defaultMessage: 'Edited {time}'}, {time: intl.formatDate(post.edit_at, {dateStyle: 'medium', timeStyle: 'short'})});
    const tags = (
        <>
            {post.is_pinned && (
                <span className={am('meta-tag', 'pin')}>
                    <Icon name='pin'/>
                    {formatMessage({id: 'fusion.message.pinned', defaultMessage: 'Pinned'})}
                </span>
            )}
            {saved && (
                <span className={am('meta-tag', 'saved')}>
                    <Icon name='bookmark'/>
                    {formatMessage({id: 'fusion.message.saved', defaultMessage: 'Saved'})}
                </span>
            )}
            {post.edit_at > 0 && !deleted && (post.user_id === me ? (

                // Your own edits lead to the message's edit history, as in the classic web app.
                <button
                    type='button'
                    className={am('meta-tag', 'edited')}
                    title={editedTitle + ' · ' + formatMessage({id: 'fusion.message.editHistory', defaultMessage: 'Click to see the edit history'})}
                    onClick={() => dispatch(openShowEditHistory(post))}
                >
                    {formatMessage({id: 'fusion.message.edited', defaultMessage: '(edited)'})}
                </button>
            ) : (
                <span
                    className={am('meta-tag', 'edited')}
                    title={editedTitle}
                >
                    {formatMessage({id: 'fusion.message.edited', defaultMessage: '(edited)'})}
                </span>
            ))}
            {priority && (
                <span className={am('prio', priority)}>
                    <Icon name='flag'/>
                    {priority === 'urgent' ? formatMessage({id: 'fusion.message.urgent', defaultMessage: 'Urgent'}) : formatMessage({id: 'fusion.message.important', defaultMessage: 'Important'})}
                </span>
            )}
            {burn && <BurnTag post={post}/>}
        </>
    );

    const body = (
        <>
            {concealed && <BurnCover post={post}/>}
            {!concealed && editing && (
                <EditForm
                    post={post}
                    inThread={inThread}
                />
            )}
            {!concealed && !editing && (
                <div className={am('body')}>
                    <MessageContent
                        post={post}
                        inThread={inThread}
                        autotranslated={autotranslated}
                    />
                </div>
            )}
            {post.file_ids && post.file_ids.length > 0 && !deleted && !concealed && <Files post={post}/>}
            <Acknowledge post={post}/>
            <Reactions postId={post.id}/>
            {!inThread && crt && !post.root_id && <ThreadSummary post={post}/>}
        </>
    );

    const popovers = (
        <>
            {popover === 'user' && (
                <UserPopover
                    userId={post.user_id}
                    anchor={avatarRef.current}
                    onClose={() => setPopover(null)}
                />
            )}
            {popover === 'menu' && (
                <MessageMenu
                    post={post}
                    anchor={moreRef.current}
                    inThread={inThread}
                    onClose={() => setPopover(null)}
                />
            )}
            {popover === 'react' && (
                <EmojiPicker
                    anchor={reactRef.current}
                    onPick={(emojiName) => dispatch(toggleReaction(post.id, emojiName))}
                    onClose={() => setPopover(null)}
                />
            )}
        </>
    );

    const rowClass = am('msg', {
        cont: consecutive,
        ephemeral,
        'p-important': priority === 'important',
        'p-urgent': priority === 'urgent',
        'hl-me': concernsMe,
        'menu-open': popover === 'menu' || popover === 'react',
        flash: highlighted,
    });

    if (ephemeral) {
        return (
            <div
                className={rowClass}
                id={`post_${post.id}`}
            >
                <span
                    className={am('av', 'lg', 'ai-av')}
                    style={{background: 'var(--am-raise-2)', color: 'var(--am-text-2)'}}
                >
                    <Icon name='bell'/>
                </span>
                <div>
                    <div className={am('msg-head')}>
                        <b className={am('author')}>{authorName}</b>
                        {bot && <span className={am('bot-tag')}>{'BOT'}</span>}
                        <MessageTime timestamp={post.create_at}/>
                        <span className={am('eph-note')}>
                            <Icon name='lock'/>
                            {formatMessage({id: 'fusion.message.onlyYou', defaultMessage: 'Only visible to you'})}
                        </span>
                        <button
                            className={am('eph-dismiss')}
                            onClick={() => dispatch(removePost(post))}
                        >
                            {formatMessage({id: 'fusion.message.dismiss', defaultMessage: 'Dismiss'})}
                        </button>
                    </div>
                    {body}
                </div>
            </div>
        );
    }

    if (consecutive) {
        return (
            <div
                className={rowClass}
                id={`post_${post.id}`}
            >
                <MessageTime
                    timestamp={post.create_at}
                    short={true}
                    className={am('gutter')}
                />
                <div>{body}</div>
                {tools}
                {popovers}
            </div>
        );
    }

    return (
        <div
            className={rowClass}
            id={`post_${post.id}`}
            data-self={post.user_id === me || undefined}
        >
            {overrideIcon ? (
                <span
                    className={am('hook-av')}
                    aria-hidden='true'
                >
                    <img
                        src={overrideIcon}
                        alt=''
                    />
                </span>
            ) : (
                <button
                    ref={avatarRef}
                    aria-label={authorName}
                    onClick={() => setPopover('user')}
                >
                    <Avatar
                        userId={post.user_id}
                        size='lg'
                    />
                </button>
            )}
            <div>
                <div className={am('msg-head')}>
                    <button
                        className={am('author')}
                        onClick={() => setPopover('user')}
                    >
                        {authorName}
                    </button>
                    {(bot || webhook) && <span className={am('bot-tag')}>{'BOT'}</span>}
                    {webhook && <span className={am('via')}>{formatMessage({id: 'fusion.message.viaWebhook', defaultMessage: 'via webhook'})}</span>}
                    <MessageTime timestamp={post.create_at}/>
                    {tags}
                </div>
                {body}
            </div>
            {tools}
            {popovers}
        </div>
    );
}
