// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector, useStore} from 'react-redux';

import type {Team} from '@mattermost/types/teams';

import {Permissions} from 'mattermost-redux/constants';
import {getTeamsUnreadStatuses} from 'mattermost-redux/selectors/entities/channels';
import {haveISystemPermission} from 'mattermost-redux/selectors/entities/roles';
import {getCurrentTeamId, getJoinableTeamIds} from 'mattermost-redux/selectors/entities/teams';

import {switchTeam, updateTeamsOrderForUser} from 'actions/team_actions';

import Icon from 'fusion/components/icon';
import {Dialog, Popover} from 'fusion/components/layer';
import {MenuItem} from 'fusion/components/menu';
import {useHideUnreads} from 'fusion/focus/focus';
import ServerMenu from 'fusion/popovers/server_menu';
import {getLastTeamChannelName, getSortedMyTeams} from 'fusion/selectors';
import {useDialogs} from 'fusion/shell/dialogs_context';
import {useGlobalSearch} from 'fusion/shell/global_search_context';
import {useLayout} from 'fusion/shell/layout_context';
import {markTeamAsRead} from 'fusion/sidebar/team_actions';
import {a11yRegion} from 'fusion/utils/a11y_regions';
import {am} from 'fusion/utils/class_names';
import {channelPath} from 'fusion/utils/paths';
import Pluggable from 'plugins/pluggable';
import {getHistory} from 'utils/browser_history';
import Constants from 'utils/constants';
import {imageURLForTeam} from 'utils/utils';

import type {GlobalState} from 'types/store';

import {addToFolder, FOLDER_COLORS, folderOfTeam, isFolderOpen, makeFolder, placeAfter, railItems, removeFromFolder, setFolderOpen, useFolders} from './folders';
import type {Folder} from './folders';
import HistoryButtons, {showHistoryButtons} from './history_buttons';
import RailDirectMessages from './rail_dms';

const initialsOf = (name: string) => name.split(/\s+/).map((p) => p[0] || '').slice(0, 2).join('').toUpperCase();

// Where a dragged team lands on another: before it, after it, or on it (together in a folder, as in Discord).
export type DropZone = 'before' | 'after' | 'into';

type Drop = {onDragStart: () => void; onDrop: (zone: DropZone) => void; onDragEnd: () => void};

// dropZone tells where on a team the pointer is: its top and bottom quarters put the dragged team before or after it,
// its middle puts them together.
export function dropZone(e: React.DragEvent<HTMLElement>): DropZone {
    const r = e.currentTarget.getBoundingClientRect();
    const y = (e.clientY - r.top) / (r.height || 1);
    if (y < 0.25) {
        return 'before';
    }
    return y > 0.75 ? 'after' : 'into';
}

// menuPoint is where a context menu opens: at the pointer, or beside the element when opened from the keyboard.
function menuPoint(e: React.MouseEvent<HTMLElement>): {x: number; y: number} {
    if (e.clientX || e.clientY) {
        return {x: e.clientX, y: e.clientY};
    }
    const r = e.currentTarget.getBoundingClientRect();
    return {x: r.right, y: r.top + (r.height / 2)};
}

// The unread marks while they hide.
const NO_TEAMS = new Set<string>();
const NO_MENTIONS = new Map<string, number>();

function TeamButton({team, active, unread, mentions, onSelect, drag}: {team: Team; active: boolean; unread: boolean; mentions: number; onSelect: () => void; drag: Drop}) {
    // Teams are reordered by dragging them in the rail, as in the classic team sidebar, and dropped on each other to
    // make a folder.
    const [mark, setMark] = useState<DropZone | null>(null);
    const icon = imageURLForTeam(team);

    // Right-clicking a team opens its menu, as in Discord.
    const [menu, setMenu] = useState<{x: number; y: number} | null>(null);

    // An icon that doesn't load (its file is gone from the server's storage) falls back to the initials, as
    // profile pictures do, rather than showing the browser's broken image.
    const [failedIcon, setFailedIcon] = useState('');
    return (
        <>
            <button
                className={am('srv', {active, unread: unread && !active, 'drop-before': mark === 'before', 'drop-after': mark === 'after', 'drop-in': mark === 'into'})}
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
                    setMark(dropZone(e));
                }}
                onDragLeave={() => setMark(null)}
                onDrop={(e) => {
                    if (!e.dataTransfer.types.includes('application/x-am-team')) {
                        return;
                    }
                    e.preventDefault();
                    setMark(null);
                    drag.onDrop(dropZone(e));
                }}
                data-am-tip={team.display_name}
                data-am-tip-side='right'
                aria-label={team.display_name}
                aria-current={active ? 'page' : undefined}
                onClick={onSelect}
                onContextMenu={(e) => {
                    e.preventDefault();
                    setMenu(menuPoint(e));
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

// folderLabel is what a folder is called: its name, or its teams' names, as Discord's.
function folderLabel(folder: Folder, teams: Team[]): string {
    return folder.name || teams.map((t) => t.display_name).join(', ');
}

// FolderDialog sets a server folder's name and colour.
function FolderDialog({folder, onSave, onClose}: {folder: Folder; onSave: (name: string, color: string) => void; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const [name, setName] = useState(folder.name);
    const [color, setColor] = useState(folder.color || '');
    const title = formatMessage({id: 'fusion.rail.folderSettings', defaultMessage: 'Folder settings'});
    const colorLabel = formatMessage({id: 'fusion.rail.folderColor', defaultMessage: 'Colour'});
    return (
        <Dialog
            label={title}
            onClose={onClose}
            onSubmit={() => {
                onSave(name.trim(), color);
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
                        placeholder={formatMessage({id: 'fusion.rail.folderNamePlaceholder', defaultMessage: 'Shown when you point at the folder'})}
                        onChange={(e) => setName(e.target.value)}
                    />
                </label>
                <div className={am('field')}>
                    <span>{colorLabel}</span>
                    <div
                        className={am('swatches')}
                        role='radiogroup'
                        aria-label={colorLabel}
                    >
                        <button
                            type='button'
                            role='radio'
                            aria-checked={!color}
                            aria-label={formatMessage({id: 'fusion.rail.folderColorTheme', defaultMessage: 'Theme colour'})}
                            className={am('swatch', 'theme', {on: !color})}
                            onClick={() => setColor('')}
                        />
                        {FOLDER_COLORS.map((c) => (
                            <button
                                key={c}
                                type='button'
                                role='radio'
                                aria-checked={color === c}
                                aria-label={c}
                                className={am('swatch', {on: color === c})}
                                style={{background: c}}
                                onClick={() => setColor(c)}
                            />
                        ))}
                    </div>
                </div>
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

// folderStyle gives a folder its colour, for the rail's styles.
function folderStyle(folder: Folder): React.CSSProperties | undefined {
    return folder.color ? {'--am-folder': folder.color} as React.CSSProperties : undefined;
}

// FolderButton is a closed or open server folder: its teams' faces in a square when closed, a folder when open, with
// their unread and mention counts. Teams dropped on it join it. Its name shows on hover, as the bubbles of the direct
// messages do.
function FolderButton({folder, teams, open, unread, mentions, onToggle, onDropTeam, onSettings, onMarkRead, onUngroup}: {folder: Folder; teams: Team[]; open: boolean; unread: boolean; mentions: number; onToggle: () => void; onDropTeam: () => void; onSettings: () => void; onMarkRead?: () => void; onUngroup: () => void}) {
    const {formatMessage} = useIntl();
    const [menu, setMenu] = useState<{x: number; y: number} | null>(null);
    const [over, setOver] = useState(false);
    const name = folderLabel(folder, teams);
    return (
        <>
            <button
                className={am('srv', 'folder', {open, unread: unread && !open, 'drop-in': over})}
                style={folderStyle(folder)}
                data-am-tip={name}
                data-am-tip-side='right'
                aria-label={formatMessage({id: 'fusion.rail.folderLabel', defaultMessage: 'Folder {name}, {count, plural, one {# community} other {# communities}}'}, {name, count: teams.length})}
                aria-expanded={open}
                onClick={onToggle}
                onContextMenu={(e) => {
                    e.preventDefault();
                    setMenu(menuPoint(e));
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
                    {onMarkRead && (
                        <MenuItem
                            icon='check'
                            label={formatMessage({id: 'fusion.rail.folderMarkRead', defaultMessage: 'Mark folder as read'})}
                            onClick={() => {
                                setMenu(null);
                                onMarkRead();
                            }}
                        />
                    )}
                    <MenuItem
                        icon='pen'
                        label={formatMessage({id: 'fusion.rail.folderSettings', defaultMessage: 'Folder settings'})}
                        sub={formatMessage({id: 'fusion.rail.folderSettingsSub', defaultMessage: 'Name and colour'})}
                        onClick={() => {
                            setMenu(null);
                            onSettings();
                        }}
                    />
                    <MenuItem
                        icon='x'
                        label={formatMessage({id: 'fusion.rail.ungroup', defaultMessage: 'Remove folder'})}
                        sub={formatMessage({id: 'fusion.rail.ungroupSub', defaultMessage: 'Its communities stay in the rail'})}
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

// ServerRail is the column of teams on the far left; the mockup calls teams servers. The direct messages sit at its
// bottom, above search (see RailDirectMessages).
export default function ServerRail() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const layout = useLayout();
    const globalSearch = useGlobalSearch();
    const teams = useSelector(getSortedMyTeams);
    const currentTeamId = useSelector(getCurrentTeamId);
    const [allUnreadTeams, allMentionsInTeam] = useSelector(getTeamsUnreadStatuses);

    // No unread marks while your unread badges hide (focus mode).
    const hideUnreads = useHideUnreads();
    const unreadTeams = hideUnreads ? NO_TEAMS : allUnreadTeams;
    const mentionsInTeam = hideUnreads ? NO_MENTIONS : allMentionsInTeam;
    const currentChannelId = useSelector((state: GlobalState) => state.entities.channels.currentChannelId);
    const store = useStore<GlobalState>();

    // Direct messages belong to no team: while you're in them, no team is selected, as in Discord.
    const inDirectMessages = useSelector((state: GlobalState) => {
        const type = state.entities.channels.channels[currentChannelId]?.type;
        return type === Constants.DM_CHANNEL || type === Constants.GM_CHANNEL;
    }) || layout.home;
    const canJoin = useSelector((state: GlobalState) => getJoinableTeamIds(state).length > 0);
    const canCreate = useSelector((state: GlobalState) => haveISystemPermission(state, {permission: Permissions.CREATE_TEAM}));
    const dialogs = useDialogs();

    const [folders, saveFolders] = useFolders();
    const [openFolders, setOpenFolders] = useState<Record<string, boolean>>({});
    const [editing, setEditing] = useState<Folder | null>(null);
    const [adding, setAdding] = useState(false);
    const addRef = useRef<HTMLButtonElement>(null);
    const folderOpen = (id: string) => openFolders[id] ?? isFolderOpen(id);
    const toggleFolder = (id: string) => {
        const open = !folderOpen(id);
        setFolderOpen(id, open);
        setOpenFolders({...openFolders, [id]: open});
    };

    // The team being dragged: dropped before or after another team, it moves there (the order is the user's
    // preference); dropped on it, the two make a folder, as in Discord.
    const dragged = useRef<string | null>(null);
    const order = teams.map((t) => t.id);
    const lastOf = (folder: Folder, except: string) => order.filter((id) => id !== except && folder.teamIds.includes(id)).pop();
    const dropOn = (target: Team, zone: DropZone) => {
        const id = dragged.current;
        dragged.current = null;
        if (!id || id === target.id) {
            return;
        }
        const targetFolder = folderOfTeam(teams, folders, target.id);
        const ownFolder = folderOfTeam(teams, folders, id);
        if (zone === 'into') {
            if (targetFolder && targetFolder.id === ownFolder?.id) {
                return;
            }

            // The new folder stays where the team dropped on was, the dragged team after it (or after the last of
            // the folder's teams).
            const next = makeFolder(folders, target.id, id);
            const folder = next.find((f) => f.teamIds.includes(id));
            const after = folder ? lastOf(folder, id) : target.id;
            dispatch(updateTeamsOrderForUser(placeAfter(order, id, after || target.id)));
            saveFolders(next);
            return;
        }
        const rest = order.filter((t) => t !== id);
        rest.splice(rest.indexOf(target.id) + (zone === 'after' ? 1 : 0), 0, id);
        dispatch(updateTeamsOrderForUser(rest));

        // Dropped among a folder's teams, a team joins the folder; among loose teams, it leaves its folder.
        if (targetFolder?.id !== ownFolder?.id) {
            saveFolders(targetFolder ? addToFolder(folders, id, targetFolder.id) : removeFromFolder(folders, id));
        }
    };
    const dropInFolder = (folder: Folder) => {
        const id = dragged.current;
        dragged.current = null;
        if (!id || folderOfTeam(teams, folders, id)?.id === folder.id) {
            return;
        }
        const after = lastOf(folder, id);
        if (after) {
            dispatch(updateTeamsOrderForUser(placeAfter(order, id, after)));
        }
        saveFolders(addToFolder(folders, id, folder.id));
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
            {...a11yRegion(am('servers'), 'rail')}
            aria-label={formatMessage({id: 'fusion.rail.label', defaultMessage: 'Communities'})}
        >
            {showHistoryButtons() && <HistoryButtons/>}
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
                            onDrop: (zone) => dropOn(team, zone),
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
                        style={folderStyle(item.folder)}
                        role='group'
                        aria-label={folderLabel(item.folder, item.teams)}
                    >
                        <FolderButton
                            folder={item.folder}
                            teams={item.teams}
                            open={open}
                            unread={item.teams.some((t) => unreadTeams.has(t.id))}
                            mentions={item.teams.reduce((n, t) => n + (mentionsInTeam.get(t.id) || 0), 0)}
                            onToggle={() => toggleFolder(item.folder.id)}
                            onDropTeam={() => dropInFolder(item.folder)}
                            onSettings={() => setEditing(item.folder)}
                            onMarkRead={item.teams.some((t) => unreadTeams.has(t.id)) ? () => item.teams.forEach((t) => dispatch(markTeamAsRead(t.id))) : undefined}
                            onUngroup={() => saveFolders(folders.filter((f) => f.id !== item.folder.id))}
                        />
                        {open && item.teams.map(teamButton)}
                    </div>
                );
            })}
            {editing && (
                <FolderDialog
                    folder={editing}
                    onSave={(name, color) => saveFolders(folders.map((f) => (f.id === editing.id ? {...f, name, color: color || undefined} : f)))}
                    onClose={() => setEditing(null)}
                />
            )}
            {(canJoin || canCreate) && (
                <button
                    ref={addRef}
                    className={am('srv', 'add', {active: adding})}
                    data-am-tip={formatMessage({id: 'fusion.rail.add', defaultMessage: 'Add a community'})}
                    data-am-tip-side='right'
                    aria-label={formatMessage({id: 'fusion.rail.add', defaultMessage: 'Add a community'})}
                    aria-haspopup={canJoin && canCreate ? 'menu' : undefined}
                    aria-expanded={canJoin && canCreate ? adding : undefined}
                    onClick={() => {
                        // With only one way to add a community, there's nothing to choose: skip the one-item menu
                        // and go straight to it.
                        if (canCreate && !canJoin) {
                            dialogs.createTeam();
                        } else if (canJoin && !canCreate) {
                            getHistory().push('/select_team');
                        } else {
                            setAdding(!adding);
                        }
                    }}
                >
                    <Icon name='plus'/>
                </button>
            )}
            {canJoin && canCreate && adding && (
                <Popover
                    anchor={addRef.current}
                    placement='beside'
                    className='plus-pop'
                    role='menu'
                    label={formatMessage({id: 'fusion.rail.add', defaultMessage: 'Add a community'})}
                    onClose={() => setAdding(false)}
                >
                    <MenuItem
                        icon='compass'
                        label={formatMessage({id: 'fusion.rail.join', defaultMessage: 'Join another community'})}
                        sub={formatMessage({id: 'fusion.rail.joinSub', defaultMessage: 'Browse the communities open to you'})}
                        onClick={() => {
                            setAdding(false);
                            getHistory().push('/select_team');
                        }}
                    />
                    <MenuItem
                        icon='plus'
                        label={formatMessage({id: 'fusion.rail.create', defaultMessage: 'Create a community'})}
                        sub={formatMessage({id: 'fusion.rail.createSub', defaultMessage: 'A new space with its own channels'})}
                        onClick={() => {
                            setAdding(false);
                            dialogs.createTeam();
                        }}
                    />
                </Popover>
            )}
            <span className={am('rail-grow')}/>
            <div className={am('rail-plugins')}>
                <Pluggable pluggableName='BottomTeamSidebar'/>
            </div>
            <RailDirectMessages/>
            <button
                className={am('srv', 'rail-search')}
                data-am-tip={formatMessage({id: 'fusion.rail.search', defaultMessage: 'Search everything (Ctrl K)'})}
                data-am-tip-side='right'
                aria-label={formatMessage({id: 'fusion.rail.searchLabel', defaultMessage: 'Search everything'})}
                onClick={globalSearch.open}
            >
                <Icon name='search'/>
            </button>
        </nav>
    );
}
