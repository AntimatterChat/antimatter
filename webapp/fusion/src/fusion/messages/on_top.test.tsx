// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {createIntl} from 'react-intl';

import type {OnTopPost} from '@mattermost/types/posts';

import {ChannelTypes, UserTypes} from 'mattermost-redux/action_types';
import {onTop as onTopReducer} from 'mattermost-redux/reducers/entities/channels';
import {getOnTopEntry} from 'mattermost-redux/selectors/entities/on_top_posts';

import {renderWithContext, screen} from 'tests/react_testing_utils';

import type {GlobalState} from 'types/store';

import {OnTopTag, activeOnTop, nextExpiry, timeLeft} from './on_top';

const HOUR = 3600000;
const entry = (postId: string, createAt: number, expiresAt = 0, channelId = 'ch'): OnTopPost => ({post_id: postId, channel_id: channelId, created_by: 'me', create_at: createAt, expires_at: expiresAt});

describe('fusion/messages/on_top', () => {
    const intl = createIntl({locale: 'en', defaultLocale: 'en'});
    const now = 1_000_000_000_000;

    test('tells the time left as the mockup does', () => {
        expect(timeLeft(intl, 0, now)).toBe('until removed');
        expect(timeLeft(intl, now + (2 * 24 * HOUR) + (3 * HOUR), now)).toBe('2d 3h left');
        expect(timeLeft(intl, now + (3 * HOUR) + (20 * 60000), now)).toBe('3h 20m left');
        expect(timeLeft(intl, now + (5 * 60000), now)).toBe('5m left');
        expect(timeLeft(intl, now + 1000, now)).toBe('1m left');
    });

    test('leaves out expired messages, and finds the next expiry', () => {
        const list = [entry('a', 3, now + HOUR), entry('b', 2), entry('c', 1, now - 1)];
        expect(activeOnTop(list, now).map((o) => o.post_id)).toEqual(['a', 'b']);
        expect(nextExpiry(list, now)).toBe(now + HOUR);
        expect(nextExpiry([entry('b', 2)], now)).toBeUndefined();
    });

    describe('the reducer', () => {
        test('loads a channel\'s list, and adds a message put on top first', () => {
            let state = onTopReducer({}, {type: ChannelTypes.RECEIVED_ON_TOP_POSTS, data: {channelId: 'ch', onTop: [entry('a', 2), entry('b', 1)]}});
            expect(state.ch.map((o) => o.post_id)).toEqual(['a', 'b']);

            state = onTopReducer(state, {type: ChannelTypes.POST_PUT_ON_TOP, data: entry('c', 3)});
            expect(state.ch.map((o) => o.post_id)).toEqual(['c', 'a', 'b']);

            // Put on top again: moved first, with its new time.
            state = onTopReducer(state, {type: ChannelTypes.POST_PUT_ON_TOP, data: entry('b', 4, now)});
            expect(state.ch.map((o) => o.post_id)).toEqual(['b', 'c', 'a']);
            expect(state.ch[0].expires_at).toBe(now);
        });

        test('removes a message taken off the top, and forgets everything on logout', () => {
            let state = onTopReducer({}, {type: ChannelTypes.RECEIVED_ON_TOP_POSTS, data: {channelId: 'ch', onTop: [entry('a', 2), entry('b', 1)]}});
            const same = onTopReducer(state, {type: ChannelTypes.POST_REMOVED_FROM_TOP, data: {channel_id: 'ch', post_id: 'zzz'}});
            expect(same).toBe(state);

            state = onTopReducer(state, {type: ChannelTypes.POST_REMOVED_FROM_TOP, data: {channel_id: 'ch', post_id: 'a'}});
            expect(state.ch.map((o) => o.post_id)).toEqual(['b']);
            expect(onTopReducer(state, {type: UserTypes.LOGOUT_SUCCESS})).toEqual({});
        });
    });

    test('finds a message\'s place on top while it\'s there', () => {
        const state = {entities: {channels: {onTop: {ch: [entry('a', 2, now + HOUR), entry('b', 1, now - 1)]}}}} as unknown as GlobalState;
        expect(getOnTopEntry(state, 'ch', 'a', now)?.post_id).toBe('a');
        expect(getOnTopEntry(state, 'ch', 'b', now)).toBeUndefined();
        expect(getOnTopEntry(state, 'other', 'a', now)).toBeUndefined();
    });

    describe('OnTopTag', () => {
        test('tags a message on top with its time left', () => {
            const {container} = renderWithContext(
                <OnTopTag
                    channelId='ch'
                    postId='a'
                />,
                {entities: {channels: {onTop: {ch: [entry('a', 2)]}}}},
            );
            expect(screen.getByText('On top · until removed')).toBeInTheDocument();
            expect(container.querySelector('.am-meta-tag.am-ontop-tag')).toHaveAttribute('title', 'On top of the channel');
        });

        test('shows nothing for a message not on top, or no longer', () => {
            const {container} = renderWithContext(
                <>
                    <OnTopTag
                        channelId='ch'
                        postId='a'
                    />
                    <OnTopTag
                        channelId='ch'
                        postId='other'
                    />
                </>,
                {entities: {channels: {onTop: {ch: [entry('a', 2, Date.now() - 1000)]}}}},
            );
            expect(container).toBeEmptyDOMElement();
        });
    });
});
