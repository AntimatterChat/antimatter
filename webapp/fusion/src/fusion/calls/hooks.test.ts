// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import configureStore from 'store';

import {TestHelper} from 'utils/test_helper';

import type {GlobalState} from 'types/store';

import {canEndCall, canMakeHost} from './hooks';
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
