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

import Message from './message';

jest.spyOn(webSocketClient, 'addMessageListener').mockImplementation(() => {});
jest.spyOn(webSocketClient, 'addReconnectListener').mockImplementation(() => {});

// Agents (Antimatter) are tagged AGENT where bots are BOT; while one works, its messages show only a dot by its name,
// never what it's doing.
describe('fusion/messages: agents', () => {
    const me = TestHelper.getUserMock({id: 'me', username: 'jason'});
    const coder = TestHelper.getUserMock({id: 'coder', username: 'coder', first_name: 'Coder', is_bot: true, is_agent: true});
    const releaseBot = TestHelper.getUserMock({id: 'release', username: 'release', is_bot: true});

    beforeEach(() => {
        resetAgentActivities();
        jest.spyOn(Client4, 'getAgentActivitiesByIds').mockResolvedValue([]);
        fakeMatchMedia(() => false);
    });

    const render = (userId: string, props: Record<string, unknown> = {}, message = 'Done: the export lives under the poll\'s menu.') => {
        const post = TestHelper.getPostMock({id: 'p1', type: '', channel_id: 'channel', user_id: userId, message, props});
        return renderWithContext(
            <Message postId={post.id}/>,
            {
                entities: {
                    general: {config: {EnablePostUsernameOverride: 'false', EnablePostIconOverride: 'false'}},
                    users: {currentUserId: me.id, profiles: {me, coder, release: releaseBot}},
                    posts: {posts: {[post.id]: post}},
                    channels: {channels: {channel: TestHelper.getChannelMock({id: 'channel', team_id: 'team'})}},
                    teams: {currentTeamId: 'team', teams: {team: TestHelper.getTeamMock({id: 'team', name: 'team'})}},
                },
            } as never,
        );
    };

    test('an agent\'s post is tagged AGENT, not BOT', () => {
        render('coder', {from_bot: 'true'});
        expect(screen.getByText('AGENT')).toBeInTheDocument();
        expect(screen.queryByText('BOT')).not.toBeInTheDocument();
    });

    test('a plain bot\'s post stays BOT', () => {
        render('release', {from_bot: 'true'});
        expect(screen.getByText('BOT')).toBeInTheDocument();
        expect(screen.queryByText('AGENT')).not.toBeInTheDocument();
    });

    test('a webhook posting through an agent\'s account is still BOT', () => {
        render('coder', {from_webhook: 'true'});
        expect(screen.getByText('BOT')).toBeInTheDocument();
        expect(screen.queryByText('AGENT')).not.toBeInTheDocument();
    });

    test('while the agent works, the header shows only the dot, never the activity text', () => {
        render('coder');
        act(() => receivedActivity(activity('coder', 'working', 'Running the poll export tests')));
        expect(screen.getByTestId('agent-dot')).toHaveClass('am-agent-busy');
        expect(screen.queryByText('Running the poll export tests')).not.toBeInTheDocument();
        expect(screen.queryByTestId('agent-activity')).not.toBeInTheDocument();
    });

    test('an idle agent has no dot', () => {
        render('coder');
        act(() => receivedActivity(activity('coder', 'idle')));
        expect(screen.queryByTestId('agent-dot')).not.toBeInTheDocument();
    });

    test('a reply being written is marked so', () => {
        const {container} = render('coder', {agent_streaming: true}, 'The export');
        expect(screen.getByText('Writing…')).toBeInTheDocument();
        expect(container.querySelector('.am-msg.am-agent-writing')).not.toBeNull();
    });
});
