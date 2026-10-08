// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {createContext, useContext, useEffect, useMemo, useState} from 'react';

import SettingsModal from 'fusion/modals/settings_modal';
import type {SettingsTab} from 'fusion/modals/settings_modal';

// Opens the Fusion settings from outside React, e.g. the /settings command: detail is the tab, if any.
export const OPEN_SETTINGS_EVENT = 'am-open-settings';

type Settings = {
    open: (tab?: SettingsTab) => void;
};

const SettingsContext = createContext<Settings>({open: () => {}});

// SettingsProvider owns the Fusion UI's settings dialog.
export function SettingsProvider({children}: {children: React.ReactNode}) {
    const [tab, setTab] = useState<SettingsTab | null>(null);
    const value = useMemo(() => ({open: (t: SettingsTab = 'account') => setTab(t)}), []);
    useEffect(() => {
        const onOpen = (e: Event) => setTab((e as CustomEvent<SettingsTab | undefined>).detail || 'account');
        window.addEventListener(OPEN_SETTINGS_EVENT, onOpen);
        return () => window.removeEventListener(OPEN_SETTINGS_EVENT, onOpen);
    }, []);
    return (
        <SettingsContext.Provider value={value}>
            {children}
            {tab && (
                <SettingsModal
                    tab={tab}
                    onTab={setTab}
                    onClose={() => setTab(null)}
                />
            )}
        </SettingsContext.Provider>
    );
}

export function useSettings() {
    return useContext(SettingsContext);
}
