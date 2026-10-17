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
    ch('town', 'town-square', 'Town Square'),
    ch('dev', 'dev', 'Dev'),
    ch('ann', 'announcements', 'Announcements'),
    ch('secret', 'secret', 'Secret', {type: 'P'}),
    ch('old', 'old', 'Old', {delete_at: 4}),
];

function state({allChannels = false, defaultChannelId = '', list = channels, kind = 'community' as TeamKind} = {}) {
    return {
        entities: {
            teams: {currentTeamId: 't1', teams: {t1: TestHelper.getTeamMock({id: 't1', all_channels_membership: allChannels, default_channel_id: defaultChannelId, kind})}},
            channels: {
                channels: Object.fromEntries(list.map((c) => [c.id, c])),
                channelsInTeam: {t1: new Set(list.map((c) => c.id))},
            },
        },
    };
}

const options = () => within(screen.getByRole('combobox', {name: 'Default channel'})).getAllByRole('option').map((o) => o.textContent);

describe('fusion/teams/ChannelsSettings', () => {
    test('makes the community corporate, and back', async () => {
        renderWithContext(<ChannelsSettings/>, state() as never);
        const kind = screen.getByRole('group', {name: 'Kind of community'});
        expect(within(kind).getByRole('button', {name: 'Community'})).toHaveAttribute('aria-pressed', 'true');
        await userEvent.click(within(kind).getByRole('button', {name: 'Corporate'}));
        expect(patchTeam).toHaveBeenCalledWith({id: 't1', kind: 'corporate'});
        await waitFor(() => expect(mockToast).toHaveBeenCalledWith('Members now archive the channels they created'));
    });

    test('shows a corporate community as such', async () => {
        renderWithContext(<ChannelsSettings/>, state({kind: 'corporate'}) as never);
        const kind = screen.getByRole('group', {name: 'Kind of community'});
        expect(within(kind).getByRole('button', {name: 'Corporate'})).toHaveAttribute('aria-pressed', 'true');
        await userEvent.click(within(kind).getByRole('button', {name: 'Community'}));
        expect(patchTeam).toHaveBeenCalledWith({id: 't1', kind: 'community'});
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

    test('the default channel is Town Square until another public channel is chosen', async () => {
        renderWithContext(<ChannelsSettings/>, state() as never);
        expect(screen.getByRole('combobox', {name: 'Default channel'})).toHaveValue('');
        expect(options()).toEqual(['#Town Square (the first channel)', '#Announcements', '#Dev']);

        await userEvent.selectOptions(screen.getByRole('combobox', {name: 'Default channel'}), 'dev');
        expect(patchTeam).toHaveBeenCalledWith({id: 't1', default_channel_id: 'dev'});
        await waitFor(() => expect(mockToast).toHaveBeenCalledWith('Default channel saved'));
    });

    test('without Town Square, newcomers land in their first channel', () => {
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
});
