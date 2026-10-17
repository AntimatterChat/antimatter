// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {AgentPostStreamData} from '@mattermost/types/ai_agents';
import type {Post} from '@mattermost/types/posts';

import {receivedPost} from 'mattermost-redux/actions/posts';
import {getPost} from 'mattermost-redux/selectors/entities/posts';
import {isCollapsedThreadsEnabled} from 'mattermost-redux/selectors/entities/preferences';

import type {GlobalState} from 'types/store';

// An agent's answer streaming into its post (Antimatter): the server sends the whole text so far with each
// agent_post_stream event, then saves it with a normal post_edited (which replaces what's shown here). Meanwhile the
// post shows the text as it grows, marked as being written (props.agent_streaming), and never as edited.

export const AGENT_POST_STREAM_EVENT = 'agent_post_stream';

type Dispatch = (action: unknown) => unknown;

// streamedPost gives the post with the streamed text, or undefined when the event changes nothing: a post that isn't
// loaded (it arrives with its text when it is), or from another account than the agent's.
export function streamedPost(post: Post | undefined, data: AgentPostStreamData): Post | undefined {
    if (!post || (data.user_id && post.user_id !== data.user_id)) {
        return undefined;
    }
    const done = data.control === 'end' || data.control === 'error' || data.control === 'cancel';
    const props = {...(post.props || {})};
    if (done) {
        delete props.agent_streaming;
    } else {
        props.agent_streaming = true;
    }
    if (post.message === data.message && Boolean(post.props?.agent_streaming) === !done) {
        return undefined;
    }

    // The text arriving isn't an edit: edit_at stays as it was.
    return {...post, message: typeof data.message === 'string' ? data.message : post.message, props};
}

export function handleAgentPostStream(data: AgentPostStreamData, dispatch: Dispatch, getState: () => GlobalState) {
    if (!data?.post_id) {
        return;
    }
    const state = getState();
    const next = streamedPost(getPost(state, data.post_id), data);
    if (next) {
        dispatch(receivedPost(next, isCollapsedThreadsEnabled(state)));
    }
}
