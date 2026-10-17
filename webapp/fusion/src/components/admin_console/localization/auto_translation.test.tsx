// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import React from 'react';

import type {AIConnection} from '@mattermost/types/ai_connections';
import type {AutoTranslationSettings} from '@mattermost/types/config';

import {Client4} from 'mattermost-redux/client';

import {renderWithContext, screen, userEvent, waitFor, within} from 'tests/react_testing_utils';

import AutoTranslation from './auto_translation';

// Antimatter: a model connection is the recommended translation provider; the AI chat app's service ("agents") is the
// older one, which now names a model connection too.
describe('components/admin_console/localization/AutoTranslation', () => {
    const connection = {id: 'c1', name: 'Translator', scope: 'server', provider: 'openai_compatible', models: {chat: 'hy-mt', embeddings: '', transcription: '', decisions: ''}, needs_attention: ''} as AIConnection;
    let list: jest.SpyInstance;

    beforeEach(() => {
        list = jest.spyOn(Client4, 'getAIConnections').mockResolvedValue([connection]);
    });
    afterEach(() => {
        list.mockRestore();
    });

    const render = (settings: Partial<AutoTranslationSettings>, aiAutoTranslation = {ConnectionId: '', Model: ''}) => {
        const onChange = jest.fn();
        renderWithContext(
            <AutoTranslation
                id='AutoTranslationSettings'
                label=''
                helpText=''
                value={{Enable: true, TargetLanguages: ['en'], TimeoutMs: 5000, RestrictDMAndGM: false, LibreTranslate: {URL: '', APIKey: ''}, ...settings}}
                disabled={false}
                config={{AISettings: {AutoTranslation: aiAutoTranslation}} as never}
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

    test('offers the model connection first, as recommended, and the AI chat app\'s service as the older one', () => {
        render({Provider: 'libretranslate'});
        const options = within(screen.getByTestId('Providerdropdown')).getAllByRole('option');
        expect(options.map((o) => (o as HTMLOptionElement).value)).toEqual(['connection', 'libretranslate', 'openai', 'agents']);
        expect(options[0]).toHaveTextContent('Model connection (recommended)');
        expect(options[3]).toHaveTextContent('AI chat app\'s service (older)');
    });

    test('a new setup uses a model connection', () => {
        render({Provider: ''});
        expect(screen.getByTestId('Providerdropdown')).toHaveValue('connection');
    });

    test('with a model connection, picks it and saves it as auto-translation\'s', async () => {
        const onChange = render({Provider: 'connection'});
        const select = document.getElementById('AutoTranslationConnectionId') as HTMLSelectElement;
        await waitFor(() => expect(select).toBeEnabled());
        await userEvent.selectOptions(select, 'c1');
        expect(onChange).toHaveBeenCalledWith('AISettings.AutoTranslation', {ConnectionId: 'c1', Model: ''});
        expect(await screen.findByLabelText('Model')).toHaveDisplayValue('The connection\'s chat model (hy-mt)');
    });

    test('shows the connection already picked', async () => {
        render({Provider: 'connection'}, {ConnectionId: 'c1', Model: 'hy-mt'});
        await waitFor(() => expect(document.getElementById('AutoTranslationConnectionId')).toHaveValue('c1'));
        expect(screen.getByLabelText('Model')).toHaveValue('hy-mt');
    });

    test('the older provider switches to the model connection migrated from the app\'s service', async () => {
        const onChange = render({Provider: 'agents', Agents: {LLMServiceID: 'svc1'}});
        expect(screen.queryByText(/The Agents plugin is/)).not.toBeInTheDocument();
        expect(screen.getByText(/This older setting no longer needs the AI chat app/)).toBeInTheDocument();
        await userEvent.click(screen.getByRole('button', {name: 'Switch to Model connection'}));
        expect(onChange).toHaveBeenCalledWith('AISettings.AutoTranslation', {ConnectionId: 'svc1', Model: ''});
        expect(onChange).toHaveBeenCalledWith('AutoTranslationSettings', expect.objectContaining({Provider: 'connection'}));
    });
});
