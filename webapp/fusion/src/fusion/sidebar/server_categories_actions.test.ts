// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {ServerCategory} from '@mattermost/types/channel_categories';

import {saveServerCategories} from 'mattermost-redux/actions/channel_categories';
import {haveIChannelPermission, haveITeamPermission} from 'mattermost-redux/selectors/entities/roles';

import type {GlobalState} from 'types/store';

import {addServerCategory, canFileChannel, deleteServerCategory, findServerCategory, moveServerCategory, renameServerCategory, setServerCategoryLocked} from './server_categories';

jest.mock('mattermost-redux/actions/channel_categories', () => ({
    ...jest.requireActual('mattermost-redux/actions/channel_categories'),
    saveServerCategories: jest.fn(() => ({type: 'SAVED'})),
}));

jest.mock('mattermost-redux/selectors/entities/roles', () => ({
    ...jest.requireActual('mattermost-redux/selectors/entities/roles'),
    haveITeamPermission: jest.fn(() => false),
    haveIChannelPermission: jest.fn(() => false),
}));

const save = saveServerCategories as jest.Mock;
const teamAdmin = haveITeamPermission as unknown as jest.Mock;
const channelAdmin = haveIChannelPermission as unknown as jest.Mock;

const list: ServerCategory[] = [{name: 'Rules', locked: true}, {name: 'Community', locked: false}] as ServerCategory[];

const stateWith = (mappings: Record<string, string> = {}) => ({
    entities: {
        general: {config: {FeatureFlagManagedChannelCategories: 'true'}},
        channelCategories: {serverCategories: {team: list}, managedCategoryMappings: {team: mappings}},
    },
}) as unknown as GlobalState;

// run dispatches an admin edit and gives the list it saves.
const run = async (action: ReturnType<typeof addServerCategory>, state = stateWith()) => {
    save.mockClear();
    const dispatch: jest.Mock = jest.fn((a: unknown): unknown => (typeof a === 'function' ? a(dispatch, () => state) : a));
    await action(dispatch as never, () => state, undefined);
    return save.mock.calls[0]?.[1];
};

describe('server category edits', () => {
    test('finds categories ignoring case and spaces', () => {
        expect(findServerCategory(list, '  community ')).toBe(list[1]);
        expect(findServerCategory(list, 'Archive')).toBeUndefined();
    });

    test('adding one puts it last', async () => {
        expect(await run(addServerCategory('team', ' Voice ', false))).toEqual({categories: [...list, {name: 'Voice', locked: false}]});
    });

    test('renaming one keeps its place and says what was renamed', async () => {
        expect(await run(renameServerCategory('team', 'community', 'Hangout '))).toEqual({
            categories: [{name: 'Rules', locked: true}, {name: 'Hangout', locked: false}],
            renames: {Community: 'Hangout'},
        });
        expect(await run(renameServerCategory('team', 'Legacy', 'Archive'))).toEqual({
            categories: [...list, {name: 'Archive', locked: false}],
            renames: {Legacy: 'Archive'},
        });
    });

    test('locking one changes only it', async () => {
        expect(await run(setServerCategoryLocked('team', 'COMMUNITY', true))).toEqual({categories: [{name: 'Rules', locked: true}, {name: 'Community', locked: true}]});
    });

    test('moving one swaps it with its neighbour, and not past the ends', async () => {
        expect(await run(moveServerCategory('team', 'Community', -1))).toEqual({categories: [list[1], list[0]]});
        expect(await run(moveServerCategory('team', 'Rules', -1))).toBeUndefined();
        expect(await run(moveServerCategory('team', 'Community', 1))).toBeUndefined();
        expect(await run(moveServerCategory('team', 'Unknown', 1))).toBeUndefined();
    });

    test('deleting one keeps the others, and says which went', async () => {
        expect(await run(deleteServerCategory('team', 'rules'))).toEqual({categories: [list[1]], removed: ['Rules']});
    });

    test('a category only filed under can be deleted too', async () => {
        expect(await run(deleteServerCategory('team', 'Legacy'))).toEqual({categories: list, removed: ['Legacy']});
    });
});

describe('canFileChannel', () => {
    const channel = {id: 'dev', team_id: 'team'};

    beforeEach(() => {
        teamAdmin.mockReturnValue(false);
        channelAdmin.mockReturnValue(true);
    });

    test('team admins file anywhere', () => {
        teamAdmin.mockReturnValue(true);
        channelAdmin.mockReturnValue(false);
        expect(canFileChannel(stateWith({dev: 'Rules'}), channel, '')).toBe(true);
        expect(canFileChannel(stateWith(), channel, 'Rules')).toBe(true);
    });

    test('channel admins move channels between open categories', () => {
        expect(canFileChannel(stateWith(), channel, 'community')).toBe(true);
        expect(canFileChannel(stateWith({dev: 'Community'}), channel, '')).toBe(true);
    });

    test('nor into nor out of a locked category', () => {
        expect(canFileChannel(stateWith(), channel, 'Rules')).toBe(false);
        expect(canFileChannel(stateWith({dev: 'Rules'}), channel, 'Community')).toBe(false);
    });

    test('nor into a category that does not exist', () => {
        expect(canFileChannel(stateWith(), channel, 'Archive')).toBe(false);
    });

    test('members who do not manage the channel do not file it', () => {
        channelAdmin.mockReturnValue(false);
        expect(canFileChannel(stateWith(), channel, 'Community')).toBe(false);
    });
});

describe('getSidebarCategories', () => {
    test('the same layout comes back while nothing changed, folded categories included', () => {
        const {getSidebarCategories} = jest.requireActual('./server_categories');
        const state = {
            entities: {
                general: {config: {FeatureFlagManagedChannelCategories: 'true'}},
                users: {currentUserId: 'me'},
                teams: {currentTeamId: 'team', myMembers: {}, teams: {team: {id: 'team'}}},
                channelCategories: {byId: {}, orderByTeam: {team: []}, serverCategories: {team: list}, managedCategoryMappings: {team: {}}},
                preferences: {myPreferences: {'fusion_server_collapsed--team': {category: 'fusion_server_collapsed', name: 'team', value: '["Rules"]'}}},
                channels: {channels: {}, myMembers: {}},
                roles: {roles: {}},
            },
        } as unknown as GlobalState;
        expect(getSidebarCategories(state)).toBe(getSidebarCategories(state));
    });
});
