// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {DeepPartial} from '@mattermost/types/utilities';

import {act, fireEvent, renderWithContext, screen} from 'tests/react_testing_utils';
import {isPopoutWindow, popoutRhsPlugin} from 'utils/popouts/popout_windows';

import type {GlobalState} from 'types/store';

import RhsPlugin from './rhs_plugin';

import ConnectedRhsPlugin from '.';

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
    isPopoutWindow: jest.fn(() => false),
    isChannelPopoutWindow: jest.fn(() => false),
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
        const newWindow = () => screen.queryByRole('button', {name: 'Open in a new window'});

        it('draws the generic app header, with its new window button when showPopout is true', () => {
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

            expect(screen.getByTestId('app-panel-header')).toHaveTextContent('Test Title');
            expect(newWindow()).toBeInTheDocument();
            expect(screen.getByRole('button', {name: 'Close'})).toBeInTheDocument();
        });

        it('has no new window button when showPopout is false', () => {
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

            expect(newWindow()).not.toBeInTheDocument();
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

            expect(newWindow()).toBeInTheDocument();
        });

        it('closes the panel from the header', () => {
            const {store} = renderWithContext(
                <RhsPlugin
                    showPluggable={true}
                    pluggableId='pluggable-id'
                    title='Test Title'
                    pluginId='plugin-id'
                />,
                {...baseState, views: {rhs: {pluggableId: 'pluggable-id', isSidebarOpen: true, rhsState: 'plugin'}}} as DeepPartial<GlobalState>,
            );

            fireEvent.click(screen.getByRole('button', {name: 'Close'}));
            expect(store.getState().views.rhs.isSidebarOpen).toBe(false);
        });

        it('has no close button in a popout window of the panel', () => {
            (isPopoutWindow as jest.Mock).mockReturnValue(true);
            renderWithContext(
                <RhsPlugin
                    showPluggable={true}
                    pluggableId='pluggable-id'
                    title='Test Title'
                    pluginId='plugin-id'
                />,
                baseState,
            );

            expect(screen.queryByRole('button', {name: 'Close'})).not.toBeInTheDocument();
            (isPopoutWindow as jest.Mock).mockReturnValue(false);
        });

        it("shows the restore size button once resized, which restores the panel's size", () => {
            const onRestoreSize = jest.fn();
            const props = {showPluggable: true, pluggableId: 'pluggable-id', title: 'Test Title', pluginId: 'plugin-id', onRestoreSize};
            const {rerender} = renderWithContext(<RhsPlugin {...props}/>, baseState);
            expect(screen.queryByRole('button', {name: "Restore the panel's size"})).not.toBeInTheDocument();

            rerender(
                <RhsPlugin
                    {...props}
                    sized={true}
                />,
            );
            fireEvent.click(screen.getByRole('button', {name: "Restore the panel's size"}));
            expect(onRestoreSize).toHaveBeenCalledTimes(1);
        });

        it("puts nothing of the app's title in the header but its text", () => {
            renderWithContext(
                <RhsPlugin
                    showPluggable={true}
                    pluggableId='pluggable-id'
                    title={<button type='button'>{'App control'}</button>}
                    pluginId='plugin-id'
                />,
                baseState,
            );

            expect(screen.queryByRole('button', {name: 'App control'})).not.toBeInTheDocument();
            expect(screen.getByTestId('app-panel-header')).toHaveTextContent('Test Plugin');
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

            expect(screen.getByRole('button', {name: 'Open in a new window'})).toBeInTheDocument();
        });

        it('passes showPopout: false through to hide the popout button', () => {
            renderWithContext(<ConnectedRhsPlugin/>, stateWithRegisteredComponent(false));

            expect(screen.queryByRole('button', {name: 'Open in a new window'})).not.toBeInTheDocument();
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
            fireEvent.click(screen.getByRole('button', {name: 'Open in a new window'}));

            expect(popoutRhsPlugin).toHaveBeenCalledWith(expect.any(String), 'plugin-id', 'test-team', 'test-channel', 'note/n1');
        });

        it('opens the popout on the home of an app that never set a state, or went back to its home', () => {
            renderWithContext(<RhsPlugin {...props}/>, baseState);

            fireEvent.click(screen.getByRole('button', {name: 'Open in a new window'}));
            expect(popoutRhsPlugin).toHaveBeenLastCalledWith(expect.any(String), 'plugin-id', 'test-team', 'test-channel', undefined);

            act(() => mockPanelProps.current?.setPanelState?.('note/n1'));
            act(() => mockPanelProps.current?.setPanelState?.(null));
            fireEvent.click(screen.getByRole('button', {name: 'Open in a new window'}));
            expect(popoutRhsPlugin).toHaveBeenLastCalledWith(expect.any(String), 'plugin-id', 'test-team', 'test-channel', undefined);
        });

        it('drops a state it would not put in a URL', () => {
            renderWithContext(<RhsPlugin {...props}/>, baseState);

            act(() => mockPanelProps.current?.setPanelState?.('note/n1\nsecond line'));
            fireEvent.click(screen.getByRole('button', {name: 'Open in a new window'}));

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
            fireEvent.click(screen.getByRole('button', {name: 'Open in a new window'}));

            expect(popoutRhsPlugin).toHaveBeenLastCalledWith(expect.any(String), 'other-plugin-id', 'test-team', 'test-channel', undefined);
        });
    });
});
