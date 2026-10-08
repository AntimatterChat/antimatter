// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Team} from '@mattermost/types/teams';

import {Permissions} from 'mattermost-redux/constants';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {haveITeamPermission} from 'mattermost-redux/selectors/entities/roles';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';

import {setUnreadFilterEnabled} from 'actions/views/channel_sidebar';
import {getMainMenuPluginComponents} from 'selectors/plugins';
import {isUnreadFilterEnabled} from 'selectors/views/channel_sidebar';

import {Popover} from 'fusion/components/layer';
import {Flyout, MenuHeading, MenuItem, MenuSeparator} from 'fusion/components/menu';
import {useDialogs} from 'fusion/shell/dialogs_context';
import {useSettings} from 'fusion/shell/settings_context';
import {useToast} from 'fusion/shell/toast_context';
import {
    getUnreadChannelIdsInTeam,
    isHidingMutedChannels,
    isTeamMuted,
    markTeamAsRead,
    setHidingMutedChannels,
    setTeamMuted,
    withTeam,
} from 'fusion/sidebar/team_actions';
import {
    openBrowseChannels,
    openInvitePeople,
    openLeaveTeam,
    openTeamMembers,
    openTeamSettings,
} from 'fusion/utils/modals';

import type {GlobalState} from 'types/store';

type Props = {

    /** The team the menu is for: the current one by default. */
    team?: Team;
    anchor?: HTMLElement | null;
    point?: {x: number; y: number};
    width?: number;
    onClose: () => void;
};

// ServerMenu is a team's menu, opened from the sidebar header for the current team, and by right-clicking a team in
// the team rail. Its dialogs act on the current team: for another team, they switch to it first.
export default function ServerMenu({team: forTeam, anchor, point, width, onClose}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const dialogs = useDialogs();
    const toast = useToast();
    const currentTeam = useSelector(getCurrentTeam);
    const team = forTeam || currentTeam;
    const teamId = team?.id || '';
    const config = useSelector(getConfig);
    const canManage = useSelector((state: GlobalState) => haveITeamPermission(state, teamId, Permissions.MANAGE_TEAM));
    const canAddUsers = useSelector((state: GlobalState) => haveITeamPermission(state, teamId, Permissions.ADD_USER_TO_TEAM));
    const canInviteGuests = useSelector((state: GlobalState) => haveITeamPermission(state, teamId, Permissions.INVITE_GUEST)) && config.EnableGuestAccounts === 'true';
    const unread = useSelector((state: GlobalState) => getUnreadChannelIdsInTeam(state, teamId).length > 0);
    const muted = useSelector((state: GlobalState) => isTeamMuted(state, teamId));
    const hidingMuted = useSelector((state: GlobalState) => isHidingMutedChannels(state, teamId));
    const unreadOnly = useSelector(isUnreadFilterEnabled);
    const pluginItems = useSelector(getMainMenuPluginComponents);
    const plugins = useSelector((state: GlobalState) => state.plugins.plugins);
    const settings = useSettings();

    // Integrations' items, grouped under each plugin's name as in the mockup.
    const pluginGroups: Array<[string, typeof pluginItems]> = [];
    for (const item of pluginItems) {
        const name = plugins[item.pluginId]?.name || formatMessage({id: 'fusion.serverMenu.otherPlugins', defaultMessage: 'Other'});
        const group = pluginGroups.find(([n]) => n === name);
        if (group) {
            group[1].push(item);
        } else {
            pluginGroups.push([name, [item]]);
        }
    }

    const run = (action: () => void) => () => {
        action();
        onClose();
    };

    // Runs a dialog of the current team on this menu's team.
    const inTeam = (action: () => void) => run(() => {
        if (team) {
            dispatch(withTeam(team, action));
        }
    });

    return (
        <Popover
            anchor={anchor}
            point={point}
            placement={point ? 'point' : 'below'}
            className='plus-pop server-menu'
            role='menu'
            label={forTeam ? formatMessage({id: 'fusion.serverMenu.labelFor', defaultMessage: '{team} menu'}, {team: forTeam.display_name}) : formatMessage({id: 'fusion.serverMenu.label', defaultMessage: 'Team menu'})}
            onClose={onClose}
            style={width ? {width} : undefined}
        >
            {unread && (
                <MenuItem
                    icon='check'
                    label={formatMessage({id: 'fusion.serverMenu.markRead', defaultMessage: 'Mark all as read'})}
                    onClick={run(() => {
                        dispatch(markTeamAsRead(teamId));
                        toast(formatMessage({id: 'fusion.toast.markedRead', defaultMessage: 'Marked {name} as read'}, {name: team?.display_name || ''}));
                    })}
                />
            )}
            <MenuItem
                icon={muted ? 'bell' : 'bell-off'}
                label={muted ? formatMessage({id: 'fusion.serverMenu.unmute', defaultMessage: 'Unmute team'}) : formatMessage({id: 'fusion.serverMenu.mute', defaultMessage: 'Mute team'})}
                sub={muted ? undefined : formatMessage({id: 'fusion.serverMenu.muteSub', defaultMessage: 'Mutes all its categories'})}
                onClick={run(() => dispatch(setTeamMuted(teamId, !muted)))}
            />
            <MenuItem
                role='menuitemcheckbox'
                checked={unreadOnly}
                label={formatMessage({id: 'fusion.serverMenu.unreadOnly', defaultMessage: 'Show unread channels only'})}
                sub='Ctrl+Shift+U'
                onClick={run(() => dispatch(setUnreadFilterEnabled(!unreadOnly)))}
            />
            <MenuItem
                role='menuitemcheckbox'
                checked={hidingMuted}
                label={formatMessage({id: 'fusion.serverMenu.hideMuted', defaultMessage: 'Hide muted channels'})}
                onClick={run(() => dispatch(setHidingMutedChannels(teamId, !hidingMuted)))}
            />
            <MenuItem
                icon='bell'
                label={formatMessage({id: 'fusion.serverMenu.notifications', defaultMessage: 'Notification settings'})}
                onClick={run(() => settings.open('notifications'))}
            />
            <MenuSeparator/>
            {(canAddUsers || canInviteGuests) && (
                <MenuItem
                    icon='user-plus'
                    accent={true}
                    label={formatMessage({id: 'fusion.serverMenu.invite', defaultMessage: 'Invite people'})}
                    onClick={inTeam(() => dispatch(openInvitePeople()))}
                />
            )}
            {canManage && (
                <MenuItem
                    icon='cog'
                    label={formatMessage({id: 'fusion.serverMenu.settings', defaultMessage: 'Team settings'})}
                    onClick={inTeam(() => dispatch(openTeamSettings()))}
                />
            )}
            <MenuItem
                icon='users'
                label={canManage ? formatMessage({id: 'fusion.serverMenu.manageMembers', defaultMessage: 'Manage members'}) : formatMessage({id: 'fusion.serverMenu.viewMembers', defaultMessage: 'View members'})}
                onClick={inTeam(() => dispatch(openTeamMembers()))}
            />
            <MenuSeparator/>
            <MenuItem
                icon='plus'
                label={formatMessage({id: 'fusion.serverMenu.createChannel', defaultMessage: 'Create channel'})}
                onClick={inTeam(() => dialogs.createChannel())}
            />
            <MenuItem
                icon='folder'
                label={formatMessage({id: 'fusion.serverMenu.createCategory', defaultMessage: 'Create category'})}
                onClick={inTeam(() => dialogs.createCategory())}
            />
            <MenuItem
                icon='compass'
                label={formatMessage({id: 'fusion.serverMenu.browse', defaultMessage: 'Browse channels'})}
                onClick={inTeam(() => dispatch(openBrowseChannels()))}
            />
            {pluginItems.length > 0 && (
                <>
                    <MenuSeparator/>
                    <Flyout
                        icon='plug'
                        label={formatMessage({id: 'fusion.serverMenu.more', defaultMessage: 'More actions'})}
                        menuLabel={formatMessage({id: 'fusion.serverMenu.moreLabel', defaultMessage: 'More actions from plugins'})}
                    >
                        {pluginGroups.map(([name, items]) => (
                            <React.Fragment key={name}>
                                {pluginGroups.length > 1 && <MenuHeading>{name}</MenuHeading>}
                                {items.map((item) => (
                                    <button
                                        key={item.id}
                                        type='button'
                                        role='menuitem'
                                        onClick={run(() => item.action())}
                                    >
                                        {item.text}
                                    </button>
                                ))}
                            </React.Fragment>
                        ))}
                    </Flyout>
                </>
            )}
            <MenuSeparator/>
            <MenuItem
                icon='leave'
                danger={true}
                label={formatMessage({id: 'fusion.serverMenu.leave', defaultMessage: 'Leave team'})}
                onClick={inTeam(() => dispatch(openLeaveTeam()))}
            />
        </Popover>
    );
}
