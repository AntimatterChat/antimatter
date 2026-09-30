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

import {Popover} from 'fusion/components/layer';
import {MenuItem, MenuSeparator} from 'fusion/components/menu';
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

    const canLeave = (channel.type === Constants.OPEN_CHANNEL && channel.name !== Constants.DEFAULT_CHANNEL) || channel.type === Constants.PRIVATE_CHANNEL;
    const leave = () => {
        if (channel.type === Constants.PRIVATE_CHANNEL || channel.policy_enforced) {
            dispatch(openDialog(ModalIdentifiers.LEAVE_PRIVATE_CHANNEL_MODAL, LeaveChannelModal, {channel}));
        } else {
            dispatch(leaveChannel(channel.id));
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
                    onClick={run(() => dispatch(readMultipleChannels([channel.id])))}
                />
            ) : (
                <MenuItem
                    icon='inbox'
                    label={formatMessage({id: 'fusion.channelMenu.markUnread', defaultMessage: 'Mark as unread'})}
                    onClick={run(() => dispatch(markMostRecentPostInChannelAsUnread(channel.id)))}
                />
            )}
            <MenuItem
                icon='star'
                label={favorite ? formatMessage({id: 'fusion.channelMenu.unfavorite', defaultMessage: 'Unfavorite'}) : formatMessage({id: 'fusion.channelMenu.favorite', defaultMessage: 'Favorite'})}
                onClick={run(() => dispatch(favorite ? unfavoriteChannel(channel.id) : favoriteChannel(channel.id)))}
            />
            <MenuItem
                icon={muted ? 'bell' : 'bell-off'}
                label={muted ? formatMessage({id: 'fusion.channelMenu.unmute', defaultMessage: 'Unmute channel'}) : formatMessage({id: 'fusion.channelMenu.mute', defaultMessage: 'Mute channel'})}
                onClick={run(() => dispatch(muted ? unmuteChannel(userId, channel.id) : muteChannel(userId, channel.id)))}
            />
            <MenuItem
                icon='folder'
                label={formatMessage({id: 'fusion.channelMenu.move', defaultMessage: 'Move to…'})}
                onClick={() => setMoving(!moving)}
            >
                <span className={am('end')}/>
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
                            onClick={run(() => dispatch(addChannelToCategory(c.id, channel.id)))}
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
                onClick={run(() => team && copyToClipboard(getSiteURL() + channelPath(team.name, channel)))}
            />
            <MenuItem
                icon='user-plus'
                label={formatMessage({id: 'fusion.channelMenu.addMembers', defaultMessage: 'Add members'})}
                onClick={run(() => dispatch(openDialog(ModalIdentifiers.CHANNEL_INVITE, ChannelInviteModal, {channel})))}
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
