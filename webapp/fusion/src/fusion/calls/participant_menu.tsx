// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useState} from 'react';
import {useIntl} from 'react-intl';
import {useStore} from 'react-redux';

import {Popover} from 'fusion/components/layer';
import {MenuHeading, MenuItem, MenuSeparator} from 'fusion/components/menu';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import UserPopover from 'fusion/popovers/user_popover';
import {am} from 'fusion/utils/class_names';
import {copyToClipboard} from 'utils/utils';

import type {GlobalState} from 'types/store';

import {goToChannel, useCallActions, useChannelLabel} from './actions';
import {getCallsAPI} from './calls_api';
import type {VoiceParticipant} from './calls_api';
import {canSetVolume, getUserVolume, setUserVolume} from './extras';
import {useCanMakeHost, useCanModerateCall, useIsVoiceChannel} from './hooks';

type MenuProps = {
    participant: VoiceParticipant;
    channelId: string;
    point: {x: number; y: number};
    onProfile: () => void;
    onClose: () => void;
};

// ParticipantMenu is the mockup's user context menu for someone in a call: profile, message, call, copy their
// username, and for the call's host and moderators the host controls Calls offers, asking them to unmute, and a
// moderator mute, which they can't lift themselves. The host (and system admins) can also make them the host.
function ParticipantMenu({participant: p, channelId, point, onProfile, onClose}: MenuProps) {
    const {formatMessage} = useIntl();
    const store = useStore<GlobalState>();
    const actions = useCallActions();
    const user = useUser(p.userId);
    const name = useDisplayName(user);
    const where = useChannelLabel(channelId);
    const voice = useIsVoiceChannel(channelId);
    const moderate = useCanModerateCall(channelId) && !p.isMe;

    // The host passes it on to someone in the call (system admins can too); bots can't be hosts.
    const makeHost = useCanMakeHost(channelId) && !p.isMe && !p.isHost && !user?.is_bot;
    const moderation = Boolean(getCallsAPI()?.moderation);
    const [volume, setVolume] = useState(() => getUserVolume(p.userId));

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
            {!p.isMe && canSetVolume() && (
                <label className={am('vol-row')}>
                    <span>{formatMessage({id: 'fusion.calls.menu.volume', defaultMessage: 'Volume'})}</span>
                    <input
                        type='range'
                        min={0}
                        max={100}
                        step={5}
                        value={Math.round(volume * 100)}
                        onChange={(e) => {
                            const v = Number(e.target.value) / 100;
                            setVolume(v);
                            setUserVolume(p, v);
                        }}
                    />
                    <b>{`${Math.round(volume * 100)}%`}</b>
                </label>
            )}
            {moderate && (
                <>
                    <MenuSeparator/>
                    <MenuHeading>{formatMessage({id: 'fusion.calls.menu.in', defaultMessage: 'In {where}'}, {where})}</MenuHeading>
                    {p.muted ? (
                        moderation && !p.modMuted && (
                            <MenuItem
                                icon='mic'
                                label={formatMessage({id: 'fusion.calls.menu.askUnmute', defaultMessage: 'Ask to unmute'})}
                                onClick={run(() => actions.askToUnmute(p, channelId))}
                            />
                        )
                    ) : (
                        <MenuItem
                            icon='mic-off'
                            label={formatMessage({id: 'fusion.calls.menu.mute', defaultMessage: 'Mute'})}
                            onClick={run(() => actions.hostMute(p, channelId))}
                        />
                    )}
                    {moderation && (
                        <MenuItem
                            icon='mic-lock'
                            label={p.modMuted ? formatMessage({id: 'fusion.calls.menu.liftModMute', defaultMessage: 'Allow to unmute'}) : formatMessage({id: 'fusion.calls.menu.modMute', defaultMessage: 'Mute for this call'})}
                            sub={p.modMuted ? undefined : formatMessage({id: 'fusion.calls.menu.modMuteSub', defaultMessage: 'They stay muted until you allow it, even if they rejoin'})}
                            onClick={run(() => actions.modMute(p, channelId, !p.modMuted))}
                        />
                    )}
                    {makeHost && (
                        <MenuItem
                            icon='star'
                            label={formatMessage({id: 'fusion.calls.menu.makeHost', defaultMessage: 'Make host'})}
                            sub={formatMessage({id: 'fusion.calls.menu.makeHostSub', defaultMessage: 'The host controls go to them'})}
                            onClick={run(() => actions.makeHost(p, channelId))}
                        />
                    )}
                    {p.raisedHand && (
                        <MenuItem
                            icon='hand'
                            label={formatMessage({id: 'fusion.calls.menu.lowerHand', defaultMessage: 'Lower hand'})}
                            onClick={run(() => actions.hostLowerHand(p, channelId))}
                        />
                    )}
                    {p.screenSharing && (
                        <MenuItem
                            icon='screen'
                            label={formatMessage({id: 'fusion.calls.menu.stopScreen', defaultMessage: 'Stop screen share'})}
                            onClick={run(() => actions.hostStopScreen(p, channelId))}
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
