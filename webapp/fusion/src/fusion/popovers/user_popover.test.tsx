// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {Permissions} from 'mattermost-redux/constants';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import UserPopover from './user_popover';

// The card's other parts have tests of their own.
jest.mock('fusion/calls/call_user_button', () => ({__esModule: true, default: () => null, useCanCall: () => false}));
jest.mock('fusion/teams/team_card', () => ({TeamTag: () => null}));
jest.mock('plugins/pluggable', () => () => null);
jest.mock('./profile_facts', () => () => null);
jest.mock('./role_chips', () => () => null);
jest.mock('./status_popover', () => () => null);
jest.mock('./user_menu', () => ({UserMenu: () => null}));
jest.mock('./user_note', () => () => null);

// Antimatter: your own profile card links to the System Console for every role that may read a part of it, as the
// status menu does (the product menu, which had it for them, only shows when an app has a page of its own).
describe('fusion/popovers/UserPopover: the System Console link', () => {
    beforeEach(() => {
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
    });

    const renderCard = (userId: string, roles: string, permissions: string[] = []) => renderWithContext(
        <UserPopover
            userId={userId}
            anchor={document.body}
            onClose={jest.fn()}
        />,
        {
            entities: {
                general: {config: {}},
                users: {
                    currentUserId: 'me',
                    profiles: {
                        me: TestHelper.getUserMock({id: 'me', username: 'me', roles}),
                        sofia: TestHelper.getUserMock({id: 'sofia', username: 'sofia', roles: 'system_user'}),
                    },
                },
                teams: {
                    currentTeamId: 't1',
                    teams: {t1: TestHelper.getTeamMock({id: 't1', name: 'lumen'})},
                },
                roles: {roles: {system_user: {permissions}, system_admin: {permissions: []}}},
            },
        } as never,
    );

    test('is not on the card of a member without any System Console permission', () => {
        renderCard('me', 'system_user');
        expect(screen.queryByRole('button', {name: 'System Console'})).not.toBeInTheDocument();
    });

    test('is on a system admin\'s own card', () => {
        renderCard('me', 'system_user system_admin');
        expect(screen.getByRole('button', {name: 'System Console'})).toBeInTheDocument();
    });

    test('is on the own card of a role that may read a part of the System Console', () => {
        renderCard('me', 'system_user', [Permissions.SYSCONSOLE_READ_REPORTING_SITE_STATISTICS]);
        expect(screen.getByRole('button', {name: 'System Console'})).toBeInTheDocument();
    });

    test('is never on someone else\'s card', () => {
        renderCard('sofia', 'system_user system_admin');
        expect(screen.queryByRole('button', {name: 'System Console'})).not.toBeInTheDocument();
    });
});

// Antimatter: in a community where someone set a nickname or picture of their own, their card has their name there,
// and how they show outside it.
describe('fusion/popovers/UserPopover: community profiles', () => {
    beforeEach(() => {
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
    });

    const renderCard = (profiles: Record<string, unknown>) => renderWithContext(
        <UserPopover
            userId='sofia'
            anchor={document.body}
            onClose={jest.fn()}
        />,
        {
            entities: {
                general: {config: {TeammateNameDisplay: 'full_name'}},
                users: {
                    currentUserId: 'me',
                    profiles: {
                        me: TestHelper.getUserMock({id: 'me', username: 'me', roles: 'system_user'}),
                        sofia: TestHelper.getUserMock({id: 'sofia', username: 'sofia', first_name: 'Sofia', last_name: 'Reyes', roles: 'system_user'}),
                    },
                },
                teams: {
                    currentTeamId: 't1',
                    teams: {t1: TestHelper.getTeamMock({id: 't1', name: 'lumen', name_display: 'nickname_full_name'})},
                    communityProfiles: {t1: profiles},
                },
                roles: {roles: {system_user: {permissions: []}}},
            },
        } as never,
    );

    test('shows their nickname in the community, and their name outside it', () => {
        renderCard({sofia: {team_id: 't1', user_id: 'sofia', nickname: 'Captain', update_at: 1}});
        expect(screen.getByRole('heading', {name: 'Captain'})).toBeInTheDocument();
        expect(screen.getByText('Outside this community: Sofia Reyes')).toBeInTheDocument();
    });

    test('without a profile of their own there, no second name', () => {
        renderCard({});
        expect(screen.getByRole('heading', {name: 'Sofia Reyes'})).toBeInTheDocument();
        expect(screen.queryByText(/Outside this community/)).not.toBeInTheDocument();
    });
});
