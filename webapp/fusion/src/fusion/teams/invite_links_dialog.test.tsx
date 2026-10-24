// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {Client4} from 'mattermost-redux/client';

import {LAYER_ID} from 'fusion/components/layer';
import {renderWithContext, screen, userEvent, waitFor, within} from 'tests/react_testing_utils';
import {copyToClipboard} from 'utils/utils';

import InviteLinksDialog, {inviteLinkURL} from './invite_links_dialog';

jest.mock('mattermost-redux/actions/users', () => ({
    ...jest.requireActual('mattermost-redux/actions/users'),
    getMissingProfilesByIds: jest.fn(() => ({type: 'MOCK_PROFILES'})),
}));
jest.mock('utils/utils', () => ({...jest.requireActual('utils/utils'), copyToClipboard: jest.fn()}));
const mockToast = jest.fn();
jest.mock('fusion/shell/toast_context', () => ({useToast: () => mockToast}));

const HOUR = 3600 * 1000;
const link = (id: string, extra = {}) => ({id, team_id: 't1', creator_id: 'me', create_at: 1, expire_at: 0, max_uses: 0, uses: 0, ...extra});

const state = {
    entities: {
        general: {config: {}},
        teams: {
            currentTeamId: 't1',
            teams: {t1: {id: 't1', name: 'one', display_name: 'One', delete_at: 0}},
            myMembers: {t1: {team_id: 't1', user_id: 'me', roles: 'team_user'}},
        },
        roles: {roles: {team_user: {name: 'team_user', permissions: ['invite_user']}}},
        users: {currentUserId: 'me', profiles: {me: {id: 'me', username: 'maya', roles: 'system_user'}}},
    },
} as never;

beforeEach(() => {
    const layer = document.createElement('div');
    layer.id = LAYER_ID;
    document.body.appendChild(layer);
});

afterEach(() => {
    document.getElementById(LAYER_ID)?.remove();
    jest.restoreAllMocks();
    jest.clearAllMocks();
});

const row = (id: string) => document.querySelector(`[data-link="${id}"]`) as HTMLElement;

describe('InviteLinksDialog', () => {
    test('is wide, like the community settings modal\'s other full-pane dialogs', async () => {
        jest.spyOn(Client4, 'getTeamInviteLinks').mockResolvedValue([]);
        renderWithContext(<InviteLinksDialog onExited={jest.fn()}/>, state);

        expect(await screen.findByRole('dialog', {name: 'Invite links'})).toHaveClass('am-wide');
    });

    test('lists the links, with their uses and their end; ended and used-up ones can only be revoked', async () => {
        jest.spyOn(Client4, 'getTeamInviteLinks').mockResolvedValue([
            link('open'),
            link('limited', {max_uses: 5, uses: 2, expire_at: Date.now() + HOUR}),
            link('ended', {expire_at: Date.now() - HOUR}),
            link('usedup', {max_uses: 1, uses: 1}),
        ]);
        renderWithContext(<InviteLinksDialog onExited={jest.fn()}/>, state);

        await waitFor(() => expect(row('open')).toBeTruthy());
        expect(within(row('open')).getByText(/0 uses · Never expires/)).toBeInTheDocument();
        expect(within(row('limited')).getByText(/2 of 5 uses · Expires/)).toBeInTheDocument();
        expect(within(row('ended')).getByText(/Expired/)).toBeInTheDocument();
        expect(within(row('usedup')).getByText(/Used up/)).toBeInTheDocument();
        expect(within(row('open')).getByRole('button', {name: /Copy the link/})).toBeInTheDocument();
        expect(within(row('ended')).queryByRole('button', {name: /Copy the link/})).not.toBeInTheDocument();
        expect(within(row('usedup')).queryByRole('button', {name: /Copy the link/})).not.toBeInTheDocument();
        expect(within(row('ended')).getByRole('button', {name: /Revoke the link/})).toBeInTheDocument();
        expect(within(row('open')).getByText(inviteLinkURL(link('open')))).toBeInTheDocument();
    });

    test('creates a link that ends after 7 days and has no limit by default, and copies it', async () => {
        jest.spyOn(Client4, 'getTeamInviteLinks').mockResolvedValue([]);
        const created = link('fresh', {expire_at: Date.now() + (7 * 24 * HOUR)});
        const create = jest.spyOn(Client4, 'createTeamInviteLink').mockResolvedValue(created);
        renderWithContext(<InviteLinksDialog onExited={jest.fn()}/>, state);

        expect(await screen.findByText('No links yet.')).toBeInTheDocument();
        const before = Date.now();
        await userEvent.click(screen.getByRole('button', {name: 'Create link'}));
        await waitFor(() => expect(row('fresh')).toBeTruthy());
        const [teamId, request] = create.mock.calls[0];
        expect(teamId).toBe('t1');
        expect(request.max_uses).toBe(0);
        expect(request.expire_at).toBeGreaterThanOrEqual(before + (7 * 24 * HOUR));
        expect(request.expire_at).toBeLessThan(before + (7 * 24 * HOUR) + 10000);
        expect(copyToClipboard).toHaveBeenCalledWith(inviteLinkURL(created));
    });

    test('creates a link that never ends, for a number of uses', async () => {
        jest.spyOn(Client4, 'getTeamInviteLinks').mockResolvedValue([]);
        const create = jest.spyOn(Client4, 'createTeamInviteLink').mockResolvedValue(link('ten', {max_uses: 10}));
        renderWithContext(<InviteLinksDialog onExited={jest.fn()}/>, state);

        await screen.findByText('No links yet.');
        await userEvent.selectOptions(screen.getByRole('combobox', {name: 'Expire after'}), 'Never');
        await userEvent.selectOptions(screen.getByRole('combobox', {name: 'Maximum number of uses'}), '10');
        await userEvent.click(screen.getByRole('button', {name: 'Create link'}));
        await waitFor(() => expect(create).toHaveBeenCalledWith('t1', {expire_at: 0, max_uses: 10}));
    });

    test('shows why the server refused a link', async () => {
        jest.spyOn(Client4, 'getTeamInviteLinks').mockResolvedValue([]);
        jest.spyOn(Client4, 'createTeamInviteLink').mockRejectedValue({message: 'This community has 200 invite links, the most it may have. Revoke some to make more.'});
        renderWithContext(<InviteLinksDialog onExited={jest.fn()}/>, state);

        await screen.findByText('No links yet.');
        await userEvent.click(screen.getByRole('button', {name: 'Create link'}));
        expect(await screen.findByRole('alert')).toHaveTextContent('the most it may have');
    });

    test('copying and revoking a link', async () => {
        jest.spyOn(Client4, 'getTeamInviteLinks').mockResolvedValue([link('a'), link('b')]);
        const revoke = jest.spyOn(Client4, 'revokeTeamInviteLink').mockResolvedValue({status: 'OK'});
        renderWithContext(<InviteLinksDialog onExited={jest.fn()}/>, state);

        await waitFor(() => expect(row('a')).toBeTruthy());
        await userEvent.click(within(row('a')).getByRole('button', {name: /Copy the link/}));
        expect(copyToClipboard).toHaveBeenCalledWith(inviteLinkURL(link('a')));

        await userEvent.click(within(row('a')).getByRole('button', {name: /Revoke the link/}));
        await waitFor(() => expect(revoke).toHaveBeenCalledWith('t1', 'a'));
        await waitFor(() => expect(row('a')).toBeNull());
        expect(row('b')).toBeTruthy();
    });
});
