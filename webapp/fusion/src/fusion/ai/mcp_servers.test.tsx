// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {AIMCPServer} from '@mattermost/types/ai_tools';

import {Client4} from 'mattermost-redux/client';

import {renderWithContext, screen, userEvent, within} from 'tests/react_testing_utils';

import MCPServers from './mcp_servers';

const server = (patch: Partial<AIMCPServer> = {}): AIMCPServer => ({
    id: 's1',
    name: 'Tracker',
    slug: 'tracker',
    scope: 'server',
    url: 'https://mcp.example.com/mcp',
    auth: 'oauth',
    oauth_client_id: '',
    oauth_client_secret_set: false,
    oauth_scopes: '',
    oauth_registered: false,
    header_names: [],
    sharing: {all_teams: true, team_ids: []},
    needs_attention: '',
    creator_id: 'me',
    updater_id: 'me',
    create_at: 1,
    update_at: 1,
    delete_at: 0,
    ...patch,
});

const state = {
    entities: {
        general: {config: {}},
        users: {currentUserId: 'me'},
        teams: {currentTeamId: 't1', teams: {t1: {id: 't1', name: 'lumen', display_name: 'Lumen', delete_at: 0}}},
    },
} as never;

describe('fusion/ai/MCPServers', () => {
    afterEach(() => {
        jest.restoreAllMocks();
    });

    test('lists the servers with their tools\' names, sign-in and scope', async () => {
        jest.spyOn(Client4, 'getAIMCPServers').mockResolvedValue([
            server(),
            server({id: 's2', name: 'Docs', slug: 'docs', scope: 't1', auth: 'service_account', header_names: ['Authorization'], sharing: {all_teams: false, team_ids: []}}),
        ]);
        renderWithContext(
            <MCPServers
                scope='server'
                listScope=''
            />,
            state,
        );
        const list = await screen.findByRole('list', {name: 'MCP servers'});
        const rows = within(list).getAllByRole('listitem');
        expect(rows[0]).toHaveTextContent('Docs');
        expect(rows[0]).toHaveTextContent('mcp.docs.*');
        expect(rows[0]).toHaveTextContent('Shared account (reads only) · Community: Lumen');
        expect(rows[1]).toHaveTextContent('Each person signs in · Server-wide, all communities');
    });

    test('adds a server: the short name follows the name, the secret is sent once', async () => {
        jest.spyOn(Client4, 'getAIMCPServers').mockResolvedValue([]);
        const create = jest.spyOn(Client4, 'createAIMCPServer').mockImplementation(async (req) => server({id: 'new', name: req.name, slug: req.slug}));
        renderWithContext(
            <MCPServers
                scope='server'
                listScope=''
            />,
            state,
        );
        await userEvent.click(await screen.findByRole('button', {name: 'Add an MCP server'}));
        await userEvent.type(screen.getByLabelText('Name'), 'Jira Cloud');
        expect(screen.getByDisplayValue('jira-cloud')).toBeInTheDocument();
        await userEvent.type(screen.getByPlaceholderText('https://mcp.example.com/mcp'), 'https://mcp.atlassian.example/v1/mcp');
        await userEvent.type(screen.getByLabelText('OAuth client secret'), 'client-secret-1');
        await userEvent.click(screen.getByRole('button', {name: 'Add server'}));
        expect(create).toHaveBeenCalledWith(expect.objectContaining({name: 'Jira Cloud', slug: 'jira-cloud', scope: 'server', auth: 'oauth', oauth_client_secret: 'client-secret-1', sharing: {all_teams: true, team_ids: []}}));
        expect(await screen.findByRole('status')).toHaveTextContent('Jira Cloud added.');
    });

    test('a shared account needs its headers before it can be saved', async () => {
        jest.spyOn(Client4, 'getAIMCPServers').mockResolvedValue([]);
        renderWithContext(
            <MCPServers
                scope='t1'
                listScope='t1'
            />,
            state,
        );
        await userEvent.click(await screen.findByRole('button', {name: 'Add an MCP server'}));
        await userEvent.type(screen.getByLabelText('Name'), 'Docs');
        await userEvent.type(screen.getByPlaceholderText('https://mcp.example.com/mcp'), 'https://docs.example.com/mcp');
        await userEvent.click(screen.getByRole('button', {name: 'Shared'}));
        expect(screen.getByRole('button', {name: 'Add server'})).toBeDisabled();
        await userEvent.type(screen.getByLabelText('Value of header 1'), 'Bearer read-only');
        expect(screen.getByRole('button', {name: 'Add server'})).toBeEnabled();
        expect(screen.queryByRole('switch', {name: 'All communities'})).not.toBeInTheDocument();
    });

    test('deleting asks first, in the page', async () => {
        jest.spyOn(Client4, 'getAIMCPServers').mockResolvedValue([server()]);
        const remove = jest.spyOn(Client4, 'deleteAIMCPServer').mockResolvedValue({status: 'OK'});
        renderWithContext(
            <MCPServers
                scope='server'
                listScope=''
            />,
            state,
        );
        await userEvent.click(await screen.findByRole('button', {name: 'Delete Tracker'}));
        expect(remove).not.toHaveBeenCalled();
        await userEvent.click(screen.getByRole('button', {name: 'Delete server'}));
        expect(remove).toHaveBeenCalledWith('s1');
        expect(await screen.findByRole('status')).toHaveTextContent('Tracker deleted, with everyone\'s sign-in to it.');
    });
});
