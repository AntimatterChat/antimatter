// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {createContext, useContext, useEffect, useMemo, useState} from 'react';
import {useStore} from 'react-redux';

import type {GlobalState} from 'types/store';

import {isKeybind} from './keybinds';

type GlobalSearch = {
    isOpen: boolean;
    open: () => void;
    close: () => void;
};

const GlobalSearchContext = createContext<GlobalSearch>({isOpen: false, open: () => {}, close: () => {}});

// GlobalSearchProvider owns the Ctrl/⌘+K search overlay (see GlobalSearch).
export function GlobalSearchProvider({children}: {children: React.ReactNode}) {
    const [isOpen, setOpen] = useState(false);
    const store = useStore<GlobalState>();

    useEffect(() => {
        const onKeyDown = (e: KeyboardEvent) => {
            if (isKeybind(store.getState(), e, 'search')) {
                e.preventDefault();
                e.stopPropagation();
                setOpen((open) => !open);
            }
        };

        // Capture phase, so the classic channel switcher's own Ctrl+K handler never sees the event.
        document.addEventListener('keydown', onKeyDown, true);
        return () => document.removeEventListener('keydown', onKeyDown, true);
    }, [store]);

    const value = useMemo(() => ({
        isOpen,
        open: () => setOpen(true),
        close: () => setOpen(false),
    }), [isOpen]);

    return <GlobalSearchContext.Provider value={value}>{children}</GlobalSearchContext.Provider>;
}

export function useGlobalSearch() {
    return useContext(GlobalSearchContext);
}
