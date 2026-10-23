// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {AIAgentToolPolicy} from '@mattermost/types/ai_agents';
import type {AIToolCatalogueSource} from '@mattermost/types/ai_tools';

import {autoRunsReads, inNamespace, sourceMode, unknownNames, withAutoRunReads, withSourceMode, withTool} from './tool_policy';

const source = (namespace: string, tools: string[] = []): AIToolCatalogueSource => ({
    key: namespace,
    source: namespace === 'antimatter' ? 'builtin' : 'mcp',
    source_id: namespace,
    name: namespace,
    namespace,
    tools: tools.map((name) => ({name: `${namespace}.${name}`, title: name, description: '', kind: 'read', on: true, auto_run: false})),
});

const builtin = source('antimatter', ['search_messages', 'post_message']);
const jira = source('mcp.jira', ['search']);
const sources = [builtin, jira];
const policy = (allowed: string[], autoRun: string[] = []): AIAgentToolPolicy => ({enabled: true, allowed, auto_run: autoRun});

describe('fusion/agents/tool_policy', () => {
    test('a namespace is one level: mcp.jira.* is not mcp.*', () => {
        expect(inNamespace('mcp.jira.search', 'mcp.jira')).toBe(true);
        expect(inNamespace('mcp.jira.search', 'mcp')).toBe(false);
        expect(inNamespace('antimatter.search', 'mcp.jira')).toBe(false);
    });

    test('a source is off, all or chosen', () => {
        expect(sourceMode(policy([]), builtin)).toBe('off');
        expect(sourceMode(policy(['antimatter.*']), builtin)).toBe('all');
        expect(sourceMode(policy(['*']), jira)).toBe('all');
        expect(sourceMode(policy(['antimatter.search_messages']), builtin)).toBe('some');
        expect(sourceMode(policy(['antimatter.search_messages']), jira)).toBe('off');
    });

    test('changing a source leaves the others alone, even under "*"', () => {
        const narrowed = withSourceMode(policy(['*'], ['*']), sources, jira, 'off');
        expect(narrowed.allowed).toEqual(['antimatter.*']);
        expect(narrowed.auto_run).toEqual(['antimatter.*']);
        expect(withSourceMode(policy(['antimatter.search_messages']), sources, builtin, 'all').allowed).toEqual(['antimatter.*']);
        expect(withSourceMode(policy(['antimatter.*']), sources, builtin, 'some').allowed).toEqual([]);
        expect(withSourceMode(policy(['antimatter.search_messages', 'mcp.jira.*'], ['antimatter.*']), sources, builtin, 'off')).toEqual(policy(['mcp.jira.*']));
    });

    test('choosing tools one by one', () => {
        const one = withTool(policy([]), 'antimatter.search_messages', true);
        expect(one.allowed).toEqual(['antimatter.search_messages']);
        expect(withTool(one, 'antimatter.search_messages', true).allowed).toEqual(['antimatter.search_messages']);
        expect(withTool(policy(['antimatter.search_messages'], ['antimatter.search_messages']), 'antimatter.search_messages', false)).toEqual(policy([]));
    });

    test('reads run without asking per source', () => {
        const on = withAutoRunReads(policy(['antimatter.*']), sources, builtin, true);
        expect(on.auto_run).toEqual(['antimatter.*']);
        expect(autoRunsReads(on, builtin)).toBe(true);
        expect(autoRunsReads(on, jira)).toBe(false);
        expect(withAutoRunReads(on, sources, builtin, false).auto_run).toEqual([]);
        expect(withAutoRunReads(policy(['*'], ['*']), sources, jira, false).auto_run).toEqual(['antimatter.*']);
    });

    test('names no source has are kept and listed', () => {
        expect(unknownNames(policy(['mcp.gone.*', 'antimatter.*', 'com.example.todo.add', '*']), sources)).toEqual(['mcp.gone.*', 'com.example.todo.add']);
    });
});
