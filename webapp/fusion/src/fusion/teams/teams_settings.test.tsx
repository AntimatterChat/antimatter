// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import React from 'react';

import type {Group} from '@mattermost/types/groups';

import {addUsersToGroup, archiveGroup, createGroupWithUserIds, getGroups, patchGroup, removeUsersFromGroup, restoreGroup} from 'mattermost-redux/actions/groups';
import {patchTeam} from 'mattermost-redux/actions/teams';
import {Client4} from 'mattermost-redux/client';

import {renderWithContext, screen, userEvent, waitFor, within} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import {mentionFromName} from './team_editor';
import TeamsSettings, {teamsShown} from './teams_settings';

jest.mock('mattermost-redux/actions/groups', () => ({
    ...jest.requireActual('mattermost-redux/actions/groups'),
    archiveGroup: jest.fn(() => ({type: 'MOCK_ARCHIVE', data: {}})),
    restoreGroup: jest.fn(() => ({type: 'MOCK_RESTORE', data: {}})),
    getGroups: jest.fn(() => ({type: 'MOCK_GET_GROUPS', data: []})),
    createGroupWithUserIds: jest.fn(() => ({type: 'MOCK_CREATE', data: {id: 'new'}})),
    patchGroup: jest.fn(() => ({type: 'MOCK_PATCH', data: {}})),
    addUsersToGroup: jest.fn(() => ({type: 'MOCK_ADD', data: []})),
    removeUsersFromGroup: jest.fn(() => ({type: 'MOCK_REMOVE', data: []})),
}));
jest.mock('mattermost-redux/actions/teams', () => ({
    ...jest.requireActual('mattermost-redux/actions/teams'),
    patchTeam: jest.fn(() => ({type: 'MOCK_PATCH_TEAM'})),
}));
jest.mock('mattermost-redux/actions/users', () => ({
    ...jest.requireActual('mattermost-redux/actions/users'),
    getMissingProfilesByIds: jest.fn(() => ({type: 'MOCK_PROFILES'})),
}));
jest.mock('./teams', () => ({
    ...jest.requireActual('./teams'),
    loadTeams: jest.fn(() => ({type: 'MOCK_LOAD_TEAMS'})),
}));
const mockToast = jest.fn();
jest.mock('fusion/shell/toast_context', () => ({useToast: () => mockToast}));

const team = (id: string, name: string, extra: Partial<Group> = {}) => ({
    id, name: name.toLowerCase(), display_name: name, tag: name.slice(0, 3).toUpperCase(), source: 'custom', is_team: true,
    team_id: 'c1', delete_at: 0, member_ids: ['me', 'ann'], lead_id: '', channel_id: '', description: '', ...extra,
}) as unknown as Group;

const design = team('design', 'Design');
const ops = team('ops', 'Ops', {member_ids: ['ann']});
const old = team('old', 'Old', {delete_at: 5});
const elsewhere = team('else', 'Elsewhere', {team_id: 'c2'});

function state(permissions = ['manage_team', 'manage_team_roles'], badges = false) {
    return {
        entities: {
            general: {config: {EnableCustomGroups: 'true'}},
            users: {
                currentUserId: 'me',
                profiles: {me: TestHelper.getUserMock({id: 'me', username: 'me', roles: 'system_user'}), ann: TestHelper.getUserMock({id: 'ann', username: 'ann'})},
            },
            teams: {
                currentTeamId: 'c1',
                teams: {c1: TestHelper.getTeamMock({id: 'c1', name: 'lumen', require_team_badges: badges})},
                myMembers: {c1: {team_id: 'c1', user_id: 'me', roles: 'team_user'}},
            },
            groups: {groups: {design, ops, old, else: elsewhere}},
            roles: {roles: {system_user: {name: 'system_user', permissions: []}, team_user: {name: 'team_user', permissions}}},
            channels: {channels: {}, myMembers: {}},
        },
    } as never;
}

const names = () => screen.getAllByRole('button', {name: /members/}).map((b) => within(b).getByText(/^(Design|Ops|Old)$/).textContent);

describe('teamsShown', () => {
    test('all those not archived, yours, or the archived ones, matching a search', () => {
        const teams = [design, ops, old];
        expect(teamsShown(teams, 'all', 'me', '').map((t) => t.id)).toEqual(['design', 'ops']);
        expect(teamsShown(teams, 'mine', 'me', '').map((t) => t.id)).toEqual(['design']);
        expect(teamsShown(teams, 'archived', 'me', '').map((t) => t.id)).toEqual(['old']);
        expect(teamsShown(teams, 'all', 'me', 'op').map((t) => t.id)).toEqual(['ops']);
        expect(teamsShown(teams, 'all', 'me', 'DES').map((t) => t.id)).toEqual(['design']);
    });
});

describe('mentionFromName', () => {
    test('a team\'s name suggests its mention', () => {
        expect(mentionFromName('Détecteur Physics!')).toBe('detecteur-physics');
    });
});

describe('fusion/teams/TeamsSettings', () => {
    beforeEach(() => {
        jest.clearAllMocks();
        jest.spyOn(Client4, 'autocompleteUsers').mockResolvedValue({users: []} as never);
    });

    test('lists the community\'s teams, finds them, and shows the archived ones', async () => {
        renderWithContext(<TeamsSettings/>, state());
        expect(names()).toEqual(['Design', 'Ops']);

        await userEvent.click(screen.getByRole('button', {name: 'Yours'}));
        expect(names()).toEqual(['Design']);

        await userEvent.click(screen.getByRole('button', {name: 'Archived'}));
        expect(getGroups).toHaveBeenCalledWith(expect.objectContaining({filter_archived: true, in_team_id: 'c1', is_team: true}));
        expect(names()).toEqual(['Old']);
        await userEvent.click(screen.getByRole('button', {name: 'Restore'}));
        expect(restoreGroup).toHaveBeenCalledWith('old');

        await userEvent.click(screen.getByRole('button', {name: 'All'}));
        await userEvent.type(screen.getByRole('searchbox', {name: 'Find a team'}), 'ops');
        expect(names()).toEqual(['Ops']);
        expect(screen.queryByText('Elsewhere')).not.toBeInTheDocument();
    });

    test('the community\'s admins set whether everyone shows a team badge', async () => {
        renderWithContext(<TeamsSettings/>, state());
        await userEvent.click(screen.getByRole('switch', {name: 'Everyone shows a team badge'}));
        expect(patchTeam).toHaveBeenCalledWith({id: 'c1', require_team_badges: true});
    });

    test('creates a team with its name, mention and badge', async () => {
        renderWithContext(<TeamsSettings/>, state());
        await userEvent.click(screen.getByRole('button', {name: 'Create team'}));
        await userEvent.type(screen.getByRole('textbox', {name: 'Name'}), 'Detector team');
        expect(screen.getByRole('textbox', {name: /Mention/})).toHaveValue('detector-team');
        await userEvent.type(screen.getByRole('textbox', {name: /Badge/}), 'det');
        await userEvent.click(screen.getByRole('button', {name: 'Create team'}));
        await waitFor(() => expect(createGroupWithUserIds).toHaveBeenCalledWith(expect.objectContaining({
            display_name: 'Detector team', name: 'detector-team', tag: 'DET', team_id: 'c1', is_team: true, source: 'custom', allow_reference: true, user_ids: [],
        })));
    });

    test('edits a team: its name, lead and members, saved together', async () => {
        const setUnsaved = jest.fn();
        renderWithContext(<TeamsSettings setUnsaved={setUnsaved}/>, state());
        await userEvent.click(screen.getAllByRole('button', {name: 'Edit the team'})[0]);
        const name = screen.getByRole('textbox', {name: 'Name'});
        await userEvent.clear(name);
        await userEvent.type(name, 'Design studio');
        await userEvent.selectOptions(screen.getByRole('combobox', {name: 'Lead'}), 'ann');
        await userEvent.click(screen.getAllByRole('button', {name: 'Remove from the team'})[0]);
        await waitFor(() => expect(setUnsaved).toHaveBeenLastCalledWith(true));

        await userEvent.click(screen.getByRole('button', {name: 'Save'}));
        await waitFor(() => expect(removeUsersFromGroup).toHaveBeenCalledWith('design', ['me']));
        expect(addUsersToGroup).not.toHaveBeenCalled();
        expect(patchGroup).toHaveBeenCalledWith('design', {display_name: 'Design studio', lead_id: 'ann'});
        await waitFor(() => expect(setUnsaved).toHaveBeenLastCalledWith(false));
    });

    test('archives a team', async () => {
        renderWithContext(<TeamsSettings/>, state());
        await userEvent.click(screen.getAllByRole('button', {name: 'Archive the team'})[1]);
        expect(archiveGroup).toHaveBeenCalledWith('ops');
        await waitFor(() => expect(mockToast).toHaveBeenCalledWith('Archived Ops'));
    });

    test('those who may not manage teams only see and open them', () => {
        renderWithContext(<TeamsSettings/>, state([]));
        expect(names()).toEqual(['Design', 'Ops']);
        expect(screen.queryByRole('button', {name: 'Create team'})).not.toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Edit the team'})).not.toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Archive the team'})).not.toBeInTheDocument();
        expect(screen.queryByRole('switch', {name: 'Everyone shows a team badge'})).not.toBeInTheDocument();
    });
});
