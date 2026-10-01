// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {createContext, useContext, useEffect, useMemo, useState} from 'react';

import Constants from 'utils/constants';
import * as Keyboard from 'utils/keyboard';

type GlobalSearch = {
    isOpen: boolean;
    open: () => void;
    close: () => void;
};

const GlobalSearchContext = createContext<GlobalSearch>({isOpen: false, open: () => {}, close: () => {}});

// GlobalSearchProvider owns the Ctrl/⌘+K search overlay (see GlobalSearch).
export function GlobalSearchProvider({children}: {children: React.ReactNode}) {
    const [isOpen, setOpen] = useState(false);

    useEffect(() => {
        const onKeyDown = (e: KeyboardEvent) => {
            if (Keyboard.cmdOrCtrlPressed(e) && !e.shiftKey && !e.altKey && Keyboard.isKeyPressed(e, Constants.KeyCodes.K)) {
                e.preventDefault();
                e.stopPropagation();
                setOpen((open) => !open);
            }
        };

        // Capture phase, so the classic channel switcher's own Ctrl+K handler never sees the event.
        document.addEventListener('keydown', onKeyDown, true);
        return () => document.removeEventListener('keydown', onKeyDown, true);
    }, []);

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
