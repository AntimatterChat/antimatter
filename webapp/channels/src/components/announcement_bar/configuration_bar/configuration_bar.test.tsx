// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import ConfigurationBar from 'components/announcement_bar/configuration_bar/configuration_bar';

import {renderWithContext, screen} from 'tests/react_testing_utils';

describe('components/ConfigurationBar', () => {
    const baseProps = {
        canViewSystemErrors: true,
        config: {
            SendEmailNotifications: 'true',
            SiteURL: 'http://localhost:8065',
        },
        siteURL: 'http://localhost:8065',
    };

    test('should show nothing when everything is configured', () => {
        const {container} = renderWithContext(
            <ConfigurationBar {...baseProps}/>,
        );

        expect(container).toBeEmptyDOMElement();
    });

    test('should show the preview mode bar when email notifications are not configured', () => {
        const props = {...baseProps, config: {...baseProps.config, SendEmailNotifications: 'false', EnablePreviewModeBanner: 'true'}};
        renderWithContext(
            <ConfigurationBar {...props}/>,
        );

        expect(screen.getByText('Preview Mode: Email notifications have not been configured.')).toBeInTheDocument();
    });

    test('should ask system admins to configure the site URL', () => {
        const props = {...baseProps, config: {...baseProps.config, SiteURL: ''}};
        renderWithContext(
            <ConfigurationBar {...props}/>,
        );

        expect(screen.getByText('site URL')).toBeInTheDocument();
    });

    test('should not show the site URL bar to regular users', () => {
        const props = {...baseProps, canViewSystemErrors: false, config: {...baseProps.config, SiteURL: ''}};
        const {container} = renderWithContext(
            <ConfigurationBar {...props}/>,
        );

        expect(container).toBeEmptyDOMElement();
    });
});
