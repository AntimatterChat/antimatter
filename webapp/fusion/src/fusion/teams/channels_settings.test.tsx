// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Channel} from '@mattermost/types/channels';
import type {TeamKind} from '@mattermost/types/teams';

import {patchTeam} from 'mattermost-redux/actions/teams';

import {renderWithContext, screen, userEvent, waitFor, within} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import ChannelsSettings from './channels_settings';

jest.mock('mattermost-redux/actions/channels', () => ({
    ...jest.requireActual('mattermost-redux/actions/channels'),
    getChannels: jest.fn(() => ({type: 'MOCK_GET_CHANNELS'})),
}));
jest.mock('mattermost-redux/actions/teams', () => ({
    ...jest.requireActual('mattermost-redux/actions/teams'),
    patchTeam: jest.fn(() => ({type: 'MOCK_PATCH_TEAM'})),
}));
const mockToast = jest.fn();
jest.mock('fusion/shell/toast_context', () => ({useToast: () => mockToast}));

const ch = (id: string, name: string, display: string, extra: Partial<Channel> = {}) => TestHelper.getChannelMock({id, name, display_name: display, team_id: 't1', type: 'O', delete_at: 0, ...extra});
const channels = [
    ch('town', 'general', 'General'),
    ch('dev', 'dev', 'Dev'),
    ch('ann', 'announcements', 'Announcements'),
    ch('secret', 'secret', 'Secret', {type: 'P'}),
    ch('old', 'old', 'Old', {delete_at: 4}),
];

function state({allChannels = false, defaultChannelId = '', list = channels, kind = 'guild' as TeamKind, welcome = '', joinLeaveOff = false} = {}) {
    return {
        entities: {
            teams: {currentTeamId: 't1', teams: {t1: TestHelper.getTeamMock({id: 't1', all_channels_membership: allChannels, default_channel_id: defaultChannelId, kind, welcome_channel_id: welcome, disable_join_leave_messages: joinLeaveOff})}},
            channels: {
                channels: Object.fromEntries(list.map((c) => [c.id, c])),
                channelsInTeam: {t1: new Set(list.map((c) => c.id))},
            },
        },
    };
}

const options = () => within(screen.getByRole('combobox', {name: 'Default channel'})).getAllByRole('option').map((o) => o.textContent);

describe('fusion/teams/ChannelsSettings', () => {
    test('makes the guild a workspace, and back', async () => {
        renderWithContext(<ChannelsSettings/>, state() as never);
        const kind = screen.getByRole('group', {name: 'Kind of community'});
        expect(within(kind).getByRole('button', {name: 'Guild'})).toHaveAttribute('aria-pressed', 'true');
        await userEvent.click(within(kind).getByRole('button', {name: 'Workspace'}));
        expect(patchTeam).toHaveBeenCalledWith({id: 't1', kind: 'workspace'});
        await waitFor(() => expect(mockToast).toHaveBeenCalledWith('It\'s a workspace: members join the channels they want and archive those they created'));
    });

    test('shows a workspace as such', async () => {
        renderWithContext(<ChannelsSettings/>, state({kind: 'workspace'}) as never);
        const kind = screen.getByRole('group', {name: 'Kind of community'});
        expect(within(kind).getByRole('button', {name: 'Workspace'})).toHaveAttribute('aria-pressed', 'true');
        await userEvent.click(within(kind).getByRole('button', {name: 'Guild'}));
        expect(patchTeam).toHaveBeenCalledWith({id: 't1', kind: 'guild'});
        await waitFor(() => expect(mockToast).toHaveBeenCalledWith('It\'s a guild: everyone is being added to the channels they may see'));
    });

    test('a community stored with a former kind name shows as its kind', () => {
        renderWithContext(<ChannelsSettings/>, state({kind: 'community' as TeamKind}) as never);
        expect(within(screen.getByRole('group', {name: 'Kind of community'})).getByRole('button', {name: 'Guild'})).toHaveAttribute('aria-pressed', 'true');
    });

    test('turns on "everyone in every channel"', async () => {
        renderWithContext(<ChannelsSettings/>, state() as never);
        const toggle = screen.getByRole('switch', {name: 'Members are in every channel'});
        expect(toggle).toHaveAttribute('aria-checked', 'false');
        await userEvent.click(toggle);
        expect(patchTeam).toHaveBeenCalledWith({id: 't1', all_channels_membership: true});
        await waitFor(() => expect(mockToast).toHaveBeenCalledWith('Everyone is being added to the channels they may see'));
    });

    test('turns it off', async () => {
        renderWithContext(<ChannelsSettings/>, state({allChannels: true}) as never);
        await userEvent.click(screen.getByRole('switch', {name: 'Members are in every channel'}));
        expect(patchTeam).toHaveBeenCalledWith({id: 't1', all_channels_membership: false});
        await waitFor(() => expect(mockToast).toHaveBeenCalledWith('Members can leave channels again'));
    });

    test('the default channel is General until another public channel is chosen', async () => {
        renderWithContext(<ChannelsSettings/>, state() as never);
        expect(screen.getByRole('combobox', {name: 'Default channel'})).toHaveValue('');
        expect(options()).toEqual(['#General (the first channel)', '#Announcements', '#Dev']);

        await userEvent.selectOptions(screen.getByRole('combobox', {name: 'Default channel'}), 'dev');
        expect(patchTeam).toHaveBeenCalledWith({id: 't1', default_channel_id: 'dev'});
        await waitFor(() => expect(mockToast).toHaveBeenCalledWith('Default channel saved'));
    });

    test('without General, newcomers land in their first channel', () => {
        renderWithContext(<ChannelsSettings/>, state({list: channels.slice(1)}) as never);
        expect(options()[0]).toBe('None: the first channel each person is in');
    });

    test('a chosen default channel that was archived is shown as such', () => {
        renderWithContext(<ChannelsSettings/>, state({defaultChannelId: 'old'}) as never);
        expect(screen.getByRole('combobox', {name: 'Default channel'})).toHaveValue('old');
        expect(options()).toContain('An archived channel');
    });

    test('tells why the setting wasn\'t saved', async () => {
        jest.mocked(patchTeam).mockReturnValueOnce((() => ({error: {message: 'Not allowed'}})) as never);
        renderWithContext(<ChannelsSettings/>, state() as never);
        await userEvent.click(screen.getByRole('switch', {name: 'Members are in every channel'}));
        await waitFor(() => expect(mockToast).toHaveBeenCalledWith('Not allowed'));
    });

    test('the join and leave messages go to the Welcome channel, another public channel, or nowhere', async () => {
        const list = [...channels, ch('wel', 'welcome', 'Welcome')];
        renderWithContext(<ChannelsSettings/>, state({list, welcome: 'wel'}) as never);
        const menu = screen.getByRole('combobox', {name: 'Join and leave messages'});
        expect(menu).toHaveValue('wel');
        expect(within(menu).getAllByRole('option').map((o) => o.textContent)).toEqual(['Off', 'In the default channel', '#Announcements', '#Dev', '#General', '#Welcome']);

        await userEvent.selectOptions(menu, 'dev');
        expect(patchTeam).toHaveBeenCalledWith({id: 't1', disable_join_leave_messages: false, welcome_channel_id: 'dev'});
        await waitFor(() => expect(mockToast).toHaveBeenCalledWith('Join and leave messages saved'));

        await userEvent.selectOptions(menu, 'off');
        expect(patchTeam).toHaveBeenCalledWith({id: 't1', disable_join_leave_messages: true});
        await waitFor(() => expect(mockToast).toHaveBeenCalledWith('Nobody is told who joins and leaves'));
    });

    test('a community made before Welcome tells them in its default channel', () => {
        renderWithContext(<ChannelsSettings/>, state() as never);
        expect(screen.getByRole('combobox', {name: 'Join and leave messages'})).toHaveValue('');
    });

    test('turned off, they show as off', () => {
        renderWithContext(<ChannelsSettings/>, state({welcome: 'dev', joinLeaveOff: true}) as never);
        expect(screen.getByRole('combobox', {name: 'Join and leave messages'})).toHaveValue('off');
    });
});
