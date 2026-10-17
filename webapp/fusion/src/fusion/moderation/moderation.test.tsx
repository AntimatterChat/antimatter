// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {TeamTypes} from 'mattermost-redux/action_types';
import {Client4} from 'mattermost-redux/client';
import {timeouts} from 'mattermost-redux/reducers/entities/teams';

import {LAYER_ID} from 'fusion/components/layer';
import {renderWithContext, screen, userEvent, waitFor, within} from 'tests/react_testing_utils';
import {RHSStates} from 'utils/constants';

import ModViewPanel from './mod_view';
import TimeoutDialog from './timeout_dialog';

const mockDialogs = {timeOut: jest.fn(), removeFromTeam: jest.fn(), confirm: jest.fn()};
jest.mock('fusion/shell/dialogs_context', () => ({useDialogs: () => mockDialogs}));
jest.mock('fusion/teams/teams', () => ({...jest.requireActual('fusion/teams/teams'), loadTeams: jest.fn(() => ({type: 'MOCK_LOAD_TEAMS'}))}));

const team = {id: 'team', name: 'antimatter', display_name: 'Antimatter'};
const bob = {id: 'bob', username: 'bob', roles: 'system_user', create_at: Date.UTC(2024, 0, 15), delete_at: 0};
const admin = {id: 'me', username: 'me', roles: 'system_user'};
const roles = {
    system_user: {name: 'system_user', permissions: []},
    system_admin: {name: 'system_admin', permissions: ['manage_system', 'manage_team', 'remove_user_from_team', 'manage_team_roles', 'delete_others_posts']},
    team_user: {name: 'team_user', permissions: []},
    team_admin: {name: 'team_admin', permissions: ['manage_team', 'remove_user_from_team', 'manage_team_roles']},

    // A moderator who manages messages only: they get the mod view, but no timeouts nor removals.
    message_mod: {name: 'message_mod', permissions: ['delete_others_posts']},
};
const role = (id: string, name: string, members: string[]) => ({id, team_id: 'team', is_team: false, source: 'custom', delete_at: 0, display_name: name, name: id, member_ids: members, color: '#e11d48', position: 0});
const stateWith = (myTeamRoles: string, timeout?: {until: number; reason: string}, mySystemRoles = 'system_user') => ({
    entities: {
        users: {currentUserId: 'me', profiles: {me: {...admin, roles: mySystemRoles}, bob}, statuses: {bob: 'online'}},
        teams: {
            currentTeamId: 'team',
            teams: {team},
            myMembers: {team: {team_id: 'team', user_id: 'me', roles: myTeamRoles}},
            membersInTeam: {team: {bob: {team_id: 'team', user_id: 'bob', roles: 'team_user'}}},
            timeouts: {team: timeout ? [{team_id: 'team', user_id: 'bob', created_by: 'me', create_at: 0, ...timeout}] : []},
        },
        roles: {roles},
        groups: {groups: {helpers: role('helpers', 'Helpers', ['bob']), mods: role('mods', 'Mods', [])}},
    },
    views: {rhs: {rhsState: RHSStates.MOD_VIEW, modViewUserId: 'bob'}},
}) as never;

beforeEach(() => {
    const layer = document.createElement('div');
    layer.id = LAYER_ID;
    document.body.appendChild(layer);
    jest.spyOn(Client4, 'getMemberModeration').mockResolvedValue({team_id: 'team', user_id: 'bob', joined_at: Date.UTC(2025, 2, 1), posts_last_day: 3, posts_last_week: 12});
    jest.spyOn(Client4, 'getTeamMember').mockResolvedValue({team_id: 'team', user_id: 'bob', roles: 'team_user'} as never);
    jest.spyOn(Client4, 'getRolesByNames').mockResolvedValue([] as never);
});

afterEach(() => {
    document.getElementById(LAYER_ID)?.remove();
    jest.restoreAllMocks();
    jest.clearAllMocks();
});

// The mod view's actions, by their data-action, in order.
const actionIds = () => Array.from(screen.getByRole('toolbar', {name: 'Actions'}).querySelectorAll('[data-action]')).map((el) => el.getAttribute('data-action'));

describe('timeouts reducer', () => {
    test('keeps each team\'s timeouts as received', () => {
        const list = [{team_id: 'team', user_id: 'bob', until: 1, reason: '', created_by: 'me', create_at: 0}];
        expect(timeouts({}, {type: TeamTypes.RECEIVED_TEAM_TIMEOUTS, data: {teamId: 'team', timeouts: list}})).toEqual({team: list});
    });
});

describe('ModViewPanel', () => {
    test('shows a moderator someone\'s summary and the actions they may take', async () => {
        renderWithContext(<ModViewPanel/>, stateWith('team_user team_admin'));

        expect(await screen.findByText('3 in the last day · 12 in the last week')).toBeInTheDocument();
        expect(screen.getByRole('heading', {name: 'Mod view'})).toBeInTheDocument();
        expect(screen.getByText('Account created')).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Time out…'})).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Remove from community…'})).toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Deactivate account…'})).not.toBeInTheDocument();
    });

    test('a timeout shows, with a way to lift it', async () => {
        const lift = jest.spyOn(Client4, 'setMemberTimeout').mockResolvedValue({status: 'OK'} as never);
        jest.spyOn(Client4, 'getTeamTimeouts').mockResolvedValue([]);
        renderWithContext(<ModViewPanel/>, stateWith('team_user team_admin', {until: Date.now() + 3600000, reason: 'flooding'}));

        expect(await screen.findByText('flooding')).toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Time out…'})).not.toBeInTheDocument();
        await userEvent.click(screen.getByRole('button', {name: 'Remove timeout'}));
        await waitFor(() => expect(lift).toHaveBeenCalledWith('team', 'bob', 0, ''));
    });

    test('shows nothing to members who can\'t moderate', () => {
        const {container} = renderWithContext(<ModViewPanel/>, stateWith('team_user'));
        expect(container).toBeEmptyDOMElement();
    });
});

describe('ModViewPanel: its actions, per role, each once', () => {
    test.each([
        ['a community admin', 'team_user team_admin', 'system_user', ['message', 'search-messages', 'time-out', 'remove-from-community']],
        ['a system admin', 'team_user', 'system_user system_admin', ['message', 'search-messages', 'time-out', 'remove-from-community', 'deactivate']],
        ['a moderator of messages', 'team_user message_mod', 'system_user', ['message', 'search-messages']],
    ])('%s gets the actions they may take, in order, the destructive ones last', async (who, teamRoles, systemRoles, expected) => {
        renderWithContext(<ModViewPanel/>, stateWith(teamRoles, undefined, systemRoles));
        await screen.findByText('3 in the last day · 12 in the last week');
        expect(actionIds()).toEqual(expected);
        expect(new Set(actionIds()).size).toBe(actionIds().length);
        for (const id of ['remove-from-community', 'deactivate']) {
            const button = screen.getByRole('toolbar').querySelector(`[data-action="${id}"]`);
            if (button) {
                expect(button).toHaveClass('am-danger');
            }
        }
    });

    test('a timed-out member\'s timeout is lifted from the actions, once: there is no "Time out…" beside it', async () => {
        renderWithContext(<ModViewPanel/>, stateWith('team_user team_admin', {until: Date.now() + 3600000, reason: 'flooding'}));
        await screen.findByText('flooding');
        expect(actionIds()).toEqual(['message', 'search-messages', 'lift-timeout', 'remove-from-community']);
        expect(screen.getAllByRole('button', {name: 'Remove timeout'})).toHaveLength(1);
    });

    // RBAC: what a role doesn't allow isn't offered.
    test('a moderator of messages neither times out, removes nor deactivates, and doesn\'t change roles', async () => {
        renderWithContext(<ModViewPanel/>, stateWith('team_user message_mod'));
        await screen.findByText('3 in the last day · 12 in the last week');
        expect(screen.queryByRole('button', {name: 'Time out…'})).not.toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Remove from community…'})).not.toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Deactivate account…'})).not.toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Add a role'})).not.toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Remove the role Helpers'})).not.toBeInTheDocument();
        expect(screen.getByText('Helpers')).toBeInTheDocument();
    });

    test('a community admin who isn\'t a system admin doesn\'t deactivate accounts', async () => {
        renderWithContext(<ModViewPanel/>, stateWith('team_user team_admin'));
        await screen.findByText('3 in the last day · 12 in the last week');
        expect(actionIds()).not.toContain('deactivate');
    });
});

describe('ModViewPanel: the destructive actions ask first', () => {
    test('removing from the community opens its confirmation', async () => {
        renderWithContext(<ModViewPanel/>, stateWith('team_user team_admin'));
        await userEvent.click(await screen.findByRole('button', {name: 'Remove from community…'}));
        expect(mockDialogs.removeFromTeam).toHaveBeenCalledWith('bob');
    });

    test('deactivating asks, and only deactivates once confirmed', async () => {
        const deactivate = jest.spyOn(Client4, 'updateUserActive').mockResolvedValue({status: 'OK'} as never);
        renderWithContext(<ModViewPanel/>, stateWith('team_user', undefined, 'system_user system_admin'));
        await userEvent.click(await screen.findByRole('button', {name: 'Deactivate account…'}));
        expect(mockDialogs.confirm).toHaveBeenCalledWith(expect.objectContaining({confirmLabel: 'Deactivate account', danger: true}));
        expect(deactivate).not.toHaveBeenCalled();
        await mockDialogs.confirm.mock.calls[0][0].onConfirm();
        expect(deactivate).toHaveBeenCalledWith('bob', false);
    });

    test('timing out opens the timeout dialog, to choose for how long', async () => {
        renderWithContext(<ModViewPanel/>, stateWith('team_user team_admin'));
        await userEvent.click(await screen.findByRole('button', {name: 'Time out…'}));
        expect(mockDialogs.timeOut).toHaveBeenCalledWith('bob');
    });
});

describe('ModViewPanel: the keyboard, and roles', () => {
    test('the arrow keys, Home and End move along the actions', async () => {
        renderWithContext(<ModViewPanel/>, stateWith('team_user team_admin'));
        const message = await screen.findByRole('button', {name: 'Message'});
        message.focus();
        await userEvent.keyboard('{ArrowDown}');
        expect(screen.getByRole('button', {name: 'Search their messages'})).toHaveFocus();
        await userEvent.keyboard('{End}');
        expect(screen.getByRole('button', {name: 'Remove from community…'})).toHaveFocus();
        await userEvent.keyboard('{ArrowDown}');
        expect(message).toHaveFocus();
        await userEvent.keyboard('{ArrowUp}');
        expect(screen.getByRole('button', {name: 'Remove from community…'})).toHaveFocus();
    });

    test('"Add a role" is a menu of the roles they don\'t have', async () => {
        const add = jest.spyOn(Client4, 'addUsersToGroup').mockResolvedValue([] as never);
        renderWithContext(<ModViewPanel/>, stateWith('team_user team_admin'));
        const button = await screen.findByRole('button', {name: 'Add a role'});
        expect(button).toHaveAttribute('aria-haspopup', 'menu');
        await userEvent.click(button);
        const menu = screen.getByRole('menu', {name: 'Add a role'});
        expect(within(menu).queryByRole('menuitem', {name: 'Helpers'})).not.toBeInTheDocument();
        expect(within(menu).getByRole('menuitem', {name: 'Mods'})).toHaveFocus();
        await userEvent.click(within(menu).getByRole('menuitem', {name: 'Mods'}));
        await waitFor(() => expect(add).toHaveBeenCalledWith('mods', ['bob']));
        expect(screen.queryByRole('menu', {name: 'Add a role'})).not.toBeInTheDocument();
    });

    test('the panel is a region named by its title', async () => {
        renderWithContext(<ModViewPanel/>, stateWith('team_user team_admin'));
        expect(await screen.findByRole('region', {name: 'Mod view'})).toBeInTheDocument();
    });
});

describe('TimeoutDialog', () => {
    test('times someone out for the chosen time, with a reason', async () => {
        const set = jest.spyOn(Client4, 'setMemberTimeout').mockResolvedValue({} as never);
        jest.spyOn(Client4, 'getTeamTimeouts').mockResolvedValue([]);
        const onClose = jest.fn();
        renderWithContext(
            <TimeoutDialog
                userId='bob'
                onClose={onClose}
            />,
            stateWith('team_user team_admin'),
        );

        await userEvent.click(screen.getByRole('radio', {name: '1 hour'}));
        await userEvent.type(screen.getByRole('textbox'), 'flooding');
        const before = Date.now();
        await userEvent.click(screen.getByRole('button', {name: 'Time out'}));

        await waitFor(() => expect(onClose).toHaveBeenCalled());
        const [teamId, userId, until, reason] = set.mock.calls[0];
        expect([teamId, userId, reason]).toEqual(['team', 'bob', 'flooding']);
        expect(until).toBeGreaterThanOrEqual(before + 3600000);
        expect(until).toBeLessThan(before + 3600000 + 10000);
    });
});
