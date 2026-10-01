// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useMemo, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';
import {Link} from 'react-router-dom';

import {getChannel, getCurrentChannelId, getMyChannelMembership, makeGetChannelUnreadCount} from 'mattermost-redux/selectors/entities/channels';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {isChannelMuted} from 'mattermost-redux/utils/channel_utils';

import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import ChannelMenu from 'fusion/popovers/channel_menu';
import {useLayout} from 'fusion/shell/layout_context';
import ChannelIcon from 'fusion/sidebar/channel_icon';
import {am} from 'fusion/utils/class_names';
import {channelPath} from 'fusion/utils/paths';
import Pluggable from 'plugins/pluggable';

import type {GlobalState} from 'types/store';

import {useCallActions} from './actions';
import {isSpeaking} from './calls_api';
import type {VoiceParticipant} from './calls_api';
import {useLocalCall, useParticipants} from './hooks';
import {useParticipantMenu} from './participant_menu';
import {useVoiceChatActions} from './voice_chat';

function VoicePerson({participant: p, onContextMenu}: {participant: VoiceParticipant; onContextMenu: (e: React.MouseEvent<HTMLElement>) => void}) {
    const {formatMessage} = useIntl();
    const name = useDisplayName(useUser(p.userId));

    // After LIVE and the camera, one audio icon: deafened beats muted.
    let audio = null;
    if (p.deafened) {
        audio = (
            <Icon
                name='headphones-off'
                size='xs'
            />
        );
    } else if (p.muted) {
        audio = (
            <Icon
                name='mic-off'
                size='xs'
            />
        );
    }

    return (
        <div
            className={am('vp')}
            data-voice-user={p.userId}
            onContextMenu={onContextMenu}
        >
            <Avatar
                userId={p.userId}
                size='sm'
                speaking={isSpeaking(p)}
            />
            <span className={am('name')}>{name}</span>
            {p.screenSharing && <span className={am('live')}>{formatMessage({id: 'fusion.calls.live', defaultMessage: 'LIVE'})}</span>}
            {p.video && (
                <Icon
                    name='video'
                    size='xs'
                />
            )}
            {p.raisedHand && (
                <span
                    className={am('hand')}
                    title={formatMessage({id: 'fusion.calls.handRaised', defaultMessage: 'Hand raised'})}
                >
                    <Icon
                        name='hand'
                        size='xs'
                    />
                </span>
            )}
            {audio}
        </div>
    );
}

// VoicePeople lists the people in a voice channel under its sidebar row.
function VoicePeople({channelId, participants}: {channelId: string; participants: VoiceParticipant[]}) {
    const menu = useParticipantMenu(channelId);
    return (
        <div className={am('voice-people')}>
            {participants.map((p) => (
                <VoicePerson
                    key={p.sessionId}
                    participant={p}
                    onContextMenu={(e) => menu.open(p, e)}
                />
            ))}
            {menu.element}
        </div>
    );
}

type Props = {
    channelId: string;
    collapsed?: boolean;
};

// VoiceChannelRow is a voice channel in the sidebar, the mockup's voice row: opening it joins the room (as the voice
// channels plugin is set to), its chat button opens its text chat without joining, and the people in it are listed
// underneath, even when its category is collapsed.
export default function VoiceChannelRow({channelId, collapsed}: Props) {
    const {formatMessage} = useIntl();
    const layout = useLayout();
    const actions = useCallActions();
    const chat = useVoiceChatActions();
    const getUnreadCount = useMemo(() => makeGetChannelUnreadCount(), []);
    const channel = useSelector((state: GlobalState) => getChannel(state, channelId));
    const team = useSelector(getCurrentTeam);
    const active = useSelector(getCurrentChannelId) === channelId;
    const unread = useSelector((state: GlobalState) => getUnreadCount(state, channelId));
    const muted = useSelector((state: GlobalState) => isChannelMuted(getMyChannelMembership(state, channelId)));
    const participants = useParticipants(channelId);
    const connected = useLocalCall()?.channelId === channelId;
    const menuButton = useRef<HTMLButtonElement>(null);
    const [menu, setMenu] = useState<{x: number; y: number} | 'button' | null>(null);
    const hasFooters = useSelector((state: GlobalState) => Boolean(state.plugins.components.SidebarChannelFooter?.length));

    const live = participants.length > 0;
    if (!channel || !team || (collapsed && !active && !live)) {
        return null;
    }

    return (
        <>
            <div
                className={am('ch-row', {connected})}
                onContextMenu={(e) => {
                    e.preventDefault();
                    setMenu({x: e.clientX, y: e.clientY});
                }}
            >
                <Link
                    to={channelPath(team.name, channel)}
                    className={am('ch', 'ch-voice', {active, unread: unread.showUnread && !muted, connected, live, muted})}
                    aria-current={active ? 'page' : undefined}
                    title={connected ? formatMessage({id: 'fusion.calls.youAreConnected', defaultMessage: 'You are connected'}) : undefined}
                    onClick={() => {
                        layout.setNavOpen(false);
                        actions.autoJoinVoice(channelId);
                    }}
                >
                    <ChannelIcon channel={channel}/>
                    <span className={am('name')}>{channel.display_name}</span>
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
                    {unread.mentions > 0 && !muted && <span className={am('badge')}>{unread.mentions}</span>}
                </Link>
                <div className={am('ch-actions', {open: menu !== null})}>
                    <button
                        className={am('ch-act')}
                        data-act='voice-chat-open'
                        title={formatMessage({id: 'fusion.calls.openTextChat', defaultMessage: 'Open text chat'})}
                        aria-label={formatMessage({id: 'fusion.calls.openTextChatFor', defaultMessage: 'Open text chat for {name}'}, {name: channel.display_name})}
                        onClick={() => {
                            layout.setNavOpen(false);
                            chat.open(channelId);
                        }}
                    >
                        <Icon name='chat'/>
                    </button>
                    <button
                        ref={menuButton}
                        className={am('ch-act')}
                        data-act='ch-menu'
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
            {live && (
                <VoicePeople
                    channelId={channelId}
                    participants={participants}
                />
            )}
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
