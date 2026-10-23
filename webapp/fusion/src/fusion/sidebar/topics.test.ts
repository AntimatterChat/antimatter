// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {Posts} from 'mattermost-redux/constants';

import {THREAD_TITLE_PROP} from 'fusion/messages/thread_title';

import type {GlobalState} from 'types/store';

import {activeTopicAt, DEFAULT_TOPIC_PERIOD, getTopicPeriodMs, topicIdsForChannel, topicPeriodMs, TOPIC_PERIODS} from './topics';

const DAY = 864e5;
const NOW = Date.UTC(2026, 9, 23);

describe('fusion/sidebar/topicPeriodMs', () => {
    test('knows every choice', () => {
        expect(topicPeriodMs('1d')).toBe(DAY);
        expect(topicPeriodMs('3d')).toBe(3 * DAY);
        expect(topicPeriodMs('1w')).toBe(7 * DAY);
        expect(topicPeriodMs('2w')).toBe(14 * DAY);
        expect(topicPeriodMs('1m')).toBe(30 * DAY);
    });

    test('falls back to the default for anything else', () => {
        expect(topicPeriodMs('nope')).toBe(topicPeriodMs(DEFAULT_TOPIC_PERIOD));
    });

    test('every listed choice round-trips', () => {
        for (const [key, ms] of TOPIC_PERIODS) {
            expect(topicPeriodMs(key)).toBe(ms);
        }
    });
});

describe('fusion/sidebar/getTopicPeriodMs', () => {
    test('defaults to a week', () => {
        const state = {entities: {preferences: {myPreferences: {}}}} as unknown as GlobalState;
        expect(getTopicPeriodMs(state)).toBe(7 * DAY);
    });

    test('reads the preference', () => {
        const state = {entities: {preferences: {myPreferences: {
            'fusion_sidebar--topic_period': {category: 'fusion_sidebar', name: 'topic_period', value: '1m'},
        }}}} as unknown as GlobalState;
        expect(getTopicPeriodMs(state)).toBe(30 * DAY);
    });
});

describe('fusion/sidebar/activeTopicAt', () => {
    const topic = {root_id: '', delete_at: 0, create_at: 100, props: {[THREAD_TITLE_PROP]: 'Release 2.1'}};

    test('a topic is active when it was last replied to, else when it started', () => {
        expect(activeTopicAt(topic)).toBe(100);
        expect(activeTopicAt({...topic, last_reply_at: 500})).toBe(500);
    });

    test('not a reply, not deleted, and named', () => {
        expect(activeTopicAt({...topic, root_id: 'root'})).toBeUndefined();
        expect(activeTopicAt({...topic, delete_at: 1})).toBeUndefined();
        expect(activeTopicAt({...topic, state: Posts.POST_DELETED})).toBeUndefined();
        expect(activeTopicAt({...topic, props: {}})).toBeUndefined();
        expect(activeTopicAt(undefined)).toBeUndefined();
    });
});

describe('fusion/sidebar/topicIdsForChannel', () => {
    const post = (id: string, overrides: Record<string, unknown> = {}) => ({
        id,
        channel_id: 'ch',
        root_id: '',
        delete_at: 0,
        create_at: NOW - DAY,
        props: {[THREAD_TITLE_PROP]: `Topic ${id}`},
        ...overrides,
    });
    const stateWith = (posts: Array<ReturnType<typeof post>>, order: string[]) => ({
        entities: {
            posts: {
                posts: Object.fromEntries(posts.map((p) => [p.id, p])),
                postsInChannel: {ch: [{order, recent: true}]},
            },
            preferences: {myPreferences: {}},
        },
    }) as unknown as GlobalState;

    test('every topic active within the default period (a week), in no particular order', () => {
        const posts = [
            post('fresh', {create_at: NOW - DAY}),
            post('stale', {create_at: NOW - (10 * DAY)}),
            post('unnamed', {props: {}}),
            post('reply', {root_id: 'fresh'}),
        ];
        expect(topicIdsForChannel(stateWith(posts, posts.map((p) => p.id)), 'ch', NOW)).toEqual(['fresh']);
    });

    test('a longer period keeps older topics too', () => {
        const posts = [post('fresh', {create_at: NOW - DAY}), post('stale', {create_at: NOW - (10 * DAY)})];
        const state = stateWith(posts, posts.map((p) => p.id));
        (state as never as {entities: {preferences: {myPreferences: Record<string, unknown>}}}).entities.preferences.myPreferences = {
            'fusion_sidebar--topic_period': {category: 'fusion_sidebar', name: 'topic_period', value: '2w'},
        };
        expect(topicIdsForChannel(state, 'ch', NOW).sort()).toEqual(['fresh', 'stale']);
    });

    test('a topic someone replied to recently stays active by that reply', () => {
        const posts = [post('old', {create_at: NOW - (20 * DAY), last_reply_at: NOW - DAY})];
        expect(topicIdsForChannel(stateWith(posts, posts.map((p) => p.id)), 'ch', NOW)).toEqual(['old']);
    });

    test('nothing when the channel has no loaded posts', () => {
        expect(topicIdsForChannel(stateWith([], []), 'other', NOW)).toEqual([]);
    });
});
