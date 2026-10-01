// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {ClientConfig} from '@mattermost/types/config';

import AboutBuildModal from 'components/about_build_modal/about_build_modal';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';
import {AboutLinks} from 'utils/constants';

describe('components/AboutBuildModal', () => {
    const RealDate: DateConstructor = Date;

    function mockDate(date: Date) {
        function mock() {
            return new RealDate(date);
        }
        mock.now = () => date.getTime();
        global.Date = mock as any;
    }

    let config: Partial<ClientConfig> = {};
    let socketStatus = {
        connected: false,
        serverHostname: '',
    };

    afterEach(() => {
        global.Date = RealDate;
        config = {};
        socketStatus = {
            connected: false,
            serverHostname: '',
        };
        jest.restoreAllMocks();
    });

    beforeEach(() => {
        mockDate(new Date(2017, 6, 1));

        config = {
            Version: '3.6.0',
            SchemaVersion: '77',
            BuildNumber: '123456',
            SQLDriverName: 'Postgres',
            BuildHash: 'abcdef1234567890',
            BuildDate: '21 January 2017',
            TermsOfServiceLink: AboutLinks.TERMS_OF_SERVICE,
            PrivacyPolicyLink: AboutLinks.PRIVACY_POLICY,
        };
        socketStatus = {
            connected: true,
            serverHostname: 'mock.localhost',
        };
    });

    test('should render build information', () => {
        renderAboutBuildModal({config, socketStatus});
        expect(screen.getByTestId('aboutModalVersionInfo')).toHaveTextContent('Server Version: 3.6.0');
        expect(screen.getByTestId('aboutModalVersionInfo')).toHaveTextContent('Database Schema Version: 77');
        expect(screen.getByTestId('aboutModalVersionInfo')).toHaveTextContent('Build Number: 123456');
        expect(screen.getByText('Antimatter')).toBeInTheDocument();
        expect(screen.getByText('All your team communication in one place, instantly searchable and accessible anywhere.')).toBeInTheDocument();
        expect(screen.getByRole('link', {name: 'github.com/AntimatterChat'})).toHaveAttribute('href', 'https://github.com/AntimatterChat');
        expect(screen.getByText('Build Hash: abcdef1234567890', {exact: false})).toBeInTheDocument();
        expect(screen.queryByText('EE Build Hash', {exact: false})).not.toBeInTheDocument();
        expect(screen.queryByText('Licensed to', {exact: false})).not.toBeInTheDocument();
        expect(screen.queryByText('Hostname: mock.localhost', {exact: false})).toBeInTheDocument();

        expect(screen.getByRole('link', {name: 'server'})).toHaveAttribute('href', 'https://github.com/AntimatterChat/antimatter/blob/antimatter/NOTICE.txt');
        expect(screen.getByRole('link', {name: 'desktop'})).toHaveAttribute('href', 'https://github.com/mattermost/desktop/blob/master/NOTICE.txt');
        expect(screen.getByRole('link', {name: 'mobile'})).toHaveAttribute('href', 'https://github.com/mattermost/mattermost-mobile/blob/master/NOTICE.txt');
    });

    test('should show n/a if this is a dev build', () => {
        const sameBuildConfig = {
            ...config,
            Version: '3.6.0',
            SchemaVersion: '77',
            BuildNumber: 'dev',
        };

        renderAboutBuildModal({config: sameBuildConfig, socketStatus: {connected: true}});

        expect(screen.getByTestId('aboutModalVersionInfo')).toHaveTextContent('Server Version: dev');
        expect(screen.getByTestId('aboutModalVersionInfo')).toHaveTextContent('Database Schema Version: 77');
        expect(screen.getByTestId('aboutModalVersionInfo')).toHaveTextContent('Build Number: n/a');
        expect(screen.getByRole('link', {name: 'github.com/AntimatterChat'})).toHaveAttribute('href', 'https://github.com/AntimatterChat');
        expect(screen.queryByText('Hostname: server did not provide hostname', {exact: false})).toBeInTheDocument();

        expect(screen.getByRole('link', {name: 'server'})).toHaveAttribute('href', 'https://github.com/AntimatterChat/antimatter/blob/antimatter/NOTICE.txt');
        expect(screen.getByRole('link', {name: 'desktop'})).toHaveAttribute('href', 'https://github.com/mattermost/desktop/blob/master/NOTICE.txt');
        expect(screen.getByRole('link', {name: 'mobile'})).toHaveAttribute('href', 'https://github.com/mattermost/mattermost-mobile/blob/master/NOTICE.txt');
    });

    test('should call onExited callback when the modal is hidden', async () => {
        const onExited = jest.fn();
        const state = {
            entities: {
                general: {
                    config: {},
                },
                users: {
                    currentUserId: 'currentUserId',
                },
            },
        };

        renderWithContext(
            <AboutBuildModal
                config={config}
                socketStatus={socketStatus}
                onExited={onExited}
            />,
            state,
        );

        await userEvent.click(screen.getByText('Close'));
        expect(onExited).toHaveBeenCalledTimes(1);
    });

    test('should show default tos and privacy policy links and not the config links', () => {
        const state = {
            entities: {
                general: {
                    config,
                },
                users: {
                    currentUserId: 'currentUserId',
                },
            },
        };
        renderWithContext(
            <AboutBuildModal
                config={config}
                socketStatus={socketStatus}
                onExited={jest.fn()}
            />,
            state,
        );

        expect(screen.getByRole('link', {name: 'Terms of Use'})).toHaveAttribute('href', `${AboutLinks.TERMS_OF_SERVICE}?utm_source=mattermost&utm_medium=in-product&utm_content=about_build_modal&uid=currentUserId&sid=&server_version=3.6.0`);

        expect(screen.getByRole('link', {name: 'Privacy Policy'})).toHaveAttribute('href', `${AboutLinks.PRIVACY_POLICY}?utm_source=mattermost&utm_medium=in-product&utm_content=about_build_modal&uid=currentUserId&sid=&server_version=3.6.0`);

        expect(screen.getByRole('link', {name: 'Terms of Use'})).not.toHaveAttribute('href', config?.TermsOfServiceLink);
        expect(screen.getByRole('link', {name: 'Privacy Policy'})).not.toHaveAttribute('href', config?.PrivacyPolicyLink);
    });

    test('should show FIPS indicator when IsFipsEnabled is true', () => {
        const fipsConfig = {
            ...config,
            IsFipsEnabled: 'true',
        };

        renderAboutBuildModal({config: fipsConfig});

        expect(screen.getByTestId('aboutModalVersionInfo')).toHaveTextContent('Server Version: 3.6.0 (FIPS)');
    });

    test('should not show FIPS indicator when IsFipsEnabled is false', () => {
        const nonFipsConfig = {
            ...config,
            IsFipsEnabled: 'false',
        };

        renderAboutBuildModal({config: nonFipsConfig});

        expect(screen.getByTestId('aboutModalVersionInfo')).toHaveTextContent('Server Version: 3.6.0');
        expect(screen.getByTestId('aboutModalVersionInfo')).not.toHaveTextContent('(FIPS)');
    });

    test('should not show FIPS indicator when IsFipsEnabled is not set', () => {
        const nonFipsConfig = {
            ...config,
        };

        renderAboutBuildModal({config: nonFipsConfig});

        expect(screen.getByTestId('aboutModalVersionInfo')).toHaveTextContent('Server Version: 3.6.0');
        expect(screen.getByTestId('aboutModalVersionInfo')).not.toHaveTextContent('(FIPS)');
    });

    function renderAboutBuildModal(props = {}) {
        const onExited = jest.fn();
        const show = true;

        const allProps = {
            show,
            onExited,
            config,
            socketStatus,
            ...props,
        };

        // Create state with the config for useExternalLink hook to access
        const state = {
            entities: {
                general: {
                    config: allProps.config,
                },
                users: {
                    currentUserId: '',
                },
            },
        };

        return renderWithContext(<AboutBuildModal {...allProps}/>, state);
    }
});
