// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Post} from '@mattermost/types/posts';

import {Posts} from 'mattermost-redux/constants';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import Files from './content/files';
import {clearImagesShown} from './content/image_fold';
import MessageMenu from './message_menu';

jest.mock('components/file_preview_modal', () => () => null);
jest.mock('components/emoji/render_emoji', () => () => null);
const mockNoEmoji: string[] = [];
jest.mock('selectors/emojis', () => ({...jest.requireActual('selectors/emojis'), getRecentEmojisNames: () => mockNoEmoji}));
jest.mock('fusion/shell/toast_context', () => ({useToast: () => jest.fn()}));
jest.mock('fusion/shell/dialogs_context', () => ({useDialogs: () => ({deleteMessage: jest.fn()})}));
jest.mock('fusion/sidebar/group_dm', () => ({useNotesToSelf: () => jest.fn()}));
jest.mock('fusion/rhs/thread_to', () => ({canMoveThread: () => false}));
jest.mock('./plugin_menu_actions', () => () => null);

const imageFile = {id: 'f1', name: 'cat.png', extension: 'png', mime_type: 'image/png', has_preview_image: true, width: 100, height: 80, size: 1000, post_id: 'p1', user_id: 'u1', create_at: 0, update_at: 0, delete_at: 0};

// Hiding a message's images is in its ⋯ menu (the hover button over the images is gone).
describe('fusion/messages/MessageMenu: hiding images', () => {
    beforeEach(() => {
        clearImagesShown();
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
    });

    const post = (id: string) => ({id, channel_id: 'c1', user_id: 'u1', root_id: '', type: '', message: 'look', create_at: 0, props: {}, metadata: {}} as unknown as Post);
    const state = (collapsed = false) => ({
        entities: {
            users: {currentUserId: 'me', profiles: {me: {id: 'me', roles: 'system_user'}}},
            teams: {currentTeamId: 't1', teams: {t1: {id: 't1', name: 'one', display_name: 'One', delete_at: 0}}},
            channels: {channels: {c1: {id: 'c1', team_id: 't1', type: 'O', name: 'town', display_name: 'Town', delete_at: 0}}},
            posts: {posts: {p1: post('p1'), p2: post('p2')}},
            files: {files: {f1: imageFile}, fileIdsByPostId: {p1: ['f1']}},
            preferences: {myPreferences: collapsed ? {'display_settings--collapse_previews': {category: 'display_settings', name: 'collapse_previews', value: 'true', user_id: 'me'}} : {}},
        },
    } as never);
    const renderMenu = (id: string, collapsed = false) => renderWithContext(
        <>
            <Files post={post(id)}/>
            <MessageMenu
                post={post(id)}
                anchor={null}
                inThread={false}
                onClose={jest.fn()}
                onMoreReactions={jest.fn()}
            />
        </>,
        state(collapsed),
    );

    test('a message with an image offers Hide image, which hides it', async () => {
        renderMenu('p1');
        expect(screen.getByRole('button', {name: 'Open image cat.png'})).toBeInTheDocument();
        const hide = screen.getByRole('menuitem', {name: 'Hide image'});
        expect(hide.querySelector('use')).toHaveAttribute('href', '#am-i-eye-off');

        await userEvent.click(hide);
        expect(screen.queryByRole('button', {name: 'Open image cat.png'})).not.toBeInTheDocument();
        expect(screen.getByRole('menuitem', {name: 'Show image'})).toBeInTheDocument();
    });

    test('with previews collapsed, it offers Show image', async () => {
        renderMenu('p1', true);
        expect(screen.queryByRole('button', {name: 'Open image cat.png'})).not.toBeInTheDocument();
        await userEvent.click(screen.getByRole('menuitem', {name: 'Show image'}));
        expect(screen.getByRole('button', {name: 'Open image cat.png'})).toBeInTheDocument();
    });

    test('a message without images has no such entry', () => {
        renderMenu('p2');
        expect(screen.getByRole('menu', {name: 'Message actions'})).toBeInTheDocument();
        expect(screen.queryByRole('menuitem', {name: /image/})).not.toBeInTheDocument();
    });
});

// As in the classic menu, Pin is only offered where the server allows it.
describe('fusion/messages/MessageMenu: pinning', () => {
    beforeEach(() => {
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
    });

    const post = (type = '') => ({id: 'p1', channel_id: 'c1', user_id: 'u1', root_id: '', type, message: 'look', create_at: 0, props: {}, metadata: {}} as unknown as Post);
    const renderMenu = (p: Post, archived = false) => renderWithContext(
        <MessageMenu
            post={p}
            anchor={null}
            inThread={false}
            onClose={jest.fn()}
            onMoreReactions={jest.fn()}
        />,
        {
            entities: {
                users: {currentUserId: 'me', profiles: {me: {id: 'me', roles: 'system_user'}}},
                teams: {currentTeamId: 't1', teams: {t1: {id: 't1', name: 'one', display_name: 'One', delete_at: 0}}},
                channels: {channels: {c1: {id: 'c1', team_id: 't1', type: 'O', name: 'town', display_name: 'Town', delete_at: archived ? 1 : 0}}},
                posts: {posts: {p1: p}},
            },
        } as never,
    );

    test('a message can be pinned', () => {
        renderMenu(post());
        expect(screen.getByRole('menuitem', {name: 'Pin to channel'})).toBeInTheDocument();
    });

    test('a burn-on-read message is not offered to pin', () => {
        renderMenu(post(Posts.POST_TYPES.BURN_ON_READ));
        expect(screen.getByRole('menu', {name: 'Message actions'})).toBeInTheDocument();
        expect(screen.queryByRole('menuitem', {name: 'Pin to channel'})).not.toBeInTheDocument();
    });

    test('nothing is offered to pin in an archived channel', () => {
        renderMenu(post(), true);
        expect(screen.queryByRole('menuitem', {name: 'Pin to channel'})).not.toBeInTheDocument();
    });
});
