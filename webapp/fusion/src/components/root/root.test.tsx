// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {createMemoryHistory} from 'history';
import React from 'react';
import type {RouteComponentProps} from 'react-router-dom';
import {bindActionCreators} from 'redux';

import * as GlobalActions from 'actions/global_actions';

import testConfigureStore from 'packages/mattermost-redux/test/test_store';
import {initializePlugins} from 'plugins';
import {renderWithContext, waitFor} from 'tests/react_testing_utils';
import * as BrowserUtils from 'utils/browser_utils';
import {StoragePrefixes} from 'utils/constants';

import {handleLoginLogoutSignal, redirectToOnboardingOrDefaultTeam} from './actions';
import type {Props} from './root';
import Root, {doesRouteBelongToTeamControllerRoutes} from './root';

jest.mock('actions/telemetry_actions');

jest.mock('components/announcement_bar', () => () => <div/>);
jest.mock('components/team_sidebar', () => () => <div/>);
jest.mock('components/mobile_view_watcher', () => () => <div/>);
jest.mock('./performance_reporter_controller', () => () => <div/>);

// The logged-in pages aren't under test here: a route that only says where it is.
jest.mock('components/logged_in_route', () => ({path}: {path: string}) => <div data-testid={`logged-in-route ${path}`}/>);

// A logged-in page shows its content (the app link redirect, a product's page) without loading the user.
jest.mock('components/logged_in', () => ({children}: {children: React.ReactNode}) => <>{children}</>);
jest.mock('components/product_pluggable', () => () => <div data-testid='product-page'/>);

jest.mock('utils/utils', () => ({
    applyTheme: jest.fn(),
    makeIsEligibleForClick: jest.fn(),
}));

jest.mock('utils/browser_utils', () => ({
    reloadPage: jest.fn(),
}));

jest.mock('actions/global_actions', () => ({
    redirectUserToDefaultTeam: jest.fn(),
}));

jest.mock('plugins', () => ({
    ...jest.requireActual('plugins'),
    initializePlugins: jest.fn(() => Promise.resolve()),
}));

jest.mock('mattermost-redux/actions/general', () => ({
    ...jest.requireActual('mattermost-redux/actions/general'),
    setUrl: () => {},
}));

describe('components/Root', () => {
    const store = testConfigureStore();

    const baseProps: Props = {
        isConfigLoaded: true,
        telemetryEnabled: true,
        noAccounts: false,
        telemetryId: '1234ab',
        serviceEnvironment: undefined,
        siteURL: 'http://localhost:8065',
        showTermsOfService: false,
        plugins: [],
        products: [],
        showLaunchingWorkspace: false,
        rhsIsExpanded: false,
        rhsIsOpen: false,
        rhsState: null,
        shouldShowAppBar: false,
        actions: {
            loadConfigAndMe: jest.fn().mockImplementation(() => {
                return Promise.resolve({
                    isLoaded: true,
                    isMeRequested: false,
                });
            }),
            loadRecentlyUsedCustomEmojis: jest.fn(),
            migrateRecentEmojis: jest.fn(),
            initializeProducts: jest.fn(),
            ...bindActionCreators({
                handleLoginLogoutSignal,
                redirectToOnboardingOrDefaultTeam,
            }, store.dispatch),
        },
        dispatch: store.dispatch,
        permalinkRedirectTeamName: 'myTeam',
        currentUserId: '',
        ...{
            location: {
                pathname: '/',
            },
            history: {
                push: jest.fn(),
            } as unknown as RouteComponentProps['history'],
        } as RouteComponentProps,
        isDevModeEnabled: false,
    };

    let originalMatchMedia: (query: string) => MediaQueryList;

    beforeAll(() => {
        originalMatchMedia = window.matchMedia;

        Object.defineProperty(window, 'matchMedia', {
            writable: true,
            value: jest.fn().mockImplementation((query) => ({
                matches: false,
                media: query,
            })),
        });
    });

    afterEach(() => {
        jest.restoreAllMocks();

        // Reset the reloadPage mock after each test
        (BrowserUtils.reloadPage as jest.Mock).mockClear();
        (initializePlugins as jest.Mock).mockClear();
    });

    afterAll(() => {
        window.matchMedia = originalMatchMedia;
    });

    test('should load config and license on mount and redirect to sign-up page', async () => {
        const props = {
            ...baseProps,
            noAccounts: true,
        };

        renderWithContext(<Root {...props}/>);

        await waitFor(() => {
            expect(props.history.push).toHaveBeenCalledWith('/signup_user_complete');
        });
    });

    test('should load user, config, and license on mount and redirect to defaultTeam on success', async () => {
        document.cookie = 'MMUSERID=userid';
        localStorage.setItem('was_logged_in', 'true');

        const props = {
            ...baseProps,
            actions: {
                ...baseProps.actions,
                loadConfigAndMe: jest.fn().mockImplementation(() => {
                    return Promise.resolve({
                        isLoaded: true,
                        isMeRequested: true,
                    });
                }),
            },
        };

        renderWithContext(<Root {...props}/>);

        await waitFor(() => {
            expect(props.actions.loadConfigAndMe).toHaveBeenCalledTimes(1);
            expect(GlobalActions.redirectUserToDefaultTeam).toHaveBeenCalledTimes(1);
        });
    });

    test('should load user, config, and license on mount and should not redirect to defaultTeam id pathname is not root', async () => {
        document.cookie = 'MMUSERID=userid';
        localStorage.setItem('was_logged_in', 'true');

        const props = {
            ...baseProps,
            location: {
                pathname: '/admin_console',
            } as RouteComponentProps['location'],
            actions: {
                ...baseProps.actions,
                loadConfigAndMe: jest.fn().mockImplementation(() => {
                    return Promise.resolve({
                        isLoaded: true,
                        isMeRequested: true,
                    });
                }),
            },
        };

        renderWithContext(<Root {...props}/>);

        await waitFor(() => {
            expect(props.actions.loadConfigAndMe).toHaveBeenCalledTimes(1);
            expect(GlobalActions.redirectUserToDefaultTeam).not.toHaveBeenCalled();
        });
    });

    test('should call history on props change', async () => {
        const props = {
            ...baseProps,
            noAccounts: false,
            history: {
                push: jest.fn(),
            } as unknown as RouteComponentProps['history'],
        };

        const {rerender} = renderWithContext(<Root {...props}/>);

        expect(props.history.push).not.toHaveBeenCalled();

        const props2 = {
            ...props,
            noAccounts: true,
        };

        rerender(<Root {...props2}/>);

        await waitFor(() => {
            expect(props.history.push).toHaveBeenLastCalledWith('/signup_user_complete');
        });
    });

    // Antimatter: the Kanban plugin (once Boards) had its full page at /kanban, and left it for the side panel: its old
    // /kanban and /boards links keep their whole path and query, and open in the last community, where the plugin opens
    // the panel on the page they point to.
    test.each([
        ['/boards', '/myTeam/focalboard', ''],
        ['/boards/team/lumen/b1', '/myTeam/focalboard/team/lumen/b1', ''],
        ['/boards/team/lumen/b1/v2/c3', '/myTeam/focalboard/team/lumen/b1/v2/c3', '?cardId=c3'],
        ['/kanban/team/lumen/b1/v2/c3', '/myTeam/focalboard/team/lumen/b1/v2/c3', '?r=token'],
        ['/kanban', '/myTeam/focalboard', ''],
    ])('redirects %s to the Kanban plugin', async (from, to, search) => {
        const history = createMemoryHistory({initialEntries: [from + search]});
        const props = {
            ...baseProps,
            location: {pathname: from} as RouteComponentProps['location'],
        };

        renderWithContext(<Root {...props}/>, {}, {history});

        await waitFor(() => {
            expect(history.location.pathname).toBe(to);
        });
        expect(history.location.search).toBe(search);
    });

    test('leaves paths that only start like /boards alone', async () => {
        const history = createMemoryHistory({initialEntries: ['/boardsroom/channels/general']});
        const props = {
            ...baseProps,
            location: {pathname: '/boardsroom/channels/general'} as RouteComponentProps['location'],
        };

        renderWithContext(<Root {...props}/>, {}, {history});

        await waitFor(() => {
            expect(props.actions.loadConfigAndMe).toHaveBeenCalled();
        });
        expect(history.location.pathname).toBe('/boardsroom/channels/general');
    });

    // Antimatter: Playbooks left its full page for the side panel; its old links open in the last community, where
    // the app opens the panel on the page they point to.
    test.each([
        ['/playbooks/runs/abcdefghijklmnopqrstuvwxyz', '/myTeam/playbooks/runs/abcdefghijklmnopqrstuvwxyz', '?from=dm_assignedtask'],
        ['/playbooks/playbooks/abcdefghijklmnopqrstuvwxyz', '/myTeam/playbooks/playbooks/abcdefghijklmnopqrstuvwxyz', ''],
    ])('sends the old app link %s to the community', async (from, to, search) => {
        const history = createMemoryHistory({initialEntries: [from + search]});
        const props = {
            ...baseProps,
            location: {pathname: from} as RouteComponentProps['location'],
        };

        renderWithContext(<Root {...props}/>, {}, {history});

        await waitFor(() => {
            expect(history.location.pathname).toBe(to);
        });
        expect(history.location.search).toBe(search);
    });

    test('leaves the old app links to an app that still has its full page there', async () => {
        const history = createMemoryHistory({initialEntries: ['/playbooks/runs/abcdefghijklmnopqrstuvwxyz']});
        const props = {
            ...baseProps,
            products: [{id: 'playbooks', pluginId: 'playbooks', baseURL: '/playbooks'}] as unknown as Props['products'],
            location: {pathname: '/playbooks/runs/abcdefghijklmnopqrstuvwxyz'} as RouteComponentProps['location'],
        };

        const {findByTestId} = renderWithContext(<Root {...props}/>, {}, {history});

        await findByTestId('product-page');
        expect(history.location.pathname).toBe('/playbooks/runs/abcdefghijklmnopqrstuvwxyz');
    });

    test('should not load plugins before sign-in', async () => {
        const props = {
            ...baseProps,
            location: {pathname: '/login'} as RouteComponentProps['location'],
        };

        renderWithContext(<Root {...props}/>);

        await waitFor(() => {
            expect(props.actions.initializeProducts).toHaveBeenCalled();
        });
        expect(initializePlugins).not.toHaveBeenCalled();
    });

    test('should load plugins once the user signs in', async () => {
        const props = {
            ...baseProps,
            actions: {...baseProps.actions, initializeProducts: jest.fn()},
            location: {pathname: '/login'} as RouteComponentProps['location'],
        };

        const {rerender} = renderWithContext(<Root {...props}/>);
        await waitFor(() => {
            expect(props.actions.initializeProducts).toHaveBeenCalled();
        });

        rerender(<Root {...{...props, currentUserId: 'user1'}}/>);
        rerender(<Root {...{...props, currentUserId: 'user1', noAccounts: true}}/>);

        await waitFor(() => {
            expect(initializePlugins).toHaveBeenCalledTimes(1);
        });
    });

    test('should load plugins for a signed-in user or a public plugin page', async () => {
        for (const props of [
            {...baseProps, currentUserId: 'user1'},
            {...baseProps, location: {pathname: '/boards/public/abc'} as RouteComponentProps['location']},
            {...baseProps, location: {pathname: '/plug/oauth'} as RouteComponentProps['location']},
        ]) {
            (initializePlugins as jest.Mock).mockClear();
            const {unmount} = renderWithContext(<Root {...props}/>);

            // eslint-disable-next-line no-await-in-loop
            await waitFor(() => {
                expect(initializePlugins).toHaveBeenCalledTimes(1);
            });
            unmount();
        }
    });

    test('should reload on focus after getting signal login event from another tab', async () => {
        renderWithContext(<Root {...baseProps}/>);

        const loginSignal = new StorageEvent('storage', {
            key: StoragePrefixes.LOGIN,
            newValue: String(Math.random()),
            storageArea: localStorage,
        });

        window.dispatchEvent(loginSignal);
        window.dispatchEvent(new Event('focus'));

        await waitFor(() => {
            expect(BrowserUtils.reloadPage).toHaveBeenCalledTimes(1);
        });
    });
});

describe('doesRouteBelongToTeamControllerRoutes', () => {
    test('should return true for some of team_controller routes', () => {
        expect(doesRouteBelongToTeamControllerRoutes('/team_name_example_1/messages/abc')).toBe(true);
        expect(doesRouteBelongToTeamControllerRoutes('/team_name_example_1/messages')).toBe(true);
        expect(doesRouteBelongToTeamControllerRoutes('/team_name_example_1/channels/cde')).toBe(true);
        expect(doesRouteBelongToTeamControllerRoutes('/team_name_example_1/channels')).toBe(true);
        expect(doesRouteBelongToTeamControllerRoutes('/team_name_example_1/threads/efg')).toBe(true);
        expect(doesRouteBelongToTeamControllerRoutes('/team_name_example_1/threads')).toBe(true);
        expect(doesRouteBelongToTeamControllerRoutes('/team_name_example_1/drafts')).toBe(true);

        // F125: the Unreads view (fusion/unreads/unreads_view.tsx) is a team controller route too, so Root draws
        // its own Fusion frame (am-root-main) around it instead of the classic page grid (main-wrapper) -
        // otherwise .am-app loses the flex:1 that stretches it to the window's width, and the view renders with
        // no width at all on a cold load straight to /:team/unreads.
        expect(doesRouteBelongToTeamControllerRoutes('/team_name_example_1/unreads/abc')).toBe(true);
        expect(doesRouteBelongToTeamControllerRoutes('/team_name_example_1/unreads')).toBe(true);
        expect(doesRouteBelongToTeamControllerRoutes('/team_name_example_1/integrations/klm')).toBe(true);
        expect(doesRouteBelongToTeamControllerRoutes('/team_name_example_1/emoji/nop')).toBe(true);
        expect(doesRouteBelongToTeamControllerRoutes('/team_name_example_1/integrations')).toBe(true);
        expect(doesRouteBelongToTeamControllerRoutes('/team_name_example_1/emoji')).toBe(true);
    });

    test('should return false for other of team_controller routes', () => {
        expect(doesRouteBelongToTeamControllerRoutes('/team_name_example_2')).toBe(false);
        expect(doesRouteBelongToTeamControllerRoutes('/team_name_example_2/pl/permalink123')).toBe(false);
        expect(doesRouteBelongToTeamControllerRoutes('/team_name_example_2/needs_team_component_plugin')).toBe(false);
    });

    test('should return false for other routes of root', () => {
        expect(doesRouteBelongToTeamControllerRoutes('/plug/custom_route_component')).toBe(false);
        expect(doesRouteBelongToTeamControllerRoutes('/main_component_product_1')).toBe(false);
        expect(doesRouteBelongToTeamControllerRoutes('/product_1/public')).toBe(false);
        expect(doesRouteBelongToTeamControllerRoutes('/_redirect/pl/message_1')).toBe(false);
        expect(doesRouteBelongToTeamControllerRoutes('/preparing-workspace')).toBe(false);
        expect(doesRouteBelongToTeamControllerRoutes('/mfa')).toBe(false);
        expect(doesRouteBelongToTeamControllerRoutes('/create_team')).toBe(false);
        expect(doesRouteBelongToTeamControllerRoutes('/oauth/authorize')).toBe(false);
        expect(doesRouteBelongToTeamControllerRoutes('/select_team')).toBe(false);
        expect(doesRouteBelongToTeamControllerRoutes('/admin_console')).toBe(false);
        expect(doesRouteBelongToTeamControllerRoutes('/landing')).toBe(false);
        expect(doesRouteBelongToTeamControllerRoutes('/terms_of_service')).toBe(false);
        expect(doesRouteBelongToTeamControllerRoutes('/claim')).toBe(false);
        expect(doesRouteBelongToTeamControllerRoutes('/do_verify_email')).toBe(false);
        expect(doesRouteBelongToTeamControllerRoutes('/should_verify_email')).toBe(false);
        expect(doesRouteBelongToTeamControllerRoutes('/signup_user_complete')).toBe(false);
        expect(doesRouteBelongToTeamControllerRoutes('/reset_password_complete')).toBe(false);
        expect(doesRouteBelongToTeamControllerRoutes('/reset_password')).toBe(false);
        expect(doesRouteBelongToTeamControllerRoutes('/access_problem')).toBe(false);
        expect(doesRouteBelongToTeamControllerRoutes('/login')).toBe(false);
        expect(doesRouteBelongToTeamControllerRoutes('/error')).toBe(false);
    });

    test('should return false for admin_console routes', () => {
        expect(doesRouteBelongToTeamControllerRoutes('/admin_console/about')).toBe(false);
        expect(doesRouteBelongToTeamControllerRoutes('/admin_console/reporting')).toBe(false);
        expect(doesRouteBelongToTeamControllerRoutes('/admin_console/user_management')).toBe(false);
        expect(doesRouteBelongToTeamControllerRoutes('/admin_console/environment')).toBe(false);
        expect(doesRouteBelongToTeamControllerRoutes('/admin_console/site_config')).toBe(false);
        expect(doesRouteBelongToTeamControllerRoutes('/admin_console/plugins/')).toBe(false);
        expect(doesRouteBelongToTeamControllerRoutes('/admin_console/integrations/')).toBe(false);
        expect(doesRouteBelongToTeamControllerRoutes('/admin_console/integrations/bot_accounts')).toBe(false);
        expect(doesRouteBelongToTeamControllerRoutes('/admin_console/compliance')).toBe(false);
        expect(doesRouteBelongToTeamControllerRoutes('/admin_console/experimental')).toBe(false);
    });
});
