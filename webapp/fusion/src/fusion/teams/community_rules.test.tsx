// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {CommunityRules} from '@mattermost/types/community_rules';

import {TeamTypes} from 'mattermost-redux/action_types';
import {Client4} from 'mattermost-redux/client';
import {communityRules} from 'mattermost-redux/reducers/entities/teams';

import {LAYER_ID} from 'fusion/components/layer';
import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';

import type {GlobalState} from 'types/store';

import {mustAcceptCommunityRules} from './community_rules';
import {CommunityRulesGate} from './community_rules_dialog';
import TeamSetupHost from './team_setup_host';

const rules = (patch: Partial<CommunityRules> = {}): CommunityRules => ({
    team_id: 'team',
    enabled: true,
    text: '1. **Be kind.**\n2. No spam.',
    version: 2,
    updated_by: '',
    update_at: 1,
    accepted_version: 0,
    must_accept: true,
    ...patch,
});

const state = (communityRulesByTeam: Record<string, CommunityRules> = {}): Parameters<typeof renderWithContext>[1] => ({
    entities: {
        teams: {
            currentTeamId: 'team',
            teams: {team: {id: 'team', name: 'lumen', display_name: 'Lumen Labs'}},
            communityRules: communityRulesByTeam,
        },
        channels: {
            channels: {
                general: {id: 'general', team_id: 'team', type: 'O', name: 'general', display_name: 'General', delete_at: 0},
                dm: {id: 'dm', team_id: '', type: 'D', name: 'me__you', display_name: 'You', delete_at: 0},
            },
        },
        users: {currentUserId: 'me', profiles: {me: {id: 'me', username: 'maya'}}},
    },
});

describe('community rules', () => {
    let fetchRules: jest.SpyInstance;
    let accept: jest.SpyInstance;

    beforeEach(() => {
        const layer = document.createElement('div');
        layer.id = LAYER_ID;
        document.body.appendChild(layer);
        fetchRules = jest.spyOn(Client4, 'getCommunityRules').mockResolvedValue(rules());
        accept = jest.spyOn(Client4, 'acceptCommunityRules').mockResolvedValue(rules({accepted_version: 2, must_accept: false}));
    });

    afterEach(() => {
        document.getElementById(LAYER_ID)?.remove();
        fetchRules.mockRestore();
        accept.mockRestore();
    });

    test('the reducer keeps each team\'s rules as the server gave them', () => {
        const next = communityRules({}, {type: TeamTypes.RECEIVED_COMMUNITY_RULES, data: rules()});
        expect(next.team.version).toBe(2);
        expect(mustAcceptCommunityRules({entities: {teams: {communityRules: next}}} as unknown as GlobalState, 'team')).toBe(true);
        expect(mustAcceptCommunityRules({entities: {teams: {communityRules: next}}} as unknown as GlobalState, 'other')).toBe(false);
        expect(mustAcceptCommunityRules({entities: {teams: {}}} as unknown as GlobalState, 'team')).toBe(false);
    });

    test('the message box shows when there are no rules to agree to', () => {
        renderWithContext(<CommunityRulesGate channelId='general'><textarea aria-label='Message'/></CommunityRulesGate>, state({team: rules({must_accept: false, accepted_version: 2})}));
        expect(screen.getByRole('textbox', {name: 'Message'})).toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Read and agree'})).not.toBeInTheDocument();
    });

    test('direct messages have no community, so no rules', () => {
        renderWithContext(<CommunityRulesGate channelId='dm'><textarea aria-label='Message'/></CommunityRulesGate>, state({team: rules()}));
        expect(screen.getByRole('textbox', {name: 'Message'})).toBeInTheDocument();
    });

    test('until one agrees, a notice takes the place of the message box; agreeing brings it back', async () => {
        renderWithContext(<CommunityRulesGate channelId='general'><textarea aria-label='Message'/></CommunityRulesGate>, state({team: rules()}));
        expect(screen.queryByRole('textbox', {name: 'Message'})).not.toBeInTheDocument();
        expect(screen.getByText('Agree to the rules to post here')).toBeInTheDocument();

        await userEvent.click(screen.getByRole('button', {name: 'Read and agree'}));
        expect(screen.getByRole('dialog', {name: 'Rules of Lumen Labs'})).toBeInTheDocument();
        expect(screen.getByText('Be kind.')).toBeInTheDocument();

        await userEvent.click(screen.getByRole('button', {name: 'I agree'}));
        expect(accept).toHaveBeenCalledWith('team', 2);
        await waitFor(() => expect(screen.getByRole('textbox', {name: 'Message'})).toBeInTheDocument());
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });

    test('rules changed while reading are shown again before agreeing', async () => {
        accept.mockRejectedValue(Object.assign(new Error('changed'), {status_code: 409}));
        fetchRules.mockResolvedValue(rules({version: 3, text: 'New rules'}));
        renderWithContext(<CommunityRulesGate channelId='general'><textarea aria-label='Message'/></CommunityRulesGate>, state({team: rules()}));
        await userEvent.click(screen.getByRole('button', {name: 'Read and agree'}));
        await userEvent.click(screen.getByRole('button', {name: 'I agree'}));
        expect(await screen.findByRole('alert')).toHaveTextContent('These rules just changed: read them again, then agree.');
        expect(await screen.findByText('New rules')).toBeInTheDocument();
        expect(screen.queryByRole('textbox', {name: 'Message'})).not.toBeInTheDocument();
    });

    test('opening a community whose rules one hasn\'t agreed to shows them; "Not now" puts them off', async () => {
        renderWithContext(<TeamSetupHost/>, state({team: rules()}));
        expect(await screen.findByRole('dialog', {name: 'Rules of Lumen Labs'})).toBeInTheDocument();
        await userEvent.click(screen.getByRole('button', {name: 'Not now'}));
        expect(screen.queryByRole('dialog', {name: 'Rules of Lumen Labs'})).not.toBeInTheDocument();
        expect(accept).not.toHaveBeenCalled();
    });

    test('a community whose rules one agreed to opens without them', () => {
        fetchRules.mockResolvedValue(rules({accepted_version: 2, must_accept: false}));
        renderWithContext(<TeamSetupHost/>, state({team: rules({accepted_version: 2, must_accept: false})}));
        expect(screen.queryByRole('dialog', {name: 'Rules of Lumen Labs'})).not.toBeInTheDocument();
    });
});
