// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {createContext, useContext, useMemo, useState} from 'react';
import {useDispatch} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';

import AddPeopleDialog from 'fusion/modals/add_people_dialog';
import CreateChannelDialog from 'fusion/modals/create_channel_dialog';
import {openInvitePeople, openNewChannel} from 'fusion/utils/modals';

type Dialogs = {

    // Create a channel, in the given sidebar category.
    createChannel: (categoryId?: string) => void;

    // Add members of the team to a channel.
    addPeople: (channel: Channel) => void;

    // Invite people to the team (the classic invitation dialog, which also invites guests).
    invite: () => void;
};

const DialogsContext = createContext<Dialogs>({createChannel: () => {}, addPeople: () => {}, invite: () => {}});

type Open = {kind: 'create'; categoryId?: string} | {kind: 'add'; channel: Channel} | null;

// DialogsProvider owns the Fusion UI's own dialogs that menus and buttons open, outliving the menu that opened them.
export function DialogsProvider({children}: {children: React.ReactNode}) {
    const dispatch = useDispatch();
    const [open, setOpen] = useState<Open>(null);

    const value = useMemo<Dialogs>(() => ({
        createChannel: (categoryId) => setOpen({kind: 'create', categoryId}),
        addPeople: (channel) => setOpen({kind: 'add', channel}),
        invite: () => dispatch(openInvitePeople()),
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
        </DialogsContext.Provider>
    );
}

export function useDialogs(): Dialogs {
    return useContext(DialogsContext);
}
