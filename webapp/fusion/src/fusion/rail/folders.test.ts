// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Team} from '@mattermost/types/teams';

import {addToFolder, folderOfTeam, makeFolder, parseFolders, placeAfter, railItems, removeFromFolder} from './folders';
import type {RailItem} from './folders';

const team = (id: string) => ({id, display_name: id}) as Team;
const teams = ['a', 'b', 'c', 'd'].map(team);
const layout = (items: RailItem[]) => items.map((i) => (i.kind === 'team' ? i.team.id : `[${i.teams.map((t) => t.id).join('')}]`));

describe('server folders', () => {
    test('a folder sits where its first team is, with its teams in rail order', () => {
        expect(layout(railItems(teams, [{id: 'f', name: 'Work', teamIds: ['d', 'b']}]))).toEqual(['a', '[bd]', 'c']);
    });

    test('a folder with fewer than two of your teams is not shown', () => {
        expect(layout(railItems(teams, [{id: 'f', name: '', teamIds: ['gone']}]))).toEqual(['a', 'b', 'c', 'd']);
        expect(layout(railItems(teams, [{id: 'f', name: '', teamIds: ['b', 'gone']}]))).toEqual(['a', 'b', 'c', 'd']);
        expect(folderOfTeam(teams, [{id: 'f', name: '', teamIds: ['b', 'gone']}], 'b')).toBeUndefined();
    });

    test('dropping a team on another makes a folder of the two, the other first', () => {
        const folders = makeFolder([], 'b', 'd', 'Work');
        expect(folders).toHaveLength(1);
        expect(folders[0]).toMatchObject({name: 'Work', teamIds: ['b', 'd']});
        expect(makeFolder(folders, 'b', 'b')).toBe(folders);
    });

    test('dropping a team on one of a folder puts it in that folder', () => {
        let folders = makeFolder([], 'a', 'b');
        folders = makeFolder(folders, 'b', 'c');
        expect(folders).toHaveLength(1);
        expect(folders[0].teamIds).toEqual(['a', 'b', 'c']);
    });

    test('moving a team out of a folder of two undoes it', () => {
        let folders = makeFolder([], 'a', 'b');
        folders = makeFolder(folders, 'c', 'b');
        expect(folders.map((f) => f.teamIds)).toEqual([['c', 'b']]);
        expect(removeFromFolder(folders, 'b')).toEqual([]);
    });

    test('adding moves a team between folders', () => {
        let folders = [...makeFolder([], 'a', 'b'), ...makeFolder([], 'c', 'd')];
        folders = addToFolder(folders, 'd', folders[0].id);
        expect(folders.map((f) => f.teamIds)).toEqual([['a', 'b', 'd']]);
    });

    test('a team placed after another moves there in the rail order', () => {
        expect(placeAfter(['a', 'b', 'c', 'd'], 'a', 'c')).toEqual(['b', 'c', 'a', 'd']);
        expect(placeAfter(['a', 'b', 'c', 'd'], 'd', 'a')).toEqual(['a', 'd', 'b', 'c']);
        expect(placeAfter(['a', 'b'], 'a', 'gone')).toEqual(['a', 'b']);
    });

    test('bad saved folders are ignored, and colours are checked', () => {
        expect(parseFolders('nope')).toEqual([]);
        expect(parseFolders('[{"id":"f","teamIds":["a","b"]},{"id":3}]')).toHaveLength(1);
        const [f] = parseFolders('[{"id":"f","name":"Work","color":"#3ba55c","teamIds":["a","b"]}]');
        expect(f).toEqual({id: 'f', name: 'Work', color: '#3ba55c', teamIds: ['a', 'b']});
        expect(parseFolders('[{"id":"f","color":"red;","teamIds":["a","b"]}]')[0].color).toBeUndefined();
    });
});
