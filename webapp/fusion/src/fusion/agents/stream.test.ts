// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {AgentPostStreamData} from '@mattermost/types/ai_agents';

import {PostTypes} from 'mattermost-redux/action_types';

import {TestHelper} from 'utils/test_helper';

import {handleAgentPostStream, streamedPost} from './stream';

const data = (patch: Partial<AgentPostStreamData>): AgentPostStreamData => ({post_id: 'reply', channel_id: 'c', root_id: 'root', user_id: 'coder', message: '', control: 'text', ...patch});

describe('fusion/agents/stream', () => {
    const post = TestHelper.getPostMock({id: 'reply', user_id: 'coder', channel_id: 'c', root_id: 'root', message: '', edit_at: 0, props: {agent_streaming: true}});

    test('the text so far shows in the post, which stays unedited and marked as being written', () => {
        const next = streamedPost(post, data({message: 'The export lives'}));
        expect(next).toEqual(expect.objectContaining({message: 'The export lives', edit_at: 0}));
        expect(next?.props.agent_streaming).toBe(true);
    });

    test('the end removes the mark', () => {
        const next = streamedPost({...post, message: 'Done'}, data({message: 'Done.', control: 'end'}));
        expect(next?.message).toBe('Done.');
        expect(next?.props.agent_streaming).toBeUndefined();
    });

    test('an error or a cancel ends the stream too', () => {
        expect(streamedPost(post, data({message: 'Partial', control: 'error'}))?.props.agent_streaming).toBeUndefined();
        expect(streamedPost(post, data({message: 'Partial', control: 'cancel'}))?.props.agent_streaming).toBeUndefined();
    });

    test('nothing changes for a post that isn\'t loaded, or that another account wrote', () => {
        expect(streamedPost(undefined, data({message: 'x'}))).toBeUndefined();
        expect(streamedPost({...post, user_id: 'maya'}, data({message: 'Spoofed'}))).toBeUndefined();
    });

    test('nothing changes when the text and the state are the same', () => {
        expect(streamedPost({...post, message: 'Same'}, data({message: 'Same'}))).toBeUndefined();
    });

    test('the handler stores the streamed post', () => {
        const dispatch = jest.fn();
        const state = {
            entities: {
                posts: {posts: {reply: post}},
                preferences: {myPreferences: {}},
                general: {config: {}},
            },
        };
        handleAgentPostStream(data({message: 'Hello'}), dispatch, () => state as never);
        expect(dispatch).toHaveBeenCalledWith(expect.objectContaining({type: PostTypes.RECEIVED_POST, data: expect.objectContaining({id: 'reply', message: 'Hello'})}));

        dispatch.mockClear();
        handleAgentPostStream(data({post_id: 'unknown', message: 'Hello'}), dispatch, () => state as never);
        expect(dispatch).not.toHaveBeenCalled();
    });
});
