// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useMemo, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';
import {Link} from 'react-router-dom';

import {getChannel, getCurrentChannelId, getMyChannelMembership, makeGetChannelUnreadCount} from 'mattermost-redux/selectors/entities/channels';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {isChannelMuted} from 'mattermost-redux/utils/channel_utils';

import {CallLiveMarker} from 'fusion/calls/markers';
import Icon from 'fusion/components/icon';
import ChannelMenu from 'fusion/popovers/channel_menu';
import {useLayout} from 'fusion/shell/layout_context';
import {am} from 'fusion/utils/class_names';
import {channelPath} from 'fusion/utils/paths';
import Pluggable from 'plugins/pluggable';

import type {GlobalState} from 'types/store';

import ChannelIcon from './channel_icon';

type Props = {
    channelId: string;
    collapsed?: boolean;
};

// ChannelRow is a channel in the sidebar, with its unread state, mention badge and menu.
export default function ChannelRow({channelId, collapsed}: Props) {
    const {formatMessage} = useIntl();
    const layout = useLayout();
    const getUnreadCount = useMemo(() => makeGetChannelUnreadCount(), []);
    const channel = useSelector((state: GlobalState) => getChannel(state, channelId));
    const team = useSelector(getCurrentTeam);
    const active = useSelector(getCurrentChannelId) === channelId;
    const unread = useSelector((state: GlobalState) => getUnreadCount(state, channelId));
    const muted = useSelector((state: GlobalState) => isChannelMuted(getMyChannelMembership(state, channelId)));
    const menuButton = useRef<HTMLButtonElement>(null);
    const [menu, setMenu] = useState<{x: number; y: number} | 'button' | null>(null);

    // Plugins can show something below the channel, e.g. the people in its call
    const hasFooters = useSelector((state: GlobalState) => Boolean(state.plugins.components.SidebarChannelFooter?.length));

    if (!channel || !team || (collapsed && !active && !unread.showUnread)) {
        return null;
    }

    return (
        <>
            <div
                className={am('ch-row', 'one-act')}
                onContextMenu={(e) => {
                    e.preventDefault();
                    setMenu({x: e.clientX, y: e.clientY});
                }}
            >
                <Link
                    to={channelPath(team.name, channel)}
                    className={am('ch', 'ch-text', {active, unread: unread.showUnread && !muted, muted})}
                    aria-current={active ? 'page' : undefined}
                    onClick={() => layout.setNavOpen(false)}
                >
                    <ChannelIcon channel={channel}/>
                    <span className={am('name')}>{channel.display_name}</span>
                    <CallLiveMarker channelId={channel.id}/>
                    <Pluggable
                        pluggableName='SidebarChannelLinkLabel'
                        channel={channel}
                    />
                    {muted && (
                        <span
                            className={am('mute-ic')}
                            title={formatMessage({id: 'fusion.sidebar.muted', defaultMessage: 'Muted'})}
                        >
                            <Icon
                                name='bell-off'
                                size='xs'
                            />
                        </span>
                    )}
                    {unread.mentions > 0 && <span className={am('badge')}>{unread.mentions}</span>}
                </Link>
                <div className={am('ch-actions', {open: menu !== null})}>
                    <button
                        ref={menuButton}
                        className={am('ch-act')}
                        title={formatMessage({id: 'fusion.sidebar.channelOptions', defaultMessage: 'Channel options'})}
                        aria-label={formatMessage({id: 'fusion.sidebar.channelOptionsFor', defaultMessage: 'Channel options for {name}'}, {name: channel.display_name})}
                        aria-haspopup='menu'
                        aria-expanded={menu !== null}
                        onClick={() => setMenu(menu ? null : 'button')}
                    >
                        <Icon name='dots'/>
                    </button>
                </div>
            </div>
            {hasFooters && !collapsed && (
                <div className={am('ch-footer')}>
                    <Pluggable
                        pluggableName='SidebarChannelFooter'
                        channel={channel}
                    />
                </div>
            )}
            {menu && (
                <ChannelMenu
                    channel={channel}
                    anchor={menu === 'button' ? menuButton.current : null}
                    point={menu === 'button' ? undefined : menu}
                    onClose={() => setMenu(null)}
                />
            )}
        </>
    );
}
