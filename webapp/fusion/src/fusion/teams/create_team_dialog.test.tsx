// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {createTeam} from 'mattermost-redux/actions/teams';

import {LAYER_ID} from 'fusion/components/layer';
import {renderWithContext, screen, userEvent, waitFor, within} from 'tests/react_testing_utils';
import {getHistory} from 'utils/browser_history';

import CreateTeamDialog from './create_team_dialog';

// The server makes the address from the name.
jest.mock('mattermost-redux/actions/teams', () => ({
    ...jest.requireActual('mattermost-redux/actions/teams'),
    createTeam: jest.fn((team: {display_name: string}) => ({type: 'CREATED', data: {...team, id: 'new', name: team.display_name.toLowerCase().replace(/ /g, '-')}})),
}));

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

    const render = (defaultKind = '') => {
        const onClose = jest.fn();
        renderWithContext(<CreateTeamDialog onClose={onClose}/>, {entities: {general: {config: {DefaultCommunityKind: defaultKind}}}} as never);
        return onClose;
    };

    test('creates an open guild with the address the server makes, and opens it', async () => {
        const push = jest.spyOn(getHistory(), 'push').mockImplementation(() => undefined);
        const onClose = render();
        expect(screen.queryByRole('textbox', {name: 'Address'})).not.toBeInTheDocument();
        await userEvent.type(screen.getByRole('textbox', {name: 'Name'}), 'Detector Physics');
        expect(within(screen.getByRole('group', {name: 'Kind'})).getByRole('button', {name: 'Guild'})).toHaveAttribute('aria-pressed', 'true');
        await userEvent.click(screen.getByRole('button', {name: 'Create community'}));

        await waitFor(() => expect(onClose).toHaveBeenCalled());
        const sent = (createTeam as jest.Mock).mock.calls[0][0];
        expect(sent).toEqual({display_name: 'Detector Physics', type: 'O', allow_open_invite: true, kind: 'guild'});
        expect(sent.name).toBeUndefined();
        expect(push).toHaveBeenCalledWith('/detector-physics/channels/general');
    });

    test('makes a workspace', async () => {
        jest.spyOn(getHistory(), 'push').mockImplementation(() => undefined);
        render();
        await userEvent.type(screen.getByRole('textbox', {name: 'Name'}), 'Acme');
        await userEvent.click(within(screen.getByRole('group', {name: 'Kind'})).getByRole('button', {name: 'Workspace'}));
        expect(screen.getByText(/members join the channels they want/)).toBeInTheDocument();
        await userEvent.click(screen.getByRole('button', {name: 'Create community'}));
        await waitFor(() => expect(createTeam).toHaveBeenCalledWith(expect.objectContaining({kind: 'workspace'})));
    });

    test('starts from the server\'s kind of new communities', () => {
        render('workspace');
        expect(within(screen.getByRole('group', {name: 'Kind'})).getByRole('button', {name: 'Workspace'})).toHaveAttribute('aria-pressed', 'true');
    });

    test('a name too short can\'t be used', async () => {
        render();
        await userEvent.type(screen.getByRole('textbox', {name: 'Name'}), 'x');
        expect(screen.getByRole('button', {name: 'Create community'})).toBeDisabled();
    });

    test('an invitation-only community', async () => {
        jest.spyOn(getHistory(), 'push').mockImplementation(() => undefined);
        render();
        await userEvent.type(screen.getByRole('textbox', {name: 'Name'}), 'Secret lab');
        await userEvent.click(screen.getByRole('switch'));
        await waitFor(() => expect(screen.getByRole('button', {name: 'Create community'})).toBeEnabled());
        await userEvent.click(screen.getByRole('button', {name: 'Create community'}));
        await waitFor(() => expect(createTeam).toHaveBeenCalledWith(expect.objectContaining({type: 'I', allow_open_invite: false})));
    });

    test('tells why the community wasn\'t created', async () => {
        (createTeam as jest.Mock).mockReturnValueOnce({type: 'FAILED', error: {message: 'Too many communities'}});
        const onClose = render();
        await userEvent.type(screen.getByRole('textbox', {name: 'Name'}), 'Lumen');
        await userEvent.click(screen.getByRole('button', {name: 'Create community'}));
        expect(await screen.findByRole('alert')).toHaveTextContent('Too many communities');
        expect(onClose).not.toHaveBeenCalled();
    });
});
