// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector, useStore} from 'react-redux';

import type {Team} from '@mattermost/types/teams';

import {Permissions} from 'mattermost-redux/constants';
import {getTeamsUnreadStatuses} from 'mattermost-redux/selectors/entities/channels';
import {haveISystemPermission} from 'mattermost-redux/selectors/entities/roles';
import {getCurrentTeam, getCurrentTeamId, getJoinableTeamIds} from 'mattermost-redux/selectors/entities/teams';

import {switchTeam} from 'actions/team_actions';

import Icon from 'fusion/components/icon';
import ServerMenu from 'fusion/popovers/server_menu';
import {getLastTeamChannelName, getSortedMyTeams, getUnreadDirectChannels} from 'fusion/selectors';
import {useGlobalSearch} from 'fusion/shell/global_search_context';
import {useLayout} from 'fusion/shell/layout_context';
import {DirectFace} from 'fusion/sidebar/direct_row';
import {am} from 'fusion/utils/class_names';
import {channelPath} from 'fusion/utils/paths';
import {getHistory} from 'utils/browser_history';
import Constants from 'utils/constants';
import {imageURLForTeam} from 'utils/utils';

import type {GlobalState} from 'types/store';

const initialsOf = (name: string) => name.split(/\s+/).map((p) => p[0] || '').slice(0, 2).join('').toUpperCase();

function TeamButton({team, active, unread, mentions, onSelect}: {team: Team; active: boolean; unread: boolean; mentions: number; onSelect: () => void}) {
    const icon = imageURLForTeam(team);

    // Right-clicking a team opens its menu, as in Discord.
    const [menu, setMenu] = useState<{x: number; y: number} | null>(null);

    // An icon that doesn't load (its file is gone from the server's storage) falls back to the initials, as
    // profile pictures do, rather than showing the browser's broken image.
    const [failedIcon, setFailedIcon] = useState('');
    return (
        <>
            <button
                className={am('srv', {active, unread: unread && !active})}
                title={team.display_name}
                aria-label={team.display_name}
                aria-current={active ? 'page' : undefined}
                onClick={onSelect}
                onContextMenu={(e) => {
                    e.preventDefault();
                    setMenu({x: e.clientX, y: e.clientY});
                }}
            >
                {icon && icon !== failedIcon ? (
                    <img
                        className={am('srv-img')}
                        src={icon}
                        alt=''
                        onError={() => setFailedIcon(icon)}
                    />
                ) : initialsOf(team.display_name)}
                {mentions > 0 && !active && <span className={am('badge')}>{mentions}</span>}
            </button>
            {menu && (
                <ServerMenu
                    team={team}
                    point={menu}
                    onClose={() => setMenu(null)}
                />
            )}
        </>
    );
}

// ServerRail is the column of teams on the far left; the mockup calls teams servers. Direct messages waiting for you
// sit at its bottom, above search, as round faces under a chat bubble so they don't read as servers. All the
// conversations open from the chat bubble of the sidebar's dock.
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
    const store = useStore<GlobalState>();

    // Direct messages belong to no team: while you're in them, no team is selected, as in Discord.
    const inDirectMessages = useSelector((state: GlobalState) => {
        const type = state.entities.channels.channels[currentChannelId]?.type;
        return type === Constants.DM_CHANNEL || type === Constants.GM_CHANNEL;
    }) || layout.home;
    const canJoin = useSelector((state: GlobalState) => getJoinableTeamIds(state).length > 0);
    const canCreate = useSelector((state: GlobalState) => haveISystemPermission(state, {permission: Permissions.CREATE_TEAM}));

    const waitingDMs = unreadDMs.filter((dm) => dm.channel.id !== currentChannelId).slice(0, 5);

    const selectTeam = (team: Team) => {
        layout.setHome(false);
        layout.setNavOpen(true);
        if (team.id === currentTeamId && !inDirectMessages) {
            return;
        }

        // Back to the team's channel you were last in, rather than the direct message you just left.
        const channelName = getLastTeamChannelName(store.getState(), team.id);
        if (channelName) {
            getHistory().push(channelPath(team.name, {name: channelName}));
        } else {
            dispatch(switchTeam(`/${team.name}`));
        }
    };

    return (
        <nav
            className={am('servers')}
            aria-label={formatMessage({id: 'fusion.rail.label', defaultMessage: 'Teams'})}
        >
            {teams.map((team) => (
                <TeamButton
                    key={team.id}
                    team={team}
                    active={team.id === currentTeamId && !inDirectMessages}
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
            {waitingDMs.length > 0 && (
                <div
                    className={am('dm-dock')}
                    role='group'
                    aria-label={formatMessage({id: 'fusion.rail.unreadDMs', defaultMessage: 'Unread direct messages'})}
                >
                    <span
                        className={am('dm-dock-head')}
                        aria-hidden='true'
                    >
                        <Icon
                            name='chat'
                            size='sm'
                        />
                    </span>
                    {waitingDMs.map(({channel, mentions, messages}) => (
                        <button
                            key={channel.id}
                            className={am('srv', 'dm-srv')}
                            title={channel.display_name}
                            aria-label={formatMessage({id: 'fusion.rail.unreadDM', defaultMessage: '{name}, {count, plural, one {# unread message} other {# unread messages}}'}, {name: channel.display_name, count: mentions || messages})}
                            onClick={() => {
                                // The conversation opens beside the team's sidebar, as in the mockup.
                                layout.setNavOpen(false);
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
