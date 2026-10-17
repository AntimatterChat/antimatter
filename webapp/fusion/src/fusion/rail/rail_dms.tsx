// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useMemo} from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import {getCurrentChannelId} from 'mattermost-redux/selectors/entities/channels';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';

import Icon from 'fusion/components/icon';
import {useReducedMotion} from 'fusion/hooks/media';
import {getUnreadDirectChannels} from 'fusion/selectors';
import {isPhoneLayout, useLayout} from 'fusion/shell/layout_context';
import {DirectFace} from 'fusion/sidebar/direct_row';
import {makeGetDockChannelIds} from 'fusion/sidebar/dock_channels';
import {DM_FIND_ID} from 'fusion/sidebar/home_sidebar';
import {am} from 'fusion/utils/class_names';
import {channelPath} from 'fusion/utils/paths';
import {getHistory} from 'utils/browser_history';

import type {GlobalState} from 'types/store';

// How many unread conversations the rail shows at most.
const MAX_BUBBLES = 5;

// RailDirectMessages sits at the bottom of the team rail, above search: the direct messages waiting for you, as round
// faces with their counts, under a small chat bubble that opens the list of all the direct messages. Without any, the
// chat bubble is a rail bubble of its own; it shrinks to the small icon, or grows back, with an animation (none with
// reduced motion). The faces are the unread conversations the sidebar's Open conversations dock has no room for (it
// keeps the latest active, as many as fit, up to the "Open conversations" setting): those in the dock show there
// already, with their red ring. None while the list of all the direct messages takes the dock's place: they're all in
// it.
export default function RailDirectMessages() {
    const {formatMessage} = useIntl();
    const layout = useLayout();
    const still = useReducedMotion();
    const unreadDMs = useSelector(getUnreadDirectChannels);
    const currentChannelId = useSelector(getCurrentChannelId);
    const currentTeam = useSelector(getCurrentTeam);
    const getDockChannelIds = useMemo(() => makeGetDockChannelIds(), []);
    const dockIds = useSelector((state: GlobalState) => getDockChannelIds(state, layout.dockRows));

    const inDock = useMemo(() => new Set(dockIds), [dockIds]);
    const waiting = layout.home ? [] : unreadDMs.filter((dm) => dm.channel.id !== currentChannelId && !inDock.has(dm.channel.id)).slice(0, MAX_BUBBLES);
    const stacked = waiting.length > 0;

    const unread = unreadDMs.reduce((n, dm) => n + Math.max(dm.messages, dm.mentions), 0);
    const label = formatMessage({id: 'fusion.dock.all', defaultMessage: 'All direct messages'});

    return (
        <div
            className={am('dm-dock', {stacked, still})}
            role={stacked ? 'group' : undefined}
            aria-label={stacked ? formatMessage({id: 'fusion.rail.unreadDMs', defaultMessage: 'Unread direct messages'}) : undefined}
        >
            <button
                className={am('srv', 'dm-home', {mini: stacked, active: layout.home})}
                data-am-tip={label}
                data-am-tip-side='right'
                aria-label={unread ? formatMessage({id: 'fusion.dock.allUnread', defaultMessage: '{label}, {count, plural, one {# unread message} other {# unread messages}}'}, {label, count: unread}) : label}
                aria-current={layout.home ? 'page' : undefined}
                onClick={() => {
                    layout.setHome(true);
                    layout.setNavOpen(true);

                    // Ready to find a conversation; not on a phone, where the keyboard would cover the list.
                    if (!isPhoneLayout()) {
                        requestAnimationFrame(() => document.getElementById(DM_FIND_ID)?.focus());
                    }
                }}
            >
                <Icon name='chat'/>
            </button>
            {waiting.map(({channel, mentions, messages}) => (
                <button
                    key={channel.id}
                    className={am('srv', 'dm-srv')}
                    data-am-tip={channel.display_name}
                    data-am-tip-side='right'
                    aria-label={formatMessage({id: 'fusion.rail.unreadDM', defaultMessage: '{name}, {count, plural, one {# unread message} other {# unread messages}}'}, {name: channel.display_name, count: mentions || messages})}
                    onClick={() => {
                        // The conversation opens beside the team's sidebar, as in the mockup.
                        layout.setNavOpen(false);
                        if (currentTeam) {
                            getHistory().push(channelPath(currentTeam.name, channel));
                        }
                    }}
                >
                    <DirectFace channel={channel}/>
                    <span className={am('badge')}>{mentions || messages}</span>
                </button>
            ))}
        </div>
    );
}
