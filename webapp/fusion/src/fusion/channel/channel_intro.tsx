// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';

import {favoriteChannel, unfavoriteChannel} from 'mattermost-redux/actions/channels';
import {Permissions} from 'mattermost-redux/constants';
import {getDirectTeammate, getMyChannelMembership, isFavoriteChannel} from 'mattermost-redux/selectors/entities/channels';
import {haveIChannelPermission} from 'mattermost-redux/selectors/entities/roles';
import {getCurrentUser, getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import {getChannelIntroOverride} from 'selectors/channel_intro';

import ChannelIntroRenderer from 'components/channel_intro_renderer/channel_intro_renderer';
import ChannelNotificationsModal from 'components/channel_notifications_modal';
import EditChannelHeaderModal from 'components/edit_channel_header_modal';

import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import type {IconName} from 'fusion/components/icon';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import {useDialogs} from 'fusion/shell/dialogs_context';
import {useConversationName} from 'fusion/sidebar/group_dm';
import {am} from 'fusion/utils/class_names';
import {openDialog} from 'fusion/utils/modals';
import Constants, {ModalIdentifiers} from 'utils/constants';

import type {GlobalState} from 'types/store';

// ChannelIntro is the start of a conversation, above its first message: what it is, and the first things to do in it
// (add people, set the header, notifications, favorite, and plugins' buttons), as the classic channel intro.
export default function ChannelIntro({channel}: {channel: Channel}) {
    const intl = useIntl();
    const {formatMessage} = intl;
    const dispatch = useDispatch();
    const dialogs = useDialogs();
    const me = useSelector(getCurrentUser);
    const myId = useSelector(getCurrentUserId);
    const teammate = useSelector((state: GlobalState) => (channel.type === Constants.DM_CHANNEL ? getDirectTeammate(state, channel.id) : undefined));
    const creator = useUser(channel.creator_id);
    const creatorName = useDisplayName(creator);
    const name = useConversationName(channel);
    const favorite = useSelector((state: GlobalState) => isFavoriteChannel(state, channel.id));
    const member = useSelector((state: GlobalState) => getMyChannelMembership(state, channel.id));
    const pluginButtons = useSelector((state: GlobalState) => state.plugins.components.ChannelIntroButton) || [];
    const isPrivate = channel.type === Constants.PRIVATE_CHANNEL;
    const direct = channel.type === Constants.DM_CHANNEL || channel.type === Constants.GM_CHANNEL;
    const archived = channel.delete_at !== 0;
    const canEditHeader = useSelector((state: GlobalState) => direct || haveIChannelPermission(state, channel.team_id, channel.id, isPrivate ? Permissions.MANAGE_PRIVATE_CHANNEL_PROPERTIES : Permissions.MANAGE_PUBLIC_CHANNEL_PROPERTIES));
    const notes = teammate?.id === myId;
    const override = useSelector((state: GlobalState) => getChannelIntroOverride(state, channel.id));

    let art;
    let title;
    let lead;
    if (channel.type === Constants.DM_CHANNEL && teammate) {
        art = (
            <Avatar
                userId={teammate.id}
                size='xl'
            />
        );
        title = name;
        lead = notes ? formatMessage({id: 'fusion.intro.notes', defaultMessage: 'Your space for notes, reminders and drafts: only you can see it. Keep a message here with “Save to notes” in its menu.'}) : formatMessage({id: 'fusion.intro.direct', defaultMessage: 'This is the start of your conversation with @{username}. Only the two of you can see it.'}, {username: teammate.username});
    } else if (channel.type === Constants.GM_CHANNEL) {
        art = <span className={am('av', 'group', 'xl')}>{channel.display_name.split(',').length}</span>;
        title = name;
        lead = formatMessage({id: 'fusion.intro.group', defaultMessage: 'This is the start of your group conversation with {names}. Only its members can see it.'}, {names: channel.display_name});
    } else {
        art = (
            <span className={am('intro-icon')}>
                <Icon name={(isPrivate ? 'lock' : 'hash') as IconName}/>
            </span>
        );
        title = formatMessage({id: 'fusion.intro.welcome', defaultMessage: 'Welcome to #{name}'}, {name: channel.display_name});
        if (channel.name === Constants.DEFAULT_CHANNEL) {
            lead = formatMessage({id: 'fusion.intro.townSquare', defaultMessage: 'This is the start of #{name}, the channel everyone in the community is in. Post here what everyone should see.'}, {name: channel.display_name});
        } else if (creator) {
            lead = formatMessage({id: 'fusion.intro.channelBy', defaultMessage: 'This is the start of the #{name} channel, created by {creator} on {date}.'}, {name: channel.display_name, creator: creatorName, date: intl.formatDate(channel.create_at, {dateStyle: 'long'})});
        } else {
            lead = formatMessage({id: 'fusion.intro.channel', defaultMessage: 'This is the start of the #{name} channel.'}, {name: channel.display_name});
        }
    }

    const action = (icon: IconName, label: string, onClick: () => void, on = false) => (
        <button
            type='button'
            className={am('btn', {on})}
            onClick={onClick}
        >
            <Icon
                name={icon}
                size='sm'
            />
            {label}
        </button>
    );

    // A plugin can draw the intro of some channels (registerChannelIntro).
    if (override) {
        return (
            <div className={am('intro', 'plugin-intro')}>
                <ChannelIntroRenderer
                    registration={override}
                    channel={channel}
                />
            </div>
        );
    }

    return (
        <div className={am('intro')}>
            {art}
            <h2>{title}</h2>
            <p>{lead}</p>
            {channel.purpose && !direct && <p className={am('intro-purpose')}>{channel.purpose}</p>}
            {!archived && (
                <div className={am('intro-actions')}>
                    {!direct && action('user-plus', formatMessage({id: 'fusion.intro.addMembers', defaultMessage: 'Add members'}), () => dialogs.addPeople(channel))}
                    {canEditHeader && !channel.header && action('pen', formatMessage({id: 'fusion.intro.setHeader', defaultMessage: 'Set a header'}), () => dispatch(openDialog(ModalIdentifiers.EDIT_CHANNEL_HEADER, EditChannelHeaderModal, {channel})))}
                    {!notes && action('bell', formatMessage({id: 'fusion.intro.notifications', defaultMessage: 'Notifications'}), () => dispatch(openDialog(ModalIdentifiers.CHANNEL_NOTIFICATIONS, ChannelNotificationsModal, {channel, currentUser: me})))}
                    {action('star', favorite ? formatMessage({id: 'fusion.intro.favorited', defaultMessage: 'Favorite'}) : formatMessage({id: 'fusion.intro.favorite', defaultMessage: 'Add to favorites'}), () => dispatch(favorite ? unfavoriteChannel(channel.id) : favoriteChannel(channel.id)), favorite)}
                    {member && pluginButtons.map((button) => (
                        <button
                            key={button.id}
                            type='button'
                            className={am('btn')}
                            onClick={() => button.action?.(channel, member)}
                        >
                            {button.icon}
                            {button.text}
                        </button>
                    ))}
                </div>
            )}
        </div>
    );
}
