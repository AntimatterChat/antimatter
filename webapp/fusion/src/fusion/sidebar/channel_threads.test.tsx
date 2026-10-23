// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {CollapsedThreads} from '@mattermost/types/config';

import {THREAD_TITLE_PROP} from 'fusion/messages/thread_title';
import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import ChannelThreads from './channel_threads';

jest.mock('fusion/focus/focus', () => ({useHideUnreads: () => false}));
jest.mock('actions/views/rhs', () => ({selectPostById: jest.fn(() => ({type: 'SELECT'}))}));
jest.mock('mattermost-redux/actions/threads', () => ({getThreadsForCurrentTeam: jest.fn(() => ({type: 'GET'}))}));

const NOW = Date.UTC(2026, 9, 23);
const DAY = 864e5;

describe('fusion/sidebar/ChannelThreads', () => {
    beforeEach(() => {
        jest.useFakeTimers({advanceTimers: true}).setSystemTime(NOW);
    });
    afterEach(() => jest.useRealTimers());

    const topic = TestHelper.getPostMock({id: 'topic', channel_id: 'ch', create_at: NOW - DAY, props: {[THREAD_TITLE_PROP]: 'Release 2.1'}});
    const staleTopic = TestHelper.getPostMock({id: 'stale', channel_id: 'ch', create_at: NOW - (10 * DAY), props: {[THREAD_TITLE_PROP]: 'Old topic'}});
    const plainReply = TestHelper.getPostMock({id: 'reply-root', channel_id: 'ch', create_at: NOW - DAY, message: 'Does anyone know…'});

    const render = (posts: Array<ReturnType<typeof TestHelper.getPostMock>>, threads: Record<string, unknown> = {}, period?: string) => renderWithContext(
        <ChannelThreads channelId='ch'/>,
        {
            entities: {
                general: {config: {CollapsedThreads: CollapsedThreads.ALWAYS_ON}},
                posts: {posts: Object.fromEntries(posts.map((p) => [p.id, p])), postsInChannel: {ch: [{order: posts.map((p) => p.id), recent: true}]}},
                threads: {threads},
                preferences: {myPreferences: period ? {'fusion_sidebar--topic_period': {category: 'fusion_sidebar', name: 'topic_period', value: period}} : {}},
            },
            views: {rhs: {selectedPostId: ''}},
        } as never,
    );

    test('shows a topic under its channel even when nobody there follows it', () => {
        render([topic]);
        expect(screen.getByRole('button', {name: /^Topic: Release 2\.1/})).toBeInTheDocument();
    });

    test('drops a topic once nothing new was said in it past the period (a week by default)', () => {
        render([topic, staleTopic]);
        expect(screen.getByRole('button', {name: /^Topic: Release 2\.1/})).toBeInTheDocument();
        expect(screen.queryByRole('button', {name: /^Topic: Old topic/})).not.toBeInTheDocument();
    });

    test('a longer topic period (your preference) keeps it', () => {
        render([staleTopic], {}, '2w');
        expect(screen.getByRole('button', {name: /^Topic: Old topic/})).toBeInTheDocument();
    });

    test('an ordinary thread only shows while followed, with new replies, or open', () => {
        render([plainReply], {
            [plainReply.id]: {id: plainReply.id, post: {channel_id: 'ch'}, is_following: true, unread_replies: 2, unread_mentions: 1, last_reply_at: NOW - DAY},
        });
        const row = screen.getByRole('button', {name: /^Thread:/});
        expect(row).toHaveClass('am-unread');
        expect(row).toHaveTextContent('1');
    });

    test('an ordinary thread with no unread replies and not open doesn\'t show', () => {
        render([plainReply], {
            [plainReply.id]: {id: plainReply.id, post: {channel_id: 'ch'}, is_following: true, unread_replies: 0, unread_mentions: 0, last_reply_at: NOW - DAY},
        });
        expect(screen.queryByRole('button')).not.toBeInTheDocument();
    });

    test('a thread of another channel doesn\'t show here', () => {
        render([plainReply], {
            [plainReply.id]: {id: plainReply.id, post: {channel_id: 'other'}, is_following: true, unread_replies: 3, unread_mentions: 0, last_reply_at: NOW - DAY},
        });
        expect(screen.queryByRole('button')).not.toBeInTheDocument();
    });

    test('shows nothing (not even a container) with nothing to show', () => {
        const {container} = render([]);
        expect(container).toBeEmptyDOMElement();
    });

    test('a topic you also follow, with new replies, still shows its unread mark', () => {
        render([topic], {
            [topic.id]: {id: topic.id, post: {channel_id: 'ch'}, is_following: true, unread_replies: 4, unread_mentions: 2, last_reply_at: NOW - DAY},
        });
        const row = screen.getByRole('button', {name: /^Topic: Release 2\.1/});
        expect(row).toHaveClass('am-unread');
        expect(row).toHaveTextContent('2');
    });
});
