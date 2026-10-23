// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {AIMCPAuth, AIMCPServer, AIMCPServerPatch, AIMCPServerRequest} from '@mattermost/types/ai_tools';

import {AI_CONNECTION_SCOPE_SERVER, isValidHeaderName, newHeaderRow} from './connections';
import type {HeaderRow, KeyAction} from './connections';

// MCP servers (Antimatter): the services whose tools agents may use, set up like the model connections. Each person
// signs in to one themselves (OAuth), it needs no sign-in, or an admin gives it one shared service account (its
// headers): then only its tools that read are offered. Its tools are named mcp.<slug>.<tool>; the slug never changes.

export type MCPDraft = {
    name: string;
    slug: string;

    // The slug follows the name until it's typed.
    slugTouched: boolean;
    url: string;
    auth: AIMCPAuth;
    clientId: string;
    secretAction: KeyAction;
    clientSecret: string;
    scopes: string;
    headersChanged: boolean;
    headers: HeaderRow[];
    allTeams: boolean;
    teamIds: string[];
};

export function newMCPDraft(scope: string): MCPDraft {
    return {
        name: '',
        slug: '',
        slugTouched: false,
        url: '',
        auth: 'oauth',
        clientId: '',
        secretAction: 'replace',
        clientSecret: '',
        scopes: '',
        headersChanged: true,
        headers: [],
        allTeams: scope === AI_CONNECTION_SCOPE_SERVER,
        teamIds: [],
    };
}

export function draftFromMCPServer(s: AIMCPServer): MCPDraft {
    return {
        name: s.name,
        slug: s.slug,
        slugTouched: true,
        url: s.url,
        auth: s.auth,
        clientId: s.oauth_registered ? '' : s.oauth_client_id || '',
        secretAction: 'keep',
        clientSecret: '',
        scopes: s.oauth_scopes || '',
        headersChanged: false,
        headers: (s.header_names || []).map((name) => newHeaderRow(name)),
        allTeams: Boolean(s.sharing?.all_teams),
        teamIds: s.sharing?.team_ids || [],
    };
}

// slugFromName gives a slug for a name: lower-case letters, digits, - and _, starting with a letter or digit, at most
// 32.
export function slugFromName(name: string): string {
    return name.toLowerCase().normalize('NFKD').replace(/[̀-ͯ]/g, '').replace(/[^a-z0-9_-]+/g, '-').replace(/^[-_]+/, '').slice(0, 32).replace(/-+$/, '');
}

export function isValidSlug(slug: string): boolean {
    return (/^[a-z0-9][a-z0-9_-]{0,31}$/).test(slug);
}

// isValidMCPURL tells whether the server takes an MCP server's address: http(s), with a host, without a password or a
// fragment (a query is fine).
export function isValidMCPURL(raw: string): boolean {
    if (!raw || raw.length > 1024) {
        return false;
    }
    try {
        const u = new URL(raw);
        return (u.protocol === 'http:' || u.protocol === 'https:') && Boolean(u.host) && !u.username && !u.password && !u.hash;
    } catch {
        return false;
    }
}

export type MCPProblem = 'name' | 'slug' | 'url' | 'headers';

export function mcpProblemsOf(draft: MCPDraft, editing: boolean): MCPProblem[] {
    const problems: MCPProblem[] = [];
    const name = draft.name.trim();
    if (!name || name.length > 64) {
        problems.push('name');
    }
    if (!editing && !isValidSlug(draft.slug)) {
        problems.push('slug');
    }
    if (!isValidMCPURL(draft.url.trim())) {
        problems.push('url');
    }
    if (draft.auth === 'service_account') {
        const named = draft.headers.filter((h) => h.name.trim());
        const needsHeaders = draft.headersChanged || draft.headers.length === 0;
        if ((needsHeaders && named.length === 0) || named.some((h) => !isValidHeaderName(h.name.trim()) || (draft.headersChanged && !h.value))) {
            problems.push('headers');
        }
    }
    return problems;
}

function headersOf(draft: MCPDraft): Record<string, string> {
    const out: Record<string, string> = {};
    for (const row of draft.headers) {
        const name = row.name.trim();
        if (name) {
            out[name] = row.value;
        }
    }
    return out;
}

function sharingOf(draft: MCPDraft, scope: string) {
    if (scope !== AI_CONNECTION_SCOPE_SERVER) {
        return {all_teams: false, team_ids: []};
    }
    return {all_teams: draft.allTeams, team_ids: draft.allTeams ? [] : draft.teamIds};
}

export function mcpRequestOf(draft: MCPDraft, scope: string): AIMCPServerRequest {
    const request: AIMCPServerRequest = {
        name: draft.name.trim(),
        slug: draft.slug,
        scope,
        url: draft.url.trim(),
        auth: draft.auth,
        sharing: sharingOf(draft, scope),
    };
    if (draft.auth === 'oauth') {
        request.oauth_client_id = draft.clientId.trim();
        request.oauth_client_secret = draft.clientSecret;
        request.oauth_scopes = draft.scopes.trim();
    }
    if (draft.auth === 'service_account') {
        request.headers = headersOf(draft);
    }
    return request;
}

// mcpPatchOf sends what the form edits; the secret only when replaced or removed, the headers only when changed.
export function mcpPatchOf(draft: MCPDraft, saved: AIMCPServer): AIMCPServerPatch {
    const patch: AIMCPServerPatch = {
        name: draft.name.trim(),
        url: draft.url.trim(),
        auth: draft.auth,
        sharing: sharingOf(draft, saved.scope),
    };
    if (draft.auth === 'oauth') {
        const clientId = draft.clientId.trim();
        if (clientId !== (saved.oauth_registered ? '' : saved.oauth_client_id || '')) {
            patch.oauth_client_id = clientId;
        }
        patch.oauth_scopes = draft.scopes.trim();
        if (draft.secretAction === 'replace' && draft.clientSecret) {
            patch.oauth_client_secret = draft.clientSecret;
        } else if (draft.secretAction === 'remove') {
            patch.oauth_client_secret = '';
        }
    }
    if (draft.auth === 'service_account' && (draft.headersChanged || saved.auth !== 'service_account')) {
        patch.headers = headersOf(draft);
    }
    return patch;
}

// The namespace of an MCP server's tools, for agents' policies.
export function mcpNamespace(slug: string): string {
    return `mcp.${slug}`;
}
