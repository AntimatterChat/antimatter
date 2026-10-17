// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {PostPriority} from '@mattermost/types/posts';
import type {OnTopPost, Post} from '@mattermost/types/posts';

import {ChannelTypes} from 'mattermost-redux/action_types';
import {Client4} from 'mattermost-redux/client';

import {act, renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import OnTopStrip, {onTopShown} from './on_top_strip';

// The message's rendered text is the classic renderer's; its plain text is enough here.
jest.mock('fusion/messages/content/message_content', () => ({
    __esModule: true,
    default: ({post}: {post: Post}) => <p>{post.message}</p>,
}));

const HOUR = 3600000;

describe('fusion/channel/OnTopStrip', () => {
    const me = TestHelper.getUserMock({id: 'me', username: 'jason'});
    const marie = TestHelper.getUserMock({id: 'marie', username: 'marie'});
    const channel = TestHelper.getChannelMock({id: 'ch', name: 'general', display_name: 'General', team_id: 'team', delete_at: 0});
    const post = (id: string, message: string, extra: Partial<Post> = {}) => TestHelper.getPostMock({id, channel_id: 'ch', user_id: 'marie', message, type: '', ...extra});
    const entry = (postId: string, createAt: number, expiresAt = 0, by = 'me'): OnTopPost => ({post_id: postId, channel_id: 'ch', created_by: by, create_at: createAt, expires_at: expiresAt});

    const later = Date.now() + HOUR;
    const posts = {
        first: post('first', 'Beam test at 14:00'),
        second: post('second', 'Firmware 0.9.3 is out', {metadata: {embeds: [], emojis: [], files: [], images: {}, reactions: [], priority: {priority: PostPriority.URGENT}}}),
        third: post('third', 'Read the runbook'),
    };
    const list = [entry('first', 3, later), entry('second', 2, 0, 'marie'), entry('third', 1)];

    const state = (onTop: OnTopPost[], postsById: Record<string, Post> = posts, channelPermissions = ['create_post', 'manage_channel_roles']) => ({
        entities: {
            users: {currentUserId: me.id, profiles: {me: {...me, roles: 'system_user'}, marie}},
            posts: {posts: postsById},
            channels: {onTop: {ch: onTop}, channels: {ch: channel}, myMembers: {ch: {channel_id: 'ch', user_id: 'me', roles: 'channel_user'}}, roles: {ch: new Set(['channel_user'])}},
            teams: {currentTeamId: 'team', teams: {team: TestHelper.getTeamMock({id: 'team', name: 'team'})}, myMembers: {team: {team_id: 'team', user_id: 'me', roles: 'team_user'}}},
            roles: {roles: {
                system_user: {name: 'system_user', permissions: []},
                team_user: {name: 'team_user', permissions: []},
                channel_user: {name: 'channel_user', permissions: channelPermissions},
            }},
            general: {config: {PostPriority: 'true'}},
        },
    });

    beforeEach(() => {
        jest.spyOn(Client4, 'getOnTopPosts').mockResolvedValue(list);
        jest.spyOn(Client4, 'getProfilesByIds').mockResolvedValue([]);
    });

    afterEach(() => {
        jest.restoreAllMocks();
    });

    test('leaves out expired, missing and deleted messages', () => {
        const now = Date.now();
        const shown = onTopShown(
            [entry('first', 4, now - 1), entry('second', 3), entry('gone', 2), entry('third', 1)],
            {first: posts.first, second: posts.second, third: {...posts.third, delete_at: 5, state: 'DELETED'}},
            now,
        );
        expect(shown.map((o) => o.post_id)).toEqual(['second']);
    });

    test('shows the latest put on top first, who put it there and its time left', async () => {
        renderWithContext(<OnTopStrip channel={channel}/>, state(list));

        const region = await screen.findByRole('region', {name: 'On top of this channel'});
        expect(region).toHaveTextContent('Beam test at 14:00');
        expect(region).toHaveTextContent('put on top by you');
        expect(region).toHaveTextContent(/(59|60)m left|1h 0m left/);
        expect(region).toHaveTextContent('1/3');
        expect(screen.getByText('1/3')).toHaveAttribute('aria-live', 'polite');
        expect(region).not.toHaveClass('am-urgent');
    });

    test('goes to the next and previous messages, an urgent one in red', async () => {
        renderWithContext(<OnTopStrip channel={channel}/>, state(list));
        const region = await screen.findByRole('region', {name: 'On top of this channel'});

        await userEvent.click(screen.getByRole('button', {name: 'Next'}));
        expect(region).toHaveTextContent('Firmware 0.9.3 is out');
        expect(region).toHaveTextContent('2/3');
        expect(region).toHaveTextContent('put on top by marie');
        expect(region).toHaveTextContent('until removed');
        expect(region).toHaveClass('am-urgent');

        await userEvent.click(screen.getByRole('button', {name: 'Previous'}));
        await userEvent.click(screen.getByRole('button', {name: 'Previous'}));
        expect(region).toHaveTextContent('Read the runbook');
        expect(region).toHaveTextContent('3/3');
    });

    test('has no navigation for a single message, and shows nothing without one', async () => {
        jest.spyOn(Client4, 'getOnTopPosts').mockResolvedValue([entry('third', 1)]);
        renderWithContext(<OnTopStrip channel={channel}/>, state([entry('third', 1)]));
        await screen.findByRole('region', {name: 'On top of this channel'});
        expect(screen.queryByRole('button', {name: 'Next'})).not.toBeInTheDocument();

        jest.spyOn(Client4, 'getOnTopPosts').mockResolvedValue([]);
        const {container} = renderWithContext(<OnTopStrip channel={{...channel, id: 'empty'}}/>, state([]));
        await act(async () => {});
        expect(container).toBeEmptyDOMElement();
    });

    test('takes the message shown off the top', async () => {
        const remove = jest.spyOn(Client4, 'removePostFromTop').mockResolvedValue({status: 'OK'});
        renderWithContext(<OnTopStrip channel={channel}/>, state(list));
        await screen.findByRole('region', {name: 'On top of this channel'});

        await userEvent.click(screen.getByRole('button', {name: 'Remove from top'}));
        expect(remove).toHaveBeenCalledWith('first');
        await waitFor(() => expect(screen.getByRole('region', {name: 'On top of this channel'})).toHaveTextContent('Firmware 0.9.3 is out'));
        expect(screen.getByRole('region', {name: 'On top of this channel'})).toHaveTextContent('1/2');
    });

    test('has no remove button where the server would refuse it', async () => {
        renderWithContext(<OnTopStrip channel={channel}/>, state(list, posts, []));
        await screen.findByRole('region', {name: 'On top of this channel'});
        expect(screen.queryByRole('button', {name: 'Remove from top'})).not.toBeInTheDocument();
    });

    test('has no remove button for members, who only post', async () => {
        renderWithContext(<OnTopStrip channel={channel}/>, state(list, posts, ['create_post']));
        await screen.findByRole('region', {name: 'On top of this channel'});
        expect(screen.queryByRole('button', {name: 'Remove from top'})).not.toBeInTheDocument();
    });

    test('has no remove button in an archived channel', async () => {
        renderWithContext(<OnTopStrip channel={{...channel, delete_at: 5}}/>, state(list));
        await screen.findByRole('region', {name: 'On top of this channel'});
        expect(screen.queryByRole('button', {name: 'Remove from top'})).not.toBeInTheDocument();
    });

    test('loads again the messages whose loading failed', async () => {
        const fetch = jest.spyOn(Client4, 'getPostsByIds').mockRejectedValueOnce(new Error('offline')).mockResolvedValue([]);
        const {store} = renderWithContext(<OnTopStrip channel={channel}/>, state(list, {first: posts.first}));
        await waitFor(() => expect(fetch).toHaveBeenCalledTimes(1));
        await act(async () => {});

        // What's on top changes, and the strip looks again for what's missing.
        await act(async () => {
            store.dispatch({type: ChannelTypes.POST_PUT_ON_TOP, data: entry('fourth', 4)});
        });
        await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
        expect(fetch).toHaveBeenLastCalledWith(expect.arrayContaining(['second', 'third']));
    });

    test('loads the messages on top that aren\'t loaded', async () => {
        const fetch = jest.spyOn(Client4, 'getPostsByIds').mockResolvedValue([]);
        renderWithContext(<OnTopStrip channel={channel}/>, state(list, {first: posts.first}));
        await waitFor(() => expect(fetch).toHaveBeenCalledWith(['second', 'third']));
    });
});
