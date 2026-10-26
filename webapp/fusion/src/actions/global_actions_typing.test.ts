// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {emitLocalUserTypingEvent} from 'actions/global_actions';
import reduxStore from 'stores/redux_store';

import WebSocketClient from 'client/web_websocket_client';

import type {GlobalState} from 'types/store';

jest.mock('stores/redux_store', () => ({
    dispatch: jest.fn(),
    getState: jest.fn(),
}));

jest.mock('client/web_websocket_client', () => ({
    __esModule: true,
    default: {userTyping: jest.fn()},
}));

// sec-fusion #7: typing events went out whatever your status, so someone Invisible ("you appear offline") showed
// they were online to everyone in the conversation as soon as they typed.
describe('actions/global_actions: emitLocalUserTypingEvent', () => {
    const stateWith = (status: string) => ({
        entities: {
            general: {config: {EnableUserTypingMessages: 'true', TimeBetweenUserTypingUpdatesMilliseconds: '5000', MaxNotificationsPerChannel: '1000'}},
            users: {currentUserId: 'me', statuses: {me: status}, isManualStatus: {me: status === 'offline'}},
            channels: {currentChannelId: 'c1', stats: {c1: {channel_id: 'c1', member_count: 3}}},
            preferences: {myPreferences: {}},
        },
    } as unknown as GlobalState);

    // The action is a thunk dispatched to the app's store: run it as the store would.
    const run = (state: GlobalState) => {
        jest.mocked(reduxStore.dispatch).mockClear();
        emitLocalUserTypingEvent('c1', '');
        const thunk = jest.mocked(reduxStore.dispatch).mock.calls[0][0] as unknown as (d: unknown, g: () => GlobalState) => Promise<unknown>;
        return thunk(jest.fn(), () => state);
    };

    beforeEach(() => {
        jest.mocked(WebSocketClient.userTyping).mockClear();
    });

    // Invisible first: the throttle (one event per TimeBetweenUserTypingUpdatesMilliseconds) starts at the first one sent.
    test('isn\'t sent while you\'re Invisible', async () => {
        await run(stateWith('offline'));
        expect(WebSocketClient.userTyping).not.toHaveBeenCalled();
    });

    test('is sent while you\'re online', async () => {
        await run(stateWith('online'));
        expect(WebSocketClient.userTyping).toHaveBeenCalledWith('c1', '');
    });
});
