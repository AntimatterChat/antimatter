// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {Client4} from 'mattermost-redux/client';

import webSocketClient from 'client/web_websocket_client';
import {receivedActivity, resetAgentActivities} from 'fusion/agents/activity';
import {activity} from 'fusion/agents/agent_test_utils';
import {fakeMatchMedia} from 'fusion/hooks/media_test_utils';
import {act, renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import {Member} from './member_list';

jest.mock('fusion/components/avatar', () => () => null);
jest.mock('fusion/teams/team_card', () => ({TeamTag: () => null}));
jest.mock('fusion/calls/markers', () => ({TalkNote: ({children}: {children: React.ReactNode}) => children}));

jest.spyOn(webSocketClient, 'addMessageListener').mockImplementation(() => {});
jest.spyOn(webSocketClient, 'addReconnectListener').mockImplementation(() => {});

describe('fusion/members/Member: agents', () => {
    const coder = TestHelper.getUserMock({id: 'coder', username: 'coder', first_name: 'Coder', is_bot: true, is_agent: true, position: 'Writes code'});
    const bot = TestHelper.getUserMock({id: 'release', username: 'release', is_bot: true});

    beforeEach(() => {
        resetAgentActivities();
        jest.spyOn(Client4, 'getAgentActivitiesByIds').mockResolvedValue([]);
        fakeMatchMedia(() => false);
    });

    const render = (user: typeof coder) => renderWithContext(
        <Member
            user={user}
            off={false}
            role='bot'
            onOpen={jest.fn()}
            onMenu={jest.fn()}
        />,
        {entities: {users: {currentUserId: 'me', profiles: {coder, release: bot}}, teams: {currentTeamId: 't1'}}} as never,
    );

    test('an agent is tagged AGENT, a bot BOT', () => {
        const {unmount} = render(coder);
        expect(screen.getByText('AGENT')).toBeInTheDocument();
        unmount();
        render(bot);
        expect(screen.getByText('BOT')).toBeInTheDocument();
    });

    test('a working agent\'s status line starts with the dot, then what it does', () => {
        render(coder);
        act(() => receivedActivity(activity('coder', 'working', 'Running the poll tests')));
        const line = screen.getByTestId('agent-activity');
        expect(line).toHaveTextContent('Running the poll tests');
        expect(line.firstElementChild).toHaveClass('am-agent-dot', 'am-agent-pulse');
        expect(screen.queryByText('Writes code')).not.toBeInTheDocument();
    });

    test('an idle agent shows its plain line, as a person', () => {
        render(coder);
        act(() => receivedActivity(activity('coder', 'idle')));
        expect(screen.queryByTestId('agent-activity')).not.toBeInTheDocument();
        expect(screen.getByText('Writes code')).toBeInTheDocument();
    });

    test('a blocked agent uses the attention colour', () => {
        render(coder);
        act(() => receivedActivity(activity('coder', 'blocked', 'Needs the deploy key')));
        const line = screen.getByTestId('agent-activity');
        expect(line).toHaveClass('am-agent-activity-waiting');
        expect(line.firstElementChild).toHaveClass('am-agent-waiting');
        expect(line.firstElementChild).not.toHaveClass('am-agent-pulse');
    });
});
