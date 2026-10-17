// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import {Permissions} from 'mattermost-redux/constants';
import {haveITeamPermission} from 'mattermost-redux/selectors/entities/roles';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';

import MemberListTeam from 'components/member_list_team';
import {MembershipRequirementsBanner} from 'components/team_members_modal/team_members_modal';

import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

type Props = {

    // Leaves the settings for the invitation dialog.
    onInvite: () => void;
};

// MembersSettings is the Members page of the community settings (the former Manage members dialog): the community's
// members, to search, with their role; those who manage the community's roles make people its admins or members, and
// remove them, from each one's role menu. Those who may add people invite more from here.
export default function MembersSettings({onInvite}: Props) {
    const {formatMessage} = useIntl();
    const team = useSelector(getCurrentTeam);
    const canInvite = useSelector((state: GlobalState) => Boolean(team) && (haveITeamPermission(state, team!.id, Permissions.ADD_USER_TO_TEAM) || haveITeamPermission(state, team!.id, Permissions.INVITE_GUEST)));
    if (!team) {
        return null;
    }
    return (
        <div className={am('team-members-page')}>
            {canInvite && (
                <div className={am('team-members-head')}>
                    <button
                        type='button'
                        className={am('btn', 'primary')}
                        onClick={onInvite}
                    >
                        <Icon
                            name='user-plus'
                            size='sm'
                        />
                        {formatMessage({id: 'fusion.serverMenu.invite', defaultMessage: 'Invite people'})}
                    </button>
                </div>
            )}
            <MembershipRequirementsBanner team={team}/>

            {/* The classic member list, with its search and its role menus, drawn with the Fusion theme. */}
            <div className={'more-modal ' + am('team-members')}>
                <MemberListTeam teamId={team.id}/>
            </div>
        </div>
    );
}
