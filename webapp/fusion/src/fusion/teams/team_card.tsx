// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
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
import {useExclusiveCard} from 'fusion/components/layer/exclusive';
import type {Placement} from 'fusion/components/layer/layer';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import {am} from 'fusion/utils/class_names';
import {channelPath} from 'fusion/utils/paths';
import {getHistory} from 'utils/browser_history';

import type {GlobalState} from 'types/store';

import TeamEditDialog from './team_edit_dialog';
import {useBadgeTeam} from './teams';

const FACES = 10;

const OPEN_TEAM_CARD = 'am-open-team-card';

type OpenTeamCard = {teamId: string; x: number; y: number};

// openTeamCard opens a team's card under its tag, for the whole app (see TeamCardHost).
function openTeamCard(e: React.MouseEvent<HTMLElement>, teamId: string) {
    e.stopPropagation();
    const rect = e.currentTarget.getBoundingClientRect();
    window.dispatchEvent(new CustomEvent<OpenTeamCard>(OPEN_TEAM_CARD, {detail: {teamId, x: rect.left, y: rect.bottom + 6}}));
}

// TeamTag is the short tag of the team someone shows next to their name, as a server tag; it opens the team's card,
// in place of a profile card it may sit in (one card at a time).
// A tag inside something clickable (a member row) is a label, not a button (buttons don't nest), but a click on it
// still opens the team's card rather than the row's profile card, as the tag does everywhere else; the keyboard
// reaches the team from the profile card.
// Teams belong to one community: the tag is the one shown in communityId (a message's community), else in the current
// community.
export function TeamTag({user, label = false, communityId}: {user: UserProfile | undefined; label?: boolean; communityId?: string}) {
    const team = useBadgeTeam(user, communityId);
    if (!team) {
        return null;
    }
    if (label) {
        return (
            <span
                className={am('team-tag')}
                title={team.display_name}
                onClick={(e) => openTeamCard(e, team.id)}
            >
                {team.tag}
            </span>
        );
    }
    return (
        <button
            type='button'
            className={am('team-tag')}
            title={team.display_name}
            onClick={(e) => openTeamCard(e, team.id)}
        >
            {team.tag}
        </button>
    );
}

// TeamCardHost shows the team card a tag opened, for the whole app: the card outlives the profile card or message the
// tag was in.
export function TeamCardHost() {
    const [open, setOpen] = useState<OpenTeamCard | null>(null);
    const team = useSelector((state: GlobalState) => (open ? state.entities.groups.groups[open.teamId] : undefined));
    useEffect(() => {
        const onOpen = (e: Event) => setOpen((e as CustomEvent<OpenTeamCard>).detail);
        window.addEventListener(OPEN_TEAM_CARD, onOpen);
        return () => window.removeEventListener(OPEN_TEAM_CARD, onOpen);
    }, []);
    if (!open || !team) {
        return null;
    }
    return (
        <TeamCard
            key={`${open.teamId}-${open.x}-${open.y}`}
            team={team}
            point={{x: open.x, y: open.y}}
            onClose={() => setOpen(null)}
        />
    );
}

type CardProps = {
    team: Group;
    anchor?: HTMLElement | null;
    point?: {x: number; y: number};
    placement?: Placement;
    onClose: () => void;
};

// TeamCard shows a team as in the mockup: its tag and name, what it does, its lead, its channel and its members, and a
// way to reach it: its channel, else its lead. Those who may edit the team can change its tag, lead and channel here.
export default function TeamCard({team, anchor, point, placement = 'below-start', onClose}: CardProps) {
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
    useExclusiveCard(onClose);

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
            point={point}
            placement={point ? 'point' : placement}
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
