// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {Client4} from 'mattermost-redux/client';

import webSocketClient from 'client/web_websocket_client';
import {receivedActivity, resetAgentActivities} from 'fusion/agents/activity';
import {activity, ensureLayer} from 'fusion/agents/agent_test_utils';
import {fakeMatchMedia} from 'fusion/hooks/media_test_utils';
import {act, renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import UserPopover from './user_popover';

jest.mock('fusion/calls/call_user_button', () => ({__esModule: true, default: () => null, useCanCall: () => false}));
jest.mock('fusion/teams/team_card', () => ({TeamTag: () => null}));
jest.mock('plugins/pluggable', () => () => null);
jest.mock('./profile_facts', () => () => null);
jest.mock('./role_chips', () => () => null);
jest.mock('./status_popover', () => () => null);
jest.mock('./user_menu', () => ({UserMenu: () => null}));
jest.mock('./user_note', () => () => null);

jest.spyOn(webSocketClient, 'addMessageListener').mockImplementation(() => {});
jest.spyOn(webSocketClient, 'addReconnectListener').mockImplementation(() => {});

describe('fusion/popovers/UserPopover: agents', () => {
    const coder = TestHelper.getUserMock({id: 'coder', username: 'coder', first_name: 'Coder', is_bot: true, is_agent: true, bot_description: ''});

    beforeEach(() => {
        ensureLayer();
        resetAgentActivities();
        jest.spyOn(Client4, 'getAgentActivitiesByIds').mockResolvedValue([]);
        fakeMatchMedia(() => false);
    });

    const renderCard = () => renderWithContext(
        <UserPopover
            userId='coder'
            anchor={document.body}
            onClose={jest.fn()}
        />,
        {
            entities: {
                general: {config: {}},
                users: {currentUserId: 'me', profiles: {me: TestHelper.getUserMock({id: 'me'}), coder}, statuses: {coder: 'online'}},
                teams: {currentTeamId: 't1', teams: {t1: TestHelper.getTeamMock({id: 't1', name: 'lumen'})}},
                roles: {roles: {system_user: {permissions: []}}},
            },
        } as never,
    );

    test('an agent\'s card is tagged AGENT and says what an agent is', () => {
        renderCard();
        expect(screen.getByText('AGENT')).toBeInTheDocument();
        expect(screen.getByText('Agent · mention it or message it, like a coworker')).toBeInTheDocument();
    });

    test('a working agent\'s status starts with the dot and its activity', () => {
        renderCard();
        act(() => receivedActivity(activity('coder', 'working', 'Running the poll tests')));
        const line = screen.getByTestId('agent-activity');
        expect(line).toHaveTextContent('Running the poll tests');
        expect(line.firstElementChild).toHaveClass('am-agent-dot', 'am-agent-busy');
        expect(screen.queryByText(/· Online/)).not.toBeInTheDocument();
    });

    test('an idle agent shows its plain status', () => {
        renderCard();
        act(() => receivedActivity(activity('coder', 'idle')));
        expect(screen.queryByTestId('agent-activity')).not.toBeInTheDocument();
        expect(screen.getByText('@coder · Online')).toBeInTheDocument();
    });
});
