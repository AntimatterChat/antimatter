// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Channel, ServerChannel} from '@mattermost/types/channels';

import {Client4} from 'mattermost-redux/client';

import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';

import ChannelSettingsTypeTab from './channel_settings_type_tab';

describe('ChannelSettingsTypeTab', () => {
    const render = (channel: Channel, channelTypes: Record<string, unknown>) => renderWithContext(
        <ChannelSettingsTypeTab channel={channel}/>,
        {
            entities: {
                channels: {channelTypes},
            },
        } as never,
    );

    test('picking another preset saves its mode and root_posts', async () => {
        const channel = {id: 'c1', team_id: 'team', type: 'O', display_name: 'Beam'} as Channel;
        const update = jest.spyOn(Client4, 'updateChannelType').mockResolvedValue({id: 'c1', props: {}} as ServerChannel);
        render(channel, {c1: {type: 'text', mode: 'chat', root_posts: 'members', preset: 'text_chat'}});

        await userEvent.click(screen.getByRole('radio', {name: /^Announcement/}));
        await userEvent.click(screen.getByTestId('SaveChangesPanel__save-btn'));

        await waitFor(() => expect(update).toHaveBeenCalledWith('c1', {mode: 'chat', root_posts: 'moderators'}));
    });

    test('a forum\'s save carries its default view, and tuning root_posts alone keeps the mode', async () => {
        const channel = {id: 'c2', team_id: 'team', type: 'O', display_name: 'Help'} as Channel;
        const update = jest.spyOn(Client4, 'updateChannelType').mockResolvedValue({id: 'c2', props: {}} as ServerChannel);
        render(channel, {c2: {type: 'forum', mode: 'discussion', root_posts: 'members', view: 'topics', preset: 'forum_agora'}});

        await userEvent.click(screen.getByRole('button', {name: 'Moderators'}));
        await userEvent.click(screen.getByTestId('SaveChangesPanel__save-btn'));

        await waitFor(() => expect(update).toHaveBeenCalledWith('c2', {mode: 'discussion', root_posts: 'moderators', view: 'topics'}));
    });

    test('a failed save shows the error and keeps the save panel open', async () => {
        const channel = {id: 'c3', team_id: 'team', type: 'O', display_name: 'Beam'} as Channel;
        jest.spyOn(Client4, 'updateChannelType').mockRejectedValue({message: 'nope'});
        render(channel, {c3: {type: 'text', mode: 'chat', root_posts: 'members', preset: 'text_chat'}});

        await userEvent.click(screen.getByRole('radio', {name: /^Announcement/}));
        await userEvent.click(screen.getByTestId('SaveChangesPanel__save-btn'));

        expect(await screen.findByText('nope')).toBeInTheDocument();
    });
});
