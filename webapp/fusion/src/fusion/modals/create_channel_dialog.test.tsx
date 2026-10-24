// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Channel} from '@mattermost/types/channels';

import {Client4} from 'mattermost-redux/client';

import {LAYER_ID} from 'fusion/components/layer';
import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';

import CreateChannelDialog from './create_channel_dialog';

describe('CreateChannelDialog', () => {
    const render = () => {
        const onClose = jest.fn();
        const onMoreOptions = jest.fn();
        renderWithContext(
            <CreateChannelDialog
                onMoreOptions={onMoreOptions}
                onClose={onClose}
            />,
            {
                entities: {
                    teams: {
                        currentTeamId: 'team',
                        teams: {team: {id: 'team', name: 'lumen', display_name: 'Lumen'}},
                        myMembers: {team: {team_id: 'team', user_id: 'me', roles: 'test_role'}},
                    },
                    roles: {roles: {test_role: {name: 'test_role', permissions: ['create_public_channel', 'create_private_channel']}}},
                    users: {currentUserId: 'me'},
                },
            } as never,
        );
        return {onClose, onMoreOptions};
    };

    beforeEach(() => {
        const layer = document.createElement('div');
        layer.id = LAYER_ID;
        document.body.appendChild(layer);
    });

    afterEach(() => {
        document.getElementById(LAYER_ID)?.remove();
        jest.restoreAllMocks();
    });

    test('creating a text channel sends its preset as props.am_preset', async () => {
        const create = jest.spyOn(Client4, 'createChannel').mockResolvedValue({id: 'new', name: 'beam', display_name: 'Beam', team_id: 'team', type: 'O', create_at: 0} as Channel);
        render();

        await userEvent.type(screen.getByLabelText('Name'), 'Beam');
        await userEvent.click(screen.getByRole('button', {name: 'Create channel'}));

        await waitFor(() => expect(create).toHaveBeenCalled());
        const sent = create.mock.calls[0][0] as Channel;
        expect(sent.props).toEqual({am_preset: 'text_chat'});
        expect(sent.type).toBe('O');
    });

    test('picking another preset sends it, and switching type picks that type\'s default', async () => {
        const create = jest.spyOn(Client4, 'createChannel').mockResolvedValue({id: 'new', name: 'news', display_name: 'News', team_id: 'team', type: 'O', create_at: 0} as Channel);
        render();

        await userEvent.click(screen.getByRole('radio', {name: /^Announcement/}));
        await userEvent.type(screen.getByLabelText('Name'), 'News');
        await userEvent.click(screen.getByRole('button', {name: 'Create channel'}));

        await waitFor(() => expect(create).toHaveBeenCalled());
        expect((create.mock.calls[0][0] as Channel).props).toEqual({am_preset: 'text_announcement'});
    });

    test('switching to Forum shows forum presets, and the first is sent', async () => {
        const create = jest.spyOn(Client4, 'createChannel').mockResolvedValue({id: 'new', name: 'help', display_name: 'Help', team_id: 'team', type: 'O', create_at: 0} as Channel);
        render();

        await userEvent.click(screen.getByRole('radio', {name: 'Forum'}));
        expect(screen.getByRole('radio', {name: /^Discussion/})).toHaveAttribute('aria-checked', 'true');

        await userEvent.type(screen.getByLabelText('Name'), 'Help');
        await userEvent.click(screen.getByRole('button', {name: 'Create channel'}));

        await waitFor(() => expect(create).toHaveBeenCalled());
        expect((create.mock.calls[0][0] as Channel).props).toEqual({am_preset: 'forum_agora'});
    });

    test('the Voice segment is disabled while the voice-channels app is not active', () => {
        render();
        expect(screen.getByRole('radio', {name: 'Voice'})).toBeDisabled();
    });
});
