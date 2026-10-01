// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {removeChannelMember} from 'mattermost-redux/actions/channels';
import {removeUserFromTeam} from 'mattermost-redux/actions/teams';
import {Permissions} from 'mattermost-redux/constants';
import {getCurrentChannel} from 'mattermost-redux/selectors/entities/channels';
import {haveIChannelPermission, haveITeamPermission} from 'mattermost-redux/selectors/entities/roles';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import {openDirectChannelToUserId} from 'actions/channel_actions';

import {Dialog, Popover} from 'fusion/components/layer';
import {MenuItem, MenuSeparator} from 'fusion/components/menu';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';
import {channelPath} from 'fusion/utils/paths';
import {getHistory} from 'utils/browser_history';
import Constants from 'utils/constants';
import {copyToClipboard} from 'utils/utils';

import type {GlobalState} from 'types/store';

import UserPopover from './user_popover';

// The composer of the open conversation listens for this event to insert a mention.
export const MENTION_EVENT = 'am-mention';

export function insertMention(username: string) {
    window.dispatchEvent(new CustomEvent(MENTION_EVENT, {detail: `@${username} `}));
}

type Target = {userId: string; anchor: HTMLElement; point: {x: number; y: number}};

// ConfirmRemove asks before removing someone from the team: the mockup's moderation dialog, without the reason
// (Mattermost doesn't keep one).
function ConfirmRemove({userId, onClose}: {userId: string; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const user = useUser(userId);
    const name = useDisplayName(user);
    const team = useSelector(getCurrentTeam);
    const [error, setError] = useState('');

    const remove = async () => {
        if (!team) {
            return;
        }
        const result = await dispatch(removeUserFromTeam(team.id, userId));
        if (result && 'error' in result && result.error) {
            setError((result.error as {message?: string}).message || formatMessage({id: 'fusion.userMenu.removeFailed', defaultMessage: 'Could not remove {name}.'}, {name}));
            return;
        }
        toast(formatMessage({id: 'fusion.toast.removedFromTeam', defaultMessage: '{name} was removed from {team}'}, {name, team: team.display_name}));
        onClose();
    };

    return (
        <Dialog
            label={formatMessage({id: 'fusion.userMenu.removeTitle', defaultMessage: 'Remove {name} from {team}?'}, {name, team: team?.display_name})}
            onClose={onClose}
            onSubmit={remove}
        >
            <div className={am('pane')}>
                <h2>{formatMessage({id: 'fusion.userMenu.removeTitle', defaultMessage: 'Remove {name} from {team}?'}, {name, team: team?.display_name})}</h2>
                <p className={am('lead')}>{formatMessage({id: 'fusion.userMenu.removeLead', defaultMessage: '{name} leaves every channel of the team, but can rejoin with a new invite.'}, {name})}</p>
                {error && (
                    <p
                        className={am('lead')}
                        role='alert'
                        style={{color: 'var(--am-danger)'}}
                    >
                        {error}
                    </p>
                )}
                <div className={am('modal-actions')}>
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={onClose}
                    >
                        {formatMessage({id: 'fusion.userMenu.cancel', defaultMessage: 'Cancel'})}
                    </button>
                    <button className={am('btn', 'danger')}>
                        {formatMessage({id: 'fusion.userMenu.remove', defaultMessage: 'Remove'})}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}

// UserMenu is the right-click menu of a person (avatar, name, member row): the mockup's userMenu, with Mattermost's
// moderation (remove from the channel or the team) for those allowed to.
function UserMenu({target, onProfile, onRemoveFromTeam, onClose}: {target: Target; onProfile: () => void; onRemoveFromTeam: () => void; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const {userId} = target;
    const user = useUser(userId);
    const name = useDisplayName(user);
    const me = useSelector(getCurrentUserId);
    const team = useSelector(getCurrentTeam);
    const channel = useSelector(getCurrentChannel);
    const inChannel = useSelector((state: GlobalState) => Boolean(channel && state.entities.users.profilesInChannel[channel.id]?.has(userId)));
    const canRemoveFromChannel = useSelector((state: GlobalState) => {
        if (!channel || !team || (channel.type !== Constants.OPEN_CHANNEL && channel.type !== Constants.PRIVATE_CHANNEL) || channel.name === Constants.DEFAULT_CHANNEL || channel.delete_at) {
            return false;
        }
        const permission = channel.type === Constants.OPEN_CHANNEL ? Permissions.MANAGE_PUBLIC_CHANNEL_MEMBERS : Permissions.MANAGE_PRIVATE_CHANNEL_MEMBERS;
        return haveIChannelPermission(state, team.id, channel.id, permission);
    });
    const canRemoveFromTeam = useSelector((state: GlobalState) => Boolean(team) && haveITeamPermission(state, team!.id, Permissions.REMOVE_USER_FROM_TEAM));

    if (!user) {
        return null;
    }
    const isMe = userId === me;
    const run = (action: () => void) => () => {
        onClose();
        action();
    };

    const message = async () => {
        const result = await dispatch(openDirectChannelToUserId(userId));
        if ('data' in result && result.data && team) {
            getHistory().push(channelPath(team.name, result.data));
        }
    };
    const removeFromChannel = async () => {
        if (!channel) {
            return;
        }
        const result = await dispatch(removeChannelMember(channel.id, userId));
        if (!(result && 'error' in result && result.error)) {
            toast(formatMessage({id: 'fusion.toast.removedFromChannel', defaultMessage: '{name} was removed from {channel}'}, {name, channel: '#' + channel.display_name}));
        }
    };

    const moderation = !isMe && ((canRemoveFromChannel && inChannel) || canRemoveFromTeam);
    return (
        <Popover
            point={target.point}
            placement='point'
            className='plus-pop ch-menu user-menu'
            role='menu'
            label={name}
            onClose={onClose}
        >
            <MenuItem
                icon='users'
                label={formatMessage({id: 'fusion.userMenu.profile', defaultMessage: 'Profile'})}
                onClick={onProfile}
            />
            {!isMe && (
                <MenuItem
                    icon='chat'
                    label={formatMessage({id: 'fusion.userMenu.message', defaultMessage: 'Message'})}
                    onClick={run(message)}
                />
            )}
            <MenuItem
                icon='at'
                label={formatMessage({id: 'fusion.userMenu.mention', defaultMessage: 'Mention'})}
                onClick={run(() => insertMention(user.username))}
            />
            <MenuItem
                icon='copy'
                label={formatMessage({id: 'fusion.userMenu.copy', defaultMessage: 'Copy username'})}
                onClick={run(() => {
                    copyToClipboard('@' + user.username);
                    toast(formatMessage({id: 'fusion.toast.copiedUser', defaultMessage: 'Copied @{username}'}, {username: user.username}));
                })}
            />
            {moderation && <MenuSeparator/>}
            {moderation && canRemoveFromChannel && inChannel && (
                <MenuItem
                    icon='leave'
                    danger={true}
                    label={formatMessage({id: 'fusion.userMenu.removeChannel', defaultMessage: 'Remove from channel'})}
                    onClick={run(removeFromChannel)}
                />
            )}
            {moderation && canRemoveFromTeam && (
                <MenuItem
                    icon='shield'
                    danger={true}
                    label={formatMessage({id: 'fusion.userMenu.removeTeam', defaultMessage: 'Remove from team'})}
                    onClick={onRemoveFromTeam}
                />
            )}
        </Popover>
    );
}

// useUserMenu gives a component a person's right-click menu: the menu (and the profile card or confirmation it
// leads to) to render, and the context menu handler to put on the elements that stand for that person.
export function useUserMenu(): [React.ReactNode, (userId: string, e: React.MouseEvent<HTMLElement>) => void] {
    const [target, setTarget] = useState<Target | null>(null);
    const [mode, setMode] = useState<'menu' | 'profile' | 'remove'>('menu');
    const close = useCallback(() => setTarget(null), []);

    const open = useCallback((userId: string, e: React.MouseEvent<HTMLElement>) => {
        e.preventDefault();
        setMode('menu');
        setTarget({userId, anchor: e.currentTarget, point: {x: e.clientX, y: e.clientY}});
    }, []);

    let node: React.ReactNode = null;
    if (target && mode === 'menu') {
        node = (
            <UserMenu
                target={target}
                onProfile={() => setMode('profile')}
                onRemoveFromTeam={() => setMode('remove')}
                onClose={close}
            />
        );
    } else if (target && mode === 'profile') {
        node = (
            <UserPopover
                userId={target.userId}
                anchor={target.anchor}
                onClose={close}
            />
        );
    } else if (target && mode === 'remove') {
        node = (
            <ConfirmRemove
                userId={target.userId}
                onClose={close}
            />
        );
    }
    return [node, open];
}
