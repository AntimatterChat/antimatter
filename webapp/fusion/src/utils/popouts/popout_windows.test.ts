// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {isDesktopApp} from '@mattermost/shared/utils/user_agent';

import {readPanelState} from 'plugins/rhs_plugin/panel_state';
import DesktopApp from 'utils/desktop_api';
import {getBasePath} from 'utils/url';

import {POPOUT_FOCUSED, POPOUT_BLURRED, getFocusedPopoutInfo} from './focus';
import {FOCUS_REPLY_POST, popoutChannel, popoutRhsPlugin, popoutRhsSearch, popoutThread} from './popout_windows';

jest.mock('utils/desktop_api', () => ({
    __esModule: true,
    default: {
        setupDesktopPopout: jest.fn(),
        sendToParentWindow: jest.fn(),
        onMessageFromParentWindow: jest.fn(),
    },
}));

jest.mock('@mattermost/shared/utils/user_agent', () => ({
    isDesktopApp: jest.fn(),
}));

jest.mock('utils/url', () => ({
    getBasePath: jest.fn(),
}));

jest.mock('./browser_popouts', () => {
    const mockFn = jest.fn();
    (globalThis as typeof globalThis & {mockSetupBrowserPopout: typeof mockFn}).mockSetupBrowserPopout = mockFn;
    return {
        __esModule: true,
        default: {
            setupBrowserPopout: mockFn,
        },
    };
});

const mockDesktopApp = jest.mocked(DesktopApp);

const getMockSetupBrowserPopout = (): jest.Mock => (globalThis as typeof globalThis & {mockSetupBrowserPopout: jest.Mock}).mockSetupBrowserPopout;

const mockListeners = {
    sendToPopout: jest.fn(),
    onMessageFromPopout: jest.fn(),
    onClosePopout: jest.fn(),
};

function setupDesktop() {
    jest.mocked(isDesktopApp).mockReturnValue(true);
    mockDesktopApp.setupDesktopPopout.mockResolvedValue(mockListeners);
}

function setupBrowser(basePath = '') {
    jest.mocked(isDesktopApp).mockReturnValue(false);
    jest.mocked(getBasePath).mockReturnValue(basePath);
    getMockSetupBrowserPopout().mockReturnValue(mockListeners);
}

describe('popout_windows', () => {
    beforeEach(() => {
        getMockSetupBrowserPopout().mockClear();
        jest.mocked(getBasePath).mockReturnValue('');
    });

    describe('popoutThread', () => {
        const mockOnFocusPost = jest.fn();

        beforeEach(() => {
            mockOnFocusPost.mockClear();
        });

        it('should call popout with correct path and props for desktop app', async () => {
            setupDesktop();

            await popoutThread('Thread - {channelName} - {teamName} - {serverName}', 'thread-123', 'test-team', mockOnFocusPost);

            expect(mockDesktopApp.setupDesktopPopout).toHaveBeenCalledWith(
                '/_popout/thread/test-team/thread-123',
                {
                    isRHS: true,
                    titleTemplate: 'Thread - {channelName} - {teamName} - {serverName}',
                },
            );
        });

        it('should call popout with correct path and props for browser popout', async () => {
            setupBrowser();

            await popoutThread('Thread - {channelName} - {teamName} - {serverName}', 'thread-123', 'test-team', mockOnFocusPost);

            expect(getMockSetupBrowserPopout()).toHaveBeenCalledWith(
                '/_popout/thread/test-team/thread-123',
            );
        });

        it('should register focus and FOCUS_REPLY_POST listeners', async () => {
            setupBrowser();
            const messageListeners: Array<(channel: string, ...args: unknown[]) => void> = [];
            getMockSetupBrowserPopout().mockReturnValue({
                ...mockListeners,
                onMessageFromPopout: (listener: (channel: string, ...args: unknown[]) => void) => {
                    messageListeners.push(listener);
                },
                onClosePopout: jest.fn(),
            });

            await popoutThread('Thread - {channelName} - {teamName} - {serverName}', 'thread-123', 'test-team', mockOnFocusPost);

            expect(messageListeners).toHaveLength(2);

            messageListeners.forEach((listener) => listener(POPOUT_FOCUSED, 'channel-abc', 'thread-123'));
            expect(getFocusedPopoutInfo()).toEqual({channelId: 'channel-abc', threadId: 'thread-123'});

            messageListeners.forEach((listener) => listener(FOCUS_REPLY_POST, 'post-123', '/team/pl/post-123'));
            expect(mockOnFocusPost).toHaveBeenCalledWith('post-123', '/team/pl/post-123');

            messageListeners.forEach((listener) => listener(POPOUT_BLURRED));
            expect(getFocusedPopoutInfo()).toBeNull();
        });

        it('should include subpath in popout URL when basename is set', async () => {
            setupBrowser('/company/mattermost');

            await popoutThread('Thread - {channelName} - {teamName} - {serverName}', 'thread-123', 'test-team', mockOnFocusPost);

            expect(getMockSetupBrowserPopout()).toHaveBeenCalledWith(
                '/company/mattermost/_popout/thread/test-team/thread-123',
            );
        });
    });

    describe('popoutRhsPlugin', () => {
        it('should call popout with correct path and props for desktop app', async () => {
            setupDesktop();

            await popoutRhsPlugin('{pluginDisplayName} - {serverName}', 'test-plugin-id', 'test-team', 'test-channel');

            expect(mockDesktopApp.setupDesktopPopout).toHaveBeenCalledWith(
                '/_popout/rhs/test-team/plugin/test-plugin-id?channel=test-channel',
                {
                    isRHS: true,
                    titleTemplate: '{pluginDisplayName} - {serverName}',
                },
            );
        });

        it('should call popout with correct path and props for browser popout', async () => {
            setupBrowser();

            await popoutRhsPlugin('{pluginDisplayName} - {serverName}', 'test-plugin-id', 'test-team', 'test-channel');

            expect(getMockSetupBrowserPopout()).toHaveBeenCalledWith(
                '/_popout/rhs/test-team/plugin/test-plugin-id?channel=test-channel',
            );
        });

        it('should return popout listeners', async () => {
            setupDesktop();

            const result = await popoutRhsPlugin('{pluginDisplayName} - {serverName}', 'test-plugin-id', 'test-team', 'test-channel');

            expect(result).toEqual(mockListeners);
        });

        it('should include subpath in popout URL when basename is set', async () => {
            setupBrowser('/company/mattermost');

            await popoutRhsPlugin('{pluginDisplayName} - {serverName}', 'test-plugin-id', 'test-team', 'test-channel');

            expect(getMockSetupBrowserPopout()).toHaveBeenCalledWith(
                '/company/mattermost/_popout/rhs/test-team/plugin/test-plugin-id?channel=test-channel',
            );
        });

        it('puts the panel state in the popout URL, escaped, and it reads back the same', async () => {
            setupBrowser();

            await popoutRhsPlugin('{pluginDisplayName} - {serverName}', 'test-plugin-id', 'test-team', 'test-channel', 'note/n1&channel=other#x');

            const path = getMockSetupBrowserPopout().mock.calls[0][0] as string;
            expect(path).toBe('/_popout/rhs/test-team/plugin/test-plugin-id?channel=test-channel&state=note%2Fn1%26channel%3Dother%23x');

            const search = path.slice(path.indexOf('?'));
            expect(readPanelState(search)).toBe('note/n1&channel=other#x');
            expect(new URLSearchParams(search).getAll('channel')).toEqual(['test-channel']);
        });

        it('puts the panel state in the desktop popout URL too', async () => {
            setupDesktop();

            await popoutRhsPlugin('{pluginDisplayName} - {serverName}', 'test-plugin-id', 'test-team', 'test-channel', 'run/r1');

            expect(mockDesktopApp.setupDesktopPopout).toHaveBeenCalledWith(
                '/_popout/rhs/test-team/plugin/test-plugin-id?channel=test-channel&state=run%2Fr1',
                {
                    isRHS: true,
                    titleTemplate: '{pluginDisplayName} - {serverName}',
                },
            );
        });

        it('has no state parameter when the panel never set one, or is on its home', async () => {
            setupBrowser();

            await popoutRhsPlugin('{pluginDisplayName} - {serverName}', 'test-plugin-id', 'test-team', 'test-channel', undefined);
            await popoutRhsPlugin('{pluginDisplayName} - {serverName}', 'test-plugin-id', 'test-team', 'test-channel', '');
            await popoutRhsPlugin('{pluginDisplayName} - {serverName}', 'test-plugin-id', 'test-team', undefined, undefined);

            expect(getMockSetupBrowserPopout().mock.calls.map((call) => call[0])).toEqual([
                '/_popout/rhs/test-team/plugin/test-plugin-id?channel=test-channel',
                '/_popout/rhs/test-team/plugin/test-plugin-id?channel=test-channel',
                '/_popout/rhs/test-team/plugin/test-plugin-id',
            ]);
        });
    });

    describe('popoutChannel', () => {
        it('should include the path segment in the URL for regular channels', async () => {
            setupBrowser();

            await popoutChannel('Title', 'test-team', 'channels', 'general');

            expect(getMockSetupBrowserPopout()).toHaveBeenCalledWith(
                '/_popout/channel/test-team/channels/general',
            );
        });

        it('should include the messages path for DM/GM channels', async () => {
            setupBrowser();

            await popoutChannel('Title', 'test-team', 'messages', '@otheruser');

            expect(getMockSetupBrowserPopout()).toHaveBeenCalledWith(
                '/_popout/channel/test-team/messages/@otheruser',
            );
        });

        it('should include subpath in popout URL when basename is set', async () => {
            setupBrowser('/company/mattermost');

            await popoutChannel('Title', 'test-team', 'channels', 'general');

            expect(getMockSetupBrowserPopout()).toHaveBeenCalledWith(
                '/company/mattermost/_popout/channel/test-team/channels/general',
            );
        });

        it('should call desktop popout with correct path for desktop app', async () => {
            setupDesktop();

            await popoutChannel('Channel Title', 'test-team', 'channels', 'general');

            expect(mockDesktopApp.setupDesktopPopout).toHaveBeenCalledWith(
                '/_popout/channel/test-team/channels/general',
                {titleTemplate: 'Channel Title'},
            );
        });
    });

    describe('popoutRhsSearch', () => {
        async function callDesktop(...args: Parameters<typeof popoutRhsSearch>) {
            setupDesktop();
            const result = await popoutRhsSearch(...args);
            return result;
        }

        function expectDesktopPath(substring: string) {
            expect(mockDesktopApp.setupDesktopPopout).toHaveBeenCalledWith(
                expect.stringContaining(substring),
                expect.objectContaining({isRHS: true}),
            );
        }

        it('should call popout with correct path and query params for desktop app', async () => {
            await callDesktop('Search Results - {serverName}', 'test-team', 'hello world', 'search', 'messages');

            expectDesktopPath('/_popout/rhs/test-team/search');
            expectDesktopPath('q=hello+world');
            expectDesktopPath('type=messages');
            expectDesktopPath('mode=search');
        });

        it('should include channel param when provided', async () => {
            await callDesktop('Pinned Messages - {serverName}', 'test-team', '', 'pin', 'messages', 'general');
            expectDesktopPath('channel=general');
        });

        it('should include searchTeamId when provided', async () => {
            await callDesktop('Search Results - {serverName}', 'test-team', 'test', 'search', 'messages', undefined, 'team-123');
            expectDesktopPath('searchTeamId=team-123');
        });

        it('should include searchTeamId with empty string for All Teams', async () => {
            await callDesktop('Search Results - {serverName}', 'test-team', 'test', 'search', 'messages', undefined, '');
            expectDesktopPath('searchTeamId=');
        });

        it('should call browser popout when not desktop app', async () => {
            setupBrowser();

            await popoutRhsSearch('Search Results - {serverName}', 'test-team', 'hello', 'search', 'messages');

            expect(getMockSetupBrowserPopout()).toHaveBeenCalledWith(
                expect.stringContaining('/_popout/rhs/test-team/search'),
            );
            expect(getMockSetupBrowserPopout()).toHaveBeenCalledWith(
                expect.stringContaining('q=hello'),
            );
        });

        it('should include subpath when basename is set', async () => {
            setupBrowser('/company/mattermost');

            await popoutRhsSearch('Search Results - {serverName}', 'test-team', 'hello', 'search', 'messages');

            expect(getMockSetupBrowserPopout()).toHaveBeenCalledWith(
                expect.stringContaining('/company/mattermost/_popout/rhs/test-team/search'),
            );
        });

        it('should return popout listeners', async () => {
            const result = await callDesktop('Search Results - {serverName}', 'test-team', 'test', 'search', 'messages');
            expect(result).toEqual(mockListeners);
        });
    });
});
