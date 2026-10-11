// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useMemo, useState} from 'react';
import {useSelector} from 'react-redux';

import {Preferences} from 'mattermost-redux/constants';
import {getTheme} from 'mattermost-redux/selectors/entities/preferences';

import {applyTheme} from 'utils/utils';

import type {GlobalState} from 'types/store';

import {ThemeContext} from './theme_context';

export default function ThemeProvider({children}: {children: React.ReactNode}) {
    // This keeps track of if we're in a themed part of the app. Realistically, it should only ever be 0 (unthemed) or
    // 1 (themed), but using a counter lets us handle cases where the start/stop functions are called multiple times or
    // in the wrong order.
    const [usingUserTheme, setUsingUserTheme] = useState(0);

    const theme = useSelector((state: GlobalState) => {
        if (usingUserTheme > 0) {
            return getTheme(state);
        }

        // The pages outside the app (logging in, signing up, the System Console) follow the system's light or dark
        // mode in the Fusion colours, where the classic web app shows them in Denim.
        return Preferences.THEMES.fusionSystem;
    });

    useEffect(() => {
        applyTheme(theme);
    }, [theme]);

    const context = useMemo(() => ({
        startUsingUserTheme: () => setUsingUserTheme((count) => count + 1),
        stopUsingUserTheme: () => setUsingUserTheme((count) => count - 1),
    }), []);

    return (
        <ThemeContext.Provider value={context}>
            {children}
        </ThemeContext.Provider>
    );
}
