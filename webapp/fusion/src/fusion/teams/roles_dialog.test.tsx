// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Group} from '@mattermost/types/groups';
import type {TeamMemberListView} from '@mattermost/types/teams';

import {patchTeam} from 'mattermost-redux/actions/teams';
import {Client4} from 'mattermost-redux/client';

import {renderWithContext, screen, userEvent, waitFor, within} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import RolesSettings from './roles_dialog';

jest.mock('mattermost-redux/actions/teams', () => ({
    ...jest.requireActual('mattermost-redux/actions/teams'),
    patchTeam: jest.fn(() => ({type: 'MOCK_PATCH_TEAM'})),
    getTeam: jest.fn(() => ({type: 'MOCK_GET_TEAM'})),
}));
jest.mock('./teams', () => ({
    ...jest.requireActual('./teams'),
    loadTeams: jest.fn(() => ({type: 'MOCK_LOAD_TEAMS'})),
}));
jest.mock('fusion/shell/toast_context', () => ({useToast: () => jest.fn()}));

const mods = {id: 'mods', name: 'mods', display_name: 'Moderators', source: 'custom', team_id: 't1', delete_at: 0, position: 0, member_ids: [], color: '#e67e22'} as unknown as Group;
const everyone = {id: 'everyone', name: 'default-t1', display_name: 'Members', source: 'custom', team_id: 't1', delete_at: 0, position: 10000, member_ids: ['me'], color: ''} as unknown as Group;

function state(view?: TeamMemberListView, defaultRole = false) {
    return {
        entities: {
            general: {config: {}},
            users: {currentUserId: 'me', profiles: {me: TestHelper.getUserMock({id: 'me', roles: 'system_user'})}},
            teams: {
                currentTeamId: 't1',
                teams: {t1: TestHelper.getTeamMock({id: 't1', name: 'lumen', member_list_view: view, default_role_id: defaultRole ? 'everyone' : ''})},
                myMembers: {t1: {team_id: 't1', user_id: 'me', roles: 'team_user team_admin'}},
            },
            groups: {groups: defaultRole ? {mods, everyone} : {mods}},
            roles: {roles: {
                system_user: {name: 'system_user', permissions: []},
                team_user: {name: 'team_user', permissions: []},
                team_admin: {name: 'team_admin', permissions: ['manage_team', 'manage_team_roles', 'remove_user_from_team', 'delete_others_posts', 'edit_others_posts']},
            }},
        },
    } as never;
}

describe('fusion/teams/RolesSettings', () => {
    let ensure: jest.SpyInstance;
    beforeEach(() => {
        (patchTeam as jest.Mock).mockClear();
        ensure = jest.spyOn(Client4, 'ensureTeamDefaultRole').mockResolvedValue(everyone as never);
    });

    afterEach(() => {
        ensure.mockRestore();
    });

    test('sets up the default role the first time the roles are managed', async () => {
        renderWithContext(<RolesSettings onClose={jest.fn()}/>, state());
        await waitFor(() => expect(ensure).toHaveBeenCalledWith('t1'));
    });

    test('the default role comes last, and can be edited but not moved, deleted, given or opened to all', async () => {
        jest.spyOn(Client4, 'getTeamRole').mockResolvedValue({permissions: []} as never);
        renderWithContext(<RolesSettings onClose={jest.fn()}/>, state(undefined, true));
        expect(ensure).not.toHaveBeenCalled();

        const rows = screen.getAllByText(/Moderators|Members/).map((el) => el.textContent);
        expect(rows).toEqual(['Moderators', 'Members']);
        expect(screen.getByText('Default role: everyone in the community has it')).toBeInTheDocument();
        expect(screen.getAllByRole('button', {name: 'Delete the role'})).toHaveLength(1);
        expect(screen.getAllByRole('button', {name: 'Move down'})).toHaveLength(1);
        expect(screen.getAllByRole('button', {name: 'Move down'})[0]).toBeDisabled();

        await userEvent.click(screen.getAllByRole('button', {name: 'Edit the role'})[1]);
        expect(screen.getByDisplayValue('Members')).toBeInTheDocument();
        expect(screen.queryByRole('switch', {name: 'Anyone can take it'})).not.toBeInTheDocument();
        expect(screen.queryByRole('textbox', {name: 'Add people of the community…'})).not.toBeInTheDocument();
        expect(screen.getByText(/Everyone in the community has this role/)).toBeInTheDocument();
    });

    test('the team badge setting is in the Teams tab, not here', () => {
        renderWithContext(<RolesSettings onClose={jest.fn()}/>, state());
        expect(screen.queryByRole('switch', {name: 'Everyone shows a team badge'})).not.toBeInTheDocument();
    });

    test('the community\'s admins choose how the member list groups people', async () => {
        renderWithContext(<RolesSettings onClose={jest.fn()}/>, state());
        const view = screen.getByRole('group', {name: 'Member list'});
        expect(within(view).getByRole('button', {name: 'Let members choose'})).toHaveAttribute('aria-pressed', 'true');

        await userEvent.click(within(view).getByRole('button', {name: 'By team'}));
        expect(patchTeam).toHaveBeenCalledWith({id: 't1', member_list_view: 'teams'});
    });

    test('shows the member list view chosen', () => {
        renderWithContext(<RolesSettings onClose={jest.fn()}/>, state('roles'));
        expect(within(screen.getByRole('group', {name: 'Member list'})).getByRole('button', {name: 'By roles'})).toHaveAttribute('aria-pressed', 'true');
    });

    test('Close closes the community settings', async () => {
        const onClose = jest.fn();
        renderWithContext(<RolesSettings onClose={onClose}/>, state());
        await userEvent.click(screen.getByRole('button', {name: 'Close'}));
        expect(onClose).toHaveBeenCalled();
    });

    test('Administrator includes the other switches', async () => {
        jest.spyOn(Client4, 'getTeamRole').mockResolvedValue({permissions: ['manage_team']} as never);
        renderWithContext(<RolesSettings onClose={jest.fn()}/>, state());
        await userEvent.click(screen.getByRole('button', {name: 'Edit the role'}));

        await waitFor(() => expect(screen.getByRole('switch', {name: 'Administrator'})).toHaveAttribute('aria-checked', 'true'));
        const messages = screen.getByRole('switch', {name: 'Manage messages'});
        expect(messages).toHaveAttribute('aria-checked', 'true');
        expect(messages).toBeDisabled();
        expect(screen.getByRole('switch', {name: 'Remove people'})).toHaveAttribute('aria-checked', 'true');

        // Turned off, the others are as the role holds them.
        await userEvent.click(screen.getByRole('switch', {name: 'Administrator'}));
        expect(screen.getByRole('switch', {name: 'Manage messages'})).toHaveAttribute('aria-checked', 'false');
        expect(screen.getByRole('switch', {name: 'Manage messages'})).toBeEnabled();
    });

    test('Administrator shows the management of the community\'s model connections on', async () => {
        jest.spyOn(Client4, 'getTeamRole').mockResolvedValue({permissions: ['manage_team']} as never);
        renderWithContext(<RolesSettings onClose={jest.fn()}/>, state());
        await userEvent.click(screen.getByRole('button', {name: 'Edit the role'}));

        const connections = await screen.findByRole('switch', {name: 'Manage model connections'});
        await waitFor(() => expect(connections).toHaveAttribute('aria-checked', 'true'));
        expect(connections).toBeDisabled();
    });

    test('a role can grant the management of the community\'s model connections on its own', async () => {
        jest.spyOn(Client4, 'getTeamRole').mockResolvedValue({permissions: []} as never);
        const update = jest.spyOn(Client4, 'updateTeamRolePermissions').mockResolvedValue({permissions: ['manage_team_ai_connections']} as never);
        const patch = jest.spyOn(Client4, 'patchGroup').mockResolvedValue(mods as never);
        const admin = state();
        (admin as unknown as {entities: {roles: {roles: {team_admin: {permissions: string[]}}}}}).entities.roles.roles.team_admin.permissions.push('manage_team_ai_connections');
        renderWithContext(<RolesSettings onClose={jest.fn()}/>, admin);
        await userEvent.click(screen.getByRole('button', {name: 'Edit the role'}));

        const connections = await screen.findByRole('switch', {name: 'Manage model connections'});
        await waitFor(() => expect(connections).toBeEnabled());
        expect(connections).toHaveAttribute('aria-checked', 'false');
        await userEvent.click(connections);
        expect(connections).toHaveAttribute('aria-checked', 'true');

        await userEvent.click(screen.getByRole('button', {name: 'Save'}));
        await waitFor(() => expect(update).toHaveBeenCalledWith('mods', ['manage_team_ai_connections']));
        patch.mockRestore();
        update.mockRestore();
    });
});
