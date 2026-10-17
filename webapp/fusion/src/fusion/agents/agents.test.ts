// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {draftFromAgent, externalAgentExamples, newAgentDraft, patchOf, problemsOf, requestOf, validUsername} from './agents';
import type {AgentDraft} from './agents';

const built = (patch: Partial<AgentDraft> = {}): AgentDraft => ({...newAgentDraft(), username: 'researcher', connectionId: 'conn1', ...patch});

describe('fusion/agents/agents', () => {
    test('usernames follow the server\'s rules', () => {
        expect(validUsername('researcher')).toBe(true);
        expect(validUsername('co.der-2_x')).toBe(true);
        expect(validUsername('ab')).toBe(false);
        expect(validUsername('2fast')).toBe(false);
        expect(validUsername('Has Space')).toBe(false);
    });

    test('a built-in agent needs a connection; an external one doesn\'t', () => {
        expect(problemsOf(built({connectionId: ''}))).toContain('connection');
        expect(problemsOf(built({kind: 'external', connectionId: ''}))).toEqual([]);
    });

    test('tool turns are a whole number up to 250, empty for the default', () => {
        expect(problemsOf(built({maxToolTurns: ''}))).toEqual([]);
        expect(problemsOf(built({maxToolTurns: '250'}))).toEqual([]);
        expect(problemsOf(built({maxToolTurns: '251'}))).toContain('maxToolTurns');
        expect(problemsOf(built({maxToolTurns: '1.5'}))).toContain('maxToolTurns');
    });

    test('without Everyone, someone must be chosen', () => {
        expect(problemsOf(built({everyone: false}))).toContain('access');
        expect(problemsOf(built({everyone: false, channelIds: ['ch1']}))).toEqual([]);
    });

    test('an external agent is made with no runtime at all', () => {
        const req = requestOf(built({kind: 'external', model: 'gpt', systemPrompt: 'secret', reasoningEffort: 'high', maxToolTurns: '9'}), 'server');
        expect(req).toEqual(expect.objectContaining({external: true, connection_id: '', model: '', system_prompt: '', max_tool_turns: 0, scope: 'server'}));
        expect(req.reasoning.enabled).toBe(false);
    });

    test('a built-in agent\'s request carries its settings, and Everyone clears the lists', () => {
        const req = requestOf(built({username: ' Researcher ', model: ' gpt-5 ', reasoningEffort: 'medium', maxToolTurns: '12', channelIds: ['ch1']}), 'team1');
        expect(req).toEqual(expect.objectContaining({username: 'researcher', model: 'gpt-5', max_tool_turns: 12, scope: 'team1', external: false}));
        expect(req.reasoning).toEqual({enabled: true, effort: 'medium', budget_tokens: 0});
        expect(req.access.channel_ids).toEqual([]);
    });

    test('a patch never changes the username, the scope or the kind', () => {
        const patch = patchOf(built({displayName: 'R'})) as Record<string, unknown>;
        expect(patch.username).toBeUndefined();
        expect(patch.scope).toBeUndefined();
        expect(patch.external).toBeUndefined();
        expect(patch.display_name).toBe('R');
        expect(patch.connection_id).toBe('conn1');
        expect((patchOf(built({kind: 'external'})) as Record<string, unknown>).connection_id).toBeUndefined();
    });

    test('a saved agent becomes the same draft', () => {
        const draft = draftFromAgent({
            user_id: 'u', scope: 'server', external: false, connection_id: 'c', model: 'm', system_prompt: 'p',
            tools: {enabled: false, allowed: [], auto_run: []}, reasoning: {enabled: true, effort: 'high', budget_tokens: 0},
            max_tool_turns: 40, access: {everyone: false, team_ids: [], channel_ids: ['ch'], roles: ['team_admin'], user_ids: ['u2'], allow_guests: true},
            needs_attention: '', creator_id: '', updater_id: '', create_at: 1, update_at: 1, delete_at: 0,
            username: 'r', display_name: 'R', description: 'd', last_icon_update: 0, enabled: true, connection_name: 'C',
        });
        expect(draft).toEqual(expect.objectContaining({kind: 'builtin', reasoningEffort: 'high', maxToolTurns: '40', channelIds: ['ch'], userIds: ['u2'], allowGuests: true}));
    });

    test('the external agent\'s help names its account and the activity endpoint', () => {
        const ex = externalAgentExamples('https://chat.example.com/', 'agent1');
        expect(ex.token).toContain('https://chat.example.com/api/v4/users/agent1/tokens');
        expect(ex.activity).toContain('/api/v4/ai/agents/agent1/activity');
    });
});
