// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {screen} from '@testing-library/react';
import React from 'react';
import {MemoryRouter, Route} from 'react-router-dom';

import {renderWithContext} from 'tests/react_testing_utils';

import ThreadPopout from './thread_popout';

// Remove usePost mock since we'll use Redux state instead

jest.mock('fusion/shell/fusion_app', () => ({
    FusionPopout: ({children}: {children: React.ReactNode}) => <div data-testid='fusion-popout'>{children}</div>,
}));

jest.mock('fusion/rhs/thread_panel', () => ({
    __esModule: true,
    default: ({rootId, inWindow}: {rootId: string; inWindow: boolean}) => (
        <div
            data-testid='thread-panel'
            data-root-id={rootId}
            data-in-window={String(inWindow)}
        >
            {'Thread'}
        </div>
    ),
}));

jest.mock('components/unreads_status_handler', () => ({
    __esModule: true,
    default: () => <div data-testid='unreads-status-handler'>{'Unreads Status Handler'}</div>,
}));

// Mock only the essential Redux dependencies
jest.mock('mattermost-redux/actions/channels', () => ({
    fetchChannelsAndMembers: jest.fn().mockReturnValue(() => ({type: 'FETCH_CHANNELS_AND_MEMBERS'})),
    selectChannel: jest.fn().mockReturnValue(() => ({type: 'SELECT_CHANNEL'})),
}));

jest.mock('mattermost-redux/actions/teams', () => ({
    selectTeam: jest.fn().mockReturnValue(() => ({type: 'SELECT_TEAM'})),
}));

jest.mock('mattermost-redux/actions/threads', () => ({
    getThread: jest.fn().mockReturnValue(() => ({type: 'GET_THREAD'})),
}));

jest.mock('mattermost-redux/actions/users', () => ({
    getProfiles: jest.fn().mockReturnValue(() => ({type: 'GET_PROFILES'})),
}));

const mockMarkThreadAsRead = jest.fn().mockReturnValue(() => ({type: 'MARK_THREAD_AS_READ'}));
jest.mock('actions/views/threads', () => ({
    markThreadAsRead: jest.fn((...args) => mockMarkThreadAsRead(...args)),
}));

jest.mock('utils/popouts/popout_windows', () => ({
    isThreadPopoutWindow: jest.fn().mockReturnValue(true),
}));

describe('ThreadPopout', () => {
    const mockPost = {
        id: 'post-123',
        channel_id: 'channel-123',
        message: 'Test post',
    };

    const mockTeam = {
        id: 'team-123',
        name: 'test-team',
        display_name: 'Test Team',
    };

    const baseState = {
        entities: {
            channels: {
                currentChannelId: 'channel-123',
                channels: {
                    'channel-123': {
                        id: 'channel-123',
                        name: 'test-channel',
                        display_name: 'Test Channel',
                        type: 'O' as const,
                        delete_at: 0,
                    },
                },
            },
            teams: {
                currentTeamId: 'team-123',
                teams: {
                    'team-123': mockTeam,
                },
            },
            users: {
                currentUserId: 'user-123',
            },
            posts: {
                posts: {
                    'post-123': mockPost,
                },
            },
            threads: {
                threads: {
                    'post-123': {
                        id: 'post-123',
                        last_reply_at: 1000,
                    },
                },
            },
        },
        views: {
            threads: {
                manuallyUnread: {},
            },
        },
    };

    beforeEach(() => {
        Object.defineProperty(window, 'isActive', {
            writable: true,
            value: false,
        });
        jest.spyOn(window, 'addEventListener').mockImplementation();
        jest.spyOn(window, 'removeEventListener').mockImplementation();
    });

    afterEach(() => {
        jest.restoreAllMocks();
    });

    it('should render nothing when thread is not available', () => {
        const stateWithoutPost = {
            ...baseState,
            entities: {
                ...baseState.entities,
                posts: {
                    posts: {},
                },
            },
        };

        const {container} = renderWithContext(
            <MemoryRouter initialEntries={['/_popout/thread/test-team/post-123']}>
                <ThreadPopout/>
            </MemoryRouter>,
            stateWithoutPost,
        );

        expect(container.firstChild).toBeNull();
    });

    it('should show the thread in the Fusion UI\'s frame, as a window', () => {
        renderWithContext(
            <MemoryRouter initialEntries={['/_popout/thread/test-team/post-123']}>
                <Route
                    path='/_popout/thread/:team/:postId'
                    component={ThreadPopout}
                />
            </MemoryRouter>,
            baseState,
        );

        const panel = screen.getByTestId('thread-panel');
        expect(screen.getByTestId('fusion-popout')).toContainElement(panel);
        expect(panel).toHaveAttribute('data-root-id', 'post-123');
        expect(panel).toHaveAttribute('data-in-window', 'true');
    });

    it('should handle missing post gracefully', () => {
        const stateWithoutPost = {
            ...baseState,
            entities: {
                ...baseState.entities,
                posts: {
                    posts: {},
                },
            },
        };

        const {container} = renderWithContext(
            <MemoryRouter initialEntries={['/_popout/thread/test-team/post-123']}>
                <ThreadPopout/>
            </MemoryRouter>,
            stateWithoutPost,
        );

        expect(container.firstChild).toBeNull();
    });

    describe('window focus and blur events', () => {
        it('should set window.isActive to true and call markThreadAsRead on focus', () => {
            let focusHandler: (() => void) | undefined;
            (window.addEventListener as jest.Mock).mockImplementation((event, handler) => {
                if (event === 'focus') {
                    focusHandler = handler;
                }
            });

            renderWithContext(
                <MemoryRouter initialEntries={['/_popout/thread/test-team/post-123']}>
                    <Route
                        path='/_popout/thread/:team/:postId'
                        component={ThreadPopout}
                    />
                </MemoryRouter>,
                baseState,
            );

            expect(screen.getByTestId('thread-panel')).toBeInTheDocument();
            expect(focusHandler).toBeDefined();

            if (focusHandler) {
                Object.defineProperty(window, 'isActive', {
                    writable: true,
                    value: false,
                });
                focusHandler();
            }

            expect((window as Window & {isActive?: boolean}).isActive).toBe(true);
            expect(mockMarkThreadAsRead).toHaveBeenCalledWith('post-123');
        });

        it('should set window.isActive to false on blur', () => {
            let blurHandler: (() => void) | undefined;
            (window.addEventListener as jest.Mock).mockImplementation((event, handler) => {
                if (event === 'blur') {
                    blurHandler = handler;
                }
            });

            renderWithContext(
                <MemoryRouter initialEntries={['/_popout/thread/test-team/post-123']}>
                    <Route
                        path='/_popout/thread/:team/:postId'
                        component={ThreadPopout}
                    />
                </MemoryRouter>,
                baseState,
            );

            expect(screen.getByTestId('thread-panel')).toBeInTheDocument();
            expect(blurHandler).toBeDefined();

            if (blurHandler) {
                Object.defineProperty(window, 'isActive', {
                    writable: true,
                    value: true,
                });
                blurHandler();
            }

            expect((window as Window & {isActive?: boolean}).isActive).toBe(false);
        });
    });
});
