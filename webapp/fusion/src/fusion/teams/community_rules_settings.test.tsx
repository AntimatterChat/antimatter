// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {CommunityRules} from '@mattermost/types/community_rules';

import {Client4} from 'mattermost-redux/client';

import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';

import CommunityRulesSettings from './community_rules_settings';

const rules = (patch: Partial<CommunityRules> = {}): CommunityRules => ({
    team_id: 'team',
    enabled: false,
    text: '',
    version: 0,
    updated_by: '',
    update_at: 0,
    accepted_version: 0,
    must_accept: false,
    ...patch,
});

describe('CommunityRulesSettings', () => {
    let get: jest.SpyInstance;
    let update: jest.SpyInstance;
    let stats: jest.SpyInstance;

    const render = () => {
        const setUnsaved = jest.fn();
        renderWithContext(
            <CommunityRulesSettings setUnsaved={setUnsaved}/>,
            {entities: {teams: {currentTeamId: 'team', teams: {team: {id: 'team', name: 'lumen', display_name: 'Lumen Labs'}}}}},
        );
        return setUnsaved;
    };

    beforeEach(() => {
        get = jest.spyOn(Client4, 'getCommunityRules').mockResolvedValue(rules());
        update = jest.spyOn(Client4, 'updateCommunityRules').mockImplementation(async (_team, request) => rules({enabled: request.enabled, text: request.text, version: 1}));
        stats = jest.spyOn(Client4, 'getCommunityRulesStats').mockResolvedValue({team_id: 'team', version: 0, accepted_count: 0, member_count: 4});
    });

    afterEach(() => {
        get.mockRestore();
        update.mockRestore();
        stats.mockRestore();
    });

    test('turns the rules on with their text', async () => {
        const setUnsaved = render();
        const toggle = await screen.findByRole('switch', {name: 'Members must agree to the rules'});
        expect(toggle).toHaveAttribute('aria-checked', 'false');
        expect(screen.getByText('Once the rules are on, every member agrees to them before they post.')).toBeInTheDocument();
        expect(screen.queryByRole('checkbox', {name: /Ask everyone to agree again/})).not.toBeInTheDocument();

        await userEvent.click(toggle);
        await userEvent.type(screen.getByRole('textbox', {name: 'Rules (Markdown)'}), 'Be kind.');
        await waitFor(() => expect(setUnsaved).toHaveBeenLastCalledWith(true));
        await userEvent.click(screen.getByRole('button', {name: 'Save'}));

        await waitFor(() => expect(update).toHaveBeenCalledWith('team', {enabled: true, text: 'Be kind.', ask_again: false}));
        await waitFor(() => expect(setUnsaved).toHaveBeenLastCalledWith(false));
    });

    test('doesn\'t turn them on empty', async () => {
        render();
        await userEvent.click(await screen.findByRole('switch', {name: 'Members must agree to the rules'}));
        await userEvent.click(screen.getByRole('button', {name: 'Save'}));
        expect(screen.getByRole('alert')).toHaveTextContent('Write the rules before turning them on.');
        expect(update).not.toHaveBeenCalled();
    });

    test('previews the Markdown as members will see it', async () => {
        get.mockResolvedValue(rules({enabled: true, text: '**Be kind.**', version: 1}));
        render();
        await screen.findByDisplayValue('**Be kind.**');
        await userEvent.click(screen.getByRole('tab', {name: 'Preview'}));
        expect(screen.getByText('Be kind.').tagName).toBe('STRONG');
        await userEvent.click(screen.getByRole('tab', {name: 'Write'}));
        expect(screen.getByRole('textbox', {name: 'Rules (Markdown)'})).toBeInTheDocument();
    });

    test('rules in place can be fixed without asking again, or asked again; the count of who agreed shows', async () => {
        get.mockResolvedValue(rules({enabled: true, text: 'Be kind.', version: 3}));
        stats.mockResolvedValue({team_id: 'team', version: 3, accepted_count: 1, member_count: 4});
        render();
        expect(await screen.findByText('1 of 4 members agreed to the current rules.')).toBeInTheDocument();

        const text = screen.getByRole('textbox', {name: 'Rules (Markdown)'});
        await userEvent.type(text, ' No spam.');
        await userEvent.click(screen.getByRole('button', {name: 'Save'}));
        await waitFor(() => expect(update).toHaveBeenLastCalledWith('team', {enabled: true, text: 'Be kind. No spam.', ask_again: false}));

        await userEvent.click(screen.getByRole('checkbox', {name: /Ask everyone to agree again/}));
        await userEvent.click(screen.getByRole('button', {name: 'Save'}));
        await waitFor(() => expect(update).toHaveBeenLastCalledWith('team', {enabled: true, text: 'Be kind. No spam.', ask_again: true}));
    });
});
