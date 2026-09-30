// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useState} from 'react';
import {useIntl} from 'react-intl';
import {useSelector, useStore} from 'react-redux';

import {getCurrentUser} from 'mattermost-redux/selectors/entities/users';
import {isSystemAdmin} from 'mattermost-redux/utils/user_utils';

import {Popover} from 'fusion/components/layer';
import {MenuHeading, MenuItem, MenuSeparator} from 'fusion/components/menu';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import UserPopover from 'fusion/popovers/user_popover';
import {copyToClipboard} from 'utils/utils';

import type {GlobalState} from 'types/store';

import {goToChannel, useCallActions, useChannelLabel} from './actions';
import type {VoiceParticipant} from './calls_api';
import {useIsVoiceChannel, useMyCall} from './hooks';

type MenuProps = {
    participant: VoiceParticipant;
    channelId: string;
    point: {x: number; y: number};
    onProfile: () => void;
    onClose: () => void;
};

// ParticipantMenu is the mockup's user context menu for someone in a call: profile, message, call, copy their
// username, and for the call's host (or an admin) the host controls Calls offers.
function ParticipantMenu({participant: p, channelId, point, onProfile, onClose}: MenuProps) {
    const {formatMessage} = useIntl();
    const store = useStore<GlobalState>();
    const actions = useCallActions();
    const user = useUser(p.userId);
    const name = useDisplayName(user);
    const where = useChannelLabel(channelId);
    const voice = useIsVoiceChannel(channelId);
    const my = useMyCall();
    const admin = useSelector((state: GlobalState) => isSystemAdmin(getCurrentUser(state)?.roles || ''));
    const host = my?.call.channelId === channelId && my.isHost;
    const moderate = !p.isMe && (admin || host);

    const run = (action: () => void) => () => {
        onClose();
        action();
    };

    return (
        <Popover
            point={point}
            placement='point'
            className='plus-pop ch-menu user-menu'
            role='menu'
            label={name}
            onClose={onClose}
        >
            <MenuItem
                icon='users'
                label={formatMessage({id: 'fusion.calls.menu.profile', defaultMessage: 'Profile'})}
                onClick={onProfile}
            />
            {!p.isMe && (
                <>
                    <MenuItem
                        icon='chat'
                        label={formatMessage({id: 'fusion.calls.menu.message', defaultMessage: 'Message'})}
                        onClick={run(async () => {
                            const channel = await actions.openDirect(p.userId);
                            if (channel) {
                                goToChannel(store.getState(), channel.id);
                            }
                        })}
                    />
                    <MenuItem
                        icon='phone'
                        label={formatMessage({id: 'fusion.calls.menu.call', defaultMessage: 'Call'})}
                        onClick={run(() => actions.callUser(p.userId))}
                    />
                </>
            )}
            {user && (
                <MenuItem
                    icon='copy'
                    label={formatMessage({id: 'fusion.calls.menu.copyUsername', defaultMessage: 'Copy username'})}
                    onClick={run(() => copyToClipboard(user.username))}
                />
            )}
            {moderate && (
                <>
                    <MenuSeparator/>
                    <MenuHeading>{formatMessage({id: 'fusion.calls.menu.in', defaultMessage: 'In {where}'}, {where})}</MenuHeading>
                    <MenuItem
                        icon='mic-off'
                        label={formatMessage({id: 'fusion.calls.menu.mute', defaultMessage: 'Mute'})}
                        disabled={p.muted}
                        onClick={run(() => actions.hostMute(p, channelId))}
                    />
                    {p.raisedHand && (
                        <MenuItem
                            icon='hand'
                            label={formatMessage({id: 'fusion.calls.menu.lowerHand', defaultMessage: 'Lower hand'})}
                            onClick={run(() => actions.hostLowerHand(p))}
                        />
                    )}
                    {p.screenSharing && (
                        <MenuItem
                            icon='screen'
                            label={formatMessage({id: 'fusion.calls.menu.stopScreen', defaultMessage: 'Stop screen share'})}
                            onClick={run(() => actions.hostStopScreen(p))}
                        />
                    )}
                    <MenuItem
                        icon='hangup'
                        danger={true}
                        label={voice ? formatMessage({id: 'fusion.calls.menu.kick', defaultMessage: 'Kick from voice channel'}) : formatMessage({id: 'fusion.calls.menu.remove', defaultMessage: 'Remove from call'})}
                        onClick={run(() => actions.hostRemove(p, channelId))}
                    />
                </>
            )}
        </Popover>
    );
}

type OpenMenu = {participant: VoiceParticipant; point: {x: number; y: number}; anchor: HTMLElement};

// useParticipantMenu opens the participant menu on right-click of a participant's row or tile: spread onContextMenu
// on the element, and render element.
export function useParticipantMenu(channelId: string) {
    const [menu, setMenu] = useState<OpenMenu | null>(null);
    const [profile, setProfile] = useState<{userId: string; anchor: HTMLElement} | null>(null);

    const open = useCallback((participant: VoiceParticipant, e: React.MouseEvent<HTMLElement>) => {
        e.preventDefault();
        setProfile(null);
        setMenu({participant, point: {x: e.clientX, y: e.clientY}, anchor: e.currentTarget});
    }, []);
    const close = useCallback(() => setMenu(null), []);
    const closeProfile = useCallback(() => setProfile(null), []);

    const element = (
        <>
            {menu && (
                <ParticipantMenu
                    participant={menu.participant}
                    channelId={channelId}
                    point={menu.point}
                    onProfile={() => {
                        setProfile({userId: menu.participant.userId, anchor: menu.anchor});
                        setMenu(null);
                    }}
                    onClose={close}
                />
            )}
            {profile && (
                <UserPopover
                    userId={profile.userId}
                    anchor={profile.anchor}
                    onClose={closeProfile}
                />
            )}
        </>
    );
    return {open, element};
}
