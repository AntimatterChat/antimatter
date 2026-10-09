// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Channel} from '@mattermost/types/channels';

import {Client4} from 'mattermost-redux/client';

import {LAYER_ID} from 'fusion/components/layer';
import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';

import CategoryPermissionsDialog from './category_permissions_dialog';
import ChannelPermissionsDialog from './channel_permissions_dialog';

describe('ChannelPermissionsDialog', () => {
    const channel = {id: 'dev', team_id: 'team', type: 'O', display_name: 'Dev'} as Channel;
    const mods = {id: 'mods', display_name: 'Mods', source: 'custom', delete_at: 0, team_id: 'team', position: 0};
    const override = {channel_id: 'dev', target_type: 'role' as const, target_id: 'mods', allow: [], deny: [], update_at: 0};

    const render = (filedIn?: string) => renderWithContext(
        <ChannelPermissionsDialog
            channel={channel}
            onClose={jest.fn()}
        />,
        {
            entities: {
                general: {config: {FeatureFlagManagedChannelCategories: 'true'}},
                groups: {groups: {mods}},
                users: {currentUserId: 'me', profiles: {ada: {id: 'ada', username: 'ada'}}},
                channelCategories: {managedCategoryMappings: {team: filedIn ? {dev: filedIn} : {}}},
            },
        } as never,
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

    test('a channel shows what its category gives, line by line', async () => {
        jest.spyOn(Client4, 'getChannelOverrides').mockResolvedValue([]);
        jest.spyOn(Client4, 'getCategoryOverrides').mockResolvedValue([
            {team_id: 'team', category: 'community', target_type: 'role', target_id: 'mods', allow: [], deny: ['create_post'], update_at: 0},
            {team_id: 'team', category: 'rules', target_type: 'role', target_id: 'mods', allow: ['upload_file'], deny: [], update_at: 0},
        ]);
        render('Community');

        expect(await screen.findByText('denied by the category')).toBeInTheDocument();
        expect(screen.queryByText('allowed by the category')).not.toBeInTheDocument();
        expect(screen.getByText(/inherits the permissions of the category Community/)).toBeInTheDocument();
    });

    test('a person can be added, and their first change saves them', async () => {
        jest.spyOn(Client4, 'getChannelOverrides').mockResolvedValue([]);
        jest.spyOn(Client4, 'autocompleteUsers').mockResolvedValue({users: [{id: 'ada', username: 'ada'}]} as never);
        const save = jest.spyOn(Client4, 'saveChannelOverride').mockImplementation((_, o) => Promise.resolve({...override, ...o}));
        render();

        await userEvent.type(await screen.findByRole('textbox', {name: 'Add a person…'}), 'ad');
        await userEvent.click(await screen.findByRole('button', {name: /ada/}));
        const deny = await screen.findAllByRole('radio', {name: 'Denied here'});
        await userEvent.click(deny[0]);

        await waitFor(() => expect(save).toHaveBeenCalledWith('dev', {target_type: 'user', target_id: 'ada', allow: [], deny: ['create_post']}));
    });
});

describe('CategoryPermissionsDialog', () => {
    beforeEach(() => {
        const layer = document.createElement('div');
        layer.id = LAYER_ID;
        document.body.appendChild(layer);
    });

    afterEach(() => {
        document.getElementById(LAYER_ID)?.remove();
        jest.restoreAllMocks();
    });

    test('lists the category\'s overrides and saves them under its name', async () => {
        jest.spyOn(Client4, 'getCategoryOverrides').mockResolvedValue([
            {team_id: 'team', category: 'community', target_type: 'role', target_id: 'mods', allow: [], deny: [], update_at: 0},
            {team_id: 'team', category: 'rules', target_type: 'role', target_id: 'other', allow: [], deny: ['create_post'], update_at: 0},
        ]);
        const save = jest.spyOn(Client4, 'saveCategoryOverride').mockResolvedValue({} as never);
        renderWithContext(
            <CategoryPermissionsDialog
                teamId='team'
                name='Community'
                onClose={jest.fn()}
            />,
            {entities: {groups: {groups: {mods: {id: 'mods', display_name: 'Mods', source: 'custom', delete_at: 0, team_id: 'team', position: 0}}}}} as never,
        );

        const deny = await screen.findAllByRole('radio', {name: 'Denied here'});
        expect(deny).toHaveLength(8);
        await userEvent.click(deny[0]);
        await waitFor(() => expect(save).toHaveBeenCalledWith('team', {category: 'Community', target_type: 'role', target_id: 'mods', allow: [], deny: ['create_post']}));
    });
});
