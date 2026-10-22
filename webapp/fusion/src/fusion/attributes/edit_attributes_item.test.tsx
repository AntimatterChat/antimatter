// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {Client4} from 'mattermost-redux/client';

import {openDialog} from 'fusion/utils/modals';
import {act, renderWithContext, screen, userEvent} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import {attributesState, field} from './attributes_test_utils';
import EditAttributesItem, {USER_ATTRIBUTES_DIALOG} from './edit_attributes_item';

let mockTeamPerms = new Set<string>();
let mockSystemPerms = new Set<string>();
jest.mock('mattermost-redux/selectors/entities/roles', () => ({
    ...jest.requireActual('mattermost-redux/selectors/entities/roles'),
    haveITeamPermission: jest.fn((state: unknown, teamId: string, permission: string) => mockTeamPerms.has(permission)),
    haveISystemPermission: jest.fn((state: unknown, {permission}: {permission: string}) => mockSystemPerms.has(permission)),
}));
jest.mock('fusion/utils/modals', () => ({openDialog: jest.fn(() => ({type: 'MOCK_OPEN_DIALOG'}))}));
jest.mock('fusion/shell/toast_context', () => ({useToast: () => jest.fn()}));

const ITEM = 'Edit attributes…';

// Lets the community's attributes load.
const settle = () => act(() => new Promise((resolve) => setTimeout(resolve, 0)));

// Each test has its own community: the attributes are cached by community.
let team = 0;

describe('fusion/attributes/EditAttributesItem', () => {
    beforeEach(() => {
        mockTeamPerms = new Set();
        mockSystemPerms = new Set();
        team++;
    });

    const render = (userId: string, fields: Array<ReturnType<typeof field>>, serverFields: Record<string, unknown> = {}, users: Record<string, unknown> = {}, bots: Record<string, unknown> = {}) => {
        jest.spyOn(Client4, 'getCommunityAttributes').mockResolvedValue(fields);
        jest.spyOn(Client4, 'getCommunityAttributeValues').mockResolvedValue({});
        const onDone = jest.fn();
        renderWithContext(
            <EditAttributesItem
                userId={userId}
                onDone={onDone}
            />,
            attributesState(`team${team}`, serverFields, users, bots) as never,
        );
        return onDone;
    };
    const department = {f1: {id: 'f1', name: 'Department', type: 'text', attrs: {}}};

    test('a member can\'t edit someone else\'s attributes', async () => {
        render('sofia', [field('company')]);
        await settle();
        expect(screen.queryByRole('menuitem', {name: ITEM})).not.toBeInTheDocument();
    });

    test('a member edits their own attributes when some are theirs to set', async () => {
        render('me', [field('company', {managed: 'admin'}), field('site')]);
        expect(await screen.findByRole('menuitem', {name: ITEM})).toBeInTheDocument();
    });

    test('a member has nothing to edit when the admins set every attribute', async () => {
        render('me', [field('company', {managed: 'admin'})]);
        await settle();
        expect(screen.queryByRole('menuitem', {name: ITEM})).not.toBeInTheDocument();
    });

    test('a community admin edits anyone\'s attributes, and it opens the dialog', async () => {
        mockTeamPerms = new Set(['manage_team']);
        const onDone = render('sofia', [field('company', {managed: 'admin'})]);
        await userEvent.click(await screen.findByRole('menuitem', {name: ITEM}));
        expect(onDone).toHaveBeenCalled();
        expect(openDialog).toHaveBeenCalledWith(USER_ATTRIBUTES_DIALOG, expect.anything(), expect.objectContaining({userId: 'sofia', teamId: `team${team}`}));
    });

    test('a community admin has nothing to edit in a community without attributes', async () => {
        mockTeamPerms = new Set(['manage_team']);
        render('sofia', []);
        await settle();
        expect(screen.queryByRole('menuitem', {name: ITEM})).not.toBeInTheDocument();
    });

    test('a user administrator edits the server\'s attributes, not those only an integration reads', async () => {
        mockSystemPerms = new Set(['edit_other_users']);
        render('sofia', [], {f1: {id: 'f1', name: 'Badge', type: 'text', attrs: {access_mode: 'source_only'}}});
        await settle();
        expect(screen.queryByRole('menuitem', {name: ITEM})).not.toBeInTheDocument();
    });

    test('a user administrator edits the server\'s attributes', async () => {
        mockSystemPerms = new Set(['edit_other_users']);
        render('sofia', [], {f1: {id: 'f1', name: 'Department', type: 'text', attrs: {}}});
        expect(await screen.findByRole('menuitem', {name: ITEM})).toBeInTheDocument();
    });

    test('a user administrator doesn\'t edit a system admin\'s server attributes', async () => {
        mockSystemPerms = new Set(['edit_other_users']);
        render('sofia', [], department, {sofia: TestHelper.getUserMock({id: 'sofia', username: 'sofia', roles: 'system_user system_admin'})});
        await settle();
        expect(screen.queryByRole('menuitem', {name: ITEM})).not.toBeInTheDocument();
    });

    test('a bot\'s owner who may manage their bots edits its server attributes', async () => {
        mockSystemPerms = new Set(['manage_bots']);
        render('bot', [], department, {}, {bot: {user_id: 'bot', owner_id: 'me'}});
        expect(await screen.findByRole('menuitem', {name: ITEM})).toBeInTheDocument();
    });

    test('a bot\'s owner without manage_bots doesn\'t', async () => {
        mockSystemPerms = new Set(['read_bots']);
        render('bot', [], department, {}, {bot: {user_id: 'bot', owner_id: 'me'}});
        await settle();
        expect(screen.queryByRole('menuitem', {name: ITEM})).not.toBeInTheDocument();
    });

    test('manage_bots doesn\'t reach someone else\'s bot', async () => {
        mockSystemPerms = new Set(['manage_bots']);
        render('bot', [], department, {}, {bot: {user_id: 'bot', owner_id: 'sofia'}});
        await settle();
        expect(screen.queryByRole('menuitem', {name: ITEM})).not.toBeInTheDocument();
    });

    test('manage_others_bots reaches any bot, an agent\'s included', async () => {
        mockSystemPerms = new Set(['manage_others_bots']);
        render('bot', [], department, {}, {bot: {user_id: 'bot', owner_id: 'antimatter-agents'}});
        expect(await screen.findByRole('menuitem', {name: ITEM})).toBeInTheDocument();
    });

    test('the bot is loaded to learn its owner', async () => {
        mockSystemPerms = new Set(['manage_bots']);
        jest.spyOn(Client4, 'getBot').mockResolvedValue({user_id: 'bot', owner_id: 'me'} as never);
        render('bot', [], department);
        expect(await screen.findByRole('menuitem', {name: ITEM})).toBeInTheDocument();
        expect(Client4.getBot).toHaveBeenCalledWith('bot');
    });

    test('bots have no community attributes to edit', async () => {
        mockTeamPerms = new Set(['manage_team']);
        render('bot', [field('company')], department);
        await settle();
        expect(screen.queryByRole('menuitem', {name: ITEM})).not.toBeInTheDocument();
    });
});
