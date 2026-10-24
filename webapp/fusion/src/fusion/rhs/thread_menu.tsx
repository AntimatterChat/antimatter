// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Post} from '@mattermost/types/posts';

import {markLastPostInThreadAsUnread, setThreadFollow, updateThreadRead} from 'mattermost-redux/actions/threads';
import {isPostFlagged} from 'mattermost-redux/selectors/entities/posts';
import {isCollapsedThreadsEnabled} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {getThread} from 'mattermost-redux/selectors/entities/threads';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import {flagPost, markPostAsUnread, unflagPost} from 'actions/post_actions';
import {closeRightHandSide} from 'actions/views/rhs';
import {manuallyMarkThreadAsUnread} from 'actions/views/threads';

import {Popover} from 'fusion/components/layer';
import {MenuItem, MenuSeparator} from 'fusion/components/menu';
import PluginMenuActions, {usePluginMenuActions} from 'fusion/messages/plugin_menu_actions';
import {useToast} from 'fusion/shell/toast_context';
import {permalinkPath} from 'fusion/utils/paths';
import {getHistory} from 'utils/browser_history';
import {getSiteURL} from 'utils/url';
import {copyToClipboard} from 'utils/utils';

import type {GlobalState} from 'types/store';

import {useThreadConversions} from './thread_to';
import type {ThreadConversion} from './thread_to';

type Props = {
    root: Post;

    // When the thread's last reply was posted (or edited): marking the thread unread starts there.
    lastReplyAt: number;
    anchor: HTMLElement | null;
    onClose: () => void;

    // Opens the dialog that makes the thread a topic (or renames it) or a channel.
    onConvert: (to: ThreadConversion) => void;
};

// ThreadMenu is the thread panel's ⋯ menu: the mockup's threadMenu, with what Mattermost backs (follow, copy link,
// mark as unread, and the message actions integrations add for the thread's first message).
export default function ThreadMenu({root, lastReplyAt, anchor, onClose, onConvert}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const me = useSelector(getCurrentUserId);
    const team = useSelector(getCurrentTeam);
    const crt = useSelector(isCollapsedThreadsEnabled);
    const more = usePluginMenuActions(root);
    const can = useThreadConversions(root);
    const saved = useSelector((state: GlobalState) => isPostFlagged(state, root.id));

    // Collapsed reply threads track their own read state (no unread_replies left); without it, there's only the
    // channel's unread marker, which "Mark as unread" already covers.
    const thread = useSelector((state: GlobalState) => (crt ? getThread(state, root.id) : undefined));
    const hasUnreads = Boolean(thread && thread.unread_replies > 0);

    const run = (action: () => void) => () => {
        action();
        onClose();
    };

    const markUnread = () => {
        if (crt && team) {
            dispatch(manuallyMarkThreadAsUnread(root.id, lastReplyAt));
            dispatch(markLastPostInThreadAsUnread(me, team.id, root.id));
        } else {
            dispatch(markPostAsUnread(root));
        }
        toast(formatMessage({id: 'fusion.toast.threadUnread', defaultMessage: 'Thread marked as unread'}));
    };

    const markRead = () => {
        if (!team) {
            return;
        }
        dispatch(manuallyMarkThreadAsUnread(root.id, Date.now()));
        dispatch(updateThreadRead(me, team.id, root.id, Date.now()));
        toast(formatMessage({id: 'fusion.toast.threadRead', defaultMessage: 'Thread marked as read'}));
    };

    // Opens where the thread's first message lives, leaving the thread panel (the mockup's "Open in channel").
    const openInChannel = () => {
        if (!team) {
            return;
        }
        dispatch(closeRightHandSide());
        getHistory().push(permalinkPath(team.name, root.id));
    };

    return (
        <Popover
            anchor={anchor}
            placement='below'
            className='plus-pop thread-menu'
            role='menu'
            label={formatMessage({id: 'fusion.threadMenu.label', defaultMessage: 'Thread actions'})}
            onClose={onClose}
        >
            {crt && team && (
                <MenuItem
                    icon='follow'
                    label={root.is_following ? formatMessage({id: 'fusion.messageMenu.unfollow', defaultMessage: 'Unfollow thread'}) : formatMessage({id: 'fusion.messageMenu.follow', defaultMessage: 'Follow thread'})}
                    onClick={run(() => {
                        dispatch(setThreadFollow(me, team.id, root.id, !root.is_following));
                        toast(root.is_following ? formatMessage({id: 'fusion.toast.unfollowed', defaultMessage: 'Unfollowed thread'}) : formatMessage({id: 'fusion.toast.following', defaultMessage: 'Following thread'}));
                    })}
                />
            )}
            {team && (
                <MenuItem
                    icon='popout'
                    label={formatMessage({id: 'fusion.threadMenu.openInChannel', defaultMessage: 'Open in channel'})}
                    onClick={run(openInChannel)}
                />
            )}
            {(can.topic || can.renameTopic) && (
                <MenuItem
                    icon='thread'
                    label={can.renameTopic ? formatMessage({id: 'fusion.messageMenu.renameTopic', defaultMessage: 'Rename topic…'}) : formatMessage({id: 'fusion.messageMenu.makeTopic', defaultMessage: 'Make it a topic…'})}
                    sub={can.renameTopic ? undefined : formatMessage({id: 'fusion.messageMenu.makeTopicSub', defaultMessage: 'A named conversation, listed under the channel'})}
                    onClick={run(() => onConvert('topic'))}
                />
            )}
            {can.channel && (
                <MenuItem
                    icon='hash'
                    label={formatMessage({id: 'fusion.threadMenu.makeChannel', defaultMessage: 'Make it a channel…'})}
                    sub={formatMessage({id: 'fusion.threadMenu.makeChannelSub', defaultMessage: 'Carry on in a new channel'})}
                    onClick={run(() => onConvert('channel'))}
                />
            )}
            {team ? <MenuSeparator/> : null}
            <MenuItem
                icon='link'
                label={formatMessage({id: 'fusion.messageMenu.link', defaultMessage: 'Copy link'})}
                onClick={run(() => {
                    if (team) {
                        copyToClipboard(getSiteURL() + permalinkPath(team.name, root.id));
                        toast(formatMessage({id: 'fusion.toast.linkCopied', defaultMessage: 'Link copied'}));
                    }
                })}
            />
            <MenuItem
                icon='inbox'
                label={hasUnreads ? formatMessage({id: 'fusion.messageMenu.read', defaultMessage: 'Mark as read'}) : formatMessage({id: 'fusion.messageMenu.unread', defaultMessage: 'Mark as unread'})}
                onClick={run(hasUnreads ? markRead : markUnread)}
            />
            <MenuItem
                icon='bookmark'
                label={saved ? formatMessage({id: 'fusion.messageMenu.unsave', defaultMessage: 'Remove from saved'}) : formatMessage({id: 'fusion.messageMenu.save', defaultMessage: 'Save'})}
                onClick={run(() => {
                    dispatch(saved ? unflagPost(root.id) : flagPost(root.id));
                    toast(saved ? formatMessage({id: 'fusion.toast.unsaved', defaultMessage: 'Removed from saved'}) : formatMessage({id: 'fusion.toast.saved', defaultMessage: 'Saved — find it in Saved messages'}));
                })}
            />
            {more.any && <MenuSeparator/>}
            <PluginMenuActions
                post={root}
                label={formatMessage({id: 'fusion.threadMenu.more', defaultMessage: 'More actions'})}
                menuLabel={formatMessage({id: 'fusion.threadMenu.moreLabel', defaultMessage: 'More thread actions'})}
                run={run}
            />
        </Popover>
    );
}
