// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {checkIfTeamExists, createTeam} from 'mattermost-redux/actions/teams';

import {LAYER_ID} from 'fusion/components/layer';
import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {getHistory} from 'utils/browser_history';

import CreateTeamDialog, {teamUrlFromName, teamUrlProblem} from './create_team_dialog';

jest.mock('mattermost-redux/actions/teams', () => ({
    ...jest.requireActual('mattermost-redux/actions/teams'),
    checkIfTeamExists: jest.fn((name: string) => ({type: 'CHECKED', data: name === 'taken'})),
    createTeam: jest.fn((team: {name: string}) => ({type: 'CREATED', data: team})),
}));

describe('team addresses', () => {
    test('a name suggests an address', () => {
        expect(teamUrlFromName('Detector Physics!')).toBe('detector-physics');
        expect(teamUrlFromName('  Ops  ')).toBe('ops');
    });

    test('an address follows Mattermost\'s rules', () => {
        expect(teamUrlProblem('detector-physics')).toBeUndefined();
        expect(teamUrlProblem('a')).toBe('length');
        expect(teamUrlProblem('Caps')).toBe('format');
        expect(teamUrlProblem('9lives')).toBe('format');
        expect(teamUrlProblem('ends-')).toBe('format');
        expect(teamUrlProblem('admin-team')).toBe('reserved');
    });
});

describe('CreateTeamDialog', () => {
    beforeEach(() => {
        const layer = document.createElement('div');
        layer.id = LAYER_ID;
        document.body.appendChild(layer);
        (createTeam as jest.Mock).mockClear();
    });

    afterEach(() => {
        document.getElementById(LAYER_ID)?.remove();
    });

    const render = () => {
        const onClose = jest.fn();
        renderWithContext(<CreateTeamDialog onClose={onClose}/>);
        return onClose;
    };

    test('creates an open team at the address its name suggests, and opens it', async () => {
        const push = jest.spyOn(getHistory(), 'push').mockImplementation(() => undefined);
        const onClose = render();
        await userEvent.type(screen.getByRole('textbox', {name: 'Name'}), 'Detector Physics');
        expect(screen.getByRole('textbox', {name: 'Address'})).toHaveValue('detector-physics');

        await waitFor(() => expect(checkIfTeamExists).toHaveBeenCalledWith('detector-physics'));
        await userEvent.click(screen.getByRole('button', {name: 'Create community'}));

        await waitFor(() => expect(onClose).toHaveBeenCalled());
        expect(createTeam).toHaveBeenCalledWith(expect.objectContaining({display_name: 'Detector Physics', name: 'detector-physics', type: 'O', allow_open_invite: true}));
        expect(push).toHaveBeenCalledWith('/detector-physics/channels/town-square');
    });

    test('an address that\'s taken or malformed can\'t be used', async () => {
        render();
        await userEvent.type(screen.getByRole('textbox', {name: 'Name'}), 'x');
        await userEvent.clear(screen.getByRole('textbox', {name: 'Address'}));
        await userEvent.type(screen.getByRole('textbox', {name: 'Address'}), 'taken');
        expect(await screen.findByText('That address is taken.')).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Create community'})).toBeDisabled();

        await userEvent.clear(screen.getByRole('textbox', {name: 'Address'}));
        await userEvent.type(screen.getByRole('textbox', {name: 'Address'}), '9lives');
        expect(screen.getByText(/Lower case letters, numbers and dashes/)).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Create community'})).toBeDisabled();
    });

    test('an invitation-only team', async () => {
        jest.spyOn(getHistory(), 'push').mockImplementation(() => undefined);
        render();
        await userEvent.type(screen.getByRole('textbox', {name: 'Name'}), 'Secret lab');
        await userEvent.click(screen.getByRole('switch'));
        await waitFor(() => expect(screen.getByRole('button', {name: 'Create community'})).toBeEnabled());
        await userEvent.click(screen.getByRole('button', {name: 'Create community'}));
        await waitFor(() => expect(createTeam).toHaveBeenCalledWith(expect.objectContaining({type: 'I', allow_open_invite: false})));
    });
});
