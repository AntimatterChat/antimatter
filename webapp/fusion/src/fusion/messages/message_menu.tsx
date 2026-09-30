// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Post} from '@mattermost/types/posts';

import {addPostReminder} from 'mattermost-redux/actions/posts';
import {setThreadFollow} from 'mattermost-redux/actions/threads';
import {Posts} from 'mattermost-redux/constants';
import {getChannel} from 'mattermost-redux/selectors/entities/channels';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {isPostFlagged} from 'mattermost-redux/selectors/entities/posts';
import {isCollapsedThreadsEnabled} from 'mattermost-redux/selectors/entities/preferences';
import {contentFlaggingEnabledInTeam, getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import {flagPost, markPostAsUnread, pinPost, setEditingPost, toggleReaction, unflagPost, unpinPost} from 'actions/post_actions';
import {selectPost} from 'actions/views/rhs';

import DeletePostModal from 'components/delete_post_modal';
import RenderEmoji from 'components/emoji/render_emoji';
import FlagPostModal from 'components/flag_message_modal/flag_post_modal';
import ForwardPostModal from 'components/forward_post_modal';

import {Popover} from 'fusion/components/layer';
import {Flyout, MenuItem, MenuSeparator} from 'fusion/components/menu';
import {am} from 'fusion/utils/class_names';
import {openDialog} from 'fusion/utils/modals';
import {permalinkPath} from 'fusion/utils/paths';
import {ModalIdentifiers} from 'utils/constants';
import {canDeletePost, canEditPost, isSystemMessage} from 'utils/post_utils';
import {getSiteURL} from 'utils/url';
import {copyToClipboard} from 'utils/utils';

import type {GlobalState} from 'types/store';

const QUICK_REACTIONS = ['+1', 'heart', 'joy', 'tada', 'eyes', 'white_check_mark'];

// Reminder times offered by "Remind me", like the classic web app: in 30 minutes, 1 hour, 2 hours, or tomorrow at 9:00.
function reminderTimes(): Array<[string, number]> {
    const now = Date.now();
    const tomorrow = new Date();
    tomorrow.setDate(tomorrow.getDate() + 1);
    tomorrow.setHours(9, 0, 0, 0);
    return [['30min', now + (30 * 6e4)], ['1h', now + 36e5], ['2h', now + 72e5], ['tomorrow', tomorrow.getTime()]];
}

type Props = {
    post: Post;
    anchor: HTMLElement | null;
    inThread: boolean;
    onClose: () => void;
};

// MessageMenu is a message's ⋯ menu.
export default function MessageMenu({post, anchor, inThread, onClose}: Props) {
    const intl = useIntl();
    const {formatMessage} = intl;
    const dispatch = useDispatch();
    const me = useSelector(getCurrentUserId);
    const team = useSelector(getCurrentTeam);
    const config = useSelector(getConfig);
    const channel = useSelector((state: GlobalState) => getChannel(state, post.channel_id));
    const saved = useSelector((state: GlobalState) => isPostFlagged(state, post.id));
    const crt = useSelector(isCollapsedThreadsEnabled);
    const canEdit = useSelector((state: GlobalState) => canEditPost(state, post, config, channel, me));
    const canDelete = useSelector((state: GlobalState) => canDeletePost(state, post, channel));
    const canReport = useSelector((state: GlobalState) => Boolean(channel) && !isSystemMessage(post) && contentFlaggingEnabledInTeam(state, channel!.team_id));
    const pluginActions = useSelector((state: GlobalState) => state.plugins.components.PostDropdownMenu || []);

    const run = (action: () => void) => () => {
        action();
        onClose();
    };
    const system = isSystemMessage(post);

    // Burn-on-Read messages can't be replied to or forwarded.
    const burn = post.type === Posts.POST_TYPES.BURN_ON_READ;
    const threadId = post.root_id || post.id;
    const plugins = pluginActions.filter((a) => !a.filter || a.filter(post.id));

    return (
        <Popover
            anchor={anchor}
            placement='below'
            className='plus-pop msg-menu'
            role='menu'
            label={formatMessage({id: 'fusion.messageMenu.label', defaultMessage: 'Message actions'})}
            onClose={onClose}
        >
            <div className={am('quick-react')}>
                {QUICK_REACTIONS.map((name) => (
                    <button
                        key={name}
                        aria-label={formatMessage({id: 'fusion.messageMenu.reactWith', defaultMessage: 'React with :{name}:'}, {name})}
                        onClick={run(() => dispatch(toggleReaction(post.id, name)))}
                    >
                        <RenderEmoji
                            emojiName={name}
                            size={20}
                        />
                    </button>
                ))}
            </div>
            {!inThread && !burn && (
                <MenuItem
                    icon='reply'
                    label={formatMessage({id: 'fusion.messageMenu.reply', defaultMessage: 'Reply in thread'})}
                    onClick={run(() => dispatch(selectPost(post)))}
                />
            )}
            {!system && !burn && (
                <MenuItem
                    icon='forward'
                    label={formatMessage({id: 'fusion.messageMenu.forward', defaultMessage: 'Forward'})}
                    onClick={run(() => dispatch(openDialog(ModalIdentifiers.FORWARD_POST_MODAL, ForwardPostModal, {post})))}
                />
            )}
            {crt && team && (
                <MenuItem
                    icon='follow'
                    label={post.is_following ? formatMessage({id: 'fusion.messageMenu.unfollow', defaultMessage: 'Unfollow thread'}) : formatMessage({id: 'fusion.messageMenu.follow', defaultMessage: 'Follow thread'})}
                    onClick={run(() => dispatch(setThreadFollow(me, team.id, threadId, !post.is_following)))}
                />
            )}
            <MenuItem
                icon='inbox'
                label={formatMessage({id: 'fusion.messageMenu.unread', defaultMessage: 'Mark as unread'})}
                onClick={run(() => dispatch(markPostAsUnread(post)))}
            />
            <Flyout
                icon='clock'
                label={formatMessage({id: 'fusion.messageMenu.remind', defaultMessage: 'Remind me'})}
                menuLabel={formatMessage({id: 'fusion.messageMenu.remind', defaultMessage: 'Remind me'})}
            >
                {reminderTimes().map(([key, at]) => (
                    <button
                        key={key}
                        type='button'
                        role='menuitem'
                        onClick={run(() => dispatch(addPostReminder(me, post.id, Math.floor(at / 1000))))}
                    >
                        {{
                            '30min': formatMessage({id: 'fusion.messageMenu.remind30', defaultMessage: 'In 30 minutes'}),
                            '1h': formatMessage({id: 'fusion.messageMenu.remind1h', defaultMessage: 'In 1 hour'}),
                            '2h': formatMessage({id: 'fusion.messageMenu.remind2h', defaultMessage: 'In 2 hours'}),
                            tomorrow: formatMessage({id: 'fusion.messageMenu.remindTomorrow', defaultMessage: 'Tomorrow at 9:00'}),
                        }[key]}
                    </button>
                ))}
            </Flyout>
            <MenuItem
                icon='bookmark'
                label={saved ? formatMessage({id: 'fusion.messageMenu.unsave', defaultMessage: 'Remove from saved'}) : formatMessage({id: 'fusion.messageMenu.save', defaultMessage: 'Save'})}
                onClick={run(() => dispatch(saved ? unflagPost(post.id) : flagPost(post.id)))}
            />
            {!system && (
                <MenuItem
                    icon='pin'
                    label={post.is_pinned ? formatMessage({id: 'fusion.messageMenu.unpin', defaultMessage: 'Unpin from channel'}) : formatMessage({id: 'fusion.messageMenu.pin', defaultMessage: 'Pin to channel'})}
                    onClick={run(() => dispatch(post.is_pinned ? unpinPost(post.id) : pinPost(post.id)))}
                />
            )}
            <MenuSeparator/>
            <MenuItem
                icon='link'
                label={formatMessage({id: 'fusion.messageMenu.link', defaultMessage: 'Copy link'})}
                onClick={run(() => team && copyToClipboard(getSiteURL() + permalinkPath(team.name, post.id)))}
            />
            <MenuItem
                icon='copy'
                label={formatMessage({id: 'fusion.messageMenu.copy', defaultMessage: 'Copy text'})}
                onClick={run(() => copyToClipboard(post.message))}
            />
            {plugins.length > 0 && (
                <Flyout
                    icon='plug'
                    label={formatMessage({id: 'fusion.messageMenu.plugins', defaultMessage: 'Message actions'})}
                    menuLabel={formatMessage({id: 'fusion.messageMenu.pluginsLabel', defaultMessage: 'Message actions from integrations'})}
                >
                    {plugins.map((action) => (
                        <button
                            key={action.id}
                            type='button'
                            role='menuitem'
                            onClick={run(() => action.action(post.id))}
                        >
                            {action.text}
                        </button>
                    ))}
                </Flyout>
            )}
            {(canEdit || canDelete || (canReport && post.user_id !== me)) && <MenuSeparator/>}
            {canEdit && (
                <MenuItem
                    icon='pen'
                    label={formatMessage({id: 'fusion.messageMenu.edit', defaultMessage: 'Edit'})}
                    onClick={run(() => dispatch(setEditingPost(post.id, '', inThread)))}
                />
            )}
            {canDelete && (
                <MenuItem
                    icon='trash'
                    danger={true}
                    label={formatMessage({id: 'fusion.messageMenu.delete', defaultMessage: 'Delete'})}
                    onClick={run(() => dispatch(openDialog(ModalIdentifiers.DELETE_POST, DeletePostModal, {post, isRHS: inThread})))}
                />
            )}
            {canReport && post.user_id !== me && (
                <MenuItem
                    icon='flag'
                    danger={true}
                    label={formatMessage({id: 'fusion.messageMenu.report', defaultMessage: 'Report message'})}
                    onClick={run(() => dispatch(openDialog(ModalIdentifiers.FLAG_POST, FlagPostModal, {postId: post.id})))}
                />
            )}
        </Popover>
    );
}
