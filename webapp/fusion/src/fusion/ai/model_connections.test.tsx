// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {AIConnection} from '@mattermost/types/ai_connections';

import {Client4} from 'mattermost-redux/client';

import {renderWithContext, screen, userEvent, waitFor, within} from 'tests/react_testing_utils';

import ModelConnections from './model_connections';

jest.mock('fusion/teams/teams', () => ({
    ...jest.requireActual('fusion/teams/teams'),
    loadTeams: jest.fn(() => ({type: 'MOCK_LOAD_TEAMS'})),
}));
jest.mock('mattermost-redux/actions/teams', () => ({
    ...jest.requireActual('mattermost-redux/actions/teams'),
    getTeams: jest.fn(() => ({type: 'MOCK_GET_TEAMS'})),
}));

const connection = (patch: Partial<AIConnection> = {}): AIConnection => ({
    id: 'conn1',
    name: 'Lumen OpenAI',
    scope: 'server',
    provider: 'openai_compatible',
    base_url: 'https://api.openai.com/v1',
    api_key_set: true,
    api_key_last4: 'wxyz',
    extra_header_names: [],
    models: {chat: 'gpt-5', embeddings: '', transcription: '', decisions: ''},
    decisions_dialect: 'openai',
    limits: {requests_per_minute: 0, monthly_token_budget: 0, per_user_requests_per_minute: 0, per_user_monthly_token_budget: 0},
    sharing: {all_teams: true, team_ids: []},
    audience: {everyone: true, group_ids: [], roles: [], allow_guests: false},
    needs_attention: '',
    creator_id: 'me',
    updater_id: 'me',
    create_at: 1,
    update_at: 1,
    delete_at: 0,
    ...patch,
});

const usage = {connection_id: 'conn1', month: 202610, totals: {requests: 140, input_tokens: 10000, output_tokens: 2300}, monthly_token_budget: 0, days: [], by_purpose: {}};

const state = {
    entities: {
        general: {config: {}},
        users: {currentUserId: 'me'},
        teams: {
            currentTeamId: 't1',
            teams: {
                t1: {id: 't1', name: 'lumen', display_name: 'Lumen', delete_at: 0},
                t2: {id: 't2', name: 'labs', display_name: 'Labs', delete_at: 0},
            },
        },
        groups: {groups: {
            design: {id: 'design', display_name: 'Design', source: 'custom', team_id: 't1', is_team: true, delete_at: 0},
            mods: {id: 'mods', display_name: 'Moderators', source: 'custom', team_id: 't1', delete_at: 0, position: 0},
        }},
    },
} as never;

describe('fusion/ai/ModelConnections', () => {
    let spies: jest.SpyInstance[] = [];

    // Spies on a Client4 method; the mock's calls are read as the requests sent.
    type Impl = (...args: never[]) => unknown;
    const spy = (name: keyof typeof Client4, impl: (...args: never[]) => unknown) => {
        const s = jest.spyOn(Client4 as unknown as Record<string, Impl>, name as string).mockImplementation(impl) as jest.SpyInstance;
        spies.push(s);
        return s;
    };

    beforeEach(() => {
        spies = [];
        spy('getAIConnectionUsage', async () => usage);
        spy('getAIMCPServers', async () => []);
    });
    afterEach(() => {
        spies.forEach((s) => s.mockRestore());
    });

    const renderServer = () => renderWithContext(
        <ModelConnections
            scope='server'
            listScope=''
        />,
        state,
    );

    test('lists the connections with their provider, scope, usage and what needs attention', async () => {
        spy('getAIConnections', async () => [
            connection(),
            connection({id: 'conn2', name: 'Old Bedrock', scope: 't1', provider: 'anthropic', needs_attention: 'The AI chat app\'s Bedrock service: Antimatter speaks no Bedrock API. Use its OpenAI-compatible endpoint.'}),
        ]);
        renderServer();

        const list = await screen.findByRole('list', {name: 'Model connections'});
        const rows = within(list).getAllByRole('listitem');
        expect(rows[0]).toHaveTextContent('Lumen OpenAI');
        expect(rows[0]).toHaveTextContent('OpenAI-compatible · Server-wide, all communities');
        await waitFor(() => expect(rows[0]).toHaveTextContent('This month: 12.3K tokens, 140 requests'));
        expect(rows[1]).toHaveTextContent('Anthropic · Community: Lumen');
        expect(within(rows[1]).getByText('Needs attention')).toBeInTheDocument();
        expect(rows[1]).toHaveTextContent('Antimatter speaks no Bedrock API');
        expect(within(rows[0]).queryByText('Needs attention')).not.toBeInTheDocument();
    });

    test('adds a server-wide connection, shared with every community by default, guests off', async () => {
        spy('getAIConnections', async () => []);
        const create = spy('createAIConnection', async (req: {name: string}) => connection({id: 'new1', name: req.name}));
        renderServer();

        await userEvent.click(await screen.findByRole('button', {name: 'Add a connection'}));
        await userEvent.click(screen.getByRole('button', {name: 'Mistral'}));
        expect(screen.getByDisplayValue('https://api.mistral.ai/v1')).toBeInTheDocument();
        expect(screen.getByDisplayValue('Mistral')).toBeInTheDocument();
        await userEvent.type(screen.getByLabelText('API key'), 'sk-secret-1234');
        expect(screen.getByRole('switch', {name: 'All communities'})).toHaveAttribute('aria-checked', 'true');
        expect(screen.getByRole('switch', {name: 'Guests too'})).toHaveAttribute('aria-checked', 'false');

        await userEvent.click(screen.getByRole('button', {name: 'Add connection'}));
        await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
        expect(create.mock.calls[0][0]).toMatchObject({
            name: 'Mistral',
            scope: 'server',
            provider: 'openai_compatible',
            base_url: 'https://api.mistral.ai/v1',
            api_key: 'sk-secret-1234',
            sharing: {all_teams: true, team_ids: []},
            audience: {everyone: true, group_ids: [], roles: [], allow_guests: false},
        });

        // Saved, back to the list.
        expect(await screen.findByRole('status')).toHaveTextContent('Mistral added.');
        expect(within(screen.getByRole('list', {name: 'Model connections'})).getByText('Mistral')).toBeInTheDocument();
    });

    test('tests a new connection before it\'s saved, with what the form holds', async () => {
        spy('getAIConnections', async () => []);
        const testRequest = spy('testAIConnectionRequest', async () => ({ok: true, latency_ms: 120, model_count: 3}));
        const create = spy('createAIConnection', async () => connection());
        renderServer();

        await userEvent.click(await screen.findByRole('button', {name: 'Add a connection'}));
        await userEvent.click(screen.getByRole('button', {name: 'OpenAI'}));
        await userEvent.type(screen.getByLabelText('API key'), 'sk-test');
        await userEvent.click(screen.getByRole('button', {name: 'Test connection'}));

        await waitFor(() => expect(testRequest).toHaveBeenCalledTimes(1));
        expect(testRequest.mock.calls[0][0]).toMatchObject({base_url: 'https://api.openai.com/v1', api_key: 'sk-test', scope: 'server'});
        expect(await screen.findByText(/It works: 3 models in 120 ms/)).toBeInTheDocument();
        expect(create).not.toHaveBeenCalled();
    });

    test('shows why a test failed, as the endpoint said', async () => {
        spy('getAIConnections', async () => []);
        spy('testAIConnectionRequest', async () => ({ok: false, error: 'Incorrect API key provided', status_code: 401, latency_ms: 80, model_count: 0}));
        renderServer();

        await userEvent.click(await screen.findByRole('button', {name: 'Add a connection'}));
        await userEvent.click(screen.getByRole('button', {name: 'Test connection'}));
        expect(await screen.findByRole('alert')).toHaveTextContent('It doesn\'t work (HTTP 401): Incorrect API key provided');
    });

    test('edits a connection without retyping its key', async () => {
        spy('getAIConnections', async () => [connection()]);
        const patch = spy('patchAIConnection', async (_id: string, p: {name: string}) => connection({name: p.name}));
        renderServer();

        await userEvent.click(await screen.findByRole('button', {name: 'Edit Lumen OpenAI'}));
        expect(screen.getByText('Set, ends in ••wxyz')).toBeInTheDocument();
        expect(screen.queryByLabelText('New API key')).not.toBeInTheDocument();
        const name = screen.getByDisplayValue('Lumen OpenAI');
        await userEvent.clear(name);
        await userEvent.type(name, 'Lumen GPT');
        await userEvent.click(screen.getByRole('button', {name: 'Save'}));

        await waitFor(() => expect(patch).toHaveBeenCalledTimes(1));
        expect(patch.mock.calls[0][0]).toBe('conn1');
        expect(patch.mock.calls[0][1].name).toBe('Lumen GPT');
        expect(patch.mock.calls[0][1]).not.toHaveProperty('api_key');
        expect(patch.mock.calls[0][1]).not.toHaveProperty('extra_headers');
        expect(patch.mock.calls[0][1]).not.toHaveProperty('scope');
        expect(await screen.findByRole('status')).toHaveTextContent('Lumen GPT saved.');
    });

    test('replaces the key', async () => {
        spy('getAIConnections', async () => [connection()]);
        const patch = spy('patchAIConnection', async () => connection());
        renderServer();

        await userEvent.click(await screen.findByRole('button', {name: 'Edit Lumen OpenAI'}));
        await userEvent.click(screen.getByRole('button', {name: 'Replace'}));
        await userEvent.type(screen.getByLabelText('New API key'), 'sk-new-key');
        await userEvent.click(screen.getByRole('button', {name: 'Save'}));

        await waitFor(() => expect(patch).toHaveBeenCalledTimes(1));
        expect(patch.mock.calls[0][1].api_key).toBe('sk-new-key');
    });

    test('Replace with nothing typed keeps the key', async () => {
        spy('getAIConnections', async () => [connection()]);
        const patch = spy('patchAIConnection', async () => connection());
        renderServer();

        await userEvent.click(await screen.findByRole('button', {name: 'Edit Lumen OpenAI'}));
        await userEvent.click(screen.getByRole('button', {name: 'Replace'}));
        await userEvent.click(screen.getByRole('button', {name: 'Save'}));

        await waitFor(() => expect(patch).toHaveBeenCalledTimes(1));
        expect(patch.mock.calls[0][1]).not.toHaveProperty('api_key');
    });

    test('removes the key', async () => {
        spy('getAIConnections', async () => [connection()]);
        const patch = spy('patchAIConnection', async () => connection({api_key_set: false, api_key_last4: ''}));
        renderServer();

        await userEvent.click(await screen.findByRole('button', {name: 'Edit Lumen OpenAI'}));
        await userEvent.click(screen.getByRole('button', {name: 'Remove'}));
        expect(screen.getByText('The key is removed when you save.')).toBeInTheDocument();
        await userEvent.click(screen.getByRole('button', {name: 'Save'}));

        await waitFor(() => expect(patch).toHaveBeenCalledTimes(1));
        expect(patch.mock.calls[0][1].api_key).toBe('');
    });

    test('tests a saved connection as saved, with its own key', async () => {
        spy('getAIConnections', async () => [connection()]);
        const testSaved = spy('testAIConnection', async () => ({ok: true, latency_ms: 40, model_count: 12}));
        const testRequest = spy('testAIConnectionRequest', async () => ({ok: true, latency_ms: 1, model_count: 0}));
        renderServer();

        await userEvent.click(await screen.findByRole('button', {name: 'Edit Lumen OpenAI'}));
        await userEvent.click(screen.getByRole('button', {name: 'Test connection'}));
        await waitFor(() => expect(testSaved).toHaveBeenCalledWith('conn1'));
        expect(testRequest).not.toHaveBeenCalled();
        expect(await screen.findByText(/It works: 12 models in 40 ms/)).toBeInTheDocument();
    });

    test('a new address can\'t be tested with the saved key, which stays on the server', async () => {
        spy('getAIConnections', async () => [connection()]);
        const testSaved = spy('testAIConnection', async () => ({ok: true, latency_ms: 1, model_count: 0}));
        const testRequest = spy('testAIConnectionRequest', async () => ({ok: true, latency_ms: 1, model_count: 0}));
        renderServer();

        await userEvent.click(await screen.findByRole('button', {name: 'Edit Lumen OpenAI'}));
        await userEvent.click(screen.getByRole('button', {name: 'OpenRouter'}));
        await userEvent.click(screen.getByRole('button', {name: 'Test connection'}));
        expect(await screen.findByText(/save first, or type the key again/)).toBeInTheDocument();
        expect(testSaved).not.toHaveBeenCalled();
        expect(testRequest).not.toHaveBeenCalled();
    });

    test('loads the models of a saved connection', async () => {
        spy('getAIConnections', async () => [connection()]);
        const models = spy('getAIConnectionModels', async () => [{id: 'gpt-5'}, {id: 'text-embedding-3-small'}]);
        renderServer();

        await userEvent.click(await screen.findByRole('button', {name: 'Edit Lumen OpenAI'}));
        await userEvent.click(screen.getByRole('button', {name: 'Load models'}));
        await waitFor(() => expect(models).toHaveBeenCalledWith('conn1'));
        expect(await screen.findByText('2 models: pick them as you type.')).toBeInTheDocument();
    });

    test('loads the models of a new connection before it\'s saved, with what the form holds', async () => {
        spy('getAIConnections', async () => []);
        const unsaved = spy('getAIConnectionRequestModels', async () => [{id: 'mistral-large'}, {id: 'mistral-small'}]);
        const saved = spy('getAIConnectionModels', async () => []);
        const create = spy('createAIConnection', async () => connection());
        renderServer();

        await userEvent.click(await screen.findByRole('button', {name: 'Add a connection'}));
        await userEvent.click(screen.getByRole('button', {name: 'Mistral'}));
        await userEvent.type(screen.getByLabelText('API key'), 'sk-mistral');
        await userEvent.click(screen.getByRole('button', {name: 'Load models'}));

        await waitFor(() => expect(unsaved).toHaveBeenCalledTimes(1));
        expect(unsaved.mock.calls[0][0]).toMatchObject({scope: 'server', base_url: 'https://api.mistral.ai/v1', api_key: 'sk-mistral'});
        expect(await screen.findByText('2 models: pick them as you type.')).toBeInTheDocument();
        expect(document.querySelectorAll('datalist option')).toHaveLength(2);
        expect(saved).not.toHaveBeenCalled();
        expect(create).not.toHaveBeenCalled();
    });

    test('loads a changed connection\'s models with the form\'s key, never the saved one', async () => {
        spy('getAIConnections', async () => [connection()]);
        const unsaved = spy('getAIConnectionRequestModels', async () => [{id: 'gpt-x'}]);
        const saved = spy('getAIConnectionModels', async () => []);
        renderServer();

        await userEvent.click(await screen.findByRole('button', {name: 'Edit Lumen OpenAI'}));
        await userEvent.click(screen.getByRole('button', {name: 'OpenRouter'}));
        await userEvent.click(screen.getByRole('button', {name: 'Load models'}));
        expect(await screen.findByText(/to load the models of the new address with them, save first/)).toBeInTheDocument();
        expect(unsaved).not.toHaveBeenCalled();

        await userEvent.click(screen.getByRole('button', {name: 'Replace'}));
        await userEvent.type(screen.getByLabelText('New API key'), 'sk-router');
        await userEvent.click(screen.getByRole('button', {name: 'Load models'}));
        await waitFor(() => expect(unsaved).toHaveBeenCalledTimes(1));
        expect(unsaved.mock.calls[0][0]).toMatchObject({base_url: 'https://openrouter.ai/api/v1', api_key: 'sk-router'});
        expect(saved).not.toHaveBeenCalled();
    });

    test('asks in the page before deleting', async () => {
        spy('getAIConnections', async () => [connection()]);
        const remove = spy('deleteAIConnection', async () => ({status: 'OK'}));
        const confirm = jest.spyOn(window, 'confirm');
        renderServer();

        await userEvent.click(await screen.findByRole('button', {name: 'Delete Lumen OpenAI'}));
        expect(screen.getByRole('group', {name: 'Delete Lumen OpenAI?'})).toHaveTextContent('This can\'t be undone.');
        expect(remove).not.toHaveBeenCalled();

        await userEvent.click(screen.getByRole('button', {name: 'Cancel'}));
        expect(screen.queryByRole('group', {name: 'Delete Lumen OpenAI?'})).not.toBeInTheDocument();
        expect(remove).not.toHaveBeenCalled();

        await userEvent.click(screen.getByRole('button', {name: 'Delete Lumen OpenAI'}));
        await userEvent.click(screen.getByRole('button', {name: 'Delete connection'}));
        await waitFor(() => expect(remove).toHaveBeenCalledWith('conn1'));
        expect(await screen.findByRole('status')).toHaveTextContent('Lumen OpenAI deleted.');
        expect(screen.queryByRole('listitem')).not.toBeInTheDocument();
        expect(confirm).not.toHaveBeenCalled();
        confirm.mockRestore();
    });

    test('shows the server\'s message when a change is refused', async () => {
        spy('getAIConnections', async () => [connection()]);
        spy('patchAIConnection', async () => {
            throw Object.assign(new Error('Invalid base URL.'), {server_error_id: 'model.ai_connection.base_url.app_error', status_code: 400});
        });
        renderServer();

        await userEvent.click(await screen.findByRole('button', {name: 'Edit Lumen OpenAI'}));
        await userEvent.click(screen.getByRole('button', {name: 'Save'}));
        expect(await screen.findByRole('alert')).toHaveTextContent('Invalid base URL.');
    });

    test('shows the usage of the month, by use and by day', async () => {
        spy('getAIConnections', async () => [connection({limits: {requests_per_minute: 0, monthly_token_budget: 100000, per_user_requests_per_minute: 0, per_user_monthly_token_budget: 0}})]);
        spy('getAIConnectionUsage', async () => ({
            ...usage,
            monthly_token_budget: 100000,
            by_purpose: {rewrite: {requests: 40, input_tokens: 2000, output_tokens: 300}, user: {requests: 100, input_tokens: 8000, output_tokens: 2000}},
            days: [
                {connection_id: 'conn1', user_id: '', purpose: 'rewrite', day: 20261016, requests: 40, input_tokens: 2000, output_tokens: 300},
                {connection_id: 'conn1', user_id: 'u1', purpose: 'user', day: 20261017, requests: 100, input_tokens: 8000, output_tokens: 2000},
            ],
        }));
        renderServer();

        await userEvent.click(await screen.findByRole('button', {name: 'Usage of Lumen OpenAI'}));
        const view = await screen.findByRole('region', {name: /Usage in October 2026/});
        expect(view).toHaveTextContent('140');
        expect(within(view).getByRole('meter', {name: 'Monthly token budget'})).toHaveAttribute('aria-valuenow', '12300');
        const purposes = within(view).getAllByRole('row');
        expect(purposes[0]).toHaveTextContent('People, through apps');
        expect(purposes[1]).toHaveTextContent('Rewrite');
        expect(within(view).getAllByRole('listitem')).toHaveLength(2);
    });

    describe('a community\'s own', () => {
        const renderCommunity = () => renderWithContext(
            <ModelConnections
                scope='t1'
                listScope='t1'
            />,
            state,
        );

        test('lists only the community\'s, and adds them with its scope, without sharing', async () => {
            const list = spy('getAIConnections', async () => []);
            const create = spy('createAIConnection', async () => connection({scope: 't1'}));
            renderCommunity();
            await waitFor(() => expect(list).toHaveBeenCalledWith('t1'));

            await userEvent.click(await screen.findByRole('button', {name: 'Add a connection'}));
            expect(screen.queryByRole('switch', {name: 'All communities'})).not.toBeInTheDocument();

            // The server's own network is out of a community's reach.
            expect(screen.queryByRole('button', {name: 'Ollama on this server'})).not.toBeInTheDocument();

            await userEvent.type(screen.getByPlaceholderText('e.g. Company OpenAI'), 'Our key');
            await userEvent.click(screen.getByRole('button', {name: 'Add connection'}));
            await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
            expect(create.mock.calls[0][0]).toMatchObject({scope: 't1', sharing: {all_teams: false, team_ids: []}});
        });

        test('its audience names only the community\'s teams and roles', async () => {
            spy('getAIConnections', async () => []);
            const create = spy('createAIConnection', async () => connection({scope: 't1'}));
            renderCommunity();

            await userEvent.click(await screen.findByRole('button', {name: 'Add a connection'}));
            await userEvent.type(screen.getByPlaceholderText('e.g. Company OpenAI'), 'Ours');
            await userEvent.click(screen.getByRole('switch', {name: 'Everyone'}));
            const chosen = screen.getByRole('group', {name: 'Teams, roles and admins chosen'});
            expect(within(chosen).getByRole('button', {name: 'Design'})).toBeInTheDocument();
            expect(within(chosen).queryByRole('button', {name: 'System admins'})).not.toBeInTheDocument();
            await userEvent.click(within(chosen).getByRole('button', {name: 'Moderators'}));
            await userEvent.click(screen.getByRole('button', {name: 'Add connection'}));

            await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
            expect(create.mock.calls[0][0].audience).toEqual({everyone: false, group_ids: ['mods'], roles: [], allow_guests: false});
        });

        test('editing one sends no sharing', async () => {
            spy('getAIConnections', async () => [connection({scope: 't1', sharing: {all_teams: false, team_ids: []}})]);
            const patch = spy('patchAIConnection', async () => connection({scope: 't1'}));
            renderCommunity();

            await userEvent.click(await screen.findByRole('button', {name: 'Edit Lumen OpenAI'}));
            await userEvent.click(screen.getByRole('button', {name: 'Save'}));
            await waitFor(() => expect(patch).toHaveBeenCalledTimes(1));
            expect(patch.mock.calls[0][1]).not.toHaveProperty('sharing');
        });
    });
});
