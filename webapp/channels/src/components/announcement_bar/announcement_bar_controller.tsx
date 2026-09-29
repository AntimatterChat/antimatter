// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {ClientConfig} from '@mattermost/types/config';

import ConfigurationAnnouncementBar from './configuration_bar';
import AnnouncementBar from './default_announcement_bar';
import NotificationPermissionBar from './notification_permission_bar';
import TextDismissableBar from './text_dismissable_bar';
import VersionBar from './version_bar';

type Props = {
    config?: Partial<ClientConfig>;
    canViewSystemErrors: boolean;
    latestError?: {
        error: any;
    };
    actions: {
        dismissError: (index: number) => void;
    };
};

class AnnouncementBarController extends React.PureComponent<Props> {
    render() {
        let adminConfiguredAnnouncementBar = null;
        if (this.props.config?.EnableBanner === 'true' && this.props.config.BannerText?.trim()) {
            adminConfiguredAnnouncementBar = (
                <TextDismissableBar
                    className='admin-announcement'
                    color={this.props.config.BannerColor}
                    textColor={this.props.config.BannerTextColor}
                    allowDismissal={this.props.config.AllowBannerDismissal === 'true'}
                    text={this.props.config.BannerText}
                />
            );
        }

        let errorBar = null;
        if (this.props.latestError) {
            errorBar = (
                <AnnouncementBar
                    type={this.props.latestError.error.type}
                    message={this.props.latestError.error.message}
                    showCloseButton={true}
                    handleClose={this.props.actions.dismissError}
                />
            );
        }

        // The component specified further down takes priority over the component above it.
        // For example, consider this-
        // {
        //    Foo
        //    Bar
        //    Baz
        // }
        // Even if all Foo, Bar and Baz render, only Baz is visible as it's further down.
        // One exception to this rule is for admin configured announcement banners
        // If set with class 'admin-announcement', they will always be visible, stacked vertically.
        return (
            <>
                <NotificationPermissionBar/>
                {adminConfiguredAnnouncementBar}
                {errorBar}
                <VersionBar/>
                <ConfigurationAnnouncementBar
                    config={this.props.config}
                    canViewSystemErrors={this.props.canViewSystemErrors}
                />
            </>
        );
    }
}

export default AnnouncementBarController;
