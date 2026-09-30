// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Team} from '@mattermost/types/teams';

import {Permissions} from 'mattermost-redux/constants';
import {getTeamsUnreadStatuses} from 'mattermost-redux/selectors/entities/channels';
import {haveISystemPermission} from 'mattermost-redux/selectors/entities/roles';
import {getCurrentTeam, getCurrentTeamId, getJoinableTeamIds} from 'mattermost-redux/selectors/entities/teams';

import {switchTeam} from 'actions/team_actions';

import Icon from 'fusion/components/icon';
import {getSortedMyTeams, getUnreadDirectChannels} from 'fusion/selectors';
import {useGlobalSearch} from 'fusion/shell/global_search_context';
import {useLayout} from 'fusion/shell/layout_context';
import {DirectFace} from 'fusion/sidebar/direct_row';
import {am} from 'fusion/utils/class_names';
import {channelPath} from 'fusion/utils/paths';
import {getHistory} from 'utils/browser_history';
import {imageURLForTeam} from 'utils/utils';

import type {GlobalState} from 'types/store';

const initialsOf = (name: string) => name.split(/\s+/).map((p) => p[0] || '').slice(0, 2).join('').toUpperCase();

function TeamButton({team, active, unread, mentions, onSelect}: {team: Team; active: boolean; unread: boolean; mentions: number; onSelect: () => void}) {
    const icon = imageURLForTeam(team);
    return (
        <button
            className={am('srv', {active, unread: unread && !active})}
            title={team.display_name}
            aria-label={team.display_name}
            aria-current={active ? 'page' : undefined}
            onClick={onSelect}
        >
            {icon ? (
                <img
                    className={am('srv-img')}
                    src={icon}
                    alt=''
                />
            ) : initialsOf(team.display_name)}
            {mentions > 0 && !active && <span className={am('badge')}>{mentions}</span>}
        </button>
    );
}

// ServerRail is the column of teams on the far left. The mockup calls teams servers; direct messages live under
// the home icon at the top.
export default function ServerRail() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const layout = useLayout();
    const globalSearch = useGlobalSearch();
    const teams = useSelector(getSortedMyTeams);
    const currentTeamId = useSelector(getCurrentTeamId);
    const currentTeam = useSelector(getCurrentTeam);
    const [unreadTeams, mentionsInTeam] = useSelector(getTeamsUnreadStatuses);
    const unreadDMs = useSelector(getUnreadDirectChannels);
    const currentChannelId = useSelector((state: GlobalState) => state.entities.channels.currentChannelId);
    const canJoin = useSelector((state: GlobalState) => getJoinableTeamIds(state).length > 0);
    const canCreate = useSelector((state: GlobalState) => haveISystemPermission(state, {permission: Permissions.CREATE_TEAM}));

    const dmMentions = unreadDMs.reduce((n, dm) => n + dm.mentions, 0);
    const dmBadge = dmMentions || unreadDMs.length;
    const homeLabel = formatMessage({id: 'fusion.rail.home', defaultMessage: 'Direct messages'});

    const selectTeam = (team: Team) => {
        layout.setHome(false);
        layout.setNavOpen(true);
        if (team.id !== currentTeamId) {
            dispatch(switchTeam(`/${team.name}`));
        }
    };

    return (
        <nav
            className={am('servers')}
            aria-label={formatMessage({id: 'fusion.rail.label', defaultMessage: 'Teams'})}
        >
            <button
                className={am('srv', 'home', {active: layout.home, unread: dmBadge > 0 && !layout.home})}
                title={homeLabel}
                aria-label={homeLabel}
                onClick={() => layout.setHome(true)}
            >
                <svg
                    className={am('mark')}
                    viewBox='0 0 64 64'
                    aria-hidden='true'
                >
                    <use href='#am-i-mark'/>
                </svg>
                {dmBadge > 0 && !layout.home && <span className={am('badge')}>{dmBadge}</span>}
            </button>
            {unreadDMs.filter((dm) => dm.channel.id !== currentChannelId).length > 0 && (
                <div
                    className={am('dm-tree')}
                    role='group'
                    aria-label={formatMessage({id: 'fusion.rail.unreadDMs', defaultMessage: 'Unread direct messages'})}
                >
                    {unreadDMs.filter((dm) => dm.channel.id !== currentChannelId).slice(0, 5).map(({channel, mentions, messages}) => (
                        <button
                            key={channel.id}
                            className={am('srv', 'dm-srv')}
                            title={channel.display_name}
                            aria-label={channel.display_name}
                            onClick={() => {
                                layout.setHome(true);
                                if (currentTeam) {
                                    getHistory().push(channelPath(currentTeam.name, channel));
                                }
                            }}
                        >
                            <DirectFace channel={channel}/>
                            <span className={am('badge')}>{mentions || messages}</span>
                        </button>
                    ))}
                </div>
            )}
            <span className={am('srv-sep')}/>
            {teams.map((team) => (
                <TeamButton
                    key={team.id}
                    team={team}
                    active={team.id === currentTeamId && !layout.home}
                    unread={unreadTeams.has(team.id)}
                    mentions={mentionsInTeam.get(team.id) || 0}
                    onSelect={() => selectTeam(team)}
                />
            ))}
            {(canJoin || canCreate) && (
                <button
                    className={am('srv', 'add')}
                    title={canJoin ? formatMessage({id: 'fusion.rail.join', defaultMessage: 'Join another team'}) : formatMessage({id: 'fusion.rail.create', defaultMessage: 'Create a team'})}
                    aria-label={canJoin ? formatMessage({id: 'fusion.rail.join', defaultMessage: 'Join another team'}) : formatMessage({id: 'fusion.rail.create', defaultMessage: 'Create a team'})}
                    onClick={() => getHistory().push(canJoin ? '/select_team' : '/create_team')}
                >
                    <Icon name='plus'/>
                </button>
            )}
            <span className={am('rail-grow')}/>
            <button
                className={am('srv', 'rail-search')}
                title={formatMessage({id: 'fusion.rail.search', defaultMessage: 'Search everything (Ctrl K)'})}
                aria-label={formatMessage({id: 'fusion.rail.searchLabel', defaultMessage: 'Search everything'})}
                onClick={globalSearch.open}
            >
                <Icon name='search'/>
            </button>
        </nav>
    );
}
