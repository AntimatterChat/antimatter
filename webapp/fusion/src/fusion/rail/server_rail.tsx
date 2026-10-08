// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector, useStore} from 'react-redux';

import type {Team} from '@mattermost/types/teams';

import {Permissions} from 'mattermost-redux/constants';
import {getTeamsUnreadStatuses} from 'mattermost-redux/selectors/entities/channels';
import {haveISystemPermission} from 'mattermost-redux/selectors/entities/roles';
import {getCurrentTeam, getCurrentTeamId, getJoinableTeamIds} from 'mattermost-redux/selectors/entities/teams';

import {switchTeam, updateTeamsOrderForUser} from 'actions/team_actions';

import Icon from 'fusion/components/icon';
import {Dialog, Popover} from 'fusion/components/layer';
import {MenuItem} from 'fusion/components/menu';
import ServerMenu from 'fusion/popovers/server_menu';
import {getLastTeamChannelName, getSortedMyTeams, getUnreadDirectChannels} from 'fusion/selectors';
import {useGlobalSearch} from 'fusion/shell/global_search_context';
import {useLayout} from 'fusion/shell/layout_context';
import {DirectFace} from 'fusion/sidebar/direct_row';
import {am} from 'fusion/utils/class_names';
import {channelPath} from 'fusion/utils/paths';
import Pluggable from 'plugins/pluggable';
import {getHistory} from 'utils/browser_history';
import Constants from 'utils/constants';
import {imageURLForTeam} from 'utils/utils';

import type {GlobalState} from 'types/store';

import {addToFolder, isFolderOpen, railItems, removeFromFolder, setFolderOpen, useFolders} from './folders';
import type {Folder} from './folders';

const initialsOf = (name: string) => name.split(/\s+/).map((p) => p[0] || '').slice(0, 2).join('').toUpperCase();

type Drop = {onDragStart: () => void; onDrop: (after: boolean) => void; onDragEnd: () => void};

function TeamButton({team, active, unread, mentions, onSelect, drag}: {team: Team; active: boolean; unread: boolean; mentions: number; onSelect: () => void; drag: Drop}) {
    // Teams are reordered by dragging them in the rail, as in the classic team sidebar.
    const [mark, setMark] = useState<'before' | 'after' | null>(null);
    const icon = imageURLForTeam(team);

    // Right-clicking a team opens its menu, as in Discord.
    const [menu, setMenu] = useState<{x: number; y: number} | null>(null);

    // An icon that doesn't load (its file is gone from the server's storage) falls back to the initials, as
    // profile pictures do, rather than showing the browser's broken image.
    const [failedIcon, setFailedIcon] = useState('');
    return (
        <>
            <button
                className={am('srv', {active, unread: unread && !active, 'drop-before': mark === 'before', 'drop-after': mark === 'after'})}
                draggable={true}
                onDragStart={(e) => {
                    e.dataTransfer.effectAllowed = 'move';
                    e.dataTransfer.setData('application/x-am-team', team.id);
                    drag.onDragStart();
                }}
                onDragEnd={() => {
                    setMark(null);
                    drag.onDragEnd();
                }}
                onDragOver={(e) => {
                    if (!e.dataTransfer.types.includes('application/x-am-team')) {
                        return;
                    }
                    e.preventDefault();
                    const r = e.currentTarget.getBoundingClientRect();
                    setMark(e.clientY > r.top + (r.height / 2) ? 'after' : 'before');
                }}
                onDragLeave={() => setMark(null)}
                onDrop={(e) => {
                    if (!e.dataTransfer.types.includes('application/x-am-team')) {
                        return;
                    }
                    e.preventDefault();
                    const r = e.currentTarget.getBoundingClientRect();
                    setMark(null);
                    drag.onDrop(e.clientY > r.top + (r.height / 2));
                }}
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

function TeamFace({team, size}: {team: Team; size?: 'mini'}) {
    const icon = imageURLForTeam(team);
    const [failed, setFailed] = useState('');
    if (icon && icon !== failed) {
        return (
            <img
                className={am('srv-img', size)}
                src={icon}
                alt=''
                onError={() => setFailed(icon)}
            />
        );
    }
    return <span className={am('srv-initials', size)}>{initialsOf(team.display_name)}</span>;
}

// FolderDialog names a server folder.
function FolderDialog({folder, onSave, onClose}: {folder: Folder; onSave: (name: string) => void; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const [name, setName] = useState(folder.name);
    const title = formatMessage({id: 'fusion.rail.renameFolder', defaultMessage: 'Rename folder'});
    return (
        <Dialog
            label={title}
            onClose={onClose}
            onSubmit={() => {
                onSave(name.trim());
                onClose();
            }}
        >
            <div className={am('pane')}>
                <h2>{title}</h2>
                <label className={am('field')}>
                    <span>{formatMessage({id: 'fusion.rail.folderName', defaultMessage: 'Name'})}</span>
                    <input
                        autoFocus={true}
                        value={name}
                        maxLength={40}
                        onChange={(e) => setName(e.target.value)}
                    />
                </label>
                <div className={am('modal-actions')}>
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={onClose}
                    >
                        {formatMessage({id: 'fusion.settings.cancel', defaultMessage: 'Cancel'})}
                    </button>
                    <button className={am('btn', 'primary')}>{formatMessage({id: 'fusion.settings.save', defaultMessage: 'Save'})}</button>
                </div>
            </div>
        </Dialog>
    );
}

// FolderButton is a closed or open server folder: its teams' faces in a square when closed, a folder when open, with
// their unread and mention counts. Teams dropped on it join it.
function FolderButton({folder, teams, open, unread, mentions, onToggle, onDropTeam, onRename, onUngroup}: {folder: Folder; teams: Team[]; open: boolean; unread: boolean; mentions: number; onToggle: () => void; onDropTeam: () => void; onRename: () => void; onUngroup: () => void}) {
    const {formatMessage} = useIntl();
    const [menu, setMenu] = useState<{x: number; y: number} | null>(null);
    const [over, setOver] = useState(false);
    const name = folder.name || formatMessage({id: 'fusion.rail.folder', defaultMessage: 'Folder'});
    return (
        <>
            <button
                className={am('srv', 'folder', {open, unread: unread && !open, 'drop-in': over})}
                title={`${name} · ${teams.map((t) => t.display_name).join(', ')}`}
                aria-label={formatMessage({id: 'fusion.rail.folderLabel', defaultMessage: 'Folder {name}, {count, plural, one {# team} other {# teams}}'}, {name, count: teams.length})}
                aria-expanded={open}
                onClick={onToggle}
                onContextMenu={(e) => {
                    e.preventDefault();
                    setMenu({x: e.clientX, y: e.clientY});
                }}
                onDragOver={(e) => {
                    if (e.dataTransfer.types.includes('application/x-am-team')) {
                        e.preventDefault();
                        setOver(true);
                    }
                }}
                onDragLeave={() => setOver(false)}
                onDrop={(e) => {
                    if (e.dataTransfer.types.includes('application/x-am-team')) {
                        e.preventDefault();
                        setOver(false);
                        onDropTeam();
                    }
                }}
            >
                {open ? <Icon name='folder'/> : (
                    <span className={am('folder-faces')}>
                        {teams.slice(0, 4).map((team) => (
                            <TeamFace
                                key={team.id}
                                team={team}
                                size='mini'
                            />
                        ))}
                    </span>
                )}
                {mentions > 0 && !open && <span className={am('badge')}>{mentions}</span>}
            </button>
            {menu && (
                <Popover
                    point={menu}
                    placement='point'
                    className='plus-pop'
                    role='menu'
                    label={name}
                    onClose={() => setMenu(null)}
                >
                    <MenuItem
                        icon='pen'
                        label={formatMessage({id: 'fusion.rail.renameFolder', defaultMessage: 'Rename folder'})}
                        onClick={() => {
                            setMenu(null);
                            onRename();
                        }}
                    />
                    <MenuItem
                        icon='x'
                        label={formatMessage({id: 'fusion.rail.ungroup', defaultMessage: 'Remove folder'})}
                        sub={formatMessage({id: 'fusion.rail.ungroupSub', defaultMessage: 'Its teams stay in the rail'})}
                        onClick={() => {
                            setMenu(null);
                            onUngroup();
                        }}
                    />
                </Popover>
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
    const [folders, saveFolders] = useFolders();
    const [openFolders, setOpenFolders] = useState<Record<string, boolean>>({});
    const [renaming, setRenaming] = useState<Folder | null>(null);
    const [adding, setAdding] = useState(false);
    const addRef = useRef<HTMLButtonElement>(null);
    const folderOpen = (id: string) => openFolders[id] ?? isFolderOpen(id);
    const toggleFolder = (id: string) => {
        const open = !folderOpen(id);
        setFolderOpen(id, open);
        setOpenFolders({...openFolders, [id]: open});
    };

    // The team being dragged, to put it before or after the team it's dropped on; the order is the user's preference.
    const dragged = useRef<string | null>(null);
    const dropOn = (target: Team, after: boolean) => {
        const id = dragged.current;
        dragged.current = null;
        if (!id || id === target.id) {
            return;
        }
        const order = teams.map((t) => t.id).filter((t) => t !== id);
        order.splice(order.indexOf(target.id) + (after ? 1 : 0), 0, id);
        dispatch(updateTeamsOrderForUser(order));

        // Dropped among a folder's teams, a team joins the folder; among loose teams, it leaves its folder.
        const targetFolder = folders.find((f) => f.teamIds.includes(target.id));
        const ownFolder = folders.find((f) => f.teamIds.includes(id));
        if (targetFolder?.id !== ownFolder?.id) {
            saveFolders(targetFolder ? addToFolder(folders, id, targetFolder.id) : removeFromFolder(folders, id));
        }
    };
    const dropInFolder = (folder: Folder) => {
        const id = dragged.current;
        dragged.current = null;
        if (id && !folder.teamIds.includes(id)) {
            saveFolders(addToFolder(folders, id, folder.id));
        }
    };

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
            {railItems(teams, folders).map((item) => {
                const teamButton = (team: Team) => (
                    <TeamButton
                        key={team.id}
                        team={team}
                        active={team.id === currentTeamId && !inDirectMessages}
                        unread={unreadTeams.has(team.id)}
                        mentions={mentionsInTeam.get(team.id) || 0}
                        onSelect={() => selectTeam(team)}
                        drag={{
                            onDragStart: () => {
                                dragged.current = team.id;
                            },
                            onDrop: (after) => dropOn(team, after),
                            onDragEnd: () => {
                                dragged.current = null;
                            },
                        }}
                    />
                );
                if (item.kind === 'team') {
                    return teamButton(item.team);
                }
                const open = folderOpen(item.folder.id);
                return (
                    <div
                        key={item.folder.id}
                        className={am('srv-folder', {open})}
                    >
                        <FolderButton
                            folder={item.folder}
                            teams={item.teams}
                            open={open}
                            unread={item.teams.some((t) => unreadTeams.has(t.id))}
                            mentions={item.teams.reduce((n, t) => n + (mentionsInTeam.get(t.id) || 0), 0)}
                            onToggle={() => toggleFolder(item.folder.id)}
                            onDropTeam={() => dropInFolder(item.folder)}
                            onRename={() => setRenaming(item.folder)}
                            onUngroup={() => saveFolders(folders.filter((f) => f.id !== item.folder.id))}
                        />
                        {open && item.teams.map(teamButton)}
                    </div>
                );
            })}
            {renaming && (
                <FolderDialog
                    folder={renaming}
                    onSave={(name) => saveFolders(folders.map((f) => (f.id === renaming.id ? {...f, name} : f)))}
                    onClose={() => setRenaming(null)}
                />
            )}
            {(canJoin || canCreate) && (
                <button
                    ref={addRef}
                    className={am('srv', 'add', {active: adding})}
                    title={formatMessage({id: 'fusion.rail.add', defaultMessage: 'Add a team'})}
                    aria-label={formatMessage({id: 'fusion.rail.add', defaultMessage: 'Add a team'})}
                    aria-haspopup='menu'
                    aria-expanded={adding}
                    onClick={() => setAdding(!adding)}
                >
                    <Icon name='plus'/>
                </button>
            )}
            {adding && (
                <Popover
                    anchor={addRef.current}
                    placement='beside'
                    className='plus-pop'
                    role='menu'
                    label={formatMessage({id: 'fusion.rail.add', defaultMessage: 'Add a team'})}
                    onClose={() => setAdding(false)}
                >
                    {canJoin && (
                        <MenuItem
                            icon='compass'
                            label={formatMessage({id: 'fusion.rail.join', defaultMessage: 'Join another team'})}
                            sub={formatMessage({id: 'fusion.rail.joinSub', defaultMessage: 'Browse the teams open to you'})}
                            onClick={() => {
                                setAdding(false);
                                getHistory().push('/select_team');
                            }}
                        />
                    )}
                    {canCreate && (
                        <MenuItem
                            icon='plus'
                            label={formatMessage({id: 'fusion.rail.create', defaultMessage: 'Create a team'})}
                            sub={formatMessage({id: 'fusion.rail.createSub', defaultMessage: 'A new space with its own channels'})}
                            onClick={() => {
                                setAdding(false);
                                getHistory().push('/create_team');
                            }}
                        />
                    )}
                </Popover>
            )}
            <span className={am('rail-grow')}/>
            <div className={am('rail-plugins')}>
                <Pluggable pluggableName='BottomTeamSidebar'/>
            </div>
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
