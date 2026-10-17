// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useSelector} from 'react-redux';

import {addChannelMembers, createChannel} from 'mattermost-redux/actions/channels';
import {createPost, moveThreadToChannel} from 'mattermost-redux/actions/posts';

import {switchToChannel} from 'actions/views/channel';

import {LAYER_ID} from 'fusion/components/layer';
import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import type {GlobalState} from 'types/store';

import {canMoveThread, ThreadSuggestion, ThreadToChannelDialog} from './thread_to';

jest.mock('mattermost-redux/actions/channels', () => ({
    ...jest.requireActual('mattermost-redux/actions/channels'),
    createChannel: jest.fn((channel: {name: string}) => ({type: 'CREATED', data: {...channel, id: 'newch'}})),
    addChannelMembers: jest.fn(() => ({type: 'ADDED', data: true})),
}));
jest.mock('mattermost-redux/actions/posts', () => ({
    ...jest.requireActual('mattermost-redux/actions/posts'),
    createPost: jest.fn(() => ({type: 'POSTED', data: true})),
    moveThreadToChannel: jest.fn(() => ({type: 'MOVED', data: {id: 'moved'}})),
}));
jest.mock('actions/views/channel', () => ({
    ...jest.requireActual('actions/views/channel'),
    switchToChannel: jest.fn(() => ({type: 'SWITCHED'})),
}));

const me = TestHelper.getUserMock({id: 'me', username: 'me', roles: 'system_user'});
const team = TestHelper.getTeamMock({id: 'team', name: 'lumen', display_name: 'Lumen Labs'});
const channel = TestHelper.getChannelMock({id: 'ch', name: 'beam-schedule', display_name: 'Beam schedule', type: 'O', team_id: 'team', delete_at: 0});
const dm = TestHelper.getChannelMock({id: 'dm', name: 'me__bob', type: 'D', team_id: '', delete_at: 0});
const root = TestHelper.getPostMock({id: 'root', channel_id: 'ch', user_id: 'me', root_id: '', message: 'Shall we move the **beam** test to Friday?', props: {}});
const replyBob = TestHelper.getPostMock({id: 'r1', channel_id: 'ch', user_id: 'bob', root_id: 'root', message: 'Fine by me', create_at: 2});
const replyMaya = TestHelper.getPostMock({id: 'r2', channel_id: 'ch', user_id: 'maya', root_id: 'root', message: 'Friday works', create_at: 3});

function state({teamPermissions = ['create_public_channel', 'create_private_channel'], channelPermissions = ['edit_post'], posts = [root, replyBob, replyMaya], channelAdmin = false, systemPermissions = []}: {teamPermissions?: string[]; channelPermissions?: string[]; posts?: Array<ReturnType<typeof TestHelper.getPostMock>>; channelAdmin?: boolean; systemPermissions?: string[]} = {}) {
    return {
        entities: {
            general: {config: {CollapsedThreads: 'always_on', PostEditTimeLimit: '-1'}},
            users: {currentUserId: 'me', profiles: {me}},
            teams: {currentTeamId: 'team', teams: {team}, myMembers: {team: {team_id: 'team', user_id: 'me', roles: 'team_user'}}},
            channels: {
                currentChannelId: 'ch',
                channels: {ch: channel, dm},
                myMembers: {ch: {channel_id: 'ch', user_id: 'me', roles: 'channel_user', scheme_admin: channelAdmin}, dm: {channel_id: 'dm', user_id: 'me', roles: 'channel_user'}},
                roles: {ch: new Set(['channel_user']), dm: new Set(['channel_user'])},
            },
            posts: {
                posts: Object.fromEntries(posts.map((p) => [p.id, p])),
                postsInThread: {root: posts.filter((p) => p.root_id === 'root').map((p) => p.id)},
            },
            roles: {roles: {
                system_user: {name: 'system_user', permissions: systemPermissions},
                team_user: {name: 'team_user', permissions: teamPermissions},
                channel_user: {name: 'channel_user', permissions: channelPermissions},
            }},
            preferences: {myPreferences: {}},
        },
    } as never;
}

describe('fusion/rhs/ThreadSuggestion', () => {
    test('proposes to make a thread with replies a topic or a channel', async () => {
        const onConvert = jest.fn();
        renderWithContext(
            <ThreadSuggestion
                root={root}
                replyCount={2}
                onConvert={onConvert}
            />,
            state(),
        );
        await userEvent.click(screen.getByRole('button', {name: 'Make it a topic'}));
        expect(onConvert).toHaveBeenLastCalledWith('topic');
        await userEvent.click(screen.getByRole('button', {name: 'Make it a channel'}));
        expect(onConvert).toHaveBeenLastCalledWith('channel');
    });

    test('offers only a channel to who may not edit the first message', () => {
        renderWithContext(
            <ThreadSuggestion
                root={root}
                replyCount={2}
                onConvert={jest.fn()}
            />,
            state({channelPermissions: []}),
        );
        expect(screen.queryByRole('button', {name: 'Make it a topic'})).not.toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Make it a channel'})).toBeInTheDocument();
    });

    test('offers only a topic to who may not create channels', () => {
        renderWithContext(
            <ThreadSuggestion
                root={root}
                replyCount={2}
                onConvert={jest.fn()}
            />,
            state({teamPermissions: []}),
        );
        expect(screen.getByRole('button', {name: 'Make it a topic'})).toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Make it a channel'})).not.toBeInTheDocument();
    });

    test('stays away from topics, threads without replies, and direct messages', () => {
        const topic = {...root, id: 'topic', props: {am_thread_title: 'Beam test'}};
        const {rerender} = renderWithContext(
            <ThreadSuggestion
                root={topic}
                replyCount={2}
                onConvert={jest.fn()}
            />,
            state({posts: [topic]}),
        );
        expect(screen.queryByRole('note')).not.toBeInTheDocument();

        rerender(
            <ThreadSuggestion
                root={root}
                replyCount={0}
                onConvert={jest.fn()}
            />,
        );
        expect(screen.queryByRole('note')).not.toBeInTheDocument();

        rerender(
            <ThreadSuggestion
                root={{...root, id: 'dmroot', channel_id: 'dm', user_id: 'bob'}}
                replyCount={2}
                onConvert={jest.fn()}
            />,
        );
        expect(screen.queryByRole('note')).not.toBeInTheDocument();
    });

    test('can be dismissed', async () => {
        const other = {...root, id: 'other'};
        renderWithContext(
            <ThreadSuggestion
                root={other}
                replyCount={1}
                onConvert={jest.fn()}
            />,
            state({posts: [other]}),
        );
        await userEvent.click(screen.getByRole('button', {name: 'Dismiss'}));
        expect(screen.queryByRole('note')).not.toBeInTheDocument();
    });
});

describe('fusion/rhs/ThreadToChannelDialog', () => {
    beforeEach(() => {
        const layer = document.createElement('div');
        layer.id = LAYER_ID;
        document.body.appendChild(layer);
        jest.clearAllMocks();
    });
    afterEach(() => {
        document.getElementById(LAYER_ID)?.remove();
    });

    test('creates a channel named after the thread, brings its people, and moves the thread\'s messages there', async () => {
        const onClose = jest.fn();
        renderWithContext(
            <ThreadToChannelDialog
                root={root}
                onClose={onClose}
            />,
            state(),
        );
        expect(screen.getByRole('textbox', {name: 'Name'})).toHaveValue('Shall we move the beam test to Friday?');
        await userEvent.clear(screen.getByRole('textbox', {name: 'Name'}));
        await userEvent.type(screen.getByRole('textbox', {name: 'Name'}), 'Beam test');
        expect(screen.getByRole('switch', {name: 'Add the 2 other people in the thread'})).toBeChecked();

        await userEvent.click(screen.getByRole('button', {name: 'Create the channel'}));
        await waitFor(() => expect(onClose).toHaveBeenCalled());

        expect(createChannel).toHaveBeenCalledWith(expect.objectContaining({team_id: 'team', name: 'beam-test', display_name: 'Beam test', type: 'O'}), 'me');
        expect(addChannelMembers).toHaveBeenCalledWith('newch', ['bob', 'maya']);
        expect(moveThreadToChannel).toHaveBeenCalledWith('root', 'newch', true);
        expect(createPost).not.toHaveBeenCalled();
        expect(switchToChannel).toHaveBeenCalledWith(expect.objectContaining({id: 'newch'}));
    });

    test('who may not move the thread starts the channel with links both ways', async () => {
        const bobsRoot = {...root, id: 'bobroot', user_id: 'bob'};
        const onClose = jest.fn();
        renderWithContext(
            <ThreadToChannelDialog
                root={bobsRoot}
                onClose={onClose}
            />,
            state({posts: [bobsRoot]}),
        );
        expect(screen.getByText(/the replies stay in the thread/)).toBeInTheDocument();
        await userEvent.click(screen.getByRole('button', {name: 'Create the channel'}));
        await waitFor(() => expect(onClose).toHaveBeenCalled());

        expect(moveThreadToChannel).not.toHaveBeenCalled();
        expect(createPost).toHaveBeenCalledWith(expect.objectContaining({channel_id: 'newch', user_id: 'me', message: expect.stringMatching(/^Continued from a thread in #beam-schedule: .*\/lumen\/pl\/bobroot$/)}));
        expect(createPost).toHaveBeenCalledWith(expect.objectContaining({channel_id: 'ch', root_id: 'bobroot', user_id: 'me', message: expect.stringMatching(/^This conversation continues in #shall-we-move/)}));
    });

    test('links both ways when the move fails', async () => {
        (moveThreadToChannel as jest.Mock).mockImplementationOnce(() => ({type: 'FAILED', error: {message: 'Nope'}}));
        const onClose = jest.fn();
        renderWithContext(
            <ThreadToChannelDialog
                root={root}
                onClose={onClose}
            />,
            state(),
        );
        await userEvent.click(screen.getByRole('button', {name: 'Create the channel'}));
        await waitFor(() => expect(onClose).toHaveBeenCalled());
        expect(createPost).toHaveBeenCalledTimes(2);
    });

    test('can leave the thread\'s people out, and make the channel private', async () => {
        renderWithContext(
            <ThreadToChannelDialog
                root={root}
                onClose={jest.fn()}
            />,
            state(),
        );
        await userEvent.click(screen.getByRole('switch', {name: 'Add the 2 other people in the thread'}));
        await userEvent.click(screen.getByRole('switch', {name: 'Private channel'}));
        await userEvent.click(screen.getByRole('button', {name: 'Create the channel'}));
        await waitFor(() => expect(createChannel).toHaveBeenCalledWith(expect.objectContaining({type: 'P'}), 'me'));
        expect(addChannelMembers).not.toHaveBeenCalled();
    });

    test('shows why the channel couldn\'t be made', async () => {
        (createChannel as jest.Mock).mockImplementationOnce(() => ({type: 'FAILED', error: {message: 'A channel with that name already exists'}}));
        const onClose = jest.fn();
        renderWithContext(
            <ThreadToChannelDialog
                root={root}
                onClose={onClose}
            />,
            state(),
        );
        await userEvent.click(screen.getByRole('button', {name: 'Create the channel'}));
        expect(await screen.findByRole('alert')).toHaveTextContent('A channel with that name already exists');
        expect(createPost).not.toHaveBeenCalled();
        expect(onClose).not.toHaveBeenCalled();
    });
});

describe('fusion/rhs/canMoveThread', () => {
    const othersRoot = {...root, id: 'other', user_id: 'bob'};
    function Probe({post}: {post: typeof root}) {
        const ok = useSelector((st: GlobalState) => canMoveThread(st, post));
        return <span data-testid='can'>{String(ok)}</span>;
    }
    const can = (st: never, post = othersRoot) => {
        const {unmount} = renderWithContext(<Probe post={post}/>, st);
        const ok = screen.getByTestId('can').textContent === 'true';
        unmount();
        return ok;
    };

    test('the author may, a plain member may not', () => {
        expect(can(state(), root)).toBe(true);
        expect(can(state())).toBe(false);
    });

    test('the channel\'s admins, the community\'s admins, "Manage messages" and system admins may', () => {
        expect(can(state({channelAdmin: true}))).toBe(true);
        expect(can(state({teamPermissions: ['manage_team']}))).toBe(true);
        expect(can(state({channelPermissions: ['delete_others_posts']}))).toBe(true);
        expect(can(state({systemPermissions: ['manage_system']}))).toBe(true);
    });

    test('never in direct messages nor archived channels', () => {
        expect(can(state(), {...root, channel_id: 'dm'})).toBe(false);
        const st = state() as unknown as {entities: {channels: {channels: Record<string, unknown>}}};
        st.entities.channels.channels.ch = {...channel, delete_at: 1};
        expect(can(st as never, root)).toBe(false);
    });
});
