// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import type {Group} from '@mattermost/types/groups';

import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';

import TeamCard from './team_card';
import {getTeams} from './teams';

type Props = {

    // Leaves the settings for the teams dialog, which lists them all (yours, archived ones) and creates them.
    onOpenAll: () => void;
};

// TeamsSettings is the Teams page of the community settings (the former Teams… entry of its menu): the community's
// teams, each opening its card (where those who may edit it change its tag, lead and channel), and the way to the
// teams dialog to find, create and archive them.
export default function TeamsSettings({onOpenAll}: Props) {
    const {formatMessage} = useIntl();
    const teams = useSelector(getTeams);
    const [open, setOpen] = useState<{team: Group; anchor: HTMLElement} | null>(null);
    return (
        <div className={am('team-teams-page')}>
            <p className={am('lead')}>{formatMessage({id: 'fusion.teamSettings.teamsDesc', defaultMessage: 'Teams have a tag people show next to their name, a lead and a channel.'})}</p>
            {teams.length === 0 && <p className={am('lead')}>{formatMessage({id: 'fusion.teamSettings.noTeams', defaultMessage: 'No teams yet.'})}</p>}
            <div className={am('role-list')}>
                {teams.map((team) => (
                    <button
                        key={team.id}
                        type='button'
                        className={am('role-row', 'team-row')}
                        aria-haspopup='dialog'
                        onClick={(e) => {
                            const anchor = e.currentTarget;
                            setOpen((current) => (current?.team.id === team.id ? null : {team, anchor}));
                        }}
                    >
                        <span className={am('team-tag', 'static')}>{team.tag || team.display_name.slice(0, 2).toUpperCase()}</span>
                        <span className={am('grow')}>
                            <b>{team.display_name}</b>
                            <span>{formatMessage({id: 'fusion.teams.members', defaultMessage: '{count, plural, one {# member} other {# members}}'}, {count: team.member_ids?.length || team.member_count || 0})}</span>
                        </span>
                    </button>
                ))}
            </div>
            <div className={am('team-teams-foot')}>
                <button
                    type='button'
                    className={am('btn')}
                    onClick={onOpenAll}
                >
                    <Icon
                        name='tag'
                        size='sm'
                    />
                    {formatMessage({id: 'fusion.teamSettings.allTeams', defaultMessage: 'Find and create teams…'})}
                </button>
            </div>
            {open && (
                <TeamCard
                    team={open.team}
                    anchor={open.anchor}
                    onClose={() => setOpen(null)}
                />
            )}
        </div>
    );
}
