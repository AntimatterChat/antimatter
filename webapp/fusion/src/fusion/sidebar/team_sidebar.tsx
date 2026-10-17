// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useDispatch, useSelector, useStore} from 'react-redux';

import {getCurrentTeam, getCurrentTeamId} from 'mattermost-redux/selectors/entities/teams';

import {setUnreadFilterEnabled} from 'actions/views/channel_sidebar';
import {isUnreadFilterEnabled} from 'selectors/views/channel_sidebar';

import Icon from 'fusion/components/icon';
import ServerMenu from 'fusion/popovers/server_menu';
import {isKeybind} from 'fusion/shell/keybinds';
import {a11yRegion} from 'fusion/utils/a11y_regions';
import {am} from 'fusion/utils/class_names';
import Pluggable from 'plugins/pluggable';

import type {GlobalState} from 'types/store';

import Category from './category';
import ChannelRow from './channel_row';
import {useSidebarThreads} from './channel_threads';
import Dock from './dock';
import {getHiddenChannelIds} from './hidden';
import HiddenSection from './hidden_section';
import MoreUnreads from './more_unreads';
import {getSidebarCategories} from './server_categories';
import UnreadCategory from './unread_category';
import VoicePanel from './voice_panel';

// TeamSidebar lists the current team's channels by category.
export default function TeamSidebar() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const scrollRef = useRef<HTMLDivElement>(null);
    const sidebarRef = useRef<HTMLElement>(null);

    // The unread filter (Ctrl/Cmd+Shift+U, or the team menu) lists only the unread channels, as in the classic sidebar.
    const unreadOnly = useSelector(isUnreadFilterEnabled);
    useSidebarThreads();
    const store = useStore<GlobalState>();
    useEffect(() => {
        const onKeyDown = (e: KeyboardEvent) => {
            if (isKeybind(store.getState(), e, 'unreadFilter')) {
                e.preventDefault();
                dispatch(setUnreadFilterEnabled(!unreadOnly));
            }
        };
        document.addEventListener('keydown', onKeyDown);
        return () => document.removeEventListener('keydown', onKeyDown);
    }, [dispatch, unreadOnly, store]);
    const team = useSelector(getCurrentTeam);
    const categories = useSelector(getSidebarCategories);

    // The team's channels hidden until they get new messages, listed folded at the end.
    const hiddenChannels = useSelector((state: GlobalState) => {
        const teamId = getCurrentTeamId(state);
        return [...getHiddenChannelIds(state)].filter((id) => {
            const channel = state.entities.channels.channels[id];
            return channel?.team_id === teamId && channel.type !== 'D' && channel.type !== 'G';
        });
    }, shallowEqual);
    const headButton = useRef<HTMLButtonElement>(null);
    const [menuOpen, setMenuOpen] = useState(false);

    if (!team) {
        return <aside className={am('sidebar')}/>;
    }

    return (
        <aside
            ref={sidebarRef}
            {...a11yRegion(am('sidebar'), 'sidebar')}
            aria-label={formatMessage({id: 'fusion.sidebar.label', defaultMessage: 'Channels and direct messages'})}
        >
            <div className={am('plugin-side-head')}>
                <Pluggable pluggableName='LeftSidebarHeader'/>
            </div>
            <div className={am('side-head', 'menu-head')}>
                <button
                    ref={headButton}
                    className={am('srv-head-btn')}
                    aria-haspopup='menu'
                    aria-expanded={menuOpen}
                    aria-label={formatMessage({id: 'fusion.sidebar.teamMenu', defaultMessage: '{team} menu'}, {team: team.display_name})}
                    onClick={() => setMenuOpen(!menuOpen)}
                >
                    <h2>{team.display_name}</h2>
                    <Icon
                        name={menuOpen ? 'x' : 'chev'}
                        size='sm'
                    />
                </button>
            </div>
            {unreadOnly && (
                <div className={am('filter-bar')}>
                    <span>{formatMessage({id: 'fusion.sidebar.unreadOnly', defaultMessage: 'Unread channels only'})}</span>
                    <button
                        type='button'
                        onClick={() => dispatch(setUnreadFilterEnabled(false))}
                    >
                        {formatMessage({id: 'fusion.sidebar.showAll', defaultMessage: 'Show all'})}
                    </button>
                </div>
            )}
            <div
                ref={scrollRef}
                className={am('chan-scroll')}
            >
                <MoreUnreads scrollRef={scrollRef}/>
                <UnreadCategory filter={unreadOnly}/>
                {!unreadOnly && categories.filter((c) => c.type !== 'direct_messages').map((category) => (
                    <Category
                        key={category.id}
                        category={category}
                    />
                ))}
                {!unreadOnly && (
                    <HiddenSection
                        items={hiddenChannels}
                        render={(id) => (
                            <ChannelRow
                                key={id}
                                channelId={id}
                            />
                        )}
                    />
                )}
            </div>
            <Dock
                sidebarRef={sidebarRef}
                scrollRef={scrollRef}
            />
            <VoicePanel/>
            {menuOpen && (
                <ServerMenu
                    anchor={headButton.current}
                    width={Math.min((headButton.current?.offsetWidth || 252) - 16, 280)}
                    onClose={() => setMenuOpen(false)}
                />
            )}
        </aside>
    );
}
