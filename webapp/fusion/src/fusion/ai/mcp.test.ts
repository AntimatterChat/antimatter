// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {AIMCPServer} from '@mattermost/types/ai_tools';

import {newHeaderRow} from './connections';
import {draftFromMCPServer, isValidMCPURL, isValidSlug, mcpPatchOf, mcpProblemsOf, mcpRequestOf, newMCPDraft, slugFromName} from './mcp';

const saved = (patch: Partial<AIMCPServer> = {}): AIMCPServer => ({
    id: 's1',
    name: 'Tracker',
    slug: 'tracker',
    scope: 'server',
    url: 'https://mcp.example.com/mcp',
    auth: 'oauth',
    oauth_client_id: 'client',
    oauth_client_secret_set: true,
    oauth_scopes: 'read',
    oauth_registered: false,
    header_names: [],
    sharing: {all_teams: true, team_ids: []},
    needs_attention: '',
    creator_id: 'me',
    updater_id: 'me',
    create_at: 1,
    update_at: 1,
    delete_at: 0,
    ...patch,
});

describe('fusion/ai/mcp', () => {
    test('a slug follows the name', () => {
        expect(slugFromName('Jira Cloud')).toBe('jira-cloud');
        expect(slugFromName('  Équipe — Tickets  ')).toBe('equipe-tickets');
        expect(slugFromName('x'.repeat(40))).toHaveLength(32);
        expect(isValidSlug('jira_cloud-2')).toBe(true);
        expect(isValidSlug('-jira')).toBe(false);
        expect(isValidSlug('Jira')).toBe(false);
        expect(isValidSlug('')).toBe(false);
    });

    test('an address is http(s), without a password or a fragment', () => {
        expect(isValidMCPURL('https://mcp.example.com/mcp?team=1')).toBe(true);
        expect(isValidMCPURL('http://10.0.0.5:8080/mcp')).toBe(true);
        expect(isValidMCPURL('ftp://mcp.example.com')).toBe(false);
        expect(isValidMCPURL('https://user:pw@mcp.example.com')).toBe(false);
        expect(isValidMCPURL('https://mcp.example.com/#x')).toBe(false);
        expect(isValidMCPURL('')).toBe(false);
    });

    test('what keeps a draft from being saved', () => {
        const draft = {...newMCPDraft('server'), name: 'Tracker', slug: 'tracker', url: 'https://mcp.example.com/mcp'};
        expect(mcpProblemsOf(draft, false)).toEqual([]);
        expect(mcpProblemsOf({...draft, slug: 'Bad Slug'}, false)).toEqual(['slug']);
        expect(mcpProblemsOf({...draft, slug: 'Bad Slug'}, true)).toEqual([]);
        expect(mcpProblemsOf({...draft, auth: 'service_account', headers: []}, false)).toEqual(['headers']);
        expect(mcpProblemsOf({...draft, auth: 'service_account', headers: [newHeaderRow('Authorization', '')]}, false)).toEqual(['headers']);
        expect(mcpProblemsOf({...draft, auth: 'service_account', headers: [newHeaderRow('Authorization', 'Bearer x')]}, false)).toEqual([]);
    });

    test('a new server-wide server is shared with every community; a community\'s with none', () => {
        const draft = {...newMCPDraft('server'), name: ' Tracker ', slug: 'tracker', url: 'https://mcp.example.com/mcp', clientSecret: 's3cret'};
        expect(mcpRequestOf(draft, 'server')).toEqual({
            name: 'Tracker',
            slug: 'tracker',
            scope: 'server',
            url: 'https://mcp.example.com/mcp',
            auth: 'oauth',
            sharing: {all_teams: true, team_ids: []},
            oauth_client_id: '',
            oauth_client_secret: 's3cret',
            oauth_scopes: '',
        });
        expect(mcpRequestOf({...newMCPDraft('team1'), name: 'T', slug: 't', url: 'https://x.example'}, 'team1').sharing).toEqual({all_teams: false, team_ids: []});
    });

    test('a patch sends the secret only when replaced or removed, the headers only when changed', () => {
        const draft = draftFromMCPServer(saved());
        expect(draft.secretAction).toBe('keep');
        const patch = mcpPatchOf(draft, saved());
        expect(patch).not.toHaveProperty('oauth_client_secret');
        expect(patch).not.toHaveProperty('oauth_client_id');
        expect(mcpPatchOf({...draft, secretAction: 'remove'}, saved()).oauth_client_secret).toBe('');
        expect(mcpPatchOf({...draft, secretAction: 'replace', clientSecret: 'new'}, saved()).oauth_client_secret).toBe('new');

        const service = saved({auth: 'service_account', header_names: ['Authorization'], oauth_client_id: '', oauth_client_secret_set: false});
        const serviceDraft = draftFromMCPServer(service);
        expect(mcpPatchOf(serviceDraft, service)).not.toHaveProperty('headers');
        expect(mcpProblemsOf(serviceDraft, true)).toEqual([]);
        expect(mcpPatchOf({...serviceDraft, headersChanged: true, headers: [newHeaderRow('Authorization', 'Bearer y')]}, service).headers).toEqual({Authorization: 'Bearer y'});
    });

    test('a client the server registered itself isn\'t shown as the admin\'s', () => {
        expect(draftFromMCPServer(saved({oauth_registered: true, oauth_client_id: 'registered'})).clientId).toBe('');
        expect(mcpPatchOf(draftFromMCPServer(saved({oauth_registered: true, oauth_client_id: 'registered'})), saved({oauth_registered: true, oauth_client_id: 'registered'}))).not.toHaveProperty('oauth_client_id');
    });
});
