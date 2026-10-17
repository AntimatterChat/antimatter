// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Antimatter: model connections, the only place an AI provider's address and key live (/api/v4/ai/connections).
// System admins manage the server-wide ones (scope "server"), which they may share with chosen communities; a
// community's admins manage its own (scope: the team's ID). Keys and header values are write-only: the API only says
// whether a key is set and gives its last four characters.

export const AI_CONNECTION_SCOPE_SERVER = 'server';

export type AIProvider = 'openai_compatible' | 'anthropic';

export type AIDecisionsDialect = 'openai' | 'systemone';

// A connection's default model for each capability; empty means the capability isn't set up.
export type AIConnectionModels = {
    chat: string;
    embeddings: string;
    transcription: string;
    decisions: string;
};

export type AIModelCapability = keyof AIConnectionModels;

// 0 means no limit.
export type AIConnectionLimits = {
    requests_per_minute: number;
    monthly_token_budget: number;
    per_user_requests_per_minute: number;
    per_user_monthly_token_budget: number;
};

// Where people may use a server-wide connection: every community, or those listed.
export type AIConnectionSharing = {
    all_teams: boolean;
    team_ids: string[];
};

// Who may use a connection where it's available: everyone, or the members of chosen groups (teams and roles) and the
// holders of chosen built-in roles. Guests only with allow_guests.
export type AIConnectionAudience = {
    everyone: boolean;
    group_ids: string[];
    roles: string[];
    allow_guests: boolean;
};

export type AIConnection = {
    id: string;
    name: string;
    scope: string;
    provider: AIProvider;
    base_url: string;
    api_key_set: boolean;
    api_key_last4: string;
    extra_header_names: string[];
    models: AIConnectionModels;
    decisions_dialect: AIDecisionsDialect;
    limits: AIConnectionLimits;
    sharing: AIConnectionSharing;
    audience: AIConnectionAudience;

    // Why the connection can't be used until an admin fixes it; saving any change clears it.
    needs_attention: string;
    creator_id: string;
    updater_id: string;
    create_at: number;
    update_at: number;
    delete_at: number;
};

// AIConnectionRequest creates a connection, or tests one before it's saved: the key and headers in clear, this once.
export type AIConnectionRequest = {
    name: string;
    scope: string;
    provider: AIProvider;
    base_url: string;
    api_key: string;
    extra_headers: Record<string, string>;
    models: AIConnectionModels;
    decisions_dialect: AIDecisionsDialect;
    limits: AIConnectionLimits;
    sharing: AIConnectionSharing;
    audience: AIConnectionAudience;
};

// AIConnectionPatch changes a connection: a field left out is kept; api_key replaces the key ('' removes it) and
// extra_headers replaces all the headers. The scope never changes.
export type AIConnectionPatch = Partial<Omit<AIConnectionRequest, 'scope'>>;

export type AIConnectionTestResult = {
    ok: boolean;
    error?: string;
    status_code?: number;
    latency_ms: number;
    model_count: number;
};

export type AIModelInfo = {
    id: string;
    owned_by?: string;
};

// What someone who may use a connection sees of it.
export type AIConnectionInfo = {
    id: string;
    name: string;
    scope: string;
    provider: AIProvider;
    models: AIConnectionModels;
    capabilities: string[];
};

export type AIUsageTotals = {
    requests: number;
    input_tokens: number;
    output_tokens: number;
};

// One person's (user_id, '' for a built-in feature) use of a connection for one purpose on one day (20261017).
export type AIUsage = AIUsageTotals & {
    connection_id: string;
    user_id: string;
    purpose: string;
    day: number;
};

// A connection's usage this month (202610): totals, per day and per purpose (user, rewrite, recaps,
// auto_translation, text_to_speech, plugin:<id>).
export type AIConnectionUsage = {
    connection_id: string;
    month: number;
    totals: AIUsageTotals;
    monthly_token_budget: number;
    days: AIUsage[] | null;
    by_purpose: Record<string, AIUsageTotals>;
};
