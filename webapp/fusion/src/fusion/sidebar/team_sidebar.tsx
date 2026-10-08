// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useMemo, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useDispatch, useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';

import {makeGetChannelIdsForCategory} from 'mattermost-redux/selectors/entities/channel_categories';
import {makeGetChannel} from 'mattermost-redux/selectors/entities/channels';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';

import {setUnreadFilterEnabled} from 'actions/views/channel_sidebar';
import {getCategoriesForCurrentTeam, isUnreadFilterEnabled} from 'selectors/views/channel_sidebar';

import Icon from 'fusion/components/icon';
import ServerMenu from 'fusion/popovers/server_menu';
import {getUnreadDirectChannels} from 'fusion/selectors';
import {useLayout} from 'fusion/shell/layout_context';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import Category from './category';
import DirectRow from './direct_row';
import {DM_FIND_ID} from './home_sidebar';
import MoreUnreads from './more_unreads';
import UnreadCategory from './unread_category';
import VoicePanel from './voice_panel';

// The direct messages category is shown as a dock of open conversations at the bottom of the sidebar. Its chat bubble
// opens all the conversations, and counts the messages waiting in them.
function Dock() {
    const {formatMessage} = useIntl();
    const layout = useLayout();
    const categories = useSelector(getCategoriesForCurrentTeam);
    const dmCategory = categories.find((c) => c.type === 'direct_messages');

    // All the open conversations, unread or not: the Unreads section above the categories leaves them here.
    const getChannelIds = useMemo(() => makeGetChannelIdsForCategory(), []);
    const channelIds = useSelector((state: GlobalState) => (dmCategory ? getChannelIds(state, dmCategory) : []));
    const getChannel = useMemo(() => makeGetChannel(), []);
    const channels = useSelector((state: GlobalState) => channelIds.map((id) => getChannel(state, id)).filter(Boolean) as Channel[], shallowEqual);
    const unread = useSelector(getUnreadDirectChannels).reduce((n, dm) => n + Math.max(dm.messages, dm.mentions), 0);
    const allLabel = formatMessage({id: 'fusion.dock.all', defaultMessage: 'All direct messages'});

    return (
        <div
            className={am('dock')}
            aria-label={formatMessage({id: 'fusion.dock.label', defaultMessage: 'Open conversations'})}
        >
            <div className={am('dock-head')}>
                <span className={am('grow')}>{formatMessage({id: 'fusion.dock.title', defaultMessage: 'Open conversations'})}</span>
                <button
                    className={am('dock-all')}
                    title={allLabel}
                    aria-label={unread ? formatMessage({id: 'fusion.dock.allUnread', defaultMessage: '{label}, {count, plural, one {# unread message} other {# unread messages}}'}, {label: allLabel, count: unread}) : allLabel}
                    onClick={() => {
                        // Ready to find a conversation, as the team rail's home icon used to be.
                        layout.setHome(true);
                        requestAnimationFrame(() => document.getElementById(DM_FIND_ID)?.focus());
                    }}
                >
                    <Icon
                        name='chat'
                        size='sm'
                    />
                    {unread > 0 && <span className={am('badge')}>{unread > 99 ? '99+' : unread}</span>}
                </button>
            </div>
            {channels.length > 0 && (
                <div className={am('dock-list')}>
                    {channels.map((channel) => (
                        <DirectRow
                            key={channel.id}
                            channel={channel}
                            dock={true}
                        />
                    ))}
                </div>
            )}
        </div>
    );
}

// TeamSidebar lists the current team's channels by category.
export default function TeamSidebar() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const scrollRef = useRef<HTMLDivElement>(null);

    // The unread filter (Ctrl/Cmd+Shift+U, or the team menu) lists only the unread channels, as in the classic sidebar.
    const unreadOnly = useSelector(isUnreadFilterEnabled);
    useEffect(() => {
        const onKeyDown = (e: KeyboardEvent) => {
            if ((e.ctrlKey || e.metaKey) && e.shiftKey && !e.altKey && e.key.toLowerCase() === 'u') {
                e.preventDefault();
                dispatch(setUnreadFilterEnabled(!unreadOnly));
            }
        };
        document.addEventListener('keydown', onKeyDown);
        return () => document.removeEventListener('keydown', onKeyDown);
    }, [dispatch, unreadOnly]);
    const team = useSelector(getCurrentTeam);
    const categories = useSelector(getCategoriesForCurrentTeam);
    const headButton = useRef<HTMLButtonElement>(null);
    const [menuOpen, setMenuOpen] = useState(false);

    if (!team) {
        return <aside className={am('sidebar')}/>;
    }

    return (
        <aside
            className={am('sidebar')}
            aria-label={formatMessage({id: 'fusion.sidebar.label', defaultMessage: 'Channels and direct messages'})}
        >
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
            </div>
            <Dock/>
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
