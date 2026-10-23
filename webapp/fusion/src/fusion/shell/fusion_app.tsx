// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useRef} from 'react';
import {useDispatch, useSelector, useStore} from 'react-redux';
import {useRouteMatch} from 'react-router-dom';

import {fetchMyCategories} from 'mattermost-redux/actions/channel_categories';
import {fetchChannelOverrides} from 'mattermost-redux/actions/channels';
import {fetchCommunityProfiles, fetchCommunityRules, fetchTeamTimeouts} from 'mattermost-redux/actions/teams';
import {getCurrentChannelId} from 'mattermost-redux/selectors/entities/channels';
import {getTheme} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentTeamId} from 'mattermost-redux/selectors/entities/teams';

import {closeRightHandSide} from 'actions/views/rhs';
import {getIsRhsOpen} from 'selectors/rhs';

import UnreadsStatusHandler from 'components/unreads_status_handler';

import AppRail from 'fusion/apps/app_rail';
import CallEvents from 'fusion/calls/call_events';
import CallPopout from 'fusion/calls/call_popout';
import CallPrompts from 'fusion/calls/call_prompts';
import CallWindow from 'fusion/calls/call_window';
import {ScreenSourcePicker} from 'fusion/calls/screen_sources';
import {useOpenVoiceChat} from 'fusion/calls/voice_chat';
import {IconSprite} from 'fusion/components/icon';
import {LAYER_ID} from 'fusion/components/layer';
import Toasts from 'fusion/components/toast';
import {useHideUnreads} from 'fusion/focus/focus';
import FocusSummaryHost from 'fusion/focus/focus_summary';
import {isPhone} from 'fusion/hooks/media';
import MemberList from 'fusion/members/member_list';
import ServerRail from 'fusion/rail/server_rail';
import RightPanel from 'fusion/rhs/right_panel';
import GlobalSearch from 'fusion/search/global_search';
import {useSyncedDrafts} from 'fusion/sidebar/drafts';
import HomeSidebar from 'fusion/sidebar/home_sidebar';
import TeamSidebar from 'fusion/sidebar/team_sidebar';
import {useMuteExpiry} from 'fusion/sidebar/timed_mute';
import {TeamCardHost} from 'fusion/teams/team_card';
import TeamSetupHost from 'fusion/teams/team_setup_host';
import {useLoadTeams} from 'fusion/teams/teams';
import {am} from 'fusion/utils/class_names';
import {openKeyboardShortcuts} from 'fusion/utils/modals';
import {SCHEDULED_POST_URL_SUFFIX} from 'utils/constants';
import {applyTheme} from 'utils/utils';

import type {GlobalState} from 'types/store';

import {DialogsProvider} from './dialogs_context';
import {useDisplayModes} from './display_modes';
import {useFaceTips} from './face_tips';
import {GlobalSearchProvider, useGlobalSearch} from './global_search_context';
import {isKeybind} from './keybinds';
import {LayoutProvider, isPhoneLayout, useLayout} from './layout_context';
import {useLongPress} from './long_press';
import {useNavShortcuts} from './nav_shortcuts';
import {SettingsProvider, useSettings} from './settings_context';
import SignInWarnings from './sign_in_warnings';
import {useSwipes} from './swipes';
import {ToastProvider} from './toast_context';
import {useVisualViewport} from './viewport';

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
    const voiceChat = useOpenVoiceChat();

    // Focus mode hides your unread marks, in the tab's title and icon and the desktop app's badge too.
    const hideUnreads = useHideUnreads();

    // The home's pages (drafts, scheduled messages, recaps, threads) belong to no channel: no member list beside them.
    const onPage = Boolean(useRouteMatch({path: `/:team/(drafts|${SCHEDULED_POST_URL_SUFFIX}|recaps|threads)`}));
    const channelId = useSelector(getCurrentChannelId);
    const appRef = useRef<HTMLDivElement>(null);

    useSwipes(appRef, layout);
    useLongPress();
    useVisualViewport();
    useFaceTips();
    useMuteExpiry();
    useNavShortcuts();
    useLoadTeams();
    useDisplayModes();

    // The drafts kept on the server, for the pencils of the sidebar and the composers.
    useSyncedDrafts();

    // The channel's overrides (Antimatter), which permission checks apply: loaded when it's first opened, then kept
    // up to date by the server's events.
    useEffect(() => {
        const state = store.getState();
        const channel = channelId ? state.entities.channels.channels[channelId] : undefined;
        if (channel?.team_id && !state.entities.channels.overrides?.[channelId]) {
            dispatch(fetchChannelOverrides(channelId));
        }
    }, [channelId, dispatch, store]);

    // The sidebar's categories, including the direct messages shown in the dock; who's timed out in the team
    // (Antimatter), which permission checks apply; its rules, which one may have to agree to before posting; and its
    // members' profiles there (nicknames, pictures, who isn't shown by name), all at once.
    useEffect(() => {
        if (teamId) {
            dispatch(fetchMyCategories(teamId));
            dispatch(fetchTeamTimeouts(teamId));
            dispatch(fetchCommunityRules(teamId));
            dispatch(fetchCommunityProfiles(teamId));
        }
    }, [teamId, dispatch]);

    // Ctrl/⌘+/ lists the keyboard shortcuts, Ctrl/⌘+Shift+A opens the settings, as in the classic web app.
    useEffect(() => {
        const onKeyDown = (e: KeyboardEvent) => {
            if (isKeybind(store.getState(), e, 'shortcuts')) {
                e.preventDefault();
                dispatch(openKeyboardShortcuts());
            } else if (isKeybind(store.getState(), e, 'settings')) {
                e.preventDefault();
                settings.open('account');
            }
        };
        window.addEventListener('keydown', onKeyDown);
        return () => window.removeEventListener('keydown', onKeyDown);
    }, [dispatch, settings, store]);

    // Escape closes the right-hand panel, last in the mockup's chain: popovers, dialogs, the edit form and the
    // autocomplete take it first (they stop it or mark it handled).
    useEffect(() => {
        const onKeyDown = (e: KeyboardEvent) => {
            if (e.key !== 'Escape' || e.defaultPrevented || !getIsRhsOpen(store.getState())) {
                return;
            }
            const layer = Array.from(document.getElementById(LAYER_ID)?.children || []).filter((el) => !el.classList.contains('am-toast') && !el.classList.contains('am-tip'));
            if (layer.length || document.querySelector('.modal.show, [role="dialog"][aria-modal="true"]')) {
                return;
            }
            dispatch(closeRightHandSide());
        };
        window.addEventListener('keydown', onKeyDown);
        return () => window.removeEventListener('keydown', onKeyDown);
    }, [dispatch, store]);

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
    // when it shows the member list. On a phone, where the panel took the whole screen, its back button goes back to
    // the conversation.
    useEffect(() => {
        if (rhsOpen) {
            layout.setRightOpen(true);
        } else if (layout.rightOpen) {
            layout.setRightOpen(layout.showMembers && isPhoneLayout() && !isPhone());
        }

        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [rhsOpen]);

    // Closing the right-hand drawer (a swipe, a tap beside it) closes the panel in it: left open out of sight, it
    // would keep the next thread opened from sliding the drawer in.
    const wasRightOpen = useRef(layout.rightOpen);
    useEffect(() => {
        if (wasRightOpen.current && !layout.rightOpen && isPhoneLayout() && getIsRhsOpen(store.getState())) {
            dispatch(closeRightHandSide());
        }
        wasRightOpen.current = layout.rightOpen;
    }, [layout.rightOpen, dispatch, store]);

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
                {!rhsOpen && !voiceChat && layout.showMembers && !onPage && <MemberList/>}
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
            <CallPopout/>
            <CallPrompts/>
            <ScreenSourcePicker/>
            <CallEvents/>
            <Toasts/>
            <TeamCardHost/>
            <TeamSetupHost/>

            {/* The tab's title and icon tell about unread messages and mentions, as in the classic web app. */}
            <UnreadsStatusHandler hideUnreads={hideUnreads}/>

            {/* What you missed, when a focus of yours ends. */}
            <FocusSummaryHost/>

            {/* Someone signed in to your account: disconnect them, or it was you. */}
            <SignInWarnings/>
        </>
    );
}

// FusionApp is the Fusion UI's frame around a team's views: team rail, sidebar, main column, right-hand panel or
// member list, and app rail.
export default function FusionApp({children}: Props) {
    return (
        <LayoutProvider>
            <ToastProvider>
                <DialogsProvider>
                    <GlobalSearchProvider>
                        <SettingsProvider>
                            <Frame>{children}</Frame>
                        </SettingsProvider>
                    </GlobalSearchProvider>
                </DialogsProvider>
            </ToastProvider>
        </LayoutProvider>
    );
}

// PopoutFrame is the Fusion UI in a window of its own (a conversation or a thread opened in a new window): the
// conversation and its right-hand panel, without the rails and the sidebar.
function PopoutFrame({children, panel}: Props & {panel: boolean}) {
    const store = useStore<GlobalState>();
    useFaceTips();
    useDisplayModes();

    useEffect(() => {
        const root = document.getElementById('root');
        root?.classList.add('am-root');
        return () => root?.classList.remove('am-root');
    }, []);

    useEffect(() => {
        const media = window.matchMedia('(prefers-color-scheme: light)');
        const onChange = () => applyTheme(getTheme(store.getState()));
        media.addEventListener('change', onChange);
        return () => media.removeEventListener('change', onChange);
    }, [store]);

    return (
        <>
            <IconSprite/>
            <div className={am('app', 'popout')}>
                <main className={am('main')}>{children}</main>
                {panel && <RightPanel/>}
            </div>
            <div
                id={LAYER_ID}
                className={am('layer')}
            />
            <Toasts/>
            <TeamCardHost/>
        </>
    );
}

// FusionPopout is the Fusion UI's frame in a popout window; panel shows the right-hand panel beside its content.
export function FusionPopout({children, panel = true}: Props & {panel?: boolean}) {
    return (
        <LayoutProvider>
            <ToastProvider>
                <DialogsProvider>
                    <GlobalSearchProvider>
                        <SettingsProvider>
                            <PopoutFrame panel={panel}>{children}</PopoutFrame>
                        </SettingsProvider>
                    </GlobalSearchProvider>
                </DialogsProvider>
            </ToastProvider>
        </LayoutProvider>
    );
}
