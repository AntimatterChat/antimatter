// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {AIAgentAccess, AIAgentDetails, AIAgentPatch, AIAgentReasoningEffort, AIAgentRequest, AIAgentToolPolicy} from '@mattermost/types/ai_agents';
import {AI_AGENT_MAX_TOOL_TURNS, AI_AGENT_SCOPE_SERVER} from '@mattermost/types/ai_agents';

// Agents (Antimatter): what the agents' list and form share, on the Integrations page (server-wide agents, for system
// admins) and in a community's settings (its own, for its admins). A built-in agent answers in the server with a model
// connection, a model and a system prompt; an external one is a program elsewhere, signed in with the account's token.

export {AI_AGENT_SCOPE_SERVER};

export type AgentKind = 'builtin' | 'external';

export type AgentDraft = {
    kind: AgentKind;
    username: string;
    displayName: string;
    description: string;
    connectionId: string;
    model: string;
    systemPrompt: string;
    everyone: boolean;
    allowGuests: boolean;
    teamIds: string[];
    channelIds: string[];
    roles: string[];

    // Not edited here (the server keeps them): people chosen through the API.
    userIds: string[];
    reasoningEffort: AIAgentReasoningEffort;

    // As typed, until saved: '' is the default.
    maxToolTurns: string;

    // The tool registry comes later: the policy is kept as it is.
    tools: AIAgentToolPolicy;
};

export const REASONING_EFFORTS: AIAgentReasoningEffort[] = ['', 'low', 'medium', 'high'];

// The roles the access can name: a community's agent its admins; a server one also the system admins.
export const SERVER_ROLES = ['system_admin', 'team_admin'];
export const COMMUNITY_ROLES = ['team_admin'];

const NO_TOOLS: AIAgentToolPolicy = {enabled: false, allowed: [], auto_run: []};

export function newAgentDraft(kind: AgentKind = 'builtin'): AgentDraft {
    return {
        kind,
        username: '',
        displayName: '',
        description: '',
        connectionId: '',
        model: '',
        systemPrompt: '',
        everyone: true,
        allowGuests: false,
        teamIds: [],
        channelIds: [],
        roles: [],
        userIds: [],
        reasoningEffort: '',
        maxToolTurns: '',
        tools: NO_TOOLS,
    };
}

export function draftFromAgent(agent: AIAgentDetails): AgentDraft {
    return {
        kind: agent.external ? 'external' : 'builtin',
        username: agent.username,
        displayName: agent.display_name || '',
        description: agent.description || '',
        connectionId: agent.connection_id || '',
        model: agent.model || '',
        systemPrompt: agent.system_prompt || '',
        everyone: Boolean(agent.access?.everyone),
        allowGuests: Boolean(agent.access?.allow_guests),
        teamIds: agent.access?.team_ids || [],
        channelIds: agent.access?.channel_ids || [],
        roles: agent.access?.roles || [],
        userIds: agent.access?.user_ids || [],
        reasoningEffort: agent.reasoning?.enabled ? agent.reasoning.effort || 'medium' : '',
        maxToolTurns: agent.max_tool_turns ? String(agent.max_tool_turns) : '',
        tools: agent.tools || NO_TOOLS,
    };
}

// A username as the server takes it: 3 to 22 lower-case letters, digits, dots, dashes and underscores, starting with a
// letter.
export function validUsername(username: string): boolean {
    return (/^[a-z][a-z0-9._-]{2,21}$/).test(username);
}

export type AgentProblem = 'username' | 'connection' | 'maxToolTurns' | 'access';

export function problemsOf(draft: AgentDraft, editing = false): AgentProblem[] {
    const problems: AgentProblem[] = [];
    if (!editing && !validUsername(draft.username.trim().toLowerCase())) {
        problems.push('username');
    }
    if (draft.kind === 'builtin' && !draft.connectionId) {
        problems.push('connection');
    }
    const turns = draft.maxToolTurns.trim();
    if (turns && (!(/^\d+$/).test(turns) || Number(turns) > AI_AGENT_MAX_TOOL_TURNS)) {
        problems.push('maxToolTurns');
    }
    if (!draft.everyone && !draft.teamIds.length && !draft.channelIds.length && !draft.roles.length && !draft.userIds.length) {
        problems.push('access');
    }
    return problems;
}

function accessOf(draft: AgentDraft): AIAgentAccess {
    return {
        everyone: draft.everyone,
        team_ids: draft.everyone ? [] : draft.teamIds,
        channel_ids: draft.everyone ? [] : draft.channelIds,
        roles: draft.everyone ? [] : draft.roles,
        user_ids: draft.everyone ? [] : draft.userIds,
        allow_guests: draft.allowGuests,
    };
}

function runtimeOf(draft: AgentDraft) {
    if (draft.kind === 'external') {
        return {
            connection_id: '',
            model: '',
            system_prompt: '',
            tools: NO_TOOLS,
            reasoning: {enabled: false, effort: '' as AIAgentReasoningEffort, budget_tokens: 0},
            max_tool_turns: 0,
        };
    }
    return {
        connection_id: draft.connectionId,
        model: draft.model.trim(),
        system_prompt: draft.systemPrompt.trim(),
        tools: draft.tools,
        reasoning: {enabled: draft.reasoningEffort !== '', effort: draft.reasoningEffort, budget_tokens: 0},
        max_tool_turns: draft.maxToolTurns.trim() ? Number(draft.maxToolTurns.trim()) : 0,
    };
}

export function requestOf(draft: AgentDraft, scope: string): AIAgentRequest {
    return {
        username: draft.username.trim().toLowerCase(),
        display_name: draft.displayName.trim(),
        description: draft.description.trim(),
        scope,
        external: draft.kind === 'external',
        ...runtimeOf(draft),
        access: accessOf(draft),
    };
}

// patchOf sends everything the form edits; the scope and the kind never change, nor (here) the username.
export function patchOf(draft: AgentDraft): AIAgentPatch {
    const runtime = runtimeOf(draft);
    const patch: AIAgentPatch = {
        display_name: draft.displayName.trim(),
        description: draft.description.trim(),
        access: accessOf(draft),
    };
    if (draft.kind === 'builtin') {
        Object.assign(patch, runtime);
    }
    return patch;
}

export function agentName(agent: Pick<AIAgentDetails, 'display_name' | 'username'>): string {
    return agent.display_name || agent.username;
}

// The commands an external agent's program uses, for the help shown once it's made.
export function externalAgentExamples(siteURL: string, userId: string): {token: string; activity: string} {
    const base = `${siteURL.replace(/\/+$/, '')}/api/v4`;
    return {
        token: `curl -X POST ${base}/users/${userId}/tokens \\\n  -H 'Authorization: Bearer <admin token>' \\\n  -d '{"description": "agent"}'`,
        activity: `curl -X PUT ${base}/ai/agents/${userId}/activity \\\n  -H 'Authorization: Bearer <agent token>' \\\n  -d '{"state": "working", "text": "Running the tests"}'`,
    };
}
