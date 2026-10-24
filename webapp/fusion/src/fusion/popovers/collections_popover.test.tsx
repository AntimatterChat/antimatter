// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {Client4} from 'mattermost-redux/client';

import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import CollectionsPopover from './collections_popover';

const mockMarkAllRead = jest.fn(() => ({type: 'NOOP_MARK_ALL_READ'}));
jest.mock('mattermost-redux/actions/threads', () => ({
    ...jest.requireActual('mattermost-redux/actions/threads'),
    markAllThreadsInTeamRead: (...args: unknown[]) => mockMarkAllRead(...args),
}));

const mockDialogs = {confirm: jest.fn((options: {onConfirm: () => unknown}) => options.onConfirm())};
jest.mock('fusion/shell/dialogs_context', () => ({useDialogs: () => mockDialogs}));

const mockToast = jest.fn();
jest.mock('fusion/shell/toast_context', () => ({useToast: () => mockToast}));

// Antimatter: the Files tab only shows when ServiceSettings.EnableFileSearch (and
// FileSettings.EnableFileAttachments) are on, as the classic search results already hide.
describe('fusion/popovers/CollectionsPopover: the Files tab', () => {
    beforeEach(() => {
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
    });

    const channel = TestHelper.getChannelMock({id: 'channel1', name: 'test-channel'});

    const renderPopover = (config: Record<string, string>) => renderWithContext(
        <CollectionsPopover
            anchor={document.body}
            channel={channel}
            onClose={jest.fn()}
        />,
        {
            entities: {
                general: {config},
                channels: {channels: {[channel.id]: channel}},
            },
        } as never,
    );

    test('shows the Files tab when file search is enabled', () => {
        renderPopover({EnableFileAttachments: 'true', EnableFileSearch: 'true'});
        expect(screen.getByText('Files')).toBeInTheDocument();
    });

    test('hides the Files tab when file search is disabled', () => {
        renderPopover({EnableFileAttachments: 'true', EnableFileSearch: 'false'});
        expect(screen.queryByText('Files')).not.toBeInTheDocument();
    });

    test('hides the Files tab when file attachments are disabled', () => {
        renderPopover({EnableFileAttachments: 'false', EnableFileSearch: 'true'});
        expect(screen.queryByText('Files')).not.toBeInTheDocument();
    });

    test('falls back to Mentions when the Files tab was requested but file search is disabled', () => {
        renderWithContext(
            <CollectionsPopover
                anchor={document.body}
                channel={channel}
                initialTab='files'
                onClose={jest.fn()}
            />,
            {
                entities: {
                    general: {config: {EnableFileAttachments: 'true', EnableFileSearch: 'false'}},
                    channels: {channels: {[channel.id]: channel}},
                },
            } as never,
        );
        expect(screen.queryByText('Files')).not.toBeInTheDocument();
        expect(screen.getByRole('heading', {name: 'Mentions'})).toBeInTheDocument();
    });
});

// F121: All/Unreads sub-tabs and "Mark all as read" on the threads tab.
describe('fusion/popovers/CollectionsPopover: the Threads tab', () => {
    const team = TestHelper.getTeamMock({id: 'team1', name: 'lumen'});

    beforeEach(() => {
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
        mockMarkAllRead.mockClear();
        mockDialogs.confirm.mockClear();
        mockToast.mockClear();
        jest.spyOn(Client4, 'getUserThreads').mockResolvedValue({threads: [], total: 0, total_unread_threads: 0, total_unread_mentions: 0});
    });

    const renderThreads = () => renderWithContext(
        <CollectionsPopover
            anchor={document.body}
            initialTab='threads'
            onClose={jest.fn()}
        />,
        {
            entities: {
                general: {config: {}},
                users: {currentUserId: 'me'},
                teams: {currentTeamId: team.id, teams: {[team.id]: team}},
                channels: {channels: {}},
            },
        } as never,
    );

    test('offers All and Unreads, with All selected by default', async () => {
        renderThreads();
        await waitFor(() => expect(Client4.getUserThreads).toHaveBeenCalled());
        expect(screen.getByRole('radio', {name: 'All'})).toHaveAttribute('aria-checked', 'true');
        expect(screen.getByRole('radio', {name: 'Unreads'})).toHaveAttribute('aria-checked', 'false');
    });

    test('fetches all followed threads, then only the unread ones when asked', async () => {
        renderThreads();
        await waitFor(() => expect(Client4.getUserThreads).toHaveBeenCalledWith('me', 'team1', expect.objectContaining({unread: false})));
        await userEvent.click(screen.getByRole('radio', {name: 'Unreads'}));
        await waitFor(() => expect(Client4.getUserThreads).toHaveBeenCalledWith('me', 'team1', expect.objectContaining({unread: true})));
    });

    test('marks every followed thread as read, with a confirmation', async () => {
        renderThreads();
        await waitFor(() => expect(Client4.getUserThreads).toHaveBeenCalled());
        await userEvent.click(screen.getByRole('button', {name: 'Mark all as read'}));
        expect(mockDialogs.confirm).toHaveBeenCalled();
        expect(mockMarkAllRead).toHaveBeenCalledWith('me', 'team1');
        await waitFor(() => expect(mockToast).toHaveBeenCalledWith('All threads marked as read'));
    });
});
