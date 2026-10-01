// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect} from 'react';
import {useDispatch, useSelector, useStore} from 'react-redux';

import {fetchMyCategories} from 'mattermost-redux/actions/channel_categories';
import {getTheme} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentTeamId} from 'mattermost-redux/selectors/entities/teams';

import {getIsRhsOpen} from 'selectors/rhs';

import AppRail from 'fusion/apps/app_rail';
import CallWindow from 'fusion/calls/call_window';
import {IconSprite} from 'fusion/components/icon';
import {LAYER_ID} from 'fusion/components/layer';
import Toasts from 'fusion/components/toast';
import MemberList from 'fusion/members/member_list';
import ServerRail from 'fusion/rail/server_rail';
import RightPanel from 'fusion/rhs/right_panel';
import GlobalSearch from 'fusion/search/global_search';
import HomeSidebar from 'fusion/sidebar/home_sidebar';
import TeamSidebar from 'fusion/sidebar/team_sidebar';
import {am} from 'fusion/utils/class_names';
import {openKeyboardShortcuts} from 'fusion/utils/modals';
import Constants from 'utils/constants';
import {cmdOrCtrlPressed, isKeyPressed} from 'utils/keyboard';
import {applyTheme} from 'utils/utils';

import type {GlobalState} from 'types/store';

import {GlobalSearchProvider, useGlobalSearch} from './global_search_context';
import {LayoutProvider, useLayout} from './layout_context';
import {SettingsProvider, useSettings} from './settings_context';

import 'fusion/styles/_module.scss';

type Props = {
    children: React.ReactNode;
};

function Frame({children}: Props) {
    const layout = useLayout();
    const search = useGlobalSearch();
    const rhsOpen = useSelector(getIsRhsOpen);
    const store = useStore<GlobalState>();
    const dispatch = useDispatch();
    const settings = useSettings();
    const teamId = useSelector(getCurrentTeamId);

    // The sidebar's categories, including the direct messages shown in the dock.
    useEffect(() => {
        if (teamId) {
            dispatch(fetchMyCategories(teamId));
        }
    }, [teamId, dispatch]);

    // Ctrl/⌘+/ lists the keyboard shortcuts, Ctrl/⌘+Shift+A opens the settings, as in the classic web app.
    useEffect(() => {
        const onKeyDown = (e: KeyboardEvent) => {
            if (!cmdOrCtrlPressed(e, true)) {
                return;
            }
            if (isKeyPressed(e, Constants.KeyCodes.FORWARD_SLASH)) {
                e.preventDefault();
                dispatch(openKeyboardShortcuts());
            } else if (e.shiftKey && isKeyPressed(e, Constants.KeyCodes.A)) {
                e.preventDefault();
                settings.open('account');
            }
        };
        window.addEventListener('keydown', onKeyDown);
        return () => window.removeEventListener('keydown', onKeyDown);
    }, [dispatch, settings]);

    // The page frame of the Fusion UI takes the whole window.
    useEffect(() => {
        const root = document.getElementById('root');
        root?.classList.add('am-root');
        return () => root?.classList.remove('am-root');
    }, []);

    // Fusion System follows the operating system's light or dark mode, even when it changes.
    useEffect(() => {
        const media = window.matchMedia('(prefers-color-scheme: light)');
        const onChange = () => applyTheme(getTheme(store.getState()));
        media.addEventListener('change', onChange);
        return () => media.removeEventListener('change', onChange);
    }, [store]);

    // Opening a panel on a narrow screen slides the right-hand drawer in.
    useEffect(() => {
        if (rhsOpen) {
            layout.setRightOpen(true);
        }

        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [rhsOpen]);

    const closeDrawers = () => {
        layout.setNavOpen(false);
        layout.setRightOpen(false);
    };

    return (
        <>
            <IconSprite/>
            <div className={am('app', {'nav-open': layout.navOpen, 'right-open': layout.rightOpen})}>
                <ServerRail/>
                {layout.home ? <HomeSidebar/> : <TeamSidebar/>}
                <main className={am('main')}>{children}</main>
                <RightPanel/>
                {!rhsOpen && layout.showMembers && <MemberList/>}
                <AppRail/>
                <div
                    className={am('drawer-scrim')}
                    aria-hidden='true'
                    onClick={closeDrawers}
                />
            </div>
            <div
                id={LAYER_ID}
                className={am('layer')}
            />
            {search.isOpen && <GlobalSearch/>}
            <CallWindow/>
            <Toasts/>
        </>
    );
}

// FusionApp is the Fusion UI's frame around a team's views: team rail, sidebar, main column, right-hand panel or
// member list, and app rail.
export default function FusionApp({children}: Props) {
    return (
        <LayoutProvider>
            <GlobalSearchProvider>
                <SettingsProvider>
                    <Frame>{children}</Frame>
                </SettingsProvider>
            </GlobalSearchProvider>
        </LayoutProvider>
    );
}
