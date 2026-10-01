// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {createContext, useContext, useMemo, useState} from 'react';
import {useDispatch} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';
import type {Post} from '@mattermost/types/posts';

import AddPeopleDialog from 'fusion/modals/add_people_dialog';
import CreateCategoryDialog from 'fusion/modals/create_category_dialog';
import CreateChannelDialog from 'fusion/modals/create_channel_dialog';
import DeleteMessageDialog from 'fusion/modals/delete_message_dialog';
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
};

const DialogsContext = createContext<Dialogs>({createChannel: () => {}, addPeople: () => {}, invite: () => {}, deleteMessage: () => {}, createCategory: () => {}});

type Open = {kind: 'create'; categoryId?: string} | {kind: 'add'; channel: Channel} | {kind: 'delete'; post: Post} | {kind: 'category'} | null;

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
            {open?.kind === 'category' && <CreateCategoryDialog onClose={close}/>}
        </DialogsContext.Provider>
    );
}

export function useDialogs(): Dialogs {
    return useContext(DialogsContext);
}
