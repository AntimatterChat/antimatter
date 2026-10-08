// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Group} from '@mattermost/types/groups';
import type {UserProfile} from '@mattermost/types/users';

import {getChannel as fetchChannel} from 'mattermost-redux/actions/channels';
import {getMissingProfilesByIds} from 'mattermost-redux/actions/users';
import {Permissions} from 'mattermost-redux/constants';
import {getChannel} from 'mattermost-redux/selectors/entities/channels';
import {haveIGroupPermission} from 'mattermost-redux/selectors/entities/roles';
import {getCurrentTeam, getTeam} from 'mattermost-redux/selectors/entities/teams';

import {openDirectChannelToUserId} from 'actions/channel_actions';

import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import {Popover} from 'fusion/components/layer';
import type {Placement} from 'fusion/components/layer/layer';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import {am} from 'fusion/utils/class_names';
import {channelPath} from 'fusion/utils/paths';
import {getHistory} from 'utils/browser_history';

import type {GlobalState} from 'types/store';

import TeamEditDialog from './team_edit_dialog';
import {useBadgeTeam} from './teams';

const FACES = 10;

// TeamTag is the short tag of the team someone shows next to their name, as a server tag; it opens the team's card.
export function TeamTag({user}: {user: UserProfile | undefined}) {
    const team = useBadgeTeam(user);
    const ref = useRef<HTMLButtonElement>(null);
    const [open, setOpen] = useState(false);
    if (!team) {
        return null;
    }
    return (
        <>
            <button
                ref={ref}
                type='button'
                className={am('team-tag')}
                title={team.display_name}
                onClick={(e) => {
                    e.stopPropagation();
                    setOpen(!open);
                }}
            >
                {team.tag}
            </button>
            {open && (
                <TeamCard
                    team={team}
                    anchor={ref.current}
                    onClose={() => setOpen(false)}
                />
            )}
        </>
    );
}

type CardProps = {
    team: Group;
    anchor: HTMLElement | null;
    placement?: Placement;
    onClose: () => void;
};

// TeamCard shows a team as in the mockup: its tag and name, what it does, its lead, its channel and its members, and a
// way to reach it: its channel, else its lead. Those who may edit the team can change its tag, lead and channel here.
export default function TeamCard({team, anchor, placement = 'below-start', onClose}: CardProps) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const members = team.member_ids || [];
    const lead = useUser(team.lead_id || undefined);
    const leadName = useDisplayName(lead);
    const channel = useSelector((state: GlobalState) => (team.channel_id ? getChannel(state, team.channel_id) : undefined));
    const channelTeam = useSelector((state: GlobalState) => (channel?.team_id ? getTeam(state, channel.team_id) : undefined));
    const currentTeam = useSelector(getCurrentTeam);
    const canEdit = useSelector((state: GlobalState) => haveIGroupPermission(state, team.id, Permissions.EDIT_CUSTOM_GROUP));
    const [editing, setEditing] = useState(false);

    useEffect(() => {
        dispatch(getMissingProfilesByIds([...members.slice(0, FACES), ...(team.lead_id ? [team.lead_id] : [])]));
        if (team.channel_id && !channel) {
            dispatch(fetchChannel(team.channel_id));
        }

        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [team.id]);

    if (editing) {
        return (
            <TeamEditDialog
                team={team}
                onClose={onClose}
            />
        );
    }

    const go = async () => {
        if (channel && channelTeam) {
            getHistory().push(channelPath(channelTeam.name, channel));
        } else if (team.lead_id && currentTeam) {
            const result = await dispatch(openDirectChannelToUserId(team.lead_id));
            if ('data' in result && result.data) {
                getHistory().push(channelPath(currentTeam.name, result.data));
            }
        }
        onClose();
    };

    let reach = null;
    if (channel && channelTeam) {
        reach = (
            <button
                type='button'
                className={am('btn', 'primary')}
                onClick={go}
            >
                <Icon
                    name='hash'
                    size='sm'
                />
                {formatMessage({id: 'fusion.teams.jump', defaultMessage: 'Jump to #{channel}'}, {channel: channel.display_name})}
            </button>
        );
    } else if (lead) {
        reach = (
            <button
                type='button'
                className={am('btn')}
                onClick={go}
            >
                <Icon
                    name='chat'
                    size='sm'
                />
                {formatMessage({id: 'fusion.teams.messageLead', defaultMessage: 'Message {name}, the lead'}, {name: lead.first_name || leadName})}
            </button>
        );
    }

    return (
        <Popover
            anchor={anchor}
            placement={placement}
            className='team-pop'
            label={team.display_name}
            onClose={onClose}
        >
            <div className={am('thead')}>
                <span className={am('big-tag')}>{team.tag || team.display_name.slice(0, 2).toUpperCase()}</span>
                <div>
                    <h4>{team.display_name}</h4>
                    <span className={am('sub')}>{team.name ? `@${team.name}` : formatMessage({id: 'fusion.teams.team', defaultMessage: 'Team'})}</span>
                </div>
                {canEdit && (
                    <button
                        type='button'
                        className={am('icon-btn')}
                        title={formatMessage({id: 'fusion.teams.edit', defaultMessage: 'Edit the team'})}
                        aria-label={formatMessage({id: 'fusion.teams.edit', defaultMessage: 'Edit the team'})}
                        onClick={() => setEditing(true)}
                    >
                        <Icon
                            name='pen'
                            size='sm'
                        />
                    </button>
                )}
            </div>
            {team.description && <p>{team.description}</p>}
            <div className={am('facts')}>
                <span>
                    <Icon name='users'/>
                    {formatMessage({id: 'fusion.teams.members', defaultMessage: '{count, plural, one {# member} other {# members}}'}, {count: members.length || team.member_count || 0})}
                </span>
                {lead && (
                    <span>
                        <Icon name='shield'/>
                        {formatMessage({id: 'fusion.teams.lead', defaultMessage: 'Lead: {name}'}, {name: leadName})}
                    </span>
                )}
                <span>
                    <Icon name={channel ? 'hash' : 'chat'}/>
                    {channel ? formatMessage({id: 'fusion.teams.channel', defaultMessage: 'Channel: #{channel}'}, {channel: channel.display_name}) : formatMessage({id: 'fusion.teams.noChannel', defaultMessage: 'No team channel yet'})}
                </span>
            </div>
            {members.length > 0 && (
                <div className={am('faces')}>
                    {members.slice(0, FACES).map((id) => (
                        <Avatar
                            key={id}
                            userId={id}
                            size='sm'
                            status={true}
                        />
                    ))}
                    {members.length > FACES && <span className={am('more-faces')}>{`+${members.length - FACES}`}</span>}
                </div>
            )}
            {reach}
        </Popover>
    );
}
