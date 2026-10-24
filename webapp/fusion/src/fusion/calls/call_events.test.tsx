// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {showToast} from 'fusion/components/toast';
import {act, renderWithContext} from 'tests/react_testing_utils';

import {goToChannel} from './actions';
import CallEvents from './call_events';
import type {MyCallState} from './hooks';
import type {AntimatterCallsAPI, CallsErrorEvent, CallsMoveRequest} from './plugin_api/calls_public_api';

let mockMy: MyCallState | null = null;
jest.mock('./actions', () => ({
    useCallActions: () => ({}),
    channelLabel: (_state: unknown, channelId: string) => ({room: 'Lobby', v2: 'Gaming'}[channelId] || channelId),
    goToChannel: jest.fn(),
}));
jest.mock('./hooks', () => ({
    useMyCall: () => mockMy,
    useParticipants: () => [],
}));
jest.mock('./extras', () => ({useApplyVolumes: () => undefined}));
jest.mock('fusion/components/toast', () => ({showToast: jest.fn()}));

const stateIn = (currentChannelId: string) => ({entities: {channels: {currentChannelId, channels: {}}}}) as never;

function mockCalls() {
    const listeners: {[event: string]: Array<(arg: unknown) => void>} = {};
    window.antimatterCalls = {
        version: 1,
        on: jest.fn((event: string, cb: (arg: unknown) => void) => {
            (listeners[event] ||= []).push(cb);
            return () => undefined;
        }),
    } as unknown as AntimatterCallsAPI;
    return (event: string, arg: unknown) => act(() => (listeners[event] || []).forEach((l) => l(arg)));
}

const move: CallsMoveRequest = {channelId: 'room', sessionId: 's1', toChannelId: 'v2', byUserId: 'mod'};

describe('fusion/calls/CallEvents', () => {
    beforeEach(() => {
        jest.clearAllMocks();
        mockMy = null;
    });
    afterEach(() => {
        delete window.antimatterCalls;
    });

    test('moved by a moderator, the view of the voice channel follows them', () => {
        const fire = mockCalls();
        renderWithContext(<CallEvents/>, stateIn('room'));

        fire('move', move);
        expect(showToast).toHaveBeenCalledWith('A moderator moved you to Gaming');
        expect(goToChannel).toHaveBeenCalledWith(expect.anything(), 'v2');
    });

    test('moved while looking elsewhere, they stay where they are', () => {
        const fire = mockCalls();
        renderWithContext(<CallEvents/>, stateIn('elsewhere'));

        fire('move', move);
        expect(showToast).toHaveBeenCalledWith('A moderator moved you to Gaming');
        expect(goToChannel).not.toHaveBeenCalled();
    });

    test('unmuting while deafened by a moderator says why it can\'t', () => {
        const fire = mockCalls();
        renderWithContext(<CallEvents/>, stateIn('room'));

        fire('error', {kind: 'mod-deafened'} as CallsErrorEvent);
        expect(showToast).toHaveBeenCalledWith('A moderator deafened you: you can unmute once they undeafen you');
    });

    test('being deafened by a moderator, and undeafened, is told', () => {
        mockCalls();
        const my = (modDeafened: boolean) => ({call: {channelId: 'room', state: 'connected'}, modDeafened, modMuted: false} as unknown as MyCallState);
        mockMy = my(false);
        const {rerender} = renderWithContext(<CallEvents/>, stateIn('room'));
        expect(showToast).not.toHaveBeenCalled();

        mockMy = my(true);
        rerender(<CallEvents/>);
        expect(showToast).toHaveBeenLastCalledWith('A moderator deafened you in Lobby: you\'ll hear the others and can unmute once they undeafen you');

        mockMy = my(false);
        rerender(<CallEvents/>);
        expect(showToast).toHaveBeenLastCalledWith('A moderator undeafened you in Lobby');
    });

    test('in a stage\'s audience, what they can\'t do says why, and an invitation to speak is told', () => {
        const fire = mockCalls();
        renderWithContext(<CallEvents/>, stateIn('room'));

        fire('error', {kind: 'stage-audience'} as CallsErrorEvent);
        expect(showToast).toHaveBeenLastCalledWith('Only speakers can talk here: request to speak, and a moderator may invite you');

        fire('stage-invite', {channelId: 'room', byUserId: 'mod'});
        expect(showToast).toHaveBeenLastCalledWith('A moderator invited you to speak in Lobby: unmute when you\'re ready');
    });
});
