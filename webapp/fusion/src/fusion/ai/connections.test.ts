// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {AIConnection} from '@mattermost/types/ai_connections';

import {
    draftFromConnection,
    endpointChanged,
    newDraft,
    newHeaderRow,
    parseLimit,
    patchOf,
    presetsFor,
    problemsOf,
    requestOf,
    sortedPurposes,
    testRequestNeedsSavedSecrets,
    usageByDay,
} from './connections';

const saved: AIConnection = {
    id: 'c1',
    name: 'Lumen',
    scope: 'server',
    provider: 'openai_compatible',
    base_url: 'https://api.openai.com/v1',
    api_key_set: true,
    api_key_last4: 'wxyz',
    extra_header_names: ['X-Org'],
    models: {chat: 'gpt-5', embeddings: 'text-embedding-3-small', transcription: '', decisions: ''},
    decisions_dialect: 'openai',
    limits: {requests_per_minute: 60, monthly_token_budget: 0, per_user_requests_per_minute: 0, per_user_monthly_token_budget: 0},
    sharing: {all_teams: false, team_ids: ['t1']},
    audience: {everyone: true, group_ids: [], roles: [], allow_guests: false},
    needs_attention: '',
    creator_id: '',
    updater_id: '',
    create_at: 1,
    update_at: 1,
    delete_at: 0,
};

describe('fusion/ai/connections', () => {
    test('a new server-wide connection is shared with every community; a community\'s with none', () => {
        expect(newDraft('server').allTeams).toBe(true);
        expect(requestOf(newDraft('t1'), 't1').sharing).toEqual({all_teams: false, team_ids: []});
        expect(newDraft('server').audience).toEqual({everyone: true, group_ids: [], roles: [], allow_guests: false});
    });

    test('an edited connection keeps its key and headers unless they change', () => {
        const draft = draftFromConnection(saved);
        const patch = patchOf(draft, 'server');
        expect(patch).not.toHaveProperty('api_key');
        expect(patch).not.toHaveProperty('extra_headers');
        expect(patch).not.toHaveProperty('scope');
        expect(patch.limits).toEqual(saved.limits);
        expect(patch.sharing).toEqual({all_teams: false, team_ids: ['t1']});

        expect(patchOf({...draft, keyAction: 'replace', apiKey: ' sk-new '}, 'server').api_key).toBe('sk-new');
        expect(patchOf({...draft, keyAction: 'remove'}, 'server').api_key).toBe('');
        expect(patchOf({...draft, keyAction: 'replace', apiKey: ''}, 'server')).not.toHaveProperty('api_key');
        expect(patchOf({...draft, headersChanged: true, headers: [newHeaderRow('X-Org', 'acme')]}, 'server').extra_headers).toEqual({'X-Org': 'acme'});
        expect(patchOf({...draft, headersChanged: true, headers: []}, 'server').extra_headers).toEqual({});
    });

    test('a community\'s connection is patched without sharing', () => {
        expect(patchOf(draftFromConnection({...saved, scope: 't1'}), 't1')).not.toHaveProperty('sharing');
    });

    test('an Anthropic connection only has a chat model', () => {
        const request = requestOf({...draftFromConnection(saved), provider: 'anthropic'}, 'server');
        expect(request.models).toEqual({chat: 'gpt-5', embeddings: '', transcription: '', decisions: ''});
    });

    test('everyone clears the chosen audience', () => {
        const draft = {...newDraft('server'), audience: {everyone: true, group_ids: ['g1'], roles: ['team_admin'], allow_guests: true}};
        expect(requestOf(draft, 'server').audience).toEqual({everyone: true, group_ids: [], roles: [], allow_guests: true});
    });

    test('reads limits as whole numbers, empty being no limit', () => {
        expect(parseLimit('')).toBe(0);
        expect(parseLimit(' 1 000 000 ')).toBe(1000000);
        expect(parseLimit('1,000')).toBe(1000);
        expect(parseLimit('-1')).toBeUndefined();
        expect(parseLimit('1.5')).toBeUndefined();
        expect(parseLimit('lots')).toBeUndefined();
    });

    test('finds what the server would refuse', () => {
        const draft = newDraft('server');
        expect(problemsOf(draft)).toEqual(['name']);
        expect(problemsOf({...draft, name: 'x', baseURL: 'ftp://host'})).toEqual(['baseURL']);
        expect(problemsOf({...draft, name: 'x', baseURL: 'https://user:pw@host/v1'})).toEqual(['baseURL']);
        expect(problemsOf({...draft, name: 'x', baseURL: 'https://host/v1?x=1'})).toEqual(['baseURL']);
        expect(problemsOf({...draft, name: 'x', baseURL: 'http://localhost:11434/v1'})).toEqual([]);
        expect(problemsOf({...draft, name: 'x', limits: {...draft.limits, requests_per_minute: '-3'}})).toEqual(['limits']);
        expect(problemsOf({...draft, name: 'x', headers: [newHeaderRow('Cookie', 'a')]})).toEqual(['headers']);
        expect(problemsOf({...draft, name: 'x', headers: [newHeaderRow('X-Ok', 'a\nb')]})).toEqual(['headers']);
        expect(problemsOf({...draft, name: 'x', headers: [newHeaderRow('', '')]})).toEqual([]);
    });

    test('a changed address can\'t be tested with the saved key, which the form never has', () => {
        const draft = draftFromConnection(saved);
        expect(endpointChanged(draft, saved)).toBe(false);
        expect(endpointChanged({...draft, name: 'Renamed'}, saved)).toBe(false);
        const moved = {...draft, baseURL: 'https://openrouter.ai/api/v1'};
        expect(endpointChanged(moved, saved)).toBe(true);
        expect(testRequestNeedsSavedSecrets(moved, saved)).toBe(true);
        expect(testRequestNeedsSavedSecrets({...moved, keyAction: 'replace', apiKey: 'k', headersChanged: true, headers: []}, saved)).toBe(false);
    });

    test('a community is offered no preset on the server\'s own network', () => {
        expect(presetsFor('server').some((p) => p.local)).toBe(true);
        expect(presetsFor('t1').some((p) => p.local)).toBe(false);
    });

    test('adds up the usage per day and orders the uses', () => {
        const usage = {
            connection_id: 'c1',
            month: 202610,
            totals: {requests: 3, input_tokens: 30, output_tokens: 3},
            monthly_token_budget: 0,
            days: [
                {connection_id: 'c1', user_id: 'u1', purpose: 'user', day: 20261017, requests: 1, input_tokens: 10, output_tokens: 1},
                {connection_id: 'c1', user_id: '', purpose: 'rewrite', day: 20261016, requests: 1, input_tokens: 10, output_tokens: 1},
                {connection_id: 'c1', user_id: 'u2', purpose: 'user', day: 20261017, requests: 1, input_tokens: 10, output_tokens: 1},
            ],
            by_purpose: {'plugin:todo': {requests: 1, input_tokens: 1, output_tokens: 1}, recaps: {requests: 1, input_tokens: 1, output_tokens: 1}, user: {requests: 1, input_tokens: 1, output_tokens: 1}},
        };
        expect(usageByDay(usage)).toEqual([{day: 20261016, requests: 1, tokens: 11}, {day: 20261017, requests: 2, tokens: 22}]);
        expect(usageByDay({...usage, days: null})).toEqual([]);
        expect(sortedPurposes(usage).map(([p]) => p)).toEqual(['user', 'recaps', 'plugin:todo']);
    });
});
