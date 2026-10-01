// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {Permissions} from 'mattermost-redux/constants';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {haveICurrentTeamPermission} from 'mattermost-redux/selectors/entities/roles';

import {getMainMenuPluginComponents} from 'selectors/plugins';

import {Popover} from 'fusion/components/layer';
import {Flyout, MenuHeading, MenuItem, MenuSeparator} from 'fusion/components/menu';
import {useDialogs} from 'fusion/shell/dialogs_context';
import {useSettings} from 'fusion/shell/settings_context';
import {
    openBrowseChannels,
    openInvitePeople,
    openLeaveTeam,
    openTeamMembers,
    openTeamSettings,
} from 'fusion/utils/modals';

import type {GlobalState} from 'types/store';

type Props = {
    anchor: HTMLElement | null;
    width: number;
    onClose: () => void;
};

// ServerMenu is the team's menu, opened from the sidebar header.
export default function ServerMenu({anchor, width, onClose}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const dialogs = useDialogs();
    const config = useSelector(getConfig);
    const canManage = useSelector((state: GlobalState) => haveICurrentTeamPermission(state, Permissions.MANAGE_TEAM));
    const canAddUsers = useSelector((state: GlobalState) => haveICurrentTeamPermission(state, Permissions.ADD_USER_TO_TEAM));
    const canInviteGuests = useSelector((state: GlobalState) => haveICurrentTeamPermission(state, Permissions.INVITE_GUEST)) && config.EnableGuestAccounts === 'true';
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

    return (
        <Popover
            anchor={anchor}
            placement='below'
            className='plus-pop server-menu'
            role='menu'
            label={formatMessage({id: 'fusion.serverMenu.label', defaultMessage: 'Team menu'})}
            onClose={onClose}
            style={{width}}
        >
            {(canAddUsers || canInviteGuests) && (
                <MenuItem
                    icon='user-plus'
                    accent={true}
                    label={formatMessage({id: 'fusion.serverMenu.invite', defaultMessage: 'Invite people'})}
                    onClick={run(() => dispatch(openInvitePeople()))}
                />
            )}
            {canManage && (
                <MenuItem
                    icon='cog'
                    label={formatMessage({id: 'fusion.serverMenu.settings', defaultMessage: 'Team settings'})}
                    onClick={run(() => dispatch(openTeamSettings()))}
                />
            )}
            <MenuItem
                icon='users'
                label={canManage ? formatMessage({id: 'fusion.serverMenu.manageMembers', defaultMessage: 'Manage members'}) : formatMessage({id: 'fusion.serverMenu.viewMembers', defaultMessage: 'View members'})}
                onClick={run(() => dispatch(openTeamMembers()))}
            />
            <MenuSeparator/>
            <MenuItem
                icon='plus'
                label={formatMessage({id: 'fusion.serverMenu.createChannel', defaultMessage: 'Create channel'})}
                onClick={run(() => dialogs.createChannel())}
            />
            <MenuItem
                icon='folder'
                label={formatMessage({id: 'fusion.serverMenu.createCategory', defaultMessage: 'Create category'})}
                onClick={run(() => dialogs.createCategory())}
            />
            <MenuItem
                icon='compass'
                label={formatMessage({id: 'fusion.serverMenu.browse', defaultMessage: 'Browse channels'})}
                onClick={run(() => dispatch(openBrowseChannels()))}
            />
            <MenuSeparator/>
            <MenuItem
                icon='bell'
                label={formatMessage({id: 'fusion.serverMenu.notifications', defaultMessage: 'Notification settings'})}
                onClick={run(() => settings.open('notifications'))}
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
                onClick={run(() => dispatch(openLeaveTeam()))}
            />
        </Popover>
    );
}
