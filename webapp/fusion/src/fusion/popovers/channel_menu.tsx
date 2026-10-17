// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useDispatch, useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';

import {addChannelToCategory} from 'mattermost-redux/actions/channel_categories';
import {readMultipleChannels} from 'mattermost-redux/actions/channels';
import {getCategoryInTeamWithChannel, getManagedCategoryMappings} from 'mattermost-redux/selectors/entities/channel_categories';
import {makeGetChannelUnreadCount} from 'mattermost-redux/selectors/entities/channels';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';

import {markMostRecentPostInChannelAsUnread} from 'actions/post_actions';
import {getCategoriesForCurrentTeam} from 'selectors/views/channel_sidebar';

import {ChannelActionGroups, useChannelActions} from 'fusion/channel/channel_actions';
import Icon from 'fusion/components/icon';
import {Popover} from 'fusion/components/layer';
import type {Placement} from 'fusion/components/layer';
import {MenuItem, MenuKeys} from 'fusion/components/menu';
import CopyIdItem from 'fusion/popovers/copy_id_item';
import {useToast} from 'fusion/shell/toast_context';
import {categoryName} from 'fusion/sidebar/category_name';
import {hideConversation, isHidden, showConversation} from 'fusion/sidebar/hidden';
import {canFileChannel, fileChannel, fileChannelHere, findServerCategory, getServerCategories, getSidebarCategories, isServerCategory, SERVER_CATEGORY_PREFIX} from 'fusion/sidebar/server_categories';
import {am} from 'fusion/utils/class_names';
import Constants from 'utils/constants';

import type {GlobalState} from 'types/store';

type Props = {
    channel: Channel;
    anchor?: HTMLElement | null;
    point?: {x: number; y: number};
    placement?: Placement;
    onClose: () => void;
};

// ChannelMenu is the ⋯ menu of a channel or a conversation in the sidebar (also its right-click menu): its actions as
// the menu under its name in the chat header has them (see useChannelActions), with the sidebar's own among the
// frequent ones: marking it read or unread, moving it to a category, hiding it.
export default function ChannelMenu({channel, anchor, point, placement = 'right', onClose}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const {actions, replacement} = useChannelActions(channel, onClose);
    const [moving, setMoving] = useState(false);
    const [getUnreadCount] = useState(makeGetChannelUnreadCount);
    const team = useSelector(getCurrentTeam);
    const unread = useSelector((state: GlobalState) => getUnreadCount(state, channel.id)).showUnread;
    const hidden = useSelector((state: GlobalState) => isHidden(state, channel.id));
    const categories = useSelector(getCategoriesForCurrentTeam);
    const currentCategory = useSelector((state: GlobalState) => (team ? getCategoryInTeamWithChannel(state, team.id, channel.id) : undefined));
    const direct = channel.type === Constants.DM_CHANNEL || channel.type === Constants.GM_CHANNEL;

    // Where the channel can move: the team's categories for everyone it may be filed in, then the person's own, unless
    // it sits in a locked category. Back to Channels unfiles it, for those allowed to.
    const sidebar = useSelector(getSidebarCategories);
    const shownIn = sidebar.find((c) => c.channel_ids.includes(channel.id));
    const listed = useSelector((state: GlobalState) => getServerCategories(state, channel.team_id));
    const filedName = useSelector((state: GlobalState) => getManagedCategoryMappings(state, channel.team_id)?.[channel.id] || '');
    const filedLocked = Boolean(filedName && findServerCategory(listed, filedName)?.locked);
    const serverTargets = useSelector((state: GlobalState) => listed.filter((c) => canFileChannel(state, channel, c.name)), shallowEqual);
    const canUnfile = useSelector((state: GlobalState) => Boolean(filedName) && canFileChannel(state, channel, ''));

    // Out of a locked category, only back to Channels, for those who may unfile it (team admins).
    const personalTargets = filedLocked ? categories.filter((c) => c.type === 'channels' && canUnfile) : categories.filter((c) => c.type !== 'direct_messages' && !isServerCategory(c) && !(c.type === 'channels' && filedName && !canUnfile));

    if (replacement) {
        return <>{replacement}</>;
    }

    const run = (action: () => void) => () => {
        action();
        onClose();
    };
    const label = direct ? channel.display_name : '#' + channel.display_name;

    // The sidebar's own entries, among the frequent ones: marking read first, moving and hiding last.
    const markRead = unread ? (
        <MenuItem
            action='mark-read'
            icon='check'
            label={formatMessage({id: 'fusion.channelMenu.markRead', defaultMessage: 'Mark as read'})}
            onClick={run(() => {
                dispatch(readMultipleChannels([channel.id]));
                toast(formatMessage({id: 'fusion.toast.markedRead', defaultMessage: 'Marked {name} as read'}, {name: label}));
            })}
        />
    ) : (
        <MenuItem
            action='mark-read'
            icon='inbox'
            label={formatMessage({id: 'fusion.channelMenu.markUnread', defaultMessage: 'Mark as unread'})}
            onClick={run(() => {
                dispatch(markMostRecentPostInChannelAsUnread(channel.id));
                toast(formatMessage({id: 'fusion.toast.markedUnread', defaultMessage: 'Marked {name} as unread'}, {name: label}));
            })}
        />
    );
    const organise = (
        <>
            {!direct && (
                <MenuItem
                    action='move'
                    icon='folder'
                    label={formatMessage({id: 'fusion.channelMenu.move', defaultMessage: 'Move to…'})}
                    expanded={moving}
                    onClick={() => setMoving(!moving)}
                >
                    <span className={am('end', 'sub-chev', {open: moving})}>
                        <Icon
                            name='chev'
                            size='xs'
                        />
                    </span>
                </MenuItem>
            )}
            {moving && (
                <div
                    className={am('sub')}
                    role='group'
                    aria-label={formatMessage({id: 'fusion.channelMenu.move', defaultMessage: 'Move to…'})}
                >
                    {serverTargets.length > 0 && <div className={am('sub-head')}>{formatMessage({id: 'fusion.channelMenu.moveForEveryone', defaultMessage: 'For everyone'})}</div>}
                    {serverTargets.map((c) => {
                        const here = shownIn?.id === SERVER_CATEGORY_PREFIX + c.name;
                        return (
                            <button
                                key={c.name}
                                type='button'
                                role='menuitemradio'
                                aria-checked={here}
                                className={am({cur: here})}
                                onClick={run(async () => {
                                    if (here) {
                                        return;
                                    }
                                    const result = await dispatch(fileChannelHere(channel, c.name));
                                    if ('error' in result && result.error) {
                                        toast((result.error as {message?: string}).message || formatMessage({id: 'fusion.toast.fileFailed', defaultMessage: 'The channel couldn\'t be moved.'}));
                                    } else {
                                        toast(formatMessage({id: 'fusion.toast.filed', defaultMessage: 'Moved to {category}, for everyone'}, {category: c.name}));
                                    }
                                })}
                            >
                                {c.name}
                            </button>
                        );
                    })}
                    {serverTargets.length > 0 && personalTargets.length > 0 && <div className={am('sub-head')}>{formatMessage({id: 'fusion.channelMenu.moveYours', defaultMessage: 'Yours'})}</div>}
                    {personalTargets.map((c) => {
                        const here = shownIn?.id === c.id;
                        return (
                            <button
                                key={c.id}
                                type='button'
                                role='menuitemradio'
                                aria-checked={here}
                                className={am({cur: here})}
                                onClick={run(() => {
                                    if (here) {
                                        return;
                                    }
                                    if (c.type === 'channels' && filedName) {
                                        dispatch(fileChannel(channel.id, ''));
                                    }
                                    if (c.id !== currentCategory?.id) {
                                        dispatch(addChannelToCategory(c.id, channel.id));
                                    }
                                    toast(formatMessage({id: 'fusion.toast.moved', defaultMessage: 'Moved to {category}'}, {category: categoryName({formatMessage}, c)}));
                                })}
                            >
                                {categoryName({formatMessage}, c)}
                            </button>
                        );
                    })}
                    {filedLocked && <div className={am('sub-note')}>{formatMessage({id: 'fusion.channelMenu.lockedNote', defaultMessage: '{category} is locked: its channels stay there.'}, {category: filedName})}</div>}
                </div>
            )}
            {hidden ? (
                <MenuItem
                    action='hide'
                    icon='inbox'
                    label={formatMessage({id: 'fusion.channelMenu.unhide', defaultMessage: 'Show in the sidebar'})}
                    onClick={run(() => dispatch(showConversation(channel.id)))}
                />
            ) : (
                <MenuItem
                    action='hide'
                    icon='eye-off'
                    label={formatMessage({id: 'fusion.channelMenu.hide', defaultMessage: 'Hide until new messages'})}
                    sub={formatMessage({id: 'fusion.channelMenu.hideSub', defaultMessage: 'It comes back when someone writes in it'})}
                    onClick={run(() => {
                        dispatch(hideConversation(channel.id));
                        toast(formatMessage({id: 'fusion.toast.hidden', defaultMessage: 'Hidden until new messages — find it under Hidden at the end of the list'}));
                    })}
                />
            )}
        </>
    );

    return (
        <Popover
            anchor={anchor}
            point={point}
            placement={point ? 'point' : placement}
            className='plus-pop ch-menu'
            role='menu'
            label={formatMessage({id: 'fusion.channelMenu.label', defaultMessage: 'Options for {name}'}, {name: channel.display_name})}
            onClose={onClose}
        >
            <MenuKeys>
                <ChannelActionGroups
                    actions={actions}
                    onClose={onClose}
                    extra={{quick: {before: markRead, after: organise}}}
                />
                <CopyIdItem
                    id={channel.id}
                    onDone={onClose}
                />
            </MenuKeys>
        </Popover>
    );
}
