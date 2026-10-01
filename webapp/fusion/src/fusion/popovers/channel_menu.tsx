// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';

import {addChannelToCategory} from 'mattermost-redux/actions/channel_categories';
import {favoriteChannel, readMultipleChannels, unfavoriteChannel} from 'mattermost-redux/actions/channels';
import {getCategoryInTeamWithChannel} from 'mattermost-redux/selectors/entities/channel_categories';
import {getMyChannelMembership, isFavoriteChannel, makeGetChannelUnreadCount} from 'mattermost-redux/selectors/entities/channels';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';
import {isChannelMuted} from 'mattermost-redux/utils/channel_utils';

import {muteChannel, unmuteChannel} from 'actions/channel_actions';
import {markMostRecentPostInChannelAsUnread} from 'actions/post_actions';
import {leaveChannel} from 'actions/views/channel';
import {getCategoriesForCurrentTeam} from 'selectors/views/channel_sidebar';

import ChannelInviteModal from 'components/channel_invite_modal';
import ChannelSettingsModal from 'components/channel_settings_modal/channel_settings_modal';
import LeaveChannelModal from 'components/leave_channel_modal';

import Icon from 'fusion/components/icon';
import {Popover} from 'fusion/components/layer';
import {MenuItem, MenuSeparator} from 'fusion/components/menu';
import {useDialogs} from 'fusion/shell/dialogs_context';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';
import {openDialog} from 'fusion/utils/modals';
import {channelPath} from 'fusion/utils/paths';
import Constants, {ModalIdentifiers} from 'utils/constants';
import {getSiteURL} from 'utils/url';
import {copyToClipboard} from 'utils/utils';

import type {GlobalState} from 'types/store';

type Props = {
    channel: Channel;
    anchor?: HTMLElement | null;
    point?: {x: number; y: number};
    onClose: () => void;
};

// ChannelMenu is the ⋯ menu of a sidebar channel (also its right-click menu).
export default function ChannelMenu({channel, anchor, point, onClose}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const dialogs = useDialogs();
    const [moving, setMoving] = useState(false);
    const [getUnreadCount] = useState(makeGetChannelUnreadCount);
    const userId = useSelector(getCurrentUserId);
    const team = useSelector(getCurrentTeam);
    const favorite = useSelector((state: GlobalState) => isFavoriteChannel(state, channel.id));
    const muted = useSelector((state: GlobalState) => isChannelMuted(getMyChannelMembership(state, channel.id)));
    const unread = useSelector((state: GlobalState) => getUnreadCount(state, channel.id)).showUnread;
    const categories = useSelector(getCategoriesForCurrentTeam);
    const currentCategory = useSelector((state: GlobalState) => (team ? getCategoryInTeamWithChannel(state, team.id, channel.id) : undefined));

    const run = (action: () => void) => () => {
        action();
        onClose();
    };

    const label = channel.type === Constants.OPEN_CHANNEL || channel.type === Constants.PRIVATE_CHANNEL ? '#' + channel.display_name : channel.display_name;

    const canLeave = (channel.type === Constants.OPEN_CHANNEL && channel.name !== Constants.DEFAULT_CHANNEL) || channel.type === Constants.PRIVATE_CHANNEL;
    const leave = () => {
        if (channel.type === Constants.PRIVATE_CHANNEL || channel.policy_enforced) {
            dispatch(openDialog(ModalIdentifiers.LEAVE_PRIVATE_CHANNEL_MODAL, LeaveChannelModal, {channel}));
        } else {
            dispatch(leaveChannel(channel.id));
            toast(formatMessage({id: 'fusion.toast.left', defaultMessage: 'You left {name}'}, {name: label}));
        }
    };

    return (
        <Popover
            anchor={anchor}
            point={point}
            placement={point ? 'point' : 'right'}
            className='plus-pop ch-menu'
            role='menu'
            label={formatMessage({id: 'fusion.channelMenu.label', defaultMessage: 'Options for {name}'}, {name: channel.display_name})}
            onClose={onClose}
        >
            {unread ? (
                <MenuItem
                    icon='check'
                    label={formatMessage({id: 'fusion.channelMenu.markRead', defaultMessage: 'Mark as read'})}
                    onClick={run(() => {
                        dispatch(readMultipleChannels([channel.id]));
                        toast(formatMessage({id: 'fusion.toast.markedRead', defaultMessage: 'Marked {name} as read'}, {name: label}));
                    })}
                />
            ) : (
                <MenuItem
                    icon='inbox'
                    label={formatMessage({id: 'fusion.channelMenu.markUnread', defaultMessage: 'Mark as unread'})}
                    onClick={run(() => {
                        dispatch(markMostRecentPostInChannelAsUnread(channel.id));
                        toast(formatMessage({id: 'fusion.toast.markedUnread', defaultMessage: 'Marked {name} as unread'}, {name: label}));
                    })}
                />
            )}
            <MenuItem
                icon='star'
                label={favorite ? formatMessage({id: 'fusion.channelMenu.unfavorite', defaultMessage: 'Unfavorite'}) : formatMessage({id: 'fusion.channelMenu.favorite', defaultMessage: 'Favorite'})}
                onClick={run(() => {
                    dispatch(favorite ? unfavoriteChannel(channel.id) : favoriteChannel(channel.id));
                    toast(favorite ? formatMessage({id: 'fusion.toast.unfavorited', defaultMessage: 'Removed from Favorites'}) : formatMessage({id: 'fusion.toast.favorited', defaultMessage: 'Added to Favorites'}));
                })}
            />
            <MenuItem
                icon={muted ? 'bell' : 'bell-off'}
                label={muted ? formatMessage({id: 'fusion.channelMenu.unmute', defaultMessage: 'Unmute channel'}) : formatMessage({id: 'fusion.channelMenu.mute', defaultMessage: 'Mute channel'})}
                onClick={run(() => {
                    dispatch(muted ? unmuteChannel(userId, channel.id) : muteChannel(userId, channel.id));
                    toast(muted ? formatMessage({id: 'fusion.toast.unmuted', defaultMessage: 'Unmuted'}) : formatMessage({id: 'fusion.toast.muted', defaultMessage: 'Muted — no notifications or unread badges'}));
                })}
            />
            <MenuItem
                icon='folder'
                label={formatMessage({id: 'fusion.channelMenu.move', defaultMessage: 'Move to…'})}
                onClick={() => setMoving(!moving)}
            >
                <span className={am('end')}>
                    <Icon
                        name='chev'
                        size='xs'
                    />
                </span>
            </MenuItem>
            {moving && (
                <div className={am('sub')}>
                    {categories.filter((c) => c.type !== 'direct_messages').map((c) => (
                        <button
                            key={c.id}
                            type='button'
                            role='menuitemradio'
                            aria-checked={c.id === currentCategory?.id}
                            className={am({cur: c.id === currentCategory?.id})}
                            onClick={run(() => {
                                if (c.id !== currentCategory?.id) {
                                    dispatch(addChannelToCategory(c.id, channel.id));
                                    toast(formatMessage({id: 'fusion.toast.moved', defaultMessage: 'Moved to {category}'}, {category: c.display_name}));
                                }
                            })}
                        >
                            {c.display_name}
                        </button>
                    ))}
                </div>
            )}
            <MenuSeparator/>
            <MenuItem
                icon='link'
                label={formatMessage({id: 'fusion.channelMenu.copyLink', defaultMessage: 'Copy link'})}
                onClick={run(() => {
                    if (team) {
                        copyToClipboard(getSiteURL() + channelPath(team.name, channel));
                        toast(formatMessage({id: 'fusion.toast.linkCopied', defaultMessage: 'Link copied'}));
                    }
                })}
            />
            <MenuItem
                icon='user-plus'
                label={formatMessage({id: 'fusion.channelMenu.addMembers', defaultMessage: 'Add members'})}
                onClick={run(() => (channel.type === Constants.OPEN_CHANNEL || channel.type === Constants.PRIVATE_CHANNEL ? dialogs.addPeople(channel) : dispatch(openDialog(ModalIdentifiers.CHANNEL_INVITE, ChannelInviteModal, {channel}))))}
            />
            <MenuItem
                icon='cog'
                label={formatMessage({id: 'fusion.channelMenu.settings', defaultMessage: 'Channel settings'})}
                onClick={run(() => dispatch(openDialog(ModalIdentifiers.CHANNEL_SETTINGS, ChannelSettingsModal, {channelId: channel.id, isOpen: true})))}
            />
            {canLeave && (
                <>
                    <MenuSeparator/>
                    <MenuItem
                        icon='leave'
                        danger={true}
                        label={formatMessage({id: 'fusion.channelMenu.leave', defaultMessage: 'Leave channel'})}
                        onClick={run(leave)}
                    />
                </>
            )}
        </Popover>
    );
}
