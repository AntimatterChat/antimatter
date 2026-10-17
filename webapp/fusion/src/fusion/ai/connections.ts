// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {
    AIConnection,
    AIConnectionAudience,
    AIConnectionLimits,
    AIConnectionModels,
    AIConnectionPatch,
    AIConnectionRequest,
    AIConnectionUsage,
    AIDecisionsDialect,
    AIModelCapability,
    AIProvider,
    AIUsageTotals,
} from '@mattermost/types/ai_connections';
import {AI_CONNECTION_SCOPE_SERVER} from '@mattermost/types/ai_connections';

// Model connections (Antimatter): what the list and the form of the System Console's AI › Model connections and of
// the community settings' Model connections tab share. A connection is server-wide (scope "server") or a community's
// own (scope: its team ID); its key is write-only, so the form never has it, only "set, ends in ••wxyz".

export {AI_CONNECTION_SCOPE_SERVER};

export const CAPABILITIES: AIModelCapability[] = ['chat', 'embeddings', 'transcription', 'decisions'];

// The capabilities a provider has: Anthropic's API only chats.
export function capabilitiesOf(provider: AIProvider): AIModelCapability[] {
    return provider === 'anthropic' ? ['chat'] : CAPABILITIES;
}

export type Preset = {
    id: string;
    name: string;
    provider: AIProvider;
    baseURL: string;

    // An address on the server's own machine or network: a community's connections can't reach it.
    local?: boolean;
};

// Presets fill the provider and base URL of well-known endpoints; anything else is typed. Cloud services with their own
// sign-in (Bedrock, Vertex, Azure Entra) go through their OpenAI-compatible endpoint or a gateway such as LiteLLM.
export const PRESETS: Preset[] = [
    {id: 'openai', name: 'OpenAI', provider: 'openai_compatible', baseURL: 'https://api.openai.com/v1'},
    {id: 'mistral', name: 'Mistral', provider: 'openai_compatible', baseURL: 'https://api.mistral.ai/v1'},
    {id: 'gemini', name: 'Gemini', provider: 'openai_compatible', baseURL: 'https://generativelanguage.googleapis.com/v1beta/openai'},
    {id: 'openrouter', name: 'OpenRouter', provider: 'openai_compatible', baseURL: 'https://openrouter.ai/api/v1'},
    {id: 'ollama', name: 'Ollama', provider: 'openai_compatible', baseURL: 'http://localhost:11434/v1', local: true},
    {id: 'litellm', name: 'LiteLLM', provider: 'openai_compatible', baseURL: 'http://localhost:4000/v1', local: true},
];

// The presets offered for a scope: a community's connection may not reach the server's own network.
export function presetsFor(scope: string): Preset[] {
    return scope === AI_CONNECTION_SCOPE_SERVER ? PRESETS : PRESETS.filter((p) => !p.local);
}

// The provider's own address when the base URL is left empty, as the server uses it.
export const DEFAULT_BASE_URLS: Record<AIProvider, string> = {
    openai_compatible: 'https://api.openai.com/v1',
    anthropic: 'https://api.anthropic.com/v1',
};

// keepKey leaves the saved key as it is; replace sends the typed one; remove sends '' (no key).
export type KeyAction = 'keep' | 'replace' | 'remove';

export type HeaderRow = {key: number; name: string; value: string};

// Draft is the form's state. Limits are kept as typed (strings) until saved.
export type Draft = {
    name: string;
    provider: AIProvider;
    baseURL: string;
    keyAction: KeyAction;
    apiKey: string;

    // The headers are only sent when changed: the saved values are never given back, so changing them means typing
    // them all again.
    headersChanged: boolean;
    headers: HeaderRow[];
    models: AIConnectionModels;
    decisionsDialect: AIDecisionsDialect;
    limits: Record<keyof AIConnectionLimits, string>;
    allTeams: boolean;
    teamIds: string[];
    audience: AIConnectionAudience;
};

const NO_MODELS: AIConnectionModels = {chat: '', embeddings: '', transcription: '', decisions: ''};

let headerKey = 0;
export function newHeaderRow(name = '', value = ''): HeaderRow {
    headerKey += 1;
    return {key: headerKey, name, value};
}

// newDraft is the form of a new connection: server-wide ones are shared with every community until the admin
// chooses; everyone in scope may use it, guests not.
export function newDraft(scope: string): Draft {
    return {
        name: '',
        provider: 'openai_compatible',
        baseURL: '',
        keyAction: 'replace',
        apiKey: '',
        headersChanged: true,
        headers: [],
        models: {...NO_MODELS},
        decisionsDialect: 'openai',
        limits: {requests_per_minute: '', monthly_token_budget: '', per_user_requests_per_minute: '', per_user_monthly_token_budget: ''},
        allTeams: scope === AI_CONNECTION_SCOPE_SERVER,
        teamIds: [],
        audience: {everyone: true, group_ids: [], roles: [], allow_guests: false},
    };
}

const limitText = (n?: number) => (n && n > 0 ? String(n) : '');

// draftFromConnection is the form of a saved connection: its key kept, its headers' names only.
export function draftFromConnection(c: AIConnection): Draft {
    return {
        name: c.name,
        provider: c.provider,
        baseURL: c.base_url || '',
        keyAction: 'keep',
        apiKey: '',
        headersChanged: false,
        headers: (c.extra_header_names || []).map((name) => newHeaderRow(name)),
        models: {...NO_MODELS, ...c.models},
        decisionsDialect: c.decisions_dialect || 'openai',
        limits: {
            requests_per_minute: limitText(c.limits?.requests_per_minute),
            monthly_token_budget: limitText(c.limits?.monthly_token_budget),
            per_user_requests_per_minute: limitText(c.limits?.per_user_requests_per_minute),
            per_user_monthly_token_budget: limitText(c.limits?.per_user_monthly_token_budget),
        },
        allTeams: Boolean(c.sharing?.all_teams),
        teamIds: c.sharing?.team_ids || [],
        audience: {
            everyone: Boolean(c.audience?.everyone),
            group_ids: c.audience?.group_ids || [],
            roles: c.audience?.roles || [],
            allow_guests: Boolean(c.audience?.allow_guests),
        },
    };
}

// parseLimit reads a typed limit: empty is 0 (no limit); undefined when it isn't a whole number of 0 or more.
export function parseLimit(text: string): number | undefined {
    const t = text.trim().replace(/[\s,_]/g, '');
    if (!t) {
        return 0;
    }
    if (!(/^\d+$/).test(t)) {
        return undefined;
    }
    const n = Number(t);
    return Number.isSafeInteger(n) ? n : undefined;
}

export function limitsOf(draft: Draft): AIConnectionLimits | undefined {
    const out = {} as AIConnectionLimits;
    for (const [key, text] of Object.entries(draft.limits) as Array<[keyof AIConnectionLimits, string]>) {
        const n = parseLimit(text);
        if (n === undefined) {
            return undefined;
        }
        out[key] = n;
    }
    return out;
}

// modelsOf keeps only what the provider can do (the server refuses Anthropic embeddings and the like).
function modelsOf(draft: Draft): AIConnectionModels {
    const allowed = capabilitiesOf(draft.provider);
    const out = {...NO_MODELS};
    for (const cap of allowed) {
        out[cap] = draft.models[cap].trim();
    }
    return out;
}

export function headersOf(draft: Draft): Record<string, string> {
    const out: Record<string, string> = {};
    for (const row of draft.headers) {
        const name = row.name.trim();
        if (name) {
            out[name] = row.value;
        }
    }
    return out;
}

// A header name the server accepts (an HTTP token), and not one the HTTP client owns.
const FORBIDDEN_HEADERS = ['host', 'content-length', 'content-type', 'transfer-encoding', 'connection', 'upgrade', 'te', 'trailer', 'cookie'];
export function isValidHeaderName(name: string): boolean {
    return (/^[!#$%&'*+\-.^_`|~0-9A-Za-z]{1,128}$/).test(name) && !FORBIDDEN_HEADERS.includes(name.toLowerCase());
}

// isValidBaseURL tells whether the server takes a base URL: http(s), with a host, and no password, query or fragment.
export function isValidBaseURL(raw: string): boolean {
    if (raw.length > 1024) {
        return false;
    }
    try {
        const u = new URL(raw);
        return (u.protocol === 'http:' || u.protocol === 'https:') && Boolean(u.host) && !u.username && !u.password && !u.search && !u.hash;
    } catch {
        return false;
    }
}

export type DraftProblem = 'name' | 'baseURL' | 'limits' | 'headers' | 'apiKey';

// problemsOf lists what keeps a draft from being saved, as the server would refuse it.
export function problemsOf(draft: Draft): DraftProblem[] {
    const problems: DraftProblem[] = [];
    if (!draft.name.trim()) {
        problems.push('name');
    }
    const url = draft.baseURL.trim();
    if (url && !isValidBaseURL(url)) {
        problems.push('baseURL');
    }
    if (!limitsOf(draft)) {
        problems.push('limits');
    }
    if (draft.headersChanged && draft.headers.some((h) => (h.name.trim() || h.value) && (!isValidHeaderName(h.name.trim()) || (/[\r\n]/).test(h.value)))) {
        problems.push('headers');
    }
    if (draft.keyAction === 'replace' && (/[\r\n]/).test(draft.apiKey)) {
        problems.push('apiKey');
    }
    return problems;
}

// A community's connection has no sharing: it's for that community only.
function sharingOf(draft: Draft, scope: string) {
    if (scope !== AI_CONNECTION_SCOPE_SERVER) {
        return {all_teams: false, team_ids: []};
    }
    return {all_teams: draft.allTeams, team_ids: draft.allTeams ? [] : draft.teamIds};
}

function audienceOf(draft: Draft): AIConnectionAudience {
    const a = draft.audience;
    return a.everyone ? {everyone: true, group_ids: [], roles: [], allow_guests: a.allow_guests} : {...a};
}

// requestOf is the request creating a connection, or testing the draft before it's saved. A saved key that's kept isn't
// in it: the server tests a saved connection with its own key (testRequestNeedsSavedSecrets).
export function requestOf(draft: Draft, scope: string): AIConnectionRequest {
    return {
        name: draft.name.trim(),
        scope,
        provider: draft.provider,
        base_url: draft.baseURL.trim().replace(/\/+$/, ''),
        api_key: draft.keyAction === 'replace' ? draft.apiKey.trim() : '',
        extra_headers: draft.headersChanged ? headersOf(draft) : {},
        models: modelsOf(draft),
        decisions_dialect: draft.provider === 'anthropic' ? 'openai' : draft.decisionsDialect,
        limits: limitsOf(draft) || {requests_per_minute: 0, monthly_token_budget: 0, per_user_requests_per_minute: 0, per_user_monthly_token_budget: 0},
        sharing: sharingOf(draft, scope),
        audience: audienceOf(draft),
    };
}

// patchOf is the patch saving an edited connection: the key only when replaced or removed, the headers only when
// changed; everything else as shown.
export function patchOf(draft: Draft, scope: string): AIConnectionPatch {
    const request = requestOf(draft, scope);
    const patch: AIConnectionPatch = {
        name: request.name,
        provider: request.provider,
        base_url: request.base_url,
        models: request.models,
        decisions_dialect: request.decisions_dialect,
        limits: request.limits,
        audience: request.audience,
    };
    if (scope === AI_CONNECTION_SCOPE_SERVER) {
        patch.sharing = request.sharing;
    }
    if (draft.keyAction === 'replace' && request.api_key) {
        patch.api_key = request.api_key;
    } else if (draft.keyAction === 'remove') {
        patch.api_key = '';
    }
    if (draft.headersChanged) {
        patch.extra_headers = request.extra_headers;
    }
    return patch;
}

// endpointChanged tells whether a saved connection's draft reaches its endpoint differently: then the saved
// connection's own test no longer tests what would be saved.
export function endpointChanged(draft: Draft, saved: AIConnection): boolean {
    return draft.provider !== saved.provider ||
        draft.baseURL.trim().replace(/\/+$/, '') !== (saved.base_url || '') ||
        keyActionOf(draft) !== 'keep' ||
        draft.headersChanged;
}

// keyActionOf is what saving does to a saved key: Replace with nothing typed keeps it.
export function keyActionOf(draft: Draft): KeyAction {
    return draft.keyAction === 'replace' && !draft.apiKey.trim() ? 'keep' : draft.keyAction;
}

// testRequestNeedsSavedSecrets tells whether testing a changed draft would need the saved key or headers, which the
// form doesn't have: the admin saves first, or types them again.
export function testRequestNeedsSavedSecrets(draft: Draft, saved: AIConnection): boolean {
    return (keyActionOf(draft) === 'keep' && saved.api_key_set) || (!draft.headersChanged && (saved.extra_header_names || []).length > 0);
}

// The message of an API error (its AppError's, translated by the server), else the fallback.
export function errorMessage(error: unknown, fallback: string): string {
    const message = (error as {message?: string} | undefined)?.message;
    return typeof message === 'string' && message.trim() ? message : fallback;
}

export function totalTokens(t: AIUsageTotals | undefined): number {
    return t ? t.input_tokens + t.output_tokens : 0;
}

export type DayUsage = {day: number; requests: number; tokens: number};

// usageByDay adds up a month's usage rows (one per person, purpose and day) per day, oldest first.
export function usageByDay(usage: AIConnectionUsage | undefined): DayUsage[] {
    const byDay = new Map<number, DayUsage>();
    for (const row of usage?.days || []) {
        const d = byDay.get(row.day) || {day: row.day, requests: 0, tokens: 0};
        d.requests += row.requests;
        d.tokens += row.input_tokens + row.output_tokens;
        byDay.set(row.day, d);
    }
    return [...byDay.values()].sort((a, b) => a.day - b.day);
}

// dayDate turns a usage day (20261017, UTC) into a date.
export function dayDate(day: number): Date {
    return new Date(Date.UTC(Math.floor(day / 10000), (Math.floor(day / 100) % 100) - 1, day % 100));
}

// The purposes of usage: people (through apps), the built-in features, and apps' own ("plugin:<id>").
export const BUILT_IN_PURPOSES = ['user', 'rewrite', 'recaps', 'auto_translation', 'text_to_speech'];

export function sortedPurposes(usage: AIConnectionUsage | undefined): Array<[string, AIUsageTotals]> {
    const entries = Object.entries(usage?.by_purpose || {});
    const rank = (p: string) => {
        const i = BUILT_IN_PURPOSES.indexOf(p);
        return i === -1 ? BUILT_IN_PURPOSES.length : i;
    };
    return entries.sort(([a], [b]) => (rank(a) - rank(b)) || a.localeCompare(b));
}

// The default models a connection names, each once, for the model pickers.
export function defaultModels(models: AIConnectionModels | undefined): string[] {
    const out: string[] = [];
    for (const cap of CAPABILITIES) {
        const m = models?.[cap];
        if (m && !out.includes(m)) {
            out.push(m);
        }
    }
    return out;
}
