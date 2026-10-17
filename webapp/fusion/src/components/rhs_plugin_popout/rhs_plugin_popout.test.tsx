// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {MemoryRouter, Route} from 'react-router-dom';

import {act, renderWithContext, screen} from 'tests/react_testing_utils';

import RhsPluginPopout from './rhs_plugin_popout';

jest.mock('components/loading_screen', () => ({
    __esModule: true,
    default: () => <div data-testid='loading-screen'>{'Loading Screen'}</div>,
}));

jest.mock('components/search_results_header', () => ({
    __esModule: true,
    default: ({children}: {children: React.ReactNode}) => (
        <div data-testid='search-results-header'>{children}</div>
    ),
}));

type MockPanelProps = {pluggableName: string; pluggableId: string; initialPanelState?: string; setPanelState?: (state: string | null) => void};
const mockPanelProps: {current?: MockPanelProps} = {};
jest.mock('plugins/pluggable', () => ({
    __esModule: true,
    default: (props: MockPanelProps) => {
        mockPanelProps.current = props;
        return (
            <div data-testid={`pluggable-${props.pluggableName}-${props.pluggableId}`}>
                {`Pluggable: ${props.pluggableName} - ${props.pluggableId}`}
            </div>
        );
    },
}));

const mockUseParams = jest.spyOn(require('react-router-dom'), 'useParams');
const pluginId = 'test-plugin';
const pluggableId = 'pluggable-123';
const pluginTitle = 'Test Plugin Title';
const baseState = {
    entities: {
        general: {
            config: {
                SiteName: 'Test Server',
            },
        },
        channels: {
            channels: {},
        },
        teams: {
            teams: {},
        },
    },
    plugins: {
        plugins: {
            [pluginId]: {
                name: 'Test Plugin',
            },
        },
        components: {
            RightHandSidebarComponent: [{
                pluginId,
                id: pluggableId,
                title: pluginTitle,
            }],
        },
    },
};

describe('RhsPluginPopout', () => {
    beforeEach(() => {
        mockUseParams.mockReturnValue({pluginId});
    });

    it('should render LoadingScreen when plugin is not found', () => {
        mockUseParams.mockReturnValue({pluginId: 'non-existent-plugin'});

        renderWithContext(
            <MemoryRouter initialEntries={['/_popout/rhs/team1/channel1/plugin/non-existent-plugin']}>
                <Route
                    path='/_popout/rhs/:team/:identifier/plugin/:pluginId'
                    component={RhsPluginPopout}
                />
            </MemoryRouter>,
            {
                ...baseState,
                plugins: {
                    plugins: {},
                },
            },
        );

        expect(screen.getByTestId('loading-screen')).toBeInTheDocument();
        expect(screen.queryByTestId('search-results-header')).not.toBeInTheDocument();
        expect(screen.queryByTestId('pluggable-RightHandSidebarComponent-')).not.toBeInTheDocument();
    });

    it('should render SearchResultsHeader and Pluggable when plugin is found', () => {
        renderWithContext(
            <MemoryRouter initialEntries={['/_popout/rhs/team1/channel1/plugin/test-plugin']}>
                <Route
                    path='/_popout/rhs/:team/:identifier/plugin/:pluginId'
                    component={RhsPluginPopout}
                />
            </MemoryRouter>,
            baseState,
        );

        expect(screen.queryByTestId('loading-screen')).not.toBeInTheDocument();
        expect(screen.getByTestId('search-results-header')).toBeInTheDocument();
        expect(screen.getByText(pluginTitle)).toBeInTheDocument();
        expect(screen.getByTestId(`pluggable-RightHandSidebarComponent-${pluggableId}`)).toBeInTheDocument();

        const pluggable = screen.getByTestId(`pluggable-RightHandSidebarComponent-${pluggableId}`);
        expect(pluggable).toBeInTheDocument();
        expect(pluggable).toHaveTextContent(`Pluggable: RightHandSidebarComponent - ${pluggableId}`);
    });

    it('should handle empty title when plugin component has no title', () => {
        mockUseParams.mockReturnValue({pluginId});

        renderWithContext(
            <MemoryRouter initialEntries={['/_popout/rhs/team1/channel1/plugin/test-plugin']}>
                <Route
                    path='/_popout/rhs/:team/:identifier/plugin/:pluginId'
                    component={RhsPluginPopout}
                />
            </MemoryRouter>,
            {
                ...baseState,
                plugins: {
                    ...baseState.plugins,
                    components: {
                        ...baseState.plugins.components,
                        RightHandSidebarComponent: [{
                            ...baseState.plugins.components.RightHandSidebarComponent[0],
                            title: '',
                        }],
                    },
                },
            },
        );

        expect(screen.getByTestId('search-results-header')).toBeInTheDocument();
        expect(screen.getByTestId(`pluggable-RightHandSidebarComponent-${pluggableId}`)).toBeInTheDocument();
    });
    describe('panel state', () => {
        const renderAt = (entry: string) => renderWithContext(
            <MemoryRouter initialEntries={[entry]}>
                <Route
                    path='/_popout/rhs/:team/plugin/:pluginId'
                    component={RhsPluginPopout}
                />
            </MemoryRouter>,
            baseState,
        );

        beforeEach(() => {
            mockPanelProps.current = undefined;
            window.history.replaceState(null, '', '/_popout/rhs/team1/plugin/test-plugin?channel=general');
        });

        it('starts the app where its panel was: the state of the popout URL, unescaped', () => {
            renderAt('/_popout/rhs/team1/plugin/test-plugin?channel=general&state=note%2Fn1%26x%3Dy');

            expect(mockPanelProps.current?.initialPanelState).toBe('note/n1&x=y');
            expect(typeof mockPanelProps.current?.setPanelState).toBe('function');
        });

        it('starts the app on its home when the URL has no state, or one it would not keep', () => {
            renderAt('/_popout/rhs/team1/plugin/test-plugin?channel=general');
            expect(mockPanelProps.current?.initialPanelState).toBeUndefined();

            renderAt(`/_popout/rhs/team1/plugin/test-plugin?state=${'a'.repeat(600)}`);
            expect(mockPanelProps.current?.initialPanelState).toBeUndefined();
        });

        it("follows the app in the window's address, keeping the channel", () => {
            renderAt('/_popout/rhs/team1/plugin/test-plugin?channel=general&state=note%2Fn1');

            act(() => mockPanelProps.current?.setPanelState?.('note/n2'));
            expect(window.location.pathname).toBe('/_popout/rhs/team1/plugin/test-plugin');
            expect(window.location.search).toBe('?channel=general&state=note%2Fn2');

            // Back on its list: no state, the channel stays
            act(() => mockPanelProps.current?.setPanelState?.(null));
            expect(window.location.search).toBe('?channel=general');

            // What it started with doesn't change as it moves on
            expect(mockPanelProps.current?.initialPanelState).toBe('note/n1');
        });
    });
});

