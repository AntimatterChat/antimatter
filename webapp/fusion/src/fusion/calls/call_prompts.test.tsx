// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {LAYER_ID} from 'fusion/components/layer/layer';
import {act, renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import {callDuration} from './call_banner';
import CallPrompts, {INCOMING_CALL_TIMEOUT, pickIncomingCall} from './call_prompts';
import type {MyCallState} from './hooks';
import type {AntimatterCallsAPI, CallsIncomingCall, CallsUnmuteRequest, LocalCall} from './plugin_api/calls_public_api';

const mockActions = {joinAndShowCall: jest.fn(), toggleMute: jest.fn()};
let mockLocal: LocalCall | null = null;
let mockMy: MyCallState | null = null;
jest.mock('./actions', () => ({
    useCallActions: () => mockActions,
    firstName: () => 'Kenji',
    useChannelLabel: () => 'Design review',
}));
jest.mock('./hooks', () => ({
    useLocalCall: () => mockLocal,
    useMyCall: () => mockMy,
}));

const ring = (over: Partial<CallsIncomingCall> = {}): CallsIncomingCall => ({
    channelId: 'dm1',
    callId: 'call1',
    callerId: 'kenji',
    startAt: 1,
    type: 'dm',
    ringing: true,
    ...over,
});

const state = {
    entities: {
        users: {profiles: {kenji: {id: 'kenji', username: 'kenji', first_name: 'Kenji', last_name: 'Sato'}}},
        channels: {channels: {dm1: {id: 'dm1', type: 'D', display_name: 'kenji', name: 'me__kenji', delete_at: 0}}},
    },
} as never;

function mockCalls(incoming: CallsIncomingCall[]) {
    const listeners: Array<(r: CallsUnmuteRequest) => void> = [];
    const api = {
        version: 1,
        selectors: {getIncomingCalls: () => incoming},
        dismissIncomingCall: jest.fn(),
        stopRinging: jest.fn(),
        on: jest.fn((event: string, cb: (r: CallsUnmuteRequest) => void) => {
            if (event === 'unmute-request') {
                listeners.push(cb);
            }
            return () => undefined;
        }),
    };
    window.antimatterCalls = api as unknown as AntimatterCallsAPI;
    return {api, ask: (r: CallsUnmuteRequest) => act(() => listeners.forEach((l) => l(r)))};
}

describe('fusion/calls/call_prompts', () => {
    let layer: HTMLElement;
    beforeEach(() => {
        layer = document.createElement('div');
        layer.id = LAYER_ID;
        document.body.append(layer);
        mockLocal = null;
        mockMy = null;
        Object.values(mockActions).forEach((fn) => fn.mockClear());
    });
    afterEach(() => {
        layer.remove();
        delete window.antimatterCalls;
        jest.useRealTimers();
    });

    test('the card is for the newest call you aren\'t in, not yet answered', () => {
        const calls = [ring({callId: 'c3', channelId: 'x'}), ring({callId: 'c2', channelId: 'y'}), ring({callId: 'c1'})];
        expect(pickIncomingCall(calls, undefined, new Set())?.callId).toBe('c3');
        expect(pickIncomingCall(calls, 'x', new Set())?.callId).toBe('c2');
        expect(pickIncomingCall(calls, 'x', new Set(['c2']))?.callId).toBe('c1');
        expect(pickIncomingCall([], undefined, new Set())).toBeUndefined();
    });

    test('a ringing call can be joined, declined, or its ring muted', async () => {
        const {api} = mockCalls([ring()]);
        renderWithContext(<CallPrompts/>, state);

        expect(screen.getByRole('alertdialog', {name: /is calling you/})).toBeInTheDocument();
        expect(screen.getByText('is calling you')).toBeInTheDocument();

        await userEvent.click(screen.getByRole('button', {name: 'Mute the ring'}));
        expect(api.stopRinging).toHaveBeenCalledWith('call1');

        await userEvent.click(screen.getByRole('button', {name: 'Join'}));
        expect(mockActions.joinAndShowCall).toHaveBeenCalledWith('dm1');
        expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();
    });

    test('declining dismisses the call in Calls', async () => {
        const {api} = mockCalls([ring({ringing: false})]);
        renderWithContext(<CallPrompts/>, state);

        expect(screen.queryByRole('button', {name: 'Mute the ring'})).not.toBeInTheDocument();
        await userEvent.click(screen.getByRole('button', {name: 'Decline'}));
        expect(api.dismissIncomingCall).toHaveBeenCalledWith('dm1', 'call1');
        expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();
    });

    test('no card for the call you are in, and the card times out', () => {
        jest.useFakeTimers();
        const {api} = mockCalls([ring()]);
        mockLocal = {channelId: 'dm1', sessionId: 's', state: 'connected', mediaInThisWindow: true};
        const {rerender} = renderWithContext(<CallPrompts/>, state);
        expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();

        mockLocal = null;
        rerender(<CallPrompts/>);
        expect(screen.getByRole('alertdialog')).toBeInTheDocument();
        act(() => {
            jest.advanceTimersByTime(INCOMING_CALL_TIMEOUT);
        });
        expect(api.stopRinging).toHaveBeenCalledWith('call1');
        expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();
    });

    test('a moderator\'s request to unmute prompts the user, who can unmute or not', async () => {
        const {ask} = mockCalls([]);
        mockMy = {call: {channelId: 'ch', sessionId: 's1', state: 'connected', mediaInThisWindow: true}, muted: true, modMuted: false} as MyCallState;
        renderWithContext(<CallPrompts/>, state);
        expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();

        ask({channelId: 'ch', sessionId: 's1', byUserId: 'kenji'});
        expect(screen.getByRole('alertdialog', {name: 'Kenji asks you to unmute'})).toBeInTheDocument();
        expect(screen.getByText('You are muted in Design review')).toBeInTheDocument();
        await userEvent.click(screen.getByRole('button', {name: 'Unmute'}));
        expect(mockActions.toggleMute).toHaveBeenCalledWith(mockMy);
        expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();

        ask({channelId: 'ch', sessionId: 's1', byUserId: 'kenji'});
        await userEvent.click(screen.getByRole('button', {name: 'Not now'}));
        expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();
        expect(mockActions.toggleMute).toHaveBeenCalledTimes(1);
    });

    test('a request for another session, or once unmuted, shows nothing', () => {
        const {ask} = mockCalls([]);
        mockMy = {call: {channelId: 'ch', sessionId: 's1', state: 'connected', mediaInThisWindow: true}, muted: false, modMuted: false} as MyCallState;
        renderWithContext(<CallPrompts/>, state);

        ask({channelId: 'ch', sessionId: 's1', byUserId: 'kenji'});
        expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();
        ask({channelId: 'ch', sessionId: 'other', byUserId: 'kenji'});
        expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();
    });

    test('a call\'s duration reads as its timer', () => {
        expect(callDuration(0)).toBe('0:00');
        expect(callDuration(245000)).toBe('4:05');
        expect(callDuration(3723000)).toBe('1:02:03');
    });
});
