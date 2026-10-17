// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useSelector} from 'react-redux';

import * as UserAgent from '@mattermost/shared/utils/user_agent';
import type {DeepPartial} from '@mattermost/types/utilities';

import {Permissions} from 'mattermost-redux/constants';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import type {GlobalState} from 'types/store';

import {appDownloadLink, canOpenMarketplace, canVisitSystemConsole} from './admin_links';

jest.mock('@mattermost/shared/utils/user_agent', () => ({
    ...jest.requireActual('@mattermost/shared/utils/user_agent'),
    isDesktopApp: jest.fn(() => false),
}));

// Antimatter: the product menu only shows when an app has a page of its own; its System Console, App Marketplace and
// download entries are in Fusion's menus, for the same people.
describe('fusion/popovers/admin_links', () => {
    const Probe = () => {
        const systemConsole = useSelector(canVisitSystemConsole);
        const marketplace = useSelector(canOpenMarketplace);
        const download = useSelector(appDownloadLink);
        return (
            <ul>
                <li>{`console:${systemConsole}`}</li>
                <li>{`marketplace:${marketplace}`}</li>
                <li>{`download:${download}`}</li>
            </ul>
        );
    };

    const state = ({roles = 'system_user', permissions = [] as string[], config = {}} = {}): DeepPartial<GlobalState> => ({
        entities: {
            general: {config},
            users: {
                currentUserId: 'user_id',
                profiles: {
                    user_id: TestHelper.getUserMock({id: 'user_id', roles}),
                },
            },
            teams: {
                currentTeamId: 'team_id',
                teams: {team_id: TestHelper.getTeamMock({id: 'team_id'})},
                myMembers: {team_id: TestHelper.getTeamMembershipMock({team_id: 'team_id', user_id: 'user_id', roles: 'team_user'})},
            },
            roles: {
                roles: {
                    system_user: {permissions},
                    team_user: {permissions: []},
                },
            },
        },
    });

    test('a member without any System Console permission has no System Console entry', () => {
        renderWithContext(<Probe/>, state());
        expect(screen.getByText('console:false')).toBeInTheDocument();
    });

    test('a system admin has the System Console entry', () => {
        renderWithContext(<Probe/>, state({roles: 'system_user system_admin'}));
        expect(screen.getByText('console:true')).toBeInTheDocument();
    });

    test('a role that may read a part of the System Console (a system manager...) has it too', () => {
        renderWithContext(<Probe/>, state({permissions: [Permissions.SYSCONSOLE_READ_REPORTING_SITE_STATISTICS]}));
        expect(screen.getByText('console:true')).toBeInTheDocument();
    });

    test('the App Marketplace entry needs the marketplace on and the right to install apps', () => {
        const on = {PluginsEnabled: 'true', EnableMarketplace: 'true'};

        const {unmount} = renderWithContext(<Probe/>, state({config: on}));
        expect(screen.getByText('marketplace:false')).toBeInTheDocument();
        unmount();

        const {unmount: unmount2} = renderWithContext(<Probe/>, state({config: {...on, EnableMarketplace: 'false'}, permissions: [Permissions.SYSCONSOLE_WRITE_PLUGINS]}));
        expect(screen.getByText('marketplace:false')).toBeInTheDocument();
        unmount2();

        renderWithContext(<Probe/>, state({config: on, permissions: [Permissions.SYSCONSOLE_WRITE_PLUGINS]}));
        expect(screen.getByText('marketplace:true')).toBeInTheDocument();
    });

    test('the apps download link is the server\'s, and not in the desktop app', () => {
        const config = {AppDownloadLink: 'https://downloads.example.com/apps'};

        const {unmount} = renderWithContext(<Probe/>, state({config}));
        expect(screen.getByText('download:https://downloads.example.com/apps')).toBeInTheDocument();
        unmount();

        jest.mocked(UserAgent.isDesktopApp).mockReturnValue(true);
        const {unmount: unmount2} = renderWithContext(<Probe/>, state({config}));
        expect(screen.getByText('download:')).toBeInTheDocument();
        unmount2();
        jest.mocked(UserAgent.isDesktopApp).mockReturnValue(false);

        // eslint-disable-next-line no-script-url -- the test: a javascript: link in the config isn't offered for download
        renderWithContext(<Probe/>, state({config: {AppDownloadLink: 'javascript:alert(1)'}}));
        expect(screen.queryByText('download:javascript:alert(1)')).not.toBeInTheDocument();
    });
});
