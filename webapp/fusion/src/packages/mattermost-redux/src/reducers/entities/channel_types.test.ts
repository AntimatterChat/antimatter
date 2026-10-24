// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {ServerChannel} from '@mattermost/types/channels';

import {ChannelTypes} from 'mattermost-redux/action_types';

import {channelTypes} from './channels';

// The Fusion channelTypes store (design §1.3's Fusion store): fed only from payloads that carry props.am_type, so a
// payload that wasn't filled in (a direct message, or one from a code path §1.3 doesn't name) never erases a known
// type.
describe('channelTypes reducer', () => {
    const typed = {id: 'c1', props: {am_type: 'forum', am_mode: 'qa', am_root_posts: 'members', am_forum_view: 'topics', am_preset: 'forum_qa'}} as ServerChannel;

    test('a filled-in channel is stored, normalised', () => {
        const state = channelTypes(undefined, {type: ChannelTypes.RECEIVED_CHANNEL, data: typed});
        expect(state.c1).toEqual({type: 'forum', mode: 'qa', root_posts: 'members', view: 'topics', preset: 'forum_qa'});
    });

    test('a payload without props.am_type (not filled in) never erases what\'s known', () => {
        const before = channelTypes(undefined, {type: ChannelTypes.RECEIVED_CHANNEL, data: typed});
        const notFilledIn = {id: 'c1', props: {}} as ServerChannel;
        const after = channelTypes(before, {type: ChannelTypes.RECEIVED_CHANNEL, data: notFilledIn});
        expect(after.c1).toEqual(before.c1);
    });

    test('a direct message (no am_type) is left out, not stored as text', () => {
        const dm = {id: 'dm1', type: 'D', props: {}} as ServerChannel;
        const state = channelTypes(undefined, {type: ChannelTypes.RECEIVED_CHANNEL, data: dm});
        expect(state.dm1).toBeUndefined();
    });

    test('RECEIVED_CHANNELS updates every typed channel in the list, leaving the rest', () => {
        const before = channelTypes(undefined, {type: ChannelTypes.RECEIVED_CHANNEL, data: typed});
        const list = [
            {id: 'c1', props: {am_type: 'forum', am_mode: 'ideas', am_root_posts: 'members', am_forum_view: 'topics', am_preset: 'forum_ideas'}},
            {id: 'c2', props: {am_type: 'text', am_mode: 'chat', am_root_posts: 'members', am_preset: 'text_chat'}},
            {id: 'c3', props: {}},
        ] as ServerChannel[];
        const after = channelTypes(before, {type: ChannelTypes.RECEIVED_CHANNELS, data: list});
        expect(after.c1).toEqual({type: 'forum', mode: 'ideas', root_posts: 'members', view: 'topics', preset: 'forum_ideas'});
        expect(after.c2).toEqual({type: 'text', mode: 'chat', root_posts: 'members', preset: 'text_chat'});
        expect(after.c3).toBeUndefined();
    });
});
