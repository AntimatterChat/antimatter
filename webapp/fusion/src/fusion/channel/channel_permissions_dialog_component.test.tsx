// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Channel} from '@mattermost/types/channels';

import {Client4} from 'mattermost-redux/client';

import {LAYER_ID} from 'fusion/components/layer';
import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';

import ChannelPermissionsDialog from './channel_permissions_dialog';

describe('ChannelPermissionsDialog', () => {
    const channel = {id: 'dev', team_id: 'team', type: 'O', display_name: 'Dev'} as Channel;
    const mods = {id: 'mods', display_name: 'Mods', source: 'custom', delete_at: 0, team_id: 'team', position: 0};
    const override = {channel_id: 'dev', target_type: 'role' as const, target_id: 'mods', allow: [], deny: [], update_at: 0};

    const render = () => renderWithContext(
        <ChannelPermissionsDialog
            channel={channel}
            onClose={jest.fn()}
        />,
        {entities: {groups: {groups: {mods}}}} as never,
    );

    beforeEach(() => {
        const layer = document.createElement('div');
        layer.id = LAYER_ID;
        document.body.appendChild(layer);
    });

    afterEach(() => {
        document.getElementById(LAYER_ID)?.remove();
        jest.restoreAllMocks();
    });

    test('quick changes each build on the one before', async () => {
        jest.spyOn(Client4, 'getChannelOverrides').mockResolvedValue([override]);
        const save = jest.spyOn(Client4, 'saveChannelOverride').mockImplementation((_, o) => new Promise((resolve) => setTimeout(() => resolve({...override, ...o}), 10)));
        render();

        const deny = await screen.findAllByRole('radio', {name: 'Denied here'});
        await userEvent.click(deny[0]);
        await userEvent.click(deny[1]);

        await waitFor(() => expect(save).toHaveBeenCalledTimes(2));
        expect(save.mock.calls[1][1].deny).toEqual(['create_post', 'add_reaction', 'remove_reaction']);
    });

    test('a failed save is undone', async () => {
        jest.spyOn(Client4, 'getChannelOverrides').mockResolvedValue([override]);
        jest.spyOn(Client4, 'saveChannelOverride').mockRejectedValue(new Error('nope'));
        render();

        const deny = await screen.findAllByRole('radio', {name: 'Denied here'});
        await userEvent.click(deny[0]);
        await waitFor(() => expect(screen.getAllByRole('radio', {name: 'Denied here'})[0]).toHaveAttribute('aria-checked', 'false'));
    });

    test('nothing can be edited over permissions that couldn\'t be loaded', async () => {
        jest.spyOn(Client4, 'getChannelOverrides').mockRejectedValue(new Error('offline'));
        render();

        expect(await screen.findByRole('alert')).toHaveTextContent('couldn\'t be loaded');
        expect(screen.getByRole('combobox', {name: 'Add a role'})).toBeDisabled();
    });
});
