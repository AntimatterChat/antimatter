// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useState} from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useDispatch, useSelector} from 'react-redux';

import type {Team} from '@mattermost/types/teams';

import {removeChannelMember} from 'mattermost-redux/actions/channels';
import {addUserToTeam} from 'mattermost-redux/actions/teams';
import {Client4} from 'mattermost-redux/client';
import {Permissions} from 'mattermost-redux/constants';
import {canManageAnyChannelMembersInCurrentTeam, getCurrentChannel} from 'mattermost-redux/selectors/entities/channels';
import {haveIChannelPermission, haveITeamPermission, getMemberTimeout} from 'mattermost-redux/selectors/entities/roles';
import {getCurrentTeam, getMyTeams, getTeamMember} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';
import {isSystemAdmin} from 'mattermost-redux/utils/user_utils';

import {openDirectChannelToUserId} from 'actions/channel_actions';

import AddUserToChannelModal from 'components/add_user_to_channel_modal';

import EditAttributesItem from 'fusion/attributes/edit_attributes_item';
import {isCommunityChannelFor, isDefaultChannel} from 'fusion/channel/community_channels';
import {Popover} from 'fusion/components/layer';
import {Flyout, MenuItem, MenuSeparator} from 'fusion/components/menu';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import {canModerateTeam, openModView, timeOutMember} from 'fusion/moderation/moderation';
import CopyIdItem from 'fusion/popovers/copy_id_item';
import {useBlockActions} from 'fusion/privacy/message_requests';
import {useDialogs} from 'fusion/shell/dialogs_context';
import {useToast} from 'fusion/shell/toast_context';
import {openDialog} from 'fusion/utils/modals';
import {channelPath} from 'fusion/utils/paths';
import {getHistory} from 'utils/browser_history';
import Constants, {ModalIdentifiers} from 'utils/constants';
import {copyToClipboard} from 'utils/utils';

import type {GlobalState} from 'types/store';

import UserPopover from './user_popover';

// The composer of the open conversation listens for this event to insert a mention.
export const MENTION_EVENT = 'am-mention';

export function insertMention(username: string) {
    window.dispatchEvent(new CustomEvent(MENTION_EVENT, {detail: `@${username} `}));
}

export type Target = {userId: string; anchor: HTMLElement; point: {x: number; y: number}};

// UserMenu is the menu of a person: right-click on their avatar, name or member row, or "⋯" on their profile card.
// The mockup's userMenu, with moderation for those allowed to: the mod view, timeouts, and removing them from the
// channel or the team.
export function UserMenu({target, onProfile, onClose}: {target: Target; onProfile?: () => void; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const {userId} = target;
    const user = useUser(userId);
    const name = useDisplayName(user);
    const me = useSelector(getCurrentUserId);
    const team = useSelector(getCurrentTeam);
    const channel = useSelector(getCurrentChannel);
    const canAddToChannel = useSelector((state: GlobalState) => {
        const member = team ? getTeamMember(state, team.id, userId) : undefined;
        return userId !== me && canManageAnyChannelMembersInCurrentTeam(state) && Boolean(member) && member?.delete_at === 0;
    });
    const inChannel = useSelector((state: GlobalState) => Boolean(channel && state.entities.users.profilesInChannel[channel.id]?.has(userId)));
    const canRemoveFromChannel = useSelector((state: GlobalState) => {
        if (!channel || !team || (channel.type !== Constants.OPEN_CHANNEL && channel.type !== Constants.PRIVATE_CHANNEL) || isDefaultChannel(state, channel) || isCommunityChannelFor(state, channel, userId) || channel.delete_at) {
            return false;
        }
        const permission = channel.type === Constants.OPEN_CHANNEL ? Permissions.MANAGE_PUBLIC_CHANNEL_MEMBERS : Permissions.MANAGE_PRIVATE_CHANNEL_MEMBERS;
        if (!haveIChannelPermission(state, team.id, channel.id, permission)) {
            return false;
        }

        // As the server: only the channel's admins, the community's admins and its moderators remove others; the
        // server also checks that a moderator ranks above the person.
        return haveIChannelPermission(state, team.id, channel.id, Permissions.MANAGE_CHANNEL_ROLES) ||
            haveITeamPermission(state, team.id, Permissions.MANAGE_TEAM) ||
            haveITeamPermission(state, team.id, Permissions.REMOVE_USER_FROM_TEAM);
    });
    const canRemoveFromTeam = useSelector((state: GlobalState) => Boolean(team) && haveITeamPermission(state, team!.id, Permissions.REMOVE_USER_FROM_TEAM));
    const canModerate = useSelector((state: GlobalState) => Boolean(team) && canModerateTeam(state, team!.id));
    const timedOut = useSelector((state: GlobalState) => Boolean(team) && Boolean(getMemberTimeout(state, team!.id, userId)));
    const dialogs = useDialogs();
    const blocking = useBlockActions(userId);

    // The other communities this person could be invited to: those you're in and may add people to, where they aren't
    // known to be a member (archived communities and those synced to LDAP groups take nobody by hand).
    const inviteTargets = useSelector((state: GlobalState) => {
        if (userId === me) {
            return [];
        }
        return getMyTeams(state).filter((t) => {
            const member = getTeamMember(state, t.id, userId);
            return t.id !== team?.id && !t.delete_at && !t.group_constrained && !(member && member.delete_at === 0) &&
                haveITeamPermission(state, t.id, Permissions.ADD_USER_TO_TEAM);
        });
    }, shallowEqual);

    if (!user) {
        return null;
    }
    const isMe = userId === me;
    const inviteTo = async (target: Team) => {
        const community = target.display_name;
        try {
            // Asking first tells an invitation from someone already there, whom adding again would silently keep.
            await Client4.getTeamMember(target.id, userId);
            toast(formatMessage({id: 'fusion.userMenu.inviteAlready', defaultMessage: '{name} is already in {community}'}, {name, community}));
            return;
        } catch {
            // Not a member: add them below.
        }
        const result = await dispatch(addUserToTeam(target.id, userId));
        if ('error' in result && result.error) {
            toast((result.error as {message?: string}).message || formatMessage({id: 'fusion.userMenu.inviteFailed', defaultMessage: '{name} couldn\'t be added to {community}'}, {name, community}));
            return;
        }
        toast(formatMessage({id: 'fusion.userMenu.invited', defaultMessage: '{name} was added to {community}'}, {name, community}));
    };
    const run = (action: () => void) => () => {
        onClose();
        action();
    };

    const message = async () => {
        const result = await dispatch(openDirectChannelToUserId(userId));
        if ('data' in result && result.data && team) {
            getHistory().push(channelPath(team.name, result.data));
        } else if ('error' in result && result.error) {
            // Someone who blocked you, or whom you block.
            toast((result.error as {message?: string}).message || formatMessage({id: 'fusion.userMenu.messageFailed', defaultMessage: 'You can\'t message {name}.'}, {name}));
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

    const liftTimeout = async () => {
        if (!team) {
            return;
        }
        const result = await dispatch(timeOutMember(team.id, userId, 0));
        toast('error' in result && result.error ? ((result.error as {message?: string}).message || formatMessage({id: 'fusion.timeout.liftFailed', defaultMessage: 'The timeout couldn\'t be lifted.'})) : formatMessage({id: 'fusion.timeout.lifted', defaultMessage: '{name}\'s timeout is lifted'}, {name}));
    };

    const moderation = !isMe && ((canRemoveFromChannel && inChannel) || canRemoveFromTeam || canModerate);
    return (
        <Popover
            point={target.point}
            placement='point'
            className='plus-pop ch-menu user-menu'
            toggles={[target.anchor]}
            role='menu'
            label={name}
            onClose={onClose}
        >
            {onProfile && (
                <MenuItem
                    icon='users'
                    label={formatMessage({id: 'fusion.userMenu.profile', defaultMessage: 'Profile'})}
                    onClick={onProfile}
                />
            )}
            {!isMe && (
                <MenuItem
                    icon='chat'
                    label={formatMessage({id: 'fusion.userMenu.message', defaultMessage: 'Message'})}
                    onClick={run(message)}
                />
            )}
            {canAddToChannel && user && (
                <MenuItem
                    icon='user-plus'
                    label={formatMessage({id: 'fusion.profile.addToChannel', defaultMessage: 'Add to a channel'})}
                    onClick={run(() => dispatch(openDialog(ModalIdentifiers.ADD_USER_TO_CHANNEL, AddUserToChannelModal, {user})))}
                />
            )}
            {inviteTargets.length > 0 && (
                <Flyout
                    icon='users'
                    label={formatMessage({id: 'fusion.userMenu.invite', defaultMessage: 'Invite to a community'})}
                    menuLabel={formatMessage({id: 'fusion.userMenu.inviteMenu', defaultMessage: 'Communities'})}
                >
                    {inviteTargets.map((t) => (
                        <MenuItem
                            key={t.id}
                            label={t.display_name}
                            onClick={run(() => inviteTo(t))}
                        />
                    ))}
                </Flyout>
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
            <EditAttributesItem
                userId={userId}
                onDone={onClose}
            />
            {/* Ignoring and blocking, as in Discord, unless the admin turned them off. System admins can be ignored,
                but not blocked: they always reach people. */}
            {!isMe && blocking.enabled && <MenuSeparator/>}
            {!isMe && blocking.enabled && (
                <MenuItem
                    icon={blocking.level === 'ignore' ? 'eye' : 'eye-off'}
                    label={blocking.level === 'ignore' ? formatMessage({id: 'fusion.userMenu.unignore', defaultMessage: 'Stop ignoring'}) : formatMessage({id: 'fusion.userMenu.ignore', defaultMessage: 'Ignore'})}
                    disabled={blocking.level === 'block'}
                    onClick={run(() => blocking.toggleIgnore())}
                />
            )}
            {!isMe && blocking.enabled && !isSystemAdmin(user.roles) && (
                <MenuItem
                    icon='ban'
                    danger={blocking.level !== 'block'}
                    label={blocking.level === 'block' ? formatMessage({id: 'fusion.userMenu.unblock', defaultMessage: 'Unblock'}) : formatMessage({id: 'fusion.userMenu.block', defaultMessage: 'Block…'})}
                    onClick={run(() => blocking.toggleBlock())}
                />
            )}
            {moderation && <MenuSeparator/>}
            {moderation && canModerate && (
                <MenuItem
                    icon='shield'
                    label={formatMessage({id: 'fusion.userMenu.modView', defaultMessage: 'Mod view'})}
                    onClick={run(() => dispatch(openModView(userId)))}
                />
            )}
            {moderation && canRemoveFromTeam && !timedOut && (
                <MenuItem
                    icon='timer'
                    danger={true}
                    label={formatMessage({id: 'fusion.userMenu.timeOut', defaultMessage: 'Time out…'})}
                    onClick={run(() => dialogs.timeOut(userId))}
                />
            )}
            {moderation && canRemoveFromTeam && timedOut && (
                <MenuItem
                    icon='timer'
                    label={formatMessage({id: 'fusion.userMenu.liftTimeout', defaultMessage: 'Remove timeout'})}
                    onClick={run(liftTimeout)}
                />
            )}
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
                    label={formatMessage({id: 'fusion.userMenu.removeTeam', defaultMessage: 'Remove from community…'})}
                    onClick={run(() => dialogs.removeFromTeam(userId))}
                />
            )}
            <CopyIdItem
                id={userId}
                onDone={onClose}
            />
        </Popover>
    );
}

// useUserMenu gives a component a person's right-click menu: the menu (and the profile card or confirmation it
// leads to) to render, the context menu handler to put on the elements that stand for that person, and a way to close
// it. Right-clicking the same element again closes it.
export function useUserMenu(): [React.ReactNode, (userId: string, e: React.MouseEvent<HTMLElement>) => void, () => void] {
    const [target, setTarget] = useState<Target | null>(null);
    const [mode, setMode] = useState<'menu' | 'profile'>('menu');
    const close = useCallback(() => setTarget(null), []);

    const open = useCallback((userId: string, e: React.MouseEvent<HTMLElement>) => {
        e.preventDefault();
        const anchor = e.currentTarget;
        const point = {x: e.clientX, y: e.clientY};
        setMode('menu');
        setTarget((open) => (open?.anchor === anchor ? null : {userId, anchor, point}));
    }, []);

    let node: React.ReactNode = null;
    if (target && mode === 'menu') {
        node = (
            <UserMenu
                target={target}
                onProfile={() => setMode('profile')}
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
    }
    return [node, open, close];
}
