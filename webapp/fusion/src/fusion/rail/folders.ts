// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useDispatch, useSelector} from 'react-redux';

import type {Team} from '@mattermost/types/teams';

import {savePreferences} from 'mattermost-redux/actions/preferences';
import {get as getPreference} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';
import {generateId} from 'mattermost-redux/utils/helpers';

import type {GlobalState} from 'types/store';

// Server folders group teams in the rail, as Discord's: a personal preference, kept on the server so it follows you
// across devices, the teams keeping their order. A folder holds two teams or more: dropping a team on another makes
// one, and a folder left with a single team is undone.
export const CATEGORY_RAIL = 'fusion_rail';
export const NAME_FOLDERS = 'folders';

export type Folder = {
    id: string;
    name: string;

    /** A colour of FOLDER_COLORS, as #rrggbb; none for the theme's. */
    color?: string;
    teamIds: string[];
};

// The colours a folder can take, as Discord's: the theme's accent first (none).
export const FOLDER_COLORS = ['#5865f2', '#3ba55c', '#faa61a', '#ed4245', '#eb459e', '#9b59b6', '#1abc9c', '#99aab5'];

export type RailItem = {kind: 'team'; team: Team} | {kind: 'folder'; folder: Folder; teams: Team[]};

const COLOR = /^#[0-9a-f]{6}$/i;

export function parseFolders(value: string): Folder[] {
    try {
        const folders = JSON.parse(value);
        return Array.isArray(folders) ? folders.filter((f) => f && typeof f.id === 'string' && Array.isArray(f.teamIds)).map((f) => ({
            id: f.id,
            name: typeof f.name === 'string' ? f.name : '',
            ...(typeof f.color === 'string' && COLOR.test(f.color) ? {color: f.color} : {}),
            teamIds: f.teamIds.filter((id: unknown) => typeof id === 'string'),
        })) : [];
    } catch {
        return [];
    }
}

// railItems lays out the rail: each folder where its first team is, holding its teams in the rail's order. A folder
// with fewer than two teams of yours isn't one: its team stands alone.
export function railItems(teams: Team[], folders: Folder[]): RailItem[] {
    const mine = new Set(teams.map((t) => t.id));
    const folderOf = new Map<string, Folder>();
    for (const folder of folders) {
        if (folder.teamIds.filter((id) => mine.has(id)).length < 2) {
            continue;
        }
        for (const id of folder.teamIds) {
            if (!folderOf.has(id)) {
                folderOf.set(id, folder);
            }
        }
    }
    const items: RailItem[] = [];
    const placed = new Set<string>();
    for (const team of teams) {
        const folder = folderOf.get(team.id);
        if (!folder) {
            items.push({kind: 'team', team});
        } else if (!placed.has(folder.id)) {
            placed.add(folder.id);
            items.push({kind: 'folder', folder, teams: teams.filter((t) => folderOf.get(t.id) === folder)});
        }
    }
    return items;
}

// folderOfTeam gives the folder a team is shown in, if any.
export function folderOfTeam(teams: Team[], folders: Folder[], teamId: string): Folder | undefined {
    for (const item of railItems(teams, folders)) {
        if (item.kind === 'folder' && item.teams.some((t) => t.id === teamId)) {
            return item.folder;
        }
    }
    return undefined;
}

// addToFolder puts a team in a folder, taking it out of any other.
export function addToFolder(folders: Folder[], teamId: string, folderId: string): Folder[] {
    return removeFromFolder(folders, teamId).map((f) => (f.id === folderId ? {...f, teamIds: [...f.teamIds, teamId]} : f));
}

// makeFolder puts a team dropped on another together in a new folder, the other first; or in the other's folder.
export function makeFolder(folders: Folder[], targetId: string, teamId: string, name = ''): Folder[] {
    if (targetId === teamId) {
        return folders;
    }
    const target = folders.find((f) => f.teamIds.includes(targetId));
    if (target) {
        return target.teamIds.includes(teamId) ? folders : addToFolder(folders, teamId, target.id);
    }
    return [...removeFromFolder(folders, teamId), {id: generateId(), name, teamIds: [targetId, teamId]}];
}

// removeFromFolder takes a team out of its folder; a folder left with a single team goes.
export function removeFromFolder(folders: Folder[], teamId: string): Folder[] {
    return folders.map((f) => ({...f, teamIds: f.teamIds.filter((id) => id !== teamId)})).filter((f) => f.teamIds.length > 1);
}

// placeAfter moves a team right after another in the rail's order (after the last of a folder's teams to join it).
export function placeAfter(order: string[], teamId: string, afterId: string): string[] {
    const rest = order.filter((id) => id !== teamId);
    const at = rest.indexOf(afterId);
    if (at < 0) {
        return order;
    }
    rest.splice(at + 1, 0, teamId);
    return rest;
}

export function getFolders(state: GlobalState): Folder[] {
    return parseFolders(getPreference(state, CATEGORY_RAIL, NAME_FOLDERS, '[]'));
}

// useFolders gives the folders, and saves them.
export function useFolders(): [Folder[], (folders: Folder[]) => void] {
    const dispatch = useDispatch();
    const userId = useSelector(getCurrentUserId);
    const value = useSelector((state: GlobalState) => getPreference(state, CATEGORY_RAIL, NAME_FOLDERS, '[]'));
    const save = (folders: Folder[]) => dispatch(savePreferences(userId, [{user_id: userId, category: CATEGORY_RAIL, name: NAME_FOLDERS, value: JSON.stringify(folders)}]));
    return [parseFolders(value), save];
}

// Whether a folder is open is remembered in this browser only.
const OPEN_KEY = 'am-rail-open-folders';

export function isFolderOpen(id: string): boolean {
    try {
        return (JSON.parse(localStorage.getItem(OPEN_KEY) || '[]') as string[]).includes(id);
    } catch {
        return false;
    }
}

export function setFolderOpen(id: string, open: boolean) {
    try {
        const ids = (JSON.parse(localStorage.getItem(OPEN_KEY) || '[]') as string[]).filter((x) => x !== id);
        localStorage.setItem(OPEN_KEY, JSON.stringify(open ? [...ids, id] : ids));
    } catch {
        // Private browsing: the folder just starts closed next time.
    }
}
