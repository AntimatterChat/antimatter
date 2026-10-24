// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Post} from '@mattermost/types/posts';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';
import {getHistory} from 'utils/browser_history';

import ThreadMenu from './thread_menu';

jest.mock('mattermost-redux/selectors/entities/preferences', () => ({
    ...jest.requireActual('mattermost-redux/selectors/entities/preferences'),
    isCollapsedThreadsEnabled: () => true,
}));
jest.mock('fusion/messages/plugin_menu_actions', () => ({
    __esModule: true,
    default: () => null,
    usePluginMenuActions: () => ({any: false}),
}));
jest.mock('./thread_to', () => ({useThreadConversions: () => ({topic: false, renameTopic: false, channel: false})}));

const mockCloseRHS = jest.fn(() => ({type: 'NOOP_CLOSE_RHS'}));
jest.mock('actions/views/rhs', () => ({closeRightHandSide: () => mockCloseRHS()}));

const mockToast = jest.fn();
jest.mock('fusion/shell/toast_context', () => ({useToast: () => mockToast}));

const root = (overrides: Partial<Post> = {}) => ({
    id: 'p1',
    channel_id: 'c1',
    user_id: 'u1',
    root_id: '',
    type: '',
    message: 'hi there',
    create_at: 1000,
    is_following: true,
    ...overrides,
} as unknown as Post);

function state(overrides: {unreadReplies?: number; flagged?: boolean} = {}) {
    return {
        entities: {
            users: {currentUserId: 'me'},
            teams: {currentTeamId: 't1', teams: {t1: {id: 't1', name: 'lumen', display_name: 'Lumen', delete_at: 0}}},
            channels: {channels: {c1: {id: 'c1', team_id: 't1', type: 'O', name: 'dev', display_name: 'dev', delete_at: 0}}},
            threads: {threads: overrides.unreadReplies === undefined ? {} : {p1: {id: 'p1', unread_replies: overrides.unreadReplies}}},
            preferences: {myPreferences: overrides.flagged ? {'flagged_post--p1': {category: 'flagged_post', name: 'p1', value: 'true', user_id: 'me'}} : {}},
        },
    } as never;
}

describe('fusion/rhs/ThreadMenu: missing entries (F121)', () => {
    beforeEach(() => {
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
    });

    const render = (overrides: {unreadReplies?: number; flagged?: boolean} = {}, onClose = jest.fn()) => {
        renderWithContext(
            <ThreadMenu
                root={root()}
                lastReplyAt={2000}
                anchor={null}
                onClose={onClose}
                onConvert={jest.fn()}
            />,
            state(overrides),
        );
        return onClose;
    };

    test('opens where the message lives in the channel, closing the thread panel', async () => {
        const onClose = render();
        await userEvent.click(screen.getByRole('menuitem', {name: 'Open in channel'}));
        expect(mockCloseRHS).toHaveBeenCalled();
        expect(getHistory().push).toHaveBeenCalledWith('/lumen/pl/p1');
        expect(onClose).toHaveBeenCalledTimes(1);
    });

    test('offers Mark as unread when the thread has no unread replies', () => {
        render({unreadReplies: 0});
        expect(screen.getByRole('menuitem', {name: 'Mark as unread'})).toBeInTheDocument();
        expect(screen.queryByRole('menuitem', {name: 'Mark as read'})).not.toBeInTheDocument();
    });

    test('offers Mark as read once the thread has unread replies, and marks it read', async () => {
        const onClose = render({unreadReplies: 3});
        expect(screen.queryByRole('menuitem', {name: 'Mark as unread'})).not.toBeInTheDocument();
        await userEvent.click(screen.getByRole('menuitem', {name: 'Mark as read'}));
        expect(mockToast).toHaveBeenCalledWith('Thread marked as read');
        expect(onClose).toHaveBeenCalledTimes(1);
    });

    test('offers to save the thread\'s first message, and to remove it from saved once saved', async () => {
        render();
        await userEvent.click(screen.getByRole('menuitem', {name: 'Save'}));
        expect(mockToast).toHaveBeenCalledWith('Saved — find it in Saved messages');
    });

    test('offers to remove an already saved message from saved', async () => {
        render({flagged: true});
        expect(screen.queryByRole('menuitem', {name: 'Save'})).not.toBeInTheDocument();
        await userEvent.click(screen.getByRole('menuitem', {name: 'Remove from saved'}));
        expect(mockToast).toHaveBeenCalledWith('Removed from saved');
    });
});
