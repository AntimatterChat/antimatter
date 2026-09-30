// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {createContext, useContext, useMemo, useState} from 'react';

import SettingsModal from 'fusion/modals/settings_modal';
import type {SettingsTab} from 'fusion/modals/settings_modal';

type Settings = {
    open: (tab?: SettingsTab) => void;
};

const SettingsContext = createContext<Settings>({open: () => {}});

// SettingsProvider owns the Fusion UI's settings dialog.
export function SettingsProvider({children}: {children: React.ReactNode}) {
    const [tab, setTab] = useState<SettingsTab | null>(null);
    const value = useMemo(() => ({open: (t: SettingsTab = 'account') => setTab(t)}), []);
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
