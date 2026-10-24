// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import configureStore from 'store';

import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import type {GlobalState} from 'types/store';

import type {AntimatterCallsAPI, CallsLastingMute} from './plugin_api/calls_public_api';
import type {AntimatterVoiceChannelsAPI} from './plugin_api/voice_channels_public_api';
import VoiceModerationSection, {canListVoiceMutes} from './voice_mod_list';

const mockActions = {modMute: jest.fn(() => Promise.resolve()), modDeafen: jest.fn(() => Promise.resolve())};
jest.mock('./actions', () => ({useCallActions: () => mockActions}));
jest.mock('mattermost-redux/actions/users', () => ({getMissingProfilesByIds: () => ({type: 'MOCK_GET_PROFILES'})}));

const entry = (over: Partial<CallsLastingMute>): CallsLastingMute => ({
    userId: 'u1',
    mutedBy: '',
    mutedAt: 0,
    deafenedBy: '',
    deafenedAt: 0,
    canLiftMute: false,
    canLiftDeafen: false,
    ...over,
});

const state = {
    entities: {
        users: {
            profiles: {
                u1: {id: 'u1', username: 'ana'},
                u2: {id: 'u2', username: 'ben'},
                mod: {id: 'mod', username: 'maya'},
                admin: {id: 'admin', username: 'root'},
            },
        },
    },
} as never;

function mockCalls(listModMutes: jest.Mock) {
    window.antimatterCalls = {
        version: 1,
        selectors: {getModMutes: () => [], getModDeafens: () => []},
        moderation: {listModMutes},
    } as unknown as AntimatterCallsAPI;
}

describe('fusion/calls/VoiceModerationSection', () => {
    beforeEach(() => {
        jest.clearAllMocks();
    });
    afterEach(() => {
        delete window.antimatterCalls;
        delete window.antimatterVoiceChannels;
    });

    test('lists who is muted or deafened, by whom, and lifts what the user may', async () => {
        const list = jest.fn(() => Promise.resolve([
            entry({userId: 'u1', mutedBy: 'mod', mutedAt: 1000, canLiftMute: true}),
            entry({userId: 'u2', deafenedBy: 'admin', deafenedAt: 2000, canLiftDeafen: false}),
        ]));
        mockCalls(list);
        renderWithContext(<VoiceModerationSection channelId='voice'/>, state);

        expect(await screen.findByText(/^Muted by maya on/)).toBeInTheDocument();
        expect(screen.getByText(/^Deafened by root on/)).toBeInTheDocument();
        expect(list).toHaveBeenCalledWith('voice');

        // Given by someone above the user: disabled, with why.
        const undeafen = screen.getByRole('button', {name: 'Undeafen'});
        expect(undeafen).toBeDisabled();
        expect(undeafen).toHaveAttribute('title', expect.stringMatching(/above you/));

        await userEvent.click(screen.getByRole('button', {name: 'Allow to unmute'}));
        expect(mockActions.modMute).toHaveBeenCalledWith(expect.objectContaining({userId: 'u1'}), 'voice', false);

        // The list is loaded again once lifted.
        await waitFor(() => expect(list).toHaveBeenCalledTimes(2));
    });

    test('says when nobody is, and when the list can\'t be loaded', async () => {
        mockCalls(jest.fn(() => Promise.resolve([])));
        const {unmount} = renderWithContext(<VoiceModerationSection channelId='voice'/>, state);
        expect(await screen.findByText('Nobody is muted or deafened here.')).toBeInTheDocument();
        unmount();

        mockCalls(jest.fn(() => Promise.reject(Object.assign(new Error('no'), {status: 403}))));
        renderWithContext(<VoiceModerationSection channelId='voice'/>, state);
        expect(await screen.findByText('The list couldn\'t be loaded.')).toBeInTheDocument();
    });
});

// Who sees the list: those who may moderate the voice channel's calls, as Calls checks it.
describe('fusion/calls/voice_mod_list: canListVoiceMutes', () => {
    const stateFor = (roles: string): GlobalState => configureStore({
        entities: {
            users: {currentUserId: 'me', profiles: {me: TestHelper.getUserMock({id: 'me', roles})}},
            channels: {channels: {voice: TestHelper.getChannelMock({id: 'voice', team_id: 'team', type: 'O'}), text: TestHelper.getChannelMock({id: 'text', team_id: 'team', type: 'O'})}},
            teams: {currentTeamId: 'team', teams: {team: TestHelper.getTeamMock({id: 'team'})}},
        },
    } as never).getState();

    beforeEach(() => {
        mockCalls(jest.fn());
        window.antimatterVoiceChannels = {version: 1, selectors: {isVoiceChannel: (_: unknown, id: string) => id === 'voice'}} as unknown as AntimatterVoiceChannelsAPI;
    });
    afterEach(() => {
        delete window.antimatterCalls;
        delete window.antimatterVoiceChannels;
    });

    test('moderators of a voice channel', () => {
        expect(canListVoiceMutes(stateFor('system_admin system_user'), 'voice')).toBe(true);
    });

    test('not the others, not other channels, not older Calls versions', () => {
        expect(canListVoiceMutes(stateFor('system_user'), 'voice')).toBe(false);
        expect(canListVoiceMutes(stateFor('system_admin system_user'), 'text')).toBe(false);
        window.antimatterCalls = {version: 1, moderation: {}} as unknown as AntimatterCallsAPI;
        expect(canListVoiceMutes(stateFor('system_admin system_user'), 'voice')).toBe(false);
    });
});
