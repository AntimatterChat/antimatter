// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {AIAgentToolPolicy} from '@mattermost/types/ai_agents';
import type {AIToolCatalogueSource} from '@mattermost/types/ai_tools';

// An agent's tool policy as its form edits it, per source of tools: none of its tools, all of them
// ("<namespace>.*"), or the ones chosen (their full names). Only reads ever run without asking the person, so "run
// without asking" is per source: "<namespace>.*" in auto_run covers its reads only (the server enforces it).

export type SourceMode = 'off' | 'all' | 'some';

const wildcard = (namespace: string) => `${namespace}.*`;

// inNamespace tells whether a full tool name is one of a namespace's (and not of a namespace under it).
export function inNamespace(name: string, namespace: string): boolean {
    return name.startsWith(namespace + '.') && !name.slice(namespace.length + 1).includes('.');
}

export function sourceMode(policy: AIAgentToolPolicy, source: AIToolCatalogueSource): SourceMode {
    if (policy.allowed.includes('*') || policy.allowed.includes(wildcard(source.namespace))) {
        return 'all';
    }
    return policy.allowed.some((name) => inNamespace(name, source.namespace)) ? 'some' : 'off';
}

// expandAll replaces "*" (every tool, as set through the API) by each source's wildcard, so that one source can be
// narrowed without opening the others.
function expandAll(list: string[], sources: AIToolCatalogueSource[]): string[] {
    if (!list.includes('*')) {
        return list;
    }
    return [...list.filter((x) => x !== '*'), ...sources.map((s) => wildcard(s.namespace))];
}

function unique(list: string[]): string[] {
    return [...new Set(list)];
}

// withSourceMode gives the policy with a source's tools off, all on, or some (keeping those already chosen, or
// starting with none).
export function withSourceMode(policy: AIAgentToolPolicy, sources: AIToolCatalogueSource[], source: AIToolCatalogueSource, mode: SourceMode): AIAgentToolPolicy {
    const allowed = expandAll(policy.allowed, sources).filter((name) => name !== wildcard(source.namespace) && (mode === 'some' || !inNamespace(name, source.namespace)));
    const autoRun = expandAll(policy.auto_run, sources).filter((name) => mode !== 'off' || (name !== wildcard(source.namespace) && !inNamespace(name, source.namespace)));
    if (mode === 'all') {
        allowed.push(wildcard(source.namespace));
    }
    return {...policy, allowed: unique(allowed), auto_run: unique(autoRun)};
}

// withTool chooses one tool (of a source in "some" mode) or not.
export function withTool(policy: AIAgentToolPolicy, name: string, on: boolean): AIAgentToolPolicy {
    const allowed = policy.allowed.filter((x) => x !== name);
    if (on) {
        allowed.push(name);
    }
    return {...policy, allowed: unique(allowed), auto_run: on ? policy.auto_run : policy.auto_run.filter((x) => x !== name)};
}

// autoRunsReads tells whether a source's reads run without asking.
export function autoRunsReads(policy: AIAgentToolPolicy, source: AIToolCatalogueSource): boolean {
    return policy.auto_run.includes('*') || policy.auto_run.includes(wildcard(source.namespace));
}

export function withAutoRunReads(policy: AIAgentToolPolicy, sources: AIToolCatalogueSource[], source: AIToolCatalogueSource, on: boolean): AIAgentToolPolicy {
    const autoRun = expandAll(policy.auto_run, sources).filter((x) => x !== wildcard(source.namespace) && !inNamespace(x, source.namespace));
    if (on) {
        autoRun.push(wildcard(source.namespace));
    }
    return {...policy, auto_run: unique(autoRun)};
}

// The policy's names that no source of the catalogue has (a server since removed, an app not running): kept as they
// are, and shown.
export function unknownNames(policy: AIAgentToolPolicy, sources: AIToolCatalogueSource[]): string[] {
    return policy.allowed.filter((name) => name !== '*' && !sources.some((s) => name === wildcard(s.namespace) || inNamespace(name, s.namespace)));
}
