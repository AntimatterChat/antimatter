// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {getChannelMember} from 'mattermost-redux/actions/channels';
import {getTeamMember} from 'mattermost-redux/actions/teams';
import {getChannelMembersInChannels, getCurrentChannel} from 'mattermost-redux/selectors/entities/channels';
import {getCurrentTeamId, getMembersInTeams} from 'mattermost-redux/selectors/entities/teams';
import {isGuest, isSystemAdmin} from 'mattermost-redux/utils/user_utils';

import {useUser} from 'fusion/hooks/users';
import {ROLE_COLORS} from 'fusion/members/member_groups';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

// RoleChips shows someone's Mattermost roles on their profile card, as the mockup's role chips: System Admin, Team
// Admin, Channel Admin, Guest, Bot.
export default function RoleChips({userId}: {userId: string}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const user = useUser(userId);
    const teamId = useSelector(getCurrentTeamId);
    const channel = useSelector(getCurrentChannel);
    const channelId = channel && (channel.type === 'O' || channel.type === 'P') ? channel.id : '';
    const teamMember = useSelector((state: GlobalState) => getMembersInTeams(state)[teamId]?.[userId]);
    const channelMember = useSelector((state: GlobalState) => (channelId ? getChannelMembersInChannels(state)[channelId]?.[userId] : undefined));
    const inChannel = useSelector((state: GlobalState) => Boolean(channelId && state.entities.users.profilesInChannel[channelId]?.has(userId)));

    useEffect(() => {
        if (teamId && !teamMember) {
            dispatch(getTeamMember(teamId, userId));
        }
        if (channelId && inChannel && !channelMember) {
            dispatch(getChannelMember(channelId, userId));
        }

        // Once per person and place.
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [teamId, channelId, userId, inChannel]);

    if (!user) {
        return null;
    }
    const chips: Array<[string, string]> = [];
    if (isSystemAdmin(user.roles || '')) {
        chips.push([formatMessage({id: 'fusion.roles.systemAdmin', defaultMessage: 'System Admin'}), ROLE_COLORS.admin]);
    }
    if (teamMember?.scheme_admin) {
        chips.push([formatMessage({id: 'fusion.roles.teamAdmin', defaultMessage: 'Team Admin'}), ROLE_COLORS.mod]);
    }
    if (channelMember?.scheme_admin) {
        chips.push([formatMessage({id: 'fusion.roles.channelAdmin', defaultMessage: 'Channel Admin'}), ROLE_COLORS.mod]);
    }
    if (isGuest(user.roles || '')) {
        chips.push([formatMessage({id: 'fusion.roles.guest', defaultMessage: 'Guest'}), 'var(--am-muted)']);
    }
    if (user.is_bot) {
        chips.push([formatMessage({id: 'fusion.roles.bot', defaultMessage: 'Bot'}), ROLE_COLORS.bot]);
    }
    if (!chips.length) {
        return null;
    }
    return (
        <div className={am('roles')}>
            {chips.map(([label, color]) => (
                <span
                    key={label}
                    className={am('role-chip')}
                >
                    <i style={{background: color}}/>
                    {label}
                </span>
            ))}
        </div>
    );
}
