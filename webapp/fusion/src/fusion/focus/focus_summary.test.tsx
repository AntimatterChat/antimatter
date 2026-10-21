// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {FocusSummary} from '@mattermost/types/focus';

import {Client4} from 'mattermost-redux/client';

import {selectPostById} from 'actions/views/rhs';

import webSocketClient from 'client/web_websocket_client';
import {act, renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {getHistory} from 'utils/browser_history';

import {setFocusSummary, useHideUnreads} from './focus';
import FocusSummaryHost from './focus_summary';

const mockToast = jest.fn();
jest.mock('fusion/shell/toast_context', () => ({useToast: () => mockToast}));
jest.mock('actions/views/rhs', () => ({selectPostById: jest.fn(() => ({type: 'MOCK_SELECT_POST'}))}));
jest.mock('mattermost-redux/actions/users', () => ({
    ...jest.requireActual('mattermost-redux/actions/users'),
    getMissingProfilesByIds: jest.fn(() => ({type: 'MISSING_PROFILES'})),
}));
const mockSavePreferences = jest.fn((userId: string, preferences: unknown[]) => ({type: 'SAVE_PREFERENCES', userId, preferences}));
jest.mock('mattermost-redux/actions/preferences', () => ({
    ...jest.requireActual('mattermost-redux/actions/preferences'),
    savePreferences: (userId: string, preferences: unknown[]) => mockSavePreferences(userId, preferences),
}));

let mockListeners: Array<(message: unknown) => void> = [];
jest.mock('client/web_websocket_client', () => ({
    __esModule: true,
    default: {
        addMessageListener: jest.fn((listener) => mockListeners.push(listener)),
        removeMessageListener: jest.fn((listener) => {
            mockListeners = mockListeners.filter((l) => l !== listener);
        }),
        addReconnectListener: jest.fn(),
        removeReconnectListener: jest.fn(),
    },
}));

// 14:00 to 15:30 in Paris.
const START = new Date('2026-10-21T12:00:00Z').getTime();
const END = new Date('2026-10-21T13:30:00Z').getTime();

const summary: FocusSummary = {
    since: START,
    until: END,
    mention_count: 3,
    channels: [
        {channel_id: 'dm', team_id: '', type: 'D', name: 'maya__me', display_name: '', count: 2, mention_count: 2, last_post_at: END - 1000, last_sender_id: 'maya'},
        {channel_id: 'release', team_id: 't1', type: 'O', name: 'release', display_name: 'Release', count: 1200, mention_count: 1, last_post_at: END - 2000, last_sender_id: 'maya'},
    ],
    threads: [
        {post_id: 'root', channel_id: 'release', team_id: 't1', count: 1, mention_count: 0, last_reply_at: END - 3000, last_sender_id: 'maya'},
    ],
};

function Badges() {
    return <span data-testid='badges'>{useHideUnreads() ? 'hidden' : 'shown'}</span>;
}

describe('fusion/focus/FocusSummaryHost', () => {
    const render = (seen = '') => renderWithContext(
        <>
            <FocusSummaryHost/>
            <Badges/>
        </>,
        {
            entities: {
                users: {
                    currentUserId: 'me',
                    profiles: {
                        me: {id: 'me', username: 'me', props: {}, timezone: {useAutomaticTimezone: 'false', manualTimezone: 'Europe/Paris', automaticTimezone: ''}},
                        maya: {id: 'maya', username: 'maya', first_name: 'Maya', last_name: 'Lund'},
                    },
                },
                teams: {currentTeamId: 't1', teams: {t1: {id: 't1', name: 'lumen', display_name: 'Lumen'}}},
                channels: {channels: {release: {id: 'release', team_id: 't1', type: 'O', name: 'release', display_name: 'Release'}}},
                preferences: {myPreferences: seen ? {'focus--summary_seen': {category: 'focus', name: 'summary_seen', user_id: 'me', value: seen}} : {}},
                general: {config: {}},
            },
        } as never,
    );
    const ended = () => act(() => {
        mockListeners.forEach((listener) => listener({event: 'focus_ended', data: {start_at: START, end_at: END, ended_by: 'timer'}}));
    });

    beforeEach(() => {
        jest.clearAllMocks();
        mockListeners = [];
        setFocusSummary(null);
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
        Client4.getFocus = jest.fn().mockResolvedValue({active: false, start_at: 0, end_at: 0, reason: '', ended_at: 0});
        Client4.getFocusSummary = jest.fn().mockResolvedValue(summary);
        Client4.markFocusSummaryRead = jest.fn().mockResolvedValue({status: 'OK'});
    });

    test('shows what you missed when the focus ends, direct messages first, with links', async () => {
        render();
        await waitFor(() => expect(webSocketClient.addMessageListener).toHaveBeenCalled());
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();

        ended();
        const dialog = await screen.findByRole('dialog', {name: /^While you were focusing \(/});
        expect(Client4.getFocusSummary).toHaveBeenCalledWith(START, END);
        expect(dialog).toHaveAccessibleName(expect.stringMatching(/2:00.*3:30/));
        expect(screen.getByText('3 mentions wait for you.')).toBeInTheDocument();

        const headings = screen.getAllByRole('heading', {level: 3}).map((h) => h.textContent);
        expect(headings).toEqual(['Direct messages', 'Channels', 'Threads']);
        expect(screen.getByRole('button', {name: /^(maya|Maya Lund)/})).toHaveTextContent('2 new messages · last from');
        expect(screen.getByRole('button', {name: /^#Release/})).toHaveTextContent('999+ new messages');
        expect(screen.getByRole('button', {name: /^Thread in #Release/})).toHaveTextContent('1 new reply');

        // The badges stay hidden while it's shown.
        expect(screen.getByTestId('badges')).toHaveTextContent('hidden');

        await userEvent.click(screen.getByRole('button', {name: /^Thread in #Release/}));
        expect(getHistory().push).toHaveBeenCalledWith('/lumen/channels/release');
        expect(selectPostById).toHaveBeenCalledWith('root');
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
        expect(screen.getByTestId('badges')).toHaveTextContent('shown');
    });

    test('"Mark all as read" marks it read up to the end of the focus, and is seen', async () => {
        render();
        ended();
        await screen.findByRole('dialog');
        await userEvent.click(screen.getByRole('button', {name: 'Mark all as read'}));

        await waitFor(() => expect(Client4.markFocusSummaryRead).toHaveBeenCalledWith(START, END));
        await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
        expect(mockSavePreferences).toHaveBeenCalledWith('me', [{user_id: 'me', category: 'focus', name: 'summary_seen', value: String(START)}]);
        expect(screen.getByTestId('badges')).toHaveTextContent('shown');
    });

    test('"Show unread badges" closes it and brings the badges back, nothing marked read', async () => {
        render();
        ended();
        await screen.findByRole('dialog');
        await userEvent.click(screen.getByRole('button', {name: 'Show unread badges'}));

        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
        expect(Client4.markFocusSummaryRead).not.toHaveBeenCalled();
        expect(mockSavePreferences).toHaveBeenCalledWith('me', [expect.objectContaining({name: 'summary_seen', value: String(START)})]);
        expect(screen.getByTestId('badges')).toHaveTextContent('shown');
    });

    test('closing it is "Show unread badges"', async () => {
        render();
        ended();
        await screen.findByRole('dialog');
        await userEvent.keyboard('{Escape}');

        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
        expect(Client4.markFocusSummaryRead).not.toHaveBeenCalled();
        expect(screen.getByTestId('badges')).toHaveTextContent('shown');
    });

    test('on loading, shows the summary of a focus that ended meanwhile, once', async () => {
        Client4.getFocus = jest.fn().mockResolvedValue({active: false, start_at: START, end_at: END, reason: '', ended_at: END});
        const {unmount} = render();
        expect(await screen.findByRole('dialog')).toBeInTheDocument();
        unmount();

        // Once seen (the preference), not again.
        jest.clearAllMocks();
        setFocusSummary(null);
        render(String(START));
        await waitFor(() => expect(Client4.getFocus).toHaveBeenCalled());
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
        expect(Client4.getFocusSummary).not.toHaveBeenCalled();
        expect(screen.getByTestId('badges')).toHaveTextContent('shown');
    });

    test('not seen again on this device once seen on another', async () => {
        render(String(START));
        ended();
        await waitFor(() => expect(screen.getByTestId('badges')).toHaveTextContent('shown'));
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
        expect(Client4.getFocusSummary).not.toHaveBeenCalled();
    });

    test('nothing missed: no summary, a word that it ended, and the badges back', async () => {
        Client4.getFocusSummary = jest.fn().mockResolvedValue({since: START, until: END, mention_count: 0, channels: [], threads: []});
        render();
        ended();
        await waitFor(() => expect(mockToast).toHaveBeenCalledWith('Focus ended: nothing new came in'));
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
        expect(screen.getByTestId('badges')).toHaveTextContent('shown');
    });
});
