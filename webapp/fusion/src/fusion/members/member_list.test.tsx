// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import MemberList, {matchesSearch} from './member_list';

jest.mock('fusion/components/avatar', () => () => null);
jest.mock('fusion/components/status_emoji', () => () => null);
jest.mock('fusion/teams/team_card', () => ({TeamTag: () => null}));
jest.mock('fusion/calls/markers', () => ({TalkNote: ({children}: {children: React.ReactNode}) => children}));
jest.mock('fusion/agents/agent_status', () => ({
    AgentActivityLine: () => null,
    IntegrationTag: () => null,
    useAgentBusy: () => false,
}));
jest.mock('fusion/attributes/member_list_attribute', () => ({
    useMemberListAttribute: () => ({filtering: false, shown: () => true, textOf: () => '', filter: null}),
}));
jest.mock('./member_add_popover', () => ({
    __esModule: true,
    default: () => null,
    useCanAddPeople: () => ({add: false, invite: false}),
}));
jest.mock('components/channel_members_rhs/pending_join_requests', () => () => null);
jest.mock('fusion/popovers/user_popover', () => () => null);
jest.mock('fusion/hooks/users', () => ({
    ...jest.requireActual('fusion/hooks/users'),
    useDisplayName: (user?: {username?: string}) => user?.username || '',
}));
jest.mock('fusion/teams/roles', () => ({
    ...jest.requireActual('fusion/teams/roles'),
    useRoleColor: () => undefined,
}));
jest.mock('fusion/teams/teams', () => ({
    ...jest.requireActual('fusion/teams/teams'),
    getTeams: () => [],
    useRoleGroupsAllowed: () => false,
}));
jest.mock('fusion/hooks/media', () => ({
    ...jest.requireActual('fusion/hooks/media'),
    useIsPhone: () => false,
}));
jest.mock('fusion/modals/settings_controls', () => ({
    ...jest.requireActual('fusion/modals/settings_controls'),
    usePreference: () => ['roles', jest.fn()],
}));
jest.mock('fusion/shell/layout_context', () => ({
    ...jest.requireActual('fusion/shell/layout_context'),
    useLayout: () => ({setRightOpen: jest.fn(), rightOpen: true}),
}));
jest.mock('selectors/views/custom_status', () => ({makeGetCustomStatus: () => () => undefined}));
jest.mock('actions/user_actions', () => ({loadProfilesAndReloadChannelMembers: jest.fn(() => ({type: 'NOOP'}))}));
jest.mock('actions/views/rhs', () => ({closeRightHandSide: jest.fn(() => ({type: 'NOOP'}))}));
jest.mock('mattermost-redux/actions/channels', () => ({
    ...jest.requireActual('mattermost-redux/actions/channels'),
    getChannelStats: jest.fn(() => ({type: 'NOOP'})),
    getChannelJoinRequests: jest.fn(() => ({type: 'NOOP'})),
    countPendingChannelJoinRequests: jest.fn(() => ({type: 'NOOP'})),
}));
jest.mock('mattermost-redux/selectors/entities/channels', () => ({
    ...jest.requireActual('mattermost-redux/selectors/entities/channels'),
    canManageChannelJoinRequests: () => false,
}));

// Grouping is covered by member_groups.test.ts; here every shown member lands in one group, so the search box's
// filtering (member_list's own job) is what the tests exercise.
jest.mock('./member_groups', () => ({
    ...jest.requireActual('./member_groups'),
    useMemberRoles: () => ({}),
    useMemberGroups: (users: Array<{id: string}>) => (users.length ? [{key: 'all', label: `All — ${users.length}`, users, off: false}] : []),
}));

const alice = TestHelper.getUserMock({id: 'alice', username: 'alice', nickname: '', first_name: 'Alice', last_name: 'Anderson'});
const bob = TestHelper.getUserMock({id: 'bob', username: 'bob', nickname: 'Bobby', first_name: 'Robert', last_name: 'Bobson'});
const carol = TestHelper.getUserMock({id: 'carol', username: 'carol', nickname: '', first_name: 'Carol', last_name: 'Carlisle'});

function state() {
    return {
        entities: {
            general: {config: {}},
            users: {
                currentUserId: 'me',
                profiles: {me: TestHelper.getUserMock({id: 'me', username: 'me'}), alice, bob, carol},
                profilesInChannel: {c1: new Set(['alice', 'bob', 'carol'])},
            },
            teams: {
                currentTeamId: 't1',
                teams: {t1: {id: 't1', name: 'lumen', display_name: 'Lumen', delete_at: 0}},
            },
            channels: {
                currentChannelId: 'c1',
                channels: {c1: {id: 'c1', team_id: 't1', type: 'O', name: 'dev', display_name: 'dev', delete_at: 0}},
                stats: {c1: {member_count: 3}},
                membersInChannel: {c1: {}},
            },
            preferences: {myPreferences: {}},
        },
    };
}

describe('fusion/members/MemberList: search box', () => {
    const render = () => renderWithContext(<MemberList/>, state() as never);

    test('lists every member when nothing is typed', () => {
        render();
        expect(screen.getByText('alice')).toBeInTheDocument();
        expect(screen.getByText('bob')).toBeInTheDocument();
        expect(screen.getByText('carol')).toBeInTheDocument();
    });

    test('filters the list as a name is typed', async () => {
        render();
        const box = screen.getByRole('searchbox', {name: 'Search members'});
        await userEvent.type(box, 'ali');
        expect(screen.getByText('alice')).toBeInTheDocument();
        expect(screen.queryByText('bob')).not.toBeInTheDocument();
        expect(screen.queryByText('carol')).not.toBeInTheDocument();
    });

    test('matches a nickname too', async () => {
        render();
        const box = screen.getByRole('searchbox', {name: 'Search members'});
        await userEvent.type(box, 'bobby');
        expect(screen.getByText('bob')).toBeInTheDocument();
        expect(screen.queryByText('alice')).not.toBeInTheDocument();
    });

    test('says so when nobody matches', async () => {
        render();
        const box = screen.getByRole('searchbox', {name: 'Search members'});
        await userEvent.type(box, 'nobody-like-this');
        expect(screen.getByText('No member matches.')).toBeInTheDocument();
        expect(screen.queryByText('alice')).not.toBeInTheDocument();
    });
});

describe('fusion/members/matchesSearch', () => {
    test('matches the username, nickname or full name, case-insensitively', () => {
        expect(matchesSearch(bob, 'bob')).toBe(true);
        expect(matchesSearch(bob, 'bobby')).toBe(true);
        expect(matchesSearch(bob, 'robert')).toBe(true);
        expect(matchesSearch(bob, 'robert bobson')).toBe(true);
        expect(matchesSearch(bob, 'BOB')).toBe(true);
        expect(matchesSearch(bob, 'carol')).toBe(false);
    });
});
