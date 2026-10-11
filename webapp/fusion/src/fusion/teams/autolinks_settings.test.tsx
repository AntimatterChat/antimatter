// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {patchTeam} from 'mattermost-redux/actions/teams';
import {Client4} from 'mattermost-redux/client';

import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';

import AutolinksSettings from './autolinks_settings';

jest.mock('mattermost-redux/actions/teams', () => ({
    ...jest.requireActual('mattermost-redux/actions/teams'),
    patchTeam: jest.fn(() => ({type: 'PATCHED', data: {}})),
}));

const patch = patchTeam as jest.Mock;

describe('AutolinksSettings', () => {
    const saved = JSON.stringify({links: [{name: 'Jira', pattern: '(LUM-\\d+)', template: '[$1](https://jira/browse/$1)'}]});
    let preview: jest.SpyInstance;

    const render = (autolinks = saved) => {
        const setUnsaved = jest.fn();
        renderWithContext(
            <AutolinksSettings setUnsaved={setUnsaved}/>,
            {
                entities: {
                    teams: {currentTeamId: 'team', teams: {team: {id: 'team', name: 'lumen', autolinks}}},
                    channels: {
                        channels: {dev: {id: 'dev', team_id: 'team', type: 'O', display_name: 'dev', name: 'dev', delete_at: 0}},
                        channelsInTeam: {team: new Set(['dev'])},
                        myMembers: {dev: {channel_id: 'dev', user_id: 'me'}},
                    },
                },
            },
        );
        return setUnsaved;
    };

    beforeEach(() => {
        patch.mockClear();
        preview = jest.spyOn(Client4, 'previewTeamAutolinks').mockImplementation(async (_team, _links, text) => ({text: text.replace('LUM-1', '[LUM-1](https://jira/browse/LUM-1)')}));
    });

    afterEach(() => {
        preview.mockRestore();
    });

    test('shows the team\'s rules', () => {
        render();
        expect(screen.getByDisplayValue('Jira')).toBeInTheDocument();
        expect(screen.getByDisplayValue('(LUM-\\d+)')).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Save'})).toBeDisabled();
    });

    test('says when there are none', () => {
        render('');
        expect(screen.getByText('No autolinks yet.')).toBeInTheDocument();
    });

    test('previews a message with the rules from the server', async () => {
        render();
        const sample = screen.getByRole('textbox', {name: 'Try a message'});
        await userEvent.clear(sample);
        await userEvent.type(sample, 'See LUM-1');
        await waitFor(() => expect(screen.getByRole('link', {name: 'LUM-1'})).toHaveAttribute('href', 'https://jira/browse/LUM-1'));
    });

    test('adds a rule from a preset and saves the team', async () => {
        const setUnsaved = render();
        await userEvent.click(screen.getByRole('button', {name: 'Add an autolink'}));
        await userEvent.click(screen.getByRole('button', {name: 'Pull requests'}));
        expect(screen.getByDisplayValue('#(\\d+)')).toBeInTheDocument();
        await waitFor(() => expect(setUnsaved).toHaveBeenLastCalledWith(true));

        await userEvent.click(screen.getByRole('button', {name: 'Save'}));
        await waitFor(() => expect(patch).toHaveBeenCalled());
        const autolinks = JSON.parse(patch.mock.calls[0][0].autolinks);
        expect(autolinks.links.map((l: {name: string}) => l.name)).toEqual(['Jira', 'Pull requests']);
        await waitFor(() => expect(setUnsaved).toHaveBeenLastCalledWith(false));
    });

    test('doesn\'t save rules the server refuses', async () => {
        render();
        preview.mockRejectedValue({detailed_error: 'autolink 1: missing closing )'});
        await userEvent.type(screen.getByDisplayValue('(LUM-\\d+)'), '(');
        await userEvent.click(screen.getByRole('button', {name: 'Save'}));
        expect(await screen.findByRole('alert')).toHaveTextContent('autolink 1: missing closing )');
        expect(patch).not.toHaveBeenCalled();
    });

    test('removes a rule', async () => {
        render();
        await userEvent.click(screen.getByRole('button', {name: 'Remove the autolink'}));
        expect(screen.getByText('No autolinks yet.')).toBeInTheDocument();
    });
});
