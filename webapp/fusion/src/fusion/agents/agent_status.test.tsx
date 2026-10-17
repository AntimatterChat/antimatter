// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {Client4} from 'mattermost-redux/client';

import webSocketClient from 'client/web_websocket_client';
import {fakeMatchMedia} from 'fusion/hooks/media_test_utils';
import {act, renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import {receivedActivity, resetAgentActivities} from './activity';
import {AgentActivityLine, AgentNameDot, IntegrationTag} from './agent_status';
import {activity} from './agent_test_utils';

jest.spyOn(webSocketClient, 'addMessageListener').mockImplementation(() => {});
jest.spyOn(webSocketClient, 'addReconnectListener').mockImplementation(() => {});

const agent = TestHelper.getUserMock({id: 'coder', username: 'coder', is_bot: true, is_agent: true});

describe('fusion/agents/agent_status', () => {
    beforeEach(() => {
        resetAgentActivities();
        jest.spyOn(Client4, 'getAgentActivitiesByIds').mockResolvedValue([]);
        fakeMatchMedia(() => false);
    });
    afterEach(() => {
        jest.spyOn(Math, 'random').mockRestore();
    });

    test('an agent is tagged AGENT, in purple; a bot BOT', () => {
        const {container, rerender} = renderWithContext(<IntegrationTag agent={true}/>);
        expect(screen.getByText('AGENT')).toHaveClass('am-bot-tag', 'am-agent-tag');
        rerender(<IntegrationTag agent={false}/>);
        expect(screen.getByText('BOT')).not.toHaveClass('am-agent-tag');
        expect(container.querySelectorAll('.am-agent-tag')).toHaveLength(0);
    });

    test('by the name, a working agent shows a pulsing yellow dot and never its text', () => {
        renderWithContext(<AgentNameDot user={agent}/>);
        act(() => receivedActivity(activity('coder', 'working', 'Running the poll tests')));
        const dot = screen.getByTestId('agent-dot');
        expect(dot).toHaveClass('am-agent-dot', 'am-agent-busy', 'am-agent-pulse');
        expect(screen.queryByText('Running the poll tests')).not.toBeInTheDocument();
    });

    test('with reduced motion, the dot doesn\'t pulse', () => {
        fakeMatchMedia((query) => query.includes('prefers-reduced-motion'));
        renderWithContext(<AgentNameDot user={agent}/>);
        act(() => receivedActivity(activity('coder', 'thinking')));
        expect(screen.getByTestId('agent-dot')).toHaveClass('am-agent-busy');
        expect(screen.getByTestId('agent-dot')).not.toHaveClass('am-agent-pulse');
    });

    test('a blocked or waiting agent has a steady dot of the other colour', () => {
        renderWithContext(<AgentNameDot user={agent}/>);
        act(() => receivedActivity(activity('coder', 'blocked', 'Needs a key')));
        const dot = screen.getByTestId('agent-dot');
        expect(dot).toHaveClass('am-agent-waiting');
        expect(dot).not.toHaveClass('am-agent-busy');
        expect(dot).not.toHaveClass('am-agent-pulse');
    });

    test('an idle agent shows no dot and no activity: its plain status shows', () => {
        renderWithContext(
            <>
                <AgentNameDot user={agent}/>
                <AgentActivityLine user={agent}/>
            </>,
        );
        act(() => receivedActivity(activity('coder', 'idle')));
        expect(screen.queryByTestId('agent-dot')).not.toBeInTheDocument();
        expect(screen.queryByTestId('agent-activity')).not.toBeInTheDocument();
    });

    test('the status line shows the dot, then what the agent is doing', () => {
        renderWithContext(<AgentActivityLine user={agent}/>);
        act(() => receivedActivity(activity('coder', 'working', 'Running the poll tests')));
        const line = screen.getByTestId('agent-activity');
        expect(line).toHaveTextContent('Running the poll tests');
        expect(line.querySelector('.am-agent-dot.am-agent-busy')).not.toBeNull();
    });

    test('without text, the status line picks a playful word', () => {
        jest.spyOn(Math, 'random').mockReturnValue(0);
        renderWithContext(<AgentActivityLine user={agent}/>);
        act(() => receivedActivity(activity('coder', 'working')));
        expect(screen.getByTestId('agent-activity')).toHaveTextContent('Pondering…');
    });

    test('another random draw gives another word', () => {
        jest.spyOn(Math, 'random').mockReturnValue(0.99);
        renderWithContext(<AgentActivityLine user={agent}/>);
        act(() => receivedActivity(activity('coder', 'thinking')));
        expect(screen.getByTestId('agent-activity')).toHaveTextContent('Distilling…');
    });

    test('a waiting agent without text is "Waiting for you…", in the other colour', () => {
        renderWithContext(<AgentActivityLine user={agent}/>);
        act(() => receivedActivity(activity('coder', 'waiting')));
        const line = screen.getByTestId('agent-activity');
        expect(line).toHaveTextContent('Waiting for you…');
        expect(line).toHaveClass('am-agent-activity-waiting');
        expect(line.querySelector('.am-agent-waiting')).not.toBeNull();
    });

    test('someone who isn\'t an agent never shows an activity, and isn\'t asked for', () => {
        const fetch = jest.spyOn(Client4, 'getAgentActivitiesByIds');
        const person = TestHelper.getUserMock({id: 'maya', username: 'maya'});
        renderWithContext(<AgentActivityLine user={person}/>);
        act(() => receivedActivity(activity('maya', 'working', 'Spoofed')));
        expect(screen.queryByTestId('agent-activity')).not.toBeInTheDocument();
        expect(fetch).not.toHaveBeenCalled();
    });
});
