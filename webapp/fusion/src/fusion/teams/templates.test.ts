// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {saveServerCategories} from 'mattermost-redux/actions/channel_categories';
import {createChannel} from 'mattermost-redux/actions/channels';
import {createGroupWithUserIds, patchGroup} from 'mattermost-redux/actions/groups';
import {patchTeam} from 'mattermost-redux/actions/teams';
import {Client4} from 'mattermost-redux/client';
import initialState from 'mattermost-redux/store/initial_state';

import type {GlobalState} from 'types/store';

import {applyTemplate, isTeamSetUp, parseTemplate, PRESETS} from './templates';

import mergeObjects from '../../packages/mattermost-redux/test/merge_objects';

jest.mock('mattermost-redux/actions/channel_categories', () => ({
    ...jest.requireActual('mattermost-redux/actions/channel_categories'),
    saveServerCategories: jest.fn(() => ({type: 'SAVED', data: []})),
}));
jest.mock('mattermost-redux/actions/channels', () => ({
    ...jest.requireActual('mattermost-redux/actions/channels'),
    createChannel: jest.fn(),
    patchChannel: jest.fn(() => ({type: 'PATCHED', data: {id: 'x', team_id: 'team'}})),
}));
jest.mock('mattermost-redux/actions/teams', () => ({
    ...jest.requireActual('mattermost-redux/actions/teams'),
    patchTeam: jest.fn(() => ({type: 'PATCHED_TEAM', data: {}})),
}));
jest.mock('mattermost-redux/actions/groups', () => ({
    ...jest.requireActual('mattermost-redux/actions/groups'),
    createGroupWithUserIds: jest.fn(),
    patchGroup: jest.fn(),
    getGroups: jest.fn(() => ({type: 'GROUPS'})),
}));

describe('parseTemplate', () => {
    test('reads a template', () => {
        const json = JSON.stringify(PRESETS.hybrid);
        expect(parseTemplate(json)).toMatchObject(PRESETS.hybrid);
    });

    test('keeps the well-formed part of each entry, and drops the unusable ones', () => {
        const parsed = parseTemplate(JSON.stringify({
            version: 1,
            server_categories: [{name: ' Info ', locked: 'yes', channels: ['a', 3]}, {locked: true}, null],
            channels: [{name: 'general', display_name: 'General', type: 'P'}, {name: 'Bad Name!', display_name: 'x'}, {name: 'nodisplay'}],
            roles: [{display_name: 'Mods', color: 'orange', hoist: 1, permissions: 'all'}, {color: '#ffffff'}],
            onboarding: 42,
        }));
        expect(parsed).toEqual({
            version: 1,
            server_categories: [{name: 'Info', locked: false, channels: ['a']}],
            channels: [{name: 'general', display_name: 'General', type: 'P', purpose: undefined, header: undefined}],
            roles: [{display_name: 'Mods', color: undefined, hoist: false, self_assignable: false, permissions: undefined}],
            onboarding: undefined,
        });
    });

    test('keeps a channel\'s known preset, and drops one that isn\'t (Antimatter channel types)', () => {
        const parsed = parseTemplate(JSON.stringify({
            version: 1,
            server_categories: [],
            channels: [
                {name: 'questions', display_name: 'Questions', type: 'O', preset: 'forum_qa'},
                {name: 'random', display_name: 'Random', type: 'O', preset: 'not-a-preset'},
            ],
            roles: [],
        }));
        expect(parsed?.channels).toEqual([
            {name: 'questions', display_name: 'Questions', type: 'O', purpose: undefined, header: undefined, preset: 'forum_qa'},
            {name: 'random', display_name: 'Random', type: 'O', purpose: undefined, header: undefined, preset: undefined},
        ]);
    });

    test('reads town-square, the default channel\'s former name, in older templates as general', () => {
        const parsed = parseTemplate(JSON.stringify({
            version: 1,
            server_categories: [{name: 'Information', locked: true, channels: ['town-square', 'rules']}],
            channels: [{name: 'town-square', display_name: 'Town Square', type: 'O'}, {name: 'rules', display_name: 'Rules', type: 'O'}],
            roles: [],
        }));
        expect(parsed?.server_categories).toEqual([{name: 'Information', locked: true, channels: ['general', 'rules']}]);
        expect(parsed?.channels.map((c) => c.name)).toEqual(['general', 'rules']);
    });

    test('the presets file Welcome under Information, and General, where people land, with the channels', () => {
        expect(PRESETS.hybrid.server_categories[0].channels).toEqual(['welcome']);
        expect(PRESETS.guild.server_categories[0].channels).toEqual(['welcome']);
        expect(PRESETS.hybrid.server_categories[1].channels).toContain('general');
        expect(PRESETS.guild.server_categories[1].channels).toContain('general');
    });

    test('reads the kind, its former names too, and drops one it does not know', () => {
        expect(parseTemplate(JSON.stringify({...PRESETS.workspace, kind: 'workspace'}))?.kind).toBe('workspace');
        expect(parseTemplate(JSON.stringify({...PRESETS.workspace, kind: 'workplace'}))?.kind).toBe('workspace');
        expect(parseTemplate(JSON.stringify({...PRESETS.guild, kind: 'community'}))?.kind).toBe('guild');
        expect(parseTemplate(JSON.stringify({...PRESETS.workspace, kind: 'castle'}))?.kind).toBeUndefined();
    });

    test('refuses what is not a template', () => {
        expect(parseTemplate('nope')).toBeUndefined();
        expect(parseTemplate('{}')).toBeUndefined();
        expect(parseTemplate('{"version":2,"server_categories":[],"channels":[],"roles":[]}')).toBeUndefined();
        expect(parseTemplate('{"version":1,"server_categories":{},"channels":[],"roles":[]}')).toBeUndefined();
    });
});

describe('PRESETS', () => {
    test('hybrid has a locked category and an open one, and a hoisted Moderators role', () => {
        const {server_categories: categories, roles} = PRESETS.hybrid;
        expect(categories.map((c) => c.locked)).toEqual([true, false]);
        expect(roles).toHaveLength(1);
        expect(roles[0]).toMatchObject({display_name: 'Moderators', hoist: true});
        expect(roles[0].permissions).toContain('delete_others_posts');
    });

    test('the moderators of the presets may time out and remove people, and manage messages', () => {
        for (const preset of [PRESETS.hybrid, PRESETS.guild]) {
            const permissions = preset.roles[0].permissions;
            expect(permissions).toContain('remove_user_from_team');
            expect(permissions).toContain('delete_others_posts');
            expect(permissions).not.toContain('manage_team');
            expect(permissions).not.toContain('delete_public_channel');
        }
    });

    test('hybrid and guild make guilds; workspace, a workspace', () => {
        expect(PRESETS.hybrid.kind).toBe('guild');
        expect(PRESETS.guild.kind).toBe('guild');
        expect(PRESETS.workspace.kind).toBe('workspace');
    });

    test('guild keeps every category locked', () => {
        expect(PRESETS.guild.server_categories.every((c) => c.locked)).toBe(true);
    });

    test('workspace has no categories for everyone', () => {
        expect(PRESETS.workspace.server_categories).toEqual([]);
    });

    test('no category is listed twice and no channel is filed twice', () => {
        for (const preset of Object.values(PRESETS)) {
            const names = preset.server_categories.map((c) => c.name.toLowerCase());
            expect(new Set(names).size).toBe(names.length);
            const channels = preset.server_categories.flatMap((c) => c.channels);
            expect(new Set(channels).size).toBe(channels.length);
        }
    });
});

describe('applyTemplate', () => {
    const state = mergeObjects(initialState, {
        entities: {
            users: {currentUserId: 'me'},
            channelCategories: {serverCategories: {team: [{name: 'Old', locked: false}]}},
            teams: {teams: {team: {id: 'team'}}},
        },
    }) as unknown as GlobalState;
    const run = async (template: Parameters<typeof applyTemplate>[1]) => {
        const dispatch: jest.Mock = jest.fn((a: unknown): unknown => (typeof a === 'function' ? a(dispatch, () => state) : a));
        return applyTemplate('team', template)(dispatch as never, () => state, undefined);
    };

    beforeEach(() => {
        (createChannel as jest.Mock).mockReset();
        (createGroupWithUserIds as jest.Mock).mockReset();
        (patchGroup as jest.Mock).mockReset();
        (saveServerCategories as jest.Mock).mockClear();
        jest.spyOn(Client4, 'updateTeamRolePermissions').mockResolvedValue({} as never);
    });

    test('goes on past what fails, and says how much did', async () => {
        (createChannel as jest.Mock).mockReturnValue({type: 'FAILED', error: {message: 'nope'}});
        (createGroupWithUserIds as jest.Mock).mockReturnValue({type: 'CREATED', data: {id: 'role'}});
        const result = await run({version: 1, server_categories: [{name: 'Info', locked: true, channels: []}], channels: [{name: 'news', display_name: 'News', type: 'O'}], roles: [{display_name: 'Mods'}]});
        expect(result).toEqual({data: {failed: 1}});
        expect(createGroupWithUserIds).toHaveBeenCalled();
        expect(saveServerCategories).toHaveBeenCalledWith('team', {categories: [{name: 'Info', locked: true}], removed: ['Old']});
    });

    test('a channel\'s preset is sent as props.am_preset; one without a preset gets no props (Antimatter channel types)', async () => {
        (createChannel as jest.Mock).mockImplementation((channel) => ({type: 'CREATED', data: {...channel, id: channel.name}}));
        await run({
            version: 1,
            server_categories: [],
            channels: [
                {name: 'questions', display_name: 'Questions', type: 'O', preset: 'forum_qa'},
                {name: 'general-talk', display_name: 'General talk', type: 'O'},
            ],
            roles: [],
        });
        expect(createChannel).toHaveBeenCalledWith(expect.objectContaining({name: 'questions', props: {am_preset: 'forum_qa'}}), '');
        expect(createChannel).toHaveBeenCalledWith(expect.objectContaining({name: 'general-talk'}), '');
        const plainCall = (createChannel as jest.Mock).mock.calls.find((args) => args[0].name === 'general-talk');
        expect(plainCall[0].props).toBeUndefined();
    });

    test('sets the kind of community the template makes', async () => {
        (patchTeam as jest.Mock).mockClear();
        await run({version: 1, server_categories: [], channels: [], roles: [], kind: 'workspace'});
        expect(patchTeam).toHaveBeenCalledWith({id: 'team', kind: 'workspace'});

        (patchTeam as jest.Mock).mockClear();
        await run({version: 1, server_categories: [], channels: [], roles: []});
        expect(patchTeam).not.toHaveBeenCalled();
    });

    test('a role without permissions keeps those it has', async () => {
        (createGroupWithUserIds as jest.Mock).mockReturnValue({type: 'CREATED', data: {id: 'role'}});
        await run({version: 1, server_categories: [], channels: [], roles: [{display_name: 'Mods'}]});
        expect(Client4.updateTeamRolePermissions).not.toHaveBeenCalled();
    });
});

describe('isTeamSetUp', () => {
    const role = (id: string) => ({id, team_id: 'team', source: 'custom', delete_at: 0, is_team: false, display_name: id, position: 0});
    const state = (roles: string[], defaultRoleId = '') => mergeObjects(initialState, {
        entities: {
            teams: {teams: {team: {id: 'team', default_role_id: defaultRoleId}}},
            groups: {groups: Object.fromEntries(roles.map((id) => [id, role(id)]))},
            channelCategories: {serverCategories: {team: []}},
        },
    }) as unknown as GlobalState;

    test('a team with roles is set up, its default role alone doesn\'t count', () => {
        expect(isTeamSetUp(state([]), 'team')).toBe(false);
        expect(isTeamSetUp(state(['mods']), 'team')).toBe(true);
        expect(isTeamSetUp(state(['members'], 'members'), 'team')).toBe(false);
        expect(isTeamSetUp(state(['members', 'mods'], 'members'), 'team')).toBe(true);
    });
});
