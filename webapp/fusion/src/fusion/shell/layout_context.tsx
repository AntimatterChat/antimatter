// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {createContext, useCallback, useContext, useMemo, useState} from 'react';
import {useSelector} from 'react-redux';

import {getIsRhsOpen} from 'selectors/rhs';

// Phone layout: the navigation and the right-hand drawer slide over the chat (see styles/_25_responsive.scss).
export const isPhoneLayout = () => window.matchMedia('(max-width: 860px)').matches;

// Layout state of the Fusion UI that only matters on this screen (the right-hand panel lives in the store's
// views.rhs, like in the classic web app).
type Layout = {

    // The sidebar shows direct messages instead of the current team's channels.
    home: boolean;
    setHome: (home: boolean) => void;

    // The member list takes the right-hand slot when no panel is open.
    showMembers: boolean;
    toggleMembers: () => void;

    // Narrow screens: the navigation (team rail + sidebar) and the right-hand drawer slide over the chat.
    navOpen: boolean;
    setNavOpen: (open: boolean) => void;
    rightOpen: boolean;
    setRightOpen: (open: boolean) => void;

    // Slides the right-hand drawer in, with the member list when no panel is open.
    openRightDrawer: () => void;
};

const LayoutContext = createContext<Layout | null>(null);

export function LayoutProvider({children}: {children: React.ReactNode}) {
    const [home, setHome] = useState(false);
    const [showMembers, setShowMembers] = useState(() => window.innerWidth > 1100);
    const [navOpen, setNavOpen] = useState(false);
    const [rightOpen, setRightOpen] = useState(false);
    const toggleMembers = useCallback(() => setShowMembers((on) => !on), []);
    const rhsOpen = useSelector(getIsRhsOpen);
    const openRightDrawer = useCallback(() => {
        if (!rhsOpen) {
            setShowMembers(true);
        }
        setNavOpen(false);
        setRightOpen(true);
    }, [rhsOpen]);

    const value = useMemo(() => ({
        home,
        setHome,
        showMembers,
        toggleMembers,
        navOpen,
        setNavOpen,
        rightOpen,
        setRightOpen,
        openRightDrawer,
    }), [home, showMembers, toggleMembers, navOpen, rightOpen, openRightDrawer]);

    return <LayoutContext.Provider value={value}>{children}</LayoutContext.Provider>;
}

export function useLayout(): Layout {
    const layout = useContext(LayoutContext);
    if (!layout) {
        throw new Error('useLayout must be used inside the Fusion UI');
    }
    return layout;
}
