// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useMemo} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Team, TeamKind} from '@mattermost/types/teams';

import {getChannels} from 'mattermost-redux/actions/channels';
import {patchTeam} from 'mattermost-redux/actions/teams';
import {getChannelsInTeam} from 'mattermost-redux/selectors/entities/channels';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';

import {Choice, Select, Toggle} from 'fusion/modals/settings_controls';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';
import Constants from 'utils/constants';

import type {GlobalState} from 'types/store';

// The value of the join and leave messages' menu for "off".
const JOIN_LEAVE_OFF = 'off';

// ChannelsSettings is what members do with the community's channels, in its General settings: the kind of community
// (a guild has everyone in all its channels and only its admins archive them; a workspace lets members join the
// channels they want and archive those they created), whether everyone is in all its channels, its default channel,
// where people land, and the channel of its join and leave messages (its Welcome channel), or none.
export default function ChannelsSettings() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const team = useSelector(getCurrentTeam);
    const teamId = team?.id || '';
    const allInTeam = useSelector((state: GlobalState) => getChannelsInTeam(state)[teamId]);
    const channelsById = useSelector((state: GlobalState) => state.entities.channels.channels);

    // The community's public channels, those the admin isn't in too.
    useEffect(() => {
        if (teamId) {
            dispatch(getChannels(teamId, 0, 200));
        }
    }, [teamId, dispatch]);

    const publicChannels = useMemo(() => [...(allInTeam || [])].
        map((id) => channelsById[id]).
        filter((c) => c && c.type === Constants.OPEN_CHANNEL && c.delete_at === 0).
        sort((a, b) => a.display_name.localeCompare(b.display_name)), [allInTeam, channelsById]);

    if (!team) {
        return null;
    }

    const save = async (patch: Partial<Team>, done: string) => {
        const result = await dispatch(patchTeam({id: team.id, ...patch}));
        if ('error' in result && result.error) {
            toast((result.error as {message?: string}).message || formatMessage({id: 'fusion.teamChannels.failed', defaultMessage: 'The setting couldn\'t be saved.'}));
        } else {
            toast(done);
        }
    };

    const archived = formatMessage({id: 'fusion.teamChannels.defaultGone', defaultMessage: 'An archived channel'});

    // No choice is General while there is one; without it, people land in the first channel they're in.
    const general = publicChannels.find((c) => c.name === Constants.DEFAULT_CHANNEL);
    const chosen = team.default_channel_id || '';
    const automatic = general ? formatMessage({id: 'fusion.teamChannels.defaultTownSquare', defaultMessage: '#{channel} (the first channel)'}, {channel: general.display_name}) : formatMessage({id: 'fusion.teamChannels.defaultNone', defaultMessage: 'None: the first channel each person is in'});
    const options: Array<[string, string]> = [['', automatic]];
    for (const channel of publicChannels) {
        if (channel.id !== general?.id || chosen === channel.id) {
            options.push([channel.id, '#' + channel.display_name]);
        }
    }
    if (chosen && !options.some(([id]) => id === chosen)) {
        options.push([chosen, archived]);
    }

    // The join and leave messages go to the Welcome channel; communities made before it have them in their default
    // channel ('').
    const joinLeave = team.disable_join_leave_messages ? JOIN_LEAVE_OFF : (team.welcome_channel_id || '');
    const joinLeaveOptions: Array<[string, string]> = [
        [JOIN_LEAVE_OFF, formatMessage({id: 'fusion.teamChannels.joinLeaveOff', defaultMessage: 'Off'})],
        ['', formatMessage({id: 'fusion.teamChannels.joinLeaveDefault', defaultMessage: 'In the default channel'})],
        ...publicChannels.map((c): [string, string] => [c.id, '#' + c.display_name]),
    ];
    if (team.welcome_channel_id && !joinLeaveOptions.some(([id]) => id === team.welcome_channel_id)) {
        joinLeaveOptions.push([team.welcome_channel_id, archived]);
    }

    const kind: TeamKind = team.kind === 'workspace' ? 'workspace' : 'guild';
    return (
        <div className={am('team-channels')}>
            <Choice<TeamKind>
                title={formatMessage({id: 'fusion.teamChannels.kind', defaultMessage: 'Kind of community'})}
                desc={formatMessage({id: 'fusion.teamChannels.kindGuildDesc', defaultMessage: 'Guild: everyone is in all its channels, and only its admins (and roles that manage channels) archive them. Workspace, for a company or an organisation: members join the channels they want, and also archive the channels they created. Changing the kind turns "Members are in every channel" on for a guild, off for a workspace. In both, only the channels\' admins, the community\'s admins and moderators put messages on top.'})}
                value={kind}
                options={[
                    ['guild', formatMessage({id: 'fusion.teamChannels.kind.guild', defaultMessage: 'Guild'})],
                    ['workspace', formatMessage({id: 'fusion.teamChannels.kind.workspace', defaultMessage: 'Workspace'})],
                ]}
                onChange={(next) => save({kind: next}, next === 'workspace' ? formatMessage({id: 'fusion.teamChannels.kindNowWorkspace', defaultMessage: 'It\'s a workspace: members join the channels they want and archive those they created'}) : formatMessage({id: 'fusion.teamChannels.kindNowGuild', defaultMessage: 'It\'s a guild: everyone is being added to the channels they may see'}))}
            />
            <Toggle
                title={formatMessage({id: 'fusion.teamChannels.all', defaultMessage: 'Members are in every channel'})}
                desc={formatMessage({id: 'fusion.teamChannels.allDesc', defaultMessage: 'Everyone in the community is in all its public channels, and in the private channels their roles open. They mute channels instead of leaving them, and nobody is removed from one: moderate through the community (removal, timeouts) or the channel\'s permissions. Guests and bots are only in the channels they\'re added to. Turning it on adds everyone to those channels (communities created before this setting existed keep their channels until you turn it on); turning it off keeps everyone where they are, free to leave.'})}
                on={Boolean(team.all_channels_membership)}
                onChange={(on) => save({all_channels_membership: on}, on ? formatMessage({id: 'fusion.teamChannels.allOn', defaultMessage: 'Everyone is being added to the channels they may see'}) : formatMessage({id: 'fusion.teamChannels.allOff', defaultMessage: 'Members can leave channels again'}))}
            />
            <Select
                title={formatMessage({id: 'fusion.teamChannels.default', defaultMessage: 'Default channel'})}
                desc={formatMessage({id: 'fusion.teamChannels.defaultLandDesc', defaultMessage: 'Where people land. Everyone is in it and can\'t leave it. Archive it and people land in the first channel they\'re in.'})}
                value={chosen}
                options={options}
                onChange={(id) => save({default_channel_id: id}, formatMessage({id: 'fusion.teamChannels.defaultSaved', defaultMessage: 'Default channel saved'}))}
            />
            <Select
                title={formatMessage({id: 'fusion.teamChannels.joinLeave', defaultMessage: 'Join and leave messages'})}
                desc={formatMessage({id: 'fusion.teamChannels.joinLeaveDesc', defaultMessage: 'The channel that tells who joins and leaves the community: new communities\' Welcome channel. Everyone is in it. Where members are in every channel, the channels themselves don\'t tell who joins or leaves them.'})}
                value={joinLeave}
                options={joinLeaveOptions}
                onChange={(value) => save(
                    value === JOIN_LEAVE_OFF ? {disable_join_leave_messages: true} : {disable_join_leave_messages: false, welcome_channel_id: value},
                    value === JOIN_LEAVE_OFF ? formatMessage({id: 'fusion.teamChannels.joinLeaveOffSaved', defaultMessage: 'Nobody is told who joins and leaves'}) : formatMessage({id: 'fusion.teamChannels.joinLeaveSaved', defaultMessage: 'Join and leave messages saved'}),
                )}
            />
        </div>
    );
}
