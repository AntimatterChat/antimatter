// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Antimatter: agents' tools. The server's own ('antimatter.*'), the apps' ('<app id>.*') and the MCP servers'
// ('mcp.<slug>.*'), offered to an agent as its policy allows, minus what the person turned off, and always run as the
// person who asked. Only a read the policy lets run on its own runs at once; every other call waits for the person on
// the agent's reply (agent_tool_calls).

export type AIToolKind = 'read' | 'write' | 'external_write';
export type AIToolSource = 'builtin' | 'app' | 'mcp';
export type AIMCPAuth = 'none' | 'oauth' | 'service_account';

export const AI_TOOL_BUILTIN_NAMESPACE = 'antimatter';
export const AI_TOOL_SOURCE_BUILTIN = 'builtin';

// The preference that keeps what a person turned off (category, name), an AIToolChoices as JSON.
export const PREFERENCE_CATEGORY_AI_TOOLS = 'ai_tools';
export const PREFERENCE_NAME_AI_TOOL_CHOICES = 'choices';

export type AIToolChoices = {

    // Sources ('builtin', 'app:<id>', 'mcp:<server id>') and tools (full names) turned off.
    off_sources: string[];
    off_tools: string[];
};

export type AIToolInfo = {
    name: string;
    title: string;
    description: string;
    kind: AIToolKind;
    on: boolean;
    auto_run: boolean;
};

// A source of an agent's tools as the person sees it.
export type AIToolSourceInfo = {
    key: string;
    source: AIToolSource;
    source_id: string;
    name: string;
    on: boolean;
    auth?: AIMCPAuth;

    // Whether the person signed in to an MCP server that needs it.
    connected: boolean;

    // Its tools couldn't be listed (the MCP server didn't answer).
    unavailable: boolean;
    tools: AIToolInfo[];
};

// What the managers of a scope's agents may give them.
export type AIToolCatalogueSource = {
    key: string;
    source: AIToolSource;
    source_id: string;
    name: string;
    namespace: string;
    auth?: AIMCPAuth;
    tools: AIToolInfo[];
};

export type AIToolCatalogue = {
    sources: AIToolCatalogueSource[];
};

export type AIToolCallStatus = 'pending' | 'done' | 'failed' | 'declined' | 'expired';

// A tool call an agent's reply shows, waiting for (or answered by) the person it's for.
export type AIToolCallSummary = {
    id: string;
    tool: string;
    title: string;
    kind: AIToolKind;
    arguments: string;
    status: AIToolCallStatus;
    user_id: string;
};

export const POST_PROP_AGENT_TOOL_CALLS = 'agent_tool_calls';

export type AIConnectionSharingOfMCP = {
    all_teams: boolean;
    team_ids: string[];
};

// An MCP server as its managers see it: never a secret, only whether one is set and the headers' names.
export type AIMCPServer = {
    id: string;
    name: string;
    slug: string;
    scope: string;
    url: string;
    auth: AIMCPAuth;
    oauth_client_id: string;
    oauth_client_secret_set: boolean;
    oauth_scopes: string;
    oauth_registered: boolean;
    header_names: string[];
    sharing: AIConnectionSharingOfMCP;
    needs_attention: string;
    creator_id: string;
    updater_id: string;
    create_at: number;
    update_at: number;
    delete_at: number;
};

export type AIMCPServerRequest = {
    name: string;
    slug: string;
    scope: string;
    url: string;
    auth: AIMCPAuth;
    oauth_client_id?: string;
    oauth_client_secret?: string;
    oauth_scopes?: string;
    headers?: Record<string, string>;
    sharing: AIConnectionSharingOfMCP;
};

// A field left out is kept; oauth_client_secret '' removes it, headers replaces them all.
export type AIMCPServerPatch = Partial<Omit<AIMCPServerRequest, 'slug' | 'scope'>>;

export type AIMCPOAuthConnect = {
    authorize_url: string;
};
