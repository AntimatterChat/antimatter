// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useDispatch, useSelector} from 'react-redux';

import type {Team} from '@mattermost/types/teams';

import {Permissions} from 'mattermost-redux/constants';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {haveITeamPermission} from 'mattermost-redux/selectors/entities/roles';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';

import {updateTeamsOrderForUser} from 'actions/team_actions';
import {setUnreadFilterEnabled} from 'actions/views/channel_sidebar';
import {getMainMenuPluginComponents, getSidebarBrowseOrAddChannelMenuPluginComponents} from 'selectors/plugins';
import {isUnreadFilterEnabled} from 'selectors/views/channel_sidebar';

import {isAllChannelsCommunity} from 'fusion/channel/community_channels';
import {Popover} from 'fusion/components/layer';
import {Flyout, MenuHeading, MenuItem, MenuSeparator} from 'fusion/components/menu';
import CopyIdItem from 'fusion/popovers/copy_id_item';
import {addToFolder, folderOfTeam, makeFolder, placeAfter, railItems, removeFromFolder, useFolders} from 'fusion/rail/folders';
import {getSortedMyTeams} from 'fusion/selectors';
import {useDialogs} from 'fusion/shell/dialogs_context';
import {comboKeys, comboLabel, getKeybinds} from 'fusion/shell/keybinds';
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
import {canOpenCommunitySettings, getCommunitySettingsAccess} from 'fusion/teams/community_settings_access';
import {getTeamRoles} from 'fusion/teams/roles';
import {am} from 'fusion/utils/class_names';
import {
    openInvitePeople,
    openLeaveTeam,
    openTeamMembers,
    openTeamSettings,
    openTeams,
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
    const hasOpenRoles = useSelector((state: GlobalState) => getTeamRoles(state, teamId).some((r) => r.self_assignable));

    // Community settings, for anyone who may manage something in it (the community, its members' roles, its roles,
    // model connections, membership rules or integrations), with only the pages they may use. Its Members and Teams
    // pages were entries of this menu: those who don't get the settings keep them here.
    const access = useSelector((state: GlobalState) => getCommunitySettingsAccess(state, teamId), shallowEqual);
    const settingsShown = canOpenCommunitySettings(access);
    const canAddUsers = useSelector((state: GlobalState) => haveITeamPermission(state, teamId, Permissions.ADD_USER_TO_TEAM));
    const canInviteGuests = useSelector((state: GlobalState) => haveITeamPermission(state, teamId, Permissions.INVITE_GUEST)) && config.EnableGuestAccounts === 'true';
    const unread = useSelector((state: GlobalState) => getUnreadChannelIdsInTeam(state, teamId).length > 0);
    const muted = useSelector((state: GlobalState) => isTeamMuted(state, teamId));
    const hidingMuted = useSelector((state: GlobalState) => isHidingMutedChannels(state, teamId));
    const unreadOnly = useSelector(isUnreadFilterEnabled);
    const unreadCombo = useSelector((state: GlobalState) => getKeybinds(state).unreadFilter);
    const unreadShortcut = unreadCombo ? {label: comboLabel(unreadCombo).join('+'), keys: comboKeys(unreadCombo)} : undefined;
    const pluginItems = useSelector(getMainMenuPluginComponents);
    const addChannelItems = useSelector(getSidebarBrowseOrAddChannelMenuPluginComponents);
    const plugins = useSelector((state: GlobalState) => state.plugins.plugins);
    const allChannels = useSelector((state: GlobalState) => isAllChannelsCommunity(state, teamId));
    const [folders, saveFolders] = useFolders();
    const myTeams = useSelector(getSortedMyTeams);
    const items = railItems(myTeams, folders);
    const folder = folderOfTeam(myTeams, folders, teamId);
    const otherFolders = items.flatMap((item) => (item.kind === 'folder' && item.folder.id !== folder?.id ? [item] : []));
    const looseTeams = items.flatMap((item) => (item.kind === 'team' && item.team.id !== teamId ? [item.team] : []));

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
            label={forTeam ? formatMessage({id: 'fusion.serverMenu.labelFor', defaultMessage: '{team} menu'}, {team: forTeam.display_name}) : formatMessage({id: 'fusion.serverMenu.label', defaultMessage: 'Community menu'})}
            onClose={onClose}
            style={width ? {width} : undefined}
        >
            {/* Who's in it: inviting, its settings (or, for those who manage nothing, its members and teams), roles. */}
            {(canAddUsers || canInviteGuests) && (
                <MenuItem
                    icon='user-plus'
                    accent={true}
                    label={formatMessage({id: 'fusion.serverMenu.invite', defaultMessage: 'Invite people'})}
                    onClick={inTeam(() => dispatch(openInvitePeople()))}
                />
            )}
            {settingsShown && (
                <MenuItem
                    icon='cog'
                    label={formatMessage({id: 'fusion.serverMenu.settings', defaultMessage: 'Community settings'})}
                    onClick={inTeam(() => dispatch(openTeamSettings()))}
                />
            )}
            {!settingsShown && (
                <MenuItem
                    icon='users'
                    label={formatMessage({id: 'fusion.serverMenu.viewMembers', defaultMessage: 'View members'})}
                    onClick={inTeam(() => dispatch(openTeamMembers()))}
                />
            )}
            {!settingsShown && access.teams && (
                <MenuItem
                    icon='tag'
                    label={formatMessage({id: 'fusion.serverMenu.userGroups', defaultMessage: 'Teams…'})}
                    onClick={inTeam(() => dispatch(openTeams()))}
                />
            )}
            {hasOpenRoles && (
                <MenuItem
                    icon='smile'
                    label={formatMessage({id: 'fusion.serverMenu.selfRoles', defaultMessage: 'Your roles…'})}
                    onClick={inTeam(() => dialogs.selfRoles())}
                />
            )}

            {/* One's picture, nickname, name and team badge in this community (My account has the global ones). */}
            <MenuItem
                icon='pen'
                label={formatMessage({id: 'fusion.serverMenu.communityProfile', defaultMessage: 'Your profile in {community}…'}, {community: team?.display_name || ''})}
                onClick={inTeam(() => dialogs.communityProfile())}
            />
            <MenuSeparator/>

            {/* Its channels. */}
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
            {addChannelItems.map((item) => (
                <button
                    key={item.id}
                    type='button'
                    role='menuitem'
                    onClick={inTeam(() => item.action?.(teamId))}
                >
                    <span className={am('plugin-ic')}>{item.icon}</span>
                    {item.text}
                </button>
            ))}
            <MenuItem
                icon='compass'
                label={formatMessage({id: 'fusion.serverMenu.browse', defaultMessage: 'Browse channels'})}
                sub={allChannels ? formatMessage({id: 'fusion.serverMenu.browseAllSub', defaultMessage: 'You\'re in every channel: archived ones, and those to ask for'}) : undefined}
                onClick={inTeam(() => dialogs.browseChannels())}
            />
            <MenuSeparator/>

            {/* How the sidebar shows it, and its notifications. The hide options are checkboxes: a check when on. */}
            <MenuItem
                icon='inbox'
                role='menuitemcheckbox'
                checked={unreadOnly}
                label={formatMessage({id: 'fusion.serverMenu.unreadOnly', defaultMessage: 'Hide read channels'})}
                shortcut={unreadShortcut}
                onClick={run(() => dispatch(setUnreadFilterEnabled(!unreadOnly)))}
            />
            <MenuItem
                icon='eye-off'
                role='menuitemcheckbox'
                checked={hidingMuted}
                label={formatMessage({id: 'fusion.serverMenu.hideMuted', defaultMessage: 'Hide muted channels'})}
                onClick={run(() => dispatch(setHidingMutedChannels(teamId, !hidingMuted)))}
            />
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
                label={muted ? formatMessage({id: 'fusion.serverMenu.unmute', defaultMessage: 'Unmute community'}) : formatMessage({id: 'fusion.serverMenu.mute', defaultMessage: 'Mute community'})}
                sub={muted ? undefined : formatMessage({id: 'fusion.serverMenu.muteSub', defaultMessage: 'Mutes all its categories'})}
                onClick={run(() => dispatch(setTeamMuted(teamId, !muted)))}
            />
            <MenuItem
                icon='bell'
                label={formatMessage({id: 'fusion.serverMenu.notifications', defaultMessage: 'Notification settings'})}
                onClick={run(() => dialogs.communityNotifications(teamId))}
            />
            <MenuItem
                icon='shield'
                label={formatMessage({id: 'fusion.serverMenu.privacy', defaultMessage: 'Privacy settings'})}
                onClick={run(() => dialogs.communityPrivacy(teamId))}
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
            {forTeam && (
                <>
                    <MenuSeparator/>
                    <Flyout
                        icon='folder'
                        label={formatMessage({id: 'fusion.serverMenu.toFolder', defaultMessage: 'Add to folder'})}
                        menuLabel={formatMessage({id: 'fusion.serverMenu.toFolderLabel', defaultMessage: 'Folders'})}
                    >
                        {otherFolders.map(({folder: f, teams: inFolder}) => (
                            <button
                                key={f.id}
                                type='button'
                                role='menuitem'
                                onClick={run(() => {
                                    const last = inFolder[inFolder.length - 1];
                                    if (last) {
                                        dispatch(updateTeamsOrderForUser(placeAfter(myTeams.map((t) => t.id), teamId, last.id)));
                                    }
                                    saveFolders(addToFolder(folders, teamId, f.id));
                                })}
                            >
                                {f.name || inFolder.map((t) => t.display_name).join(', ')}
                            </button>
                        ))}

                        {/* As dropping it on another community in the rail: the two make a folder. */}
                        {looseTeams.length > 0 && <MenuHeading>{formatMessage({id: 'fusion.serverMenu.newFolderWith', defaultMessage: 'New folder with'})}</MenuHeading>}
                        {looseTeams.map((other) => (
                            <button
                                key={other.id}
                                type='button'
                                role='menuitem'
                                onClick={run(() => {
                                    dispatch(updateTeamsOrderForUser(placeAfter(myTeams.map((t) => t.id), teamId, other.id)));
                                    saveFolders(makeFolder(folders, other.id, teamId));
                                })}
                            >
                                {other.display_name}
                            </button>
                        ))}
                    </Flyout>
                    {folder && (
                        <MenuItem
                            icon='x'
                            label={formatMessage({id: 'fusion.serverMenu.outOfFolder', defaultMessage: 'Remove from {folder}'}, {folder: folder.name || formatMessage({id: 'fusion.rail.folder', defaultMessage: 'Folder'})})}
                            onClick={run(() => saveFolders(removeFromFolder(folders, teamId)))}
                        />
                    )}
                </>
            )}
            <MenuSeparator/>
            <MenuItem
                icon='leave'
                danger={true}
                label={formatMessage({id: 'fusion.serverMenu.leave', defaultMessage: 'Leave community'})}
                onClick={inTeam(() => dispatch(openLeaveTeam()))}
            />
            <CopyIdItem
                id={team?.id || ''}
                onDone={onClose}
            />
        </Popover>
    );
}
