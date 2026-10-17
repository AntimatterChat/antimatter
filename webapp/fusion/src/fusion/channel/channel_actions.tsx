// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useId, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector, useStore} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';

import {favoriteChannel, setMyChannelAutotranslation, unfavoriteChannel} from 'mattermost-redux/actions/channels';
import {savePreferences} from 'mattermost-redux/actions/preferences';
import {patchTeam} from 'mattermost-redux/actions/teams';
import {Permissions, Preferences} from 'mattermost-redux/constants';
import {getCurrentChannelId, getMyChannelMembership, isChannelAutotranslated, isFavoriteChannel, isMyChannelAutotranslated, isUserLanguageSupportedForAutotranslation} from 'mattermost-redux/selectors/entities/channels';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {haveIChannelPermission, haveITeamPermission} from 'mattermost-redux/selectors/entities/roles';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUser, getCurrentUserId, getUser} from 'mattermost-redux/selectors/entities/users';
import {getUserIdFromChannelName, isChannelMuted} from 'mattermost-redux/utils/channel_utils';

import {leaveChannel, leaveDirectChannel} from 'actions/views/channel';

import ChannelNotificationsModal from 'components/channel_notifications_modal';
import ChannelSettingsModal from 'components/channel_settings_modal/channel_settings_modal';
import ConvertChannelModal from 'components/convert_channel_modal';
import ConvertGmToChannelModal from 'components/convert_gm_to_channel_modal';
import DeleteChannelModal from 'components/delete_channel_modal';
import DisableAutotranslationModal from 'components/disable_autotranslation_modal';
import EditChannelHeaderModal from 'components/edit_channel_header_modal';
import LeaveChannelModal from 'components/leave_channel_modal';
import UnarchiveChannelModal from 'components/unarchive_channel_modal';

import {MenuAppIcon} from 'fusion/apps/app_glyph';
import {canArchiveChannel, isAllChannelsCommunity, isCommunityChannelFor, isDefaultChannel, isFirstDefaultChannel} from 'fusion/channel/community_channels';
import Icon from 'fusion/components/icon';
import type {IconName} from 'fusion/components/icon';
import {MenuItem, MenuSeparator} from 'fusion/components/menu';
import {useCanAddPeople} from 'fusion/members/member_add_popover';
import {useDialogs} from 'fusion/shell/dialogs_context';
import {useToast} from 'fusion/shell/toast_context';
import {RenameGroupDialog} from 'fusion/sidebar/group_dm';
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
import type {ChannelHeaderAction} from 'types/store/plugins';

// The actions of a channel or a conversation, as its menus offer them: the menu under its name in the chat header
// (ChannelNameMenu) and its ⋯ menu in the sidebar (ChannelMenu), in the same groups and order, each action once, for
// those allowed to. What lives in the channel settings dialog (its name, purpose and header, public or private, its
// banner, its attributes) is reached through "Channel settings", not repeated here.

// The groups, in their order in the menus: the frequent actions, the channel's settings, the apps' actions, then
// leaving, closing or archiving it, at the bottom.
export type ChannelActionGroup = 'quick' | 'settings' | 'apps' | 'end';
export const CHANNEL_ACTION_GROUPS: ChannelActionGroup[] = ['quick', 'settings', 'apps', 'end'];

export type ChannelActionOption = {
    id: string;
    label: string;
    run: () => void;
};

export type ChannelAction = {

    // Unique in a menu: what data-action holds.
    id: string;
    group: ChannelActionGroup;
    label: React.ReactNode;
    sub?: string;
    icon?: IconName;

    // An app's action: the app whose icon it shows, in grey.
    pluginId?: string;
    danger?: boolean;
    role?: 'menuitem' | 'menuitemcheckbox';
    checked?: boolean;
    disabled?: boolean;

    // What choosing it does; the menu closes after it, unless it stays open (renaming a group replaces the menu).
    run?: () => void;
    stayOpen?: boolean;

    // Choices shown under it, in the menu, when it's chosen (a mute's durations), and their group's name.
    options?: ChannelActionOption[];
    optionsLabel?: string;
};

const NO_APP_ACTIONS: ChannelHeaderAction[] = [];

// useChannelActions gives a channel's actions, for its menus, and what replaces the menu while one of them runs in it
// (renaming a group).
export function useChannelActions(channel: Channel, onClose: () => void): {actions: ChannelAction[]; replacement: React.ReactNode} {
    const intl = useIntl();
    const {formatMessage} = intl;
    const dispatch = useDispatch();
    const store = useStore<GlobalState>();
    const toast = useToast();
    const dialogs = useDialogs();
    const [renaming, setRenaming] = useState(false);

    const me = useSelector(getCurrentUser);
    const userId = useSelector(getCurrentUserId);
    const team = useSelector(getCurrentTeam);
    const current = useSelector(getCurrentChannelId) === channel.id;
    const favorite = useSelector((state: GlobalState) => isFavoriteChannel(state, channel.id));
    const muted = useSelector((state: GlobalState) => isChannelMuted(getMyChannelMembership(state, channel.id)));
    const muteEnd = useSelector((state: GlobalState) => getMuteEnd(state, channel.id));
    const appActions = useSelector((state: GlobalState) => state.plugins.components.ChannelHeader) || NO_APP_ACTIONS;
    const teammate = useSelector((state: GlobalState) => (channel.type === Constants.DM_CHANNEL ? getUser(state, getUserIdFromChannelName(getCurrentUserId(state), channel.name)) : undefined));

    // Auto-translation, once a channel admin turned it on for the channel: each reader can turn it off for them.
    const channelTranslated = useSelector((state: GlobalState) => isChannelAutotranslated(state, channel.id));
    const myTranslation = useSelector((state: GlobalState) => isMyChannelAutotranslated(state, channel.id));
    const languageSupported = useSelector(isUserLanguageSupportedForAutotranslation);

    // The channel's admins and the team's set its permissions: overrides and, when private, the roles it's for.
    const canManageAccess = useSelector((state: GlobalState) => haveITeamPermission(state, channel.team_id, Permissions.MANAGE_TEAM) || haveIChannelPermission(state, channel.team_id, channel.id, Permissions.MANAGE_CHANNEL_ROLES));

    // Everyone who reads a channel can export its messages, unless the system admins keep that to admins.
    const canExport = useSelector((state: GlobalState) => getConfig(state).RestrictChannelExportToAdmins !== 'true') || canManageAccess;

    // Adding people to a channel takes managing its members, as the server and the member list's "+" have it; anyone
    // in a group message adds people to it (a new group with everyone).
    const canAddPeople = useCanAddPeople(channel).add;

    const direct = channel.type === Constants.DM_CHANNEL || channel.type === Constants.GM_CHANNEL;
    const isPrivate = channel.type === Constants.PRIVATE_CHANNEL;
    const archived = channel.delete_at !== 0;

    // The community's default channel can't be left nor made private; it and the channels every community starts
    // with are archived by the community's admins only. In a community that has everyone in all its channels, people
    // mute channels rather than leave them.
    const isDefault = useSelector((state: GlobalState) => isDefaultChannel(state, channel));
    const firstDefault = useSelector((state: GlobalState) => isFirstDefaultChannel(state, channel));
    const stays = useSelector((state: GlobalState) => isCommunityChannelFor(state, channel, getCurrentUserId(state)));
    const everyoneIn = useSelector((state: GlobalState) => channel.type === Constants.OPEN_CHANNEL && isAllChannelsCommunity(state, channel.team_id));
    const may = useSelector((state: GlobalState) => ({
        archive: canArchiveChannel(state, channel),
        unarchive: haveITeamPermission(state, channel.team_id, Permissions.MANAGE_TEAM),
        convert: haveIChannelPermission(state, channel.team_id, channel.id, Permissions.CONVERT_PUBLIC_CHANNEL_TO_PRIVATE),
        editHeader: haveIChannelPermission(state, channel.team_id, channel.id, isPrivate ? Permissions.MANAGE_PRIVATE_CHANNEL_PROPERTIES : Permissions.MANAGE_PUBLIC_CHANNEL_PROPERTIES),
    }), (a, b) => a.archive === b.archive && a.unarchive === b.unarchive && a.convert === b.convert && a.editHeader === b.editHeader);
    const canArchive = !direct && !archived && may.archive && (!firstDefault || may.unarchive);
    const canUnarchive = !direct && archived && may.unarchive;
    const canLeave = ((channel.type === Constants.OPEN_CHANNEL && !isDefault) || isPrivate) && !stays && !archived;

    // The settings dialog's Info tab, there for those who manage the channel's properties, has its header and its
    // public or private switch; without that tab, converting it stays here.
    const canConvert = channel.type === Constants.OPEN_CHANNEL && !archived && !isDefault && may.convert && !may.editHeader;

    if (renaming) {
        return {
            actions: [],
            replacement: (
                <RenameGroupDialog
                    channel={channel}
                    onClose={onClose}
                />
            ),
        };
    }

    const label = direct ? channel.display_name : '#' + channel.display_name;

    // Leaving asks first. A channel with a membership policy says what leaving it means there, and offers muting.
    const leave = () => {
        if (channel.policy_enforced) {
            dispatch(openDialog(ModalIdentifiers.LEAVE_PRIVATE_CHANNEL_MODAL, LeaveChannelModal, {channel}));
            return;
        }
        dialogs.confirm({
            title: formatMessage({id: 'fusion.channelMenu.leaveTitle', defaultMessage: 'Leave {name}?'}, {name: label}),
            message: isPrivate ? formatMessage({id: 'fusion.channelMenu.leavePrivateMessage', defaultMessage: 'It\'s a private channel: you\'ll need to be added again to come back.'}) : formatMessage({id: 'fusion.channelMenu.leavePublicMessage', defaultMessage: 'You can join it again at any time from Browse channels.'}),
            confirmLabel: formatMessage({id: 'fusion.channelMenu.leave', defaultMessage: 'Leave channel'}),
            danger: true,
            onConfirm: async () => {
                const result = await dispatch(leaveChannel(channel.id));
                if (!('error' in result && result.error)) {
                    toast(formatMessage({id: 'fusion.toast.left', defaultMessage: 'You left {name}'}, {name: label}));
                }
            },
        });
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

    const actions: ChannelAction[] = [];
    const add = (action: ChannelAction | false | undefined | null) => {
        if (action) {
            actions.push(action);
        }
    };

    // The frequent ones.
    add(muted ? {
        id: 'mute',
        group: 'quick',
        icon: 'bell',
        label: direct ? formatMessage({id: 'fusion.channelMenu.unmuteConversation', defaultMessage: 'Unmute conversation'}) : formatMessage({id: 'fusion.channelMenu.unmute', defaultMessage: 'Unmute channel'}),
        sub: muteEnd ? formatMessage({id: 'fusion.channelMenu.mutedUntil', defaultMessage: 'Muted until {time}'}, {time: intl.formatDate(muteEnd, {weekday: 'short', hour: 'numeric', minute: '2-digit'})}) : undefined,
        run: () => {
            dispatch(unmute(channel.id));
            toast(formatMessage({id: 'fusion.toast.unmuted', defaultMessage: 'Unmuted'}));
        },
    } : {
        id: 'mute',
        group: 'quick',
        icon: 'bell-off',
        label: direct ? formatMessage({id: 'fusion.channelMenu.muteConversation', defaultMessage: 'Mute conversation'}) : formatMessage({id: 'fusion.channelMenu.mute', defaultMessage: 'Mute channel'}),

        // In a community that has everyone in all its channels, muting is how one stays out of a channel.
        sub: stays ? formatMessage({id: 'fusion.channelMenu.muteInsteadSub', defaultMessage: 'Everyone in {community} is in all its channels'}, {community: team?.display_name || ''}) : undefined,
        optionsLabel: formatMessage({id: 'fusion.channelMenu.muteFor', defaultMessage: 'Mute for'}),
        options: MUTE_DURATIONS.map(([duration, text]) => ({
            id: `mute-${duration}`,
            label: formatMessage(text),
            run: () => {
                dispatch(muteFor(channel.id, duration));
                toast(formatMessage({id: 'fusion.toast.muted', defaultMessage: 'Muted — no notifications or unread badges'}));
            },
        })),
    });
    add({
        id: 'notifications',
        group: 'quick',
        icon: 'bell',
        label: formatMessage({id: 'fusion.channelMenu.notifications', defaultMessage: 'Notification preferences…'}),
        run: () => dispatch(openDialog(ModalIdentifiers.CHANNEL_NOTIFICATIONS, ChannelNotificationsModal, {channel, currentUser: me})),
    });
    add({
        id: 'favourite',
        group: 'quick',
        icon: 'star',
        role: 'menuitemcheckbox',
        checked: favorite,
        label: formatMessage({id: 'fusion.channelMenu.favorite', defaultMessage: 'Favourite'}),
        run: () => {
            dispatch(favorite ? unfavoriteChannel(channel.id) : favoriteChannel(channel.id));
            toast(favorite ? formatMessage({id: 'fusion.toast.unfavorited', defaultMessage: 'Removed from Favourites'}) : formatMessage({id: 'fusion.toast.favorited', defaultMessage: 'Added to Favourites'}));
        },
    });
    add({
        id: 'copy-link',
        group: 'quick',
        icon: 'link',
        label: formatMessage({id: 'fusion.channelMenu.copyLink', defaultMessage: 'Copy link'}),
        run: () => {
            if (team) {
                copyToClipboard(getSiteURL() + channelPath(team.name, channel));
                toast(formatMessage({id: 'fusion.toast.linkCopied', defaultMessage: 'Link copied'}));
            }
        },
    });

    // An archived channel still takes members, to read it (an auditor going through old information).
    add(channel.type === Constants.GM_CHANNEL && canAddPeople && {
        id: 'add-people',
        group: 'quick',
        icon: 'user-plus',
        label: formatMessage({id: 'fusion.group.addPeople', defaultMessage: 'Add people…'}),
        sub: formatMessage({id: 'fusion.group.addPeopleSub', defaultMessage: 'Starts a new group with everyone'}),
        run: () => {
            // The dialog adds to the conversation being read: open the group first.
            if (team) {
                getHistory().push(`/${team.name}/messages/${channel.name}`);
            }
            requestAnimationFrame(() => dispatch(openAddToGroup()));
        },
    });
    add(!direct && !everyoneIn && canAddPeople && {
        id: 'add-people',
        group: 'quick',
        icon: 'user-plus',
        label: formatMessage({id: 'fusion.channelMenu.addMembers', defaultMessage: 'Add members'}),
        run: () => dialogs.addPeople(channel),
    });
    add(channelTranslated && {
        id: 'translate',
        group: 'quick',
        icon: 'globe',
        role: 'menuitemcheckbox',
        checked: myTranslation,
        disabled: !languageSupported,
        label: formatMessage({id: 'fusion.channelMenu.translate', defaultMessage: 'Translate messages'}),
        sub: languageSupported ? formatMessage({id: 'fusion.channelMenu.translateSub', defaultMessage: 'Into your language, by the server'}) : formatMessage({id: 'fusion.channelMenu.translateUnsupported', defaultMessage: 'Your language isn\'t supported'}),
        run: () => {
            if (myTranslation) {
                dispatch(openDialog(ModalIdentifiers.DISABLE_AUTOTRANSLATION_CONFIRM, DisableAutotranslationModal, {channel}));
            } else {
                dispatch(setMyChannelAutotranslation(channel.id, true));
            }
        },
    });
    add(canPopout() && team && {
        id: 'new-window',
        group: 'quick',
        icon: 'popout',
        label: formatMessage({id: 'fusion.channelMenu.newWindow', defaultMessage: 'Open in new window'}),
        run: () => {
            const {path, identifier} = getChannelRoutePathAndIdentifier(channel, teammate?.username);
            popoutChannel(formatMessage(getPopoutChannelTitle(channel.type)), team.name, path, identifier);
        },
    });

    // The settings: the channel's dialog, and what isn't in it.
    add(!direct && !archived && {
        id: 'settings',
        group: 'settings',
        icon: 'cog',
        label: formatMessage({id: 'fusion.channelMenu.settings', defaultMessage: 'Channel settings'}),
        run: () => dispatch(openDialog(ModalIdentifiers.CHANNEL_SETTINGS, ChannelSettingsModal, {channelId: channel.id, isOpen: true})),
    });
    add(direct && !archived && {
        id: 'edit-header',
        group: 'settings',
        icon: 'pen',
        label: formatMessage({id: 'fusion.channelMenu.editHeader', defaultMessage: 'Edit header'}),
        run: () => dispatch(openDialog(ModalIdentifiers.EDIT_CHANNEL_HEADER, EditChannelHeaderModal, {channel})),
    });
    add(channel.type === Constants.GM_CHANNEL && {
        id: 'rename-group',
        group: 'settings',
        icon: 'pen',
        label: formatMessage({id: 'fusion.group.rename', defaultMessage: 'Rename group'}),
        stayOpen: true,
        run: () => setRenaming(true),
    });
    add(channel.type === Constants.GM_CHANNEL && {
        id: 'convert',
        group: 'settings',
        icon: 'hash',
        label: formatMessage({id: 'fusion.group.convert', defaultMessage: 'Convert to private channel'}),
        run: () => dispatch(openDialog(ModalIdentifiers.CONVERT_GM_TO_CHANNEL, ConvertGmToChannelModal, {channel})),
    });
    add(canConvert && {
        id: 'convert',
        group: 'settings',
        icon: 'lock',
        label: formatMessage({id: 'fusion.channelMenu.convert', defaultMessage: 'Convert to private channel…'}),
        run: () => dispatch(openDialog(ModalIdentifiers.CONVERT_CHANNEL, ConvertChannelModal, {channelId: channel.id, channelDisplayName: channel.display_name})),
    });
    add(!direct && !archived && canManageAccess && {
        id: 'permissions',
        group: 'settings',
        icon: 'shield',
        label: formatMessage({id: 'fusion.channelMenu.permissions', defaultMessage: 'Permissions…'}),
        sub: formatMessage({id: 'fusion.channelMenu.permissionsSub', defaultMessage: 'Who sees it, and what roles can do here'}),
        run: () => dialogs.channelPermissions(channel),
    });
    add(channel.type === Constants.OPEN_CHANNEL && !archived && !isDefault && may.unarchive && {
        id: 'make-default',
        group: 'settings',
        icon: 'flag',
        label: formatMessage({id: 'fusion.channelMenu.makeDefault', defaultMessage: 'Make it the default channel'}),
        sub: formatMessage({id: 'fusion.channelMenu.makeDefaultSub', defaultMessage: 'Newcomers land here, and everyone is in it'}),
        run: async () => {
            const result = await dispatch(patchTeam({id: channel.team_id, default_channel_id: channel.id}));
            if ('error' in result && result.error) {
                toast((result.error as {message?: string}).message || formatMessage({id: 'fusion.teamChannels.failed', defaultMessage: 'The setting couldn\'t be saved.'}));
            } else {
                toast(formatMessage({id: 'fusion.toast.madeDefault', defaultMessage: '{name} is the default channel'}, {name: label}));
            }
        },
    });
    add(canExport && {
        id: 'export',
        group: 'settings',
        icon: 'file',
        label: direct ? formatMessage({id: 'fusion.channelExport.menuConversation', defaultMessage: 'Export conversation…'}) : formatMessage({id: 'fusion.channelExport.menu', defaultMessage: 'Export channel…'}),
        run: () => dialogs.exportChannel(channel),
    });

    // The apps' actions (registerChannelHeaderMenuAction), with their apps' icons.
    for (const action of appActions) {
        if (!action.shouldRender || action.shouldRender(store.getState())) {
            add({
                id: `app:${action.id}`,
                group: 'apps',
                pluginId: action.pluginId,
                label: action.text,
                run: () => action.action?.(channel.id),
            });
        }
    }

    // Leaving, closing and archiving, at the bottom.
    add(canUnarchive && {
        id: 'unarchive',
        group: 'end',
        icon: 'folder',
        label: formatMessage({id: 'fusion.channelMenu.unarchive', defaultMessage: 'Unarchive channel'}),
        run: () => dispatch(openDialog(ModalIdentifiers.UNARCHIVE_CHANNEL, UnarchiveChannelModal, {channel})),
    });
    add((direct || archived) && {
        id: 'close',
        group: 'end',
        icon: 'x',
        label: direct ? formatMessage({id: 'fusion.channelMenu.closeConversation', defaultMessage: 'Close conversation'}) : formatMessage({id: 'fusion.channelMenu.close', defaultMessage: 'Close channel'}),
        run: close,
    });
    add(canLeave && {
        id: 'leave',
        group: 'end',
        icon: 'leave',
        danger: true,
        label: formatMessage({id: 'fusion.channelMenu.leaveAsk', defaultMessage: 'Leave channel…'}),
        run: leave,
    });
    add(canArchive && {
        id: 'archive',
        group: 'end',
        icon: 'trash',
        danger: true,
        label: formatMessage({id: 'fusion.channelMenu.archive', defaultMessage: 'Archive channel…'}),
        run: () => dispatch(openDialog(ModalIdentifiers.DELETE_CHANNEL, DeleteChannelModal, {channel})),
    });

    return {actions, replacement: null};
}

// ChannelActionItems are a group's actions as menu items. An action with choices shows them under it when chosen.
export function ChannelActionItems({actions, onClose}: {actions: ChannelAction[]; onClose: () => void}) {
    const [open, setOpen] = useState<string | null>(null);
    const done = (run?: () => void) => () => {
        run?.();
        onClose();
    };
    return (
        <>
            {actions.map((action) => {
                if (action.options) {
                    const expanded = open === action.id;
                    return (
                        <React.Fragment key={action.id}>
                            <MenuItem
                                action={action.id}
                                icon={action.icon}
                                label={action.label}
                                sub={action.sub}
                                danger={action.danger}
                                expanded={expanded}
                                onClick={() => setOpen(expanded ? null : action.id)}
                            >
                                <span className={am('end', 'sub-chev', {open: expanded})}>
                                    <Icon
                                        name='chev'
                                        size='xs'
                                    />
                                </span>
                            </MenuItem>
                            {expanded && (
                                <div
                                    className={am('sub')}
                                    role='group'
                                    aria-label={action.optionsLabel}
                                >
                                    {action.options.map((option) => (
                                        <MenuItem
                                            key={option.id}
                                            action={option.id}
                                            label={option.label}
                                            onClick={done(option.run)}
                                        />
                                    ))}
                                </div>
                            )}
                        </React.Fragment>
                    );
                }
                return (
                    <MenuItem
                        key={action.id}
                        action={action.id}
                        icon={action.icon}
                        glyph={action.pluginId ? <MenuAppIcon pluginId={action.pluginId}/> : undefined}
                        label={action.pluginId ? <span className={am('mi-label')}>{action.label}</span> : action.label}
                        sub={action.sub}
                        danger={action.danger}
                        role={action.role}
                        checked={action.checked}
                        disabled={action.disabled}
                        onClick={action.stayOpen ? action.run : done(action.run)}
                    />
                );
            })}
        </>
    );
}

// A menu's own entries in a group, before and after the shared ones (the sidebar's: marking read, moving, hiding).
export type ChannelActionExtras = Partial<Record<ChannelActionGroup, {before?: React.ReactNode; after?: React.ReactNode}>>;

// ChannelActionGroups are a menu's actions, group after group, with a line between them; the apps' under their
// heading.
export function ChannelActionGroups({actions, onClose, extra}: {actions: ChannelAction[]; onClose: () => void; extra?: ChannelActionExtras}) {
    const {formatMessage} = useIntl();
    const appsHeading = useId();
    const groups = CHANNEL_ACTION_GROUPS.
        map((group) => ({group, items: actions.filter((a) => a.group === group), more: extra?.[group]})).
        filter(({items, more}) => items.length > 0 || Boolean(more?.before || more?.after));
    return (
        <>
            {groups.map(({group, items, more}, i) => {
                const entries = (
                    <>
                        {more?.before}
                        <ChannelActionItems
                            actions={items}
                            onClose={onClose}
                        />
                        {more?.after}
                    </>
                );
                return (
                    <React.Fragment key={group}>
                        {i > 0 && <MenuSeparator/>}
                        {group === 'apps' ? (
                            <div
                                role='group'
                                aria-labelledby={appsHeading}
                            >
                                <div
                                    id={appsHeading}
                                    className={am('pop-h')}
                                >
                                    {formatMessage({id: 'fusion.channelMenu.apps', defaultMessage: 'Apps'})}
                                </div>
                                {entries}
                            </div>
                        ) : entries}
                    </React.Fragment>
                );
            })}
        </>
    );
}
