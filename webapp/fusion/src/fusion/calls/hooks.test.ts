// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import configureStore from 'store';

import {TestHelper} from 'utils/test_helper';

import type {GlobalState} from 'types/store';

import {canEndCall, canMakeHost, moveTargets} from './hooks';
import type {AntimatterCallsAPI, CallsCall} from './plugin_api/calls_public_api';
import type {AntimatterVoiceChannelsAPI} from './plugin_api/voice_channels_public_api';

// Who the call UI offers to end a call for everyone and to make someone host: the same people the Calls server lets.
describe('fusion/calls/hooks: ending a call and making someone host', () => {
    const call: CallsCall = {channelId: 'ch', id: 'call', startAt: 1, ownerId: 'starter', hostId: 'host', threadId: 'post', recording: false};

    const stateFor = (userId: string, roles = 'system_user'): GlobalState => configureStore({
        entities: {
            users: {currentUserId: userId, profiles: {[userId]: TestHelper.getUserMock({id: userId, roles})}},
            channels: {channels: {ch: TestHelper.getChannelMock({id: 'ch', team_id: 'team', type: 'O'})}},
            teams: {currentTeamId: 'team', teams: {team: TestHelper.getTeamMock({id: 'team'})}},
        },
    } as never).getState();

    beforeEach(() => {
        window.antimatterCalls = {version: 1, selectors: {getCall: (_: unknown, channelId: string) => (channelId === 'ch' ? call : null)}} as unknown as AntimatterCallsAPI;
    });
    afterEach(() => {
        delete window.antimatterCalls;
        delete window.antimatterVoiceChannels;
    });

    test('whoever started the call ends it, also once the host passed on', () => {
        expect(canEndCall(stateFor('starter'), 'ch')).toBe(true);
    });

    test('the host and system admins end it, and make someone else host', () => {
        expect(canEndCall(stateFor('host'), 'ch')).toBe(true);
        expect(canMakeHost(stateFor('host'), 'ch')).toBe(true);
        expect(canEndCall(stateFor('admin', 'system_admin system_user'), 'ch')).toBe(true);
        expect(canMakeHost(stateFor('admin', 'system_admin system_user'), 'ch')).toBe(true);
    });

    test('anyone else in the channel does neither', () => {
        expect(canEndCall(stateFor('someone'), 'ch')).toBe(false);
        expect(canMakeHost(stateFor('someone'), 'ch')).toBe(false);
        expect(canMakeHost(stateFor('starter'), 'ch')).toBe(false);
    });

    test('in a voice channel, starting its call gives no right to end it', () => {
        window.antimatterVoiceChannels = {version: 1, selectors: {isVoiceChannel: () => true}} as unknown as AntimatterVoiceChannelsAPI;
        expect(canEndCall(stateFor('starter'), 'ch')).toBe(false);
        expect(canEndCall(stateFor('host'), 'ch')).toBe(true);
    });

    test('there is nothing to end without a call', () => {
        expect(canEndCall(stateFor('admin', 'system_admin system_user'), 'other')).toBe(false);
        expect(canMakeHost(stateFor('admin', 'system_admin system_user'), 'other')).toBe(false);
    });
});

// Where the call UI offers to move someone in a voice channel's call to: the other voice channels of the community whose
// calls the user moderates (Calls checks the same).
describe('fusion/calls/hooks: moving someone to another voice channel', () => {
    const voice = new Set(['room', 'gaming', 'music', 'elsewhere', 'old']);
    const stateFor = (userId: string, roles = 'system_user'): GlobalState => configureStore({
        entities: {
            users: {currentUserId: userId, profiles: {[userId]: TestHelper.getUserMock({id: userId, roles})}},
            channels: {
                channels: {
                    room: TestHelper.getChannelMock({id: 'room', team_id: 'team', type: 'O', display_name: 'Lobby'}),
                    music: TestHelper.getChannelMock({id: 'music', team_id: 'team', type: 'O', display_name: 'Music'}),
                    gaming: TestHelper.getChannelMock({id: 'gaming', team_id: 'team', type: 'P', display_name: 'Gaming'}),
                    text: TestHelper.getChannelMock({id: 'text', team_id: 'team', type: 'O', display_name: 'Text'}),
                    old: TestHelper.getChannelMock({id: 'old', team_id: 'team', type: 'O', display_name: 'Old', delete_at: 1}),
                    elsewhere: TestHelper.getChannelMock({id: 'elsewhere', team_id: 'other', type: 'O', display_name: 'Elsewhere'}),
                },
            },
            teams: {currentTeamId: 'team', teams: {team: TestHelper.getTeamMock({id: 'team'})}},
        },
    } as never).getState();

    beforeEach(() => {
        window.antimatterCalls = {version: 1, selectors: {getCall: () => null}} as unknown as AntimatterCallsAPI;
        window.antimatterVoiceChannels = {version: 1, selectors: {isVoiceChannel: (_: unknown, id: string) => voice.has(id)}} as unknown as AntimatterVoiceChannelsAPI;
    });
    afterEach(() => {
        delete window.antimatterCalls;
        delete window.antimatterVoiceChannels;
    });

    test('a system admin moves people to the other open voice channels of the community, by name', () => {
        expect(moveTargets(stateFor('admin', 'system_admin system_user'), 'room').map((c) => c.id)).toEqual(['gaming', 'music']);
    });

    test('nobody else, without the rights in those channels', () => {
        expect(moveTargets(stateFor('someone'), 'room')).toEqual([]);
    });

    test('only from a voice channel', () => {
        expect(moveTargets(stateFor('admin', 'system_admin system_user'), 'text')).toEqual([]);
        expect(moveTargets(stateFor('admin', 'system_admin system_user'), 'unknown')).toEqual([]);
    });
});
