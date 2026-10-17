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
}));
jest.mock('./teams', () => ({
    ...jest.requireActual('./teams'),
    loadTeams: jest.fn(() => ({type: 'MOCK_LOAD_TEAMS'})),
}));
jest.mock('fusion/shell/toast_context', () => ({useToast: () => jest.fn()}));

const mods = {id: 'mods', name: 'mods', display_name: 'Moderators', source: 'custom', team_id: 't1', delete_at: 0, position: 0, member_ids: [], color: '#e67e22'} as unknown as Group;

function state(view?: TeamMemberListView) {
    return {
        entities: {
            general: {config: {}},
            users: {currentUserId: 'me', profiles: {me: TestHelper.getUserMock({id: 'me', roles: 'system_user'})}},
            teams: {
                currentTeamId: 't1',
                teams: {t1: TestHelper.getTeamMock({id: 't1', name: 'lumen', member_list_view: view})},
                myMembers: {t1: {team_id: 't1', user_id: 'me', roles: 'team_user team_admin'}},
            },
            groups: {groups: {mods}},
            roles: {roles: {
                system_user: {name: 'system_user', permissions: []},
                team_user: {name: 'team_user', permissions: []},
                team_admin: {name: 'team_admin', permissions: ['manage_team', 'manage_team_roles', 'remove_user_from_team', 'delete_others_posts', 'edit_others_posts']},
            }},
        },
    } as never;
}

describe('fusion/teams/RolesSettings', () => {
    beforeEach(() => {
        (patchTeam as jest.Mock).mockClear();
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
});
