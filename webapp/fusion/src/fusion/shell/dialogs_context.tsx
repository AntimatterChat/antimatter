// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {createContext, useContext, useMemo, useState} from 'react';
import {useDispatch} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';
import type {Post} from '@mattermost/types/posts';

import ChannelPermissionsDialog from 'fusion/channel/channel_permissions_dialog';
import AddPeopleDialog from 'fusion/modals/add_people_dialog';
import ConfirmDialog from 'fusion/modals/confirm_dialog';
import type {ConfirmOptions} from 'fusion/modals/confirm_dialog';
import CreateCategoryDialog from 'fusion/modals/create_category_dialog';
import CreateChannelDialog from 'fusion/modals/create_channel_dialog';
import DeleteMessageDialog from 'fusion/modals/delete_message_dialog';
import OnboardingEditor from 'fusion/teams/onboarding_editor';
import RolesDialog from 'fusion/teams/roles_dialog';
import SelfRolesDialog from 'fusion/teams/self_roles_dialog';
import TeamLayoutDialog from 'fusion/teams/team_layout_dialog';
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

    // Set up the team's roles (team admins).
    roles: () => void;

    // A channel's permissions: overrides, and the roles a private channel is for.
    channelPermissions: (channel: Channel) => void;

    // Take or leave the team's roles open to all.
    selfRoles: () => void;

    // Write the team's onboarding (team admins).
    onboarding: () => void;

    // Set the team up from a preset or template, or export it (team admins).
    teamLayout: () => void;

    // Ask before doing something that can't be undone.
    confirm: (options: ConfirmOptions) => void;
};

const DialogsContext = createContext<Dialogs>({createChannel: () => {}, addPeople: () => {}, invite: () => {}, deleteMessage: () => {}, createCategory: () => {}, renameServerCategory: () => {}, roles: () => {}, channelPermissions: () => {}, selfRoles: () => {}, onboarding: () => {}, teamLayout: () => {}, confirm: () => {}});

type Open = {kind: 'create'; categoryId?: string} | {kind: 'add'; channel: Channel} | {kind: 'delete'; post: Post} | {kind: 'category'; renaming?: string} | {kind: 'roles'} | {kind: 'permissions'; channel: Channel} | {kind: 'selfRoles'} | {kind: 'onboarding'} | {kind: 'layout'} | {kind: 'confirm'; options: ConfirmOptions} | null;

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
        roles: () => setOpen({kind: 'roles'}),
        channelPermissions: (channel) => setOpen({kind: 'permissions', channel}),
        selfRoles: () => setOpen({kind: 'selfRoles'}),
        onboarding: () => setOpen({kind: 'onboarding'}),
        teamLayout: () => setOpen({kind: 'layout'}),
        confirm: (options) => setOpen({kind: 'confirm', options}),
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
            {open?.kind === 'roles' && <RolesDialog onClose={close}/>}
            {open?.kind === 'selfRoles' && <SelfRolesDialog onClose={close}/>}
            {open?.kind === 'onboarding' && <OnboardingEditor onClose={close}/>}
            {open?.kind === 'layout' && <TeamLayoutDialog onClose={close}/>}
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
