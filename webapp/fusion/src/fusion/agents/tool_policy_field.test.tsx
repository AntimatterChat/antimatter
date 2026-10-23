// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {AIAgentToolPolicy} from '@mattermost/types/ai_agents';
import type {AIToolCatalogue} from '@mattermost/types/ai_tools';

import {Client4} from 'mattermost-redux/client';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import ToolPolicyField from './tool_policy_field';

const catalogue: AIToolCatalogue = {
    sources: [
        {
            key: 'builtin',
            source: 'builtin',
            source_id: '',
            name: 'Antimatter',
            namespace: 'antimatter',
            tools: [
                {name: 'antimatter.search_messages', title: 'Search messages', description: '', kind: 'read', on: true, auto_run: false},
                {name: 'antimatter.post_message', title: 'Post a message', description: '', kind: 'write', on: true, auto_run: false},
            ],
        },
        {key: 'mcp:jira', source: 'mcp', source_id: 'jira', name: 'Jira', namespace: 'mcp.jira', auth: 'oauth', tools: []},
    ],
};

describe('fusion/agents/ToolPolicyField', () => {
    afterEach(() => {
        jest.restoreAllMocks();
    });

    const renderField = (policy: AIAgentToolPolicy, onChange = jest.fn()) => {
        jest.spyOn(Client4, 'getAIToolCatalogue').mockResolvedValue(catalogue);
        renderWithContext(
            <ToolPolicyField
                scope='server'
                policy={policy}
                onChange={onChange}
            />,
        );
        return onChange;
    };

    test('off, only the switch shows; turning it on keeps the policy', async () => {
        const onChange = renderField({enabled: false, allowed: ['antimatter.*'], auto_run: []});
        expect(screen.queryByText('Antimatter')).not.toBeInTheDocument();
        await userEvent.click(screen.getByRole('switch', {name: 'Tools'}));
        expect(onChange).toHaveBeenCalledWith({enabled: true, allowed: ['antimatter.*'], auto_run: []});
    });

    test('each source of the scope, with its namespace and how people sign in', async () => {
        renderField({enabled: true, allowed: ['antimatter.*'], auto_run: []});
        expect(await screen.findByText('Antimatter')).toBeInTheDocument();
        expect(screen.getByText('antimatter.*')).toBeInTheDocument();
        expect(screen.getByText('mcp.jira.* · each person signs in')).toBeInTheDocument();
        expect(screen.getAllByRole('switch', {name: 'Its reads run without asking'})).toHaveLength(1);
    });

    test('allowing all of a source, or its reads to run on their own', async () => {
        const onChange = renderField({enabled: true, allowed: [], auto_run: []});
        await screen.findByText('Antimatter');
        await userEvent.click(screen.getAllByRole('button', {name: 'All'})[0]);
        expect(onChange).toHaveBeenLastCalledWith({enabled: true, allowed: ['antimatter.*'], auto_run: []});
    });

    test('chosen tools of a source', async () => {
        const onChange = renderField({enabled: true, allowed: [], auto_run: []});
        await screen.findByText('Antimatter');
        await userEvent.click(screen.getAllByRole('button', {name: 'Chosen'})[0]);
        await userEvent.click(screen.getByRole('button', {name: 'Search messages'}));
        expect(onChange).toHaveBeenLastCalledWith({enabled: true, allowed: ['antimatter.search_messages'], auto_run: []});
    });

    test('a server each person signs in to lists no tool to choose: the note says why', async () => {
        renderField({enabled: true, allowed: [], auto_run: []});
        await screen.findByText('Jira');
        await userEvent.click(screen.getAllByRole('button', {name: 'Chosen'})[1]);
        expect(screen.getByText(/Its tools are listed here once you sign in to it yourself/)).toBeInTheDocument();
    });

    test('names no source has are kept and shown', async () => {
        renderField({enabled: true, allowed: ['mcp.gone.*'], auto_run: []});
        expect(await screen.findByText('Also allowed, not available now: mcp.gone.*')).toBeInTheDocument();
    });
});
