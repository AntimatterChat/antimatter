// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';

import {Permissions} from 'mattermost-redux/constants';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {haveIChannelPermission, haveITeamPermission} from 'mattermost-redux/selectors/entities/roles';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';

import {Popover} from 'fusion/components/layer';
import {MenuItem} from 'fusion/components/menu';
import {useDialogs} from 'fusion/shell/dialogs_context';
import Constants from 'utils/constants';

import type {GlobalState} from 'types/store';

// useCanAddPeople tells which of the member list's "+" choices you have: adding team members to the channel, and
// inviting people to the team.
export function useCanAddPeople(channel?: Channel) {
    const team = useSelector(getCurrentTeam);
    const add = useSelector((state: GlobalState) => {
        // Anyone in a group message can add people to it: Mattermost makes a new group with everyone.
        if (channel?.type === Constants.GM_CHANNEL && !channel.delete_at) {
            return true;
        }
        if (!channel || !team || channel.delete_at || (channel.type !== Constants.OPEN_CHANNEL && channel.type !== Constants.PRIVATE_CHANNEL)) {
            return false;
        }
        const permission = channel.type === Constants.OPEN_CHANNEL ? Permissions.MANAGE_PUBLIC_CHANNEL_MEMBERS : Permissions.MANAGE_PRIVATE_CHANNEL_MEMBERS;
        return haveIChannelPermission(state, team.id, channel.id, permission);
    });
    const invite = useSelector((state: GlobalState) => Boolean(team) && (haveITeamPermission(state, team!.id, Permissions.ADD_USER_TO_TEAM) || (getConfig(state).EnableGuestAccounts === 'true' && haveITeamPermission(state, team!.id, Permissions.INVITE_GUEST))));
    return {add, invite};
}

type Props = {
    channel: Channel;
    anchor: HTMLElement | null;
    onClose: () => void;
};

// MemberAddPopover is the member list's "+" menu: the mockup's memAddPop.
export default function MemberAddPopover({channel, anchor, onClose}: Props) {
    const {formatMessage} = useIntl();
    const dialogs = useDialogs();
    const team = useSelector(getCurrentTeam);
    const can = useCanAddPeople(channel);

    const run = (action: () => void) => () => {
        onClose();
        action();
    };

    return (
        <Popover
            anchor={anchor}
            placement='beside'
            className='plus-pop mem-add-pop'
            role='menu'
            label={formatMessage({id: 'fusion.members.addMenu', defaultMessage: 'Add people'})}
            onClose={onClose}
        >
            {can.add && (
                <MenuItem
                    icon='user-plus'
                    label={formatMessage({id: 'fusion.members.addToChannel', defaultMessage: 'Add people to {where}'}, {where: '#' + channel.display_name})}
                    sub={formatMessage({id: 'fusion.members.addToChannelSub', defaultMessage: 'Members of {team}'}, {team: team?.display_name})}
                    onClick={run(() => dialogs.addPeople(channel))}
                />
            )}
            {can.invite && (
                <MenuItem
                    icon='link'
                    label={formatMessage({id: 'fusion.members.invite', defaultMessage: 'Invite to {team}'}, {team: team?.display_name})}
                    sub={formatMessage({id: 'fusion.members.inviteSub', defaultMessage: 'Create an invite link or send emails'})}
                    onClick={run(dialogs.invite)}
                />
            )}
        </Popover>
    );
}
