// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {createContext, useContext, useMemo, useState} from 'react';
import {useDispatch} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';
import type {Post} from '@mattermost/types/posts';

import BrowseChannelsDialog from 'fusion/channel/browse_channels_dialog';
import CategoryPermissionsDialog from 'fusion/channel/category_permissions_dialog';
import ChannelExportDialog from 'fusion/channel/channel_export_dialog';
import ChannelPermissionsDialog from 'fusion/channel/channel_permissions_dialog';
import FocusDialog from 'fusion/focus/focus_dialog';
import AddPeopleDialog from 'fusion/modals/add_people_dialog';
import CommunityNotificationsDialog from 'fusion/modals/community_notifications_dialog';
import ConfirmDialog from 'fusion/modals/confirm_dialog';
import type {ConfirmOptions} from 'fusion/modals/confirm_dialog';
import CreateCategoryDialog from 'fusion/modals/create_category_dialog';
import CreateChannelDialog from 'fusion/modals/create_channel_dialog';
import DeleteMessageDialog from 'fusion/modals/delete_message_dialog';
import type {HelpPage} from 'fusion/modals/help_dialog';
import HelpDialog from 'fusion/modals/help_dialog';
import RemoveMemberDialog from 'fusion/moderation/remove_member_dialog';
import TimeoutDialog from 'fusion/moderation/timeout_dialog';
import BlockPromptDialog from 'fusion/privacy/block_prompt';
import CreateTeamDialog from 'fusion/teams/create_team_dialog';
import SelfRolesDialog from 'fusion/teams/self_roles_dialog';
import {openInvitePeople, openNewChannel} from 'fusion/utils/modals';

type Dialogs = {

    // Create a channel, in the given sidebar category.
    createChannel: (categoryId?: string) => void;

    // Add members of the team to a channel.
    addPeople: (channel: Channel) => void;

    // Invite people to the team (the classic invitation dialog, which also invites guests).
    invite: () => void;

    // Confirm deleting a message.
    deleteMessage: (post: Post) => void;

    // Create a sidebar category.
    createCategory: () => void;

    // Rename a server category (team admins).
    renameServerCategory: (name: string) => void;

    // A channel's permissions: overrides, and the roles a private channel is for.
    channelPermissions: (channel: Channel) => void;

    // Download a channel's messages as a CSV file.
    exportChannel: (channel: Channel) => void;

    // A category for everyone's permissions, which its channels inherit (team admins).
    categoryPermissions: (teamId: string, name: string) => void;

    // Take or leave the team's roles open to all.
    selfRoles: () => void;

    // Make a new team.
    createTeam: () => void;

    // Find the team's channels and join them.
    browseChannels: () => void;

    // Moderation: time someone out of the team, remove them from it (with confirmation).
    timeOut: (userId: string) => void;
    removeFromTeam: (userId: string) => void;

    // Ask before doing something that can't be undone.
    confirm: (options: ConfirmOptions) => void;

    // The help pages, in a dialog.
    help: (page?: HelpPage) => void;

    // Offer to block someone whose message requests you keep ignoring.
    blockPrompt: (userId: string) => void;

    // Start focus mode (from the status menu).
    focus: () => void;

    // A community's own notification settings (from its menu), over your own.
    communityNotifications: (teamId: string) => void;
};

const DialogsContext = createContext<Dialogs>({createChannel: () => {}, addPeople: () => {}, invite: () => {}, deleteMessage: () => {}, createCategory: () => {}, renameServerCategory: () => {}, channelPermissions: () => {}, exportChannel: () => {}, categoryPermissions: () => {}, selfRoles: () => {}, confirm: () => {}, help: () => {}, createTeam: () => {}, timeOut: () => {}, removeFromTeam: () => {}, browseChannels: () => {}, blockPrompt: () => {}, focus: () => {}, communityNotifications: () => {}});

type Open = {kind: 'create'; categoryId?: string} | {kind: 'add'; channel: Channel} | {kind: 'delete'; post: Post} | {kind: 'category'; renaming?: string} | {kind: 'permissions'; channel: Channel} | {kind: 'export'; channel: Channel} | {kind: 'selfRoles'} | {kind: 'confirm'; options: ConfirmOptions} | {kind: 'help'; page?: HelpPage} | {kind: 'categoryPermissions'; teamId: string; name: string} | {kind: 'createTeam'} | {kind: 'timeOut'; userId: string} | {kind: 'removeFromTeam'; userId: string} | {kind: 'browse'} | {kind: 'blockPrompt'; userId: string} | {kind: 'focus'} | {kind: 'communityNotifications'; teamId: string} | null;

// DialogsProvider owns the Fusion UI's own dialogs that menus and buttons open, outliving the menu that opened them.
export function DialogsProvider({children}: {children: React.ReactNode}) {
    const dispatch = useDispatch();
    const [open, setOpen] = useState<Open>(null);

    const value = useMemo<Dialogs>(() => ({
        createChannel: (categoryId) => setOpen({kind: 'create', categoryId}),
        addPeople: (channel) => setOpen({kind: 'add', channel}),
        invite: () => dispatch(openInvitePeople()),
        deleteMessage: (post) => setOpen({kind: 'delete', post}),
        createCategory: () => setOpen({kind: 'category'}),
        renameServerCategory: (name) => setOpen({kind: 'category', renaming: name}),
        channelPermissions: (channel) => setOpen({kind: 'permissions', channel}),
        exportChannel: (channel) => setOpen({kind: 'export', channel}),
        categoryPermissions: (teamId, name) => setOpen({kind: 'categoryPermissions', teamId, name}),
        selfRoles: () => setOpen({kind: 'selfRoles'}),
        confirm: (options) => setOpen({kind: 'confirm', options}),
        help: (page) => setOpen({kind: 'help', page}),
        createTeam: () => setOpen({kind: 'createTeam'}),
        browseChannels: () => setOpen({kind: 'browse'}),
        timeOut: (userId) => setOpen({kind: 'timeOut', userId}),
        removeFromTeam: (userId) => setOpen({kind: 'removeFromTeam', userId}),
        blockPrompt: (userId) => setOpen({kind: 'blockPrompt', userId}),
        focus: () => setOpen({kind: 'focus'}),
        communityNotifications: (teamId) => setOpen({kind: 'communityNotifications', teamId}),
    }), [dispatch]);

    const close = () => setOpen(null);
    return (
        <DialogsContext.Provider value={value}>
            {children}
            {open?.kind === 'create' && (
                <CreateChannelDialog
                    categoryId={open.categoryId}
                    onMoreOptions={() => {
                        close();
                        dispatch(openNewChannel());
                    }}
                    onClose={close}
                />
            )}
            {open?.kind === 'add' && (
                <AddPeopleDialog
                    channel={open.channel}
                    onInvite={() => {
                        close();
                        value.invite();
                    }}
                    onClose={close}
                />
            )}
            {open?.kind === 'delete' && (
                <DeleteMessageDialog
                    post={open.post}
                    onClose={close}
                />
            )}
            {open?.kind === 'selfRoles' && <SelfRolesDialog onClose={close}/>}
            {open?.kind === 'createTeam' && <CreateTeamDialog onClose={close}/>}
            {open?.kind === 'browse' && <BrowseChannelsDialog onClose={close}/>}
            {open?.kind === 'focus' && <FocusDialog onClose={close}/>}
            {open?.kind === 'communityNotifications' && (
                <CommunityNotificationsDialog
                    teamId={open.teamId}
                    onClose={close}
                />
            )}
            {open?.kind === 'timeOut' && (
                <TimeoutDialog
                    userId={open.userId}
                    onClose={close}
                />
            )}
            {open?.kind === 'blockPrompt' && (
                <BlockPromptDialog
                    userId={open.userId}
                    onClose={close}
                />
            )}
            {open?.kind === 'removeFromTeam' && (
                <RemoveMemberDialog
                    userId={open.userId}
                    onClose={close}
                />
            )}
            {open?.kind === 'categoryPermissions' && (
                <CategoryPermissionsDialog
                    teamId={open.teamId}
                    name={open.name}
                    onClose={close}
                />
            )}
            {open?.kind === 'help' && (
                <HelpDialog
                    page={open.page}
                    onClose={close}
                />
            )}
            {open?.kind === 'confirm' && (
                <ConfirmDialog
                    {...open.options}
                    onClose={close}
                />
            )}
            {open?.kind === 'permissions' && (
                <ChannelPermissionsDialog
                    channel={open.channel}
                    onClose={close}
                />
            )}
            {open?.kind === 'export' && (
                <ChannelExportDialog
                    channel={open.channel}
                    onClose={close}
                />
            )}
            {open?.kind === 'category' && (
                <CreateCategoryDialog
                    renaming={open.renaming}
                    onClose={close}
                />
            )}
        </DialogsContext.Provider>
    );
}

export function useDialogs(): Dialogs {
    return useContext(DialogsContext);
}
