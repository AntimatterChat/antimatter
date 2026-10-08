// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useDispatch, useSelector, useStore} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';

import {addChannelToCategory} from 'mattermost-redux/actions/channel_categories';
import {favoriteChannel, readMultipleChannels, setMyChannelAutotranslation, unfavoriteChannel} from 'mattermost-redux/actions/channels';
import {savePreferences} from 'mattermost-redux/actions/preferences';
import {Permissions, Preferences} from 'mattermost-redux/constants';
import {getCategoryInTeamWithChannel, getManagedCategoryMappings} from 'mattermost-redux/selectors/entities/channel_categories';
import {getCurrentChannelId, getMyChannelMembership, isChannelAutotranslated, isFavoriteChannel, isMyChannelAutotranslated, isUserLanguageSupportedForAutotranslation, makeGetChannelUnreadCount} from 'mattermost-redux/selectors/entities/channels';
import {haveIChannelPermission, haveITeamPermission} from 'mattermost-redux/selectors/entities/roles';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUser, getCurrentUserId, getUser} from 'mattermost-redux/selectors/entities/users';
import {getUserIdFromChannelName, isChannelMuted} from 'mattermost-redux/utils/channel_utils';

import {markMostRecentPostInChannelAsUnread} from 'actions/post_actions';
import {leaveChannel, leaveDirectChannel} from 'actions/views/channel';
import {getCategoriesForCurrentTeam} from 'selectors/views/channel_sidebar';

import ChannelInviteModal from 'components/channel_invite_modal';
import ChannelNotificationsModal from 'components/channel_notifications_modal';
import ChannelSettingsModal from 'components/channel_settings_modal/channel_settings_modal';
import ConvertChannelModal from 'components/convert_channel_modal';
import ConvertGmToChannelModal from 'components/convert_gm_to_channel_modal';
import DeleteChannelModal from 'components/delete_channel_modal';
import DisableAutotranslationModal from 'components/disable_autotranslation_modal';
import EditChannelHeaderModal from 'components/edit_channel_header_modal';
import LeaveChannelModal from 'components/leave_channel_modal';
import UnarchiveChannelModal from 'components/unarchive_channel_modal';

import Icon from 'fusion/components/icon';
import {Popover} from 'fusion/components/layer';
import type {Placement} from 'fusion/components/layer';
import {MenuItem, MenuSeparator} from 'fusion/components/menu';
import CopyIdItem from 'fusion/popovers/copy_id_item';
import {useDialogs} from 'fusion/shell/dialogs_context';
import {useToast} from 'fusion/shell/toast_context';
import {RenameGroupDialog} from 'fusion/sidebar/group_dm';
import {hideConversation, isHidden, showConversation} from 'fusion/sidebar/hidden';
import {canFileChannel, fileChannel, fileChannelHere, findServerCategory, getServerCategories, getSidebarCategories, isServerCategory, SERVER_CATEGORY_PREFIX} from 'fusion/sidebar/server_categories';
import {getMuteEnd, muteFor, MUTE_DURATIONS, unmute} from 'fusion/sidebar/timed_mute';
import {am} from 'fusion/utils/class_names';
import {openAddToGroup, openDialog} from 'fusion/utils/modals';
import {channelPath} from 'fusion/utils/paths';
import {getHistory} from 'utils/browser_history';
import {getChannelRoutePathAndIdentifier} from 'utils/channel_utils';
import Constants, {ModalIdentifiers} from 'utils/constants';
import {canPopout, getPopoutChannelTitle, popoutChannel} from 'utils/popouts/popout_windows';
import {getSiteURL} from 'utils/url';
import {copyToClipboard} from 'utils/utils';

import type {GlobalState} from 'types/store';

type Props = {
    channel: Channel;
    anchor?: HTMLElement | null;
    point?: {x: number; y: number};
    placement?: Placement;
    onClose: () => void;
};

type Sub = 'move' | 'mute' | null;

// ChannelMenu is the ⋯ menu of a channel or a conversation: in the sidebar (also its right-click menu) and in the
// chat header.
export default function ChannelMenu({channel, anchor, point, placement = 'right', onClose}: Props) {
    const intl = useIntl();
    const {formatMessage} = intl;
    const dispatch = useDispatch();
    const store = useStore<GlobalState>();
    const toast = useToast();
    const dialogs = useDialogs();
    const [sub, setSub] = useState<Sub>(null);
    const [renaming, setRenaming] = useState(false);
    const [getUnreadCount] = useState(makeGetChannelUnreadCount);
    const me = useSelector(getCurrentUser);
    const userId = useSelector(getCurrentUserId);
    const team = useSelector(getCurrentTeam);
    const current = useSelector(getCurrentChannelId) === channel.id;
    const favorite = useSelector((state: GlobalState) => isFavoriteChannel(state, channel.id));
    const muted = useSelector((state: GlobalState) => isChannelMuted(getMyChannelMembership(state, channel.id)));
    const muteEnd = useSelector((state: GlobalState) => getMuteEnd(state, channel.id));
    const unread = useSelector((state: GlobalState) => getUnreadCount(state, channel.id)).showUnread;
    const categories = useSelector(getCategoriesForCurrentTeam);
    const currentCategory = useSelector((state: GlobalState) => (team ? getCategoryInTeamWithChannel(state, team.id, channel.id) : undefined));

    // Where the channel can move: the team's categories for everyone it may be filed in, then the person's own, unless
    // it sits in a locked category. Back to Channels unfiles it, for those allowed to.
    const sidebar = useSelector(getSidebarCategories);
    const shownIn = sidebar.find((c) => c.channel_ids.includes(channel.id));
    const listed = useSelector((state: GlobalState) => getServerCategories(state, channel.team_id));
    const filedName = useSelector((state: GlobalState) => getManagedCategoryMappings(state, channel.team_id)?.[channel.id] || '');
    const filedLocked = Boolean(filedName && findServerCategory(listed, filedName)?.locked);
    const serverTargets = useSelector((state: GlobalState) => listed.filter((c) => canFileChannel(state, channel, c.name)), shallowEqual);
    const canUnfile = useSelector((state: GlobalState) => Boolean(filedName) && canFileChannel(state, channel, ''));
    const personalTargets = filedLocked ? [] : categories.filter((c) => c.type !== 'direct_messages' && !isServerCategory(c) && !(c.type === 'channels' && filedName && !canUnfile));
    const pluginActions = useSelector((state: GlobalState) => state.plugins.components.ChannelHeader) || [];

    // Auto-translation, once a channel admin turned it on for the channel: each reader can turn it off for them.
    const channelTranslated = useSelector((state: GlobalState) => isChannelAutotranslated(state, channel.id));
    const hidden = useSelector((state: GlobalState) => isHidden(state, channel.id));
    const myTranslation = useSelector((state: GlobalState) => isMyChannelAutotranslated(state, channel.id));
    const languageSupported = useSelector(isUserLanguageSupportedForAutotranslation);
    const teammate = useSelector((state: GlobalState) => (channel.type === Constants.DM_CHANNEL ? getUser(state, getUserIdFromChannelName(getCurrentUserId(state), channel.name)) : undefined));

    const direct = channel.type === Constants.DM_CHANNEL || channel.type === Constants.GM_CHANNEL;
    const isPrivate = channel.type === Constants.PRIVATE_CHANNEL;
    const archived = channel.delete_at !== 0;
    const isDefault = channel.name === Constants.DEFAULT_CHANNEL;
    const may = useSelector((state: GlobalState) => ({
        archive: haveIChannelPermission(state, channel.team_id, channel.id, isPrivate ? Permissions.DELETE_PRIVATE_CHANNEL : Permissions.DELETE_PUBLIC_CHANNEL),
        unarchive: haveITeamPermission(state, channel.team_id, Permissions.MANAGE_TEAM),
        convert: haveIChannelPermission(state, channel.team_id, channel.id, Permissions.CONVERT_PUBLIC_CHANNEL_TO_PRIVATE),
        editHeader: haveIChannelPermission(state, channel.team_id, channel.id, isPrivate ? Permissions.MANAGE_PRIVATE_CHANNEL_PROPERTIES : Permissions.MANAGE_PUBLIC_CHANNEL_PROPERTIES),
    }), (a, b) => a.archive === b.archive && a.unarchive === b.unarchive && a.convert === b.convert && a.editHeader === b.editHeader);
    const canArchive = !direct && !archived && !isDefault && may.archive;
    const canUnarchive = !direct && archived && !isDefault && may.unarchive;
    const canConvert = channel.type === Constants.OPEN_CHANNEL && !archived && !isDefault && may.convert;
    const canEditHeader = !archived && (direct || may.editHeader);

    const run = (action: () => void) => () => {
        action();
        onClose();
    };

    const label = direct ? channel.display_name : '#' + channel.display_name;

    const canLeave = (channel.type === Constants.OPEN_CHANNEL && !isDefault) || isPrivate;
    const leave = () => {
        if (isPrivate || channel.policy_enforced) {
            dispatch(openDialog(ModalIdentifiers.LEAVE_PRIVATE_CHANNEL_MODAL, LeaveChannelModal, {channel}));
        } else {
            dispatch(leaveChannel(channel.id));
            toast(formatMessage({id: 'fusion.toast.left', defaultMessage: 'You left {name}'}, {name: label}));
        }
    };

    // Closing a conversation hides it from the open conversations, as the dock's ×; closing an archived channel goes
    // back to the team.
    const close = () => {
        if (direct) {
            const category = channel.type === Constants.DM_CHANNEL ? Preferences.CATEGORY_DIRECT_CHANNEL_SHOW : Preferences.CATEGORY_GROUP_CHANNEL_SHOW;
            const name = channel.type === Constants.DM_CHANNEL ? channel.teammate_id || getUserIdFromChannelName(userId, channel.name) : channel.id;
            dispatch(savePreferences(userId, [{user_id: userId, category, name, value: 'false'}]));
            dispatch(leaveDirectChannel(channel.name));
        }
        if ((current || !direct) && team) {
            getHistory().push(`/${team.name}`);
        }
    };

    if (renaming) {
        return (
            <RenameGroupDialog
                channel={channel}
                onClose={onClose}
            />
        );
    }

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
            {muted ? (
                <MenuItem
                    icon='bell'
                    label={direct ? formatMessage({id: 'fusion.channelMenu.unmuteConversation', defaultMessage: 'Unmute conversation'}) : formatMessage({id: 'fusion.channelMenu.unmute', defaultMessage: 'Unmute channel'})}
                    sub={muteEnd ? formatMessage({id: 'fusion.channelMenu.mutedUntil', defaultMessage: 'Muted until {time}'}, {time: intl.formatDate(muteEnd, {weekday: 'short', hour: 'numeric', minute: '2-digit'})}) : undefined}
                    onClick={run(() => {
                        dispatch(unmute(channel.id));
                        toast(formatMessage({id: 'fusion.toast.unmuted', defaultMessage: 'Unmuted'}));
                    })}
                />
            ) : (
                <MenuItem
                    icon='bell-off'
                    label={direct ? formatMessage({id: 'fusion.channelMenu.muteConversation', defaultMessage: 'Mute conversation'}) : formatMessage({id: 'fusion.channelMenu.mute', defaultMessage: 'Mute channel'})}
                    onClick={() => setSub(sub === 'mute' ? null : 'mute')}
                >
                    <span className={am('end')}>
                        <Icon
                            name='chev'
                            size='xs'
                        />
                    </span>
                </MenuItem>
            )}
            {sub === 'mute' && (
                <div className={am('sub')}>
                    {MUTE_DURATIONS.map(([duration, text]) => (
                        <button
                            key={duration}
                            type='button'
                            role='menuitem'
                            onClick={run(() => {
                                dispatch(muteFor(channel.id, duration));
                                toast(formatMessage({id: 'fusion.toast.muted', defaultMessage: 'Muted — no notifications or unread badges'}));
                            })}
                        >
                            {formatMessage(text)}
                        </button>
                    ))}
                </div>
            )}
            {channelTranslated && (
                <MenuItem
                    icon='globe'
                    role='menuitemcheckbox'
                    checked={myTranslation}
                    disabled={!languageSupported}
                    label={formatMessage({id: 'fusion.channelMenu.translate', defaultMessage: 'Translate messages'})}
                    sub={languageSupported ? formatMessage({id: 'fusion.channelMenu.translateSub', defaultMessage: 'Into your language, by the server'}) : formatMessage({id: 'fusion.channelMenu.translateUnsupported', defaultMessage: 'Your language isn\'t supported'})}
                    onClick={run(() => {
                        if (myTranslation) {
                            dispatch(openDialog(ModalIdentifiers.DISABLE_AUTOTRANSLATION_CONFIRM, DisableAutotranslationModal, {channel}));
                        } else {
                            dispatch(setMyChannelAutotranslation(channel.id, true));
                        }
                    })}
                />
            )}
            <MenuItem
                icon='bell'
                label={formatMessage({id: 'fusion.channelMenu.notifications', defaultMessage: 'Notification preferences…'})}
                onClick={run(() => dispatch(openDialog(ModalIdentifiers.CHANNEL_NOTIFICATIONS, ChannelNotificationsModal, {channel, currentUser: me})))}
            />
            {!direct && (
                <MenuItem
                    icon='folder'
                    label={formatMessage({id: 'fusion.channelMenu.move', defaultMessage: 'Move to…'})}
                    onClick={() => setSub(sub === 'move' ? null : 'move')}
                >
                    <span className={am('end')}>
                        <Icon
                            name='chev'
                            size='xs'
                        />
                    </span>
                </MenuItem>
            )}
            {sub === 'move' && (
                <div className={am('sub')}>
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
                                onClick={run(() => {
                                    if (!here) {
                                        dispatch(fileChannelHere(channel, c.name));
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
                                    toast(formatMessage({id: 'fusion.toast.moved', defaultMessage: 'Moved to {category}'}, {category: c.display_name}));
                                })}
                            >
                                {c.display_name}
                            </button>
                        );
                    })}
                    {filedLocked && <div className={am('sub-note')}>{formatMessage({id: 'fusion.channelMenu.lockedNote', defaultMessage: '{category} is locked: its channels stay there.'}, {category: filedName})}</div>}
                </div>
            )}
            {hidden ? (
                <MenuItem
                    icon='inbox'
                    label={formatMessage({id: 'fusion.channelMenu.unhide', defaultMessage: 'Show in the sidebar'})}
                    onClick={run(() => dispatch(showConversation(channel.id)))}
                />
            ) : (
                <MenuItem
                    icon='eye-off'
                    label={formatMessage({id: 'fusion.channelMenu.hide', defaultMessage: 'Hide until new messages'})}
                    sub={formatMessage({id: 'fusion.channelMenu.hideSub', defaultMessage: 'It comes back when someone writes in it'})}
                    onClick={run(() => {
                        dispatch(hideConversation(channel.id));
                        toast(formatMessage({id: 'fusion.toast.hidden', defaultMessage: 'Hidden until new messages — find it under Hidden at the end of the list'}));
                    })}
                />
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
            {canPopout() && team && (
                <MenuItem
                    icon='popout'
                    label={formatMessage({id: 'fusion.channelMenu.newWindow', defaultMessage: 'Open in new window'})}
                    onClick={run(() => {
                        const {path, identifier} = getChannelRoutePathAndIdentifier(channel, teammate?.username);
                        popoutChannel(formatMessage(getPopoutChannelTitle(channel.type)), team.name, path, identifier);
                    })}
                />
            )}
            {canEditHeader && (
                <MenuItem
                    icon='pen'
                    label={formatMessage({id: 'fusion.channelMenu.editHeader', defaultMessage: 'Edit header'})}
                    onClick={run(() => dispatch(openDialog(ModalIdentifiers.EDIT_CHANNEL_HEADER, EditChannelHeaderModal, {channel})))}
                />
            )}
            {channel.type === Constants.GM_CHANNEL && (
                <>
                    <MenuItem
                        icon='user-plus'
                        label={formatMessage({id: 'fusion.group.addPeople', defaultMessage: 'Add people…'})}
                        sub={formatMessage({id: 'fusion.group.addPeopleSub', defaultMessage: 'Starts a new group with everyone'})}
                        onClick={run(() => {
                            // The dialog adds to the conversation being read: open the group first.
                            if (team) {
                                getHistory().push(`/${team.name}/messages/${channel.name}`);
                            }
                            requestAnimationFrame(() => dispatch(openAddToGroup()));
                        })}
                    />
                    <MenuItem
                        icon='pen'
                        label={formatMessage({id: 'fusion.group.rename', defaultMessage: 'Rename group'})}
                        onClick={() => setRenaming(true)}
                    />
                    <MenuItem
                        icon='hash'
                        label={formatMessage({id: 'fusion.group.convert', defaultMessage: 'Convert to private channel'})}
                        onClick={run(() => dispatch(openDialog(ModalIdentifiers.CONVERT_GM_TO_CHANNEL, ConvertGmToChannelModal, {channel})))}
                    />
                </>
            )}
            {!direct && !archived && (
                <>
                    <MenuItem
                        icon='user-plus'
                        label={formatMessage({id: 'fusion.channelMenu.addMembers', defaultMessage: 'Add members'})}
                        onClick={run(() => (channel.type === Constants.OPEN_CHANNEL || isPrivate ? dialogs.addPeople(channel) : dispatch(openDialog(ModalIdentifiers.CHANNEL_INVITE, ChannelInviteModal, {channel}))))}
                    />
                    <MenuItem
                        icon='cog'
                        label={formatMessage({id: 'fusion.channelMenu.settings', defaultMessage: 'Channel settings'})}
                        onClick={run(() => dispatch(openDialog(ModalIdentifiers.CHANNEL_SETTINGS, ChannelSettingsModal, {channelId: channel.id, isOpen: true})))}
                    />
                </>
            )}
            {canConvert && (
                <MenuItem
                    icon='lock'
                    label={formatMessage({id: 'fusion.channelMenu.convert', defaultMessage: 'Convert to private channel…'})}
                    onClick={run(() => dispatch(openDialog(ModalIdentifiers.CONVERT_CHANNEL, ConvertChannelModal, {channelId: channel.id, channelDisplayName: channel.display_name})))}
                />
            )}
            {pluginActions.filter((action) => !action.shouldRender || action.shouldRender(store.getState())).map((action) => (
                <MenuItem
                    key={action.id}
                    icon='plug'
                    label={action.text as string}
                    onClick={run(() => action.action?.(channel.id))}
                />
            ))}
            {(direct || archived || canLeave || canArchive || canUnarchive) && <MenuSeparator/>}
            {canUnarchive && (
                <MenuItem
                    icon='folder'
                    label={formatMessage({id: 'fusion.channelMenu.unarchive', defaultMessage: 'Unarchive channel'})}
                    onClick={run(() => dispatch(openDialog(ModalIdentifiers.UNARCHIVE_CHANNEL, UnarchiveChannelModal, {channel})))}
                />
            )}
            {(direct || archived) && (
                <MenuItem
                    icon='x'
                    label={direct ? formatMessage({id: 'fusion.channelMenu.closeConversation', defaultMessage: 'Close conversation'}) : formatMessage({id: 'fusion.channelMenu.close', defaultMessage: 'Close channel'})}
                    onClick={run(close)}
                />
            )}
            {canLeave && !archived && (
                <MenuItem
                    icon='leave'
                    danger={true}
                    label={formatMessage({id: 'fusion.channelMenu.leave', defaultMessage: 'Leave channel'})}
                    onClick={run(leave)}
                />
            )}
            {canArchive && (
                <MenuItem
                    icon='trash'
                    danger={true}
                    label={formatMessage({id: 'fusion.channelMenu.archive', defaultMessage: 'Archive channel…'})}
                    onClick={run(() => dispatch(openDialog(ModalIdentifiers.DELETE_CHANNEL, DeleteChannelModal, {channel})))}
                />
            )}
            <CopyIdItem
                id={channel.id}
                onDone={onClose}
            />
        </Popover>
    );
}
