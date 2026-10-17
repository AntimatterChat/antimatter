// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {type JSX} from 'react';

import * as UserAgent from '@mattermost/shared/utils/user_agent';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import {getHistory} from 'utils/browser_history';

import {OnboardingTasksName} from './constants';
import {useHandleOnBoardingTaskTrigger, useTasksList} from './onboarding_tasks_manager';

const isDesktopAppMock = jest.mocked(UserAgent.isDesktopApp);

jest.mock('@mattermost/shared/utils/user_agent', () => ({
    isDesktopApp: jest.fn(() => false),
}));

// The tasks save their progress as preferences; the tests don't need the server.
jest.mock('mattermost-redux/actions/preferences', () => ({
    ...jest.requireActual('mattermost-redux/actions/preferences'),
    savePreferences: jest.fn(() => ({type: 'MOCK_SAVE_PREFERENCES'})),
}));

const WrapperComponent = (): JSX.Element => {
    const taskList = useTasksList();
    return (
        <ul>
            {taskList.map((task: string) => <li key={task}>{task}</li>)}
        </ul>
    );
};

describe('onboarding tasks manager', () => {
    const user1 = 'user1';
    const user2 = 'user2';
    const user3 = 'user3';

    const initialState = {
        entities: {
            general: {},
            preferences: {},
            users: {
                currentUserId: user1,
                profiles: {
                    [user1]: {id: user1, username: user1, roles: 'system_admin'},
                    [user2]: {id: user2, username: user2, roles: 'system_user'},
                    [user3]: {id: user3, username: user3, roles: 'system_guest'},
                },
            },
            roles: {},
        },
    };

    it('Places all the elements (5 ignoring plugins) when user is first admin or admin', () => {
        renderWithContext(
            <WrapperComponent/>,
            initialState,
        );

        expect(screen.getAllByRole('listitem')).toHaveLength(5);

        // find the visit system console
        expect(screen.getByText('visit_system_console')).toBeInTheDocument();
    });

    it('Removes visit_system_console when user is end user', () => {
        const endUserState = {...initialState, entities: {...initialState.entities, users: {...initialState.entities.users, currentUserId: user2}}};

        renderWithContext(
            <WrapperComponent/>,
            endUserState,
        );

        expect(screen.getAllByRole('listitem')).toHaveLength(4);

        // verify visit_system_console was removed
        expect(screen.queryByText('visit_system_console')).not.toBeInTheDocument();
    });

    it('Removes invite people task item when user is GUEST user', () => {
        const endUserState = {...initialState, entities: {...initialState.entities, users: {...initialState.entities.users, currentUserId: user3}}};

        renderWithContext(
            <WrapperComponent/>,
            endUserState,
        );

        expect(screen.getAllByRole('listitem')).toHaveLength(3);

        // verify invite_people was removed
        expect(screen.queryByText('invite_people')).not.toBeInTheDocument();
    });

    it('Removes download_app task when running in desktop app', () => {
        isDesktopAppMock.mockReturnValue(true);
        renderWithContext(
            <WrapperComponent/>,
            initialState,
        );

        expect(screen.getAllByRole('listitem')).toHaveLength(4);
        expect(screen.queryByText('download_app')).not.toBeInTheDocument();
        isDesktopAppMock.mockReturnValue(false);
    });

    describe('the download_app task', () => {
        const DownloadButton = (): JSX.Element => {
            const trigger = useHandleOnBoardingTaskTrigger();
            return (
                <button onClick={() => trigger(OnboardingTasksName.DOWNLOAD_APP)}>
                    {'download'}
                </button>
            );
        };

        const openSpy = jest.spyOn(window, 'open').mockImplementation(() => null);
        afterEach(() => openSpy.mockClear());
        afterAll(() => openSpy.mockRestore());

        it('opens the Antimatter desktop app releases by default', async () => {
            renderWithContext(<DownloadButton/>, initialState);
            await userEvent.click(screen.getByRole('button', {name: 'download'}));
            expect(openSpy).toHaveBeenCalledWith('https://github.com/AntimatterChat/desktop-app/releases', '_blank', 'noopener,noreferrer');
        });

        it('opens the download page the server sets', async () => {
            const state = {...initialState, entities: {...initialState.entities, general: {config: {AppDownloadLink: 'https://downloads.example.com/chat'}}}};
            renderWithContext(<DownloadButton/>, state);
            await userEvent.click(screen.getByRole('button', {name: 'download'}));
            expect(openSpy).toHaveBeenCalledWith('https://downloads.example.com/chat', '_blank', 'noopener,noreferrer');
        });
    });
    // Antimatter: the product menu, where the tour pointed at the System Console, only shows when an app has a page of
    // its own; without it, the task opens the System Console.
    describe('the visit_system_console task', () => {
        const ConsoleButton = (): JSX.Element => {
            const trigger = useHandleOnBoardingTaskTrigger();
            return (
                <button onClick={() => trigger(OnboardingTasksName.VISIT_SYSTEM_CONSOLE)}>
                    {'console'}
                </button>
            );
        };

        it('opens the System Console when there is no product menu', async () => {
            const push = jest.spyOn(getHistory(), 'push').mockImplementation(() => {});
            renderWithContext(<ConsoleButton/>, initialState);

            await userEvent.click(screen.getByRole('button', {name: 'console'}));

            expect(push).toHaveBeenCalledWith('/admin_console');
            push.mockRestore();
        });

        it('opens the product menu when it shows', async () => {
            const push = jest.spyOn(getHistory(), 'push').mockImplementation(() => {});
            const menuButton = document.createElement('button');
            menuButton.id = 'switchProductMenuButton';
            const click = jest.fn();
            menuButton.addEventListener('click', click);
            document.body.appendChild(menuButton);
            renderWithContext(<ConsoleButton/>, initialState);

            await userEvent.click(screen.getByRole('button', {name: 'console'}));

            expect(click).toHaveBeenCalled();
            expect(push).not.toHaveBeenCalledWith('/admin_console');
            menuButton.remove();
            push.mockRestore();
        });
    });
});
