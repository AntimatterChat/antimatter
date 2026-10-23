// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Antimatter: agents, the bot accounts of kind "agent" people talk to like a coworker (/api/v4/ai/agents). A built-in
// agent runs in the server with a model connection, a model and a system prompt; an external one is a program
// elsewhere that signs in with the account's token. System admins manage the server-wide agents (scope "server"); a
// community's admins its own (scope: the team's ID).

export const AI_AGENT_SCOPE_SERVER = 'server';

export const AI_AGENT_DEFAULT_MAX_TOOL_TURNS = 30;
export const AI_AGENT_MAX_TOOL_TURNS = 250;

export type BotKind = 'bot' | 'agent';

// Which tools an agent may use ('todo.add_item', 'todo.*', '*') and which reads run without asking (see ai_tools.ts):
// anything else waits for the person's confirmation.
export type AIAgentToolPolicy = {
    enabled: boolean;
    allowed: string[];
    auto_run: string[];
};

export type AIAgentReasoningEffort = '' | 'minimal' | 'low' | 'medium' | 'high';

export type AIAgentReasoning = {
    enabled: boolean;
    effort: AIAgentReasoningEffort;
    budget_tokens: number;
};

// Who may talk to an agent: everyone in its scope, or chosen communities, channels, roles and people. Guests only
// with allow_guests.
export type AIAgentAccess = {
    everyone: boolean;
    team_ids: string[];
    channel_ids: string[];
    roles: string[];
    user_ids: string[];
    allow_guests: boolean;
};

export type AIAgent = {
    user_id: string;
    scope: string;
    external: boolean;
    connection_id: string;
    model: string;
    system_prompt: string;
    tools: AIAgentToolPolicy;
    reasoning: AIAgentReasoning;

    // 0: the default (30).
    max_tool_turns: number;
    access: AIAgentAccess;

    // What an admin should review; saving any change clears it.
    needs_attention: string;
    creator_id: string;
    updater_id: string;
    create_at: number;
    update_at: number;
    delete_at: number;
};

// An agent as its managers see it: its settings and its account. No key ever appears here.
export type AIAgentDetails = AIAgent & {
    username: string;
    display_name: string;
    description: string;
    last_icon_update: number;
    enabled: boolean;
    connection_name: string;
};

export type AIAgentRequest = {
    username: string;
    display_name: string;
    description: string;
    scope: string;
    external: boolean;
    connection_id: string;
    model: string;
    system_prompt: string;
    tools: AIAgentToolPolicy;
    reasoning: AIAgentReasoning;
    max_tool_turns: number;
    access: AIAgentAccess;
};

// A field left out is kept; the scope and whether the agent is external never change.
export type AIAgentPatch = Partial<Omit<AIAgentRequest, 'scope' | 'external'>>;

export type AgentState = 'idle' | 'thinking' | 'working' | 'waiting' | 'blocked' | 'offline';

export const AGENT_BUSY_STATES: AgentState[] = ['thinking', 'working', 'waiting', 'blocked'];

// What an agent is doing now. A busy state holds until expires_at: past it, the agent is idle.
export type AgentActivity = {
    user_id: string;
    state: AgentState;
    text: string;

    // Only given to those who may read the thread; never in the websocket event.
    thread_post_id: string;
    update_at: number;
    expires_at: number;
};

export type AgentActivityRequest = {
    state: AgentState;
    text?: string;
    thread_post_id?: string;

    // 0: busy states hold for 10 minutes.
    expires_in_seconds?: number;
};

// The data of the agent_activity_changed websocket event.
export type AgentActivityChangedData = {
    user_id: string;
    activity: AgentActivity;
};

export type AgentPostStreamControl = 'start' | 'text' | 'end' | 'error' | 'cancel';

// The data of the agent_post_stream websocket event: message is the whole text so far.
export type AgentPostStreamData = {
    post_id: string;
    channel_id: string;
    root_id: string;
    user_id: string;
    message: string;
    control: AgentPostStreamControl;
};
