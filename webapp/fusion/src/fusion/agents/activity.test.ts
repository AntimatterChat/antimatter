// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {act, renderHook} from '@testing-library/react';

import {Client4} from 'mattermost-redux/client';

import webSocketClient from 'client/web_websocket_client';
import {TestHelper} from 'utils/test_helper';

import {effectiveActivity, receivedActivity, requestAgentActivity, resetAgentActivities, useAgentActivity} from './activity';
import {activity} from './agent_test_utils';

let mockListener: ((msg: unknown) => void) | undefined;
jest.spyOn(webSocketClient, 'addMessageListener').mockImplementation((listener) => {
    mockListener = listener as never;
});
jest.spyOn(webSocketClient, 'addReconnectListener').mockImplementation(() => {});

const agent = TestHelper.getUserMock({id: 'coder', username: 'coder', is_bot: true, is_agent: true});
const person = TestHelper.getUserMock({id: 'maya', username: 'maya'});

describe('fusion/agents/activity', () => {
    beforeEach(() => {
        resetAgentActivities();
        jest.useRealTimers();
    });

    test('an expired busy state is idle; idle and offline never expire', () => {
        const now = Date.now();
        const working = activity('coder', 'working', 'Running the tests', {expires_at: now + 1000});
        expect(effectiveActivity(working, now)?.state).toBe('working');
        expect(effectiveActivity(working, now + 1000)).toEqual(expect.objectContaining({state: 'idle', text: ''}));
        expect(effectiveActivity(activity('coder', 'offline'), now + 1e9)?.state).toBe('offline');
        expect(effectiveActivity(undefined)).toBeUndefined();
    });

    test('an older activity never replaces a newer one', () => {
        const {result} = renderHook(() => useAgentActivity(agent));
        act(() => {
            receivedActivity(activity('coder', 'working', 'new', {update_at: 200}));
            receivedActivity(activity('coder', 'idle', '', {update_at: 100}));
        });
        expect(result.current?.text).toBe('new');
    });

    test('the agent_activity_changed event updates what shows, and keeps the thread the server gave', () => {
        jest.spyOn(Client4, 'getAgentActivitiesByIds').mockResolvedValue([]);
        const {result} = renderHook(() => useAgentActivity(agent));
        act(() => receivedActivity(activity('coder', 'working', 'Tests', {update_at: 100, thread_post_id: 'thread1'})));
        act(() => mockListener?.({event: 'agent_activity_changed', data: {user_id: 'coder', activity: activity('coder', 'working', 'Tests', {update_at: 100})}}));
        expect(result.current?.thread_post_id).toBe('thread1');

        act(() => mockListener?.({event: 'agent_activity_changed', data: {user_id: 'coder', activity: activity('coder', 'blocked', 'Needs a key', {update_at: 300})}}));
        expect(result.current).toEqual(expect.objectContaining({state: 'blocked', text: 'Needs a key', thread_post_id: ''}));
    });

    test('a busy state shows as idle once it expires, without the server saying so', () => {
        jest.useFakeTimers();
        const {result} = renderHook(() => useAgentActivity(agent));
        act(() => receivedActivity(activity('coder', 'working', 'Tests', {expires_at: Date.now() + 5000})));
        expect(result.current?.state).toBe('working');
        act(() => {
            jest.advanceTimersByTime(5100);
        });
        expect(result.current?.state).toBe('idle');
    });

    test('the agents shown are asked for together, once; people who aren\'t agents never', async () => {
        jest.useFakeTimers();
        const fetch = jest.spyOn(Client4, 'getAgentActivitiesByIds').mockResolvedValue([activity('coder', 'thinking')]);
        const other = TestHelper.getUserMock({id: 'researcher', username: 'researcher', is_bot: true, is_agent: true});
        renderHook(() => [useAgentActivity(agent), useAgentActivity(other), useAgentActivity(person)]);
        requestAgentActivity('coder');
        await act(async () => {
            jest.advanceTimersByTime(60);
        });
        expect(fetch).toHaveBeenCalledTimes(1);
        expect(fetch.mock.calls[0][0].sort()).toEqual(['coder', 'researcher']);
    });
});
