// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {Client4} from 'mattermost-redux/client';

import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import {attributesState, field} from './attributes_test_utils';
import UserAttributesDialog from './user_attributes_dialog';

let mockTeamPerms = new Set<string>();
let mockSystemPerms = new Set<string>();
jest.mock('mattermost-redux/selectors/entities/roles', () => ({
    ...jest.requireActual('mattermost-redux/selectors/entities/roles'),
    haveITeamPermission: jest.fn((state: unknown, teamId: string, permission: string) => mockTeamPerms.has(permission)),
    haveISystemPermission: jest.fn((state: unknown, {permission}: {permission: string}) => mockSystemPerms.has(permission)),
}));
jest.mock('mattermost-redux/actions/users', () => ({
    ...jest.requireActual('mattermost-redux/actions/users'),
    getCustomProfileAttributeValues: jest.fn(() => ({type: 'MOCK_CPA_VALUES'})),
}));

// Each test has its own community: the attributes are cached by community.
let team = 0;
const LOCKED = 'Set by the community\'s admins.';

describe('fusion/attributes/UserAttributesDialog', () => {
    beforeEach(() => {
        mockTeamPerms = new Set();
        mockSystemPerms = new Set();
        team++;
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
    });

    const community = [
        field('company', {managed: 'admin'}),
        field('site'),
        field('salary', {visibility: 'admin'}),
    ];

    const render = (userId: string, serverFields: Record<string, unknown> = {}, users: Record<string, unknown> = {}) => {
        jest.spyOn(Client4, 'getCommunityAttributes').mockResolvedValue(community);
        jest.spyOn(Client4, 'getCommunityAttributeValues').mockResolvedValue({[userId]: {company: 'Lumen', site: 'Paris'}});
        jest.spyOn(Client4, 'patchCommunityAttributeValues').mockImplementation(async (t, u, values) => values as never);
        jest.spyOn(Client4, 'updateUserCustomProfileAttributesValues').mockImplementation(async (u, values) => values as never);
        const toast = jest.fn();
        const onHide = jest.fn();
        renderWithContext(
            <UserAttributesDialog
                userId={userId}
                teamId={`team${team}`}
                toast={toast}
                onHide={onHide}
            />,
            attributesState(`team${team}`, serverFields, users) as never,
        );
        return {toast, onHide};
    };

    test('a member sets only the attributes the admins leave to them', async () => {
        render('me');
        expect(await screen.findByRole('textbox', {name: 'site'})).toHaveValue('Paris');

        // Set by admins: shown, not editable.
        expect(screen.queryByRole('textbox', {name: 'company'})).not.toBeInTheDocument();
        expect(screen.getByText('Lumen')).toBeInTheDocument();
        expect(screen.getAllByText(LOCKED)).toHaveLength(1);

        // An attribute only admins see is still the member's to set, and they're told who sees it.
        expect(screen.getByRole('textbox', {name: 'salary'})).toBeInTheDocument();
        expect(screen.getByText('Only admins and me see it.')).toBeInTheDocument();
        expect(screen.queryByText('Everywhere on the server')).not.toBeInTheDocument();
    });

    test('a member can\'t set someone else\'s attributes', async () => {
        render('sofia');
        await screen.findByText('In Lumen');
        expect(screen.queryAllByRole('textbox')).toHaveLength(0);
        expect(screen.getAllByText(LOCKED)).toHaveLength(3);
    });

    test('a community admin sets anyone\'s, and saves only what changed', async () => {
        mockTeamPerms = new Set(['manage_team']);
        const {toast, onHide} = render('sofia');
        const site = await screen.findByRole('textbox', {name: 'site'});
        expect(screen.getByRole('textbox', {name: 'company'})).toBeInTheDocument();
        expect(screen.getByText('Only admins and sofia see it.')).toBeInTheDocument();

        const save = screen.getByRole('button', {name: 'Save'});
        expect(save).toBeDisabled();

        await userEvent.clear(site);
        await userEvent.type(site, 'Lyon');
        await userEvent.click(save);

        await waitFor(() => expect(Client4.patchCommunityAttributeValues).toHaveBeenCalledWith(`team${team}`, 'sofia', {site: 'Lyon'}));
        expect(Client4.updateUserCustomProfileAttributesValues).not.toHaveBeenCalled();
        expect(toast).toHaveBeenCalledWith('Saved sofia\'s attributes');
        expect(onHide).toHaveBeenCalled();
    });

    test('tells why saving failed and stays open', async () => {
        mockTeamPerms = new Set(['manage_team']);
        const {onHide} = render('sofia');
        jest.mocked(Client4.patchCommunityAttributeValues).mockRejectedValueOnce({message: 'Value too long'});
        await userEvent.type(await screen.findByRole('textbox', {name: 'site'}), '!');
        await userEvent.click(screen.getByRole('button', {name: 'Save'}));
        expect(await screen.findByRole('alert')).toHaveTextContent('Value too long');
        expect(onHide).not.toHaveBeenCalled();
    });

    test('a user administrator sets the server\'s attributes, except those a provider or an integration sets', async () => {
        mockSystemPerms = new Set(['edit_other_users']);
        const serverFields = {
            dept: {id: 'dept', name: 'Department', type: 'text', attrs: {sort_order: 0}},
            ldap: {id: 'ldap', name: 'Employee ID', type: 'text', attrs: {sort_order: 1, ldap: 'employeeID'}},
            bot: {id: 'bot', name: 'Score', type: 'text', attrs: {sort_order: 2, protected: true}},
            raw: {id: 'raw', name: 'Raw', type: 'text', attrs: {sort_order: 3, access_mode: 'source_only'}},
        };
        const ldapUser = {sofia: TestHelper.getUserMock({id: 'sofia', username: 'sofia', first_name: '', last_name: '', nickname: '', auth_service: 'ldap'})};
        render('sofia', serverFields, ldapUser);

        expect(await screen.findByText('Everywhere on the server')).toBeInTheDocument();
        const dept = screen.getByRole('textbox', {name: 'Department'});
        expect(screen.queryByRole('textbox', {name: 'Employee ID'})).not.toBeInTheDocument();
        expect(screen.getByText('Set by their login provider.')).toBeInTheDocument();
        expect(screen.getByText('Managed by an integration.')).toBeInTheDocument();
        expect(screen.queryByText('Raw')).not.toBeInTheDocument();

        await userEvent.type(dept, 'R&D');
        await userEvent.click(screen.getByRole('button', {name: 'Save'}));
        await waitFor(() => expect(Client4.updateUserCustomProfileAttributesValues).toHaveBeenCalledWith('sofia', {dept: 'R&D'}));
        expect(Client4.patchCommunityAttributeValues).not.toHaveBeenCalled();
    });

    test('an attribute synced from OpenID Connect is set by the provider of a user who signs in with it', async () => {
        mockSystemPerms = new Set(['edit_other_users']);
        const serverFields = {
            dept: {id: 'dept', name: 'Department', type: 'text', attrs: {sort_order: 0, openid: 'org.department'}},
        };
        const openIdUser = {sofia: TestHelper.getUserMock({id: 'sofia', username: 'sofia', first_name: '', last_name: '', nickname: '', auth_service: 'openid'})};
        render('sofia', serverFields, openIdUser);

        expect(await screen.findByText('Set by their login provider.')).toBeInTheDocument();
        expect(screen.queryByRole('textbox', {name: 'Department'})).not.toBeInTheDocument();
    });
});
