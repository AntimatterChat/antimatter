// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {Client4} from 'mattermost-redux/client';
import {Permissions} from 'mattermost-redux/constants';

import {renderWithContext, screen, userEvent, waitFor, within} from 'tests/react_testing_utils';

import BanList from './ban_list';
import MembersSettings from './members_settings';

jest.mock('mattermost-redux/actions/users', () => ({
    ...jest.requireActual('mattermost-redux/actions/users'),
    getMissingProfilesByIds: jest.fn(() => ({type: 'MOCK_PROFILES'})),
}));
jest.mock('components/member_list_team', () => () => null);
jest.mock('components/team_members_modal/team_members_modal', () => ({MembershipRequirementsBanner: () => null}));
const mockToast = jest.fn();
jest.mock('fusion/shell/toast_context', () => ({useToast: () => mockToast}));

const ban = (userId: string, extra = {}) => ({team_id: 't1', user_id: userId, reason: '', created_by: 'me', create_at: Date.UTC(2026, 9, 20, 12), expire_at: 0, ...extra});

const state = (teamPermissions: string[] = []) => ({
    entities: {
        general: {config: {}},
        teams: {
            currentTeamId: 't1',
            teams: {t1: {id: 't1', name: 'one', display_name: 'One', delete_at: 0}},
            myMembers: {t1: {team_id: 't1', user_id: 'me', roles: 'team_user'}},
        },
        roles: {roles: {team_user: {name: 'team_user', permissions: teamPermissions}, system_user: {name: 'system_user', permissions: []}}},
        users: {
            currentUserId: 'me',
            profiles: {
                me: {id: 'me', username: 'maya', roles: 'system_user'},
                bob: {id: 'bob', username: 'bob', roles: 'system_user'},
                eve: {id: 'eve', username: 'eve', roles: 'system_user'},
            },
        },
    },
}) as never;

afterEach(() => {
    jest.restoreAllMocks();
    jest.clearAllMocks();
});

describe('BanList', () => {
    test('lists who is banned, by whom, until when and why', async () => {
        jest.spyOn(Client4, 'getTeamBans').mockResolvedValue([
            ban('bob', {reason: 'raids'}),
            ban('eve', {expire_at: Date.UTC(2026, 10, 1, 12)}),
        ]);
        renderWithContext(<BanList teamId='t1'/>, state());

        const bob = await screen.findByText('raids');
        const bobRow = bob.closest('[data-user]') as HTMLElement;
        expect(bobRow).toHaveAttribute('data-user', 'bob');
        expect(within(bobRow).getByText(/@bob · Banned by .* on .* · until it's lifted/)).toBeInTheDocument();
        const eveRow = screen.getByText(/@eve/).closest('[data-user]') as HTMLElement;
        expect(within(eveRow).getByText(/until Nov 1, 2026/)).toBeInTheDocument();
        expect(Client4.getTeamBans).toHaveBeenCalledWith('t1');
    });

    test('lifting a ban takes it off the list', async () => {
        jest.spyOn(Client4, 'getTeamBans').mockResolvedValue([ban('bob'), ban('eve')]);
        const unban = jest.spyOn(Client4, 'unbanTeamMember').mockResolvedValue({status: 'OK'});
        renderWithContext(<BanList teamId='t1'/>, state());

        const bobRow = (await screen.findByText(/@bob/)).closest('[data-user]') as HTMLElement;
        await userEvent.click(within(bobRow).getByRole('button', {name: 'Lift ban'}));
        await waitFor(() => expect(unban).toHaveBeenCalledWith('t1', 'bob'));
        await waitFor(() => expect(screen.queryByText(/@bob/)).not.toBeInTheDocument());
        expect(screen.getByText(/@eve/)).toBeInTheDocument();
        expect(mockToast).toHaveBeenCalled();
    });

    test('a ban the server won\'t lift stays, with why', async () => {
        jest.spyOn(Client4, 'getTeamBans').mockResolvedValue([ban('bob')]);
        jest.spyOn(Client4, 'unbanTeamMember').mockRejectedValue({message: 'Only someone at or above the rank of whoever made this ban can lift it.'});
        renderWithContext(<BanList teamId='t1'/>, state());

        await userEvent.click(await screen.findByRole('button', {name: 'Lift ban'}));
        await waitFor(() => expect(mockToast).toHaveBeenCalledWith('Only someone at or above the rank of whoever made this ban can lift it.'));
        expect(screen.getByText(/@bob/)).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Lift ban'})).toBeEnabled();
    });

    test('says when nobody is banned, or the bans couldn\'t be loaded', async () => {
        jest.spyOn(Client4, 'getTeamBans').mockResolvedValue([]);
        const {unmount} = renderWithContext(<BanList teamId='t1'/>, state());
        expect(await screen.findByText(/Nobody is banned/)).toBeInTheDocument();
        unmount();

        jest.spyOn(Client4, 'getTeamBans').mockRejectedValue(new Error('nope'));
        renderWithContext(<BanList teamId='t1'/>, state());
        expect(await screen.findByText('The bans couldn\'t be loaded.')).toBeInTheDocument();
    });
});

// RBAC: the bans show on the Members page only to those who may ban.
describe('MembersSettings: the bans', () => {
    test.each<[string, string[], boolean]>([
        ['those who may remove members', [Permissions.REMOVE_USER_FROM_TEAM], true],
        ['the community\'s admins', [Permissions.MANAGE_TEAM], true],
        ['a moderator of messages', [Permissions.DELETE_OTHERS_POSTS], false],
        ['a member who invites', [Permissions.INVITE_USER, Permissions.ADD_USER_TO_TEAM], false],
    ])('%s: shown %s', async (who, permissions, shown) => {
        const get = jest.spyOn(Client4, 'getTeamBans').mockResolvedValue([]);
        renderWithContext(<MembersSettings onInvite={jest.fn()}/>, state(permissions));
        if (shown) {
            expect(await screen.findByRole('region', {name: 'Banned'})).toBeInTheDocument();
            expect(get).toHaveBeenCalledWith('t1');
        } else {
            expect(screen.queryByRole('region', {name: 'Banned'})).not.toBeInTheDocument();
            expect(get).not.toHaveBeenCalled();
        }
    });
});
