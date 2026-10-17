// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {AIAgentDetails} from '@mattermost/types/ai_agents';

import {Client4} from 'mattermost-redux/client';

import {renderWithContext, screen, userEvent, waitFor, within} from 'tests/react_testing_utils';

import AgentsSettings from './agents_settings';

jest.mock('mattermost-redux/actions/teams', () => ({
    ...jest.requireActual('mattermost-redux/actions/teams'),
    getTeams: jest.fn(() => ({type: 'MOCK_GET_TEAMS'})),
}));

const agent = (patch: Partial<AIAgentDetails> = {}): AIAgentDetails => ({
    user_id: 'researcher1',
    scope: 'server',
    external: false,
    connection_id: 'conn1',
    model: 'gpt-5',
    system_prompt: 'You research.',
    tools: {enabled: false, allowed: [], auto_run: []},
    reasoning: {enabled: false, effort: '', budget_tokens: 0},
    max_tool_turns: 0,
    access: {everyone: true, team_ids: [], channel_ids: [], roles: [], user_ids: [], allow_guests: false},
    needs_attention: '',
    creator_id: 'me',
    updater_id: 'me',
    create_at: 1,
    update_at: 1,
    delete_at: 0,
    username: 'researcher',
    display_name: 'Researcher',
    description: '',
    last_icon_update: 0,
    enabled: true,
    connection_name: 'Lumen OpenAI',
    ...patch,
});

const state = {
    entities: {
        general: {config: {SiteURL: 'https://chat.example.com'}},
        users: {currentUserId: 'me'},
        teams: {currentTeamId: 't1', teams: {t1: {id: 't1', name: 'lumen', display_name: 'Lumen', delete_at: 0}}},
        channels: {channels: {
            ch1: {id: 'ch1', team_id: 't1', type: 'O', name: 'roadmap', display_name: 'Roadmap', delete_at: 0},
            dm: {id: 'dm', team_id: '', type: 'D', name: 'a__b', display_name: '', delete_at: 0},
        }},
    },
} as never;

const connections = [{id: 'conn1', name: 'Lumen OpenAI', scope: 'server', provider: 'openai_compatible', models: {chat: 'gpt-5', embeddings: '', transcription: '', decisions: ''}, capabilities: ['chat']}];

describe('fusion/agents/AgentsSettings', () => {
    let spies: jest.SpyInstance[] = [];
    type Impl = (...args: never[]) => unknown;
    const spy = (name: keyof typeof Client4, impl: Impl) => {
        const s = jest.spyOn(Client4 as unknown as Record<string, Impl>, name as string).mockImplementation(impl) as jest.SpyInstance;
        spies.push(s);
        return s;
    };

    beforeEach(() => {
        spies = [];
        spy('getAIAgentConnections', async () => connections);
        spy('getAIAgentConnectionModels', async () => [{id: 'gpt-5'}, {id: 'gpt-5-mini'}]);
    });
    afterEach(() => {
        spies.forEach((s) => s.mockRestore());
    });

    const renderServer = () => renderWithContext(
        <AgentsSettings
            scope='server'
            teamName='lumen'
        />,
        state,
    );

    test('lists the agents with the AGENT tag, their connection and model, external ones, those off and what needs attention', async () => {
        spy('getAIAgents', async () => [
            agent(),
            agent({user_id: 'coder1', username: 'coder', display_name: 'Coder', external: true, connection_id: '', model: '', connection_name: ''}),
            agent({user_id: 'old1', username: 'ai', display_name: 'AI', enabled: false, needs_attention: 'Review who may talk to it.'}),
        ]);
        renderServer();

        const list = await screen.findByRole('list', {name: 'Agents'});
        const rows = within(list).getAllByRole('listitem');
        expect(rows[0]).toHaveTextContent('AI');
        expect(within(rows[0]).getByText('Needs attention')).toBeInTheDocument();
        expect(rows[0]).toHaveTextContent('Off');
        expect(rows[1]).toHaveTextContent('@coder · External: a program answers for it');
        expect(rows[2]).toHaveTextContent('@researcher · Lumen OpenAI · gpt-5');
        expect(within(rows[2]).getByText('AGENT')).toBeInTheDocument();
    });

    test('adds a built-in agent with a connection, a model and its instructions', async () => {
        spy('getAIAgents', async () => []);
        const create = spy('createAIAgent', async (req: {username: string}) => agent({username: req.username, display_name: 'Researcher'}));
        renderServer();

        await userEvent.click(await screen.findByRole('button', {name: 'Add an agent'}));
        await userEvent.type(screen.getByLabelText(/^Username/), 'researcher');
        await userEvent.type(screen.getByLabelText(/^Display name/), 'Researcher');
        await screen.findByRole('option', {name: 'Lumen OpenAI'});
        await userEvent.selectOptions(screen.getByLabelText(/^Model connection/), 'conn1');
        await userEvent.type(screen.getByLabelText(/^Model(?! connection)/), 'gpt-5-mini');
        await userEvent.type(screen.getByLabelText(/^Instructions/), 'You research questions.');
        expect(screen.getByRole('switch', {name: 'Everyone'})).toHaveAttribute('aria-checked', 'true');
        expect(screen.getByRole('switch', {name: 'Guests too'})).toHaveAttribute('aria-checked', 'false');

        await userEvent.click(screen.getByRole('button', {name: 'Add agent'}));
        await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
        expect(create.mock.calls[0][0]).toMatchObject({
            username: 'researcher',
            display_name: 'Researcher',
            scope: 'server',
            external: false,
            connection_id: 'conn1',
            model: 'gpt-5-mini',
            system_prompt: 'You research questions.',
            access: {everyone: true, allow_guests: false},
        });
        expect(await screen.findByRole('status')).toHaveTextContent('Researcher added.');
    });

    test('a built-in agent can\'t be added without a connection', async () => {
        spy('getAIAgents', async () => []);
        renderServer();
        await userEvent.click(await screen.findByRole('button', {name: 'Add an agent'}));
        await userEvent.type(screen.getByLabelText(/^Username/), 'researcher');
        expect(screen.getByRole('button', {name: 'Add agent'})).toBeDisabled();
    });

    test('adds an external agent, with no runtime, then shows how its program signs in', async () => {
        spy('getAIAgents', async () => []);
        const create = spy('createAIAgent', async () => agent({user_id: 'coder1', username: 'coder', display_name: 'Coder', external: true, connection_id: '', model: '', system_prompt: ''}));
        renderServer();

        await userEvent.click(await screen.findByRole('button', {name: 'Add an agent'}));
        await userEvent.click(screen.getByRole('button', {name: 'External agent'}));
        expect(screen.queryByLabelText(/^Model connection/)).not.toBeInTheDocument();
        expect(screen.queryByLabelText(/^Instructions/)).not.toBeInTheDocument();
        await userEvent.type(screen.getByLabelText(/^Username/), 'coder');
        await userEvent.click(screen.getByRole('button', {name: 'Add agent'}));

        await waitFor(() => expect(create).toHaveBeenCalledTimes(1));
        expect(create.mock.calls[0][0]).toMatchObject({username: 'coder', external: true, connection_id: '', model: '', system_prompt: '', max_tool_turns: 0});
        const help = await screen.findByRole('region', {name: 'Connect Coder'});
        expect(help).toHaveTextContent('/api/v4/users/coder1/tokens');
        expect(help).toHaveTextContent('Authorization: Bearer <token>');
        expect(help).toHaveTextContent('/api/v4/ai/agents/coder1/activity');
        expect(within(help).getByRole('link', {name: 'Open Bot accounts'})).toHaveAttribute('href', '/lumen/integrations/bots');
    });

    test('turns an agent off and on', async () => {
        spy('getAIAgents', async () => [agent()]);
        const disable = spy('disableAIAgent', async () => agent({enabled: false}));
        const enable = spy('enableAIAgent', async () => agent({enabled: true}));
        renderServer();

        await userEvent.click(await screen.findByRole('button', {name: 'Turn off Researcher'}));
        await waitFor(() => expect(disable).toHaveBeenCalledWith('researcher1'));
        expect(await screen.findByRole('status')).toHaveTextContent('Researcher is off');

        await userEvent.click(await screen.findByRole('button', {name: 'Turn on Researcher'}));
        await waitFor(() => expect(enable).toHaveBeenCalledWith('researcher1'));
    });

    test('deletes an agent only once confirmed', async () => {
        spy('getAIAgents', async () => [agent()]);
        const remove = spy('deleteAIAgent', async () => ({status: 'OK'}));
        renderServer();

        await userEvent.click(await screen.findByRole('button', {name: 'Delete Researcher'}));
        expect(remove).not.toHaveBeenCalled();
        await userEvent.click(screen.getByRole('button', {name: 'Delete agent'}));
        await waitFor(() => expect(remove).toHaveBeenCalledWith('researcher1'));
        expect(await screen.findByRole('status')).toHaveTextContent('Researcher deleted.');
    });

    test('a community\'s agents are asked for by its scope, and their access offers its channels only', async () => {
        const list = spy('getAIAgents', async () => []);
        renderWithContext(
            <AgentsSettings
                scope='t1'
                teamName='lumen'
            />,
            state,
        );
        await waitFor(() => expect(list).toHaveBeenCalledWith('t1'));
        await userEvent.click(await screen.findByRole('button', {name: 'Add an agent'}));
        await userEvent.click(screen.getByRole('switch', {name: 'Everyone'}));
        const channels = screen.getByRole('group', {name: 'Channels chosen'});
        expect(within(channels).getByRole('button', {name: 'Roadmap'})).toBeInTheDocument();
        expect(within(channels).getAllByRole('button')).toHaveLength(1);
        expect(screen.queryByRole('group', {name: 'Communities chosen'})).not.toBeInTheDocument();
    });

    test('an error from the server shows, and nothing else leaks', async () => {
        spy('getAIAgents', async () => {
            throw new Error('You do not have the appropriate permissions.');
        });
        renderServer();
        expect(await screen.findByRole('alert')).toHaveTextContent('You do not have the appropriate permissions.');
    });
});
