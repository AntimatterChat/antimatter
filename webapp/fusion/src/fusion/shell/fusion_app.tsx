// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useRef} from 'react';
import {useDispatch, useSelector, useStore} from 'react-redux';

import {fetchMyCategories} from 'mattermost-redux/actions/channel_categories';
import {getCurrentChannelId} from 'mattermost-redux/selectors/entities/channels';
import {getTheme} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentTeamId} from 'mattermost-redux/selectors/entities/teams';

import {getIsRhsOpen} from 'selectors/rhs';

import AppRail from 'fusion/apps/app_rail';
import {IconSprite} from 'fusion/components/icon';
import {LAYER_ID} from 'fusion/components/layer';
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
import {LayoutProvider, isPhoneLayout, useLayout} from './layout_context';
import {SettingsProvider, useSettings} from './settings_context';
import {useSwipes} from './swipes';
import {ToastProvider} from './toast_context';

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
    const channelId = useSelector(getCurrentChannelId);
    const appRef = useRef<HTMLDivElement>(null);

    useSwipes(appRef, layout);

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

    // Opening a panel on a narrow screen slides the right-hand drawer in; closing it keeps the drawer open only
    // when it shows the member list.
    useEffect(() => {
        if (rhsOpen) {
            layout.setRightOpen(true);
        } else if (layout.rightOpen) {
            layout.setRightOpen(layout.showMembers && isPhoneLayout());
        }

        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [rhsOpen]);

    // Switching conversations closes the right-hand drawer, unless a panel (a thread, search results…) is open.
    useEffect(() => {
        if (!rhsOpen) {
            layout.setRightOpen(false);
        }

        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [channelId]);

    const closeDrawers = () => {
        layout.setNavOpen(false);
        layout.setRightOpen(false);
    };

    return (
        <>
            <IconSprite/>
            <div
                ref={appRef}
                className={am('app', {'nav-open': layout.navOpen, 'right-open': layout.rightOpen})}
            >
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
        </>
    );
}

// FusionApp is the Fusion UI's frame around a team's views: team rail, sidebar, main column, right-hand panel or
// member list, and app rail.
export default function FusionApp({children}: Props) {
    return (
        <LayoutProvider>
            <ToastProvider>
                <GlobalSearchProvider>
                    <SettingsProvider>
                        <Frame>{children}</Frame>
                    </SettingsProvider>
                </GlobalSearchProvider>
            </ToastProvider>
        </LayoutProvider>
    );
}
