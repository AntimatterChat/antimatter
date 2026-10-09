// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {TeamTypes} from 'mattermost-redux/action_types';
import {Client4} from 'mattermost-redux/client';
import {timeouts} from 'mattermost-redux/reducers/entities/teams';

import {LAYER_ID} from 'fusion/components/layer';
import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {RHSStates} from 'utils/constants';

import ModViewPanel from './mod_view';
import TimeoutDialog from './timeout_dialog';

const team = {id: 'team', name: 'antimatter', display_name: 'Antimatter'};
const bob = {id: 'bob', username: 'bob', roles: 'system_user', create_at: Date.UTC(2024, 0, 15), delete_at: 0};
const admin = {id: 'me', username: 'me', roles: 'system_user'};
const roles = {
    system_user: {name: 'system_user', permissions: []},
    team_user: {name: 'team_user', permissions: []},
    team_admin: {name: 'team_admin', permissions: ['manage_team', 'remove_user_from_team', 'manage_team_roles']},
};
const stateWith = (myTeamRoles: string, timeout?: {until: number; reason: string}) => ({
    entities: {
        users: {currentUserId: 'me', profiles: {me: admin, bob}, statuses: {bob: 'online'}},
        teams: {
            currentTeamId: 'team',
            teams: {team},
            myMembers: {team: {team_id: 'team', user_id: 'me', roles: myTeamRoles}},
            membersInTeam: {team: {bob: {team_id: 'team', user_id: 'bob', roles: 'team_user'}}},
            timeouts: {team: timeout ? [{team_id: 'team', user_id: 'bob', created_by: 'me', create_at: 0, ...timeout}] : []},
        },
        roles: {roles},
        groups: {groups: {}},
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
});

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
        expect(screen.getByRole('button', {name: 'Time out'})).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Remove from team'})).toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Deactivate account'})).not.toBeInTheDocument();
    });

    test('a timeout shows, with a way to lift it', async () => {
        const lift = jest.spyOn(Client4, 'setMemberTimeout').mockResolvedValue({status: 'OK'} as never);
        jest.spyOn(Client4, 'getTeamTimeouts').mockResolvedValue([]);
        renderWithContext(<ModViewPanel/>, stateWith('team_user team_admin', {until: Date.now() + 3600000, reason: 'flooding'}));

        expect(await screen.findByText('flooding')).toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Time out'})).not.toBeInTheDocument();
        await userEvent.click(screen.getByRole('button', {name: 'Remove timeout'}));
        await waitFor(() => expect(lift).toHaveBeenCalledWith('team', 'bob', 0, ''));
    });

    test('shows nothing to members who can\'t moderate', () => {
        const {container} = renderWithContext(<ModViewPanel/>, stateWith('team_user'));
        expect(container).toBeEmptyDOMElement();
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
