// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {AIConnection} from '@mattermost/types/ai_connections';
import type {AISettings} from '@mattermost/types/config';

import {Client4} from 'mattermost-redux/client';

import {renderWithContext, screen, userEvent, waitFor, within} from 'tests/react_testing_utils';

import BuiltInFeatures from './builtin_features';

const connection = (patch: Partial<AIConnection>): AIConnection => ({
    id: 'c1',
    name: 'Lumen OpenAI',
    scope: 'server',
    provider: 'openai_compatible',
    base_url: '',
    api_key_set: true,
    api_key_last4: 'wxyz',
    extra_header_names: [],
    models: {chat: 'gpt-5', embeddings: 'text-embedding-3-small', transcription: '', decisions: ''},
    decisions_dialect: 'openai',
    limits: {requests_per_minute: 0, monthly_token_budget: 0, per_user_requests_per_minute: 0, per_user_monthly_token_budget: 0},
    sharing: {all_teams: true, team_ids: []},
    audience: {everyone: true, group_ids: [], roles: [], allow_guests: false},
    needs_attention: '',
    creator_id: '',
    updater_id: '',
    create_at: 1,
    update_at: 1,
    delete_at: 0,
    ...patch,
});

const off = {ConnectionId: '', Model: ''};

describe('components/admin_console/ai/BuiltInFeatures', () => {
    let list: jest.SpyInstance;

    beforeEach(() => {
        list = jest.spyOn(Client4, 'getAIConnections').mockResolvedValue([
            connection({}),
            connection({id: 'c2', name: 'Broken', needs_attention: 'Its service type isn\'t spoken.'}),
            connection({id: 'c3', name: 'Labs only', scope: 't1'}),
        ]);
    });
    afterEach(() => {
        list.mockRestore();
    });

    const render = (value: Partial<AISettings> = {}, onChange = jest.fn(), disabled = false) => {
        renderWithContext(
            <BuiltInFeatures
                id='AISettings'
                label=''
                helpText=''
                value={{Rewrite: off, Recaps: off, AutoTranslation: off, TextToSpeech: off, ...value}}
                disabled={disabled}
                config={{TextToSpeechSettings: {Model: 'tts-1'}} as never}
                setByEnv={false}
                onChange={onChange}
                registerSaveAction={jest.fn()}
                setSaveNeeded={jest.fn()}
                unRegisterSaveAction={jest.fn()}
                cancelSubmit={jest.fn()}
                showConfirm={false}
            />,
        );
        return onChange;
    };

    test('every feature is Off without a connection, and the server-wide connections are offered', async () => {
        render();
        await waitFor(() => expect(list).toHaveBeenCalledWith('server'));
        await waitFor(() => expect(document.getElementById('RewriteConnectionId')).toBeEnabled());
        for (const id of ['Rewrite', 'Recaps', 'AutoTranslation', 'TextToSpeech']) {
            const select = document.getElementById(`${id}ConnectionId`) as HTMLSelectElement;
            expect(select.value).toBe('');
            expect(select).toBeEnabled();
            const names = within(select).getAllByRole('option').map((o) => o.textContent);
            expect(names).toEqual(['Off', 'Broken (needs attention)', 'Lumen OpenAI']);
        }
        expect(screen.queryByLabelText('Model')).not.toBeInTheDocument();
    });

    test('picking a connection sets the feature, with the connection\'s default model', async () => {
        const onChange = render();
        const select = document.getElementById('RewriteConnectionId') as HTMLSelectElement;
        await waitFor(() => expect(select).toBeEnabled());
        await userEvent.selectOptions(select, 'c1');
        expect(onChange).toHaveBeenCalledWith('AISettings', {Rewrite: {ConnectionId: 'c1', Model: ''}, Recaps: off, AutoTranslation: off, TextToSpeech: off});
    });

    test('Off turns the feature\'s AI off', async () => {
        const onChange = render({Recaps: {ConnectionId: 'c1', Model: 'gpt-5'}});
        const select = document.getElementById('RecapsConnectionId') as HTMLSelectElement;
        await waitFor(() => expect(select).toBeEnabled());
        await userEvent.selectOptions(select, '');
        expect(onChange).toHaveBeenLastCalledWith('AISettings', expect.objectContaining({Recaps: {ConnectionId: '', Model: ''}}));
    });

    test('offers the connection\'s defaults, then the models its endpoint lists', async () => {
        const models = jest.spyOn(Client4, 'getAIConnectionModels').mockResolvedValue([{id: 'gpt-5'}, {id: 'gpt-5-mini'}]);
        const onChange = render({Rewrite: {ConnectionId: 'c1', Model: ''}});
        const model = await screen.findByLabelText('Model') as HTMLSelectElement;
        await waitFor(() => expect(within(model).getAllByRole('option').map((o) => o.textContent)).toEqual(['The connection\'s chat model (gpt-5)', 'gpt-5', 'text-embedding-3-small']));

        await userEvent.click(screen.getByRole('button', {name: 'Load models'}));
        await waitFor(() => expect(models).toHaveBeenCalledWith('c1'));
        await waitFor(() => expect(within(model).getAllByRole('option')).toHaveLength(4));
        await userEvent.selectOptions(model, 'gpt-5-mini');
        expect(onChange).toHaveBeenLastCalledWith('AISettings', expect.objectContaining({Rewrite: {ConnectionId: 'c1', Model: 'gpt-5-mini'}}));
        models.mockRestore();
    });

    test('text to speech defaults to the model of its own settings', async () => {
        render({TextToSpeech: {ConnectionId: 'c1', Model: ''}});
        const model = await screen.findByLabelText('Model') as HTMLSelectElement;
        await waitFor(() => expect(within(model).getAllByRole('option')[0]).toHaveTextContent('The text to speech model (tts-1)'));
    });

    test('says when a picked connection is gone or needs attention', async () => {
        render({Rewrite: {ConnectionId: 'gone', Model: ''}, Recaps: {ConnectionId: 'c2', Model: ''}});
        expect(await screen.findByText(/This connection no longer exists or isn't server-wide/)).toBeInTheDocument();
        expect(screen.getByText('This connection needs attention: Its service type isn\'t spoken.')).toBeInTheDocument();
        const rewrite = document.getElementById('RewriteConnectionId') as HTMLSelectElement;
        expect(rewrite.value).toBe('gone');
        expect(within(rewrite).getByRole('option', {name: 'Unknown connection (gone)'})).toBeInTheDocument();
    });

    test('can\'t be changed without the right to', async () => {
        render({}, jest.fn(), true);
        await waitFor(() => expect(list).toHaveBeenCalled());
        expect(document.getElementById('RewriteConnectionId')).toBeDisabled();
    });

    test('points to the model connections when there are none', async () => {
        list.mockResolvedValue([]);
        render();
        expect(await screen.findByRole('link', {name: 'AI › Connections and agents'})).toHaveAttribute('href', '/admin_console/ai/model_connections');
    });

    test('points to the recaps and text to speech pages, in AI too', async () => {
        render();
        expect(await screen.findByRole('link', {name: 'AI › Recaps'})).toHaveAttribute('href', '/admin_console/ai/recaps');
        expect(screen.getByRole('link', {name: 'AI › Text to speech'})).toHaveAttribute('href', '/admin_console/ai/text_to_speech');
    });
});
