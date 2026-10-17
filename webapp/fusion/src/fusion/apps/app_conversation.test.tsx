// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {Client4} from 'mattermost-redux/client';

import webSocketClient from 'client/web_websocket_client';
import {resetAgentActivities} from 'fusion/agents/activity';
import {handleAgentPostStream} from 'fusion/agents/stream';
import {fakeMatchMedia} from 'fusion/hooks/media_test_utils';
import {act, renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import {AppComposer, AppConversation} from './app_conversation';

jest.spyOn(webSocketClient, 'addMessageListener').mockImplementation(() => {});
jest.spyOn(webSocketClient, 'addReconnectListener').mockImplementation(() => {});

jest.mock('mattermost-redux/actions/posts', () => ({
    ...jest.requireActual('mattermost-redux/actions/posts'),
    getPostThread: jest.fn(() => () => Promise.resolve({data: true})),
}));

// The composer, reduced to its text box and the props it got.
const mockComposerProps: Array<Record<string, unknown>> = [];
jest.mock('fusion/composer/composer', () => ({
    __esModule: true,
    default: (props: Record<string, unknown>) => {
        mockComposerProps.push(props);
        return (
            <textarea
                aria-label={props.placeholder as string}
                data-testid='composer'
            />
        );
    },
}));

// The conversation Fusion lends to apps' side panels (the AI chat): the thread's messages, an agent's answer as it
// streams, and exactly one composer, replying in the thread.
describe('fusion/apps/AppConversation', () => {
    const me = TestHelper.getUserMock({id: 'me', username: 'jason'});
    const coder = TestHelper.getUserMock({id: 'coder', username: 'coder', first_name: 'Coder', is_bot: true, is_agent: true});
    const channel = TestHelper.getChannelMock({id: 'dm', type: 'D', name: 'coder__me', team_id: '', delete_at: 0});
    const root = TestHelper.getPostMock({id: 'root', channel_id: 'dm', user_id: 'me', message: 'Summarise the launch thread', root_id: '', type: '', create_at: 1000});
    const reply = TestHelper.getPostMock({id: 'reply', channel_id: 'dm', user_id: 'coder', message: 'The launch', root_id: 'root', type: '', create_at: 2000, edit_at: 0, props: {from_bot: 'true', agent_streaming: true}});

    const state = {
        entities: {
            general: {config: {EnablePostUsernameOverride: 'false', EnablePostIconOverride: 'false'}},
            users: {currentUserId: me.id, profiles: {me, coder}},
            posts: {posts: {root, reply}, postsInThread: {root: ['reply']}},
            channels: {channels: {dm: channel}},
            teams: {currentTeamId: 'team', teams: {team: TestHelper.getTeamMock({id: 'team', name: 'team'})}},
            preferences: {myPreferences: {}},
        },
    };

    beforeEach(() => {
        mockComposerProps.length = 0;
        resetAgentActivities();
        jest.spyOn(Client4, 'getAgentActivitiesByIds').mockResolvedValue([]);
        fakeMatchMedia(() => false);
    });

    test('shows the thread with one composer, replying in it', () => {
        renderWithContext(
            <AppConversation
                rootId='root'
                placeholder='Reply to Coder'
            />,
            state as never,
        );

        expect(screen.getByText('Summarise the launch thread')).toBeInTheDocument();
        expect(screen.getByText('The launch')).toBeInTheDocument();
        expect(screen.getAllByRole('textbox')).toHaveLength(1);
        expect(screen.getByRole('textbox', {name: 'Reply to Coder'})).toBeInTheDocument();
        expect(mockComposerProps[mockComposerProps.length - 1]).toEqual(expect.objectContaining({channelId: 'dm', rootId: 'root'}));
    });

    test('an agent\'s answer shows as it streams, then as saved', () => {
        const {store} = renderWithContext(<AppConversation rootId='root'/>, state as never);
        expect(screen.getByText('Writing…')).toBeInTheDocument();

        act(() => handleAgentPostStream({post_id: 'reply', channel_id: 'dm', root_id: 'root', user_id: 'coder', message: 'The launch is on Friday', control: 'text'}, store.dispatch, store.getState));
        expect(screen.getByText('The launch is on Friday')).toBeInTheDocument();

        act(() => handleAgentPostStream({post_id: 'reply', channel_id: 'dm', root_id: 'root', user_id: 'coder', message: 'The launch is on Friday.', control: 'end'}, store.dispatch, store.getState));
        expect(screen.getByText('The launch is on Friday.')).toBeInTheDocument();
        expect(screen.queryByText('Writing…')).not.toBeInTheDocument();
    });

    test('no composer in an archived conversation', () => {
        renderWithContext(<AppConversation rootId='root'/>, {
            ...state,
            entities: {...state.entities, channels: {channels: {dm: {...channel, delete_at: 1}}}},
        } as never);
        expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
    });
});

describe('fusion/apps/AppComposer', () => {
    beforeEach(() => {
        mockComposerProps.length = 0;
    });

    test('hands back the message it posted, to open the conversation it starts', () => {
        const onPosted = jest.fn();
        renderWithContext(
            <AppComposer
                channelId='dm'
                placeholder='Ask Coder anything'
                onPosted={onPosted}
            />,
        );
        expect(screen.getAllByRole('textbox')).toHaveLength(1);
        const props = mockComposerProps[mockComposerProps.length - 1];
        expect(props).toEqual(expect.objectContaining({channelId: 'dm', rootId: ''}));

        const afterSubmit = props.afterSubmit as (response: unknown) => void;
        afterSubmit({created: {id: 'new-root', channel_id: 'dm'}});
        expect(onPosted).toHaveBeenCalledWith(expect.objectContaining({id: 'new-root'}));

        onPosted.mockClear();
        afterSubmit({created: true});
        expect(onPosted).not.toHaveBeenCalled();
    });
});
