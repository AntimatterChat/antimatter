// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useDispatch, useSelector} from 'react-redux';

import type {Team} from '@mattermost/types/teams';

import {savePreferences} from 'mattermost-redux/actions/preferences';
import {get as getPreference} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';
import {generateId} from 'mattermost-redux/utils/helpers';

import type {GlobalState} from 'types/store';

// Server folders group teams in the rail, as Discord's: a personal preference, the teams keeping their order.
export const CATEGORY_RAIL = 'fusion_rail';
export const NAME_FOLDERS = 'folders';

export type Folder = {
    id: string;
    name: string;
    teamIds: string[];
};

export type RailItem = {kind: 'team'; team: Team} | {kind: 'folder'; folder: Folder; teams: Team[]};

export function parseFolders(value: string): Folder[] {
    try {
        const folders = JSON.parse(value);
        return Array.isArray(folders) ? folders.filter((f) => f && typeof f.id === 'string' && Array.isArray(f.teamIds)) : [];
    } catch {
        return [];
    }
}

// railItems lays out the rail: each folder where its first team is, holding its teams in the rail's order. A folder
// left with no team of yours isn't shown.
export function railItems(teams: Team[], folders: Folder[]): RailItem[] {
    const folderOf = new Map<string, Folder>();
    for (const folder of folders) {
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

// addToFolder puts a team in a folder (a new one, with a name, when folderId is empty), taking it out of any other.
export function addToFolder(folders: Folder[], teamId: string, folderId: string, newName = ''): Folder[] {
    const rest = removeFromFolder(folders, teamId);
    if (!folderId) {
        return [...rest, {id: generateId(), name: newName, teamIds: [teamId]}];
    }
    return rest.map((f) => (f.id === folderId ? {...f, teamIds: [...f.teamIds, teamId]} : f));
}

// removeFromFolder takes a team out of its folder; an emptied folder goes.
export function removeFromFolder(folders: Folder[], teamId: string): Folder[] {
    return folders.map((f) => ({...f, teamIds: f.teamIds.filter((id) => id !== teamId)})).filter((f) => f.teamIds.length > 0);
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
