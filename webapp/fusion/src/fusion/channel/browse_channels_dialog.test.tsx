// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Channel} from '@mattermost/types/channels';

import {Client4} from 'mattermost-redux/client';

import {LAYER_ID} from 'fusion/components/layer';
import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {getHistory} from 'utils/browser_history';

import BrowseChannelsDialog, {browseList} from './browse_channels_dialog';

const channel = (id: string, fields: Partial<Channel> = {}) => ({id, name: id, display_name: id[0].toUpperCase() + id.slice(1), team_id: 'team', type: 'O', delete_at: 0, purpose: '', ...fields}) as Channel;

describe('browseList', () => {
    const general = channel('general');
    const dev = channel('dev');
    const lab = channel('lab', {type: 'P'});
    const secret = channel('secret', {type: 'P'});
    const asks = channel('asks', {type: 'P', discoverable: true});
    const old = channel('old', {delete_at: 1});
    const all = [general, dev, lab, secret, asks, old, dev];
    const member = (id: string) => ['general', 'lab'].includes(id);
    const ids = (list: Channel[]) => list.map((c) => c.id);

    test('all: public channels, private ones joined or open to requests, by name, once each', () => {
        expect(ids(browseList(all, 'all', member, false))).toEqual(['asks', 'dev', 'general', 'lab']);
    });

    test('each kind', () => {
        expect(ids(browseList(all, 'public', member, false))).toEqual(['dev', 'general']);
        expect(ids(browseList(all, 'private', member, false))).toEqual(['lab']);
        expect(ids(browseList(all, 'archived', member, false))).toEqual(['old']);
        expect(ids(browseList(all, 'requestable', member, false))).toEqual(['asks']);
    });

    test('joined channels can be hidden', () => {
        expect(ids(browseList(all, 'all', member, true))).toEqual(['asks', 'dev']);
    });
});

describe('BrowseChannelsDialog', () => {
    const general = channel('general');
    const dev = channel('dev', {purpose: 'Building things'});
    const render = (allChannels = false) => {
        const onClose = jest.fn();
        renderWithContext(
            <BrowseChannelsDialog onClose={onClose}/>,
            {
                entities: {
                    general: {config: {}},
                    users: {currentUserId: 'me'},
                    teams: {currentTeamId: 'team', teams: {team: {id: 'team', name: 'antimatter', all_channels_membership: allChannels}}},
                    channels: {
                        channels: {general, dev},
                        channelsInTeam: {team: new Set(['general', 'dev'])},
                        myMembers: {general: {channel_id: 'general', user_id: 'me'}},
                        channelsMemberCount: {general: 5, dev: 2},
                    },
                },
            } as never,
        );
        return onClose;
    };

    beforeEach(() => {
        const layer = document.createElement('div');
        layer.id = LAYER_ID;
        document.body.appendChild(layer);
        jest.spyOn(Client4, 'getChannels').mockResolvedValue([]);
        jest.spyOn(Client4, 'getArchivedChannels').mockResolvedValue([]);
    });

    afterEach(() => {
        document.getElementById(LAYER_ID)?.remove();
        jest.restoreAllMocks();
    });

    test('lists the team\'s channels, joined or to join', () => {
        render();
        const rows = screen.getAllByRole('listitem');
        expect(rows.map((r) => r.textContent)).toEqual([
            expect.stringContaining('Dev2 members · Building thingsJoin'),
            expect.stringContaining('General5 membersJoined'),
        ]);
    });

    test('joining a channel opens it', async () => {
        const join = jest.spyOn(Client4, 'addToChannel').mockResolvedValue({channel_id: 'dev', user_id: 'me', roles: 'channel_user'} as never);
        jest.spyOn(Client4, 'getChannel').mockResolvedValue(dev as never);
        jest.spyOn(Client4, 'getChannelMember').mockResolvedValue({channel_id: 'dev', user_id: 'me'} as never);
        const push = jest.spyOn(getHistory(), 'push').mockImplementation(() => undefined);
        const onClose = render();

        await userEvent.click(screen.getByRole('button', {name: 'Join'}));
        await waitFor(() => expect(onClose).toHaveBeenCalled());
        expect(join).toHaveBeenCalledWith('me', 'dev');
        expect(push).toHaveBeenCalledWith('/antimatter/channels/dev');
    });

    test('searching asks the server, archived channels included', async () => {
        const search = jest.spyOn(Client4, 'searchAllChannels').mockResolvedValue([channel('ops', {display_name: 'Ops'})] as never);
        render();
        await userEvent.type(screen.getByRole('textbox', {name: 'Search channels'}), 'op');
        expect(await screen.findByText('Ops')).toBeInTheDocument();
        expect(search).toHaveBeenCalledWith('op', expect.objectContaining({team_ids: ['team'], include_deleted: true}));
        expect(screen.queryByText('General')).not.toBeInTheDocument();
    });

    // Antimatter: where everyone is in every channel, nobody joins or leaves channels.
    test('where members are in every channel, no Join nor Joined', async () => {
        render(true);
        await userEvent.click(screen.getByRole('checkbox', {name: 'Hide joined'}));
        expect(screen.getByText('General')).toBeInTheDocument();
        expect(screen.getByText('Dev')).toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Join'})).not.toBeInTheDocument();
        expect(screen.queryByText('Joined')).not.toBeInTheDocument();
    });

    test('the joined ones can be hidden', async () => {
        render();
        await userEvent.click(screen.getByRole('checkbox', {name: 'Hide joined'}));
        expect(screen.queryByText('General')).not.toBeInTheDocument();
        expect(screen.getByText('Dev')).toBeInTheDocument();
    });

    test('asks the voice plugin about the channels not joined, for their icons', async () => {
        const load = jest.fn(() => Promise.resolve());
        window.antimatterVoiceChannels = {version: 1, selectors: {isVoiceChannel: () => false, getParticipants: () => [], isDeafened: () => false, getAutoJoin: () => false}, join: jest.fn(), autoJoin: jest.fn(), leave: jest.fn(), setDeafened: jest.fn(), load} as never;
        render();
        await waitFor(() => expect(load).toHaveBeenCalledWith(['dev']));
        delete window.antimatterVoiceChannels;
    });
});
