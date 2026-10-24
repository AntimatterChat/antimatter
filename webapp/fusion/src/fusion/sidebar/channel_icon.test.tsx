// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Channel, ChannelTypeEntry} from '@mattermost/types/channels';

import {renderWithContext} from 'tests/react_testing_utils';

import ChannelIcon from './channel_icon';

// The glyph a channel's Antimatter type and settings give it in the sidebar (design §4.3), when no app overrides it.
describe('ChannelIcon (Antimatter channel types, §4.3)', () => {
    const cases: Array<[string, ChannelTypeEntry, string]> = [
        ['text chat', {type: 'text', mode: 'chat', root_posts: 'members', preset: 'text_chat'}, 'hash'],
        ['text with root_posts=moderators', {type: 'text', mode: 'chat', root_posts: 'moderators', preset: 'text_announcement'}, 'megaphone'],
        ['voice talk', {type: 'voice', mode: 'talk', root_posts: 'members', preset: 'voice_talk'}, 'speaker'],
        ['voice stage', {type: 'voice', mode: 'stage', root_posts: 'members', preset: 'voice_stage'}, 'podium'],
        ['forum discussion', {type: 'forum', mode: 'discussion', root_posts: 'members', view: 'topics', preset: 'forum_agora'}, 'forum'],
        ['forum qa', {type: 'forum', mode: 'qa', root_posts: 'members', view: 'topics', preset: 'forum_qa'}, 'question'],
        ['forum ideas', {type: 'forum', mode: 'ideas', root_posts: 'members', view: 'topics', preset: 'forum_ideas'}, 'idea'],
        ['forum announcement (moderators open topics) still reads by its view, not root_posts', {type: 'forum', mode: 'discussion', root_posts: 'moderators', view: 'topics', preset: 'forum_announcement'}, 'forum'],
        ['forum whose default view is the image board', {type: 'forum', mode: 'discussion', root_posts: 'members', view: 'imageboard', preset: 'forum_imageboard'}, 'images'],
    ];

    test.each(cases)('%s -> %s', (_name, typeInfo, expected) => {
        const channel = {id: 'c1', type: 'O', team_id: 'team', display_name: 'C1'} as Channel;
        const {container} = renderWithContext(
            <ChannelIcon channel={channel}/>,
            {entities: {channels: {channelTypes: {c1: typeInfo}}}} as never,
        );

        expect(container.querySelector('use')).toHaveAttribute('href', `#am-i-${expected}`);
    });

    test('a channel with no stored type (not loaded, or made before channel types) shows #', () => {
        const channel = {id: 'c2', type: 'O', team_id: 'team', display_name: 'C2'} as Channel;
        const {container} = renderWithContext(<ChannelIcon channel={channel}/>);

        expect(container.querySelector('use')).toHaveAttribute('href', '#am-i-hash');
    });

    test('a private channel keeps its lock badge on top of the type glyph', () => {
        const channel = {id: 'c3', type: 'P', team_id: 'team', display_name: 'C3'} as Channel;
        const {container} = renderWithContext(
            <ChannelIcon channel={channel}/>,
            {entities: {channels: {channelTypes: {c3: {type: 'voice', mode: 'talk', root_posts: 'members', preset: 'voice_talk'}}}}} as never,
        );

        const uses = Array.from(container.querySelectorAll('use')).map((el) => el.getAttribute('href'));
        expect(uses).toEqual(['#am-i-speaker', '#am-i-lock']);
    });
});
