// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {AIToolCallSummary} from '@mattermost/types/ai_tools';

import {Client4} from 'mattermost-redux/client';

import webSocketClient from 'client/web_websocket_client';
import {resetAgentActivities} from 'fusion/agents/activity';
import {fakeMatchMedia} from 'fusion/hooks/media_test_utils';
import Message from 'fusion/messages/message';
import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import AgentToolCalls, {prettyArguments, toolCallsOf} from './tool_calls';

jest.spyOn(webSocketClient, 'addMessageListener').mockImplementation(() => {});
jest.spyOn(webSocketClient, 'addReconnectListener').mockImplementation(() => {});

// An agent's reply shows the tool calls it waits on; only the person it answered can confirm or decline them.
describe('fusion/agents/tool_calls', () => {
    const maya = TestHelper.getUserMock({id: 'maya', username: 'maya'});
    const sam = TestHelper.getUserMock({id: 'sam', username: 'sam'});
    const coder = TestHelper.getUserMock({id: 'coder', username: 'coder', is_bot: true, is_agent: true});
    const call = (patch: Partial<AIToolCallSummary> = {}): AIToolCallSummary => ({
        id: 'call1',
        tool: 'antimatter.post_message',
        title: 'Post a message',
        kind: 'write',
        arguments: '{"channel_id":"town","message":"Shipped!"}',
        status: 'pending',
        user_id: 'maya',
        ...patch,
    });

    const render = (me: string, calls: AIToolCallSummary[], userId = 'coder') => {
        const post = TestHelper.getPostMock({id: 'reply', type: '', channel_id: 'channel', user_id: userId, message: '@maya, confirm or decline before I go on: Post a message.', props: {from_bot: 'true', agent_tool_calls: calls}});
        return renderWithContext(
            <Message postId={post.id}/>,
            {
                entities: {
                    general: {config: {EnablePostUsernameOverride: 'false', EnablePostIconOverride: 'false'}},
                    users: {currentUserId: me, profiles: {maya, sam, coder}},
                    posts: {posts: {[post.id]: post}},
                    channels: {channels: {channel: TestHelper.getChannelMock({id: 'channel', team_id: 'team'})}},
                    teams: {currentTeamId: 'team', teams: {team: TestHelper.getTeamMock({id: 'team', name: 'team'})}},
                },
            } as never,
        );
    };

    beforeEach(() => {
        resetAgentActivities();
        jest.spyOn(Client4, 'getAgentActivitiesByIds').mockResolvedValue([]);
        fakeMatchMedia(() => false);
    });
    afterEach(() => {
        jest.restoreAllMocks();
    });

    test('the person it waits for sees what will run, and answers', async () => {
        const answer = jest.spyOn(Client4, 'answerAIToolCall').mockResolvedValue(call({status: 'done'}));
        render('maya', [call()]);
        expect(screen.getByText('Post a message', {selector: 'b'})).toBeInTheDocument();
        expect(screen.getByText('antimatter.post_message')).toBeInTheDocument();
        expect(screen.getByText(/"message": "Shipped!"/)).toBeInTheDocument();
        await userEvent.click(screen.getByRole('button', {name: 'Confirm'}));
        expect(answer).toHaveBeenCalledWith('call1', true);
        expect(await screen.findByText('Done')).toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Confirm'})).not.toBeInTheDocument();
    });

    test('declining sends a decline', async () => {
        const answer = jest.spyOn(Client4, 'answerAIToolCall').mockResolvedValue(call({status: 'declined'}));
        render('maya', [call()]);
        await userEvent.click(screen.getByRole('button', {name: 'Decline'}));
        expect(answer).toHaveBeenCalledWith('call1', false);
        expect(await screen.findByText('Declined')).toBeInTheDocument();
    });

    test('someone else only sees whom it waits for', () => {
        render('sam', [call()]);
        expect(screen.queryByRole('button', {name: 'Confirm'})).not.toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Decline'})).not.toBeInTheDocument();
        expect(screen.getByText('Waiting for @maya')).toBeInTheDocument();
    });

    test('an answered call shows where it stands, without buttons', () => {
        render('maya', [call({status: 'failed'})]);
        expect(screen.getByText('Didn\'t work')).toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Confirm'})).not.toBeInTheDocument();
    });

    test('a person\'s own post with the same props shows no card', () => {
        render('maya', [call()], 'sam');
        expect(screen.queryByTestId('agent-tool-call')).not.toBeInTheDocument();
    });

    test('an answer that fails says so and keeps the buttons', async () => {
        jest.spyOn(Client4, 'answerAIToolCall').mockRejectedValue(new Error('nope'));
        render('maya', [call()]);
        await userEvent.click(screen.getByRole('button', {name: 'Confirm'}));
        expect(await screen.findByText('Your answer couldn\'t be sent. Try again.')).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Confirm'})).toBeEnabled();
    });

    test('malformed props show nothing', () => {
        const post = TestHelper.getPostMock({props: {agent_tool_calls: [{id: 1}, null, 'x', call({status: 'weird' as never})]}});
        expect(toolCallsOf(post)).toEqual([]);
        const {container} = renderWithContext(<AgentToolCalls post={post}/>);
        expect(container).toBeEmptyDOMElement();
    });

    test('arguments are shown indented, an empty object not at all', () => {
        expect(prettyArguments('{"a":1}')).toBe('{\n  "a": 1\n}');
        expect(prettyArguments('{}')).toBe('');
        expect(prettyArguments('not json')).toBe('not json');
    });
});
