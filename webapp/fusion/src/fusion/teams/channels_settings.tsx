// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useMemo} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Team} from '@mattermost/types/teams';

import {getChannels} from 'mattermost-redux/actions/channels';
import {patchTeam} from 'mattermost-redux/actions/teams';
import {getChannelsInTeam} from 'mattermost-redux/selectors/entities/channels';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';

import {Select, Toggle} from 'fusion/modals/settings_controls';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';
import Constants from 'utils/constants';

import type {GlobalState} from 'types/store';

// ChannelsSettings is the Channels tab of the community settings: whether everyone is in all its channels, as in
// Discord, and its default channel, where newcomers land.
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

    // No choice is Town Square while there is one; without it, people land in the first channel they're in.
    const townSquare = publicChannels.find((c) => c.name === Constants.DEFAULT_CHANNEL);
    const chosen = team.default_channel_id || '';
    const automatic = townSquare ? formatMessage({id: 'fusion.teamChannels.defaultTownSquare', defaultMessage: '#{channel} (the first channel)'}, {channel: townSquare.display_name}) : formatMessage({id: 'fusion.teamChannels.defaultNone', defaultMessage: 'None: the first channel each person is in'});
    const options: Array<[string, string]> = [['', automatic]];
    for (const channel of publicChannels) {
        if (channel.id !== townSquare?.id || chosen === channel.id) {
            options.push([channel.id, '#' + channel.display_name]);
        }
    }
    if (chosen && !options.some(([id]) => id === chosen)) {
        options.push([chosen, formatMessage({id: 'fusion.teamChannels.defaultGone', defaultMessage: 'An archived channel'})]);
    }

    return (
        <div className={am('team-channels')}>
            <Toggle
                title={formatMessage({id: 'fusion.teamChannels.all', defaultMessage: 'Members are in every channel'})}
                desc={formatMessage({id: 'fusion.teamChannels.allDesc', defaultMessage: 'As in Discord: everyone in the community is in all its public channels, and in the private channels their roles open. They mute channels instead of leaving them, and nobody is removed from one: moderate through the community (removal, timeouts) or the channel\'s permissions. Guests and bots are only in the channels they\'re added to. Turning it on adds everyone to those channels (communities created before this setting existed keep their channels until you turn it on); turning it off keeps everyone where they are, free to leave.'})}
                on={Boolean(team.all_channels_membership)}
                onChange={(on) => save({all_channels_membership: on}, on ? formatMessage({id: 'fusion.teamChannels.allOn', defaultMessage: 'Everyone is being added to the channels they may see'}) : formatMessage({id: 'fusion.teamChannels.allOff', defaultMessage: 'Members can leave channels again'}))}
            />
            <Select
                title={formatMessage({id: 'fusion.teamChannels.default', defaultMessage: 'Default channel'})}
                desc={formatMessage({id: 'fusion.teamChannels.defaultDesc', defaultMessage: 'Where newcomers land, and where the community\'s join and leave messages go. Everyone is in it and can\'t leave it. Archive it and people land in the first channel they\'re in.'})}
                value={chosen}
                options={options}
                onChange={(id) => save({default_channel_id: id}, formatMessage({id: 'fusion.teamChannels.defaultSaved', defaultMessage: 'Default channel saved'}))}
            />
        </div>
    );
}
