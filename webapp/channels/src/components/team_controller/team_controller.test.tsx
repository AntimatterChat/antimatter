// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {ComponentProps} from 'react';
import React from 'react';
import {MemoryRouter, Route} from 'react-router-dom';

import {act, renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import TeamController from './team_controller';

jest.mock('components/async_load', () => ({
    makeAsyncComponent: () => () => null,
    makeAsyncPluggableComponent: () => ({pluggableName}: {pluggableName: string}) => (
        <div data-testid={`pluggable-${pluggableName}`}/>
    ),
}));

jest.mock('components/common/hooks/useTelemetryIdentifySync', () => ({
    __esModule: true,
    default: () => {},
}));

jest.mock('utils/desktop_api', () => ({
    __esModule: true,
    default: {reactAppInitialized: jest.fn()},
}));

jest.mock('components/initial_loading_screen', () => ({
    __esModule: true,
    default: {stop: jest.fn()},
}));

jest.mock('stores/local_storage_store', () => ({
    __esModule: true,
    default: {setTeamIdJoinedOnLoad: jest.fn(), setPreviousTeamId: jest.fn()},
}));

const team = TestHelper.getTeamMock({id: 'team_id_1', name: 'myteam'});
const product = {
    ...TestHelper.makeProduct('spaces'),
    baseURL: '/spaces',
    switcherLinkURL: '/spaces',
    isTeamScoped: true,
    wrapped: false,
};

function baseProps(currentTeamId: string): ComponentProps<typeof TeamController> {
    return {
        currentTeamId,
        currentChannelId: '',
        teamsList: [team],
        plugins: [],
        products: [product],
        selectedThreadId: null,
        selectedPostId: '',
        mfaRequired: false,
        disableRefetchingOnBrowserFocus: false,
        disableWakeUpReconnectHandler: true,
        fetchChannelsAndMembers: jest.fn(),
        fetchAllMyTeamsChannels: jest.fn().mockResolvedValue({data: []}),
        fetchAllMyChannelMembers: jest.fn(),
        markAsReadOnFocus: jest.fn(),
        initializeTeam: jest.fn().mockResolvedValue({data: team}),
        joinTeam: jest.fn().mockResolvedValue({data: team}),
        unsetActiveChannelOnServer: jest.fn(),
        match: {params: {team: 'myteam'}, isExact: true, path: '/:team', url: '/myteam'},
    } as unknown as ComponentProps<typeof TeamController>;
}

function renderController(currentTeamId: string) {
    return renderWithContext(
        <MemoryRouter initialEntries={['/myteam/spaces']}>
            <Route path='/:team'>
                <TeamController {...baseProps(currentTeamId)}/>
            </Route>
        </MemoryRouter>,
    );
}

describe('TeamController — team-scoped products', () => {
    it('renders the team-scoped product when the URL team is the current team', async () => {
        renderController('team_id_1');
        await act(async () => {});

        expect(screen.getByTestId('pluggable-Product')).toBeInTheDocument();
    });

    it('withholds the team-scoped product until the current team matches the URL team', async () => {
        renderController('other_team_id');
        await act(async () => {});

        expect(screen.queryByTestId('pluggable-Product')).not.toBeInTheDocument();
    });

    it('renders the product once the current team catches up to the URL team', async () => {
        const {rerender} = renderController('other_team_id');
        await act(async () => {});

        expect(screen.queryByTestId('pluggable-Product')).not.toBeInTheDocument();

        rerender(
            <MemoryRouter initialEntries={['/myteam/spaces']}>
                <Route path='/:team'>
                    <TeamController {...baseProps('team_id_1')}/>
                </Route>
            </MemoryRouter>,
        );
        await act(async () => {});

        expect(screen.getByTestId('pluggable-Product')).toBeInTheDocument();
    });
});

// The apps' pages (Kanban at /kanban, once /boards; Playbooks) are no teams: before the app's routes
// are registered, the team route may see their address, and must not try to join a team by that name.
describe('TeamController — reserved names', () => {
    it.each(['kanban', 'boards', 'playbooks'])('does not join or load a team named %s', async (name) => {
        const props = {
            ...baseProps('team_id_1'),
            match: {params: {team: name}, isExact: false, path: '/:team', url: `/${name}`},
        } as unknown as ComponentProps<typeof TeamController>;

        renderWithContext(
            <MemoryRouter initialEntries={[`/${name}/team/team_id_1`]}>
                <Route path='/:team'>
                    <TeamController {...props}/>
                </Route>
            </MemoryRouter>,
        );
        await act(async () => {});

        expect(props.joinTeam).not.toHaveBeenCalled();
        expect(props.initializeTeam).not.toHaveBeenCalled();
    });
});
