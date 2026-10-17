// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {DeepPartial} from '@mattermost/types/utilities';

import {act, fireEvent, renderWithContext, screen} from 'tests/react_testing_utils';
import {popoutRhsPlugin} from 'utils/popouts/popout_windows';

import type {GlobalState} from 'types/store';

import RhsPlugin from './rhs_plugin';

import ConnectedRhsPlugin from '.';

jest.mock('components/search_results_header', () => ({
    __esModule: true,
    default: ({children, newWindowHandler}: {children: React.ReactNode; newWindowHandler?: () => void}) => (
        <div
            data-testid='search-results-header'
            data-has-window-handler={newWindowHandler === undefined ? 'false' : 'true'}
        >
            {newWindowHandler && (
                <button
                    type='button'
                    onClick={newWindowHandler}
                >
                    {'Open in new window'}
                </button>
            )}
            {children}
        </div>
    ),
}));

// The app's panel: the props the host gives it, kept for the tests to call setPanelState as the app would
const mockPanelProps: {current?: {pluggableId?: string; setPanelState?: (state: string | null) => void; initialPanelState?: string}} = {};
jest.mock('plugins/pluggable', () => ({
    __esModule: true,
    default: (props: {pluggableId?: string; setPanelState?: (state: string | null) => void; initialPanelState?: string}) => {
        mockPanelProps.current = props;
        return <div data-testid='pluggable'/>;
    },
}));

jest.mock('utils/popouts/popout_windows', () => ({
    popoutRhsPlugin: jest.fn(),
}));

const baseState: DeepPartial<GlobalState> = {
    entities: {
        general: {config: {}},
        teams: {
            currentTeamId: 'team-id',
            teams: {'team-id': {id: 'team-id', name: 'test-team'}},
        },
        channels: {
            currentChannelId: 'channel-id',
            channels: {'channel-id': {id: 'channel-id', name: 'test-channel'}},
        },
        preferences: {myPreferences: {}},
    },
    plugins: {
        plugins: {'plugin-id': {name: 'Test Plugin'}},
        components: {
            RightHandSidebarComponent: [],
        },
    },
    views: {
        rhs: {pluggableId: ''},
    },
};

describe('RhsPlugin', () => {
    describe('component', () => {
        it('passes newWindowHandler to SearchResultsHeader when showPopout is true', () => {
            renderWithContext(
                <RhsPlugin
                    showPluggable={true}
                    pluggableId='pluggable-id'
                    title='Test Title'
                    pluginId='plugin-id'
                    showPopout={true}
                />,
                baseState,
            );

            expect(screen.getByTestId('search-results-header')).toHaveAttribute('data-has-window-handler', 'true');
        });

        it('passes undefined for newWindowHandler to SearchResultsHeader when showPopout is false', () => {
            renderWithContext(
                <RhsPlugin
                    showPluggable={true}
                    pluggableId='pluggable-id'
                    title='Test Title'
                    pluginId='plugin-id'
                    showPopout={false}
                />,
                baseState,
            );

            expect(screen.getByTestId('search-results-header')).toHaveAttribute('data-has-window-handler', 'false');
        });

        it('defaults showPopout to true when the prop is omitted', () => {
            renderWithContext(
                <RhsPlugin
                    showPluggable={true}
                    pluggableId='pluggable-id'
                    title='Test Title'
                    pluginId='plugin-id'
                />,
                baseState,
            );

            expect(screen.getByTestId('search-results-header')).toHaveAttribute('data-has-window-handler', 'true');
        });
    });

    describe('mapStateToProps', () => {
        const pluggableId = 'pluggable-id';

        function stateWithRegisteredComponent(showPopout?: boolean): DeepPartial<GlobalState> {
            const component: Record<string, unknown> = {
                id: pluggableId,
                pluginId: 'plugin-id',
                title: 'Test Title',
            };
            if (showPopout !== undefined) {
                component.showPopout = showPopout;
            }
            return {
                ...baseState,
                plugins: {
                    ...baseState.plugins,
                    components: {
                        RightHandSidebarComponent: [component as any],
                    },
                },
                views: {
                    rhs: {pluggableId},
                },
            };
        }

        it('defaults showPopout to true when component showPopout field is undefined', () => {
            renderWithContext(<ConnectedRhsPlugin/>, stateWithRegisteredComponent(undefined));

            expect(screen.getByTestId('search-results-header')).toHaveAttribute('data-has-window-handler', 'true');
        });

        it('passes showPopout: false through to hide the popout button', () => {
            renderWithContext(<ConnectedRhsPlugin/>, stateWithRegisteredComponent(false));

            expect(screen.getByTestId('search-results-header')).toHaveAttribute('data-has-window-handler', 'false');
        });
    });
    describe('panel state', () => {
        const props = {
            showPluggable: true,
            pluggableId: 'pluggable-id',
            title: 'Test Title',
            pluginId: 'plugin-id',
        };

        beforeEach(() => {
            mockPanelProps.current = undefined;
        });

        it('gives the app a setPanelState prop and no initial state in the main window', () => {
            renderWithContext(<RhsPlugin {...props}/>, baseState);

            expect(typeof mockPanelProps.current?.setPanelState).toBe('function');
            expect(mockPanelProps.current?.initialPanelState).toBeUndefined();
        });

        it('opens the popout where the app said it is', () => {
            renderWithContext(<RhsPlugin {...props}/>, baseState);

            act(() => mockPanelProps.current?.setPanelState?.('note/n1'));
            fireEvent.click(screen.getByRole('button', {name: 'Open in new window'}));

            expect(popoutRhsPlugin).toHaveBeenCalledWith(expect.any(String), 'plugin-id', 'test-team', 'test-channel', 'note/n1');
        });

        it('opens the popout on the home of an app that never set a state, or went back to its home', () => {
            renderWithContext(<RhsPlugin {...props}/>, baseState);

            fireEvent.click(screen.getByRole('button', {name: 'Open in new window'}));
            expect(popoutRhsPlugin).toHaveBeenLastCalledWith(expect.any(String), 'plugin-id', 'test-team', 'test-channel', undefined);

            act(() => mockPanelProps.current?.setPanelState?.('note/n1'));
            act(() => mockPanelProps.current?.setPanelState?.(null));
            fireEvent.click(screen.getByRole('button', {name: 'Open in new window'}));
            expect(popoutRhsPlugin).toHaveBeenLastCalledWith(expect.any(String), 'plugin-id', 'test-team', 'test-channel', undefined);
        });

        it('drops a state it would not put in a URL', () => {
            renderWithContext(<RhsPlugin {...props}/>, baseState);

            act(() => mockPanelProps.current?.setPanelState?.('note/n1\nsecond line'));
            fireEvent.click(screen.getByRole('button', {name: 'Open in new window'}));

            expect(popoutRhsPlugin).toHaveBeenLastCalledWith(expect.any(String), 'plugin-id', 'test-team', 'test-channel', undefined);
        });

        it("forgets the state when another app's panel takes its place", () => {
            const {rerender} = renderWithContext(<RhsPlugin {...props}/>, baseState);
            act(() => mockPanelProps.current?.setPanelState?.('note/n1'));

            rerender(
                <RhsPlugin
                    {...props}
                    pluggableId='other-pluggable-id'
                    pluginId='other-plugin-id'
                />,
            );
            fireEvent.click(screen.getByRole('button', {name: 'Open in new window'}));

            expect(popoutRhsPlugin).toHaveBeenLastCalledWith(expect.any(String), 'other-plugin-id', 'test-team', 'test-channel', undefined);
        });
    });
});
