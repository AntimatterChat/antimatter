// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Team} from '@mattermost/types/teams';

import {addToFolder, parseFolders, railItems, removeFromFolder} from './folders';

const team = (id: string) => ({id, display_name: id}) as Team;
const teams = ['a', 'b', 'c', 'd'].map(team);

describe('server folders', () => {
    test('a folder sits where its first team is, with its teams in rail order', () => {
        const items = railItems(teams, [{id: 'f', name: 'Work', teamIds: ['d', 'b']}]);
        expect(items.map((i) => (i.kind === 'team' ? i.team.id : `[${i.teams.map((t) => t.id).join('')}]`))).toEqual(['a', '[bd]', 'c']);
    });

    test('a folder without any of your teams is not shown', () => {
        expect(railItems(teams, [{id: 'f', name: '', teamIds: ['gone']}]).map((i) => i.kind)).toEqual(['team', 'team', 'team', 'team']);
    });

    test('adding moves a team between folders, and an emptied folder goes', () => {
        let folders = addToFolder([], 'a', '', 'Work');
        expect(folders).toHaveLength(1);
        expect(folders[0]).toMatchObject({name: 'Work', teamIds: ['a']});
        folders = addToFolder(folders, 'b', folders[0].id);
        expect(folders[0].teamIds).toEqual(['a', 'b']);
        folders = addToFolder(folders, 'a', '', 'Play');
        expect(folders.map((f) => [f.name, f.teamIds])).toEqual([['Work', ['b']], ['Play', ['a']]]);
        expect(removeFromFolder(folders, 'b').map((f) => f.name)).toEqual(['Play']);
    });

    test('bad saved folders are ignored', () => {
        expect(parseFolders('nope')).toEqual([]);
        expect(parseFolders('[{"id":"f","teamIds":["a"]},{"id":3}]')).toHaveLength(1);
    });
});
