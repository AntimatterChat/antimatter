// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Post} from '@mattermost/types/posts';

import {markLastPostInThreadAsUnread, setThreadFollow} from 'mattermost-redux/actions/threads';
import {isCollapsedThreadsEnabled} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import {markPostAsUnread} from 'actions/post_actions';
import {manuallyMarkThreadAsUnread} from 'actions/views/threads';

import {Popover} from 'fusion/components/layer';
import {MenuItem, MenuSeparator} from 'fusion/components/menu';
import PluginMenuActions from 'fusion/messages/plugin_menu_actions';
import {useToast} from 'fusion/shell/toast_context';
import {permalinkPath} from 'fusion/utils/paths';
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
    const pluginActions = useSelector((state: GlobalState) => state.plugins.components.PostDropdownMenu || []);
    const plugins = pluginActions.filter((a) => !a.filter || a.filter(root.id));
    const can = useThreadConversions(root);

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
            {(crt && team) || can.topic || can.renameTopic || can.channel ? <MenuSeparator/> : null}
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
                label={formatMessage({id: 'fusion.messageMenu.unread', defaultMessage: 'Mark as unread'})}
                onClick={run(markUnread)}
            />
            {plugins.length > 0 && <MenuSeparator/>}
            <PluginMenuActions
                postId={root.id}
                label={formatMessage({id: 'fusion.threadMenu.more', defaultMessage: 'More actions'})}
                menuLabel={formatMessage({id: 'fusion.threadMenu.moreLabel', defaultMessage: 'More thread actions'})}
                run={run}
            />
        </Popover>
    );
}
